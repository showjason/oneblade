package agent

import (
	"github.com/go-kratos/blades"
)

// ReportAgentConfig 报告生成 Agent 配置
type ReportAgentConfig struct {
	Model blades.ModelProvider
}

// NewReportAgent 创建报告生成 Agent
func NewReportAgent(cfg ReportAgentConfig) (blades.Agent, error) {
	return blades.NewAgent(
		"report_agent",
		blades.WithDescription("负责汇总分析数据并生成巡检报告的 Agent"),
		blades.WithInstruction(`你是一个巡检报告撰写专家。

你的职责:
1. 汇总来自 DataCollection Agent 的分析结果
2. 生成结构化的巡检报告
3. 突出关键问题和风险点
4. 提供可操作的改进建议

报告结构:
1. 执行摘要
2. 系统健康评分
3. 关键指标分析
4. 告警汇总
5. 日志异常
6. 风险评估
7. 改进建议

确保报告简洁、专业、可操作。`),
		blades.WithModel(cfg.Model),
	)
}
