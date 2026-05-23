package model

import "time"

// Stats holds aggregated metrics.
type Stats struct {
	InputTokens    int64   `json:"input_tokens" gorm:"default:0"`
	OutputTokens   int64   `json:"output_tokens" gorm:"default:0"`
	InputCost      float64 `json:"input_cost" gorm:"default:0"`
	OutputCost     float64 `json:"output_cost" gorm:"default:0"`
	TotalRequests  int64   `json:"total_requests" gorm:"default:0"`
	FailedRequests int64   `json:"failed_requests" gorm:"default:0"`
	TotalLatencyMs int64   `json:"total_latency_ms" gorm:"default:0"`
}

// StatsDaily stores per-day aggregation.
type StatsDaily struct {
	Date string `json:"date" gorm:"primaryKey"` // format: 2006-01-02
	Stats
}

// StatsHourly stores per-hour aggregation (rolling 24h).
type StatsHourly struct {
	Hour int    `json:"hour" gorm:"primaryKey"`
	Date string `json:"date" gorm:"not null"`
	Stats
}

// StatsModel stores per-model aggregation.
type StatsModel struct {
	ID        uint   `json:"id" gorm:"primaryKey"`
	ModelName string `json:"model_name" gorm:"uniqueIndex:idx_stats_model_channel"`
	ChannelID uint   `json:"channel_id" gorm:"uniqueIndex:idx_stats_model_channel"`
	Stats
}

// StatsChannel stores per-channel aggregation.
type StatsChannel struct {
	ChannelID uint `json:"channel_id" gorm:"primaryKey"`
	Stats
}

// StatsAPIKey stores per-API-key aggregation.
type StatsAPIKey struct {
	APIKeyID uint `json:"api_key_id" gorm:"primaryKey"`
	Stats
}

// AuditLog records a single request for traceability.
type AuditLog struct {
	ID            uint      `json:"id" gorm:"primaryKey"`
	RequestID     string    `json:"request_id" gorm:"index"`
	APIKeyID      uint      `json:"api_key_id" gorm:"index"`
	Model         string    `json:"model" gorm:"index"`          // client-requested model name
	GroupName     string    `json:"group_name" gorm:"index"`     // matched group name
	UpstreamModel string    `json:"upstream_model"`              // actual model sent to upstream
	ChannelID     uint      `json:"channel_id"`
	ChannelName   string    `json:"channel_name"`
	StatusCode    int       `json:"status_code"`
	InputTokens   int64     `json:"input_tokens"`
	OutputTokens  int64     `json:"output_tokens"`
	Cost          float64   `json:"cost"`
	LatencyMs     int64     `json:"latency_ms"`
	FirstTokenMs  int64     `json:"first_token_ms"`
	Stream        bool      `json:"stream"`
	Error         string    `json:"error"`
	RequestBody   string    `json:"request_body,omitempty" gorm:"type:text"`  // stored only on error
	ResponseBody  string    `json:"response_body,omitempty" gorm:"type:text"` // stored only on error
	Attempts      int       `json:"attempts"`
	CreatedAt     time.Time `json:"created_at" gorm:"index"`
}

// ModelPrice stores pricing information per model.
type ModelPrice struct {
	ID          uint    `json:"id" gorm:"primaryKey"`
	ModelName   string  `json:"model_name" gorm:"uniqueIndex"`
	InputPrice  float64 `json:"input_price"`  // per million tokens
	OutputPrice float64 `json:"output_price"` // per million tokens
	Source      string  `json:"source"`       // e.g. "models.dev", "manual"
	UpdatedAt   int64   `json:"updated_at"`
}
