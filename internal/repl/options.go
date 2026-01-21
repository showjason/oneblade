package repl

import (
	"github.com/oneblade/internal/app"
)

// Option is a functional option for configuring the REPL.
type Option func(*REPL) error

// WithApplication sets the application instance for the REPL.
func WithApplication(application *app.Application) Option {
	return func(r *REPL) error {
		r.app = application
		return nil
	}
}

// WithTranscriptDir sets a custom directory for transcript files.
// If not set, defaults to ~/.oneblade/.
func WithTranscriptDir(dir string) Option {
	return func(r *REPL) error {
		r.transcriptDir = dir
		return nil
	}
}

// WithPrompt sets a custom prompt prefix.
// Default is "> ".
func WithPrompt(prefix string) Option {
	return func(r *REPL) error {
		r.promptPrefix = prefix
		return nil
	}
}
