package server

import (
	"io/fs"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/liuguoyuan/llmux/internal/config"
	"github.com/liuguoyuan/llmux/internal/server/handler"
	"github.com/liuguoyuan/llmux/internal/server/middleware"
	"github.com/liuguoyuan/llmux/web"
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
		admin.GET("/channels/:id/sync-models", h.SyncChannelModels)

		// Groups
		admin.GET("/groups", h.ListGroups)
		admin.POST("/groups", h.CreateGroup)
		admin.PUT("/groups/:id", h.UpdateGroup)
		admin.DELETE("/groups/:id", h.DeleteGroup)
		admin.POST("/groups/:id/duplicate", h.DuplicateGroup)

		// API Keys
		admin.GET("/apikeys", h.ListAPIKeys)
		admin.POST("/apikeys", h.CreateAPIKey)
		admin.PUT("/apikeys/:id", h.UpdateAPIKey)
		admin.DELETE("/apikeys/:id", h.DeleteAPIKey)

		// Stats
		admin.GET("/stats/overview", h.StatsOverview)
		admin.GET("/stats/daily", h.StatsDaily)
		admin.GET("/stats/hourly", h.StatsHourly)
		admin.GET("/stats/models", h.StatsModels)
		admin.GET("/stats/channels", h.StatsChannels)

		// Audit logs
		admin.GET("/logs", h.ListAuditLogs)
		admin.GET("/logs/stream", h.StreamLogs)

		// Circuit breaker status
		admin.GET("/circuit/status", h.CircuitStatus)

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

	// Serve embedded frontend static files
	staticFS, err := fs.Sub(web.StaticFS, "out")
	if err == nil {
		fileServer := http.FileServer(http.FS(staticFS))
		s.engine.NoRoute(func(c *gin.Context) {
			// Try serving the exact file first
			path := c.Request.URL.Path
			if f, err := staticFS.(fs.ReadFileFS).ReadFile(path[1:] + "index.html"); err == nil && f != nil {
				c.Data(http.StatusOK, "text/html; charset=utf-8", f)
				return
			}
			// Fallback to file server
			fileServer.ServeHTTP(c.Writer, c.Request)
		})
	}
}
