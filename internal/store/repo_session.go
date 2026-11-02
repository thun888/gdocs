package store

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
)

func (s *Store) CreateSession(ctx context.Context, sess *Session) error {
	return s.db.WithContext(ctx).Create(sess).Error
}

func (s *Store) SessionByID(ctx context.Context, id string) (*Session, error) {
	var sess Session
	if err := s.db.WithContext(ctx).Where("id = ?", id).First(&sess).Error; err != nil {
		return nil, err
	}
	return &sess, nil
}

func (s *Store) UpdateSessionTokens(ctx context.Context, id string, access, refresh []byte, expires, now time.Time) error {
	return s.db.WithContext(ctx).Model(&Session{}).Where("id = ?", id).Updates(map[string]any{
		"access_token_enc":  access,
		"refresh_token_enc": refresh,
		"expires_at":        expires,
		"last_seen_at":      now,
	}).Error
}

func (s *Store) TouchSession(ctx context.Context, id string, now time.Time) error {
	return s.db.WithContext(ctx).Model(&Session{}).Where("id = ?", id).
		Update("last_seen_at", now).Error
}

func (s *Store) DeleteSession(ctx context.Context, id string) error {
	return s.db.WithContext(ctx).Where("id = ?", id).Delete(&Session{}).Error
}

func (s *Store) DeleteUserSessions(ctx context.Context, uid uint64) error {
	return s.db.WithContext(ctx).Where("uid = ?", uid).Delete(&Session{}).Error
}

func (s *Store) PurgeExpiredSessions(ctx context.Context, now time.Time) error {
	return s.db.WithContext(ctx).Where("expires_at < ?", now).Delete(&Session{}).Error
}

func IsNotFound(err error) bool {
	return errors.Is(err, gorm.ErrRecordNotFound)
}
