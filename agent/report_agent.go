package agent

import (
	"github.com/go-kratos/blades"

	"github.com/oneblade/internal/consts"
)

type ReportAgentConfig struct {
	Model blades.ModelProvider
}

func NewReportAgent(cfg ReportAgentConfig) (blades.Agent, error) {
	return blades.NewAgent(
		consts.AgentNameReport,
		blades.WithDescription(consts.ReportAgentDescription),
		blades.WithInstruction(consts.ReportAgentInstruction),
		blades.WithModel(cfg.Model),
	)
}
