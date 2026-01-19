package repl

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// TranscriptWriter defines the interface for saving conversation transcripts.
type TranscriptWriter interface {
	// WriteUserMessage writes a user message to the transcript.
	WriteUserMessage(text string) error
	// WriteAssistantMessage writes an assistant response to the transcript.
	WriteAssistantMessage(text string) error
	// Flush ensures all buffered data is written to disk.
	Flush() error
	// Path returns the file path of the transcript.
	Path() string
	// Close closes the transcript writer and releases resources.
	Close() error
}

// FileTranscriptWriter is a file-based implementation of TranscriptWriter.
// It writes conversation transcripts to a markdown file.
type FileTranscriptWriter struct {
	path       string
	file       *os.File
	mu         sync.Mutex
	headerDone bool
}

// DefaultTranscriptDir returns the default transcript directory (~/.oneblade/).
func DefaultTranscriptDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home dir: %w", err)
	}
	return filepath.Join(homeDir, ".oneblade", "sessions"), nil
}

// NewFileTranscriptWriter creates a new FileTranscriptWriter that saves
// transcripts to ~/.oneblade/sessions/<sessionID>.md.
func NewFileTranscriptWriter(sessionID string) (*FileTranscriptWriter, error) {
	return NewFileTranscriptWriterWithDir(sessionID, "")
}

// NewFileTranscriptWriterWithDir creates a new FileTranscriptWriter with a custom directory.
// If dir is empty, it uses the default ~/.oneblade/sessions/ directory.
func NewFileTranscriptWriterWithDir(sessionID, dir string) (*FileTranscriptWriter, error) {
	if sessionID == "" {
		return nil, fmt.Errorf("session ID is required")
	}

	if dir == "" {
		var err error
		dir, err = DefaultTranscriptDir()
		if err != nil {
			return nil, err
		}
	}

	// Ensure directory exists
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create transcript dir: %w", err)
	}

	path := filepath.Join(dir, sessionID+".md")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("open transcript file: %w", err)
	}

	return &FileTranscriptWriter{
		path: path,
		file: file,
	}, nil
}

// Path returns the file path of the transcript.
func (w *FileTranscriptWriter) Path() string {
	return w.path
}

// writeHeader writes the markdown header if not already written.
func (w *FileTranscriptWriter) writeHeader() error {
	if w.headerDone {
		return nil
	}

	header := fmt.Sprintf("# OneBlade Session\n\n_Started: %s_\n\n---\n\n",
		time.Now().Format("2006-01-02 15:04:05"))

	if _, err := w.file.WriteString(header); err != nil {
		return err
	}
	w.headerDone = true
	return nil
}

// WriteUserMessage writes a user message to the transcript.
func (w *FileTranscriptWriter) WriteUserMessage(text string) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if err := w.writeHeader(); err != nil {
		return err
	}

	entry := fmt.Sprintf("### 🧑 User\n\n%s\n\n", text)
	if _, err := w.file.WriteString(entry); err != nil {
		return fmt.Errorf("write user message: %w", err)
	}

	return w.file.Sync()
}

// WriteAssistantMessage writes an assistant response to the transcript.
func (w *FileTranscriptWriter) WriteAssistantMessage(text string) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if err := w.writeHeader(); err != nil {
		return err
	}

	entry := fmt.Sprintf("### 🤖 Assistant\n\n%s\n\n---\n\n", text)
	if _, err := w.file.WriteString(entry); err != nil {
		return fmt.Errorf("write assistant message: %w", err)
	}

	return w.file.Sync()
}

// Flush ensures all buffered data is written to disk.
func (w *FileTranscriptWriter) Flush() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.file == nil {
		return nil
	}
	return w.file.Sync()
}

// Close closes the transcript file.
func (w *FileTranscriptWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.file == nil {
		return nil
	}

	if err := w.file.Sync(); err != nil {
		return fmt.Errorf("sync before close: %w", err)
	}

	err := w.file.Close()
	w.file = nil
	return err
}
