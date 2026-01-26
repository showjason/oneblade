package middleware

import (
	"context"
	"log/slog"
	"time"

	"github.com/go-kratos/blades"
)

type AgentLogging struct {
	next blades.Handler
}

func NewAgentLogging(next blades.Handler) blades.Handler {
	return &AgentLogging{next: next}
}

func (m *AgentLogging) Handle(ctx context.Context, invocation *blades.Invocation) blades.Generator[*blades.Message, error] {
	return func(yield func(*blades.Message, error) bool) {
		start := time.Now()
		agent, _ := blades.FromAgentContext(ctx)

		slog.Info("agent.invoke.start",
			"agent", agent.Name(),
			"invocation_id", invocation.ID,
			"model", invocation.Model,
			"message", invocation.Message.String(),
			"history_length", len(invocation.History),
			"tools_count", len(invocation.Tools),
		)

		var (
			messageCount int
			totalLength  int
			lastErr      error
		)

		streaming := m.next.Handle(ctx, invocation)
		for msg, err := range streaming {
			if err != nil {
				lastErr = err
				slog.Error("agent.invoke.error",
					"agent", agent.Name(),
					"invocation_id", invocation.ID,
					"duration_ms", time.Since(start).Milliseconds(),
					"error", err,
				)
				if !yield(msg, err) {
					break
				}
				break
			}

			if msg != nil {
				messageCount++
				totalLength += len(msg.Text())

				if msg.Role == blades.RoleTool {
					for _, part := range msg.Parts {
						if tp, ok := part.(blades.ToolPart); ok {
							slog.Info("tool.invoke",
								"agent", agent.Name(),
								"invocation_id", invocation.ID,
								"tool", tp.Name,
								"request", tp.Request,
								"response", tp.Response,
							)
						}
					}
				}
			}

			if !yield(msg, nil) {
				break
			}
		}

		if lastErr == nil {
			slog.Info("agent.invoke.complete",
				"agent", agent.Name(),
				"invocation_id", invocation.ID,
				"duration_ms", time.Since(start).Milliseconds(),
				"message_count", messageCount,
				"total_length", totalLength,
			)
		}
	}
}
