package middleware

import (
	"context"
	"errors"
	"testing"

	"github.com/go-kratos/blades"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testAgent struct {
	name string
}

func (a testAgent) Name() string        { return a.name }
func (a testAgent) Description() string { return "test" }
func (a testAgent) Run(context.Context, *blades.Invocation) blades.Generator[*blades.Message, error] {
	return func(func(*blades.Message, error) bool) {}
}

func TestNewAgentLogging_Passthrough(t *testing.T) {
	next := blades.HandleFunc(func(ctx context.Context, inv *blades.Invocation) blades.Generator[*blades.Message, error] {
		return func(yield func(*blades.Message, error) bool) {
			yield(blades.AssistantMessage("response"), nil)
		}
	})

	h := NewAgentLogging(next)
	ctx := blades.NewAgentContext(context.Background(), testAgent{name: "test-agent"})
	inv := &blades.Invocation{
		ID:      "inv-1",
		Model:   "m",
		Message: blades.UserMessage("hello"),
	}

	var got []*blades.Message
	for msg, err := range h.Handle(ctx, inv) {
		require.NoError(t, err)
		require.NotNil(t, msg)
		got = append(got, msg)
	}
	require.Len(t, got, 1)
	assert.Equal(t, "response", got[0].Text())
}

func TestNewAgentLogging_ToolMessagePassthrough(t *testing.T) {
	toolMsg := &blades.Message{
		ID:   "m1",
		Role: blades.RoleTool,
		Parts: []blades.Part{
			blades.ToolPart{ID: "t1", Name: "ToolX", Request: `{"a":1}`, Response: `{"ok":true}`},
		},
	}

	next := blades.HandleFunc(func(ctx context.Context, inv *blades.Invocation) blades.Generator[*blades.Message, error] {
		return func(yield func(*blades.Message, error) bool) {
			if !yield(toolMsg, nil) {
				return
			}
			yield(blades.AssistantMessage("done"), nil)
		}
	})

	h := NewAgentLogging(next)
	ctx := blades.NewAgentContext(context.Background(), testAgent{name: "test-agent"})
	inv := &blades.Invocation{
		ID:      "inv-2",
		Model:   "m",
		Message: blades.UserMessage("hello"),
	}

	var texts []string
	for msg, err := range h.Handle(ctx, inv) {
		require.NoError(t, err)
		require.NotNil(t, msg)
		texts = append(texts, msg.Text())
	}
	require.Equal(t, []string{"", "done"}, texts)
}

func TestNewAgentLogging_ErrorPassthrough(t *testing.T) {
	testErr := errors.New("boom")
	next := blades.HandleFunc(func(ctx context.Context, inv *blades.Invocation) blades.Generator[*blades.Message, error] {
		return func(yield func(*blades.Message, error) bool) {
			yield(nil, testErr)
		}
	})

	h := NewAgentLogging(next)
	ctx := blades.NewAgentContext(context.Background(), testAgent{name: "test-agent"})
	inv := &blades.Invocation{
		ID:      "inv-3",
		Model:   "m",
		Message: blades.UserMessage("hello"),
	}

	var gotErr error
	for _, err := range h.Handle(ctx, inv) {
		gotErr = err
	}
	require.ErrorIs(t, gotErr, testErr)
}
