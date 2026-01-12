package app

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"
	"time"

	"github.com/go-kratos/blades"
	"github.com/go-kratos/blades/memory"
	toolkit "github.com/go-kratos/blades/tools"

	"github.com/oneblade/agent"
	"github.com/oneblade/config"
	"github.com/oneblade/internal/consts"
	"github.com/oneblade/internal/llm"
	"github.com/oneblade/internal/persistence"
	"github.com/oneblade/service"

	// registry imports services to register init functions
	_ "github.com/oneblade/service/opensearch"
	_ "github.com/oneblade/service/pagerduty"
	_ "github.com/oneblade/service/prometheus"
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
	memoryStore  memory.MemoryStore
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

	// 2.1 初始化日志
	a.initLogger(cfg)

	// Cache enabled agents
	a.agents = make(map[string]*config.AgentConfig)
	for name, acfg := range cfg.Agents {
		if acfg.Enabled {
			a.agents[name] = &acfg
		}
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
	a.registry = registry
	return nil
}

func (a *Application) initLogger(cfg *config.Config) {
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
	enabledAgents := make([]string, 0, len(a.agents))
	for name := range a.agents {
		enabledAgents = append(enabledAgents, name)
	}

	if a.memoryStore == nil {
		a.memoryStore = memory.NewInMemoryStore()
	}

	memoryTool, err := memory.NewMemoryTool(a.memoryStore)
	if err != nil {
		return fmt.Errorf("create memory tool: %w", err)
	}
	saveTool, err := persistence.NewSaveContextTool()
	if err != nil {
		return fmt.Errorf("create SaveContext tool: %w", err)
	}
	loadTool, err := persistence.NewLoadContextTool()
	if err != nil {
		return fmt.Errorf("create LoadContext tool: %w", err)
	}
	orchestratorTools := []toolkit.Tool{memoryTool, saveTool, loadTool}

	// 创建 Orchestrator Agent
	orchestrator, err := agent.NewOrchestratorAgent(agent.OrchestratorConfig{
		ModelRegistry:          a.modelReg,
		Services:               a.registry.All(),
		EnabledAgents:          enabledAgents,
		Tools:                  orchestratorTools,
		ConversationMaxMessage: 50,
	})
	if err != nil {
		return fmt.Errorf("create orchestrator failed: %w", err)
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
func (a *Application) Run(ctx context.Context, input *blades.Message, opts ...blades.RunOption) (*blades.Message, error) {
	if a.runner == nil {
		return nil, fmt.Errorf("application not initialized")
	}
	return a.runner.Run(ctx, input, opts...)
}

func (a *Application) MemoryStore() memory.MemoryStore {
	return a.memoryStore
}

// ShutdownWithTimeout 带超时的优雅关闭
func (a *Application) ShutdownWithTimeout(timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return a.Shutdown(ctx)
}
