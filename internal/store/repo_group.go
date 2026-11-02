package store

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
)

type GroupSpec struct {
	Code string
	Name string
	Type string
}

func (s *Store) SyncGroups(ctx context.Context, userID uint64, specs []GroupSpec) ([]Group, error) {
	out := make([]Group, 0, len(specs))
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := time.Now()
		ids := make([]uint64, 0, len(specs))

		for _, spec := range specs {
			var g Group
			err := tx.Where("code = ?", spec.Code).First(&g).Error
			switch {
			case errors.Is(err, gorm.ErrRecordNotFound):
				g = Group{Code: spec.Code, Name: spec.Name, Type: spec.Type, Role: "viewer", LastSeenAt: &now}
				if err := tx.Create(&g).Error; err != nil {
					return err
				}
			case err != nil:
				return err
			default:
				if err := tx.Model(&g).Updates(map[string]any{
					"name":         spec.Name,
					"type":         spec.Type,
					"last_seen_at": now,
				}).Error; err != nil {
					return err
				}
				g.Name, g.Type, g.LastSeenAt = spec.Name, spec.Type, &now
			}
			ids = append(ids, g.ID)
			out = append(out, g)
		}

		if err := tx.Where("user_id = ?", userID).Delete(&UserGroup{}).Error; err != nil {
			return err
		}
		for _, id := range ids {
			if err := tx.Create(&UserGroup{UserID: userID, GroupID: id}).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Store) GroupsByUser(ctx context.Context, userID uint64) ([]Group, error) {
	var groups []Group
	err := s.db.WithContext(ctx).
		Joins("JOIN user_groups ug ON ug.group_id = groups.id").
		Where("ug.user_id = ?", userID).
		Order("groups.code").
		Find(&groups).Error
	return groups, err
}

type GroupRow struct {
	Group
	Members int64 `json:"members"`
}

func (s *Store) ListGroups(ctx context.Context) ([]GroupRow, error) {
	var rows []GroupRow
	err := s.db.WithContext(ctx).
		Model(&Group{}).
		Select("groups.*, count(ug.user_id) as members").
		Joins("LEFT JOIN user_groups ug ON ug.group_id = groups.id").
		Group("groups.id").
		Order("groups.code").
		Scan(&rows).Error
	return rows, err
}

func (s *Store) GroupByID(ctx context.Context, id uint64) (*Group, error) {
	var g Group
	if err := s.db.WithContext(ctx).First(&g, id).Error; err != nil {
		return nil, err
	}
	return &g, nil
}

func (s *Store) UpdateGroup(ctx context.Context, id uint64, role string, ignored bool) error {
	return s.db.WithContext(ctx).Model(&Group{}).Where("id = ?", id).
		Updates(map[string]any{"role": role, "ignored": ignored}).Error
}

func (s *Store) PurgeStaleGroups(ctx context.Context, olderThan time.Time) error {
	return s.db.WithContext(ctx).Where("last_seen_at IS NULL OR last_seen_at < ?", olderThan).
		Delete(&Group{}).Error
}
