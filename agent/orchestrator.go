package agent

import (
	"github.com/go-kratos/blades"
	"github.com/go-kratos/blades/flow"

	"github.com/oneblade/service"
)

// Orchestrator 编排 Agent 配置
type OrchestratorConfig struct {
	Model    blades.ModelProvider
	Services []service.Service
}

// NewOrchestratorAgent 创建主编排 Agent
func NewOrchestratorAgent(cfg OrchestratorConfig) (blades.Agent, error) {
	services := cfg.Services

	// 创建统一 Service Agent
	serviceAgent, err := NewServiceAgent(ServiceAgent{
		Model:    cfg.Model,
		Services: services,
	})
	if err != nil {
		return nil, err
	}

	reportAgent, err := NewReportAgent(ReportAgentConfig{
		Model: cfg.Model,
	})
	if err != nil {
		return nil, err
	}

	predictionAgent, err := NewPredictionAgent(PredictionAgentConfig{
		Model: cfg.Model,
	})
	if err != nil {
		return nil, err
	}

	// 创建顺序分析流程: 数据采集/服务操作 -> 预测分析 -> 报告生成
	// 注意: 这里的 serviceAgent 替代了原先的 dataCollectionAgent
	analysisAgent := flow.NewSequentialAgent(flow.SequentialConfig{
		Name:        "analysis_agent",
		Description: "顺序执行数据采集、预测分析和报告生成",
		SubAgents: []blades.Agent{
			serviceAgent,
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
			serviceAgent, // 直接暴露 ServiceAgent 以便进行独立操作（如解决告警）
			reportAgent,
			predictionAgent,
		},
	})
}

// NewInspectionRunner 创建巡检启动器
func NewInspectionRunner(orchestrator blades.Agent) *blades.Runner {
	return blades.NewRunner(orchestrator, blades.WithResumable(true))
}
