package showdoc

import (
	"context"
	"crypto/sha256"
	"fmt"
	"html"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

type AttachmentUploader interface {
	PutAttachment(ctx context.Context, filename, contentType string, data []byte, at time.Time) (string, string, error)
}

type AttachmentStats struct {
	Total     int
	Uploaded  int
	Reused    int
	Missing   int
	Failed    int
	Rewritten int
}

type Resolver struct {
	host   string
	bySign map[string]string
	byURL  map[string]string
	byPath map[string]string

	absAttachment *regexp.Regexp
	absUpload     *regexp.Regexp
	relAttachment *regexp.Regexp
	relUpload     *regexp.Regexp

	stats AttachmentStats
}

func NewResolver(host string) *Resolver {
	if host == "" {
		host = "showdoc.hzchu.top"
	}
	quoted := regexp.QuoteMeta(host)
	return &Resolver{
		host:          host,
		bySign:        map[string]string{},
		byURL:         map[string]string{},
		byPath:        map[string]string{},
		absAttachment: regexp.MustCompile(`(?i)https?://` + quoted + `/(?:server/)?(?:index\.php)?\?s=/api/attachment/visitFile[^\s)\]"'<>]*`),
		absUpload:     regexp.MustCompile(`(?i)https?://` + quoted + `/Public/Uploads/[^\s)\]"'<>]*`),
		relAttachment: regexp.MustCompile(`(?i)/(?:server/)?(?:index\.php)?\?s=/api/attachment/visitFile[^\s)\]"'<>]*`),
		relUpload:     regexp.MustCompile(`(?i)/Public/Uploads/[^\s)\]"'<>]*`),
	}
}

func (r *Resolver) Stats() AttachmentStats { return r.stats }

func (r *Resolver) Upload(ctx context.Context, uploader AttachmentUploader, uploads []Upload, root string) error {
	index := map[string]string{}
	if root != "" {
		_ = filepath.Walk(root, func(name string, info os.FileInfo, err error) error {
			if err != nil || info == nil || info.IsDir() {
				return nil
			}
			rel, relErr := filepath.Rel(root, name)
			if relErr != nil {
				return nil
			}
			index[strings.ToLower(filepath.ToSlash(rel))] = name
			base := strings.ToLower(filepath.Base(name))
			if _, exists := index[base]; !exists {
				index[base] = name
			}
			return nil
		})
	}

	r.stats.Total = len(uploads)
	contentCache := map[[32]byte]string{}

	for _, upload := range uploads {
		upload := upload
		local, rel, ok := locateUpload(upload, root, index)
		if !ok {
			r.stats.Missing++
			slog.Warn("附件文件不存在", "sign", upload.Sign, "name", upload.DisplayName)
			continue
		}
		data, err := os.ReadFile(local)
		if err != nil {
			r.stats.Failed++
			slog.Warn("读取附件失败", "file", local, "error", err)
			continue
		}

		sum := sha256.Sum256(data)
		if link, ok := contentCache[sum]; ok {
			r.mapUpload(upload, rel, link)
			r.stats.Reused++
			continue
		}

		filename := upload.DisplayName
		if filename == "" || !strings.EqualFold(path.Ext(filename), path.Ext(local)) {
			filename = filepath.Base(local)
		}
		contentType := upload.FileType
		sniffed := http.DetectContentType(data)
		if strings.HasPrefix(contentType, "image/") && strings.HasPrefix(sniffed, "image/") {
			contentType = sniffed
		}
		link, _, err := uploader.PutAttachment(ctx, filename, contentType, data, time.Unix(upload.AddTime, 0).In(cst))
		if err != nil {
			r.stats.Failed++
			slog.Warn("上传附件失败", "name", filename, "error", err)
			continue
		}
		contentCache[sum] = link
		r.mapUpload(upload, rel, link)
		r.stats.Uploaded++
		if r.stats.Uploaded%10 == 0 {
			slog.Info("附件上传进度", "done", r.stats.Uploaded, "total", r.stats.Total)
		}
	}
	return nil
}

func (r *Resolver) Rewrite(body string) string {
	body = html.UnescapeString(body)
	body = strings.ReplaceAll(body, "\r\n", "\n")
	body = strings.ReplaceAll(body, "\r", "\n")

	replace := func(re *regexp.Regexp, resolve func(match string) (string, bool)) {
		body = re.ReplaceAllStringFunc(body, func(match string) string {
			if link, ok := resolve(match); ok {
				r.stats.Rewritten++
				return link
			}
			return match
		})
	}

	replace(r.absAttachment, func(match string) (string, bool) {
		if parsed, err := url.Parse(match); err == nil {
			if sign := parsed.Query().Get("sign"); sign != "" {
				link, ok := r.bySign[sign]
				return link, ok
			}
		}
		link, ok := r.byURL[match]
		return link, ok
	})
	replace(r.absUpload, func(match string) (string, bool) {
		if link, ok := r.byURL[trimURLTail(match)]; ok {
			return link, true
		}
		link, ok := r.byPath[uploadPath(match)]
		return link, ok
	})
	replace(r.relAttachment, func(match string) (string, bool) {
		if parsed, err := url.Parse(match); err == nil {
			if sign := parsed.Query().Get("sign"); sign != "" {
				link, ok := r.bySign[sign]
				return link, ok
			}
		}
		link, ok := r.byURL[match]
		return link, ok
	})
	replace(r.relUpload, func(match string) (string, bool) {
		link, ok := r.byPath[uploadPath(match)]
		return link, ok
	})
	return body
}

func (r *Resolver) mapUpload(upload Upload, rel, link string) {
	if upload.Sign != "" {
		r.bySign[upload.Sign] = link
	}
	if upload.RealURL != "" {
		r.byURL[upload.RealURL] = link
	}
	if p := uploadPath(upload.RealURL); p != "" {
		r.byPath[p] = link
	}
	if rel != "" {
		r.byPath["/Public/Uploads/"+rel] = link
	}
}

func locateUpload(upload Upload, root string, index map[string]string) (string, string, bool) {
	if root == "" {
		return "", "", false
	}
	rel := uploadRelativePath(upload.RealURL)
	if rel != "" {
		local := filepath.Join(root, filepath.FromSlash(rel))
		if info, err := os.Stat(local); err == nil && !info.IsDir() {
			return local, rel, true
		}
		if match, ok := index[strings.ToLower(rel)]; ok {
			return match, rel, true
		}
	}
	if upload.DisplayName != "" {
		if match, ok := index[strings.ToLower(upload.DisplayName)]; ok {
			return match, filepath.ToSlash(mustRel(root, match)), true
		}
	}
	return "", "", false
}

func uploadRelativePath(raw string) string {
	if raw == "" {
		return ""
	}
	idx := strings.Index(raw, "/Public/Uploads/")
	if idx < 0 {
		return ""
	}
	rel := raw[idx+len("/Public/Uploads/"):]
	if cut := strings.IndexAny(rel, "?#"); cut >= 0 {
		rel = rel[:cut]
	}
	if decoded, err := url.PathUnescape(rel); err == nil {
		rel = decoded
	}
	return strings.TrimPrefix(path.Clean("/"+rel), "/")
}

func uploadPath(raw string) string {
	if raw == "" {
		return ""
	}
	if idx := strings.Index(raw, "/Public/Uploads/"); idx >= 0 {
		raw = raw[idx:]
	}
	if cut := strings.IndexAny(raw, "?#"); cut >= 0 {
		raw = raw[:cut]
	}
	if decoded, err := url.PathUnescape(raw); err == nil {
		raw = decoded
	}
	return raw
}

func trimURLTail(raw string) string {
	return strings.TrimRight(raw, ".,;:!?，。；：！？")
}

func mustRel(root, name string) string {
	rel, err := filepath.Rel(root, name)
	if err != nil {
		return filepath.Base(name)
	}
	return rel
}

func (r *Resolver) Summary() string {
	return fmt.Sprintf("附件 %d 个：上传 %d，复用 %d，缺失 %d，失败 %d，改写链接 %d",
		r.stats.Total, r.stats.Uploaded, r.stats.Reused, r.stats.Missing, r.stats.Failed, r.stats.Rewritten)
}
