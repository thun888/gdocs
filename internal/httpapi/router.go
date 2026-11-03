package httpapi

import (
	"github.com/gin-gonic/gin"

	"github.com/thun888/gdocs-go/internal/auth"
	"github.com/thun888/gdocs-go/internal/config"
	"github.com/thun888/gdocs-go/internal/doc"
	"github.com/thun888/gdocs-go/internal/storage"
	"github.com/thun888/gdocs-go/internal/store"
)

type Options struct {
	Config *config.Config
	Store  *store.Store
	Auth   *auth.Service
	Doc    *doc.Service
	Upload *storage.Uploader
}

type API struct {
	cfg    *config.Config
	store  *store.Store
	auth   *auth.Service
	doc    *doc.Service
	upload *storage.Uploader
}

func New(opts Options) *API {
	return &API{
		cfg:    opts.Config,
		store:  opts.Store,
		auth:   opts.Auth,
		doc:    opts.Doc,
		upload: opts.Upload,
	}
}

func (a *API) Register(r *gin.Engine) {
	v1 := r.Group("/api/v1")
	v1.Use(a.auth.Optional())

	v1.GET("/site", a.handleSite)
	v1.GET("/auth/login", a.handleLogin)
	v1.GET("/auth/callback", a.handleCallback)
	v1.POST("/auth/logout", a.auth.VerifyCSRF(), a.handleLogout)
	v1.GET("/me", a.handleMe)

	v1.GET("/sources", a.handleSources)
	v1.GET("/assets/*key", a.auth.RequireAuth(), a.handleAsset)
	v1.GET("/sources/:slug/tree", a.handleTree)
	v1.GET("/sources/:slug/doc", a.handleDoc)
	v1.GET("/sources/:slug/history", a.handleHistory)
	v1.GET("/sources/:slug/commit/:sha", a.auth.RequireAuth(), a.handleCommit)
	v1.GET("/sources/:slug/compare", a.auth.RequireAuth(), a.handleCompare)
	v1.GET("/sources/:slug/recent", a.handleRecent)

	write := v1.Group("")
	write.Use(a.auth.RequireAuth(), a.auth.RequireEditor(), a.auth.VerifyCSRF())
	write.PUT("/sources/:slug/doc", a.handleSaveDoc)
	write.POST("/sources/:slug/doc", a.handleCreateDoc)
	write.DELETE("/sources/:slug/doc", a.handleDeleteDoc)
	write.POST("/sources/:slug/move", a.handleMoveDoc)
	write.PUT("/sources/:slug/meta", a.handleSaveMeta)
	write.POST("/sources/:slug/import", a.handleImport)

	admin := v1.Group("/admin")
	admin.Use(a.auth.RequireAuth(), a.auth.RequireAdmin(), a.auth.VerifyCSRF())
	admin.GET("/sources", a.handleAdminSources)
	admin.POST("/sources", a.handleCreateSource)
	admin.PUT("/sources/:id", a.handleUpdateSource)
	admin.DELETE("/sources/:id", a.handleDeleteSource)
	admin.GET("/users", a.handleAdminUsers)
	admin.GET("/groups", a.handleAdminGroups)
	admin.PUT("/groups/:id", a.handleUpdateGroup)
	admin.GET("/settings", a.handleGetSettings)
	admin.PUT("/settings", a.handleSaveSettings)
	admin.POST("/sync", a.handleAdminSync)

	v1.POST("/uploads", a.auth.RequireAuth(), a.auth.RequireEditor(), a.auth.VerifyCSRF(), a.handleUpload)
}
