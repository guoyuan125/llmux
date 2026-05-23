package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/liuguoyuan/llmux/internal/config"
	"github.com/liuguoyuan/llmux/internal/gateway/circuit"
	"github.com/liuguoyuan/llmux/internal/gateway/relay"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"gorm.io/gorm"
)

// Handler holds dependencies for all HTTP handlers.
type Handler struct {
	db      *gorm.DB
	cfg     *config.Config
	gateway *relay.Gateway
}

// New creates a new handler instance.
func New(db *gorm.DB, cfg *config.Config) *Handler {
	cbCfg := &circuit.Config{
		Threshold:    cfg.Circuit.Threshold,
		ResetTimeout: time.Duration(cfg.Circuit.ResetTimeoutSec) * time.Second,
	}
	if cbCfg.Threshold <= 0 {
		cbCfg.Threshold = 3
	}
	if cbCfg.ResetTimeout <= 0 {
		cbCfg.ResetTimeout = 30 * time.Second
	}

	gw := relay.NewGatewayWithConfig(db, cbCfg)
	hub := GetLogHub()
	gw.SetLogPublisher(hub.Publish)
	return &Handler{
		db:      db,
		cfg:     cfg,
		gateway: gw,
	}
}

// Health returns server health status.
func (h *Handler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// Metrics exposes Prometheus metrics.
func (h *Handler) Metrics(c *gin.Context) {
	promhttp.Handler().ServeHTTP(c.Writer, c.Request)
}
