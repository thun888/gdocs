package server

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"net/http"
	"path"
	"strconv"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
)

var contentTypes = map[string]string{
	".css":   "text/css; charset=utf-8",
	".eot":   "application/vnd.ms-fontobject",
	".html":  "text/html; charset=utf-8",
	".ico":   "image/x-icon",
	".jpeg":  "image/jpeg",
	".jpg":   "image/jpeg",
	".js":    "text/javascript; charset=utf-8",
	".json":  "application/json; charset=utf-8",
	".map":   "application/json; charset=utf-8",
	".mjs":   "text/javascript; charset=utf-8",
	".png":   "image/png",
	".svg":   "image/svg+xml",
	".ttf":   "font/ttf",
	".txt":   "text/plain; charset=utf-8",
	".webp":  "image/webp",
	".woff":  "font/woff",
	".woff2": "font/woff2",
}

type staticAssets struct {
	fs    fs.FS
	gzip  sync.Map
	etags sync.Map
}

func (a *staticAssets) handle(c *gin.Context) {
	if strings.HasPrefix(c.Request.URL.Path, "/api/") {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "接口不存在"})
		return
	}

	name := strings.TrimPrefix(path.Clean(c.Request.URL.Path), "/")
	if name != "" && name != "." {
		if info, err := fs.Stat(a.fs, name); err == nil && !info.IsDir() {
			a.serve(c, name)
			return
		}
	}
	a.serve(c, "index.html")
}

func (a *staticAssets) serve(c *gin.Context, name string) {
	data, err := fs.ReadFile(a.fs, name)
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}

	contentType, ok := contentTypes[strings.ToLower(path.Ext(name))]
	if !ok {
		contentType = http.DetectContentType(data)
	}

	header := c.Writer.Header()
	header.Set("Content-Type", contentType)
	if strings.HasPrefix(name, "assets/") {
		header.Set("Cache-Control", "public, max-age=31536000, immutable")
	} else {
		header.Set("Cache-Control", "no-cache")
	}

	etag := a.etag(name, data)
	header.Set("ETag", etag)
	if requestETag(c.Request) == etag {
		c.Status(http.StatusNotModified)
		return
	}

	if compressible(contentType) && len(data) >= 1024 && acceptsGzip(c.Request) {
		header.Set("Content-Encoding", "gzip")
		header.Set("Vary", "Accept-Encoding")
		data = a.compressed(name, data)
	}

	header.Set("Content-Length", strconv.Itoa(len(data)))
	if c.Request.Method == http.MethodHead {
		c.Status(http.StatusOK)
		return
	}
	c.Status(http.StatusOK)
	_, _ = c.Writer.Write(data)
}

func (a *staticAssets) compressed(name string, data []byte) []byte {
	if cached, ok := a.gzip.Load(name); ok {
		return cached.([]byte)
	}
	var buffer bytes.Buffer
	writer, err := gzip.NewWriterLevel(&buffer, gzip.BestCompression)
	if err != nil {
		return data
	}
	_, _ = writer.Write(data)
	_ = writer.Close()
	payload := buffer.Bytes()
	a.gzip.Store(name, payload)
	return payload
}

func (a *staticAssets) etag(name string, data []byte) string {
	if cached, ok := a.etags.Load(name); ok {
		return cached.(string)
	}
	sum := sha256.Sum256(data)
	etag := `"` + hex.EncodeToString(sum[:8]) + `"`
	a.etags.Store(name, etag)
	return etag
}

func requestETag(request *http.Request) string {
	value := request.Header.Get("If-None-Match")
	if index := strings.IndexByte(value, ','); index >= 0 {
		value = value[:index]
	}
	return strings.TrimSpace(value)
}

func compressible(contentType string) bool {
	switch {
	case strings.HasPrefix(contentType, "text/"):
		return true
	case strings.HasPrefix(contentType, "application/json"):
		return true
	case contentType == "image/svg+xml":
		return true
	default:
		return false
	}
}

func acceptsGzip(request *http.Request) bool {
	for _, part := range strings.Split(request.Header.Get("Accept-Encoding"), ",") {
		if strings.TrimSpace(strings.SplitN(part, ";", 2)[0]) == "gzip" {
			return true
		}
	}
	return false
}
