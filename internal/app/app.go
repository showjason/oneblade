package app

import (
	"context"
	"fmt"
	"time"

	"github.com/go-kratos/blades"

	"github.com/oneblade/agent"
	"github.com/oneblade/config"
	"github.com/oneblade/service"
)

// Application 应用程序核心结构体
// 管理所有依赖和生命周期
type Application struct {
	cfg          *config.Loader
	llmConfig    *config.LLMConfig
	registry     *service.Registry
	model        blades.ModelProvider
	orchestrator blades.Agent
	runner       *blades.Runner
}

func NewApplication(configPath string) (*Application, error) {
	loader, err := config.NewLoader(configPath)
	if err != nil {
		return nil, fmt.Errorf("create config loader: %w", err)
	}

	return &Application{
		cfg: loader,
	}, nil
}

// Initialize 初始化所有依赖
func (a *Application) Initialize(ctx context.Context) error {
	// Step 1: 加载配置
	cfg, err := a.cfg.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	a.llmConfig = &cfg.LLM

	// Step 2: 初始化 LLM Model
	model, err := buildModel(a.llmConfig)
	if err != nil {
		return fmt.Errorf("build model: %w", err)
	}
	a.model = model

	// Step 3: 初始化 Service Registry
	registry := service.NewRegistry()
	if err := registry.InitFromConfig(a.cfg); err != nil {
		return fmt.Errorf("init registry: %w", err)
	}
	a.registry = registry

	// Step 4: 创建 Orchestrator Agent
	orchestrator, err := agent.NewOrchestratorAgent(agent.OrchestratorConfig{
		Model:    a.model,
		Services: a.registry.All(),
	})
	if err != nil {
		return fmt.Errorf("create orchestrator: %w", err)
	}
	a.orchestrator = orchestrator

	// Step 5: 创建 Runner
	a.runner = agent.NewInspectionRunner(a.orchestrator)

	return nil
}

// Shutdown 优雅关闭
// 按照 runner -> registry -> model 的顺序释放资源
func (a *Application) Shutdown(ctx context.Context) error {
	var errs []error

	// 1. 关闭 runner（如果支持 Close 接口）
	// blades.Runner 目前不需要显式关闭

	// 2. 关闭 registry
	if a.registry != nil {
		if err := a.registry.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close registry: %w", err))
		}
	}

	// 3. 关闭 model（如果支持 Closer 接口）
	if closer, ok := a.model.(interface{ Close() error }); ok {
		if err := closer.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close model: %w", err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("shutdown errors: %v", errs)
	}
	return nil
}

// Run 执行巡检任务
func (a *Application) Run(ctx context.Context, input *blades.Message) (*blades.Message, error) {
	if a.runner == nil {
		return nil, fmt.Errorf("application not initialized")
	}
	return a.runner.Run(ctx, input)
}

// GetLLMConfig 获取 LLM 配置
func (a *Application) GetLLMConfig() *config.LLMConfig {
	return a.llmConfig
}

// GetRegistry 获取服务注册表
func (a *Application) GetRegistry() *service.Registry {
	return a.registry
}

// ShutdownWithTimeout 带超时的优雅关闭
func (a *Application) ShutdownWithTimeout(timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return a.Shutdown(ctx)
}
