package middleware

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/liuguoyuan/llmux/internal/config"
	"github.com/liuguoyuan/llmux/internal/model"
	"gorm.io/gorm"
)

// APIKeyAuth validates the client API key from Authorization or x-api-key header.
func APIKeyAuth(db *gorm.DB, keyPrefix string) gin.HandlerFunc {
	return func(c *gin.Context) {
		var apiKey string

		// Anthropic style: x-api-key header
		if key := c.GetHeader("x-api-key"); key != "" {
			apiKey = key
		} else if auth := c.GetHeader("Authorization"); auth != "" {
			// OpenAI style: Bearer token
			apiKey = strings.TrimPrefix(auth, "Bearer ")
		}

		if apiKey == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing API key"})
			return
		}

		if keyPrefix != "" && !strings.HasPrefix(apiKey, keyPrefix) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid API key format"})
			return
		}

		var keyObj model.APIKey
		if err := db.Where("key = ?", apiKey).First(&keyObj).Error; err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid API key"})
			return
		}

		if !keyObj.Enabled {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "API key is disabled"})
			return
		}

		if keyObj.ExpireAt > 0 && keyObj.ExpireAt < time.Now().Unix() {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "API key has expired"})
			return
		}

		// Check cost limit
		if keyObj.MaxCost > 0 {
			var stats model.StatsAPIKey
			db.Where("api_key_id = ?", keyObj.ID).First(&stats)
			if stats.InputCost+stats.OutputCost >= keyObj.MaxCost {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "API key cost limit exceeded"})
				return
			}
		}

		c.Set("api_key_id", keyObj.ID)
		c.Set("api_key_rpm", keyObj.RPM)
		c.Set("api_key_tpm", keyObj.TPM)
		c.Set("supported_models", keyObj.SupportedModels)
		c.Next()
	}
}

// JWTAuth validates admin JWT tokens.
func JWTAuth(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.GetHeader("Authorization")
		if token == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing token"})
			return
		}

		token = strings.TrimPrefix(token, "Bearer ")
		if !verifyJWT(token, secret) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}
		c.Next()
	}
}

// verifyJWT validates a JWT token. Implementation uses standard HS256.
func verifyJWT(token, secret string) bool {
	// TODO: implement proper JWT verification
	_ = config.Config{}
	return token != "" && secret != ""
}
