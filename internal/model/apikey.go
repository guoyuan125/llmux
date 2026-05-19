package model

// APIKey is a client-facing access key.
type APIKey struct {
	ID              uint    `json:"id" gorm:"primaryKey"`
	Name            string  `json:"name" gorm:"not null"`
	Key             string  `json:"key" gorm:"uniqueIndex;not null"`
	Enabled         bool    `json:"enabled" gorm:"default:true"`
	ExpireAt        int64   `json:"expire_at"`        // unix timestamp, 0 = never
	MaxCost         float64 `json:"max_cost"`         // max total cost in USD, 0 = unlimited
	RPM             int     `json:"rpm_limit"`        // requests per minute, 0 = unlimited
	TPM             int     `json:"tpm_limit"`        // tokens per minute, 0 = unlimited
	SupportedModels string  `json:"supported_models"` // comma-separated model whitelist, empty = all
}
