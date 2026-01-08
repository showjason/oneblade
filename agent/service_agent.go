package agent

import (
	"fmt"
	"strings"

	"github.com/go-kratos/blades"
	"github.com/go-kratos/blades/tools"

	"github.com/oneblade/internal/consts"
	"github.com/oneblade/service"
)

// ServiceAgent Service Agent 配置
type ServiceAgent struct {
	Model    blades.ModelProvider
	Services []service.Service
}

// NewServiceAgent 创建统一服务 Tool Agent
func NewServiceAgent(cfg ServiceAgent) (blades.Agent, error) {
	// 创建各个 Service Tool
	var serviceTools []tools.Tool
	var serviceDescriptions []string

	for _, s := range cfg.Services {
		// Get Tool
		tool, err := s.AsTool()
		if err != nil {
			return nil, fmt.Errorf("create tool for %s: %v", s.Name(), err)
		}
		serviceTools = append(serviceTools, tool)

		// Get Description for Prompt
		desc := fmt.Sprintf("%d. **%s** (%s) - %s", len(serviceTools), s.Name(), s.Type(), s.Description())
		serviceDescriptions = append(serviceDescriptions, desc)
	}

	// Dynamic Instruction Construction
	instruction := fmt.Sprintf(consts.ServiceAgentInstruction, strings.Join(serviceDescriptions, "\n"))

	return blades.NewAgent(
		consts.AgentNameService,
		blades.WithDescription("负责与各类服务交互的 Agent，提供数据采集和操作能力"),
		blades.WithInstruction(instruction),
		blades.WithModel(cfg.Model),
		blades.WithTools(serviceTools...),
	)
}
