package app

import (
	"context"
	"fmt"
	"time"

	"github.com/go-kratos/blades"

	"github.com/oneblade/agent"
	"github.com/oneblade/config"
	"github.com/oneblade/internal/consts"
	"github.com/oneblade/internal/llm"
	"github.com/oneblade/service"
)

// Application 应用程序核心结构体
// 管理所有依赖和生命周期
type Application struct {
	cfg          *config.Loader
	agents       map[string]*config.AgentConfig
	registry     *service.Registry
	modelReg     *llm.ModelRegistry
	orchestrator blades.Agent
	runner       *blades.Runner
}

func NewApplication(configPath string) (*Application, error) {
	loader := config.NewLoader(configPath)

	return &Application{
		cfg:      loader,
		modelReg: llm.NewModelRegistry(),
	}, nil
}

// Initialize 初始化所有依赖
func (a *Application) Initialize(ctx context.Context) error {
	// 1. 加载配置
	cfg, err := a.cfg.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// 2. 验证 App 级业务规则
	if err := a.validateRules(cfg); err != nil {
		return fmt.Errorf("validate app rules: %w", err)
	}

	// Cache enabled agents
	a.agents = make(map[string]*config.AgentConfig)
	for name, acfg := range cfg.Agents {
		a.agents[name] = &acfg
	}

	// 3. 初始化 Services
	if err := a.initServices(); err != nil {
		return err
	}

	// 4. 初始化 Models
	if err := a.initModels(ctx); err != nil {
		return err
	}

	// 5. 初始化 Orchestrator
	if err := a.initOrchestrator(); err != nil {
		return err
	}

	return nil
}

// validateRules 验证应用运行所需的业务规则
func (a *Application) validateRules(cfg *config.Config) error {
	subAgentNames := []string{consts.AgentNameService, consts.AgentNamePrediction, consts.AgentNameReport}

	// 验证 orchestrator 必须存在且开启
	orchestrator, ok := cfg.Agents[consts.AgentNameOrchestrator]
	if !ok {
		return fmt.Errorf("orchestrator agent %s is required but not found", consts.AgentNameOrchestrator)
	}
	if !orchestrator.Enabled {
		return fmt.Errorf("orchestrator agent %s must be enabled", consts.AgentNameOrchestrator)
	}

	// 验证子 agents 至少一个开启
	enabledSubAgents := 0
	for _, name := range subAgentNames {
		if agent, ok := cfg.Agents[name]; ok && agent.Enabled {
			enabledSubAgents++
		}
	}
	if enabledSubAgents == 0 {
		return fmt.Errorf("at least one sub agent (%s, %s, %s) must be enabled", consts.AgentNameService, consts.AgentNamePrediction, consts.AgentNameReport)
	}

	return nil
}

func (a *Application) initServices() error {
	registry := service.NewRegistry()
	if err := registry.InitFromConfig(a.cfg); err != nil {
		return fmt.Errorf("init registry: %w", err)
	}
	a.registry = registry
	return nil
}

func (a *Application) initModels(ctx context.Context) error {
	factory := llm.NewFactory()

	for name, agentCfg := range a.agents {
		m, err := factory.Build(ctx, agentCfg.LLM)
		if err != nil {
			return fmt.Errorf("build model for %s: %w", name, err)
		}
		a.modelReg.Register(name, m)
	}
	return nil
}

func (a *Application) initOrchestrator() error {
	// 创建 Orchestrator Agent
	orchestrator, err := agent.NewOrchestratorAgent(agent.OrchestratorConfig{
		ModelRegistry: a.modelReg,
		Services:      a.registry.All(),
	})
	if err != nil {
		return fmt.Errorf("create orchestrator: %w", err)
	}
	a.orchestrator = orchestrator

	// 创建 Runner
	a.runner = agent.NewInspectionRunner(a.orchestrator)
	return nil
}

// Shutdown 优雅关闭
// 按照 runner -> registry -> model 的顺序释放资源
func (a *Application) Shutdown(ctx context.Context) error {
	var errs []error

	// 1. 关闭 runner (No-op currently)

	// 2. 关闭 registry
	if a.registry != nil {
		if err := a.registry.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close registry: %w", err))
		}
	}

	// 3. 关闭 models
	if a.modelReg != nil {
		if err := a.modelReg.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close models: %w", err))
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

// ShutdownWithTimeout 带超时的优雅关闭
func (a *Application) ShutdownWithTimeout(timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return a.Shutdown(ctx)
}
