package handler

import (
	"encoding/json"
	"sync"

	"github.com/liuguoyuan/llmux/internal/model"
)

// LogHub broadcasts audit log events to all connected SSE clients.
type LogHub struct {
	mu      sync.RWMutex
	clients map[chan []byte]struct{}
}

var globalLogHub = &LogHub{
	clients: make(map[chan []byte]struct{}),
}

// GetLogHub returns the global log hub instance.
func GetLogHub() *LogHub {
	return globalLogHub
}

// Subscribe adds a new client channel.
func (h *LogHub) Subscribe() chan []byte {
	ch := make(chan []byte, 16)
	h.mu.Lock()
	h.clients[ch] = struct{}{}
	h.mu.Unlock()
	return ch
}

// Unsubscribe removes a client channel.
func (h *LogHub) Unsubscribe(ch chan []byte) {
	h.mu.Lock()
	delete(h.clients, ch)
	close(ch)
	h.mu.Unlock()
}

// Publish sends an audit log to all connected clients.
func (h *LogHub) Publish(log *model.AuditLog) {
	data, err := json.Marshal(log)
	if err != nil {
		return
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	for ch := range h.clients {
		select {
		case ch <- data:
		default:
			// drop if client is slow
		}
	}
}
