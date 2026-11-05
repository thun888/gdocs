package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

type searchTarget struct {
	Slug string `json:"slug"`
	Name string `json:"name"`
	URL  string `json:"url"`
}

func (a *API) handleSite(c *gin.Context) {
	ctx := c.Request.Context()
	name, _ := a.store.SettingGet(ctx, "site_name")
	desc, _ := a.store.SettingGet(ctx, "site_desc")
	if name == "" {
		name = "gdocs"
	}
	ok(c, gin.H{
		"name":               name,
		"description":        desc,
		"public_read":        a.cfg.Server.PublicRead,
		"header_show_avatar": a.boolSetting(ctx, "header_show_avatar"),
		"header_show_name":   a.boolSetting(ctx, "header_show_name"),
		"header_show_logout": a.boolSetting(ctx, "header_show_logout"),
		"search":             a.searchTargets(ctx),
	})
}

// 顶栏检索直接落到 Git 服务器，由它自己的代码索引出结果
func (a *API) searchTargets(ctx context.Context) []searchTarget {
	list, err := a.store.ListSources(ctx, true)
	if err != nil {
		slog.Error("读取来源列表失败", "error", err)
		return []searchTarget{}
	}
	items := make([]searchTarget, 0, len(list))
	for _, src := range list {
		items = append(items, searchTarget{
			Slug: src.Slug,
			Name: src.Name,
			URL: strings.TrimSuffix(src.BaseURL, "/") + "/" +
				url.PathEscape(src.Owner) + "/" + url.PathEscape(src.Repo) + "/search",
		})
	}
	return items
}

func (a *API) handleGetSettings(c *gin.Context) {
	ctx := c.Request.Context()
	name, _ := a.store.SettingGet(ctx, "site_name")
	desc, _ := a.store.SettingGet(ctx, "site_desc")
	ok(c, gin.H{
		"site_name":          name,
		"site_desc":          desc,
		"header_show_avatar": a.boolSetting(ctx, "header_show_avatar"),
		"header_show_name":   a.boolSetting(ctx, "header_show_name"),
		"header_show_logout": a.boolSetting(ctx, "header_show_logout"),
	})
}

func (a *API) handleSaveSettings(c *gin.Context) {
	var req struct {
		SiteName         string `json:"site_name"`
		SiteDesc         string `json:"site_desc"`
		HeaderShowAvatar bool   `json:"header_show_avatar"`
		HeaderShowName   bool   `json:"header_show_name"`
		HeaderShowLogout bool   `json:"header_show_logout"`
	}
	if !bindJSON(c, &req) {
		return
	}
	values := map[string]string{
		"site_name":          req.SiteName,
		"site_desc":          req.SiteDesc,
		"header_show_avatar": strconv.FormatBool(req.HeaderShowAvatar),
		"header_show_name":   strconv.FormatBool(req.HeaderShowName),
		"header_show_logout": strconv.FormatBool(req.HeaderShowLogout),
	}
	for key, value := range values {
		if err := a.store.SettingSet(c.Request.Context(), key, value); err != nil {
			fail(c, http.StatusInternalServerError, "保存设置失败")
			return
		}
	}
	ok(c, gin.H{"saved": true})
}

// 顶栏开关缺省为显示，只有显式存成 false 才隐藏
func (a *API) boolSetting(ctx context.Context, key string) bool {
	value, _ := a.store.SettingGet(ctx, key)
	return value != "false"
}
