package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/go-kratos/blades"
	"github.com/go-kratos/blades/contrib/openai"

	"github.com/oneblade/agent"
	"github.com/oneblade/collector"
	"github.com/oneblade/config"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 加载配置
	loader, err := config.NewLoader("./configs")
	if err != nil {
		log.Fatalf("failed to create config loader: %v", err)
	}

	cfg, err := loader.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	log.Printf("[main] loaded config for env: %s", cfg.App.Env)

	// 初始化 LLM Model（仍然从环境变量读取，因为是敏感信息）
	model := openai.NewModel(os.Getenv("OPENAI_MODEL"), openai.Config{
		APIKey: os.Getenv("OPENAI_API_KEY"),
	})

	// 初始化 Collector Registry
	registry := collector.NewRegistry()
	if err := registry.InitFromConfig(loader); err != nil {
		log.Fatalf("failed to init collectors: %v", err)
	}
	defer registry.Close()

	// 创建 Orchestrator Agent
	orchestrator, err := agent.NewOrchestratorAgent(agent.OrchestratorConfig{
		Model:    model,
		Collectors: registry.All(),
	})
	if err != nil {
		log.Fatalf("failed to create orchestrator agent: %v", err)
	}

	// 创建 Runner
	runner := agent.NewInspectionRunner(orchestrator)

	// 执行巡检
	input := blades.UserMessage("请对过去24小时的系统状态进行全面巡检，生成巡检报告")

	output, err := runner.Run(ctx, input)
	if err != nil {
		log.Fatalf("failed to run inspection: %v", err)
	}

	log.Println("巡检报告:")
	log.Println(output.Text())

	// 等待退出信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("[main] shutting down...")
}
