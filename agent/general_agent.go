package agent

import (
	"fmt"

	"github.com/go-kratos/blades"
	"github.com/go-kratos/blades/tools"

	"github.com/oneblade/internal/consts"
)

// GeneralAgent Config
type GeneralAgentConfig struct {
	Model blades.ModelProvider
	Tools []tools.Tool
}

// NewGeneralAgent 创建通用工具 Agent
func NewGeneralAgent(cfg GeneralAgentConfig) (blades.Agent, error) {
	if len(cfg.Tools) == 0 {
		return nil, fmt.Errorf("GeneralAgent requires at least one tool")
	}

	return blades.NewAgent(
		consts.AgentNameGeneral,
		blades.WithDescription("General utility agent for system operations and other miscellaneous tasks."),
		blades.WithInstruction("You are a general utility agent capable of performing various system operations. Use the provided tools to fulfill the user's request."),
		blades.WithModel(cfg.Model),
		blades.WithTools(cfg.Tools...),
	)
}
