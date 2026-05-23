package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// CircuitStatus returns real-time circuit breaker status for all channels.
func (h *Handler) CircuitStatus(c *gin.Context) {
	entries := h.gateway.CircuitStatus()
	c.JSON(http.StatusOK, entries)
}
