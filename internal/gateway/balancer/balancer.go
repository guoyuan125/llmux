package balancer

import (
	"math/rand"
	"sort"
	"sync/atomic"

	"github.com/liuguoyuan/llmux/internal/model"
)

var roundRobinCounter uint64

// Balancer returns a sorted candidate list based on load balancing strategy.
type Balancer interface {
	Candidates(items []model.GroupItem) []model.GroupItem
}

// Get returns the appropriate balancer for the given mode.
func Get(mode model.GroupMode) Balancer {
	switch mode {
	case model.GroupModeRoundRobin:
		return &RoundRobin{}
	case model.GroupModeRandom:
		return &Random{}
	case model.GroupModeFailover:
		return &Failover{}
	case model.GroupModeWeighted:
		return &Weighted{}
	case model.GroupModeLeastCost:
		return &LeastCost{}
	case model.GroupModeLeastLatency:
		return &LeastLatency{}
	default:
		return &RoundRobin{}
	}
}

// RoundRobin cycles through channels sequentially.
type RoundRobin struct{}

func (b *RoundRobin) Candidates(items []model.GroupItem) []model.GroupItem {
	n := len(items)
	if n == 0 {
		return nil
	}
	idx := int(atomic.AddUint64(&roundRobinCounter, 1) % uint64(n))
	result := make([]model.GroupItem, n)
	for i := 0; i < n; i++ {
		result[i] = items[(idx+i)%n]
	}
	return result
}

// Random shuffles all candidates randomly.
type Random struct{}

func (b *Random) Candidates(items []model.GroupItem) []model.GroupItem {
	n := len(items)
	if n == 0 {
		return nil
	}
	result := make([]model.GroupItem, n)
	copy(result, items)
	rand.Shuffle(n, func(i, j int) { result[i], result[j] = result[j], result[i] })
	return result
}

// Failover sorts by priority (lowest first).
type Failover struct{}

func (b *Failover) Candidates(items []model.GroupItem) []model.GroupItem {
	if len(items) == 0 {
		return nil
	}
	result := make([]model.GroupItem, len(items))
	copy(result, items)
	sort.Slice(result, func(i, j int) bool { return result[i].Priority < result[j].Priority })
	return result
}

// Weighted uses weighted random selection.
type Weighted struct{}

func (b *Weighted) Candidates(items []model.GroupItem) []model.GroupItem {
	n := len(items)
	if n == 0 {
		return nil
	}
	type scored struct {
		item  model.GroupItem
		score float64
	}
	totalWeight := 0
	for _, item := range items {
		w := item.Weight
		if w <= 0 {
			w = 1
		}
		totalWeight += w
	}
	s := make([]scored, n)
	for i, item := range items {
		w := item.Weight
		if w <= 0 {
			w = 1
		}
		s[i] = scored{item: item, score: rand.Float64() * float64(w) / float64(totalWeight)}
	}
	sort.Slice(s, func(i, j int) bool { return s[i].score > s[j].score })
	result := make([]model.GroupItem, n)
	for i := range s {
		result[i] = s[i].item
	}
	return result
}

// LeastCost sorts by channel accumulated cost (lowest first).
// Requires runtime cost data injection via SetCostData.
type LeastCost struct{}

func (b *LeastCost) Candidates(items []model.GroupItem) []model.GroupItem {
	// In v1, falls back to priority-based sorting.
	// Cost data will be injected at runtime from StatsChannel.
	if len(items) == 0 {
		return nil
	}
	result := make([]model.GroupItem, len(items))
	copy(result, items)
	sort.Slice(result, func(i, j int) bool { return result[i].Priority < result[j].Priority })
	return result
}

// LeastLatency sorts by measured endpoint latency (lowest first).
type LeastLatency struct{}

func (b *LeastLatency) Candidates(items []model.GroupItem) []model.GroupItem {
	// In v1, falls back to priority-based sorting.
	// Latency data will be injected at runtime from ChannelURL.Latency.
	if len(items) == 0 {
		return nil
	}
	result := make([]model.GroupItem, len(items))
	copy(result, items)
	sort.Slice(result, func(i, j int) bool { return result[i].Priority < result[j].Priority })
	return result
}
