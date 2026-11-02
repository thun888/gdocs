package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/thun888/zdocs-go/internal/auth"
	"github.com/thun888/zdocs-go/internal/config"
	"github.com/thun888/zdocs-go/internal/httpapi"
	"github.com/thun888/zdocs-go/internal/server"
	"github.com/thun888/zdocs-go/internal/store"
)

func main() {
	configPath := flag.String("c", "config.yaml", "配置文件路径")
	migrateOnly := flag.Bool("migrate", false, "执行数据库迁移后退出")
	flag.Parse()

	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	cfg, err := config.Load(*configPath)
	if err != nil {
		slog.Error("加载配置失败", "error", err)
		os.Exit(1)
	}

	db, err := store.Open(cfg.Database)
	if err != nil {
		slog.Error("连接数据库失败", "error", err)
		os.Exit(1)
	}

	if *migrateOnly || cfg.Database.AutoMigrate {
		if err := store.Migrate(db); err != nil {
			slog.Error("数据库迁移失败", "error", err)
			os.Exit(1)
		}
		slog.Info("数据库迁移完成")
		if *migrateOnly {
			return
		}
	}

	st := store.New(db)
	authSvc, err := auth.New(cfg, st)
	if err != nil {
		slog.Error("初始化认证服务失败", "error", err)
		os.Exit(1)
	}

	srv := &http.Server{
		Addr:              cfg.Server.Addr,
		Handler:           server.New(cfg, db, httpapi.New(cfg, st, authSvc)),
		ReadHeaderTimeout: 5 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		slog.Info("服务启动", "addr", cfg.Server.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("服务异常退出", "error", err)
			stop()
		}
	}()

	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("优雅关闭失败", "error", err)
	}
	slog.Info("服务已停止")
}
