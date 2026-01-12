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

	// 3. 创建并添加 GeneralAgent (如果存在配置的工具)
	if len(cfg.Tools) > 0 {
		generalModel, err := cfg.ModelRegistry.Get(consts.AgentNameGeneral)
		if err != nil {
			// 如果没有专门配置 general model，降级使用 orchestrator model
			generalModel = orchestratorModel
		}

		generalAgent, err := NewGeneralAgent(GeneralAgentConfig{
			Model: generalModel,
			Tools: cfg.Tools,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create general agent: %v", err)
		}
		agentMap[consts.AgentNameGeneral] = generalAgent
	}

	// 4. 创建 analysisFlow 和最终的 SubAgents 列表
	// newAnalysisFlow 用于组织 Analysis 相关的顺序流 (Service -> Prediction -> Report)
	analysisAgent := newAnalysisFlow(agentMap)

	subAgents := []blades.Agent{analysisAgent}
	// 将其他独立的 agent 也加入到 subAgents 中 (包括 GeneralAgent)
	for name, agent := range agentMap {
		// Analysis flow 已经包含了这些 agent，这里再次添加是为了让 Orchestrator 也可以直接路由到它们 (如果需要)
		// 但为了避免混淆，我们可以只添加不属于 Analysis flow 的 agent
		// 或者，为了简单起见，且遵循 RoutingAgent 的设计，我们可以把主要能力暴露出来

		// 这里我们保留所有生成的 agent 作为候选，Orchestrator 会根据描述选择
		// 注意: analysisAgent 是一个组合 agent
		if name != consts.AgentNameService && name != consts.AgentNamePrediction && name != consts.AgentNameReport {
			subAgents = append(subAgents, agent)
		}
	}
	// 确保 GeneralAgent 被添加 (如果它不在上面的 exclusion list 中，它已经被添加了)

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
