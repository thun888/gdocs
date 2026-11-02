package doc

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"path"
	"sort"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/thun888/zdocs-go/internal/config"
	"github.com/thun888/zdocs-go/internal/cryptox"
	"github.com/thun888/zdocs-go/internal/source"
	"github.com/thun888/zdocs-go/internal/store"
)

const (
	metaFile       = "_meta.yaml"
	maxDocBytes    = 2 << 20
	maxMetaBytes   = 1 << 20
	unlistedOffset = 1000
)

type docEntry struct {
	meta Meta
	raw  []byte
}

type Node struct {
	Name     string  `json:"name"`
	Path     string  `json:"path"`
	Type     string  `json:"type"`
	Title    string  `json:"title,omitempty"`
	Private  bool    `json:"private"`
	Draft    bool    `json:"draft,omitempty"`
	Children []*Node `json:"children,omitempty"`

	rank int
}

type Service struct {
	cfg   *config.Config
	store *store.Store
	box   *cryptox.Box

	mu       sync.Mutex
	locks    map[uint64]*sync.Mutex
	lastSync map[uint64]time.Time
}

func New(cfg *config.Config, st *store.Store, box *cryptox.Box) *Service {
	return &Service{
		cfg:      cfg,
		store:    st,
		box:      box,
		locks:    map[uint64]*sync.Mutex{},
		lastSync: map[uint64]time.Time{},
	}
}

func (s *Service) SeedSources(ctx context.Context) error {
	for i := range s.cfg.Sources {
		item := s.cfg.Sources[i]
		if _, err := s.store.SourceBySlug(ctx, item.Slug); err == nil {
			continue
		} else if !store.IsNotFound(err) {
			return err
		}
		sealed, err := s.box.Seal([]byte(item.Token))
		if err != nil {
			return err
		}
		src := &store.Source{
			Slug:     item.Slug,
			Name:     item.Name,
			BaseURL:  item.BaseURL,
			TokenEnc: sealed,
			Owner:    item.Owner,
			Repo:     item.Repo,
			Branch:   item.Branch,
			Root:     item.Root,
			Sort:     item.Sort,
			Enabled:  true,
		}
		if err := s.store.CreateSource(ctx, src); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) SealToken(token string) ([]byte, error) {
	return s.box.Seal([]byte(token))
}

func (s *Service) Client(ctx context.Context, src *store.Source) (*source.Client, error) {
	token, err := s.box.Open(src.TokenEnc)
	if err != nil {
		return nil, fmt.Errorf("解密仓库令牌: %w", err)
	}
	return source.New(src.BaseURL, string(token), src.Owner, src.Repo, src.Branch, src.Root), nil
}

func (s *Service) Source(ctx context.Context, slug string) (*store.Source, error) {
	return s.store.SourceBySlug(ctx, slug)
}

func (s *Service) lock(srcID uint64) *sync.Mutex {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.locks[srcID]; !ok {
		s.locks[srcID] = &sync.Mutex{}
	}
	return s.locks[srcID]
}

func (s *Service) EnsureFresh(ctx context.Context, src *store.Source) error {
	s.mu.Lock()
	last := s.lastSync[src.ID]
	s.mu.Unlock()
	if time.Since(last) < time.Duration(s.cfg.Index.TTLMinutes)*time.Minute {
		return nil
	}
	return s.Sync(ctx, src)
}

func (s *Service) Sync(ctx context.Context, src *store.Source) error {
	lock := s.lock(src.ID)
	lock.Lock()
	defer lock.Unlock()

	client, err := s.Client(ctx, src)
	if err != nil {
		return err
	}
	body, err := client.Archive(ctx)
	if err != nil {
		return err
	}
	defer body.Close()

	limit := int64(s.cfg.Index.MaxArchiveMB) << 20
	gz, err := gzip.NewReader(&limitedReader{r: body, remaining: limit})
	if err != nil {
		return fmt.Errorf("解压仓库归档: %w", err)
	}
	defer gz.Close()

	docs := map[string]docEntry{}
	orders := map[string]map[string]int{}

	tr := tar.NewReader(gz)
	for {
		header, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("读取仓库归档: %w", err)
		}
		if header.Typeflag != tar.TypeReg {
			continue
		}

		name := strings.TrimPrefix(path.Clean(header.Name), "./")
		if i := strings.Index(name, "/"); i >= 0 {
			name = name[i+1:]
		} else {
			continue
		}
		rel, ok := UnderRoot(name, src.Root)
		if !ok || rel == "" {
			continue
		}

		switch {
		case strings.EqualFold(path.Base(rel), metaFile):
			if header.Size > maxMetaBytes {
				continue
			}
			raw, _ := io.ReadAll(io.LimitReader(tr, maxMetaBytes))
			dir := path.Dir(rel)
			if dir == "." {
				dir = ""
			}
			orders[dir] = parseOrder(raw)
		case IsDoc(rel):
			if header.Size > maxDocBytes {
				slog.Warn("跳过超大文档", "path", rel, "size", header.Size)
				continue
			}
			raw, _ := io.ReadAll(io.LimitReader(tr, maxDocBytes))
			fm := Parse(raw)
			docs[rel] = docEntry{meta: fm.Meta, raw: fm.Raw}
		}
	}

	rows := buildRows(src.ID, docs, orders)
	if err := s.store.ReplaceDocMeta(ctx, src.ID, rows); err != nil {
		return fmt.Errorf("写入文档索引: %w", err)
	}

	s.mu.Lock()
	s.lastSync[src.ID] = time.Now()
	s.mu.Unlock()
	slog.Info("文档索引已更新", "source", src.Slug, "docs", len(docs))
	return nil
}

func buildRows(sourceID uint64, docs map[string]docEntry, orders map[string]map[string]int) []store.DocMeta {
	dirs := map[string]bool{}
	for p := range docs {
		for parent := path.Dir(p); parent != "." && parent != "/" && parent != ""; parent = path.Dir(parent) {
			dirs[parent] = true
		}
	}

	rank := func(dir, name string, fallback int) int {
		if items, ok := orders[dir]; ok {
			if idx, ok := items[name]; ok {
				return idx
			}
		}
		return unlistedOffset + fallback
	}

	now := time.Now()
	rows := make([]store.DocMeta, 0, len(docs)+len(dirs))

	for dir := range dirs {
		parent := path.Dir(dir)
		if parent == "." {
			parent = ""
		}
		rows = append(rows, store.DocMeta{
			SourceID:  sourceID,
			Path:      dir,
			Type:      "dir",
			Title:     path.Base(dir),
			Sort:      rank(parent, path.Base(dir), 0),
			UpdatedAt: now,
		})
	}

	for p, item := range docs {
		parent := path.Dir(p)
		if parent == "." {
			parent = ""
		}
		rows = append(rows, store.DocMeta{
			SourceID:  sourceID,
			Path:      p,
			Type:      "doc",
			Title:     Title(p, item.meta),
			Sort:      rank(parent, path.Base(p), item.meta.Order),
			Public:    item.meta.Public,
			Draft:     item.meta.Draft,
			UpdatedAt: now,
		})
	}
	return rows
}

func parseOrder(raw []byte) map[string]int {
	var payload struct {
		Order []string `yaml:"order"`
	}
	_ = yaml.Unmarshal(raw, &payload)
	out := make(map[string]int, len(payload.Order))
	for i, name := range payload.Order {
		out[name] = i
	}
	return out
}

func (s *Service) Tree(ctx context.Context, sourceID uint64, loggedIn bool) ([]*Node, error) {
	rows, err := s.store.ListDocMeta(ctx, sourceID)
	if err != nil {
		return nil, err
	}

	nodes := make(map[string]*Node, len(rows))
	for _, row := range rows {
		nodes[row.Path] = &Node{
			Name:    path.Base(row.Path),
			Path:    row.Path,
			Type:    row.Type,
			Title:   row.Title,
			Private: !effectivePublic(row.Public, s.cfg.Server.PublicRead),
			Draft:   row.Draft,
			rank:    row.Sort,
		}
	}

	roots := make([]*Node, 0, len(rows))
	for _, row := range rows {
		node := nodes[row.Path]
		parent := path.Dir(row.Path)
		if parent == "." || parent == "/" || parent == "" {
			roots = append(roots, node)
			continue
		}
		if p, ok := nodes[parent]; ok {
			p.Children = append(p.Children, node)
		}
	}

	return filterVisible(roots, loggedIn), nil
}

func effectivePublic(flag *bool, globalDefault bool) bool {
	if flag == nil {
		return globalDefault
	}
	return *flag
}

func Visible(node *Node, loggedIn bool) bool {
	if node.Type != "doc" {
		return true
	}
	if node.Draft {
		return loggedIn
	}
	if loggedIn {
		return true
	}
	return !node.Private
}

func filterVisible(nodes []*Node, loggedIn bool) []*Node {
	out := make([]*Node, 0, len(nodes))
	for _, node := range nodes {
		if node.Type == "doc" {
			if !Visible(node, loggedIn) {
				continue
			}
			out = append(out, node)
			continue
		}
		node.Children = filterVisible(node.Children, loggedIn)
		if len(node.Children) > 0 {
			out = append(out, node)
		}
	}
	sortNodes(out)
	return out
}

func sortNodes(nodes []*Node) {
	sort.SliceStable(nodes, func(i, j int) bool {
		if nodes[i].Type != nodes[j].Type {
			return nodes[i].Type == "dir"
		}
		if nodes[i].rank != nodes[j].rank {
			return nodes[i].rank < nodes[j].rank
		}
		return nodes[i].Name < nodes[j].Name
	})
	for _, node := range nodes {
		if len(node.Children) > 0 {
			sortNodes(node.Children)
		}
	}
}

type limitedReader struct {
	r         io.Reader
	remaining int64
}

func (l *limitedReader) Read(p []byte) (int, error) {
	if l.remaining <= 0 {
		return 0, fmt.Errorf("仓库归档超过大小上限")
	}
	if int64(len(p)) > l.remaining {
		p = p[:l.remaining]
	}
	n, err := l.r.Read(p)
	l.remaining -= int64(n)
	return n, err
}
