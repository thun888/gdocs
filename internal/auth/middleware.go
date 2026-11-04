package auth

import (
	"crypto/subtle"
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/thun888/gdocs-go/internal/store"
)

const (
	ctxUser = "gdocs.user"
	ctxSID  = "gdocs.sid"
)

func abort(c *gin.Context, status int, message string) {
	c.AbortWithStatusJSON(status, gin.H{"code": status, "message": message})
}

func User(c *gin.Context) (*store.User, bool) {
	value, ok := c.Get(ctxUser)
	if !ok {
		return nil, false
	}
	user, ok := value.(*store.User)
	return user, ok
}

func SessionID(c *gin.Context) (string, bool) {
	value, ok := c.Get(ctxSID)
	if !ok {
		return "", false
	}
	sid, ok := value.(string)
	return sid, ok
}

func (s *Service) Optional() gin.HandlerFunc {
	return func(c *gin.Context) {
		if s.proxy != nil {
			if id, ok := s.proxy.identify(c); ok {
				user, err := s.proxy.resolve(c.Request.Context(), id)
				if errors.Is(err, ErrAccessDenied) {
					abort(c, http.StatusForbidden, s.denyMessage())
					return
				}
				if err != nil {
					slog.Error("代理身份认证失败", "error", err, "subject", id.subject)
					abort(c, http.StatusInternalServerError, "认证失败")
					return
				}
				c.Set(ctxUser, user)
				c.Next()
				return
			}
		}

		sid, ok := s.sess.sessionID(c)
		if !ok {
			c.Next()
			return
		}
		user, _, err := s.CurrentUser(c.Request.Context(), sid)
		if err != nil {
			s.sess.clearCookie(c)
			c.Next()
			return
		}
		c.Set(ctxUser, user)
		c.Set(ctxSID, sid)
		c.Next()
	}
}

func (s *Service) RequireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		if _, ok := User(c); !ok {
			abort(c, http.StatusUnauthorized, "未登录")
			return
		}
		c.Next()
	}
}

func (s *Service) requireRole(role string) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, ok := User(c)
		if !ok {
			abort(c, http.StatusUnauthorized, "未登录")
			return
		}
		if !roleAtLeast(user.Role, role) {
			abort(c, http.StatusForbidden, "权限不足")
			return
		}
		c.Next()
	}
}

func (s *Service) RequireEditor() gin.HandlerFunc { return s.requireRole("editor") }

func (s *Service) RequireAdmin() gin.HandlerFunc { return s.requireRole("admin") }

func (s *Service) VerifyCSRF() gin.HandlerFunc {
	return func(c *gin.Context) {
		switch c.Request.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			c.Next()
			return
		}
		want := s.CSRFToken(c)
		if want == "" {
			abort(c, http.StatusUnauthorized, "未登录")
			return
		}
		header := c.GetHeader("X-CSRF-Token")
		if header == "" || subtle.ConstantTimeCompare([]byte(header), []byte(want)) != 1 {
			abort(c, http.StatusForbidden, "CSRF 校验失败")
			return
		}
		c.Next()
	}
}
