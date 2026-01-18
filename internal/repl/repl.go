package repl

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/c-bata/go-prompt"
	"github.com/go-kratos/blades"
	"github.com/go-kratos/blades/memory"

	"github.com/oneblade/internal/app"
)

const (
	cmdExit = "exit"
	cmdQuit = "quit"
	cmdHelp = "help"
)

// REPL provides an interactive command-line interface for the OneBlade agent.
type REPL struct {
	app               *app.Application
	session           blades.Session
	memStore          memory.MemoryStore
	transcript        TranscriptWriter
	transcriptDir     string
	transcriptEnabled bool
	promptPrefix      string
	lastSavedIdx      int

	ctx    context.Context
	cancel context.CancelFunc
	done   bool
}

// NewREPL creates a new REPL instance with the given options.
func NewREPL(ctx context.Context, opts ...Option) (*REPL, error) {
	rctx, cancel := context.WithCancel(ctx)
	r := &REPL{
		ctx:               rctx,
		cancel:            cancel,
		promptPrefix:      "> ",
		transcriptEnabled: true,
	}

	for _, opt := range opts {
		if err := opt(r); err != nil {
			cancel()
			return nil, fmt.Errorf("apply option: %w", err)
		}
	}

	if r.app == nil {
		cancel()
		return nil, fmt.Errorf("application is required")
	}

	// Create session
	session, err := r.app.NewSession()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("create session: %w", err)
	}
	r.session = session
	r.memStore = r.app.MemoryStore()

	// Create transcript writer
	if r.transcriptEnabled {
		tw, err := NewFileTranscriptWriterWithDir(session.ID(), r.transcriptDir)
		if err != nil {
			cancel()
			return nil, fmt.Errorf("create transcript writer: %w", err)
		}
		r.transcript = tw
		slog.Info("[repl] transcript enabled", "path", tw.Path())
	} else {
		r.transcript = NopTranscriptWriter{}
	}

	return r, nil
}

// isExitCommand checks if the input is an exit command.
func isExitCommand(text string) bool {
	text = strings.ToLower(strings.TrimSpace(text))
	return text == cmdExit || text == cmdQuit
}

// Run starts the REPL and blocks until the user exits or context is cancelled.
func (r *REPL) Run() error {
	slog.Info("[repl] starting", "session_id", r.session.ID())
	fmt.Printf("OneBlade Agent Ready. Type '%s' or '%s' to exit, or press Ctrl+D.\n", cmdExit, cmdQuit)

	// Exit checker to handle exit/quit commands
	exitChecker := func(in string, breakline bool) bool {
		if isExitCommand(in) {
			if breakline {
				// After executor is called, we can exit
				r.done = true
				return true
			}
			// Exit immediately without calling executor
			return true
		}
		return false
	}

	p := prompt.New(
		r.executor,
		r.completer,
		prompt.OptionPrefix(r.promptPrefix),
		prompt.OptionTitle("OneBlade"),
		prompt.OptionPrefixTextColor(prompt.Cyan),
		prompt.OptionPreviewSuggestionTextColor(prompt.Blue),
		prompt.OptionSelectedSuggestionBGColor(prompt.LightGray),
		prompt.OptionSuggestionBGColor(prompt.DarkGray),
		prompt.OptionSetExitCheckerOnInput(exitChecker),
		prompt.OptionAddKeyBind(prompt.KeyBind{
			Key: prompt.ControlC,
			Fn: func(b *prompt.Buffer) {
				r.done = true
			},
		}),
	)

	p.Run()
	return nil
}

// executor handles user input and runs the agent.
func (r *REPL) executor(input string) {
	text := strings.TrimSpace(input)
	if text == "" {
		return
	}

	// Check exit commands
	switch strings.ToLower(text) {
	case cmdExit, cmdQuit:
		fmt.Println("Goodbye!")
		// ExitChecker will handle the actual exit, we just print the message
		return
	case cmdHelp:
		r.printHelp()
		return
	}

	// Check if context is cancelled
	if r.ctx.Err() != nil {
		slog.Error("[repl] context cancelled", "error", r.ctx.Err())
		r.done = true
		return
	}

	// Write user message to transcript
	if err := r.transcript.WriteUserMessage(text); err != nil {
		slog.Warn("[repl] failed to write user message to transcript", "error", err)
	}

	// Run agent
	output, err := r.app.Run(r.ctx, blades.UserMessage(text), blades.WithSession(r.session))
	if err != nil {
		if r.ctx.Err() != nil {
			slog.Error("[repl] interrupted", "error", r.ctx.Err())
			r.done = true
			return
		}
		slog.Error("[repl] run failed", "error", err)
		fmt.Printf("Error: %v\n", err)
		return
	}

	responseText := output.Text()
	fmt.Println(responseText)

	// Write assistant message to transcript
	if err := r.transcript.WriteAssistantMessage(responseText); err != nil {
		slog.Warn("[repl] failed to write assistant message to transcript", "error", err)
	}

	// Save to memory store
	r.saveToMemory()
}

// completer provides command suggestions.
func (r *REPL) completer(d prompt.Document) []prompt.Suggest {
	suggestions := []prompt.Suggest{
		{Text: cmdExit, Description: "Exit the application"},
		{Text: cmdQuit, Description: "Exit the application"},
		{Text: cmdHelp, Description: "Show available commands"},
	}
	return prompt.FilterHasPrefix(suggestions, d.GetWordBeforeCursor(), true)
}

// printHelp prints available commands.
func (r *REPL) printHelp() {
	fmt.Printf(`
Available Commands:
  %s        Show this help message
  %s, %s  Exit the application
  
Keyboard Shortcuts:
  Ctrl+C      Exit
  Ctrl+D      Exit
  Ctrl+A      Go to beginning of line
  Ctrl+E      Go to end of line
  Ctrl+W      Delete word before cursor
  ↑/↓         Navigate history
`, cmdHelp, cmdExit, cmdQuit)
}

// saveToMemory saves new messages to the memory store.
func (r *REPL) saveToMemory() {
	if r.memStore == nil {
		return
	}

	history := r.session.History()
	for _, m := range history[r.lastSavedIdx:] {
		if m == nil {
			continue
		}
		switch m.Role {
		case blades.RoleUser, blades.RoleAssistant:
			_ = r.memStore.AddMemory(r.ctx, &memory.Memory{Content: m})
		}
	}
	r.lastSavedIdx = len(history)
}

// Close performs cleanup: flushes transcript and releases resources.
func (r *REPL) Close() error {
	slog.Info("[repl] closing", "session_id", r.session.ID())

	// Flush transcript
	if err := r.transcript.Flush(); err != nil {
		slog.Warn("[repl] flush transcript failed", "error", err)
	}

	// Close transcript writer
	if err := r.transcript.Close(); err != nil {
		slog.Warn("[repl] close transcript failed", "error", err)
	}

	// Cancel context
	if r.cancel != nil {
		r.cancel()
	}

	return nil
}
