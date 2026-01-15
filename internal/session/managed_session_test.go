package session

import (
	"context"
	"testing"

	"github.com/go-kratos/blades"
	"github.com/stretchr/testify/require"

	"github.com/oneblade/config"
)

type stubSummarizer struct {
	lastPrevious string
	lastCount    int
	out          string
}

func (s *stubSummarizer) Summarize(ctx context.Context, previousSummary string, messages []*blades.Message) (string, blades.TokenUsage, error) {
	s.lastPrevious = previousSummary
	s.lastCount = len(messages)
	return s.out, blades.TokenUsage{InputTokens: 1, OutputTokens: 1, TotalTokens: 2}, nil
}

func TestManagedSession_MessageCountTrigger(t *testing.T) {
	sum := &stubSummarizer{out: "summary-1"}
	sess, err := NewManagedSession(ManagedSessionConfig{
		Conversation: config.ConversationConfig{
			ContextWindowTokens:    100,
			CompressionThreshold:   0.8,
			MaxInContextMessages:   5,
			RetainRecentMessages:   2,
			SummaryMaxOutputTokens: 16,
			SummaryModelAgent:      "orchestrator",
		},
		Summarizer: sum,
	})
	require.NoError(t, err)

	ctx := context.Background()
	// append 6 assistant completed messages to trigger count rule
	for i := 0; i < 6; i++ {
		require.NoError(t, sess.Append(ctx, &blades.Message{Role: blades.RoleAssistant, Status: blades.StatusCompleted}))
	}

	h := sess.History()
	require.GreaterOrEqual(t, len(h), 1)
	require.Equal(t, blades.RoleSystem, h[0].Role)
	require.Contains(t, h[0].Text(), "summary-1")

	// history should be trimmed to tail length 2 (plus injected system summary)
	require.Equal(t, 3, len(h))
}

func TestManagedSession_TokenTrigger(t *testing.T) {
	sum := &stubSummarizer{out: "summary-token"}
	sess, err := NewManagedSession(ManagedSessionConfig{
		Conversation: config.ConversationConfig{
			ContextWindowTokens:  100,
			CompressionThreshold: 0.8, // threshold = 80
			MaxInContextMessages: 50,
			RetainRecentMessages: 2,
		},
		Summarizer: sum,
	})
	require.NoError(t, err)

	ctx := context.Background()
	// 保证有足够的历史供裁剪/摘要：retain=2，因此至少需要 3 条历史才能产生 delta。
	require.NoError(t, sess.Append(ctx, &blades.Message{Role: blades.RoleUser}))
	require.NoError(t, sess.Append(ctx, &blades.Message{Role: blades.RoleAssistant, Status: blades.StatusCompleted}))
	require.NoError(t, sess.Append(ctx, &blades.Message{Role: blades.RoleAssistant, Status: blades.StatusCompleted, TokenUsage: blades.TokenUsage{InputTokens: 99}}))

	h := sess.History()
	require.Equal(t, blades.RoleSystem, h[0].Role)
	require.Contains(t, h[0].Text(), "summary-token")
}

func TestManagedSession_IncrementalUsesPreviousSummary(t *testing.T) {
	sum := &stubSummarizer{out: "s1"}
	sess, err := NewManagedSession(ManagedSessionConfig{
		Conversation: config.ConversationConfig{
			ContextWindowTokens:  100,
			CompressionThreshold: 0.8,
			MaxInContextMessages: 3,
			RetainRecentMessages: 1,
		},
		Summarizer: sum,
	})
	require.NoError(t, err)

	ctx := context.Background()
	for i := 0; i < 4; i++ {
		require.NoError(t, sess.Append(ctx, &blades.Message{Role: blades.RoleAssistant, Status: blades.StatusCompleted}))
	}
	require.Equal(t, "", sum.lastPrevious)

	// second summarize should receive previous summary
	sum.out = "s2"
	for i := 0; i < 4; i++ {
		require.NoError(t, sess.Append(ctx, &blades.Message{Role: blades.RoleAssistant, Status: blades.StatusCompleted}))
	}
	require.Equal(t, "s1", sum.lastPrevious)
}

