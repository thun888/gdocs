package store

import (
	"context"
	"time"
)

func (s *Store) ListSources(ctx context.Context, onlyEnabled bool) ([]Source, error) {
	var list []Source
	q := s.db.WithContext(ctx).Order("sort, id")
	if onlyEnabled {
		q = q.Where("enabled = ?", true)
	}
	err := q.Find(&list).Error
	return list, err
}

func (s *Store) SourceBySlug(ctx context.Context, slug string) (*Source, error) {
	var src Source
	if err := s.db.WithContext(ctx).Where("slug = ?", slug).First(&src).Error; err != nil {
		return nil, err
	}
	return &src, nil
}

func (s *Store) SourceByID(ctx context.Context, id uint64) (*Source, error) {
	var src Source
	if err := s.db.WithContext(ctx).First(&src, id).Error; err != nil {
		return nil, err
	}
	return &src, nil
}

func (s *Store) CreateSource(ctx context.Context, src *Source) error {
	src.CreatedAt = time.Now()
	return s.db.WithContext(ctx).Create(src).Error
}

func (s *Store) UpdateSource(ctx context.Context, src *Source) error {
	return s.db.WithContext(ctx).Model(&Source{}).Where("id = ?", src.ID).Updates(map[string]any{
		"slug":      src.Slug,
		"name":      src.Name,
		"base_url":  src.BaseURL,
		"token_enc": src.TokenEnc,
		"owner":     src.Owner,
		"repo":      src.Repo,
		"branch":    src.Branch,
		"root":      src.Root,
		"sort":      src.Sort,
		"enabled":   src.Enabled,
	}).Error
}

func (s *Store) DeleteSource(ctx context.Context, id uint64) error {
	return s.db.WithContext(ctx).Delete(&Source{}, id).Error
}
