package app

import (
	"context"
	"fmt"
	"time"

	"github.com/go-kratos/blades"
	"github.com/go-kratos/blades/memory"
	toolkit "github.com/go-kratos/blades/tools"

	"github.com/oneblade/agent"
	"github.com/oneblade/config"
	"github.com/oneblade/internal/consts"
	"github.com/oneblade/internal/llm"
	"github.com/oneblade/internal/logger"
	"github.com/oneblade/internal/persistence"
	"github.com/oneblade/service"

	_ "github.com/oneblade/service/opensearch"
	_ "github.com/oneblade/service/pagerduty"
	_ "github.com/oneblade/service/prometheus"
)

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

func (a *Application) Initialize(ctx context.Context) error {
	cfg, err := a.cfg.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	if err := a.validateRules(cfg); err != nil {
		return fmt.Errorf("validate app rules: %w", err)
	}

	logger.Initialize(cfg)

	// Cache enabled agents
	a.agents = make(map[string]*config.AgentConfig)
	for name, acfg := range cfg.Agents {
		if acfg.Enabled {
			a.agents[name] = &acfg
		}
	}

	if err := a.initServices(); err != nil {
		return err
	}

	if err := a.initModels(ctx); err != nil {
		return err
	}

	if err := a.initOrchestrator(); err != nil {
		return err
	}

	return nil
}

func (a *Application) validateRules(cfg *config.Config) error {
	subAgentNames := []string{consts.AgentNameService, consts.AgentNamePrediction, consts.AgentNameReport}

	orchestrator, ok := cfg.Agents[consts.AgentNameOrchestrator]
	if !ok {
		return fmt.Errorf("orchestrator agent %s is required but not found", consts.AgentNameOrchestrator)
	}
	if !orchestrator.Enabled {
		return fmt.Errorf("orchestrator agent %s must be enabled", consts.AgentNameOrchestrator)
	}

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

func (a *Application) initTools() ([]toolkit.Tool, error) {
	memoryTool, err := memory.NewMemoryTool(a.memoryStore)
	if err != nil {
		return nil, fmt.Errorf("create memory tool: %w", err)
	}
	saveTool, err := persistence.NewSaveContextTool()
	if err != nil {
		return nil, fmt.Errorf("create SaveContext tool: %w", err)
	}
	loadTool, err := persistence.NewLoadContextTool()
	if err != nil {
		return nil, fmt.Errorf("create LoadContext tool: %w", err)
	}
	return []toolkit.Tool{memoryTool, saveTool, loadTool}, nil
}

func (a *Application) initOrchestrator() error {
	enabledAgents := make([]string, 0, len(a.agents))
	for name := range a.agents {
		enabledAgents = append(enabledAgents, name)
	}

	baseTools, err := a.initTools()
	if err != nil {
		return err
	}

	orchestrator, err := agent.NewOrchestratorAgent(agent.OrchestratorConfig{
		ModelRegistry:          a.modelReg,
		Services:               a.registry.All(),
		EnabledAgents:          enabledAgents,
		Tools:                  baseTools,
		ConversationMaxMessage: 50,
	})
	if err != nil {
		return fmt.Errorf("create orchestrator failed: %w", err)
	}
	a.orchestrator = orchestrator

	a.runner = agent.NewInspectionRunner(a.orchestrator)
	return nil
}

func (a *Application) Shutdown(ctx context.Context) error {
	var errs []error

	if a.registry != nil {
		if err := a.registry.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close registry: %w", err))
		}
	}

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

func (a *Application) Run(ctx context.Context, input *blades.Message, opts ...blades.RunOption) (*blades.Message, error) {
	if a.runner == nil {
		return nil, fmt.Errorf("application not initialized")
	}
	return a.runner.Run(ctx, input, opts...)
}

func (a *Application) MemoryStore() memory.MemoryStore {
	return a.memoryStore
}

func (a *Application) ShutdownWithTimeout(timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return a.Shutdown(ctx)
}
