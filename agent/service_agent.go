package agent

import (
	"fmt"
	"strings"

	"github.com/go-kratos/blades"
	"github.com/go-kratos/blades/tools"

	"github.com/oneblade/internal/consts"
	"github.com/oneblade/internal/middleware"
	"github.com/oneblade/service"
)

type ServiceAgent struct {
	Model    blades.ModelProvider
	Services []service.Service
}

func NewServiceAgent(cfg ServiceAgent) (blades.Agent, error) {
	var serviceTools []tools.Tool
	var serviceDescriptions []string

	for _, s := range cfg.Services {
		tool, err := s.AsTool()
		if err != nil {
			return nil, fmt.Errorf("create tool for %s: %v", s.Name(), err)
		}
		serviceTools = append(serviceTools, tool)

		desc := fmt.Sprintf("%d. **%s** (%s) - %s", len(serviceTools), s.Name(), s.Type(), s.Description())
		serviceDescriptions = append(serviceDescriptions, desc)
	}

	instruction := fmt.Sprintf(consts.ServiceAgentInstruction, strings.Join(serviceDescriptions, "\n"))

	return blades.NewAgent(
		consts.AgentNameService,
		blades.WithDescription(consts.ServiceAgentDescription),
		blades.WithInstruction(instruction),
		blades.WithModel(cfg.Model),
		blades.WithTools(serviceTools...),
		blades.WithMiddleware(
			middleware.NewAgentLogging,
			middleware.LoadSessionHistory(),
		),
	)
}
