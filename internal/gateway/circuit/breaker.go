package circuit

import (
	"fmt"
	"sync"
	"time"
)

// State represents circuit breaker state.
type State int

const (
	StateClosed   State = iota // normal, requests flow through
	StateOpen                  // tripped, requests are rejected
	StateHalfOpen              // testing, one request allowed
)

// Breaker is a per-channel circuit breaker.
type Breaker struct {
	mu           sync.Mutex
	state        State
	failures     int
	successes    int
	lastFailure  time.Time
	threshold    int           // failures before opening
	resetTimeout time.Duration // time in open state before half-open
}

// Config holds circuit breaker configuration.
type Config struct {
	Threshold    int           // consecutive failures to trip
	ResetTimeout time.Duration // how long to wait before half-open
}

var defaultConfig = Config{
	Threshold:    3,
	ResetTimeout: 30 * time.Second,
}

// Manager manages circuit breakers per channel+key combination.
type Manager struct {
	mu       sync.RWMutex
	breakers map[string]*Breaker
	cfg      Config
}

// NewManager creates a new circuit breaker manager.
func NewManager(cfg *Config) *Manager {
	c := defaultConfig
	if cfg != nil {
		c = *cfg
	}
	return &Manager{
		breakers: make(map[string]*Breaker),
		cfg:      c,
	}
}

// Allow checks if a request to the given channel is allowed.
func (m *Manager) Allow(key string) bool {
	m.mu.RLock()
	b, exists := m.breakers[key]
	m.mu.RUnlock()

	if !exists {
		return true
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	switch b.state {
	case StateClosed:
		return true
	case StateOpen:
		if time.Since(b.lastFailure) > b.resetTimeout {
			b.state = StateHalfOpen
			return true
		}
		return false
	case StateHalfOpen:
		return true
	}
	return true
}

// RecordSuccess records a successful request.
func (m *Manager) RecordSuccess(key string) {
	m.mu.RLock()
	b, exists := m.breakers[key]
	m.mu.RUnlock()

	if !exists {
		return
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	b.failures = 0
	b.successes++
	if b.state == StateHalfOpen {
		b.state = StateClosed
	}
}

// RecordFailure records a failed request.
func (m *Manager) RecordFailure(key string) {
	m.mu.Lock()
	b, exists := m.breakers[key]
	if !exists {
		b = &Breaker{
			threshold:    m.cfg.Threshold,
			resetTimeout: m.cfg.ResetTimeout,
		}
		m.breakers[key] = b
	}
	m.mu.Unlock()

	b.mu.Lock()
	defer b.mu.Unlock()

	b.failures++
	b.lastFailure = time.Now()
	if b.failures >= b.threshold {
		b.state = StateOpen
	}
}

// BreakerKey generates the map key for a channel+key combination.
func BreakerKey(channelID uint, keyID uint) string {
	return fmt.Sprintf("%d:%d", channelID, keyID)
}
