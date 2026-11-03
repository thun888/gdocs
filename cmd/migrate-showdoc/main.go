package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/thun888/gdocs-go/internal/config"
	"github.com/thun888/gdocs-go/internal/migrate/showdoc"
)

func main() {
	configPath := flag.String("c", "config.yaml", "配置文件路径")
	dbPath := flag.String("db", "data/showdoc.db", "ShowDoc SQLite 数据库路径")
	uploadsDir := flag.String("uploads", "data/uploads", "ShowDoc 附件目录，留空则不改写附件")
	sourceSlug := flag.String("source", "", "目标来源 slug，默认取配置第一个来源")
	sourceName := flag.String("source-name", "", "迁移后更新来源名称")
	history := flag.Bool("history", true, "导入 ShowDoc 历史版本")
	push := flag.Bool("push", true, "推送仓库")
	reset := flag.Bool("reset", false, "远端已有提交时强制覆盖")
	sync := flag.Bool("sync", true, "推送后重建文档索引")
	dryRun := flag.Bool("dry-run", false, "只打印迁移计划")
	resync := flag.Bool("resync", false, "只重建文档索引")
	verbose := flag.Bool("v", false, "输出页面与排序明细")
	workDir := flag.String("workdir", "", "临时仓库父目录")
	keep := flag.Bool("keep", false, "保留临时仓库")
	clearProxy := flag.Bool("clear-git-proxy", false, "执行 git 时清空 http.proxy")
	flag.Parse()

	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	cfg, err := config.Load(*configPath)
	if err != nil {
		slog.Error("加载配置失败", "error", err)
		os.Exit(1)
	}
	if *sourceSlug == "" {
		if len(cfg.Sources) == 0 {
			slog.Error("配置中没有来源")
			os.Exit(1)
		}
		*sourceSlug = cfg.Sources[0].Slug
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if *resync {
		if err := showdoc.Resync(ctx, cfg, *sourceSlug, *sourceName); err != nil {
			slog.Error("重建索引失败", "error", err)
			os.Exit(1)
		}
		slog.Info("索引重建完成")
		return
	}

	db, err := showdoc.Open(*dbPath)
	if err != nil {
		slog.Error("打开 ShowDoc 数据库失败", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	err = showdoc.Run(ctx, showdoc.RunOptions{
		Config:        cfg,
		DB:            db,
		SourceSlug:    *sourceSlug,
		SourceName:    *sourceName,
		UploadsDir:    *uploadsDir,
		WorkDir:       *workDir,
		History:       *history,
		Push:          *push,
		Reset:         *reset,
		Sync:          *sync,
		DryRun:        *dryRun,
		Verbose:       *verbose,
		KeepWorkDir:   *keep,
		ClearGitProxy: *clearProxy,
	})
	if err != nil {
		slog.Error("迁移失败", "error", err)
		os.Exit(1)
	}
	slog.Info("迁移完成")
}
