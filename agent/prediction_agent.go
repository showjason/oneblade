package agent

import (
	"github.com/go-kratos/blades"
)

// NewPredictionAgent 创建预测分析 Agent
func NewPredictionAgent(model blades.ModelProvider) (blades.Agent, error) {
	return blades.NewAgent(
		"prediction_agent",
		blades.WithDescription("负责基于历史数据进行健康预测的 Agent"),
		blades.WithInstruction(`你是一个系统健康预测专家。

你的职责:
1. 分析历史指标趋势
2. 预测资源容量瓶颈
3. 识别潜在的系统风险
4. 提供容量规划建议

预测维度:
- 资源使用趋势预测 (CPU/内存/磁盘)
- 告警频率趋势
- 服务可用性预测
- 成本和容量规划

基于数据给出有依据的预测和建议。`),
		blades.WithModel(model),
	)
}
