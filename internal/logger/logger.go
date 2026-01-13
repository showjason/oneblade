package logger

import (
	"fmt"
	"log"
	"log/slog"
	"os"

	"github.com/oneblade/config"
)

// Initialize 初始化全局日志配置
func Initialize(cfg *config.Config) {
	var level slog.Level
	switch cfg.Log.Level {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{
		Level: level,
	}

	var writer *os.File = os.Stdout
	if cfg.Log.Output != "" && cfg.Log.Output != "stdout" {
		f, err := os.OpenFile(cfg.Log.Output, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to open log file %s: %v\n", cfg.Log.Output, err)
		} else {
			writer = f
		}
	}

	var handler slog.Handler
	if cfg.Log.Format == "json" {
		handler = slog.NewJSONHandler(writer, opts)
	} else {
		handler = slog.NewTextHandler(writer, opts)
	}

	logger := slog.New(handler)
	slog.SetDefault(logger)

	// Redirect standard log to slog
	// Note: slog.NewLogLogger returns a *log.Logger that writes to the handler
	// We set it as the default standard logger
	log.SetOutput(slog.NewLogLogger(handler, level).Writer())
}
