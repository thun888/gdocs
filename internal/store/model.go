package store

import "time"

type User struct {
	ID          uint64    `gorm:"primaryKey"`
	Subject     string    `gorm:"size:32;not null;uniqueIndex"`
	Username    string    `gorm:"size:64;not null"`
	DisplayName string    `gorm:"size:128;not null;default:''"`
	Email       string    `gorm:"size:255;not null;default:''"`
	Class       string    `gorm:"size:128;not null;default:''"`
	Role        string    `gorm:"size:16;not null;default:viewer"`
	CreatedAt   time.Time `gorm:"not null"`
	LastLoginAt *time.Time
}

type Group struct {
	ID         uint64 `gorm:"primaryKey"`
	Code       string `gorm:"size:128;not null;uniqueIndex"`
	Name       string `gorm:"size:64;not null"`
	Type       string `gorm:"size:16;not null;default:''"`
	Role       string `gorm:"size:16;not null;default:viewer"`
	Ignored    bool   `gorm:"not null;default:false"`
	LastSeenAt *time.Time
}

func (Group) TableName() string { return "groups" }

type UserGroup struct {
	UserID  uint64 `gorm:"primaryKey"`
	GroupID uint64 `gorm:"primaryKey"`
}

type Session struct {
	ID              string    `gorm:"primaryKey;size:64"`
	UID             uint64    `gorm:"not null;index"`
	AccessTokenEnc  []byte    `gorm:"type:bytea"`
	RefreshTokenEnc []byte    `gorm:"type:bytea"`
	ExpiresAt       time.Time `gorm:"not null;index"`
	LastSeenAt      time.Time `gorm:"not null"`
	IP              string    `gorm:"size:64;not null;default:''"`
	UA              string    `gorm:"not null;default:''"`
}

type Source struct {
	ID        uint64    `gorm:"primaryKey"`
	Slug      string    `gorm:"size:64;not null;uniqueIndex"`
	Name      string    `gorm:"size:128;not null"`
	BaseURL   string    `gorm:"size:255;not null"`
	TokenEnc  []byte    `gorm:"type:bytea"`
	Owner     string    `gorm:"size:128;not null"`
	Repo      string    `gorm:"size:128;not null"`
	Branch    string    `gorm:"size:128;not null;default:main"`
	Root      string    `gorm:"size:255;not null;default:''"`
	Sort      int       `gorm:"not null;default:0"`
	Enabled   bool      `gorm:"not null;default:true"`
	CreatedAt time.Time `gorm:"not null"`
}

type DocMeta struct {
	SourceID  uint64    `gorm:"primaryKey"`
	Path      string    `gorm:"primaryKey;size:1024"`
	BlobSHA   string    `gorm:"size:64;not null;default:''"`
	Title     string    `gorm:"size:255;not null;default:''"`
	Sort      int       `gorm:"column:sort;not null;default:0"`
	Public    *bool     `gorm:"column:is_public"`
	Draft     bool      `gorm:"not null;default:false"`
	UpdatedAt time.Time `gorm:"not null"`
}

type Setting struct {
	Key   string `gorm:"primaryKey;size:64"`
	Value string `gorm:"not null;default:''"`
}

type AuditLog struct {
	ID        uint64    `gorm:"primaryKey"`
	UID       *uint64   `gorm:"index"`
	Action    string    `gorm:"size:64;not null"`
	Target    string    `gorm:"size:255;not null;default:''"`
	Detail    []byte    `gorm:"type:jsonb"`
	IP        string    `gorm:"size:64;not null;default:''"`
	CreatedAt time.Time `gorm:"not null"`
}
