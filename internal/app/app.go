package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
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

	if err := a.initMemoryStore(); err != nil {
		return err
	}

	if err := a.initOrchestrator(); err != nil {
		return err
	}

	return nil
}

func (a *Application) validateRules(cfg *config.Config) error {
	orchestrator, ok := cfg.Agents[consts.AgentNameOrchestrator]
	if !ok {
		return fmt.Errorf("orchestrator agent %s is required but not found", consts.AgentNameOrchestrator)
	}
	if !orchestrator.Enabled {
		return fmt.Errorf("orchestrator agent %s must be enabled", consts.AgentNameOrchestrator)
	}

	enabledSubAgents := 0
	for _, name := range consts.RequiredSubAgents {
		if agent, ok := cfg.Agents[name]; ok && agent.Enabled {
			enabledSubAgents++
		}
	}
	if enabledSubAgents == 0 {
		agentNames := strings.Join(consts.RequiredSubAgents, ", ")
		return fmt.Errorf("at least one sub agent (%s) must be enabled", agentNames)
	}

	return nil
}

func (a *Application) initServices() error {
	slog.Info("app.init.services.start")
	registry := service.NewRegistry()
	if err := registry.InitFromConfig(a.cfg); err != nil {
		return fmt.Errorf("init registry: %w", err)
	}
	a.registry = registry

	services := registry.All()
	serviceNames := make([]string, 0, len(services))
	serviceTypes := make([]string, 0, len(services))
	for _, s := range services {
		serviceNames = append(serviceNames, s.Name())
		serviceTypes = append(serviceTypes, string(s.Type()))
	}
	slog.Info("app.init.services.complete",
		"count", len(services),
		"services", serviceNames,
		"types", serviceTypes,
	)
	return nil
}

func (a *Application) initModels(ctx context.Context) error {
	slog.Info("app.init.models.start")
	factory := llm.NewFactory()

	for name, agentCfg := range a.agents {
		m, err := factory.Build(ctx, agentCfg.LLM)
		if err != nil {
			return fmt.Errorf("build model for %s: %w", name, err)
		}
		a.modelReg.Register(name, m)
		slog.Info("app.init.models.register",
			"agent", name,
			"provider", agentCfg.LLM.Provider,
			"model", agentCfg.LLM.Model,
		)
	}
	slog.Info("app.init.models.complete", "count", len(a.agents))
	return nil
}

func (a *Application) initMemoryStore() error {
	a.memoryStore = memory.NewInMemoryStore()
	slog.Info("app.init.memory.store.complete", "type", "in-memory")
	return nil
}

func (a *Application) initTools() ([]toolkit.Tool, error) {
	slog.Info("app.init.tools.start")
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
	tools := []toolkit.Tool{memoryTool, saveTool, loadTool}
	toolNames := make([]string, 0, len(tools))
	for _, t := range tools {
		toolNames = append(toolNames, t.Name())
	}
	slog.Info("app.init.tools.complete",
		"count", len(tools),
		"tools", toolNames,
	)
	return tools, nil
}

func (a *Application) initOrchestrator() error {
	slog.Info("app.init.orchestrator.start")
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
	slog.Info("app.init.orchestrator.complete",
		"enabled_agents", enabledAgents,
	)
	return nil
}

func (a *Application) Shutdown(ctx context.Context) error {
	var err1, err2 error

	if a.registry != nil {
		if err := a.registry.Close(); err != nil {
			err1 = fmt.Errorf("close registry: %w", err)
		}
	}

	if a.modelReg != nil {
		if err := a.modelReg.Close(); err != nil {
			err2 = fmt.Errorf("close models: %w", err)
		}
	}

	return errors.Join(err1, err2)
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
