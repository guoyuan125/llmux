package relay

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/liuguoyuan/llmux/internal/gateway/balancer"
	"github.com/liuguoyuan/llmux/internal/gateway/circuit"
	"github.com/liuguoyuan/llmux/internal/gateway/session"
	"github.com/liuguoyuan/llmux/internal/model"
	"gorm.io/gorm"
)

// Gateway is the core relay engine that routes requests through channels.
type Gateway struct {
	db       *gorm.DB
	circuit  *circuit.Manager
	sessions *session.Store
}

// NewGateway creates a new relay gateway.
func NewGateway(db *gorm.DB) *Gateway {
	return &Gateway{
		db:       db,
		circuit:  circuit.NewManager(nil),
		sessions: session.NewStore(),
	}
}

// RelayRequest holds all context for a single relay operation.
type RelayRequest struct {
	Context    context.Context
	APIKeyID   uint
	Model      string
	Body       []byte
	Stream     bool
	Writer     http.ResponseWriter
}

// RelayResult holds the outcome of a relay attempt.
type RelayResult struct {
	Success      bool
	ChannelID    uint
	ChannelName  string
	StatusCode   int
	InputTokens  int64
	OutputTokens int64
	Cost         float64
	LatencyMs    int64
	FirstTokenMs int64
	Attempts     int
	Error        error
}

// Relay executes the full relay flow: find group -> balance -> try channels -> return result.
func (g *Gateway) Relay(req *RelayRequest) *RelayResult {
	result := &RelayResult{}

	// Find the group for this model
	group, err := g.findGroup(req.Model)
	if err != nil {
		result.Error = fmt.Errorf("model not found: %s", req.Model)
		return result
	}

	// Get candidate channels via balancer
	b := balancer.Get(group.Mode)
	candidates := b.Candidates(group.Items)
	if len(candidates) == 0 {
		result.Error = fmt.Errorf("no available channels for model: %s", req.Model)
		return result
	}

	// Check session stickiness
	if group.SessionKeepTime > 0 {
		if chID, _, ok := g.sessions.Get(req.APIKeyID, req.Model); ok {
			// Move sticky channel to front
			candidates = moveToFront(candidates, chID)
		}
	}

	start := time.Now()

	// Try each candidate
	for i, item := range candidates {
		result.Attempts = i + 1

		// Circuit breaker check
		cbKey := circuit.BreakerKey(item.ChannelID, 0)
		if !g.circuit.Allow(cbKey) {
			continue
		}

		// Get channel details
		var channel model.Channel
		if err := g.db.Preload("BaseURLs").Preload("Keys").First(&channel, item.ChannelID).Error; err != nil {
			continue
		}
		if !channel.Enabled {
			continue
		}

		// Try this channel
		err := g.tryChannel(req, &channel, item.ModelName)
		if err == nil {
			result.Success = true
			result.ChannelID = channel.ID
			result.ChannelName = channel.Name
			result.LatencyMs = time.Since(start).Milliseconds()
			g.circuit.RecordSuccess(cbKey)

			// Update session stickiness
			if group.SessionKeepTime > 0 {
				g.sessions.Set(req.APIKeyID, req.Model, channel.ID, 0, time.Duration(group.SessionKeepTime)*time.Second)
			}
			return result
		}

		// Record failure
		g.circuit.RecordFailure(cbKey)
		result.Error = err
	}

	result.LatencyMs = time.Since(start).Milliseconds()
	if result.Error == nil {
		result.Error = fmt.Errorf("all channels failed for model: %s", req.Model)
	}
	return result
}

func (g *Gateway) findGroup(modelName string) (*model.Group, error) {
	var group model.Group
	err := g.db.Preload("Items").Where("name = ?", modelName).First(&group).Error
	if err != nil {
		// Try regex match
		var groups []model.Group
		g.db.Preload("Items").Where("match_regex != ''").Find(&groups)
		for _, grp := range groups {
			// TODO: regex matching
			_ = grp
		}
		return nil, fmt.Errorf("no group found for model: %s", modelName)
	}
	return &group, nil
}

func (g *Gateway) tryChannel(req *RelayRequest, channel *model.Channel, upstreamModel string) error {
	// TODO: implement full relay logic
	// 1. Select best URL and key
	// 2. Build outbound request via transformer
	// 3. Send request
	// 4. Handle response (streaming or non-streaming)
	// 5. Write response to client
	_ = req
	_ = channel
	_ = upstreamModel
	return fmt.Errorf("not implemented")
}

func moveToFront(items []model.GroupItem, channelID uint) []model.GroupItem {
	for i, item := range items {
		if item.ChannelID == channelID {
			result := make([]model.GroupItem, len(items))
			result[0] = item
			copy(result[1:], items[:i])
			copy(result[i+1:], items[i+1:])
			return result
		}
	}
	return items
}

// readResponse is a helper to read response body (for non-streaming).
func readResponse(resp *http.Response) ([]byte, error) {
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}
