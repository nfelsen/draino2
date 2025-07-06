package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/go-logr/zapr"
	"go.uber.org/zap"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/nfelsen/draino2/internal/api"
	appconfig "github.com/nfelsen/draino2/internal/config"
	"github.com/nfelsen/draino2/internal/controller"
	"github.com/nfelsen/draino2/internal/drainer"
	"github.com/nfelsen/draino2/internal/metrics"
)

func main() {
	var configFile string
	flag.StringVar(&configFile, "config-file", "config/draino2.yaml", "Path to config file")
	flag.Parse()

	// Load configuration
	if err := appconfig.LoadConfig(configFile); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}
	cfg := appconfig.GetConfig()

	// Setup logging
	zapLog, err := zap.NewProduction()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create logger: %v\n", err)
		os.Exit(1)
	}
	log := zapr.NewLogger(zapLog)

	// Get Kubernetes config
	kubeConfig := appconfig.GetConfigOrDie()

	// Create manager
	mgr, err := manager.New(kubeConfig, manager.Options{
		Logger: log,
	})
	if err != nil {
		log.Error(err, "unable to start manager")
		os.Exit(1)
	}

	// Create Kubernetes client
	kubeClient, err := kubernetes.NewForConfig(kubeConfig)
	if err != nil {
		log.Error(err, "unable to create kubernetes client")
		os.Exit(1)
	}

	// Create metrics
	metrics := metrics.NewMetrics()

	// Create drainer
	drainerConfig := &drainer.DrainerConfig{
		GracePeriod:        cfg.DrainSettings.MaxGracePeriod,
		Timeout:            cfg.DrainSettings.DrainBuffer,
		Force:              cfg.DrainSettings.EvictUnreplicatedPods,
		IgnoreDaemonSets:   !cfg.DrainSettings.EvictDaemonSetPods,
		DeleteEmptyDirData: cfg.DrainSettings.EvictLocalStoragePods,
		PodSelector:        nil, // TODO: Add pod selector configuration
	}
	drainer := drainer.NewDrainer(kubeClient, mgr.GetEventRecorderFor("draino2"), drainerConfig)

	// Create and register controller
	drainController := &controller.DrainController{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		Recorder: mgr.GetEventRecorderFor("draino2"),
		Config:   &cfg,
		Drainer:  drainer,
		Metrics:  metrics,
	}

	if err := drainController.SetupWithManager(mgr); err != nil {
		log.Error(err, "unable to create controller")
		os.Exit(1)
	}

	// Start API server if enabled
	var apiServer *api.Server
	if cfg.API.Enabled {
		apiServer = api.NewServer(kubeClient, drainer, metrics, &cfg, zapLog)
		go func() {
			log.Info("Starting API server", "port", cfg.API.Port)
			if err := apiServer.Start(cfg.API.Port); err != nil {
				log.Error(err, "API server failed")
			}
		}()
	}

	// Setup graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Info("Received shutdown signal")
		cancel()

		// Stop API server gracefully
		if apiServer != nil {
			shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30)
			defer shutdownCancel()
			if err := apiServer.Stop(shutdownCtx); err != nil {
				log.Error(err, "Failed to stop API server gracefully")
			}
		}
	}()

	log.Info("starting manager")
	if err := mgr.Start(ctx); err != nil {
		log.Error(err, "problem running manager")
		os.Exit(1)
	}
}
