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
}

// NewOrchestratorAgent 创建主编排 Agent
func NewOrchestratorAgent(cfg OrchestratorConfig) (blades.Agent, error) {
	if cfg.ModelRegistry == nil {
		return nil, fmt.Errorf("ModelRegistry is required")
	}

	// 1. Resolve Models
	serviceModel, err := cfg.ModelRegistry.Get(consts.AgentNameService)
	if err != nil {
		return nil, err
	}

	predictionModel, err := cfg.ModelRegistry.Get(consts.AgentNamePrediction)
	if err != nil {
		return nil, err
	}

	reportModel, err := cfg.ModelRegistry.Get(consts.AgentNameReport)
	if err != nil {
		return nil, err
	}

	orchestratorModel, err := cfg.ModelRegistry.Get(consts.AgentNameOrchestrator)
	if err != nil {
		return nil, err
	}

	// 2. Create Leaf Agents
	serviceAgent, err := NewServiceAgent(ServiceAgent{
		Model:    serviceModel,
		Services: cfg.Services,
	})
	if err != nil {
		return nil, err
	}

	reportAgent, err := NewReportAgent(ReportAgentConfig{
		Model: reportModel,
	})
	if err != nil {
		return nil, err
	}

	predictionAgent, err := NewPredictionAgent(PredictionAgentConfig{
		Model: predictionModel,
	})
	if err != nil {
		return nil, err
	}

	// 3. Create Flows
	analysisAgent := newAnalysisFlow(serviceAgent, predictionAgent, reportAgent)

	// 4. Create Main Orchestrator (Routing)
	return flow.NewRoutingAgent(flow.RoutingConfig{
		Name:        consts.AgentNameOrchestrator,
		Description: consts.OrchestratorDescription,
		Model:       orchestratorModel,
		SubAgents: []blades.Agent{
			analysisAgent,
			serviceAgent, // Direct access for single-shot tasks
			reportAgent,
			predictionAgent,
		},
	})
}

// newAnalysisFlow creates the sequential analysis workflow
func newAnalysisFlow(serviceAgent, predictionAgent, reportAgent blades.Agent) blades.Agent {
	return flow.NewSequentialAgent(flow.SequentialConfig{
		Name:        consts.AgentNameAnalysis,
		Description: consts.AnalysisAgentDescription,
		SubAgents: []blades.Agent{
			serviceAgent,
			predictionAgent,
			reportAgent,
		},
	})
}

// NewInspectionRunner 创建巡检启动器
func NewInspectionRunner(orchestrator blades.Agent) *blades.Runner {
	return blades.NewRunner(orchestrator, blades.WithResumable(true))
}
