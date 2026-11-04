package source

import (
	"context"
	"errors"
	"io"
	"time"
)

var (
	ErrNotFound = errors.New("目标不存在")
	ErrConflict = errors.New("版本冲突")
	ErrAuth     = errors.New("仓库认证失败")
	ErrUpstream = errors.New("上游服务异常")
)

type Entry struct {
	Path string
	Dir  bool
	SHA  string
	Size int64
}

type File struct {
	Path    string
	SHA     string
	Content []byte
}

type Commit struct {
	SHA     string    `json:"sha"`
	Message string    `json:"message"`
	Author  string    `json:"author"`
	Email   string    `json:"email"`
	Date    time.Time `json:"date"`
	URL     string    `json:"url"`
}

type FileDiff struct {
	Filename  string `json:"filename"`
	Status    string `json:"status"`
	Additions int    `json:"additions"`
	Deletions int    `json:"deletions"`
	Patch     string `json:"patch"`
}

type CommitDetail struct {
	Commit
	Files []FileDiff `json:"files"`
}

type Identity struct {
	Name  string
	Email string
}

type Source interface {
	Tree(ctx context.Context) ([]Entry, error)
	Read(ctx context.Context, path string) (*File, error)
	Write(ctx context.Context, path string, content []byte, sha, message string, id Identity) (string, error)
	Create(ctx context.Context, path string, content []byte, message string, id Identity) (string, error)
	Delete(ctx context.Context, path, sha, message string, id Identity) error
	Move(ctx context.Context, from, to, sha, message string, id Identity) error
	History(ctx context.Context, path string, limit int) ([]Commit, error)
	Commit(ctx context.Context, sha string) (*CommitDetail, error)
	Compare(ctx context.Context, base, head string) ([]FileDiff, error)
	Recent(ctx context.Context, limit int) ([]Commit, error)
	Archive(ctx context.Context) (io.ReadCloser, error)
}
