package source

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"
)

type Client struct {
	baseURL string
	token   string
	owner   string
	repo    string
	branch  string
	root    string
	http    *http.Client
}

func New(baseURL, token, owner, repo, branch, root string) *Client {
	if branch == "" {
		branch = "main"
	}
	return &Client{
		baseURL: strings.TrimSuffix(baseURL, "/"),
		token:   token,
		owner:   owner,
		repo:    repo,
		branch:  branch,
		root:    strings.Trim(root, "/"),
		http:    &http.Client{Timeout: 60 * time.Second},
	}
}

func (c *Client) Root() string     { return c.root }
func (c *Client) Branch() string   { return c.branch }
func (c *Client) RepoName() string { return c.owner + "/" + c.repo }

func (c *Client) do(ctx context.Context, method, endpoint string, query url.Values, body, out any) error {
	target := c.baseURL + "/api/v1/repos/" + url.PathEscape(c.owner) + "/" + url.PathEscape(c.repo) + endpoint
	if len(query) > 0 {
		target += "?" + query.Encode()
	}

	var reader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(payload)
	}

	req, err := http.NewRequestWithContext(ctx, method, target, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "token "+c.token)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrUpstream, err)
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusNotFound:
		return ErrNotFound
	case resp.StatusCode == http.StatusConflict:
		return ErrConflict
	case resp.StatusCode == http.StatusUnauthorized, resp.StatusCode == http.StatusForbidden:
		return ErrAuth
	case resp.StatusCode >= 500:
		return fmt.Errorf("%w: %d", ErrUpstream, resp.StatusCode)
	case resp.StatusCode >= 400:
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("%w: %d %s", ErrUpstream, resp.StatusCode, strings.TrimSpace(string(snippet)))
	}

	if out == nil {
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("%w: 解析响应 %v", ErrUpstream, err)
	}
	return nil
}

func escapeSegments(p string) string {
	parts := strings.Split(strings.Trim(p, "/"), "/")
	for i, part := range parts {
		parts[i] = url.PathEscape(part)
	}
	return strings.Join(parts, "/")
}

func (c *Client) fullPath(p string) string {
	if c.root == "" {
		return strings.Trim(p, "/")
	}
	return path.Join(c.root, strings.Trim(p, "/"))
}

type treeItem struct {
	Path string `json:"path"`
	Type string `json:"type"`
	SHA  string `json:"sha"`
	Size int64  `json:"size"`
}

func (c *Client) Tree(ctx context.Context) ([]Entry, error) {
	var entries []Entry
	for page := 1; page <= 50; page++ {
		var resp struct {
			Tree      []treeItem `json:"tree"`
			Truncated bool       `json:"truncated"`
		}
		query := url.Values{"recursive": {"true"}, "per_page": {"1000"}, "page": {strconv.Itoa(page)}}
		if err := c.do(ctx, http.MethodGet, "/git/trees/"+url.PathEscape(c.branch), query, nil, &resp); err != nil {
			return nil, err
		}
		for _, item := range resp.Tree {
			if c.root != "" {
				if item.Path == c.root || !strings.HasPrefix(item.Path, c.root+"/") {
					continue
				}
			}
			rel := strings.TrimPrefix(item.Path, c.root)
			rel = strings.TrimPrefix(rel, "/")
			if rel == "" {
				continue
			}
			entries = append(entries, Entry{
				Path: rel,
				Dir:  item.Type == "tree",
				SHA:  item.SHA,
				Size: item.Size,
			})
		}
		if len(resp.Tree) < 1000 {
			break
		}
	}
	return entries, nil
}

type contentResponse struct {
	Name     string `json:"name"`
	Path     string `json:"path"`
	SHA      string `json:"sha"`
	Type     string `json:"type"`
	Size     int64  `json:"size"`
	Content  string `json:"content"`
	Encoding string `json:"encoding"`
}

type writeResponse struct {
	Content struct {
		SHA string `json:"sha"`
	} `json:"content"`
}

func (c *Client) Read(ctx context.Context, p string) (*File, error) {
	var resp contentResponse
	query := url.Values{"ref": {c.branch}}
	if err := c.do(ctx, http.MethodGet, "/contents/"+escapeSegments(c.fullPath(p)), query, nil, &resp); err != nil {
		return nil, err
	}
	if resp.Type != "file" {
		return nil, ErrNotFound
	}
	content, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(resp.Content, "\n", ""))
	if err != nil {
		return nil, fmt.Errorf("%w: 解码文件内容 %v", ErrUpstream, err)
	}
	return &File{Path: p, SHA: resp.SHA, Content: content}, nil
}

type identityPayload struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

type writeBody struct {
	Content   string           `json:"content"`
	Message   string           `json:"message"`
	SHA       string           `json:"sha,omitempty"`
	Branch    string           `json:"branch"`
	Author    *identityPayload `json:"author,omitempty"`
	Committer *identityPayload `json:"committer,omitempty"`
}

func identity(id Identity) (*identityPayload, *identityPayload) {
	if id.Name == "" && id.Email == "" {
		return nil, nil
	}
	value := &identityPayload{Name: id.Name, Email: id.Email}
	return value, value
}

func (c *Client) Write(ctx context.Context, p string, content []byte, sha, message string, id Identity) (string, error) {
	author, committer := identity(id)
	var resp writeResponse
	body := writeBody{
		Content:   base64.StdEncoding.EncodeToString(content),
		Message:   message,
		SHA:       sha,
		Branch:    c.branch,
		Author:    author,
		Committer: committer,
	}
	if err := c.do(ctx, http.MethodPut, "/contents/"+escapeSegments(c.fullPath(p)), nil, body, &resp); err != nil {
		return "", err
	}
	return resp.Content.SHA, nil
}

func (c *Client) Create(ctx context.Context, p string, content []byte, message string, id Identity) (string, error) {
	author, committer := identity(id)
	var resp writeResponse
	body := writeBody{
		Content:   base64.StdEncoding.EncodeToString(content),
		Message:   message,
		Branch:    c.branch,
		Author:    author,
		Committer: committer,
	}
	if err := c.do(ctx, http.MethodPost, "/contents/"+escapeSegments(c.fullPath(p)), nil, body, &resp); err != nil {
		return "", err
	}
	return resp.Content.SHA, nil
}

func (c *Client) Delete(ctx context.Context, p, sha, message string, id Identity) error {
	author, committer := identity(id)
	body := writeBody{Message: message, SHA: sha, Branch: c.branch, Author: author, Committer: committer}
	return c.do(ctx, http.MethodDelete, "/contents/"+escapeSegments(c.fullPath(p)), nil, body, nil)
}

func (c *Client) Move(ctx context.Context, from, to, sha, message string, id Identity) error {
	file, err := c.Read(ctx, from)
	if err != nil {
		return err
	}
	if _, err := c.Create(ctx, to, file.Content, message, id); err != nil {
		return err
	}
	return c.Delete(ctx, from, file.SHA, message, id)
}

type commitResponse struct {
	SHA    string `json:"sha"`
	Commit struct {
		Message string `json:"message"`
		Author  struct {
			Name  string    `json:"name"`
			Email string    `json:"email"`
			Date  time.Time `json:"date"`
		} `json:"author"`
	} `json:"commit"`
	Files []FileDiff `json:"files"`
}

func (r commitResponse) toCommit() Commit {
	return Commit{
		SHA:     r.SHA,
		Message: r.Commit.Message,
		Author:  r.Commit.Author.Name,
		Email:   r.Commit.Author.Email,
		Date:    r.Commit.Author.Date,
	}
}

func (c *Client) commits(ctx context.Context, p string, limit int) ([]Commit, error) {
	query := url.Values{"sha": {c.branch}, "limit": {strconv.Itoa(limit)}, "page": {"1"}}
	if p != "" {
		query.Set("path", c.fullPath(p))
	}
	var resp []commitResponse
	if err := c.do(ctx, http.MethodGet, "/commits", query, nil, &resp); err != nil {
		return nil, err
	}
	out := make([]Commit, 0, len(resp))
	for _, item := range resp {
		out = append(out, item.toCommit())
	}
	return out, nil
}

func (c *Client) History(ctx context.Context, p string, limit int) ([]Commit, error) {
	return c.commits(ctx, p, limit)
}

func (c *Client) Recent(ctx context.Context, limit int) ([]Commit, error) {
	return c.commits(ctx, "", limit)
}

func (c *Client) Commit(ctx context.Context, sha string) (*CommitDetail, error) {
	var resp commitResponse
	if err := c.do(ctx, http.MethodGet, "/git/commits/"+url.PathEscape(sha), nil, nil, &resp); err != nil {
		return nil, err
	}
	detail := &CommitDetail{Commit: resp.toCommit(), Files: resp.Files}
	if len(detail.Files) == 0 {
		var withFiles struct {
			Files []FileDiff `json:"files"`
		}
		_ = c.do(ctx, http.MethodGet, "/commits/"+url.PathEscape(sha), nil, nil, &withFiles)
		detail.Files = withFiles.Files
	}
	return detail, nil
}

func (c *Client) Compare(ctx context.Context, base, head string) ([]FileDiff, error) {
	var resp struct {
		Files []FileDiff `json:"files"`
	}
	endpoint := "/compare/" + url.PathEscape(base) + "..." + url.PathEscape(head)
	if err := c.do(ctx, http.MethodGet, endpoint, nil, nil, &resp); err != nil {
		return nil, err
	}
	return resp.Files, nil
}

func (c *Client) Archive(ctx context.Context) (io.ReadCloser, error) {
	target := c.baseURL + "/api/v1/repos/" + url.PathEscape(c.owner) + "/" + url.PathEscape(c.repo) +
		"/archive/" + url.PathEscape(c.branch) + ".tar.gz"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "token "+c.token)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUpstream, err)
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		switch resp.StatusCode {
		case http.StatusNotFound:
			return nil, ErrNotFound
		case http.StatusUnauthorized, http.StatusForbidden:
			return nil, ErrAuth
		}
		return nil, fmt.Errorf("%w: %d", ErrUpstream, resp.StatusCode)
	}
	return resp.Body, nil
}
