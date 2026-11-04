package auth

import (
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"net"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/thun888/gdocs-go/internal/config"
	"github.com/thun888/gdocs-go/internal/store"
)

type proxyIdentity struct {
	subject  string
	username string
	name     string
	email    string
	groups   []string
}

func (id proxyIdentity) groupKey() string {
	if len(id.groups) == 0 {
		return ""
	}
	sorted := append([]string(nil), id.groups...)
	sort.Strings(sorted)
	return strings.Join(sorted, "\x00")
}

type proxyEntry struct {
	user     *store.User
	groupKey string
	expires  time.Time
}

type proxyAuth struct {
	cfg       config.ProxyAuth
	access    config.Access
	bootstrap []string
	trusted   []*net.IPNet
	ttl       time.Duration
	store     *store.Store

	mu    sync.Mutex
	cache map[string]proxyEntry
	locks [64]sync.Mutex
}

func newProxyAuth(cfg *config.Config, st *store.Store) (*proxyAuth, error) {
	if !cfg.ProxyAuth.Enabled {
		return nil, nil
	}
	trusted, err := parseTrustedProxies(cfg.Server.TrustedProxies)
	if err != nil {
		return nil, err
	}
	ttl := time.Duration(cfg.ProxyAuth.CacheSeconds) * time.Second
	if ttl <= 0 {
		ttl = time.Minute
	}
	return &proxyAuth{
		cfg:       cfg.ProxyAuth,
		access:    cfg.Access,
		bootstrap: cfg.OAuth.BootstrapAdmin,
		trusted:   trusted,
		ttl:       ttl,
		store:     st,
		cache:     make(map[string]proxyEntry),
	}, nil
}

func parseTrustedProxies(items []string) ([]*net.IPNet, error) {
	out := make([]*net.IPNet, 0, len(items))
	for _, raw := range items {
		value := strings.TrimSpace(raw)
		if value == "" {
			continue
		}
		if strings.Contains(value, "/") {
			_, network, err := net.ParseCIDR(value)
			if err != nil {
				return nil, fmt.Errorf("server.trusted_proxies 无法解析 %q", raw)
			}
			out = append(out, network)
			continue
		}
		ip := net.ParseIP(value)
		if ip == nil {
			return nil, fmt.Errorf("server.trusted_proxies 无法解析 %q", raw)
		}
		bits := 128
		if ip.To4() != nil {
			bits = 32
		}
		out = append(out, &net.IPNet{IP: ip, Mask: net.CIDRMask(bits, bits)})
	}
	return out, nil
}

func (p *proxyAuth) isTrusted(remoteAddr string) bool {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		host = remoteAddr
	}
	ip := net.ParseIP(strings.Trim(host, "[]"))
	if ip == nil {
		return false
	}
	for _, network := range p.trusted {
		if network.Contains(ip) {
			return true
		}
	}
	return false
}

func (p *proxyAuth) identify(c *gin.Context) (proxyIdentity, bool) {
	if p == nil || !p.isTrusted(c.Request.RemoteAddr) {
		return proxyIdentity{}, false
	}
	username := headerValue(c, p.cfg.UserHeader)
	if username == "" {
		return proxyIdentity{}, false
	}
	subject := headerValue(c, p.cfg.SubHeader)
	if subject == "" {
		subject = username
	}
	return proxyIdentity{
		subject:  clip(subject, 255),
		username: clip(username, 64),
		name:     clip(headerValue(c, p.cfg.NameHeader), 128),
		email:    clip(headerValue(c, p.cfg.EmailHeader), 255),
		groups:   splitGroups(headerValue(c, p.cfg.GroupsHeader)),
	}, true
}

func (p *proxyAuth) resolve(ctx context.Context, id proxyIdentity) (*store.User, error) {
	groupKey := id.groupKey()
	lock := &p.locks[lockIndex(id.subject)]
	lock.Lock()
	defer lock.Unlock()

	if user := p.cached(id.subject, groupKey); user != nil {
		return user, nil
	}

	existing, err := p.store.UserBySubject(ctx, id.subject)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("读取用户: %w", err)
	}

	user := &store.User{
		Subject:     id.subject,
		Username:    id.username,
		DisplayName: id.name,
		Email:       id.email,
		Role:        "viewer",
		CreatedAt:   time.Now(),
	}
	if existing != nil {
		user.Class = existing.Class
		user.Role = existing.Role
		if user.DisplayName == "" {
			user.DisplayName = existing.DisplayName
		}
		if user.Email == "" {
			user.Email = existing.Email
		}
	}
	if user.DisplayName == "" {
		user.DisplayName = user.Username
	}
	if err := p.store.UpsertUser(ctx, user); err != nil {
		return nil, fmt.Errorf("写入用户: %w", err)
	}
	stored, err := p.store.UserBySubject(ctx, id.subject)
	if err != nil {
		return nil, fmt.Errorf("读取用户: %w", err)
	}

	bootstrap := IsBootstrapAdmin(stored.Subject, p.bootstrap)
	if bootstrap && stored.Role != "admin" {
		if err := p.store.UpdateUserRole(ctx, stored.ID, "admin"); err != nil {
			return nil, fmt.Errorf("更新角色: %w", err)
		}
		stored.Role = "admin"
	}

	if len(id.groups) > 0 {
		groups, err := p.store.SyncGroups(ctx, stored.ID, groupSpecs(id.groups))
		if err != nil {
			return nil, fmt.Errorf("同步用户组: %w", err)
		}
		role := RoleFromGroups(groups)
		if bootstrap {
			role = "admin"
		}
		if role != stored.Role {
			if err := p.store.UpdateUserRole(ctx, stored.ID, role); err != nil {
				return nil, fmt.Errorf("更新角色: %w", err)
			}
			stored.Role = role
		}
		if !bootstrap && !GroupAllowed(id.groups, p.access.AllowedGroups) {
			return nil, ErrAccessDenied
		}
	}

	_ = p.store.TouchLogin(ctx, stored.ID, time.Now())
	p.remember(id.subject, groupKey, stored)
	return stored, nil
}

func (p *proxyAuth) cached(subject, groupKey string) *store.User {
	p.mu.Lock()
	defer p.mu.Unlock()
	entry, ok := p.cache[subject]
	if !ok || entry.groupKey != groupKey || time.Now().After(entry.expires) {
		return nil
	}
	user := *entry.user
	return &user
}

func (p *proxyAuth) remember(subject, groupKey string, user *store.User) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.cache) > 4096 {
		now := time.Now()
		for key, entry := range p.cache {
			if now.After(entry.expires) {
				delete(p.cache, key)
			}
		}
	}
	p.cache[subject] = proxyEntry{user: user, groupKey: groupKey, expires: time.Now().Add(p.ttl)}
}

func (p *proxyAuth) invalidate() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.cache = make(map[string]proxyEntry)
}

func lockIndex(key string) uint32 {
	h := fnv.New32a()
	_, _ = h.Write([]byte(key))
	return h.Sum32() % 64
}

func headerValue(c *gin.Context, name string) string {
	if name == "" {
		return ""
	}
	return strings.TrimSpace(c.GetHeader(name))
}

func splitGroups(raw string) []string {
	if raw == "" {
		return nil
	}
	seen := make(map[string]bool)
	out := make([]string, 0, 4)
	for _, part := range strings.Split(raw, ",") {
		code := clip(strings.TrimSpace(part), 128)
		if code == "" || seen[code] {
			continue
		}
		seen[code] = true
		out = append(out, code)
	}
	return out
}

func clip(value string, max int) string {
	if utf8.RuneCountInString(value) <= max {
		return value
	}
	return string([]rune(value)[:max])
}
