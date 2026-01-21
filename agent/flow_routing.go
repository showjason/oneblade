package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/go-kratos/blades"
	"github.com/google/jsonschema-go/jsonschema"

	"github.com/oneblade/internal/middleware"
)

// Note:
// - This is a rewrite implementation of blades/flow/routing.go
// - blades/flow/routing.go depends on blades/internal/handoff (Go internal mechanism prevents direct import)
// - This replicates the handoff tool logic with equivalent behavior and additionally injects middleware.LoadSessionHistory
const (
	actionHandoffToAgent = "handoff_to_agent"
)

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
	Instruction string
}

type routingAgent struct {
	blades.Agent
	targets map[string]blades.Agent
}

func NewRoutingAgent(config RoutingConfig) (blades.Agent, error) {
	rootAgent, err := blades.NewAgent(
		config.Name,
		blades.WithModel(config.Model),
		blades.WithDescription(config.Description),
		blades.WithInstruction(config.Instruction),
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
			targetAgent string
			lastMessage *blades.Message
		)
		for message, err := range a.Agent.Run(ctx, invocation) {
			if err != nil {
				yield(nil, err)
				return
			}
			// Check for handoff action
			if target, ok := message.Actions[actionHandoffToAgent]; ok {
				targetAgent, _ = target.(string)
				lastMessage = message
				break
			}
			// Yield intermediate messages to preserve streaming
			if !yield(message, nil) {
				return
			}
			lastMessage = message
		}
		agent, ok := a.targets[targetAgent]
		if !ok {
			// No handoff occurred, the last message was already yielded in the loop
			if targetAgent == "" {
				return
			}
			yield(nil, fmt.Errorf("target agent not found: %s", targetAgent))
			return
		}
		// If handoff message has text content, yield it before switching
		if lastMessage != nil && lastMessage.Text() != "" {
			if !yield(lastMessage, nil) {
				return
			}
		}
		for message, err := range agent.Run(ctx, invocation) {
			if !yield(message, err) {
				return
			}
		}
	}
}
