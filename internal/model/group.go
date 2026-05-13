package model

// Group aggregates multiple channels under a unified model name.
type Group struct {
	ID                uint        `json:"id" gorm:"primaryKey"`
	Name              string      `json:"name" gorm:"uniqueIndex;not null"` // exposed model name
	Mode              GroupMode   `json:"mode" gorm:"not null"`
	MatchRegex        string      `json:"match_regex"`         // regex for auto-matching model requests
	FirstTokenTimeout int         `json:"first_token_timeout"` // seconds, 0 = disabled
	SessionKeepTime   int         `json:"session_keep_time"`   // seconds, 0 = disabled
	Items             []GroupItem `json:"items,omitempty" gorm:"foreignKey:GroupID"`
}

// GroupMode defines load balancing strategy.
type GroupMode int

const (
	GroupModeRoundRobin  GroupMode = 1
	GroupModeRandom      GroupMode = 2
	GroupModeFailover    GroupMode = 3
	GroupModeWeighted    GroupMode = 4
	GroupModeLeastCost   GroupMode = 5 // prefer cheapest channel
	GroupModeLeastLatency GroupMode = 6 // prefer lowest latency channel
)

// GroupItem links a channel to a group with routing metadata.
type GroupItem struct {
	ID        uint   `json:"id" gorm:"primaryKey"`
	GroupID   uint   `json:"group_id" gorm:"uniqueIndex:idx_group_channel_model"`
	ChannelID uint   `json:"channel_id" gorm:"uniqueIndex:idx_group_channel_model"`
	ModelName string `json:"model_name" gorm:"uniqueIndex:idx_group_channel_model"` // actual model name to send upstream
	Priority  int    `json:"priority"`
	Weight    int    `json:"weight"`
}
