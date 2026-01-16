package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/oneblade/internal/app"
	"github.com/oneblade/internal/repl"
)

func main() {
	configPath := flag.String("config", "./config.toml", "配置文件路径")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, *configPath); err != nil {
		slog.Error("application error", "error", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, configPath string) error {
	// Build application
	application, err := app.NewApplication(configPath)
	if err != nil {
		return fmt.Errorf("create application: %w", err)
	}
	defer func() {
		slog.Info("[main] shutting down...")
		if err := application.ShutdownWithTimeout(5 * time.Second); err != nil {
			slog.Error("shutdown error", "error", err)
		}
	}()

	// Initialize application
	if err := application.Initialize(ctx); err != nil {
		return fmt.Errorf("initialize application: %w", err)
	}
	slog.Info("[main] application initialized", "config_path", configPath)

	// Create and run REPL
	r, err := repl.NewREPL(ctx, repl.WithApplication(application))
	if err != nil {
		return fmt.Errorf("create repl: %w", err)
	}
	defer r.Close()

	return r.Run()
}
