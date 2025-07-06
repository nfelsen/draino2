package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/nfelsen/draino2/internal/drainer"
	"github.com/nfelsen/draino2/internal/metrics"
	"github.com/nfelsen/draino2/internal/types"
)

// Server represents the API server
type Server struct {
	client  kubernetes.Interface
	drainer *drainer.Drainer
	metrics *metrics.Metrics
	config  *types.Config
	logger  *zap.Logger
	router  *mux.Router
	server  *http.Server
}

// NewServer creates a new API server
func NewServer(client kubernetes.Interface, drainer *drainer.Drainer, metrics *metrics.Metrics, config *types.Config, logger *zap.Logger) *Server {
	s := &Server{
		client:  client,
		drainer: drainer,
		metrics: metrics,
		config:  config,
		logger:  logger,
		router:  mux.NewRouter(),
	}

	s.setupRoutes()
	return s
}

// setupRoutes configures all API routes
func (s *Server) setupRoutes() {
	// Health checks
	s.router.HandleFunc("/healthz", s.healthCheck).Methods("GET")
	s.router.HandleFunc("/readyz", s.readyCheck).Methods("GET")

	// Metrics
	s.router.Handle("/metrics", promhttp.Handler()).Methods("GET")

	// API v1 routes
	apiV1 := s.router.PathPrefix("/api/v1").Subrouter()

	// Node management
	apiV1.HandleFunc("/nodes", s.listNodes).Methods("GET")
	apiV1.HandleFunc("/nodes/{name}/drain", s.drainNode).Methods("POST")
	apiV1.HandleFunc("/nodes/{name}/cordon", s.cordonNode).Methods("POST")
	apiV1.HandleFunc("/nodes/{name}/uncordon", s.uncordonNode).Methods("POST")
	apiV1.HandleFunc("/nodes/{name}", s.getNode).Methods("GET")

	// Middleware
	s.router.Use(s.loggingMiddleware)
	s.router.Use(s.corsMiddleware)
}

// Start starts the HTTP server
func (s *Server) Start(port int) error {
	s.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: s.router,
	}

	s.logger.Info("Starting API server", zap.Int("port", port))
	return s.server.ListenAndServe()
}

// Stop gracefully stops the server
func (s *Server) Stop(ctx context.Context) error {
	if s.server != nil {
		return s.server.Shutdown(ctx)
	}
	return nil
}

// healthCheck handles health check requests
func (s *Server) healthCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status": "healthy",
		"time":   time.Now().UTC().Format(time.RFC3339),
	})
}

// readyCheck handles readiness check requests
func (s *Server) readyCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status": "ready",
		"time":   time.Now().UTC().Format(time.RFC3339),
	})
}

// listNodes returns all nodes
func (s *Server) listNodes(w http.ResponseWriter, r *http.Request) {
	nodes, err := s.client.CoreV1().Nodes().List(r.Context(), metav1.ListOptions{})
	if err != nil {
		s.logger.Error("Failed to list nodes", zap.Error(err))
		http.Error(w, "Failed to list nodes", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(nodes.Items)
}

// getNode returns a specific node
func (s *Server) getNode(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	nodeName := vars["name"]

	node, err := s.client.CoreV1().Nodes().Get(r.Context(), nodeName, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			http.Error(w, "Node not found", http.StatusNotFound)
			return
		}
		s.logger.Error("Failed to get node", zap.String("node", nodeName), zap.Error(err))
		http.Error(w, "Failed to get node", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(node)
}

// drainNode manually drains a node
func (s *Server) drainNode(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	nodeName := vars["name"]

	// Get the node
	node, err := s.client.CoreV1().Nodes().Get(r.Context(), nodeName, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			http.Error(w, "Node not found", http.StatusNotFound)
			return
		}
		s.logger.Error("Failed to get node", zap.String("node", nodeName), zap.Error(err))
		http.Error(w, "Failed to get node", http.StatusInternalServerError)
		return
	}

	// Check if node is already being drained
	if s.isNodeBeingDrained(node) {
		http.Error(w, "Node is already being drained", http.StatusConflict)
		return
	}

	// Perform drain operation
	if err := s.drainer.Drain(r.Context(), node); err != nil {
		s.logger.Error("Failed to drain node", zap.String("node", nodeName), zap.Error(err))
		http.Error(w, fmt.Sprintf("Failed to drain node: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"message": fmt.Sprintf("Successfully drained node %s", nodeName),
		"node":    nodeName,
	})
}

// cordonNode manually cordons a node
func (s *Server) cordonNode(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	nodeName := vars["name"]

	// Get the node
	node, err := s.client.CoreV1().Nodes().Get(r.Context(), nodeName, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			http.Error(w, "Node not found", http.StatusNotFound)
			return
		}
		s.logger.Error("Failed to get node", zap.String("node", nodeName), zap.Error(err))
		http.Error(w, "Failed to get node", http.StatusInternalServerError)
		return
	}

	// Perform cordon operation
	if err := s.drainer.Cordon(r.Context(), node); err != nil {
		s.logger.Error("Failed to cordon node", zap.String("node", nodeName), zap.Error(err))
		http.Error(w, fmt.Sprintf("Failed to cordon node: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"message": fmt.Sprintf("Successfully cordoned node %s", nodeName),
		"node":    nodeName,
	})
}

// uncordonNode manually uncordons a node
func (s *Server) uncordonNode(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	nodeName := vars["name"]

	// Get the node
	node, err := s.client.CoreV1().Nodes().Get(r.Context(), nodeName, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			http.Error(w, "Node not found", http.StatusNotFound)
			return
		}
		s.logger.Error("Failed to get node", zap.String("node", nodeName), zap.Error(err))
		http.Error(w, "Failed to get node", http.StatusInternalServerError)
		return
	}

	// Perform uncordon operation
	if err := s.drainer.Uncordon(r.Context(), node); err != nil {
		s.logger.Error("Failed to uncordon node", zap.String("node", nodeName), zap.Error(err))
		http.Error(w, fmt.Sprintf("Failed to uncordon node: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"message": fmt.Sprintf("Successfully uncordoned node %s", nodeName),
		"node":    nodeName,
	})
}

// isNodeBeingDrained checks if a node is currently being drained
func (s *Server) isNodeBeingDrained(node *corev1.Node) bool {
	if node.Annotations == nil {
		return false
	}
	_, exists := node.Annotations["draino2.kubernetes.io/drain-in-progress"]
	return exists
}

// loggingMiddleware adds request logging
func (s *Server) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		s.logger.Info("API request",
			zap.String("method", r.Method),
			zap.String("path", r.URL.Path),
			zap.String("remote_addr", r.RemoteAddr),
			zap.Duration("duration", time.Since(start)),
		)
	})
}

// corsMiddleware adds CORS headers
func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.config.API.CORS.Enabled {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}
