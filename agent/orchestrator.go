package agent

import (
	"fmt"

	"github.com/go-kratos/blades"
	"github.com/go-kratos/blades/flow"

	"github.com/oneblade/internal/consts"
	"github.com/oneblade/internal/llm"
	"github.com/oneblade/service"
)

// Orchestrator 编排 Agent 配置
type OrchestratorConfig struct {
	ModelRegistry *llm.ModelRegistry
	Services      []service.Service
	EnabledAgents []string
}

// NewOrchestratorAgent 创建主编排 Agent
func NewOrchestratorAgent(cfg OrchestratorConfig) (blades.Agent, error) {
	if cfg.ModelRegistry == nil {
		return nil, fmt.Errorf("ModelRegistry is required")
	}

	// 1. 获取 orchestrator model（已在 validateRules 中验证必须存在且启用）
	orchestratorModel, _ := cfg.ModelRegistry.Get(consts.AgentNameOrchestrator)

	// 2. 动态创建子 agent
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

	// 4. 创建 analysisFlow 和最终的 SubAgents 列表
	analysisAgent := newAnalysisFlow(agentMap)

	subAgents := []blades.Agent{analysisAgent}
	for _, agent := range agentMap {
		subAgents = append(subAgents, agent)
	}

	return flow.NewRoutingAgent(flow.RoutingConfig{
		Name:        consts.AgentNameOrchestrator,
		Description: consts.OrchestratorDescription,
		Model:       orchestratorModel,
		SubAgents:   subAgents,
	})
}

// newAnalysisFlow creates the sequential analysis workflow
func newAnalysisFlow(agents map[string]blades.Agent) blades.Agent {
	// 按固定顺序添加启用的 agent
	order := []string{
		consts.AgentNameService,
		consts.AgentNamePrediction,
		consts.AgentNameReport,
	}

	var subAgents []blades.Agent
	for _, name := range order {
		if agent, ok := agents[name]; ok {
			subAgents = append(subAgents, agent)
		}
	}

	return flow.NewSequentialAgent(flow.SequentialConfig{
		Name:        consts.AgentNameAnalysis,
		Description: consts.AnalysisAgentDescription,
		SubAgents:   subAgents,
	})
}

// NewInspectionRunner 创建巡检启动器
func NewInspectionRunner(orchestrator blades.Agent) *blades.Runner {
	return blades.NewRunner(orchestrator, blades.WithResumable(true))
}
