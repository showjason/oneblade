package middleware

import (
	"context"

	"github.com/go-kratos/blades"
)

// LoadSessionHistory ensures the agent has access to the full session history.
// This is critical for sub-agents in a routing flow, as they might otherwise
// receive an isolated context without previous conversation turns.
func LoadSessionHistory() blades.Middleware {
	return func(next blades.Handler) blades.Handler {
		return blades.HandleFunc(func(ctx context.Context, inv *blades.Invocation) blades.Generator[*blades.Message, error] {
			session, ok := blades.FromSessionContext(ctx)
			if !ok {
				// No session found, proceed as is
				return next.Handle(ctx, inv)
			}

			// If inv.History is relatively empty (e.g. only contains the new message),
			// and we have a richer history in the session, use the session history.
			// Currently, we'll simply prioritize the session history to ensure continuity.
			// Note: session.History() typically includes the current User message if it was added by the Runner.

			sessionHistory := session.History()
			if len(sessionHistory) > 0 {
				// Use session history as the source of truth for context
				// Copy to avoid modifying the specific session slice backing array if the agent appends prompts
				inv.History = make([]*blades.Message, len(sessionHistory))
				copy(inv.History, sessionHistory)
			}

			return next.Handle(ctx, inv)
		})
	}
}
