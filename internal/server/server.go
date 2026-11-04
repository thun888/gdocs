package server

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/thun888/gdocs-go/internal/config"
	"github.com/thun888/gdocs-go/internal/httpapi"
	"github.com/thun888/gdocs-go/internal/web"
)

type Server struct {
	cfg *config.Config
	db  *gorm.DB
}

func New(cfg *config.Config, db *gorm.DB, api *httpapi.API) *gin.Engine {
	s := &Server{cfg: cfg, db: db}

	if cfg.Log.Debug {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	_ = r.SetTrustedProxies(cfg.Server.TrustedProxies)
	r.Use(gin.Recovery())
	if cfg.Log.Access {
		r.Use(requestLogger())
	}
	r.Use(securityHeaders())
	r.GET("/healthz", s.healthz)
	api.Register(r)
	s.mountStatic(r)
	return r
}

func (s *Server) healthz(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
	defer cancel()

	sqlDB, err := s.db.DB()
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "degraded", "db": "unavailable"})
		return
	}
	if err := sqlDB.PingContext(ctx); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "degraded", "db": "down"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok", "db": "ok"})
}

func (s *Server) mountStatic(r *gin.Engine) {
	assets := &staticAssets{fs: web.Dist()}
	r.NoRoute(assets.handle)
}

func securityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.Writer.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Next()
	}
}

func requestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		slog.Info("request",
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
			"cost_ms", time.Since(start).Milliseconds(),
			"ip", c.ClientIP(),
		)
	}
}
