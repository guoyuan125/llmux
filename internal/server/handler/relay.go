package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/liuguoyuan/llmux/internal/gateway/relay"
	"github.com/liuguoyuan/llmux/internal/model"
)

// RelayChatCompletions proxies OpenAI /v1/chat/completions requests.
func (h *Handler) RelayChatCompletions(c *gin.Context) {
	h.gateway.HandleRelay(c, relay.InboundOpenAIChat)
}

// RelayResponses proxies OpenAI /v1/responses requests.
func (h *Handler) RelayResponses(c *gin.Context) {
	// OpenAI Responses API uses same inbound as chat for now
	h.gateway.HandleRelay(c, relay.InboundOpenAIChat)
}

// RelayMessages proxies Anthropic /v1/messages requests.
func (h *Handler) RelayMessages(c *gin.Context) {
	h.gateway.HandleRelay(c, relay.InboundAnthropic)
}

// RelayEmbeddings proxies /v1/embeddings requests.
func (h *Handler) RelayEmbeddings(c *gin.Context) {
	// TODO: implement embedding relay
	h.gateway.HandleRelay(c, relay.InboundOpenAIChat)
}

// ListModels returns available models based on configured groups.
func (h *Handler) ListModels(c *gin.Context) {
	var groups []model.Group
	h.db.Find(&groups)

	type modelEntry struct {
		ID      string `json:"id"`
		Object  string `json:"object"`
		OwnedBy string `json:"owned_by"`
	}

	models := make([]modelEntry, 0, len(groups))
	for _, g := range groups {
		models = append(models, modelEntry{
			ID:      g.Name,
			Object:  "model",
			OwnedBy: "llmux",
		})
	}

	c.JSON(200, gin.H{
		"object": "list",
		"data":   models,
	})
}
