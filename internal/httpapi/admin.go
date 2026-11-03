package httpapi

import (
	"log/slog"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/thun888/zdocs-go/internal/store"
)

var slugPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)

type sourceView struct {
	ID      uint64 `json:"id"`
	Slug    string `json:"slug"`
	Name    string `json:"name"`
	BaseURL string `json:"base_url"`
	Owner   string `json:"owner"`
	Repo    string `json:"repo"`
	Branch  string `json:"branch"`
	Root    string `json:"root"`
	Sort    int    `json:"sort"`
	Enabled bool   `json:"enabled"`
}

func toSourceView(src store.Source) sourceView {
	return sourceView{
		ID: src.ID, Slug: src.Slug, Name: src.Name, BaseURL: src.BaseURL,
		Owner: src.Owner, Repo: src.Repo, Branch: src.Branch, Root: src.Root,
		Sort: src.Sort, Enabled: src.Enabled,
	}
}

func (a *API) handleAdminSources(c *gin.Context) {
	list, err := a.store.ListSources(c.Request.Context(), false)
	if err != nil {
		fail(c, http.StatusInternalServerError, "读取来源失败")
		return
	}
	items := make([]sourceView, 0, len(list))
	for _, item := range list {
		items = append(items, toSourceView(item))
	}
	ok(c, gin.H{"sources": items})
}

type sourceRequest struct {
	Slug    string `json:"slug"`
	Name    string `json:"name"`
	BaseURL string `json:"base_url"`
	Token   string `json:"token"`
	Owner   string `json:"owner"`
	Repo    string `json:"repo"`
	Branch  string `json:"branch"`
	Root    string `json:"root"`
	Sort    int    `json:"sort"`
	Enabled *bool  `json:"enabled"`
}

func (a *API) handleCreateSource(c *gin.Context) {
	var req sourceRequest
	if !bindJSON(c, &req) {
		return
	}
	req.Slug = strings.TrimSpace(req.Slug)
	if !slugPattern.MatchString(req.Slug) {
		fail(c, http.StatusBadRequest, "slug 只能是小写字母、数字与连字符")
		return
	}
	if req.Name == "" || req.BaseURL == "" || req.Token == "" || req.Owner == "" || req.Repo == "" {
		fail(c, http.StatusBadRequest, "缺少必填项")
		return
	}
	sealed, err := a.doc.SealToken(req.Token)
	if err != nil {
		fail(c, http.StatusInternalServerError, "保存令牌失败")
		return
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	branch := req.Branch
	if branch == "" {
		branch = "main"
	}
	src := &store.Source{
		Slug: req.Slug, Name: req.Name, BaseURL: strings.TrimSuffix(req.BaseURL, "/"),
		TokenEnc: sealed, Owner: req.Owner, Repo: req.Repo, Branch: branch,
		Root: strings.Trim(req.Root, "/"), Sort: req.Sort, Enabled: enabled,
	}
	if err := a.store.CreateSource(c.Request.Context(), src); err != nil {
		slog.Error("创建来源失败", "error", err)
		fail(c, http.StatusInternalServerError, "创建来源失败，slug 可能已存在")
		return
	}
	ok(c, toSourceView(*src))
}

func (a *API) handleUpdateSource(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		fail(c, http.StatusBadRequest, "参数错误")
		return
	}
	src, err := a.store.SourceByID(c.Request.Context(), id)
	if err != nil {
		fail(c, http.StatusNotFound, "来源不存在")
		return
	}
	var req sourceRequest
	if !bindJSON(c, &req) {
		return
	}

	if req.Slug != "" {
		if !slugPattern.MatchString(req.Slug) {
			fail(c, http.StatusBadRequest, "slug 只能是小写字母、数字与连字符")
			return
		}
		src.Slug = req.Slug
	}
	if req.Name != "" {
		src.Name = req.Name
	}
	if req.BaseURL != "" {
		src.BaseURL = strings.TrimSuffix(req.BaseURL, "/")
	}
	if req.Owner != "" {
		src.Owner = req.Owner
	}
	if req.Repo != "" {
		src.Repo = req.Repo
	}
	if req.Branch != "" {
		src.Branch = req.Branch
	}
	src.Root = strings.Trim(req.Root, "/")
	src.Sort = req.Sort
	if req.Enabled != nil {
		src.Enabled = *req.Enabled
	}
	if req.Token != "" {
		sealed, err := a.doc.SealToken(req.Token)
		if err != nil {
			fail(c, http.StatusInternalServerError, "保存令牌失败")
			return
		}
		src.TokenEnc = sealed
	}

	if err := a.store.UpdateSource(c.Request.Context(), src); err != nil {
		slog.Error("更新来源失败", "error", err)
		fail(c, http.StatusInternalServerError, "更新来源失败")
		return
	}
	ok(c, toSourceView(*src))
}

func (a *API) handleDeleteSource(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		fail(c, http.StatusBadRequest, "参数错误")
		return
	}
	if err := a.store.DeleteSource(c.Request.Context(), id); err != nil {
		fail(c, http.StatusInternalServerError, "删除来源失败")
		return
	}
	ok(c, gin.H{"deleted": id})
}

func (a *API) handleAdminUsers(c *gin.Context) {
	users, err := a.store.ListUsers(c.Request.Context())
	if err != nil {
		fail(c, http.StatusInternalServerError, "读取用户失败")
		return
	}
	items := make([]gin.H, 0, len(users))
	for i := range users {
		groups, _ := a.store.GroupsByUser(c.Request.Context(), users[i].ID)
		codes := make([]string, 0, len(groups))
		for _, g := range groups {
			codes = append(codes, g.Code)
		}
		items = append(items, gin.H{
			"id": users[i].ID, "subject": users[i].Subject, "username": users[i].Username,
			"display_name": users[i].DisplayName, "email": users[i].Email,
			"role": users[i].Role, "groups": codes, "last_login_at": users[i].LastLoginAt,
		})
	}
	ok(c, gin.H{"users": items})
}

func (a *API) handleAdminGroups(c *gin.Context) {
	rows, err := a.store.ListGroups(c.Request.Context())
	if err != nil {
		fail(c, http.StatusInternalServerError, "读取用户组失败")
		return
	}
	items := make([]gin.H, 0, len(rows))
	for _, row := range rows {
		items = append(items, gin.H{
			"id": row.ID, "code": row.Code, "name": row.Name, "type": row.Type,
			"role": row.Role, "ignored": row.Ignored, "members": row.Members,
		})
	}
	ok(c, gin.H{"groups": items})
}

func (a *API) handleUpdateGroup(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		fail(c, http.StatusBadRequest, "参数错误")
		return
	}
	var req struct {
		Role    string `json:"role"`
		Ignored bool   `json:"ignored"`
	}
	if !bindJSON(c, &req) {
		return
	}
	switch req.Role {
	case "viewer", "editor", "admin":
	default:
		fail(c, http.StatusBadRequest, "角色不合法")
		return
	}
	if err := a.store.UpdateGroup(c.Request.Context(), id, req.Role, req.Ignored); err != nil {
		fail(c, http.StatusInternalServerError, "更新用户组失败")
		return
	}
	ok(c, gin.H{"updated": id})
}

func (a *API) handleAdminSync(c *gin.Context) {
	var req struct {
		Slug string `json:"slug"`
	}
	if !bindJSON(c, &req) {
		return
	}
	src, err := a.store.SourceBySlug(c.Request.Context(), req.Slug)
	if err != nil {
		fail(c, http.StatusNotFound, "来源不存在")
		return
	}
	if err := a.doc.Sync(c.Request.Context(), src); err != nil {
		slog.Error("重建索引失败", "error", err)
		fail(c, http.StatusBadGateway, "重建索引失败")
		return
	}
	ok(c, gin.H{"synced": src.Slug})
}
