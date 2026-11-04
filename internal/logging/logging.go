package logging

import (
	"log/slog"
	"os"

	"github.com/thun888/gdocs-go/internal/config"
)

func Setup(cfg config.Log) {
	level := slog.LevelInfo
	if cfg.Debug {
		level = slog.LevelDebug
	}

	opts := &slog.HandlerOptions{Level: level}
	var handler slog.Handler
	if cfg.Format == "json" {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}
	slog.SetDefault(slog.New(handler))
}
