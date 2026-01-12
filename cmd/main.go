package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/go-kratos/blades"
	"github.com/go-kratos/blades/memory"

	"github.com/oneblade/internal/app"
)

func main() {
	// 1. 解析命令行参数
	configPath := flag.String("config", "./config.toml", "配置文件路径")
	flag.Parse()

	// 2. 创建 context（支持信号中断）
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// 3. 构建应用
	application, err := app.NewApplication(*configPath)
	if err != nil {
		slog.Error("failed to create application", "error", err)
		return
	}

	// 4. 设置优雅关闭
	defer func() {
		slog.Info("[main] shutting down...")
		if err := application.ShutdownWithTimeout(5 * time.Second); err != nil {
			slog.Error("shutdown error", "error", err)
		}
	}()

	// 5. 初始化应用
	if err := application.Initialize(ctx); err != nil {
		slog.Error("failed to initialize application", "error", err)
		return
	}

	slog.Info("[main] application initialized", "config_path", *configPath)

	session := blades.NewSession()
	memStore := application.MemoryStore()
	var lastSavedIdx int

	slog.Info("进入多轮对话模式，输入 quit/exit 退出。")
	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("> ")
		if !scanner.Scan() {
			break
		}
		text := strings.TrimSpace(scanner.Text())
		if text == "" {
			continue
		}
		switch strings.ToLower(text) {
		case "exit", "quit":
			return
		}

		output, err := application.Run(ctx, blades.UserMessage(text), blades.WithSession(session))
		if err != nil {
			if ctx.Err() != nil {
				slog.Error("[main] interrupted", "error", ctx.Err())
				return
			}
			slog.Error("run failed", "error", err)
			continue
		}

		fmt.Println(output.Text())

		if memStore != nil {
			history := session.History()
			for _, m := range history[lastSavedIdx:] {
				if m == nil {
					continue
				}
				switch m.Role {
				case blades.RoleUser, blades.RoleAssistant:
					_ = memStore.AddMemory(ctx, &memory.Memory{Content: m})
				}
			}
			lastSavedIdx = len(history)
		}
	}

	if err := scanner.Err(); err != nil {
		slog.Error("read stdin failed", "error", err)
	}
}
