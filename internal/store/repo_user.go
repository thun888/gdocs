package store

import (
	"context"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Store struct {
	db *gorm.DB
}

func New(db *gorm.DB) *Store {
	return &Store{db: db}
}

func (s *Store) DB() *gorm.DB { return s.db }

func (s *Store) UpsertUser(ctx context.Context, u *User) error {
	return s.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "subject"}},
		DoUpdates: clause.Assignments(map[string]any{
			"username":     u.Username,
			"display_name": u.DisplayName,
			"email":        u.Email,
			"class":        u.Class,
			"role":         u.Role,
		}),
	}).Create(u).Error
}

func (s *Store) UserBySubject(ctx context.Context, subject string) (*User, error) {
	var u User
	if err := s.db.WithContext(ctx).Where("subject = ?", subject).First(&u).Error; err != nil {
		return nil, err
	}
	return &u, nil
}

func (s *Store) UserByID(ctx context.Context, id uint64) (*User, error) {
	var u User
	if err := s.db.WithContext(ctx).First(&u, id).Error; err != nil {
		return nil, err
	}
	return &u, nil
}

func (s *Store) TouchLogin(ctx context.Context, id uint64, at time.Time) error {
	return s.db.WithContext(ctx).Model(&User{}).Where("id = ?", id).
		Updates(map[string]any{"last_login_at": at}).Error
}

func (s *Store) UpdateUserRole(ctx context.Context, id uint64, role string) error {
	return s.db.WithContext(ctx).Model(&User{}).Where("id = ?", id).
		Update("role", role).Error
}

func (s *Store) ListUsers(ctx context.Context) ([]User, error) {
	var users []User
	err := s.db.WithContext(ctx).Order("id").Find(&users).Error
	return users, err
}
