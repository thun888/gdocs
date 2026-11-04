package config

import (
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server    Server    `yaml:"server"`
	Log       Log       `yaml:"log"`
	Database  Database  `yaml:"database"`
	OAuth     OAuth     `yaml:"oauth"`
	ProxyAuth ProxyAuth `yaml:"proxy_auth"`
	Access    Access    `yaml:"access"`
	Git       Git       `yaml:"git"`
	Index     Index     `yaml:"index"`
	S3        S3        `yaml:"s3"`
	Sources   []Source  `yaml:"sources"`
}

type Log struct {
	Debug  bool   `yaml:"debug"`
	Access bool   `yaml:"access"`
	Format string `yaml:"format"`
}

type Server struct {
	Addr           string   `yaml:"addr"`
	BaseURL        string   `yaml:"base_url"`
	SessionKey     string   `yaml:"session_key"`
	PublicRead     bool     `yaml:"public_read"`
	TrustedProxies []string `yaml:"trusted_proxies"`
}

type Database struct {
	DSN         string `yaml:"dsn"`
	MaxOpen     int    `yaml:"max_open"`
	MaxIdle     int    `yaml:"max_idle"`
	AutoMigrate bool   `yaml:"auto_migrate"`
}

type OAuth struct {
	Issuer         string   `yaml:"issuer"`
	ClientID       string   `yaml:"client_id"`
	ClientSecret   string   `yaml:"client_secret"`
	RedirectURL    string   `yaml:"redirect_url"`
	Scopes         []string `yaml:"scopes"`
	GroupsClaim    string   `yaml:"groups_claim"`
	BootstrapAdmin []string `yaml:"bootstrap_admin"`
}

type ProxyAuth struct {
	Enabled      bool   `yaml:"enabled"`
	UserHeader   string `yaml:"user_header"`
	SubHeader    string `yaml:"sub_header"`
	EmailHeader  string `yaml:"email_header"`
	NameHeader   string `yaml:"name_header"`
	GroupsHeader string `yaml:"groups_header"`
	LoginURL     string `yaml:"login_url"`
	LogoutURL    string `yaml:"logout_url"`
	CacheSeconds int    `yaml:"cache_seconds"`
}

type Access struct {
	AllowedGroups []string `yaml:"allowed_groups"`
	DenyMessage   string   `yaml:"deny_message"`
}

type Git struct {
	BotName  string `yaml:"bot_name"`
	BotEmail string `yaml:"bot_email"`
}

type Index struct {
	TTLMinutes   int  `yaml:"ttl_minutes"`
	SyncOnWrite  bool `yaml:"sync_on_write"`
	MaxArchiveMB int  `yaml:"max_archive_mb"`
}

type S3 struct {
	Endpoint   string   `yaml:"endpoint"`
	Region     string   `yaml:"region"`
	Bucket     string   `yaml:"bucket"`
	AccessKey  string   `yaml:"access_key"`
	SecretKey  string   `yaml:"secret_key"`
	PathPrefix string   `yaml:"path_prefix"`
	PublicBase string   `yaml:"public_base"`
	MaxSizeMB  int      `yaml:"max_size_mb"`
	AllowTypes []string `yaml:"allow_types"`
}

type Source struct {
	Slug    string `yaml:"slug"`
	Name    string `yaml:"name"`
	BaseURL string `yaml:"base_url"`
	Token   string `yaml:"token"`
	Owner   string `yaml:"owner"`
	Repo    string `yaml:"repo"`
	Branch  string `yaml:"branch"`
	Root    string `yaml:"root"`
	Sort    int    `yaml:"sort"`
}

var (
	envPattern  = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)\}`)
	slugPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)
)

func Load(path string) (*Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取配置 %s: %w", path, err)
	}
	expanded, err := expandEnv(string(raw))
	if err != nil {
		return nil, err
	}

	var cfg Config
	// yaml 只覆盖文档中出现的字段，未配置 log.access 时保持默认开启
	cfg.Log.Access = true
	dec := yaml.NewDecoder(strings.NewReader(expanded))
	dec.KnownFields(true)
	if err := dec.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("解析配置 %s: %w", path, err)
	}
	cfg.applyDefaults()
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func expandEnv(raw string) (string, error) {
	var missing []string
	out := envPattern.ReplaceAllStringFunc(raw, func(match string) string {
		name := match[2 : len(match)-1]
		value, ok := os.LookupEnv(name)
		if !ok {
			missing = append(missing, name)
		}
		return value
	})
	if len(missing) > 0 {
		return "", fmt.Errorf("环境变量未设置: %s", strings.Join(missing, ", "))
	}
	return out, nil
}

func (c *Config) applyDefaults() {
	if c.Server.Addr == "" {
		c.Server.Addr = ":8080"
	}
	if c.Log.Format == "" {
		c.Log.Format = "text"
	}
	if c.Database.MaxOpen == 0 {
		c.Database.MaxOpen = 20
	}
	if c.Database.MaxIdle == 0 {
		c.Database.MaxIdle = 5
	}
	if len(c.OAuth.Scopes) == 0 {
		c.OAuth.Scopes = []string{"openid", "profile", "email", "groups"}
	}
	if c.OAuth.GroupsClaim == "" {
		c.OAuth.GroupsClaim = "groups"
	}
	if c.ProxyAuth.UserHeader == "" {
		c.ProxyAuth.UserHeader = "Remote-User"
	}
	if c.ProxyAuth.SubHeader == "" {
		c.ProxyAuth.SubHeader = "Remote-Sub"
	}
	if c.ProxyAuth.EmailHeader == "" {
		c.ProxyAuth.EmailHeader = "Remote-Email"
	}
	if c.ProxyAuth.NameHeader == "" {
		c.ProxyAuth.NameHeader = "Remote-Name"
	}
	if c.ProxyAuth.GroupsHeader == "" {
		c.ProxyAuth.GroupsHeader = "Remote-Groups"
	}
	if c.ProxyAuth.CacheSeconds <= 0 {
		c.ProxyAuth.CacheSeconds = 60
	}
	if c.Index.TTLMinutes == 0 {
		c.Index.TTLMinutes = 10
	}
	if c.Index.MaxArchiveMB == 0 {
		c.Index.MaxArchiveMB = 50
	}
	for i := range c.Sources {
		if c.Sources[i].Branch == "" {
			c.Sources[i].Branch = "main"
		}
	}
}

func (c *Config) validate() error {
	var problems []string

	if len(c.Server.SessionKey) < 32 {
		problems = append(problems, "server.session_key 至少 32 字节")
	}
	if c.Database.DSN == "" {
		problems = append(problems, "database.dsn 不能为空")
	}
	if c.ProxyAuth.Enabled && len(c.Server.TrustedProxies) == 0 {
		problems = append(problems, "启用 proxy_auth 时 server.trusted_proxies 不能为空")
	}
	if !c.ProxyAuth.Enabled || c.ProxyAuth.LoginURL == "" {
		if c.OAuth.Issuer == "" {
			problems = append(problems, "oauth.issuer 不能为空")
		}
		if c.OAuth.ClientID == "" || c.OAuth.ClientSecret == "" {
			problems = append(problems, "oauth.client_id 与 client_secret 不能为空")
		}
		if c.OAuth.RedirectURL == "" {
			problems = append(problems, "oauth.redirect_url 不能为空")
		} else if u, err := url.Parse(c.OAuth.RedirectURL); err != nil || u.Scheme == "" || u.Host == "" {
			problems = append(problems, "oauth.redirect_url 必须是绝对地址")
		}
	}
	if c.Git.BotEmail == "" {
		problems = append(problems, "git.bot_email 不能为空")
	}
	if c.Log.Format != "text" && c.Log.Format != "json" {
		problems = append(problems, "log.format 只能是 text 或 json")
	}

	seen := make(map[string]bool, len(c.Sources))
	for i, s := range c.Sources {
		prefix := fmt.Sprintf("sources[%d]", i)
		if !slugPattern.MatchString(s.Slug) {
			problems = append(problems, prefix+".slug 只能是小写字母、数字与连字符")
		}
		if seen[s.Slug] {
			problems = append(problems, prefix+".slug 重复: "+s.Slug)
		}
		seen[s.Slug] = true
		if s.Name == "" || s.BaseURL == "" || s.Token == "" || s.Owner == "" || s.Repo == "" {
			problems = append(problems, prefix+" 缺少必填项（name/base_url/token/owner/repo）")
		}
	}

	if len(problems) > 0 {
		return fmt.Errorf("配置校验失败:\n  - %s", strings.Join(problems, "\n  - "))
	}
	return nil
}
