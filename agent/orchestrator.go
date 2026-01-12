package agent

import (
	"fmt"

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
	subAgents := []blades.Agent{analysisAgent}
	for name, agent := range agentMap {
		if name != consts.AgentNameService && name != consts.AgentNamePrediction && name != consts.AgentNameReport {
			subAgents = append(subAgents, agent)
		}
	}

	return flow.NewRoutingAgent(flow.RoutingConfig{
		Name:        consts.AgentNameOrchestrator,
		Description: consts.OrchestratorDescription,
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
