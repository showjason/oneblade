package persistence

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-kratos/blades"
	"github.com/stretchr/testify/require"
)

func TestEncodeDecodeMarkdownV1_RoundTrip(t *testing.T) {
	session := blades.NewSession(map[string]any{"k": "v"})
	require.NoError(t, session.Append(context.Background(), blades.UserMessage("hello")))
	require.NoError(t, session.Append(context.Background(), blades.AssistantMessage("world")))

	dump, err := BuildDumpV1(session)
	require.NoError(t, err)

	md, err := EncodeMarkdownV1(dump, "title")
	require.NoError(t, err)
	require.Contains(t, string(md), markdownHeaderV1)
	require.Contains(t, string(md), beginSessionJSONDump)
	require.Contains(t, string(md), endSessionJSONDump)

	got, err := DecodeMarkdownV1(md)
	require.NoError(t, err)
	require.Equal(t, dump.SessionID, got.SessionID)
	require.Equal(t, dump.State["k"], got.State["k"])
	require.Len(t, got.Messages, 2)
	require.Equal(t, "hello", got.Messages[0].Text)
	require.Equal(t, "world", got.Messages[1].Text)
}

func TestDecodeMarkdownV1_MissingMarkers(t *testing.T) {
	_, err := DecodeMarkdownV1([]byte("# hi"))
	require.Error(t, err)
	require.Contains(t, err.Error(), "missing json dump begin marker")
}

func TestSaveLoadTools_SaveThenLoad(t *testing.T) {
	saveTool, err := NewSaveContextTool()
	require.NoError(t, err)
	loadTool, err := NewLoadContextTool()
	require.NoError(t, err)

	// Build a session with one turn.
	s1 := blades.NewSession(map[string]any{"a": 1})
	ctx1 := blades.NewSessionContext(context.Background(), s1)
	require.NoError(t, s1.Append(ctx1, blades.UserMessage("u1")))
	require.NoError(t, s1.Append(ctx1, blades.AssistantMessage("a1")))

	outPath := filepath.Join(t.TempDir(), "chat.md")
	rawResp, err := saveTool.Handle(ctx1, `{"path":"`+escapeJSON(outPath)+`","title":"chat"}`)
	require.NoError(t, err)

	saveResp, err := DecodeSaveContextResponse(rawResp)
	require.NoError(t, err)
	require.Equal(t, outPath, saveResp.Path)
	require.Equal(t, 2, saveResp.MessageCount)

	_, err = os.Stat(outPath)
	require.NoError(t, err)

	// Load into a new session, verify appended history.
	s2 := blades.NewSession()
	ctx2 := blades.NewSessionContext(context.Background(), s2)
	rawLoadResp, err := loadTool.Handle(ctx2, `{"path":"`+escapeJSON(outPath)+`"}`)
	require.NoError(t, err)
	loadResp, err := DecodeLoadContextResponse(rawLoadResp)
	require.NoError(t, err)
	require.True(t, loadResp.HasState)
	require.Equal(t, 2, loadResp.MessageCount)

	h := s2.History()
	require.Len(t, h, 2)
	require.Equal(t, blades.RoleUser, h[0].Role)
	require.Equal(t, "u1", h[0].Text())
	require.Equal(t, blades.RoleAssistant, h[1].Role)
	require.Equal(t, "a1", h[1].Text())
	require.Equal(t, float64(1), s2.State()["a"]) // json round-trip number => float64
}

func escapeJSON(s string) string {
	return strings.ReplaceAll(s, `\`, `\\`)
}

