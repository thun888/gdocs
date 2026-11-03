package store

import (
	"fmt"

	"github.com/pressly/goose/v3"
	"gorm.io/gorm"

	"github.com/thun888/gdocs-go/migrations"
)

func Migrate(db *gorm.DB) error {
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("获取连接池: %w", err)
	}

	goose.SetBaseFS(migrations.FS)
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("设置迁移方言: %w", err)
	}
	if err := goose.Up(sqlDB, "."); err != nil {
		return fmt.Errorf("执行迁移: %w", err)
	}
	return nil
}
