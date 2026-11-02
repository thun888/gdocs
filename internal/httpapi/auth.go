package httpapi

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/thun888/zdocs-go/internal/auth"
	"github.com/thun888/zdocs-go/internal/store"
)

type userView struct {
	ID          uint64 `json:"id"`
	Subject     string `json:"subject"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	Email       string `json:"email"`
	Class       string `json:"class"`
	Role        string `json:"role"`
	Avatar      string `json:"avatar"`
}

func (a *API) handleLogin(c *gin.Context) {
	url, payload, err := a.auth.BeginLogin(c.Request.Context(), c.Query("redirect"))
	if err != nil {
		slog.Error("发起登录失败", "error", err)
		fail(c, http.StatusBadGateway, "无法连接统一身份认证")
		return
	}
	a.auth.SetStateCookie(c, payload)
	c.Redirect(http.StatusFound, url)
}

func (a *API) handleCallback(c *gin.Context) {
	if message := c.Query("error"); message != "" {
		slog.Warn("上游拒绝授权", "error", message, "desc", c.Query("error_description"))
		fail(c, http.StatusUnauthorized, "授权失败: "+message)
		return
	}
	payload, fresh := a.auth.StateCookie(c)
	if !fresh {
		fail(c, http.StatusBadRequest, "登录状态已失效，请重新登录")
		return
	}

	redirect, sid, err := a.auth.CompleteLogin(c.Request.Context(), c.Query("code"), c.Query("state"), payload, c.ClientIP(), c.Request.UserAgent())
	a.auth.ClearStateCookie(c)
	if err != nil {
		if errors.Is(err, auth.ErrAccessDenied) {
			fail(c, http.StatusForbidden, a.cfg.Access.DenyMessage)
			return
		}
		slog.Error("完成登录失败", "error", err)
		fail(c, http.StatusUnauthorized, "登录失败，请重试")
		return
	}

	a.auth.SetSessionCookie(c, sid)
	c.Redirect(http.StatusFound, redirect)
}

func (a *API) handleLogout(c *gin.Context) {
	if sid, ok := auth.SessionID(c); ok {
		a.auth.Logout(c.Request.Context(), sid)
	}
	a.auth.ClearSessionCookie(c)
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "已退出"})
}

func (a *API) handleMe(c *gin.Context) {
	user, loggedIn := auth.User(c)
	if !loggedIn {
		ok(c, gin.H{
			"user":        nil,
			"groups":      []any{},
			"public_read": a.cfg.Server.PublicRead,
		})
		return
	}

	sid, _ := auth.SessionID(c)
	groups, err := a.store.GroupsByUser(c.Request.Context(), user.ID)
	if err != nil {
		slog.Error("读取用户组失败", "error", err)
	}

	list := make([]gin.H, 0, len(groups))
	for _, g := range groups {
		list = append(list, gin.H{"code": g.Code, "name": g.Name, "type": g.Type, "role": g.Role})
	}

	ok(c, gin.H{
		"user":        toUserView(user),
		"groups":      list,
		"csrf":        a.auth.CSRF(sid),
		"public_read": a.cfg.Server.PublicRead,
	})
}

func toUserView(user *store.User) userView {
	initial := "?"
	if runes := []rune(user.DisplayName); len(runes) > 0 {
		initial = string(runes[0])
	}
	return userView{
		ID:          user.ID,
		Subject:     user.Subject,
		Username:    user.Username,
		DisplayName: user.DisplayName,
		Email:       user.Email,
		Class:       user.Class,
		Role:        user.Role,
		Avatar:      initial,
	}
}
