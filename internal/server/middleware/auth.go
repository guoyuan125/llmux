package middleware

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/liuguoyuan/llmux/internal/model"
	"github.com/liuguoyuan/llmux/internal/ratelimit"
	"gorm.io/gorm"
)

// APIKeyAuth validates the client API key from Authorization or x-api-key header.
func APIKeyAuth(db *gorm.DB, keyPrefix string) gin.HandlerFunc {
	return func(c *gin.Context) {
		var apiKey string

		if key := c.GetHeader("x-api-key"); key != "" {
			apiKey = key
		} else if auth := c.GetHeader("Authorization"); auth != "" {
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
		if err := db.Where("`key` = ?", apiKey).First(&keyObj).Error; err != nil {
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

		if keyObj.MaxCost > 0 {
			var stats model.StatsAPIKey
			db.Where("api_key_id = ?", keyObj.ID).First(&stats)
			if stats.InputCost+stats.OutputCost >= keyObj.MaxCost {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "API key cost limit exceeded"})
				return
			}
		}

		// Rate limiting: RPM and TPM
		if keyObj.RPM > 0 || keyObj.TPM > 0 {
			if ok, reason := ratelimit.Global.Allow(keyObj.ID, keyObj.RPM, keyObj.TPM); !ok {
				c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": reason})
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
			// Fallback to query param (needed for SSE EventSource which can't set headers)
			token = c.Query("token")
		}
		if token == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing token"})
			return
		}

		token = strings.TrimPrefix(token, "Bearer ")
		claims, err := verifyJWT(token, secret)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}

		c.Set("username", claims.Username)
		c.Next()
	}
}

// JWTClaims holds JWT payload.
type JWTClaims struct {
	Username string `json:"username"`
	Exp      int64  `json:"exp"`
}

// GenerateJWT creates a signed JWT token.
func GenerateJWT(secret, username string, duration time.Duration) string {
	header := base64url([]byte(`{"alg":"HS256","typ":"JWT"}`))
	claims := JWTClaims{
		Username: username,
		Exp:      time.Now().Add(duration).Unix(),
	}
	claimsJSON, _ := json.Marshal(claims)
	payload := base64url(claimsJSON)

	signingInput := header + "." + payload
	sig := signHS256([]byte(signingInput), []byte(secret))

	return signingInput + "." + base64url(sig)
}

func verifyJWT(token, secret string) (*JWTClaims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid token format")
	}

	signingInput := parts[0] + "." + parts[1]
	expectedSig := signHS256([]byte(signingInput), []byte(secret))
	actualSig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, fmt.Errorf("invalid signature encoding")
	}

	if !hmac.Equal(expectedSig, actualSig) {
		return nil, fmt.Errorf("invalid signature")
	}

	claimsJSON, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("invalid claims encoding")
	}

	var claims JWTClaims
	if err := json.Unmarshal(claimsJSON, &claims); err != nil {
		return nil, fmt.Errorf("invalid claims")
	}

	if claims.Exp > 0 && time.Now().Unix() > claims.Exp {
		return nil, fmt.Errorf("token expired")
	}

	return &claims, nil
}

func signHS256(data, key []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write(data)
	return h.Sum(nil)
}

func base64url(data []byte) string {
	return base64.RawURLEncoding.EncodeToString(data)
}
