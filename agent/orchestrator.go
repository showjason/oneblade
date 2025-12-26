package agent

import (
	"github.com/go-kratos/blades"
	"github.com/go-kratos/blades/flow"

	"github.com/oneblade/collector"
)

// OrchestratorConfig 编排 Agent 配置
type OrchestratorConfig struct {
	Model    blades.ModelProvider
	Registry *collector.Registry
}

// NewOrchestratorAgent 创建主编排 Agent
func NewOrchestratorAgent(cfg OrchestratorConfig) (blades.Agent, error) {
	// 从 Registry 获取所有 Collectors
	collectors := cfg.Registry.All()

	// 创建统一数据采集 Agent
	dataCollectionAgent, err := NewDataCollectionAgent(DataCollectionAgentConfig{
		Model:      cfg.Model,
		Collectors: collectors,
	})
	if err != nil {
		return nil, err
	}

	reportAgent, err := NewReportAgent(cfg.Model)
	if err != nil {
		return nil, err
	}

	predictionAgent, err := NewPredictionAgent(cfg.Model)
	if err != nil {
		return nil, err
	}

	// 创建顺序分析流程: 数据采集 -> 预测分析 -> 报告生成
	analysisAgent := flow.NewSequentialAgent(flow.SequentialConfig{
		Name:        "analysis_agent",
		Description: "顺序执行数据采集、预测分析和报告生成",
		SubAgents: []blades.Agent{
			dataCollectionAgent,
			predictionAgent,
			reportAgent,
		},
	})

	// 创建主编排 Agent（支持智能路由）
	return flow.NewRoutingAgent(flow.RoutingConfig{
		Name:        "sre_orchestrator",
		Description: "SRE 智能巡检系统主控 Agent",
		Model:       cfg.Model,
		SubAgents: []blades.Agent{
			analysisAgent,
			dataCollectionAgent,
			reportAgent,
			predictionAgent,
		},
	})
}

// NewInspectionRunner 创建巡检启动器
func NewInspectionRunner(orchestrator blades.Agent) *blades.Runner {
	return blades.NewRunner(orchestrator, blades.WithResumable(true))
}
