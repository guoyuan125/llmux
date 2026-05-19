package model

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Channel represents an upstream LLM provider connection.
type Channel struct {
	ID            uint           `json:"id" gorm:"primaryKey"`
	Name          string         `json:"name" gorm:"uniqueIndex;not null"`
	Type          ChannelType    `json:"type" gorm:"not null"`
	Enabled       bool           `json:"enabled" gorm:"default:true"`
	BaseURLs      []ChannelURL   `json:"base_urls" gorm:"foreignKey:ChannelID"`
	Keys          []ChannelKey   `json:"keys" gorm:"foreignKey:ChannelID"`
	Models        string         `json:"models"`        // comma-separated available models
	CustomModels  string         `json:"custom_models"` // user-defined model list
	AutoSync      bool           `json:"auto_sync" gorm:"default:false"`
	Proxy         string         `json:"proxy"`          // HTTP proxy for this channel
	ParamOverride string         `json:"param_override"` // JSON string to merge into request body
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
}

// ChannelType identifies the upstream API protocol.
type ChannelType int

const (
	ChannelTypeOpenAI    ChannelType = 1
	ChannelTypeAnthropic ChannelType = 2
	ChannelTypeGemini    ChannelType = 3
)

var channelTypeNames = map[ChannelType]string{
	ChannelTypeOpenAI:    "openai",
	ChannelTypeAnthropic: "anthropic",
	ChannelTypeGemini:    "gemini",
}

var channelTypeByName = map[string]ChannelType{
	"openai":    ChannelTypeOpenAI,
	"anthropic": ChannelTypeAnthropic,
	"gemini":    ChannelTypeGemini,
}

func (t ChannelType) MarshalJSON() ([]byte, error) {
	if name, ok := channelTypeNames[t]; ok {
		return []byte(`"` + name + `"`), nil
	}
	return []byte(strconv.Itoa(int(t))), nil
}

func (t *ChannelType) UnmarshalJSON(data []byte) error {
	s := strings.Trim(string(data), `"`)
	if ct, ok := channelTypeByName[s]; ok {
		*t = ct
		return nil
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return fmt.Errorf("unknown channel type: %s", s)
	}
	*t = ChannelType(n)
	return nil
}

// ChannelURL is a base URL endpoint with measured latency.
type ChannelURL struct {
	ID        uint   `json:"id" gorm:"primaryKey"`
	ChannelID uint   `json:"channel_id" gorm:"index"`
	URL       string `json:"url" gorm:"not null"`
	Latency   int    `json:"latency"` // measured latency in ms, 0 = unknown
}

// ChannelKey holds a single API key for a channel.
type ChannelKey struct {
	ID          uint    `json:"id" gorm:"primaryKey"`
	ChannelID   uint    `json:"channel_id" gorm:"index"`
	Key         string  `json:"key" gorm:"not null"`
	Enabled     bool    `json:"enabled" gorm:"default:true"`
	Remark      string  `json:"remark"`
	StatusCode  int     `json:"status_code"`         // last HTTP status from upstream
	LastUsedAt  int64   `json:"last_used_at"`        // unix timestamp
	TotalCost   float64 `json:"total_cost"`          // accumulated cost in USD
}
