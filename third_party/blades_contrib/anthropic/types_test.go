package anthropic

import (
	"testing"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/go-kratos/blades"
	"github.com/stretchr/testify/require"
)

func TestConvertClaudeToBlades_TokenUsage(t *testing.T) {
	msg := &anthropic.Message{
		Usage: anthropic.Usage{
			InputTokens:              10,
			CacheCreationInputTokens: 2,
			CacheReadInputTokens:     3,
			OutputTokens:             7,
		},
	}

	out, err := convertClaudeToBlades(msg, blades.StatusCompleted)
	require.NoError(t, err)
	require.NotNil(t, out)
	require.EqualValues(t, 15, out.Message.TokenUsage.InputTokens)
	require.EqualValues(t, 7, out.Message.TokenUsage.OutputTokens)
	require.EqualValues(t, 22, out.Message.TokenUsage.TotalTokens)
}

