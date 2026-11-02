package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func (a *API) handleSite(c *gin.Context) {
	name, _ := a.store.SettingGet(c.Request.Context(), "site_name")
	desc, _ := a.store.SettingGet(c.Request.Context(), "site_desc")
	if name == "" {
		name = "ZDocs"
	}
	ok(c, gin.H{
		"name":        name,
		"description": desc,
		"public_read": a.cfg.Server.PublicRead,
	})
}

func (a *API) handleGetSettings(c *gin.Context) {
	name, _ := a.store.SettingGet(c.Request.Context(), "site_name")
	desc, _ := a.store.SettingGet(c.Request.Context(), "site_desc")
	ok(c, gin.H{"site_name": name, "site_desc": desc})
}

func (a *API) handleSaveSettings(c *gin.Context) {
	var req struct {
		SiteName string `json:"site_name"`
		SiteDesc string `json:"site_desc"`
	}
	if !bindJSON(c, &req) {
		return
	}
	if err := a.store.SettingSet(c.Request.Context(), "site_name", req.SiteName); err != nil {
		fail(c, http.StatusInternalServerError, "保存设置失败")
		return
	}
	if err := a.store.SettingSet(c.Request.Context(), "site_desc", req.SiteDesc); err != nil {
		fail(c, http.StatusInternalServerError, "保存设置失败")
		return
	}
	ok(c, gin.H{"saved": true})
}
