package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/liuguoyuan/llmux/internal/model"
	"github.com/liuguoyuan/llmux/internal/server/middleware"
	"golang.org/x/crypto/bcrypt"
)

type loginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// Login authenticates admin user and returns JWT.
func (h *Handler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Check against DB user first
	var user model.User
	if err := h.db.Where("username = ?", req.Username).First(&user).Error; err == nil {
		if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
			return
		}
		token := middleware.GenerateJWT(h.cfg.Auth.JWTSecret, user.Username, 24*time.Hour)
		c.JSON(http.StatusOK, gin.H{"token": token, "expires_at": time.Now().Add(24 * time.Hour).Unix()})
		return
	}

	// Fallback to default admin credentials from config
	if req.Username == h.cfg.Auth.AdminUser && req.Password == h.cfg.Auth.AdminPass {
		token := middleware.GenerateJWT(h.cfg.Auth.JWTSecret, req.Username, 24*time.Hour)
		c.JSON(http.StatusOK, gin.H{"token": token, "expires_at": time.Now().Add(24 * time.Hour).Unix()})
		return
	}

	c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
}

// GetSettings returns all settings.
func (h *Handler) GetSettings(c *gin.Context) {
	var settings []model.Setting
	h.db.Find(&settings)
	c.JSON(http.StatusOK, settings)
}

// UpdateSettings updates settings.
func (h *Handler) UpdateSettings(c *gin.Context) {
	var input []model.Setting
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	for _, s := range input {
		h.db.Save(&s)
	}
	c.JSON(http.StatusOK, gin.H{"message": "updated"})
}
