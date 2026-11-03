package httpapi

import (
	"io"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
)

func (a *API) handleUpload(c *gin.Context) {
	if a.upload == nil {
		fail(c, http.StatusServiceUnavailable, "图片存储未配置")
		return
	}

	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, a.upload.MaxBytes()+1<<20)
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		fail(c, http.StatusBadRequest, "读取上传文件失败")
		return
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, a.upload.MaxBytes()+1))
	if err != nil {
		fail(c, http.StatusBadRequest, "读取上传文件失败")
		return
	}

	url, key, err := a.upload.Put(c.Request.Context(), header.Filename, data)
	if err != nil {
		slog.Warn("上传失败", "error", err, "file", header.Filename)
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	ok(c, gin.H{"url": url, "key": key, "size": len(data)})
}
