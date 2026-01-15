package gemini

import (
	"testing"

	"github.com/go-kratos/blades"
	"github.com/stretchr/testify/require"
	"google.golang.org/genai"
)

func TestConvertGenAIToBlades_TokenUsage(t *testing.T) {
	resp := &genai.GenerateContentResponse{
		Candidates: []*genai.Candidate{
			{Content: &genai.Content{Parts: []*genai.Part{{Text: "ok"}}}},
		},
		UsageMetadata: &genai.GenerateContentResponseUsageMetadata{
			PromptTokenCount:        11,
			CandidatesTokenCount:    22,
			TotalTokenCount:         33,
			CachedContentTokenCount: 4,
		},
	}

	out, err := convertGenAIToBlades(resp, blades.StatusCompleted)
	require.NoError(t, err)
	require.NotNil(t, out)
	require.EqualValues(t, 11, out.Message.TokenUsage.InputTokens)
	require.EqualValues(t, 22, out.Message.TokenUsage.OutputTokens)
	require.EqualValues(t, 33, out.Message.TokenUsage.TotalTokens)
	require.EqualValues(t, int64(4), out.Message.Metadata["cached_tokens"])
}

