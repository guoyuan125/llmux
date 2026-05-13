package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// RelayChatCompletions proxies OpenAI /v1/chat/completions requests.
func (h *Handler) RelayChatCompletions(c *gin.Context) {
	// TODO: implement via gateway.Relay
	c.JSON(http.StatusNotImplemented, gin.H{"error": "not implemented"})
}

// RelayResponses proxies OpenAI /v1/responses requests.
func (h *Handler) RelayResponses(c *gin.Context) {
	// TODO: implement via gateway.Relay
	c.JSON(http.StatusNotImplemented, gin.H{"error": "not implemented"})
}

// RelayMessages proxies Anthropic /v1/messages requests.
func (h *Handler) RelayMessages(c *gin.Context) {
	// TODO: implement via gateway.Relay
	c.JSON(http.StatusNotImplemented, gin.H{"error": "not implemented"})
}

// RelayEmbeddings proxies /v1/embeddings requests.
func (h *Handler) RelayEmbeddings(c *gin.Context) {
	// TODO: implement via gateway.Relay
	c.JSON(http.StatusNotImplemented, gin.H{"error": "not implemented"})
}

// ListModels returns available models based on configured groups.
func (h *Handler) ListModels(c *gin.Context) {
	// TODO: return models from enabled groups
	c.JSON(http.StatusOK, gin.H{
		"object": "list",
		"data":   []interface{}{},
	})
}
