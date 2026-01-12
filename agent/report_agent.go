package agent

import (
	"github.com/go-kratos/blades"

	"github.com/oneblade/internal/consts"
)

// ReportAgentConfig 报告生成 Agent 配置
type ReportAgentConfig struct {
	Model blades.ModelProvider
}

// NewReportAgent 创建报告生成 Agent
func NewReportAgent(cfg ReportAgentConfig) (blades.Agent, error) {
	return blades.NewAgent(
		consts.AgentNameReport,
		blades.WithDescription(consts.ReportAgentDescription),
		blades.WithInstruction(consts.ReportAgentInstruction),
		blades.WithModel(cfg.Model),
	)
}
