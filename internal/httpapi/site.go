package httpapi

import (
	"context"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

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
	})
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
