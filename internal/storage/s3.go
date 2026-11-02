package storage

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"mime"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"github.com/thun888/zdocs-go/internal/config"
)

type Uploader struct {
	client     *minio.Client
	bucket     string
	prefix     string
	publicBase string
	maxBytes   int64
	allow      map[string]bool
}

func New(cfg config.S3) (*Uploader, error) {
	if cfg.Endpoint == "" || cfg.Bucket == "" {
		return nil, fmt.Errorf("S3 未配置")
	}
	secure := strings.HasPrefix(strings.ToLower(cfg.Endpoint), "https://")
	endpoint := strings.TrimPrefix(strings.TrimPrefix(cfg.Endpoint, "https://"), "http://")

	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: secure,
		Region: cfg.Region,
	})
	if err != nil {
		return nil, err
	}

	allow := map[string]bool{}
	for _, item := range cfg.AllowTypes {
		allow[strings.ToLower(item)] = true
	}
	maxBytes := int64(cfg.MaxSizeMB) << 20
	if maxBytes <= 0 {
		maxBytes = 10 << 20
	}

	return &Uploader{
		client:     client,
		bucket:     cfg.Bucket,
		prefix:     strings.Trim(cfg.PathPrefix, "/"),
		publicBase: strings.TrimSuffix(cfg.PublicBase, "/"),
		maxBytes:   maxBytes,
		allow:      allow,
	}, nil
}

func (u *Uploader) MaxBytes() int64 { return u.maxBytes }

func (u *Uploader) Put(ctx context.Context, filename string, data []byte, declared string) (string, string, error) {
	if int64(len(data)) > u.maxBytes {
		return "", "", fmt.Errorf("文件超过大小限制")
	}
	if len(data) == 0 {
		return "", "", fmt.Errorf("文件为空")
	}

	contentType := http.DetectContentType(data)
	if declared != "" {
		if parsed, _, err := mime.ParseMediaType(declared); err == nil {
			contentType = parsed
		}
	}
	if len(u.allow) > 0 && !u.allow[contentType] {
		return "", "", fmt.Errorf("不支持的文件类型: %s", contentType)
	}

	sum := sha256.Sum256(data)
	name := hex.EncodeToString(sum[:8])
	ext := strings.ToLower(path.Ext(filename))
	key := path.Join(u.prefix, time.Now().Format("2006/01"), name+ext)
	if u.prefix == "" {
		key = path.Join(time.Now().Format("2006/01"), name+ext)
	}

	_, err := u.client.PutObject(ctx, u.bucket, key, bytes.NewReader(data), int64(len(data)), minio.PutObjectOptions{ContentType: contentType})
	if err != nil {
		return "", "", err
	}

	url := "/" + key
	if u.publicBase != "" {
		url = u.publicBase + "/" + key
	}
	return url, key, nil
}
