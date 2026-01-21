package repl

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFileTranscriptWriter_WriteMessages(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()
	sessionID := "test-session-123"

	tw, err := NewFileTranscriptWriterWithDir(sessionID, tmpDir)
	require.NoError(t, err)
	require.NotNil(t, tw)

	// Verify file path
	expectedPath := filepath.Join(tmpDir, sessionID+".md")
	require.Equal(t, expectedPath, tw.Path())

	// Write user message
	err = tw.WriteUserMessage("Hello, how are you?")
	require.NoError(t, err)

	// Write assistant message
	err = tw.WriteAssistantMessage("I'm doing well, thank you!")
	require.NoError(t, err)

	// Flush and close
	require.NoError(t, tw.Flush())
	require.NoError(t, tw.Close())

	// Read and verify content
	content, err := os.ReadFile(expectedPath)
	require.NoError(t, err)

	contentStr := string(content)
	require.Contains(t, contentStr, "# OneBlade Session")
	require.Contains(t, contentStr, "### 🧑 User")
	require.Contains(t, contentStr, "Hello, how are you?")
	require.Contains(t, contentStr, "### 🤖 Assistant")
	require.Contains(t, contentStr, "I'm doing well, thank you!")
}

func TestFileTranscriptWriter_EmptySessionID(t *testing.T) {
	_, err := NewFileTranscriptWriter("")
	require.Error(t, err)
	require.Contains(t, err.Error(), "session ID is required")
}

func TestFileTranscriptWriter_DefaultDir(t *testing.T) {
	dir, err := DefaultTranscriptDir()
	require.NoError(t, err)

	homeDir, err := os.UserHomeDir()
	require.NoError(t, err)
	require.Equal(t, filepath.Join(homeDir, ".oneblade", "sessions"), dir)
}

func TestFileTranscriptWriter_MultipleWrites(t *testing.T) {
	tmpDir := t.TempDir()
	sessionID := "test-multi-write"

	tw, err := NewFileTranscriptWriterWithDir(sessionID, tmpDir)
	require.NoError(t, err)

	// Write multiple conversations
	for i := 0; i < 3; i++ {
		require.NoError(t, tw.WriteUserMessage("Question "+string(rune('A'+i))))
		require.NoError(t, tw.WriteAssistantMessage("Answer "+string(rune('A'+i))))
	}

	require.NoError(t, tw.Close())

	// Verify content
	content, err := os.ReadFile(tw.Path())
	require.NoError(t, err)

	contentStr := string(content)
	require.Contains(t, contentStr, "Question A")
	require.Contains(t, contentStr, "Answer A")
	require.Contains(t, contentStr, "Question B")
	require.Contains(t, contentStr, "Answer B")
	require.Contains(t, contentStr, "Question C")
	require.Contains(t, contentStr, "Answer C")
}
