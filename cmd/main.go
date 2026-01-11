package main

import (
	"context"
	"flag"
	"log"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-kratos/blades"

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
		log.Printf("failed to create application: %v", err)
		return
	}

	// 4. 设置优雅关闭
	defer func() {
		log.Println("[main] shutting down...")
		if err := application.ShutdownWithTimeout(5 * time.Second); err != nil {
			log.Printf("shutdown error: %v", err)
		}
	}()

	// 5. 初始化应用
	if err := application.Initialize(ctx); err != nil {
		log.Printf("failed to initialize application: %v", err)
		return
	}

	log.Printf("[main] application initialized, config loaded from: %s", *configPath)

	// 6. 执行巡检
	input := blades.UserMessage("请对过去24小时的系统状态进行全面巡检，生成巡检报告")

	output, err := application.Run(ctx, input)
	if err != nil {
		// 检查是否是被中断
		if ctx.Err() != nil {
			log.Printf("[main] inspection interrupted: %v", ctx.Err())
		} else {
			log.Printf("failed to run inspection: %v", err)
		}
		return
	}

	log.Println("巡检报告:")
	log.Println(output.Text())
}
