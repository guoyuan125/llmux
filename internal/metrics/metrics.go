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
	}, []string{"model", "channel", "direction"}) // direction: input, output

	// CacheTokensTotal counts prompt cache usage reported by upstream providers.
	CacheTokensTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "llmux",
		Name:      "cache_tokens_total",
		Help:      "Total prompt cache tokens reported by upstream providers",
	}, []string{"model", "channel", "direction"}) // direction: read, write

	// CacheRequestsTotal counts whether a completed request hit the prompt cache.
	CacheRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "llmux",
		Name:      "cache_requests_total",
		Help:      "Completed requests partitioned by prompt cache hit status",
	}, []string{"model", "channel", "result"}) // result: hit, miss

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

	// FirstByteLatency measures time until upstream response headers are available.
	FirstByteLatency = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "llmux",
		Name:      "first_byte_seconds",
		Help:      "Time to first upstream response byte/header in seconds",
		Buckets:   prometheus.ExponentialBuckets(0.05, 2, 10),
	}, []string{"model", "channel", "status"})

	// ChannelAvailability reports the last health probe result for each channel URL.
	ChannelAvailability = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "llmux",
		Name:      "channel_availability",
		Help:      "Last channel URL health result (1=available, 0=unavailable)",
	}, []string{"channel", "url"})

	// ChannelHealthLatency reports the latest TCP probe latency per channel URL.
	ChannelHealthLatency = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "llmux",
		Name:      "channel_health_latency_seconds",
		Help:      "Latest channel URL TCP health probe latency in seconds",
	}, []string{"channel", "url"})

	// ModelAvailability reports configured model availability on enabled channels.
	ModelAvailability = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "llmux",
		Name:      "model_availability",
		Help:      "Configured model availability by channel (1=available, 0=unavailable)",
	}, []string{"model", "channel"})
)
