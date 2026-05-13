package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// RequestsTotal counts total relay requests.
	RequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "llmux",
		Name:      "requests_total",
		Help:      "Total number of relay requests",
	}, []string{"model", "channel", "status"})

	// RequestDuration measures request latency.
	RequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "llmux",
		Name:      "request_duration_seconds",
		Help:      "Request duration in seconds",
		Buckets:   prometheus.ExponentialBuckets(0.1, 2, 10),
	}, []string{"model", "channel"})

	// TokensTotal counts token usage.
	TokensTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "llmux",
		Name:      "tokens_total",
		Help:      "Total tokens consumed",
	}, []string{"model", "direction"}) // direction: input, output

	// CostTotal tracks accumulated cost.
	CostTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "llmux",
		Name:      "cost_usd_total",
		Help:      "Total cost in USD",
	}, []string{"model", "channel"})

	// ActiveConnections tracks concurrent streaming connections.
	ActiveConnections = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "llmux",
		Name:      "active_connections",
		Help:      "Number of active streaming connections",
	})

	// CircuitBreakerState shows current breaker states.
	CircuitBreakerState = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "llmux",
		Name:      "circuit_breaker_state",
		Help:      "Circuit breaker state (0=closed, 1=open, 2=half-open)",
	}, []string{"channel"})

	// FirstTokenLatency measures time to first token in streaming.
	FirstTokenLatency = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "llmux",
		Name:      "first_token_seconds",
		Help:      "Time to first token in seconds",
		Buckets:   prometheus.ExponentialBuckets(0.1, 2, 8),
	}, []string{"model", "channel"})
)
