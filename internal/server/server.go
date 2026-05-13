package server

import (
	"github.com/gin-gonic/gin"
	"github.com/liuguoyuan/llmux/internal/config"
	"github.com/liuguoyuan/llmux/internal/server/handler"
	"github.com/liuguoyuan/llmux/internal/server/middleware"
	"gorm.io/gorm"
)

// Server holds the HTTP server dependencies.
type Server struct {
	cfg    *config.Config
	db     *gorm.DB
	engine *gin.Engine
}

// New creates a new server instance with all routes registered.
func New(cfg *config.Config, db *gorm.DB) *Server {
	gin.SetMode(gin.ReleaseMode)
	engine := gin.New()
	engine.Use(gin.Recovery())
	engine.Use(middleware.Logger())
	engine.Use(middleware.CORS())

	s := &Server{cfg: cfg, db: db, engine: engine}
	s.registerRoutes()
	return s
}

// Engine returns the underlying gin engine.
func (s *Server) Engine() *gin.Engine {
	return s.engine
}

func (s *Server) registerRoutes() {
	h := handler.New(s.db, s.cfg)

	// Health check
	s.engine.GET("/health", h.Health)

	// Prometheus metrics
	if s.cfg.Metrics.Enabled {
		s.engine.GET("/metrics", h.Metrics)
	}

	// Auth routes
	auth := s.engine.Group("/api/auth")
	{
		auth.POST("/login", h.Login)
	}

	// Admin API (JWT protected)
	admin := s.engine.Group("/api")
	admin.Use(middleware.JWTAuth(s.cfg.Auth.JWTSecret))
	{
		// Channels
		admin.GET("/channels", h.ListChannels)
		admin.POST("/channels", h.CreateChannel)
		admin.PUT("/channels/:id", h.UpdateChannel)
		admin.DELETE("/channels/:id", h.DeleteChannel)

		// Groups
		admin.GET("/groups", h.ListGroups)
		admin.POST("/groups", h.CreateGroup)
		admin.PUT("/groups/:id", h.UpdateGroup)
		admin.DELETE("/groups/:id", h.DeleteGroup)

		// API Keys
		admin.GET("/apikeys", h.ListAPIKeys)
		admin.POST("/apikeys", h.CreateAPIKey)
		admin.PUT("/apikeys/:id", h.UpdateAPIKey)
		admin.DELETE("/apikeys/:id", h.DeleteAPIKey)

		// Stats
		admin.GET("/stats/overview", h.StatsOverview)
		admin.GET("/stats/daily", h.StatsDaily)
		admin.GET("/stats/models", h.StatsModels)

		// Audit logs
		admin.GET("/logs", h.ListAuditLogs)

		// Settings
		admin.GET("/settings", h.GetSettings)
		admin.PUT("/settings", h.UpdateSettings)
	}

	// LLM API relay (API Key protected)
	// OpenAI-compatible endpoints
	v1 := s.engine.Group("/v1")
	v1.Use(middleware.APIKeyAuth(s.db, s.cfg.Auth.KeyPrefix))
	{
		v1.POST("/chat/completions", h.RelayChatCompletions)
		v1.POST("/responses", h.RelayResponses)
		v1.POST("/embeddings", h.RelayEmbeddings)
		v1.GET("/models", h.ListModels)
	}

	// Anthropic-compatible endpoint
	anthropic := s.engine.Group("/v1")
	anthropic.Use(middleware.APIKeyAuth(s.db, s.cfg.Auth.KeyPrefix))
	{
		anthropic.POST("/messages", h.RelayMessages)
	}
}
