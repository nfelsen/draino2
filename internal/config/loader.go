package config

import (
	"fmt"
	"sync"

	"github.com/fsnotify/fsnotify"
	"github.com/nfelsen/draino2/internal/types"
	"github.com/spf13/viper"
	"k8s.io/client-go/rest"
	crconfig "sigs.k8s.io/controller-runtime/pkg/client/config"
)

var (
	config     types.Config
	configLock sync.RWMutex
)

// LoadConfig loads the configuration from file and environment variables
func LoadConfig(configFile string) error {
	v := viper.New()
	v.SetConfigFile(configFile)
	v.SetConfigType("yaml")
	v.AutomaticEnv()
	v.SetEnvPrefix("DRAINO2")

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			return fmt.Errorf("config file not found: %s", configFile)
		}
		return fmt.Errorf("error reading config: %w", err)
	}

	var c types.Config
	if err := v.Unmarshal(&c); err != nil {
		return fmt.Errorf("error unmarshaling config: %w", err)
	}

	configLock.Lock()
	config = c
	configLock.Unlock()

	return nil
}

// GetConfig returns a copy of the current config
func GetConfig() types.Config {
	configLock.RLock()
	defer configLock.RUnlock()
	return config
}

// WatchConfig watches the config file for changes and calls the callback on reload
func WatchConfig(configFile string, onChange func(types.Config)) error {
	v := viper.New()
	v.SetConfigFile(configFile)
	v.SetConfigType("yaml")
	v.AutomaticEnv()
	v.SetEnvPrefix("DRAINO2")

	if err := v.ReadInConfig(); err != nil {
		return err
	}

	v.WatchConfig()
	v.OnConfigChange(func(e fsnotify.Event) {
		var c types.Config
		if err := v.Unmarshal(&c); err == nil {
			configLock.Lock()
			config = c
			configLock.Unlock()
			onChange(c)
		}
	})
	return nil
}

// GetConfigOrDie returns a Kubernetes rest.Config for use with controller-runtime
func GetConfigOrDie() *rest.Config {
	return crconfig.GetConfigOrDie()
}
