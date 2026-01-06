package app

import (
	"context"
	"fmt"
	"time"

	"github.com/go-kratos/blades"

	"github.com/oneblade/agent"
	"github.com/oneblade/config"
	"github.com/oneblade/internal/llm"
	"github.com/oneblade/service"
)

// Application 应用程序核心结构体
// 管理所有依赖和生命周期
type Application struct {
	cfg          *config.Loader
	llmConfig    *config.LLMConfig
	registry     *service.Registry
	models       map[string]blades.ModelProvider
	orchestrator blades.Agent
	runner       *blades.Runner
}

func NewApplication(configPath string) (*Application, error) {
	loader := config.NewLoader(configPath)

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

	// Step 2: 初始化 Service Registry
	registry := service.NewRegistry()
	if err := registry.InitFromConfig(a.cfg); err != nil {
		return fmt.Errorf("init registry: %w", err)
	}
	a.registry = registry

	// Step 3: 初始化 LLM Factory（按 agentName 严格构建）
	factory := llm.NewFactory()
	models := make(map[string]blades.ModelProvider)
	resolveModel := func(agentName string) (blades.ModelProvider, error) {
		if m, ok := models[agentName]; ok {
			return m, nil
		}
		agentCfg, err := a.llmConfig.GetAgentStrict(agentName)
		if err != nil {
			return nil, err
		}
		m, err := factory.Build(ctx, *agentCfg)
		if err != nil {
			return nil, fmt.Errorf("build model for %s: %w", agentName, err)
		}
		models[agentName] = m
		return m, nil
	}
	a.models = models

	// Step 4: 创建 Orchestrator Agent
	orchestrator, err := agent.NewOrchestratorAgent(agent.OrchestratorConfig{
		ResolveModel: resolveModel,
		Services:     a.registry.All(),
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
	for name, m := range a.models {
		if closer, ok := m.(interface{ Close() error }); ok {
			if err := closer.Close(); err != nil {
				errs = append(errs, fmt.Errorf("close model %s: %w", name, err))
			}
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
