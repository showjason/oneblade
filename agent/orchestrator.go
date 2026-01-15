package agent

import (
	"fmt"
	"log/slog"

	"github.com/go-kratos/blades"
	"github.com/go-kratos/blades/flow"
	toolkit "github.com/go-kratos/blades/tools"

	"github.com/oneblade/internal/consts"
	"github.com/oneblade/internal/llm"
	"github.com/oneblade/service"
)

type OrchestratorConfig struct {
	ModelRegistry          *llm.ModelRegistry
	Services               []service.Service
	EnabledAgents          []string
	Tools                  []toolkit.Tool
	ConversationMaxMessage int
}

func NewOrchestratorAgent(cfg OrchestratorConfig) (blades.Agent, error) {
	if cfg.ModelRegistry == nil {
		return nil, fmt.Errorf("ModelRegistry is required")
	}

	orchestratorModel, _ := cfg.ModelRegistry.Get(consts.AgentNameOrchestrator)

	agentMap := make(map[string]blades.Agent)
	for _, agentName := range cfg.EnabledAgents {
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
			continue
		}

		if err != nil {
			return nil, err
		}
		agentMap[agentName] = agent
		slog.Info("orchestrator.agent.created", "agent", agentName)
	}

	// generalModel, _ := cfg.ModelRegistry.Get(consts.AgentNameGeneral)

	// generalAgent, err := NewGeneralAgent(GeneralAgentConfig{
	// 	Model: generalModel,
	// 	Tools: cfg.Tools,
	// })
	// if err != nil {
	// 	return nil, fmt.Errorf("failed to create general agent: %v", err)
	// }
	// agentMap[consts.AgentNameGeneral] = generalAgent
	analysisAgent := newAnalysisFlow(agentMap)

	// 构建 subAgents 列表：包含所有独立的 agent 和 analysisAgent
	// service_agent 需要单独列出，因为用户可能直接请求查询外部系统
	subAgents := []blades.Agent{analysisAgent}
	subAgentNames := []string{consts.AgentNameAnalysis}

	// 添加所有独立的 agent（包括 service_agent、prediction_agent、report_agent）
	// 这样 RoutingAgent 可以直接路由到它们，而不需要经过 analysisAgent
	for name, agent := range agentMap {
		subAgents = append(subAgents, agent)
		subAgentNames = append(subAgentNames, name)
	}

	slog.Info("orchestrator.created",
		"sub_agents", subAgentNames,
		"sub_agents_count", len(subAgents),
	)

	// 构建详细的 description，包含路由规则
	description := consts.BuildOrchestratorDescription(subAgentNames)

	return NewRoutingAgent(RoutingConfig{
		Name:        consts.AgentNameOrchestrator,
		Description: description,
		Model:       orchestratorModel,
		SubAgents:   subAgents,
	})
}

func newAnalysisFlow(agents map[string]blades.Agent) blades.Agent {
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

func NewInspectionRunner(orchestrator blades.Agent) *blades.Runner {
	return blades.NewRunner(orchestrator, blades.WithResumable(true))
}
