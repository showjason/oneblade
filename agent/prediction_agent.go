package agent

import (
	"github.com/go-kratos/blades"

	"github.com/oneblade/internal/consts"
)

// PredictionAgentConfig 预测分析 Agent 配置
type PredictionAgentConfig struct {
	Model blades.ModelProvider
}

// NewPredictionAgent 创建预测分析 Agent
func NewPredictionAgent(cfg PredictionAgentConfig) (blades.Agent, error) {
	return blades.NewAgent(
		consts.AgentNamePrediction,
		blades.WithDescription("负责基于历史数据进行健康预测的 Agent"),
		blades.WithInstruction(consts.PredictionAgentInstruction),
		blades.WithModel(cfg.Model),
	)
}
