package agent

import (
	"fmt"

	"github.com/go-kratos/blades"
	"github.com/go-kratos/blades/middleware"
	toolkit "github.com/go-kratos/blades/tools"

	"github.com/oneblade/internal/consts"
	"github.com/oneblade/internal/llm"
	"github.com/oneblade/service"
)

// Orchestrator 编排 Agent 配置
type OrchestratorConfig struct {
	ModelRegistry          *llm.ModelRegistry
	Services               []service.Service
	EnabledAgents          []string
	Tools                  []toolkit.Tool
	ConversationMaxMessage int
}

// NewOrchestratorAgent 创建主编排 Agent
func NewOrchestratorAgent(cfg OrchestratorConfig) (blades.Agent, error) {
	if cfg.ModelRegistry == nil {
		return nil, fmt.Errorf("ModelRegistry is required")
	}

	// 1. 获取 orchestrator model（已在 validateRules 中验证必须存在且启用）
	orchestratorModel, _ := cfg.ModelRegistry.Get(consts.AgentNameOrchestrator)

	agentMap := make(map[string]blades.Agent)
	for _, agentName := range cfg.EnabledAgents {
		// 跳过 orchestrator 自身
		if agentName == consts.AgentNameOrchestrator {
			continue
		}

		model, err := cfg.ModelRegistry.Get(agentName)
		if err != nil {
			return nil, err
		}

		var agent blades.Agent
		switch agentName {
		case consts.AgentNameService:
			agent, err = NewServiceAgent(ServiceAgent{Model: model, Services: cfg.Services})
		case consts.AgentNamePrediction:
			agent, err = NewPredictionAgent(PredictionAgentConfig{Model: model})
		case consts.AgentNameReport:
			agent, err = NewReportAgent(ReportAgentConfig{Model: model})
		default:
			continue // 忽略未知的 agent 类型
		}

		if err != nil {
			return nil, err
		}
		agentMap[agentName] = agent
	}

	allTools := make([]toolkit.Tool, 0, len(cfg.Tools)+len(agentMap))
	if len(cfg.Tools) > 0 {
		allTools = append(allTools, cfg.Tools...)
	}

	for _, a := range agentMap {
		// 使用框架提供的 NewAgentTool 将 agent 包装为工具
		tool := blades.NewAgentTool(a)
		allTools = append(allTools, tool)
	}

	maxMessages := cfg.ConversationMaxMessage
	if maxMessages <= 0 {
		maxMessages = 50
	}

	return blades.NewAgent(
		consts.AgentNameOrchestrator,
		blades.WithDescription(consts.OrchestratorDescription),
		blades.WithInstruction(consts.OrchestratorInstruction),
		blades.WithModel(orchestratorModel),
		blades.WithTools(allTools...),
		blades.WithMiddleware(middleware.ConversationBuffered(maxMessages)),
	)
}

// NewInspectionRunner 创建巡检启动器
func NewInspectionRunner(orchestrator blades.Agent) *blades.Runner {
	return blades.NewRunner(orchestrator, blades.WithResumable(true))
}
