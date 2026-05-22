package session

import (
	"fmt"
	"sync"
	"time"
)

// Entry holds session stickiness data.
type Entry struct {
	ChannelID uint
	KeyID     uint
	ExpiresAt time.Time
}

// Store manages session stickiness for API key + model combinations.
type Store struct {
	mu      sync.RWMutex
	entries map[string]*Entry
}

// NewStore creates a new session store.
func NewStore() *Store {
	s := &Store{entries: make(map[string]*Entry)}
	go s.cleanup()
	return s
}

// Get returns the sticky channel for the given API key and model.
func (s *Store) Get(apiKeyID uint, model string) (channelID uint, keyID uint, ok bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	key := sessionKey(apiKeyID, model)
	entry, exists := s.entries[key]
	if !exists || time.Now().After(entry.ExpiresAt) {
		return 0, 0, false
	}
	return entry.ChannelID, entry.KeyID, true
}

// Set stores session stickiness.
func (s *Store) Set(apiKeyID uint, model string, channelID, keyID uint, ttl time.Duration) {
	if ttl <= 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	key := sessionKey(apiKeyID, model)
	s.entries[key] = &Entry{
		ChannelID: channelID,
		KeyID:     keyID,
		ExpiresAt: time.Now().Add(ttl),
	}
}

// Delete removes a session entry for the given API key and model.
func (s *Store) Delete(apiKeyID uint, model string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.entries, sessionKey(apiKeyID, model))
}

func sessionKey(apiKeyID uint, model string) string {
	return fmt.Sprintf("%d:%s", apiKeyID, model)
}

func (s *Store) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	for range ticker.C {
		s.mu.Lock()
		now := time.Now()
		for k, v := range s.entries {
			if now.After(v.ExpiresAt) {
				delete(s.entries, k)
			}
		}
		s.mu.Unlock()
	}
}
