package agent

import (
	"fmt"

	"github.com/go-kratos/blades"
	"github.com/go-kratos/blades/tools"

	"github.com/oneblade/internal/consts"
)

type GeneralAgentConfig struct {
	Model blades.ModelProvider
	Tools []tools.Tool
}

func NewGeneralAgent(cfg GeneralAgentConfig) (blades.Agent, error) {
	if len(cfg.Tools) == 0 {
		return nil, fmt.Errorf("GeneralAgent requires at least one tool")
	}

	return blades.NewAgent(
		consts.AgentNameGeneral,
		blades.WithDescription(consts.GeneralAgentDescription),
		blades.WithInstruction(consts.GeneralAgentInstruction),
		blades.WithModel(cfg.Model),
		blades.WithTools(cfg.Tools...),
	)
}
