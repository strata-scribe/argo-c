package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	// WebhooksReceivedTotal tracks the total number of webhooks received.
	WebhooksReceivedTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "webhooks_received_total",
			Help: "Total number of webhooks received",
		},
	)

	// ReconciliationDurationSeconds tracks the duration of reconciliation operations.
	ReconciliationDurationSeconds = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "reconciliation_duration_seconds",
			Help:    "Duration of reconciliation in seconds",
			Buckets: prometheus.DefBuckets,
		},
	)

	// ReconciliationErrorsTotal tracks the total number of reconciliation errors.
	ReconciliationErrorsTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "reconciliation_errors_total",
			Help: "Total number of reconciliation errors",
		},
	)
)

func init() {
	prometheus.MustRegister(WebhooksReceivedTotal)
	prometheus.MustRegister(ReconciliationDurationSeconds)
	prometheus.MustRegister(ReconciliationErrorsTotal)
}
