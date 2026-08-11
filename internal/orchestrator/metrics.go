//go:build linux

package orchestrator

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Prometheus metrics for orchestrator operations

var (
	// FaaS metrics
	FaaSInvokeCount = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "dck_faas_invoke_total",
			Help: "Total number of function invocations",
		},
		[]string{"function"},
	)

	FaaSInvokeErrors = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "dck_faas_invoke_errors_total",
			Help: "Total number of function invocation errors",
		},
		[]string{"function", "reason"},
	)

	FaaSInvokeDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "dck_faas_invoke_duration_seconds",
			Help:    "Duration of function invocations in seconds",
			Buckets: prometheus.ExponentialBuckets(0.01, 2, 10),
		},
		[]string{"function"},
	)

	// Scale up/down metrics
	ScaleUpTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "dck_scale_up_total",
			Help: "Total number of scale up operations",
		},
		[]string{"target", "type"}, // type: function, service
	)

	ScaleUpErrors = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "dck_scale_up_errors_total",
			Help: "Total number of scale up errors",
		},
		[]string{"target", "type"},
	)

	ScaleUpDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "dck_scale_up_duration_seconds",
			Help:    "Duration of scale up operations in seconds",
			Buckets: prometheus.ExponentialBuckets(0.1, 2, 10),
		},
		[]string{"target", "type"},
	)

	ScaleDownTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "dck_scale_down_total",
			Help: "Total number of scale down operations",
		},
		[]string{"target", "type"},
	)

	ScaleDownErrors = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "dck_scale_down_errors_total",
			Help: "Total number of scale down errors",
		},
		[]string{"target", "type"},
	)

	ScaleDownDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "dck_scale_down_duration_seconds",
			Help:    "Duration of scale down operations in seconds",
			Buckets: prometheus.ExponentialBuckets(0.1, 2, 10),
		},
		[]string{"target", "type"},
	)

	// Active containers gauge
	ActiveContainers = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "dck_active_containers",
			Help: "Number of active containers",
		},
		[]string{"type"}, // function, service
	)

	// Container operations
	ContainerCreatedTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "dck_container_created_total",
			Help: "Total number of containers created",
		},
		[]string{"type"},
	)

	ContainerRemovedTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "dck_container_removed_total",
			Help: "Total number of containers removed",
		},
		[]string{"type"},
	)

	// Scheduler metrics
	ScheduleReplicaTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "dck_schedule_replica_total",
			Help: "Total number of replica scheduling attempts",
		},
		[]string{"service", "target_node"},
	)

	ScheduleReplicaErrors = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "dck_schedule_replica_errors_total",
			Help: "Total number of replica scheduling errors",
		},
		[]string{"service", "reason"},
	)

	// Auto-heal metrics
	AutoHealTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "dck_auto_heal_total",
			Help: "Total number of auto-heal cycles",
		},
	)

	AutoHealReplicasCreated = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "dck_auto_heal_replicas_created_total",
			Help: "Total number of replicas created by auto-healer",
		},
	)

	// Rolling update metrics
	RollingUpdateTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "dck_rolling_update_total",
			Help: "Total number of rolling update operations",
		},
		[]string{"service"},
	)

	RollingUpdateErrors = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "dck_rolling_update_errors_total",
			Help: "Total number of rolling update errors",
		},
		[]string{"service"},
	)
)

// RecordScaleUp records a scale up event with duration
func RecordScaleUp(target, typ string, duration float64, err error) {
	ScaleUpTotal.WithLabelValues(target, typ).Inc()
	ScaleUpDuration.WithLabelValues(target, typ).Observe(duration)
	if err != nil {
		ScaleUpErrors.WithLabelValues(target, typ).Inc()
	}
}

// RecordScaleDown records a scale down event with duration
func RecordScaleDown(target, typ string, duration float64, err error) {
	ScaleDownTotal.WithLabelValues(target, typ).Inc()
	ScaleDownDuration.WithLabelValues(target, typ).Observe(duration)
	if err != nil {
		ScaleDownErrors.WithLabelValues(target, typ).Inc()
	}
}

// UpdateActiveContainers updates the active containers gauge
func UpdateActiveContainers(typ string, count float64) {
	ActiveContainers.WithLabelValues(typ).Set(count)
}
