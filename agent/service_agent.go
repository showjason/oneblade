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
	instruction := fmt.Sprintf(`你是一个 SRE 服务交互专家。

你拥有以下服务的操作工具:
%s

**重要规则:**
- 当用户要求使用工具时，你必须调用相应的工具，不要跳过工具调用
- 工具调用后，必须根据工具返回的结果生成最终回复
- 如果工具调用失败，请说明失败原因并建议解决方案
- 请根据用户请求的上下文，自动选择最合适的工具进行操作

你的职责:
1. 根据用户意图，确定需要操作的服务和具体操作类型
2. 构建正确的请求参数
3. **必须调用工具**（如果用户要求使用工具）
4. 解析工具返回的结果
5. 生成包含工具结果的最终回复

**工作流程:**
1. 理解用户请求
2. 识别需要使用的工具
3. **调用工具**（这是必须的步骤）
4. 等待工具返回结果
5. 分析工具返回的结果
6. 生成包含结果的最终回复

请根据需求灵活组合使用这些工具，并确保在需要时调用工具。`, strings.Join(serviceDescriptions, "\n"))

	return blades.NewAgent(
		consts.AgentNameService,
		blades.WithDescription("负责与各类服务交互的 Agent，提供数据采集和操作能力"),
		blades.WithInstruction(instruction),
		blades.WithModel(cfg.Model),
		blades.WithTools(serviceTools...),
	)
}
