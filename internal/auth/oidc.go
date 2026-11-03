package auth

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"

	"github.com/thun888/gdocs-go/internal/config"
)

type Claims struct {
	Sub               string   `json:"sub"`
	Nonce             string   `json:"nonce"`
	PreferredUsername string   `json:"preferred_username"`
	Name              string   `json:"name"`
	Email             string   `json:"email"`
	EmailVerified     bool     `json:"email_verified"`
	BelongClass       string   `json:"belong_class"`
	Groups            []string `json:"groups"`
}

type oidcClient struct {
	cfg config.OAuth

	mu       sync.Mutex
	oauth    *oauth2.Config
	verifier *oidc.IDTokenVerifier
	userinfo string
	revoke   string
}

var oidcHTTPClient = &http.Client{Timeout: 15 * time.Second}

func newOIDCClient(cfg config.OAuth) *oidcClient {
	return &oidcClient{cfg: cfg}
}

func (c *oidcClient) init(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.oauth != nil {
		return nil
	}

	provider, err := oidc.NewProvider(ctx, c.cfg.Issuer)
	if err != nil {
		return fmt.Errorf("发现 OIDC 配置: %w", err)
	}
	var meta struct {
		UserInfoEndpoint   string `json:"userinfo_endpoint"`
		RevocationEndpoint string `json:"revocation_endpoint"`
	}
	if err := provider.Claims(&meta); err != nil {
		return fmt.Errorf("读取 OIDC 元数据: %w", err)
	}
	revokeURL := meta.RevocationEndpoint
	if revokeURL == "" {
		revokeURL = strings.TrimSuffix(c.cfg.Issuer, "/") + "/oidc/revoke"
	}

	c.oauth = &oauth2.Config{
		ClientID:     c.cfg.ClientID,
		ClientSecret: c.cfg.ClientSecret,
		RedirectURL:  c.cfg.RedirectURL,
		Endpoint:     provider.Endpoint(),
		Scopes:       c.cfg.Scopes,
	}
	c.verifier = provider.Verifier(&oidc.Config{ClientID: c.cfg.ClientID})
	c.userinfo = meta.UserInfoEndpoint
	c.revoke = revokeURL
	return nil
}

func (c *oidcClient) authCodeURL(ctx context.Context, state, nonce, verifier string) (string, error) {
	if err := c.init(ctx); err != nil {
		return "", err
	}
	sum := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(sum[:])

	c.mu.Lock()
	conf := *c.oauth
	c.mu.Unlock()

	return conf.AuthCodeURL(state,
		oauth2.SetAuthURLParam("nonce", nonce),
		oauth2.SetAuthURLParam("code_challenge", challenge),
		oauth2.SetAuthURLParam("code_challenge_method", "S256"),
	), nil
}

func (c *oidcClient) exchange(ctx context.Context, code, verifier, nonce string) (*oauth2.Token, *Claims, error) {
	if err := c.init(ctx); err != nil {
		return nil, nil, err
	}

	c.mu.Lock()
	conf := *c.oauth
	verifierCheck := c.verifier
	c.mu.Unlock()

	token, err := conf.Exchange(ctx, code, oauth2.VerifierOption(verifier))
	if err != nil {
		return nil, nil, fmt.Errorf("换取令牌: %w", err)
	}

	rawID, _ := token.Extra("id_token").(string)
	if rawID == "" {
		return nil, nil, fmt.Errorf("响应缺少 id_token")
	}
	idToken, err := verifierCheck.Verify(ctx, rawID)
	if err != nil {
		return nil, nil, fmt.Errorf("校验 id_token: %w", err)
	}

	var claims Claims
	if err := idToken.Claims(&claims); err != nil {
		return nil, nil, fmt.Errorf("解析 id_token: %w", err)
	}
	if nonce != "" && claims.Nonce != nonce {
		return nil, nil, fmt.Errorf("nonce 不匹配")
	}
	return token, &claims, nil
}

func (c *oidcClient) userInfo(ctx context.Context, accessToken string) (*Claims, error) {
	if err := c.init(ctx); err != nil {
		return nil, err
	}
	c.mu.Lock()
	endpoint := c.userinfo
	c.mu.Unlock()
	if endpoint == "" {
		return nil, fmt.Errorf("上游未提供 userinfo 端点")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(accessToken))

	resp, err := oidcHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("userinfo 返回 %d", resp.StatusCode)
	}

	var claims Claims
	if err := json.NewDecoder(resp.Body).Decode(&claims); err != nil {
		return nil, err
	}
	return &claims, nil
}

func (c *oidcClient) refresh(ctx context.Context, refreshToken string) (*oauth2.Token, error) {
	if err := c.init(ctx); err != nil {
		return nil, err
	}
	c.mu.Lock()
	conf := *c.oauth
	c.mu.Unlock()

	src := conf.TokenSource(ctx, &oauth2.Token{RefreshToken: refreshToken})
	token, err := src.Token()
	if err != nil {
		return nil, err
	}
	return token, nil
}

func (c *oidcClient) revokeToken(ctx context.Context, refreshToken string) {
	if err := c.init(ctx); err != nil {
		return
	}
	c.mu.Lock()
	conf := *c.oauth
	endpoint := c.revoke
	c.mu.Unlock()
	if endpoint == "" || refreshToken == "" {
		return
	}

	form := url.Values{}
	form.Set("token", refreshToken)
	form.Set("token_type_hint", "refresh_token")
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(conf.ClientID, conf.ClientSecret)
	resp, err := oidcHTTPClient.Do(req)
	if err == nil {
		resp.Body.Close()
	}
}
