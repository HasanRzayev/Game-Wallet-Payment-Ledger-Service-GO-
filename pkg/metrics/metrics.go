package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	HTTPRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "game_wallet",
			Subsystem: "http",
			Name:      "requests_total",
			Help:      "Total number of HTTP requests processed, partitioned by status code, method, and path.",
		},
		[]string{"method", "path", "status"},
	)

	HTTPRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "game_wallet",
			Subsystem: "http",
			Name:      "request_duration_seconds",
			Help:      "Histogram of HTTP request latencies.",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"method", "path"},
	)

	TransactionsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "game_wallet",
			Subsystem: "ledger",
			Name:      "transactions_total",
			Help:      "Total count of financial transactions processed, by type and status.",
		},
		[]string{"type", "status"},
	)

	TransactionDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "game_wallet",
			Subsystem: "ledger",
			Name:      "transaction_duration_seconds",
			Help:      "Duration of ledger transactions in seconds.",
			Buckets:   []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5},
		},
		[]string{"type"},
	)

	EventsProcessedTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "game_wallet",
			Subsystem: "broker",
			Name:      "events_processed_total",
			Help:      "Total number of broker events processed by the background worker.",
		},
		[]string{"event_type", "status"},
	)
)
