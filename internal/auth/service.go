package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/thun888/gdocs-go/internal/config"
	"github.com/thun888/gdocs-go/internal/store"
)

var (
	ErrUnauthorized = errors.New("未登录")
	ErrAccessDenied = errors.New("不在访问范围内")
)

type statePayload struct {
	State    string `json:"state"`
	Nonce    string `json:"nonce"`
	Verifier string `json:"verifier"`
	Redirect string `json:"redirect"`
	Exp      int64  `json:"exp"`
}

type Service struct {
	cfg   *config.Config
	store *store.Store
	oidc  *oidcClient
	sess  *sessionManager
	proxy *proxyAuth

	refreshLocks sync.Map
}

func New(cfg *config.Config, st *store.Store) (*Service, error) {
	sess, err := newSessionManager(cfg, st)
	if err != nil {
		return nil, err
	}
	proxy, err := newProxyAuth(cfg, st)
	if err != nil {
		return nil, err
	}
	return &Service{
		cfg:   cfg,
		store: st,
		oidc:  newOIDCClient(cfg.OAuth),
		sess:  sess,
		proxy: proxy,
	}, nil
}

func (s *Service) Session() *sessionManager { return s.sess }

func (s *Service) BeginLogin(ctx context.Context, redirect string) (string, []byte, error) {
	if redirect == "" || !strings.HasPrefix(redirect, "/") || strings.HasPrefix(redirect, "//") {
		redirect = "/"
	}
	state, err := randomToken(24)
	if err != nil {
		return "", nil, err
	}
	nonce, err := randomToken(24)
	if err != nil {
		return "", nil, err
	}
	verifier, err := randomToken(48)
	if err != nil {
		return "", nil, err
	}

	payload, err := json.Marshal(statePayload{
		State:    state,
		Nonce:    nonce,
		Verifier: verifier,
		Redirect: redirect,
		Exp:      time.Now().Add(10 * time.Minute).Unix(),
	})
	if err != nil {
		return "", nil, err
	}

	url, err := s.oidc.authCodeURL(ctx, state, nonce, verifier)
	if err != nil {
		return "", nil, err
	}
	return url, payload, nil
}

func (s *Service) CompleteLogin(ctx context.Context, code, state string, stateRaw []byte, ip, ua string) (string, string, error) {
	var payload statePayload
	if err := json.Unmarshal(stateRaw, &payload); err != nil {
		return "", "", fmt.Errorf("解析登录状态: %w", err)
	}
	if payload.State == "" || payload.State != state {
		return "", "", fmt.Errorf("state 不匹配")
	}
	if time.Now().Unix() > payload.Exp {
		return "", "", fmt.Errorf("登录状态已过期")
	}

	token, claims, err := s.oidc.exchange(ctx, code, payload.Verifier, payload.Nonce)
	if err != nil {
		return "", "", err
	}
	if fresh, err := s.oidc.userInfo(ctx, token.AccessToken); err == nil {
		if fresh.Sub != "" && fresh.Sub == claims.Sub {
			claims = fresh
		}
	}

	user, err := s.persist(ctx, claims)
	if err != nil {
		return "", "", err
	}

	refresh := token.RefreshToken
	sid, err := s.sess.create(ctx, user.ID, token.AccessToken, refresh, token.Expiry, ip, ua)
	if err != nil {
		return "", "", err
	}
	return payload.Redirect, sid, nil
}

func (s *Service) persist(ctx context.Context, claims *Claims) (*store.User, error) {
	if claims.Sub == "" {
		return nil, fmt.Errorf("上游未返回 sub")
	}

	user := &store.User{
		Subject:     claims.Sub,
		Username:    firstNonEmpty(claims.PreferredUsername, claims.Sub),
		DisplayName: firstNonEmpty(claims.Name, claims.PreferredUsername, claims.Sub),
		Email:       claims.Email,
		Class:       claims.BelongClass,
		Role:        "viewer",
		CreatedAt:   time.Now(),
	}
	if err := s.store.UpsertUser(ctx, user); err != nil {
		return nil, fmt.Errorf("写入用户: %w", err)
	}
	stored, err := s.store.UserBySubject(ctx, claims.Sub)
	if err != nil {
		return nil, fmt.Errorf("读取用户: %w", err)
	}

	if _, err := s.syncIdentity(ctx, stored, claims); err != nil {
		return nil, err
	}
	_ = s.store.TouchLogin(ctx, stored.ID, time.Now())
	return stored, nil
}

func (s *Service) syncIdentity(ctx context.Context, user *store.User, claims *Claims) (string, error) {
	groups, err := s.store.SyncGroups(ctx, user.ID, groupSpecs(claims.Groups))
	if err != nil {
		return "", fmt.Errorf("同步用户组: %w", err)
	}

	bootstrap := IsBootstrapAdmin(user.Subject, s.cfg.OAuth.BootstrapAdmin)
	role := RoleFromGroups(groups)
	if bootstrap {
		role = "admin"
	}
	if role != user.Role {
		if err := s.store.UpdateUserRole(ctx, user.ID, role); err != nil {
			return "", fmt.Errorf("更新角色: %w", err)
		}
		user.Role = role
	}
	if !bootstrap && !GroupAllowed(claims.Groups, s.cfg.Access.AllowedGroups) {
		return role, ErrAccessDenied
	}
	return role, nil
}

func (s *Service) CurrentUser(ctx context.Context, sid string) (*store.User, *store.Session, error) {
	sess, err := s.sess.load(ctx, sid)
	if err != nil {
		return nil, nil, ErrUnauthorized
	}
	user, err := s.store.UserByID(ctx, sess.UID)
	if err != nil {
		_ = s.store.DeleteSession(ctx, sess.ID)
		return nil, nil, ErrUnauthorized
	}

	if time.Until(sess.TokenExpiresAt) > 5*time.Minute {
		if time.Since(sess.LastSeenAt) > time.Minute {
			_ = s.store.TouchSession(ctx, sess.ID, time.Now())
		}
		return user, sess, nil
	}

	lock := s.sessionLock(sess.ID)
	lock.Lock()
	defer lock.Unlock()

	if err := s.refreshTokens(ctx, user, sess); err != nil {
		_, _ = s.sess.destroy(ctx, sess)
		return nil, nil, ErrUnauthorized
	}
	return user, sess, nil
}

func (s *Service) refreshTokens(ctx context.Context, user *store.User, sess *store.Session) error {
	refresh, err := s.sess.refreshToken(sess)
	if err != nil || refresh == "" {
		return fmt.Errorf("缺少刷新令牌")
	}

	token, err := s.oidc.refresh(ctx, refresh)
	if err != nil {
		return err
	}

	newRefresh := token.RefreshToken
	if newRefresh == "" {
		newRefresh = refresh
	}

	claims, err := s.oidc.userInfo(ctx, token.AccessToken)
	if err != nil {
		return s.sess.saveTokens(ctx, sess, token.AccessToken, newRefresh, token.Expiry)
	}
	if claims.Sub == "" || claims.Sub != user.Subject {
		return fmt.Errorf("身份不匹配")
	}
	if _, err := s.syncIdentity(ctx, user, claims); err != nil {
		return err
	}
	return s.sess.saveTokens(ctx, sess, token.AccessToken, newRefresh, token.Expiry)
}

func (s *Service) Logout(ctx context.Context, sid string) {
	sess, err := s.sess.load(ctx, sid)
	if err != nil {
		return
	}
	if refresh, ok := s.sess.destroy(ctx, sess); ok {
		s.oidc.revokeToken(ctx, refresh)
	}
}

func (s *Service) CSRFToken(c *gin.Context) string {
	if sid, ok := SessionID(c); ok {
		return s.sess.csrfToken(sid)
	}
	if user, ok := User(c); ok {
		return s.sess.csrfProxyToken(user.Subject)
	}
	return ""
}

func (s *Service) ProxyLoginURL(redirect string) string {
	if s.proxy == nil || s.cfg.ProxyAuth.LoginURL == "" {
		return ""
	}
	if redirect == "" || !strings.HasPrefix(redirect, "/") || strings.HasPrefix(redirect, "//") {
		redirect = "/"
	}
	target := redirect
	if base := strings.TrimSuffix(s.cfg.Server.BaseURL, "/"); base != "" {
		target = base + redirect
	}
	return strings.ReplaceAll(s.cfg.ProxyAuth.LoginURL, "{redirect}", url.QueryEscape(target))
}

func (s *Service) ProxyLogoutURL() string {
	if s.proxy == nil {
		return ""
	}
	return s.cfg.ProxyAuth.LogoutURL
}

func (s *Service) InvalidateProxyCache() {
	if s.proxy != nil {
		s.proxy.invalidate()
	}
}

func (s *Service) denyMessage() string {
	if message := strings.TrimSpace(s.cfg.Access.DenyMessage); message != "" {
		return message
	}
	return "当前账号不在访问范围内"
}

func (s *Service) SetSessionCookie(c *gin.Context, sid string) { s.sess.setCookie(c, sid) }

func (s *Service) ClearSessionCookie(c *gin.Context) { s.sess.clearCookie(c) }

func (s *Service) SetStateCookie(c *gin.Context, payload []byte) { s.sess.setStateCookie(c, payload) }

func (s *Service) StateCookie(c *gin.Context) ([]byte, bool) { return s.sess.stateFromCookie(c) }

func (s *Service) ClearStateCookie(c *gin.Context) { s.sess.clearStateCookie(c) }

func (s *Service) sessionLock(sid string) *sync.Mutex {
	value, _ := s.refreshLocks.LoadOrStore(sid, &sync.Mutex{})
	return value.(*sync.Mutex)
}

func groupSpecs(codes []string) []store.GroupSpec {
	specs := make([]store.GroupSpec, 0, len(codes))
	for _, code := range codes {
		if strings.TrimSpace(code) == "" {
			continue
		}
		typ, name := ParseGroupCode(code)
		specs = append(specs, store.GroupSpec{Code: code, Name: name, Type: typ})
	}
	return specs
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
