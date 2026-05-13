package handler

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/liuguoyuan/llmux/internal/model"
)

// ListAPIKeys returns all API keys.
func (h *Handler) ListAPIKeys(c *gin.Context) {
	var keys []model.APIKey
	if err := h.db.Find(&keys).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, keys)
}

// CreateAPIKey creates a new API key with auto-generated key value.
func (h *Handler) CreateAPIKey(c *gin.Context) {
	var input model.APIKey
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Generate key
	input.Key = h.cfg.Auth.KeyPrefix + generateRandomKey(32)
	input.Enabled = true

	if err := h.db.Create(&input).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, input)
}

// UpdateAPIKey updates an existing API key.
func (h *Handler) UpdateAPIKey(c *gin.Context) {
	id := c.Param("id")
	var key model.APIKey
	if err := h.db.First(&key, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "API key not found"})
		return
	}

	var input model.APIKey
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.db.Model(&key).Updates(input).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, key)
}

// DeleteAPIKey deletes an API key.
func (h *Handler) DeleteAPIKey(c *gin.Context) {
	id := c.Param("id")
	if err := h.db.Delete(&model.APIKey{}, id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

func generateRandomKey(length int) string {
	b := make([]byte, length)
	rand.Read(b)
	return hex.EncodeToString(b)[:length]
}
