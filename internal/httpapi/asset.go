package httpapi

import (
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/thun888/gdocs-go/internal/storage"
)

func (a *API) handleAsset(c *gin.Context) {
	if a.upload == nil {
		fail(c, http.StatusServiceUnavailable, "对象存储未配置")
		return
	}
	object, err := a.upload.Open(c.Request.Context(), c.Param("key"))
	if err != nil {
		switch {
		case errors.Is(err, storage.ErrObjectNotFound), errors.Is(err, storage.ErrInvalidKey):
			fail(c, http.StatusNotFound, "资源不存在")
		default:
			slog.Warn("读取资源失败", "key", c.Param("key"), "error", err)
			fail(c, http.StatusBadGateway, "读取资源失败")
		}
		return
	}
	defer object.Body.Close()

	header := c.Writer.Header()
	header.Set("Content-Type", object.ContentType)
	header.Set("Content-Length", strconv.FormatInt(object.Size, 10))
	header.Set("Cache-Control", "private, max-age=3600")
	if !object.ModTime.IsZero() {
		header.Set("Last-Modified", object.ModTime.UTC().Format(http.TimeFormat))
	}
	if object.ETag != "" {
		header.Set("ETag", object.ETag)
		if c.GetHeader("If-None-Match") == object.ETag {
			c.Status(http.StatusNotModified)
			return
		}
	}
	if strings.Contains(object.ContentType, "svg") {
		header.Set("Content-Disposition", "attachment")
	}

	c.Status(http.StatusOK)
	if _, err := io.Copy(c.Writer, object.Body); err != nil {
		slog.Warn("传输资源中断", "key", c.Param("key"), "error", err)
	}
}
