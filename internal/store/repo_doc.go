package store

import (
	"context"

	"gorm.io/gorm"
)

func (s *Store) ReplaceDocMeta(ctx context.Context, sourceID uint64, metas []DocMeta) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("source_id = ?", sourceID).Delete(&DocMeta{}).Error; err != nil {
			return err
		}
		if len(metas) == 0 {
			return nil
		}
		return tx.CreateInBatches(metas, 200).Error
	})
}

func (s *Store) ListDocMeta(ctx context.Context, sourceID uint64) ([]DocMeta, error) {
	var metas []DocMeta
	err := s.db.WithContext(ctx).Where("source_id = ?", sourceID).Order("path").Find(&metas).Error
	return metas, err
}

func (s *Store) DocMetaByPath(ctx context.Context, sourceID uint64, path string) (*DocMeta, error) {
	var meta DocMeta
	if err := s.db.WithContext(ctx).Where("source_id = ? AND path = ?", sourceID, path).First(&meta).Error; err != nil {
		return nil, err
	}
	return &meta, nil
}

func (s *Store) SettingGet(ctx context.Context, key string) (string, error) {
	var setting Setting
	if err := s.db.WithContext(ctx).Where("key = ?", key).First(&setting).Error; err != nil {
		return "", err
	}
	return setting.Value, nil
}

func (s *Store) SettingSet(ctx context.Context, key, value string) error {
	return s.db.WithContext(ctx).Save(&Setting{Key: key, Value: value}).Error
}
