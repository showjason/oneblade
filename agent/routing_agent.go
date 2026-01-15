package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"strings"

	"github.com/go-kratos/blades"
	"github.com/google/jsonschema-go/jsonschema"

	"github.com/oneblade/internal/middleware"
)

// 说明：
// - blades/flow/routing.go 依赖 blades/internal/handoff（Go internal 机制导致本仓库无法直接 import）。
// - 这里以“同等行为”的方式复刻 handoff tool + instruction 构建逻辑，并额外注入 middleware.LoadSessionHistory。

const (
	actionHandoffToAgent = "handoff_to_agent"
)

const handoffInstructionTemplate = `You have access to the following agents:
{{range .Targets}}
Agent Name: {{.Name}}
Agent Description: {{.Description}}
{{end}}
Your task:
- Determine whether you are the most appropriate agent to answer the user's question based on your own description.
- If another agent is clearly better suited to handle the user's request, you must transfer the query by calling the "handoff_to_agent" function.
- If no other agent is more suitable, respond to the user directly as a helpful assistant, providing clear, detailed, and accurate information.

Important rules:
- When transferring a query, output only the function call, and nothing else.
- Do not include explanations, reasoning, or any additional text outside of the function call.`

var handoffToAgentPromptTmpl = template.Must(template.New("handoff_to_agent_prompt").Parse(handoffInstructionTemplate))

func buildHandoffInstruction(targets []blades.Agent) (string, error) {
	var buf bytes.Buffer
	if err := handoffToAgentPromptTmpl.Execute(&buf, map[string]any{
		"Targets": targets,
	}); err != nil {
		return "", err
	}
	return buf.String(), nil
}

type handoffTool struct{}

func (h *handoffTool) Name() string { return "handoff_to_agent" }
func (h *handoffTool) Description() string {
	return `Transfer the question to another agent.
Use this tool to hand off control to a more suitable agent based on the agents' descriptions.`
}
func (h *handoffTool) InputSchema() *jsonschema.Schema {
	return &jsonschema.Schema{
		Type:     "object",
		Required: []string{"agentName"},
		Properties: map[string]*jsonschema.Schema{
			"agentName": {
				Type:        "string",
				Description: "The name of the target agent to hand off the request to.",
			},
		},
	}
}
func (h *handoffTool) OutputSchema() *jsonschema.Schema { return nil }
func (h *handoffTool) Handle(ctx context.Context, input string) (string, error) {
	args := map[string]string{}
	if err := json.Unmarshal([]byte(input), &args); err != nil {
		return "", err
	}
	agentName := strings.TrimSpace(args["agentName"])
	if agentName == "" {
		return "", fmt.Errorf("agentName must be a non-empty string")
	}
	toolCtx, ok := blades.FromToolContext(ctx)
	if !ok {
		return "", fmt.Errorf("tool context not found in context")
	}
	toolCtx.SetAction(actionHandoffToAgent, agentName)
	return "", nil
}

type RoutingConfig struct {
	Name        string
	Description string
	Model       blades.ModelProvider
	SubAgents   []blades.Agent
}

type routingAgent struct {
	blades.Agent
	targets map[string]blades.Agent
}

func NewRoutingAgent(config RoutingConfig) (blades.Agent, error) {
	instruction, err := buildHandoffInstruction(config.SubAgents)
	if err != nil {
		return nil, err
	}

	rootAgent, err := blades.NewAgent(
		config.Name,
		blades.WithModel(config.Model),
		blades.WithDescription(config.Description),
		blades.WithInstruction(instruction),
		blades.WithTools(&handoffTool{}),
		blades.WithMiddleware(
			middleware.NewAgentLogging,
			middleware.LoadSessionHistory(),
		),
	)
	if err != nil {
		return nil, err
	}

	targets := make(map[string]blades.Agent)
	for _, agent := range config.SubAgents {
		targets[strings.TrimSpace(agent.Name())] = agent
	}
	return &routingAgent{
		Agent:   rootAgent,
		targets: targets,
	}, nil
}

func (a *routingAgent) Run(ctx context.Context, invocation *blades.Invocation) blades.Generator[*blades.Message, error] {
	return func(yield func(*blades.Message, error) bool) {
		var (
			err         error
			targetAgent string
			message     *blades.Message
		)
		for message, err = range a.Agent.Run(ctx, invocation) {
			if err != nil {
				yield(nil, err)
				return
			}
			if target, ok := message.Actions[actionHandoffToAgent]; ok {
				targetAgent, _ = target.(string)
				break
			}
		}
		agent, ok := a.targets[targetAgent]
		if !ok {
			if message != nil && message.Text() != "" {
				yield(message, nil)
				return
			}
			yield(nil, fmt.Errorf("target agent not found: %s", targetAgent))
			return
		}
		for message, err := range agent.Run(ctx, invocation) {
			if !yield(message, err) {
				return
			}
		}
	}
}
