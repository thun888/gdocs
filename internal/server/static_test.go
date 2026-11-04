package server

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	"github.com/gin-gonic/gin"
)

func newTestAssets() *staticAssets {
	return &staticAssets{fs: fstest.MapFS{
		"index.html":     {Data: []byte("<!doctype html><div id=\"root\"></div>")},
		"assets/app.js":  {Data: bytes.Repeat([]byte("const value = 1;\n"), 128)},
		"images/bg.png":  {Data: []byte{0x89, 0x50, 0x4e, 0x47}},
		"assets/app.css": {Data: []byte("body{margin:0}")},
	}}
}

func newTestEngine(assets *staticAssets) *gin.Engine {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.NoRoute(assets.handle)
	return engine
}

func perform(engine *gin.Engine, path string, headers map[string]string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodGet, path, nil)
	for key, value := range headers {
		request.Header.Set(key, value)
	}
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, request)
	return recorder
}

func serveAssets(t *testing.T, path string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	return perform(newTestEngine(newTestAssets()), path, headers)
}

func TestStaticAssetUsesGzipAndImmutableCache(t *testing.T) {
	recorder := serveAssets(t, "/assets/app.js", map[string]string{"Accept-Encoding": "gzip, deflate"})

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d", recorder.Code)
	}
	if got := recorder.Header().Get("Content-Encoding"); got != "gzip" {
		t.Fatalf("content-encoding = %q", got)
	}
	if got := recorder.Header().Get("Cache-Control"); got != "public, max-age=31536000, immutable" {
		t.Fatalf("cache-control = %q", got)
	}
	if got := recorder.Header().Get("Content-Type"); got != "text/javascript; charset=utf-8" {
		t.Fatalf("content-type = %q", got)
	}

	reader, err := gzip.NewReader(bytes.NewReader(recorder.Body.Bytes()))
	if err != nil {
		t.Fatalf("gzip body: %v", err)
	}
	body, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if len(body) != 128*len("const value = 1;\n") {
		t.Fatalf("unexpected body length %d", len(body))
	}
}

func TestStaticAssetRevalidatesWithETag(t *testing.T) {
	engine := newTestEngine(newTestAssets())
	first := perform(engine, "/assets/app.js", nil)
	etag := first.Header().Get("ETag")
	if etag == "" {
		t.Fatal("missing etag")
	}

	second := perform(engine, "/assets/app.js", map[string]string{"If-None-Match": etag})
	if second.Code != http.StatusNotModified {
		t.Fatalf("status = %d", second.Code)
	}
	if second.Body.Len() != 0 {
		t.Fatalf("body = %q", second.Body.String())
	}
}

func TestStaticAssetKeepsBinaryUncompressed(t *testing.T) {
	recorder := serveAssets(t, "/images/bg.png", map[string]string{"Accept-Encoding": "gzip"})

	if got := recorder.Header().Get("Content-Encoding"); got != "" {
		t.Fatalf("content-encoding = %q", got)
	}
	if got := recorder.Header().Get("Content-Type"); got != "image/png" {
		t.Fatalf("content-type = %q", got)
	}
}

func TestStaticFallbackServesIndex(t *testing.T) {
	recorder := serveAssets(t, "/docs/guide/intro", nil)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d", recorder.Code)
	}
	if got := recorder.Header().Get("Cache-Control"); got != "no-cache" {
		t.Fatalf("cache-control = %q", got)
	}
	if !bytes.Contains(recorder.Body.Bytes(), []byte("id=\"root\"")) {
		t.Fatalf("body = %q", recorder.Body.String())
	}
}

func TestStaticApiMissReturnsJson(t *testing.T) {
	recorder := serveAssets(t, "/api/missing", nil)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d", recorder.Code)
	}
	if !bytes.Contains(recorder.Body.Bytes(), []byte("\"code\":404")) {
		t.Fatalf("body = %q", recorder.Body.String())
	}
}
