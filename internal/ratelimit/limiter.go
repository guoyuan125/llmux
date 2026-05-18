package ratelimit

import (
	"fmt"
	"sync"
	"time"
)

// window holds fixed-window counters for one API key.
type window struct {
	mu          sync.Mutex
	reqCount    int
	tokenCount  int64
	windowStart time.Time
}

// resetIfExpired resets the window if the current minute has passed.
func (w *window) resetIfExpired() {
	if time.Since(w.windowStart) >= time.Minute {
		w.reqCount = 0
		w.tokenCount = 0
		w.windowStart = time.Now()
	}
}

// Limiter is a per-key fixed-window rate limiter.
type Limiter struct {
	mu      sync.RWMutex
	windows map[uint]*window
}

// Global is the process-wide rate limiter instance.
var Global = &Limiter{windows: make(map[uint]*window)}

func (l *Limiter) getOrCreate(keyID uint) *window {
	l.mu.RLock()
	w, ok := l.windows[keyID]
	l.mu.RUnlock()
	if ok {
		return w
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if w, ok = l.windows[keyID]; ok {
		return w
	}
	w = &window{windowStart: time.Now()}
	l.windows[keyID] = w
	return w
}

// Allow checks RPM and TPM limits and increments the request counter.
// rpm=0 and tpm=0 mean unlimited.
// Returns false and a reason string if the request should be rejected.
func (l *Limiter) Allow(keyID uint, rpm, tpm int) (bool, string) {
	w := l.getOrCreate(keyID)
	w.mu.Lock()
	defer w.mu.Unlock()

	w.resetIfExpired()

	if rpm > 0 && w.reqCount >= rpm {
		return false, fmt.Sprintf("rate limit exceeded: %d rpm", rpm)
	}
	if tpm > 0 && w.tokenCount >= int64(tpm) {
		return false, fmt.Sprintf("rate limit exceeded: %d tpm", tpm)
	}

	w.reqCount++
	return true, ""
}

// RecordTokens adds consumed tokens to the current window after a request completes.
func (l *Limiter) RecordTokens(keyID uint, tokens int64) {
	if tokens <= 0 {
		return
	}
	w := l.getOrCreate(keyID)
	w.mu.Lock()
	defer w.mu.Unlock()
	w.resetIfExpired()
	w.tokenCount += tokens
}
