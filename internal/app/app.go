package app

import (
	"context"
	"fmt"
	"sync"
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
	agents       map[string]*config.AgentConfig
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
	// Step 1: 加载配置（已包含验证和筛选 enabled agents）
	cfg, err := a.cfg.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// cfg.Agents 已经只包含 enabled 的 agents
	a.agents = make(map[string]*config.AgentConfig)
	for name, acfg := range cfg.Agents {
		a.agents[name] = &acfg
	}
	if len(a.agents) == 0 {
		return fmt.Errorf("no agents configured")
	}
	// Step 2: 初始化 Service Registry
	registry := service.NewRegistry()
	if err := registry.InitFromConfig(a.cfg); err != nil {
		return fmt.Errorf("init registry: %w", err)
	}
	a.registry = registry

	// Step 3: 初始化 LLM Factory（按 agentName 严格构建）
	// 注意：a.agents 只包含 enabled 的 agents，无需再次检查
	factory := llm.NewFactory()
	models := make(map[string]blades.ModelProvider)
	var modelsMu sync.RWMutex
	resolveModel := func(agentName string) (blades.ModelProvider, error) {
		// 先尝试读锁读取
		modelsMu.RLock()
		if m, ok := models[agentName]; ok {
			modelsMu.RUnlock()
			return m, nil
		}
		modelsMu.RUnlock()

		// 需要构建，使用写锁
		modelsMu.Lock()
		defer modelsMu.Unlock()

		// double-check: 可能在等待锁期间其他 goroutine 已经构建完成
		if m, ok := models[agentName]; ok {
			return m, nil
		}

		agentCfg, ok := a.agents[agentName]
		if !ok {
			return nil, fmt.Errorf("agent %s not found or disabled", agentName)
		}
		m, err := factory.Build(ctx, agentCfg.LLM)
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
