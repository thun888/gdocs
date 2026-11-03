package httpapi

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/thun888/gdocs-go/internal/auth"
	"github.com/thun888/gdocs-go/internal/doc"
	"github.com/thun888/gdocs-go/internal/source"
	"github.com/thun888/gdocs-go/internal/store"
)

type metaUpdate struct {
	Title  *string `json:"title"`
	Public *bool   `json:"public"`
	Draft  *bool   `json:"draft"`
	Order  *int    `json:"order"`
}

type saveDocRequest struct {
	Path    string      `json:"path"`
	Content string      `json:"content"`
	SHA     string      `json:"sha"`
	Message string      `json:"message"`
	Meta    *metaUpdate `json:"meta"`
}

type createDocRequest struct {
	Path    string `json:"path"`
	Title   string `json:"title"`
	Content string `json:"content"`
	Message string `json:"message"`
}

func (a *API) commitIdentity(c *gin.Context) source.Identity {
	user, ok := auth.User(c)
	if !ok {
		return source.Identity{Name: a.cfg.Git.BotName, Email: a.cfg.Git.BotEmail}
	}
	name := user.DisplayName
	if strings.TrimSpace(name) == "" {
		name = user.Username
	}
	email := user.Email
	if strings.TrimSpace(email) == "" {
		email = a.cfg.Git.BotEmail
	}
	return source.Identity{Name: name, Email: email}
}

func (a *API) syncSource(src *store.Source) {
	if !a.cfg.Index.SyncOnWrite {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	if err := a.doc.Sync(ctx, src); err != nil {
		slog.Warn("同步文档索引失败", "source", src.Slug, "error", err)
	}
}

func (a *API) handleSaveDoc(c *gin.Context) {
	src, valid := a.source(c)
	if !valid {
		return
	}
	var req saveDocRequest
	if !bindJSON(c, &req) {
		return
	}
	clean, err := doc.Clean(req.Path)
	if err != nil || clean == "" || !doc.IsDoc(clean) {
		fail(c, http.StatusBadRequest, "路径不合法")
		return
	}

	client, err := a.doc.Client(c.Request.Context(), src)
	if err != nil {
		fail(c, http.StatusInternalServerError, "仓库配置不可用")
		return
	}

	current, err := client.Read(c.Request.Context(), clean)
	if err != nil {
		a.sourceError(c, err)
		return
	}
	if req.SHA == "" {
		fail(c, http.StatusBadRequest, "缺少版本号")
		return
	}
	if current.SHA != req.SHA {
		fail(c, http.StatusConflict, "内容已被他人修改，请刷新后重试")
		return
	}

	fm := doc.Parse(current.Content)
	raw := fm.Raw
	if req.Meta != nil {
		updates := map[string]any{}
		if req.Meta.Title != nil {
			updates["title"] = *req.Meta.Title
		}
		if req.Meta.Public != nil {
			updates["public"] = *req.Meta.Public
		}
		if req.Meta.Draft != nil {
			updates["draft"] = *req.Meta.Draft
		}
		if req.Meta.Order != nil {
			updates["order"] = *req.Meta.Order
		}
		raw = doc.Merge(raw, updates)
	}
	content := doc.Compose(raw, []byte(req.Content))
	message := req.Message
	if strings.TrimSpace(message) == "" {
		message = "docs: update " + clean
	}

	sha, err := client.Write(c.Request.Context(), clean, content, current.SHA, message, a.commitIdentity(c))
	if err != nil {
		a.sourceError(c, err)
		return
	}
	a.syncSource(src)
	ok(c, gin.H{"sha": sha})
}

func (a *API) handleCreateDoc(c *gin.Context) {
	src, valid := a.source(c)
	if !valid {
		return
	}
	var req createDocRequest
	if !bindJSON(c, &req) {
		return
	}
	clean, err := doc.Clean(req.Path)
	if err != nil || clean == "" || !doc.IsDoc(clean) {
		fail(c, http.StatusBadRequest, "路径不合法")
		return
	}

	title := strings.TrimSpace(req.Title)
	if title == "" {
		title = doc.Title(clean, doc.Meta{})
	}
	body := req.Content
	if strings.TrimSpace(body) == "" {
		body = "输入内容...\n"
	}
	raw := fmt.Sprintf("title: %s\n", title)
	content := doc.Compose([]byte(raw), []byte(body))

	message := req.Message
	if strings.TrimSpace(message) == "" {
		message = "docs: create " + clean
	}

	client, err := a.doc.Client(c.Request.Context(), src)
	if err != nil {
		fail(c, http.StatusInternalServerError, "仓库配置不可用")
		return
	}
	sha, err := client.Create(c.Request.Context(), clean, content, message, a.commitIdentity(c))
	if err != nil {
		a.sourceError(c, err)
		return
	}
	a.syncSource(src)
	ok(c, gin.H{"sha": sha, "path": clean})
}

func (a *API) handleDeleteDoc(c *gin.Context) {
	src, valid := a.source(c)
	if !valid {
		return
	}
	clean, err := doc.Clean(c.Query("path"))
	if err != nil || clean == "" || !doc.IsDoc(clean) {
		fail(c, http.StatusBadRequest, "路径不合法")
		return
	}

	client, err := a.doc.Client(c.Request.Context(), src)
	if err != nil {
		fail(c, http.StatusInternalServerError, "仓库配置不可用")
		return
	}
	file, err := client.Read(c.Request.Context(), clean)
	if err != nil {
		a.sourceError(c, err)
		return
	}
	sha := c.Query("sha")
	if sha == "" {
		fail(c, http.StatusBadRequest, "缺少版本号")
		return
	}
	if sha != file.SHA {
		fail(c, http.StatusConflict, "内容已被他人修改，请刷新后重试")
		return
	}

	if err := client.Delete(c.Request.Context(), clean, file.SHA, "docs: delete "+clean, a.commitIdentity(c)); err != nil {
		a.sourceError(c, err)
		return
	}
	a.syncSource(src)
	ok(c, gin.H{"deleted": clean})
}

type moveDocRequest struct {
	From string `json:"from"`
	To   string `json:"to"`
}

func (a *API) handleMoveDoc(c *gin.Context) {
	src, valid := a.source(c)
	if !valid {
		return
	}
	var req moveDocRequest
	if !bindJSON(c, &req) {
		return
	}
	from, err := doc.Clean(req.From)
	if err != nil || from == "" || !doc.IsDoc(from) {
		fail(c, http.StatusBadRequest, "原路径不合法")
		return
	}
	to, err := doc.Clean(req.To)
	if err != nil || to == "" || !doc.IsDoc(to) {
		fail(c, http.StatusBadRequest, "目标路径不合法")
		return
	}
	if from == to {
		fail(c, http.StatusBadRequest, "目标路径与原路径相同")
		return
	}

	client, err := a.doc.Client(c.Request.Context(), src)
	if err != nil {
		fail(c, http.StatusInternalServerError, "仓库配置不可用")
		return
	}
	message := fmt.Sprintf("docs: move %s to %s", from, to)
	if err := client.Move(c.Request.Context(), from, to, "", message, a.commitIdentity(c)); err != nil {
		a.sourceError(c, err)
		return
	}
	a.syncSource(src)
	ok(c, gin.H{"from": from, "to": to})
}

type metaRequest struct {
	Path  string   `json:"path"`
	Order []string `json:"order"`
}

func (a *API) handleSaveMeta(c *gin.Context) {
	src, valid := a.source(c)
	if !valid {
		return
	}
	var req metaRequest
	if !bindJSON(c, &req) {
		return
	}
	dir, err := doc.Clean(req.Path)
	if err != nil {
		fail(c, http.StatusBadRequest, "路径不合法")
		return
	}

	metaPath := path.Join(dir, "_meta.yaml")
	if dir == "" {
		metaPath = "_meta.yaml"
	}
	content := doc.MarshalOrder(req.Order)
	message := "docs: reorder " + dir
	if dir == "" {
		message = "docs: reorder root"
	}

	client, err := a.doc.Client(c.Request.Context(), src)
	if err != nil {
		fail(c, http.StatusInternalServerError, "仓库配置不可用")
		return
	}

	existing, readErr := client.Read(c.Request.Context(), metaPath)
	switch {
	case readErr == nil:
		if _, err := client.Write(c.Request.Context(), metaPath, content, existing.SHA, message, a.commitIdentity(c)); err != nil {
			a.sourceError(c, err)
			return
		}
	case errors.Is(readErr, source.ErrNotFound):
		if _, err := client.Create(c.Request.Context(), metaPath, content, message, a.commitIdentity(c)); err != nil {
			a.sourceError(c, err)
			return
		}
	default:
		a.sourceError(c, readErr)
		return
	}

	a.syncSource(src)
	ok(c, gin.H{"path": metaPath})
}

type importFile struct {
	Name    string `json:"name"`
	Content string `json:"content"`
}

type importRequest struct {
	Category string       `json:"category"`
	Files    []importFile `json:"files"`
}

func (a *API) handleImport(c *gin.Context) {
	src, valid := a.source(c)
	if !valid {
		return
	}
	var req importRequest
	if !bindJSON(c, &req) {
		return
	}
	dir, err := doc.Clean(req.Category)
	if err != nil {
		fail(c, http.StatusBadRequest, "分类路径不合法")
		return
	}
	if len(req.Files) == 0 {
		fail(c, http.StatusBadRequest, "没有可导入的文件")
		return
	}

	client, err := a.doc.Client(c.Request.Context(), src)
	if err != nil {
		fail(c, http.StatusInternalServerError, "仓库配置不可用")
		return
	}
	identity := a.commitIdentity(c)

	imported := make([]string, 0, len(req.Files))
	for _, item := range req.Files {
		name := strings.TrimSpace(item.Name)
		if name == "" || strings.ContainsAny(name, "/\\") {
			continue
		}
		if !doc.IsDoc(name) {
			name += ".md"
		}
		target := path.Join(dir, name)
		if dir == "" {
			target = name
		}

		fm := doc.Parse([]byte(item.Content))
		body := item.Content
		var raw []byte
		if len(fm.Raw) > 0 {
			raw = fm.Raw
			body = string(fm.Body)
		} else {
			title := doc.Title(target, doc.Meta{})
			raw = []byte(fmt.Sprintf("title: %s\n", title))
		}
		payload := doc.Compose(raw, []byte(body))

		if _, err := client.Create(c.Request.Context(), target, payload, "docs: import "+target, identity); err != nil {
			slog.Warn("导入文档失败", "path", target, "error", err)
			continue
		}
		imported = append(imported, target)
	}

	a.syncSource(src)
	ok(c, gin.H{"imported": imported, "count": len(imported)})
}
