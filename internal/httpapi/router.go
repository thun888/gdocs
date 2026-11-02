package httpapi

import (
	"github.com/gin-gonic/gin"

	"github.com/thun888/zdocs-go/internal/auth"
	"github.com/thun888/zdocs-go/internal/config"
	"github.com/thun888/zdocs-go/internal/store"
)

type API struct {
	cfg   *config.Config
	store *store.Store
	auth  *auth.Service
}

func New(cfg *config.Config, st *store.Store, authSvc *auth.Service) *API {
	return &API{cfg: cfg, store: st, auth: authSvc}
}

func (a *API) Register(r *gin.Engine) {
	v1 := r.Group("/api/v1")
	v1.Use(a.auth.Optional())

	v1.GET("/auth/login", a.handleLogin)
	v1.GET("/auth/callback", a.handleCallback)
	v1.POST("/auth/logout", a.auth.VerifyCSRF(), a.handleLogout)
	v1.GET("/me", a.handleMe)
}
