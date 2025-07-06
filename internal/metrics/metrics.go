package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Metrics holds all Prometheus metrics for draino2
type Metrics struct {
	// DrainOperationsStarted tracks the number of drain operations started
	DrainOperationsStarted prometheus.Counter
	// DrainOperationsCompleted tracks the number of drain operations completed successfully
	DrainOperationsCompleted prometheus.Counter
	// DrainOperationsFailed tracks the number of drain operations that failed
	DrainOperationsFailed prometheus.Counter
	// DrainDuration tracks the duration of drain operations
	DrainDuration prometheus.Histogram
	// PodsEvicted tracks the number of pods evicted during drains
	PodsEvicted prometheus.Counter
	// PodsFailedToEvict tracks the number of pods that failed to evict
	PodsFailedToEvict prometheus.Counter
	// NodesCordoned tracks the number of nodes cordoned
	NodesCordoned prometheus.Counter
	// NodesUncordoned tracks the number of nodes uncordoned
	NodesUncordoned prometheus.Counter
	// ActiveDrainOperations tracks the number of currently active drain operations
	ActiveDrainOperations prometheus.Gauge
}

// NewMetrics creates a new metrics instance
func NewMetrics() *Metrics {
	return &Metrics{
		DrainOperationsStarted: promauto.NewCounter(prometheus.CounterOpts{
			Name: "draino2_drain_operations_started_total",
			Help: "Total number of drain operations started",
		}),
		DrainOperationsCompleted: promauto.NewCounter(prometheus.CounterOpts{
			Name: "draino2_drain_operations_completed_total",
			Help: "Total number of drain operations completed successfully",
		}),
		DrainOperationsFailed: promauto.NewCounter(prometheus.CounterOpts{
			Name: "draino2_drain_operations_failed_total",
			Help: "Total number of drain operations that failed",
		}),
		DrainDuration: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:    "draino2_drain_duration_seconds",
			Help:    "Duration of drain operations in seconds",
			Buckets: prometheus.DefBuckets,
		}),
		PodsEvicted: promauto.NewCounter(prometheus.CounterOpts{
			Name: "draino2_pods_evicted_total",
			Help: "Total number of pods evicted during drain operations",
		}),
		PodsFailedToEvict: promauto.NewCounter(prometheus.CounterOpts{
			Name: "draino2_pods_failed_to_evict_total",
			Help: "Total number of pods that failed to evict during drain operations",
		}),
		NodesCordoned: promauto.NewCounter(prometheus.CounterOpts{
			Name: "draino2_nodes_cordoned_total",
			Help: "Total number of nodes cordoned",
		}),
		NodesUncordoned: promauto.NewCounter(prometheus.CounterOpts{
			Name: "draino2_nodes_uncordoned_total",
			Help: "Total number of nodes uncordoned",
		}),
		ActiveDrainOperations: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "draino2_active_drain_operations",
			Help: "Number of currently active drain operations",
		}),
	}
}
