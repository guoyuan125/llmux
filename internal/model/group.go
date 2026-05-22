package model

import (
	"fmt"
	"strconv"
	"strings"
)

// Group aggregates multiple channels under a unified model name.
type Group struct {
	ID                uint        `json:"id" gorm:"primaryKey"`
	Name              string      `json:"name" gorm:"uniqueIndex;not null"` // display name for management
	Models            string      `json:"models"`                           // comma-separated exact model names accepted by this group, e.g. "internal,gpt-4o"
	Mode              GroupMode   `json:"mode" gorm:"not null"`
	ContextSize       int         `json:"context_size"`          // max context window in tokens, reported via /v1/models
	FirstTokenTimeout int         `json:"first_token_timeout"`   // seconds, 0 = disabled
	SessionKeepTime   int         `json:"session_keep_time"`     // seconds, 0 = disabled
	Items             []GroupItem `json:"items,omitempty" gorm:"foreignKey:GroupID"`
}

// GroupMode defines load balancing strategy.
type GroupMode int

const (
	GroupModeRoundRobin   GroupMode = 1
	GroupModeRandom       GroupMode = 2
	GroupModeFailover     GroupMode = 3
	GroupModeWeighted     GroupMode = 4
	GroupModeLeastCost    GroupMode = 5 // prefer cheapest channel
	GroupModeLeastLatency GroupMode = 6 // prefer lowest latency channel
)

var groupModeNames = map[GroupMode]string{
	GroupModeRoundRobin:   "round_robin",
	GroupModeRandom:       "random",
	GroupModeFailover:     "failover",
	GroupModeWeighted:     "weighted",
	GroupModeLeastCost:    "least_cost",
	GroupModeLeastLatency: "least_latency",
}

var groupModeByName = map[string]GroupMode{
	"round_robin":   GroupModeRoundRobin,
	"random":        GroupModeRandom,
	"failover":      GroupModeFailover,
	"weighted":      GroupModeWeighted,
	"least_cost":    GroupModeLeastCost,
	"least_latency": GroupModeLeastLatency,
}

func (m GroupMode) MarshalJSON() ([]byte, error) {
	if name, ok := groupModeNames[m]; ok {
		return []byte(`"` + name + `"`), nil
	}
	return []byte(strconv.Itoa(int(m))), nil
}

func (m *GroupMode) UnmarshalJSON(data []byte) error {
	s := strings.Trim(string(data), `"`)
	if gm, ok := groupModeByName[s]; ok {
		*m = gm
		return nil
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return fmt.Errorf("unknown group mode: %s", s)
	}
	*m = GroupMode(n)
	return nil
}

// GroupItem links a channel to a group with routing metadata.
type GroupItem struct {
	ID        uint   `json:"id" gorm:"primaryKey"`
	GroupID   uint   `json:"group_id" gorm:"uniqueIndex:idx_group_channel_model"`
	ChannelID uint   `json:"channel_id" gorm:"uniqueIndex:idx_group_channel_model"`
	ModelName string `json:"model_name" gorm:"uniqueIndex:idx_group_channel_model"` // actual model name to send upstream
	Priority  int    `json:"priority"`
	Weight    int    `json:"weight"`

	// Runtime-only fields, not persisted. Populated by gateway before balancer call.
	RuntimeLatencyMs int64   `json:"-" gorm:"-"` // measured TCP latency in ms, 0 = unknown
	RuntimeCostTotal float64 `json:"-" gorm:"-"` // accumulated channel cost in USD
}
