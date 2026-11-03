package httpapi

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/thun888/gdocs-go/internal/auth"
	"github.com/thun888/gdocs-go/internal/doc"
	"github.com/thun888/gdocs-go/internal/source"
)

func (a *API) docVisible(c *gin.Context, client *source.Client, p string) (bool, error) {
	file, err := client.Read(c.Request.Context(), p)
	if err != nil {
		return false, err
	}
	fm := doc.Parse(file.Content)
	_, loggedIn := auth.User(c)
	if fm.Meta.Draft {
		return loggedIn, nil
	}
	if loggedIn {
		return true, nil
	}
	return effectivePublic(fm.Meta.Public, a.cfg.Server.PublicRead), nil
}

func (a *API) handleHistory(c *gin.Context) {
	src, valid := a.source(c)
	if !valid {
		return
	}
	p, err := doc.Clean(c.Query("path"))
	if err != nil || p == "" || !doc.IsDoc(p) {
		fail(c, http.StatusBadRequest, "路径不合法")
		return
	}
	client, err := a.doc.Client(c.Request.Context(), src)
	if err != nil {
		fail(c, http.StatusInternalServerError, "仓库配置不可用")
		return
	}
	visible, err := a.docVisible(c, client, p)
	if err != nil {
		a.sourceError(c, err)
		return
	}
	if !visible {
		fail(c, http.StatusNotFound, "文档不存在")
		return
	}

	commits, err := client.History(c.Request.Context(), p, 30)
	if err != nil {
		a.sourceError(c, err)
		return
	}
	ok(c, gin.H{"commits": commits})
}

func (a *API) handleCommit(c *gin.Context) {
	src, valid := a.source(c)
	if !valid {
		return
	}
	client, err := a.doc.Client(c.Request.Context(), src)
	if err != nil {
		fail(c, http.StatusInternalServerError, "仓库配置不可用")
		return
	}
	detail, err := client.Commit(c.Request.Context(), c.Param("sha"))
	if err != nil {
		a.sourceError(c, err)
		return
	}
	detail.Files = filesUnderRoot(client.Root(), detail.Files)
	ok(c, gin.H{"commit": detail})
}

func (a *API) handleCompare(c *gin.Context) {
	src, valid := a.source(c)
	if !valid {
		return
	}
	base, head := c.Query("base"), c.Query("head")
	if base == "" || head == "" {
		fail(c, http.StatusBadRequest, "缺少对比版本")
		return
	}
	client, err := a.doc.Client(c.Request.Context(), src)
	if err != nil {
		fail(c, http.StatusInternalServerError, "仓库配置不可用")
		return
	}
	files, err := client.Compare(c.Request.Context(), base, head)
	if err != nil {
		a.sourceError(c, err)
		return
	}
	ok(c, gin.H{"files": filesUnderRoot(client.Root(), files)})
}

func filesUnderRoot(root string, files []source.FileDiff) []source.FileDiff {
	if root == "" {
		return files
	}
	out := make([]source.FileDiff, 0, len(files))
	for _, f := range files {
		if _, ok := doc.UnderRoot(f.Filename, root); ok {
			out = append(out, f)
		}
	}
	return out
}

func (a *API) handleRecent(c *gin.Context) {
	src, valid := a.source(c)
	if !valid {
		return
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	if _, loggedIn := auth.User(c); !loggedIn && !a.cfg.Server.PublicRead {
		ok(c, gin.H{"commits": []any{}})
		return
	}
	client, err := a.doc.Client(c.Request.Context(), src)
	if err != nil {
		fail(c, http.StatusInternalServerError, "仓库配置不可用")
		return
	}
	commits, err := client.Recent(c.Request.Context(), limit)
	if err != nil {
		a.sourceError(c, err)
		return
	}
	ok(c, gin.H{"commits": commits})
}
