package handler

import (
	"fmt"
	"io"
	"time"

	"github.com/gin-gonic/gin"
)

// StreamLogs serves an SSE stream of real-time audit log events.
func (h *Handler) StreamLogs(c *gin.Context) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Access-Control-Allow-Origin", "*")
	c.Header("X-Accel-Buffering", "no")

	hub := GetLogHub()
	ch := hub.Subscribe()
	defer hub.Unsubscribe(ch)

	ctx := c.Request.Context()
	ticker := time.NewTicker(25 * time.Second)
	defer ticker.Stop()

	c.Stream(func(w io.Writer) bool {
		select {
		case <-ctx.Done():
			return false
		case data, ok := <-ch:
			if !ok {
				return false
			}
			fmt.Fprintf(w, "event: log\ndata: %s\n\n", data)
			return true
		case <-ticker.C:
			fmt.Fprintf(w, ": ping\n\n")
			return true
		}
	})
}
