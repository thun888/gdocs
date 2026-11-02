package auth

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/thun888/zdocs-go/internal/config"
	"github.com/thun888/zdocs-go/internal/cryptox"
	"github.com/thun888/zdocs-go/internal/store"
)

var errSessionExpired = errors.New("会话已过期")

const (
	sessionCookie = "zdocs_session"
	stateCookie   = "zdocs_state"
	sessionTTL    = 7 * 24 * time.Hour
)

type sessionManager struct {
	store  *store.Store
	box    *cryptox.Box
	key    []byte
	secure bool
}

func newSessionManager(cfg *config.Config, st *store.Store) (*sessionManager, error) {
	box, err := cryptox.NewBox(cfg.Server.SessionKey)
	if err != nil {
		return nil, err
	}
	return &sessionManager{
		store:  st,
		box:    box,
		key:    []byte(cfg.Server.SessionKey),
		secure: len(cfg.Server.BaseURL) > 8 && cfg.Server.BaseURL[:8] == "https://",
	}, nil
}

func (m *sessionManager) create(ctx context.Context, uid uint64, access, refresh string, tokenExp time.Time, ip, ua string) (string, error) {
	id, err := randomToken(32)
	if err != nil {
		return "", err
	}
	accessEnc, err := m.box.Seal([]byte(access))
	if err != nil {
		return "", err
	}
	refreshEnc, err := m.box.Seal([]byte(refresh))
	if err != nil {
		return "", err
	}

	now := time.Now()
	sess := &store.Session{
		ID:              id,
		UID:             uid,
		AccessTokenEnc:  accessEnc,
		RefreshTokenEnc: refreshEnc,
		TokenExpiresAt:  tokenExp,
		ExpiresAt:       now.Add(sessionTTL),
		LastSeenAt:      now,
		IP:              ip,
		UA:              ua,
	}
	if err := m.store.CreateSession(ctx, sess); err != nil {
		return "", err
	}
	return id, nil
}

func (m *sessionManager) load(ctx context.Context, id string) (*store.Session, error) {
	sess, err := m.store.SessionByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if time.Now().After(sess.ExpiresAt) {
		_ = m.store.DeleteSession(ctx, sess.ID)
		return nil, errSessionExpired
	}
	return sess, nil
}

func (m *sessionManager) accessToken(sess *store.Session) (string, error) {
	plain, err := m.box.Open(sess.AccessTokenEnc)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

func (m *sessionManager) refreshToken(sess *store.Session) (string, error) {
	plain, err := m.box.Open(sess.RefreshTokenEnc)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

func (m *sessionManager) saveTokens(ctx context.Context, sess *store.Session, access, refresh string, tokenExp time.Time) error {
	accessEnc, err := m.box.Seal([]byte(access))
	if err != nil {
		return err
	}
	refreshEnc, err := m.box.Seal([]byte(refresh))
	if err != nil {
		return err
	}
	now := time.Now()
	sess.AccessTokenEnc, sess.RefreshTokenEnc = accessEnc, refreshEnc
	sess.TokenExpiresAt = tokenExp
	return m.store.UpdateSessionTokens(ctx, sess.ID, accessEnc, refreshEnc, tokenExp, now)
}

func (m *sessionManager) destroy(ctx context.Context, sess *store.Session) (string, bool) {
	refresh, err := m.refreshToken(sess)
	_ = m.store.DeleteSession(ctx, sess.ID)
	if err != nil {
		return "", false
	}
	return refresh, true
}

func (m *sessionManager) setCookie(c *gin.Context, id string) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(sessionCookie, signValue(m.key, []byte(id)), int(sessionTTL.Seconds()), "/", "", m.secure, true)
}

func (m *sessionManager) clearCookie(c *gin.Context) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(sessionCookie, "", -1, "/", "", m.secure, true)
}

func (m *sessionManager) sessionID(c *gin.Context) (string, bool) {
	raw, err := c.Cookie(sessionCookie)
	if err != nil || raw == "" {
		return "", false
	}
	payload, err := verifyValue(m.key, raw)
	if err != nil {
		return "", false
	}
	return string(payload), true
}

func (m *sessionManager) csrfToken(sid string) string {
	return deriveToken(m.key, "csrf", sid)
}

func (m *sessionManager) stateValue(payload []byte) string {
	return signValue(m.key, payload)
}

func (m *sessionManager) stateFromCookie(c *gin.Context) ([]byte, bool) {
	raw, err := c.Cookie(stateCookie)
	if err != nil || raw == "" {
		return nil, false
	}
	payload, err := verifyValue(m.key, raw)
	if err != nil {
		return nil, false
	}
	return payload, true
}

func (m *sessionManager) setStateCookie(c *gin.Context, payload []byte) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(stateCookie, m.stateValue(payload), 600, "/", "", m.secure, true)
}

func (m *sessionManager) clearStateCookie(c *gin.Context) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(stateCookie, "", -1, "/", "", m.secure, true)
}
