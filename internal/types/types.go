package types

import (
	"time"

	corev1 "k8s.io/api/core/v1"
)

// LabelTrigger defines a label that can trigger a drain operation
type LabelTrigger struct {
	Key   string `json:"key" yaml:"key"`
	Value string `json:"value" yaml:"value"`
}

// NodeCondition defines a node condition that can trigger a drain operation
type NodeCondition struct {
	Type            corev1.NodeConditionType `json:"type" yaml:"type"`
	Status          corev1.ConditionStatus   `json:"status" yaml:"status"`
	MinimumDuration time.Duration            `json:"minimumDuration" yaml:"minimumDuration"`
}

// DrainSettings configures how drain operations are performed
type DrainSettings struct {
	MaxGracePeriod        time.Duration `json:"maxGracePeriod" yaml:"maxGracePeriod"`
	EvictionHeadroom      time.Duration `json:"evictionHeadroom" yaml:"evictionHeadroom"`
	DrainBuffer           time.Duration `json:"drainBuffer" yaml:"drainBuffer"`
	SkipCordon            bool          `json:"skipCordon" yaml:"skipCordon"`
	EvictDaemonSetPods    bool          `json:"evictDaemonSetPods" yaml:"evictDaemonSetPods"`
	EvictLocalStoragePods bool          `json:"evictLocalStoragePods" yaml:"evictLocalStoragePods"`
	EvictUnreplicatedPods bool          `json:"evictUnreplicatedPods" yaml:"evictUnreplicatedPods"`
}

// APIConfig configures the REST API
type APIConfig struct {
	Enabled bool `json:"enabled" yaml:"enabled"`
	Port    int  `json:"port" yaml:"port"`
	CORS    struct {
		Enabled        bool     `json:"enabled" yaml:"enabled"`
		AllowedOrigins []string `json:"allowedOrigins" yaml:"allowedOrigins"`
		AllowedMethods []string `json:"allowedMethods" yaml:"allowedMethods"`
		AllowedHeaders []string `json:"allowedHeaders" yaml:"allowedHeaders"`
	} `json:"cors" yaml:"cors"`
}

// MetricsConfig configures Prometheus metrics
type MetricsConfig struct {
	Enabled bool   `json:"enabled" yaml:"enabled"`
	Port    int    `json:"port" yaml:"port"`
	Path    string `json:"path" yaml:"path"`
}

// Config represents the main configuration for Draino2
type Config struct {
	LabelTriggers  []LabelTrigger  `json:"labelTriggers" yaml:"labelTriggers"`
	ExcludeLabels  []LabelTrigger  `json:"excludeLabels" yaml:"excludeLabels"`
	NodeConditions []NodeCondition `json:"nodeConditions" yaml:"nodeConditions"`
	DrainSettings  DrainSettings   `json:"drainSettings" yaml:"drainSettings"`
	API            APIConfig       `json:"api" yaml:"api"`
	Metrics        MetricsConfig   `json:"metrics" yaml:"metrics"`
	DryRun         bool            `json:"dryRun" yaml:"dryRun"`
}
