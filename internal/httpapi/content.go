package httpapi

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/thun888/gdocs-go/internal/auth"
	"github.com/thun888/gdocs-go/internal/doc"
	"github.com/thun888/gdocs-go/internal/source"
	"github.com/thun888/gdocs-go/internal/store"
)

func (a *API) source(c *gin.Context) (*store.Source, bool) {
	src, err := a.doc.Source(c.Request.Context(), c.Param("slug"))
	if err != nil {
		if store.IsNotFound(err) {
			fail(c, http.StatusNotFound, "来源不存在")
			return nil, false
		}
		slog.Error("读取来源失败", "error", err)
		fail(c, http.StatusInternalServerError, "读取来源失败")
		return nil, false
	}
	if !src.Enabled {
		fail(c, http.StatusNotFound, "来源已停用")
		return nil, false
	}
	return src, true
}

func (a *API) handleSources(c *gin.Context) {
	list, err := a.store.ListSources(c.Request.Context(), true)
	if err != nil {
		slog.Error("读取来源列表失败", "error", err)
		fail(c, http.StatusInternalServerError, "读取来源列表失败")
		return
	}
	items := make([]gin.H, 0, len(list))
	for _, item := range list {
		items = append(items, gin.H{"slug": item.Slug, "name": item.Name, "sort": item.Sort})
	}
	ok(c, gin.H{"sources": items})
}

func (a *API) handleTree(c *gin.Context) {
	src, valid := a.source(c)
	if !valid {
		return
	}
	if err := a.doc.EnsureFresh(c.Request.Context(), src); err != nil {
		slog.Warn("刷新文档索引失败", "source", src.Slug, "error", err)
	}

	_, loggedIn := auth.User(c)
	nodes, err := a.doc.Tree(c.Request.Context(), src.ID, loggedIn)
	if err != nil {
		slog.Error("构建目录树失败", "error", err)
		fail(c, http.StatusBadGateway, "读取目录失败")
		return
	}
	ok(c, gin.H{"source": gin.H{"slug": src.Slug, "name": src.Name}, "tree": nodes})
}

func (a *API) handleDoc(c *gin.Context) {
	src, valid := a.source(c)
	if !valid {
		return
	}
	path, err := doc.Clean(c.Query("path"))
	if err != nil || path == "" || !doc.IsDoc(path) {
		fail(c, http.StatusBadRequest, "路径不合法")
		return
	}

	client, err := a.doc.Client(c.Request.Context(), src)
	if err != nil {
		slog.Error("创建仓库客户端失败", "error", err)
		fail(c, http.StatusInternalServerError, "仓库配置不可用")
		return
	}

	file, err := client.Read(c.Request.Context(), path)
	if err != nil {
		a.sourceError(c, err)
		return
	}

	fm := doc.Parse(file.Content)
	_, loggedIn := auth.User(c)
	public := effectivePublic(fm.Meta.Public, a.cfg.Server.PublicRead)
	if fm.Meta.Draft && !loggedIn {
		fail(c, http.StatusNotFound, "文档不存在")
		return
	}
	if !loggedIn && !public {
		fail(c, http.StatusNotFound, "文档不存在")
		return
	}

	ok(c, gin.H{
		"path":    path,
		"sha":     file.SHA,
		"content": string(fm.Body),
		"meta": gin.H{
			"title":  doc.Title(path, fm.Meta),
			"order":  fm.Meta.Order,
			"public": public,
			"draft":  fm.Meta.Draft,
		},
	})
}

func (a *API) sourceError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, source.ErrNotFound):
		fail(c, http.StatusNotFound, "文档不存在")
	case errors.Is(err, source.ErrConflict):
		fail(c, http.StatusConflict, "内容已被他人修改")
	case errors.Is(err, source.ErrAuth):
		slog.Error("仓库令牌无效", "error", err)
		fail(c, http.StatusBadGateway, "仓库认证失败")
	default:
		slog.Error("仓库请求失败", "error", err)
		fail(c, http.StatusBadGateway, "仓库服务不可用")
	}
}

func effectivePublic(flag *bool, globalDefault bool) bool {
	if flag == nil {
		return globalDefault
	}
	return *flag
}
