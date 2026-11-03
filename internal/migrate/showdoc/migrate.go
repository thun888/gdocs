package showdoc

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/thun888/gdocs-go/internal/config"
	"github.com/thun888/gdocs-go/internal/cryptox"
	"github.com/thun888/gdocs-go/internal/doc"
	"github.com/thun888/gdocs-go/internal/storage"
	"github.com/thun888/gdocs-go/internal/store"
)

type RunOptions struct {
	Config        *config.Config
	DB            *DB
	SourceSlug    string
	SourceName    string
	UploadsDir    string
	WorkDir       string
	History       bool
	Push          bool
	Reset         bool
	Sync          bool
	DryRun        bool
	Verbose       bool
	KeepWorkDir   bool
	ClearGitProxy bool
}

func Run(ctx context.Context, opt RunOptions) error {
	src, ok := sourceConfig(opt.Config, opt.SourceSlug)
	if !ok {
		return fmt.Errorf("配置中找不到来源 %s", opt.SourceSlug)
	}
	if opt.SourceSlug == "" {
		opt.SourceSlug = src.Slug
	}

	items, err := opt.DB.Items(ctx)
	if err != nil {
		return fmt.Errorf("读取项目: %w", err)
	}
	catalogs, err := opt.DB.Catalogs(ctx)
	if err != nil {
		return fmt.Errorf("读取目录: %w", err)
	}
	pages, err := opt.DB.Pages(ctx)
	if err != nil {
		return fmt.Errorf("读取页面: %w", err)
	}
	histories, err := opt.DB.Histories(ctx)
	if err != nil {
		return fmt.Errorf("读取历史版本: %w", err)
	}
	users, err := opt.DB.Users(ctx)
	if err != nil {
		return fmt.Errorf("读取用户: %w", err)
	}
	uploads, err := opt.DB.Uploads(ctx)
	if err != nil {
		return fmt.Errorf("读取附件: %w", err)
	}

	plan := BuildPlan(items, catalogs, pages, histories, users, Options{
		BotName: opt.Config.Git.BotName,
		History: opt.History,
	})
	printPlan(plan)
	if opt.Verbose {
		printPlanDetail(plan)
	}
	if opt.DryRun {
		return nil
	}

	resolver := NewResolver(attachmentHost(uploads))
	if opt.UploadsDir != "" {
		if opt.Config.S3.Endpoint == "" || opt.Config.S3.Bucket == "" {
			return fmt.Errorf("配置中未启用 S3，无法迁移附件")
		}
		uploader, err := storage.New(opt.Config.S3)
		if err != nil {
			return fmt.Errorf("初始化对象存储: %w", err)
		}
		if err := resolver.Upload(ctx, uploader, uploads, opt.UploadsDir); err != nil {
			return fmt.Errorf("上传附件: %w", err)
		}
	}

	events := buildEvents(plan, resolver)
	if opt.UploadsDir != "" {
		slog.Info(resolver.Summary())
	}
	slog.Info("准备写入仓库", "commits", len(events))

	remote, err := remoteURL(src)
	if err != nil {
		return err
	}

	base := opt.WorkDir
	if base == "" {
		base = os.TempDir()
	}
	workDir, err := os.MkdirTemp(base, "gdocs-migrate-*")
	if err != nil {
		return fmt.Errorf("创建临时目录: %w", err)
	}
	if opt.KeepWorkDir {
		slog.Info("保留临时仓库", "path", workDir)
	} else {
		defer os.RemoveAll(workDir)
	}

	git := NewGit(workDir, opt.ClearGitProxy)
	exists, err := git.RemoteHasBranch(ctx, remote, src.Branch)
	if err != nil {
		return fmt.Errorf("检查远端仓库: %w", err)
	}
	if exists && !opt.Reset {
		return fmt.Errorf("远端分支 %s 已有提交，确认覆盖请加 -reset", src.Branch)
	}

	if err := git.Init(ctx, opt.Config.Git.BotName, opt.Config.Git.BotEmail); err != nil {
		return fmt.Errorf("初始化本地仓库: %w", err)
	}

	for i, event := range events {
		if err := git.CommitFile(ctx, event.Path, event.Content, event.Author, opt.Config.Git.BotEmail, event.At, event.Message); err != nil {
			return fmt.Errorf("提交 %s: %w", event.Path, err)
		}
		if (i+1)%50 == 0 {
			slog.Info("提交进度", "done", i+1, "total", len(events))
		}
	}

	if err := commitMeta(ctx, git, plan.Meta, opt.Config.Git.BotName, opt.Config.Git.BotEmail); err != nil {
		return fmt.Errorf("写入排序文件: %w", err)
	}

	if !opt.Push {
		slog.Info("未推送，本地仓库保留在", "path", workDir)
		return nil
	}
	if err := git.Push(ctx, remote, src.Branch, opt.Reset); err != nil {
		return fmt.Errorf("推送仓库: %w", err)
	}
	slog.Info("仓库已推送", "repo", src.Owner+"/"+src.Repo, "branch", src.Branch)

	if opt.Sync {
		if err := syncIndex(ctx, opt.Config, opt.SourceSlug, opt.SourceName); err != nil {
			return fmt.Errorf("内容已推送，但索引重建失败: %w", err)
		}
	}
	return nil
}

type event struct {
	PageID   int64
	Path     string
	Content  []byte
	Author   string
	Message  string
	At       time.Time
	sequence int
}

func buildEvents(plan *Plan, resolver *Resolver) []event {
	var events []event
	for pageIndex, page := range plan.Pages {
		for versionIndex, version := range page.Versions {
			body := resolver.Rewrite(version.Body)
			if !strings.HasSuffix(body, "\n") {
				body += "\n"
			}
			events = append(events, event{
				PageID:   page.PageID,
				Path:     page.Path,
				Content:  compose(version.Title, page.Order, body),
				Author:   version.Author,
				At:       version.At,
				sequence: pageIndex*10000 + versionIndex,
			})
		}
	}
	sort.SliceStable(events, func(i, j int) bool {
		if !events[i].At.Equal(events[j].At) {
			return events[i].At.Before(events[j].At)
		}
		if events[i].PageID != events[j].PageID {
			return events[i].PageID < events[j].PageID
		}
		return events[i].sequence < events[j].sequence
	})

	seen := map[string]bool{}
	last := map[string]string{}
	out := make([]event, 0, len(events))
	for _, item := range events {
		content := string(item.Content)
		if previous, ok := last[item.Path]; ok && previous == content {
			continue
		}
		if seen[item.Path] {
			item.Message = "docs: update " + item.Path
		} else {
			item.Message = "docs: import " + item.Path
			seen[item.Path] = true
		}
		last[item.Path] = content
		out = append(out, item)
	}
	return out
}

func compose(title string, order int, body string) []byte {
	raw, err := yaml.Marshal(struct {
		Title  string `yaml:"title"`
		Order  int    `yaml:"order"`
		Public bool   `yaml:"public"`
	}{Title: title, Order: order, Public: false})
	if err != nil {
		raw = []byte(fmt.Sprintf("title: %q\norder: %d\npublic: false\n", title, order))
	}
	return doc.Compose(raw, []byte(body))
}

func commitMeta(ctx context.Context, git *Git, meta map[string][]string, name, email string) error {
	dirs := make([]string, 0, len(meta))
	for dir := range meta {
		dirs = append(dirs, dir)
	}
	sort.Strings(dirs)

	written := 0
	for _, dir := range dirs {
		names := meta[dir]
		if len(names) == 0 {
			continue
		}
		rel := path.Join(dir, "_meta.yaml")
		full := filepath.Join(git.Dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(full, doc.MarshalOrder(names), 0o644); err != nil {
			return err
		}
		written++
	}
	if written == 0 {
		return nil
	}
	slog.Info("写入目录排序", "files", written)
	return git.CommitAll(ctx, name, email, time.Now(), "docs: preserve showdoc order")
}

func Resync(ctx context.Context, cfg *config.Config, slug, name string) error {
	return syncIndex(ctx, cfg, slug, name)
}

func syncIndex(ctx context.Context, cfg *config.Config, slug, name string) error {
	db, err := store.Open(cfg.Database)
	if err != nil {
		return err
	}
	box, err := cryptox.NewBox(cfg.Server.SessionKey)
	if err != nil {
		return err
	}
	st := store.New(db)
	docs := doc.New(cfg, st, box)
	if err := docs.SeedSources(ctx); err != nil {
		return err
	}
	src, err := docs.Source(ctx, slug)
	if err != nil {
		return err
	}

	if configured, ok := sourceConfig(cfg, slug); ok {
		changed := false
		token, openErr := box.Open(src.TokenEnc)
		if openErr != nil || string(token) != configured.Token {
			sealed, err := box.Seal([]byte(configured.Token))
			if err != nil {
				return err
			}
			src.TokenEnc = sealed
			changed = true
		}
		if name != "" && src.Name != name {
			src.Name = name
			changed = true
		}
		if changed {
			if err := st.UpdateSource(ctx, src); err != nil {
				return err
			}
		}
	}

	var syncErr error
	for attempt := 1; attempt <= 3; attempt++ {
		syncErr = docs.Sync(ctx, src)
		if syncErr == nil {
			break
		}
		slog.Warn("索引同步失败，准备重试", "attempt", attempt, "error", syncErr)
		time.Sleep(2 * time.Second)
	}
	if syncErr != nil {
		return syncErr
	}

	metas, err := st.ListDocMeta(ctx, src.ID)
	if err != nil {
		return err
	}
	docsCount, dirsCount := 0, 0
	for _, item := range metas {
		if item.Type == "doc" {
			docsCount++
		} else {
			dirsCount++
		}
	}
	slog.Info("索引已重建", "docs", docsCount, "dirs", dirsCount, "source", src.Slug)
	return nil
}

func sourceConfig(cfg *config.Config, slug string) (config.Source, bool) {
	if slug == "" {
		if len(cfg.Sources) == 0 {
			return config.Source{}, false
		}
		return cfg.Sources[0], true
	}
	for _, item := range cfg.Sources {
		if item.Slug == slug {
			return item, true
		}
	}
	return config.Source{}, false
}

func remoteURL(src config.Source) (string, error) {
	base, err := url.Parse(src.BaseURL)
	if err != nil || base.Scheme == "" || base.Host == "" {
		return "", fmt.Errorf("来源 %s 的 base_url 无效", src.Slug)
	}
	base.User = url.UserPassword("oauth2", src.Token)
	base.Path = strings.TrimSuffix(base.Path, "/") + "/" + path.Join(src.Owner, src.Repo+".git")
	return base.String(), nil
}

func attachmentHost(uploads []Upload) string {
	for _, upload := range uploads {
		parsed, err := url.Parse(upload.RealURL)
		if err == nil && parsed.Host != "" {
			return parsed.Host
		}
	}
	return ""
}

func printPlan(plan *Plan) {
	slog.Info("迁移计划",
		"groups", plan.Stats.Groups,
		"pages", plan.Stats.Pages,
		"versions", plan.Stats.Versions,
		"dirs", plan.Stats.Dirs,
		"meta", plan.Stats.Meta,
	)
	for _, group := range plan.Groups {
		slog.Info("项目", "name", group.Name, "path", group.Path, "pages", group.Pages)
	}
}

func printPlanDetail(plan *Plan) {
	for _, page := range plan.Pages {
		slog.Info("页面", "path", page.Path, "order", page.Order, "versions", len(page.Versions))
	}
	dirs := make([]string, 0, len(plan.Meta))
	for dir := range plan.Meta {
		dirs = append(dirs, dir)
	}
	sort.Strings(dirs)
	for _, dir := range dirs {
		if len(plan.Meta[dir]) > 1 {
			slog.Info("排序", "dir", dir, "entries", plan.Meta[dir])
		}
	}
}
