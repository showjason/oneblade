package agent

import (
	"github.com/go-kratos/blades"

	"github.com/oneblade/internal/consts"
)

type PredictionAgentConfig struct {
	Model blades.ModelProvider
}

func NewPredictionAgent(cfg PredictionAgentConfig) (blades.Agent, error) {
	return blades.NewAgent(
		consts.AgentNamePrediction,
		blades.WithDescription(consts.PredictionAgentDescription),
		blades.WithInstruction(consts.PredictionAgentInstruction),
		blades.WithModel(cfg.Model),
	)
}
