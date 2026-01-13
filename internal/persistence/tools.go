package persistence

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-kratos/blades"
	"github.com/go-kratos/blades/tools"
)

type SaveContextRequest struct {
	Path  string `json:"path,omitempty" jsonschema:"The output file path. If empty, defaults to ./sessions/<session_id>.md."`
	Title string `json:"title,omitempty" jsonschema:"Optional markdown title."`
}

type SaveContextResponse struct {
	Path         string `json:"path"`
	MessageCount int    `json:"message_count"`
}

type LoadContextRequest struct {
	Path string `json:"path" jsonschema:"The input markdown file path that contains an embedded session dump."`
}

type LoadContextResponse struct {
	MessageCount int  `json:"message_count"`
	HasState     bool `json:"has_state"`
}

func NewSaveContextTool() (tools.Tool, error) {
	return tools.NewFunc(
		"SaveContext",
		"Save the current conversation context (session history + state) into a local markdown file. The file is human readable and contains an embedded JSON dump for loading.",
		func(ctx context.Context, req SaveContextRequest) (SaveContextResponse, error) {
			session, ok := blades.FromSessionContext(ctx)
			if !ok || session == nil {
				return SaveContextResponse{}, fmt.Errorf("session not found in context")
			}

			dump, err := BuildDumpV1(session)
			if err != nil {
				return SaveContextResponse{}, err
			}
			content, err := EncodeMarkdownV1(dump, req.Title)
			if err != nil {
				return SaveContextResponse{}, err
			}

			path := strings.TrimSpace(req.Path)
			if path == "" {
				path = filepath.Join("sessions", session.ID()+".md")
			}

			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				return SaveContextResponse{}, err
			}
			if err := os.WriteFile(path, content, 0o644); err != nil {
				return SaveContextResponse{}, err
			}

			return SaveContextResponse{
				Path:         path,
				MessageCount: len(dump.Messages),
			}, nil
		},
	)
}

func NewLoadContextTool() (tools.Tool, error) {
	return tools.NewFunc(
		"LoadContext",
		"Load conversation context from a local markdown file that contains an embedded JSON dump, and append it to the current session. Note: loaded history affects subsequent turns.",
		func(ctx context.Context, req LoadContextRequest) (LoadContextResponse, error) {
			path := strings.TrimSpace(req.Path)
			if path == "" {
				return LoadContextResponse{}, fmt.Errorf("path is required")
			}

			session, ok := blades.FromSessionContext(ctx)
			if !ok || session == nil {
				return LoadContextResponse{}, fmt.Errorf("session not found in context")
			}

			b, err := os.ReadFile(path)
			if err != nil {
				return LoadContextResponse{}, err
			}

			dump, err := DecodeMarkdownV1(b)
			if err != nil {
				return LoadContextResponse{}, err
			}

			hasState := len(dump.State) > 0
			for k, v := range dump.State {
				session.SetState(k, v)
			}

			for _, m := range dump.Messages {
				role := strings.TrimSpace(m.Role)
				switch role {
				case string(blades.RoleUser):
					msg := blades.UserMessage(m.Text)
					msg.Author = m.Author
					msg.Status = blades.StatusCompleted
					if err := session.Append(ctx, msg); err != nil {
						return LoadContextResponse{}, err
					}
				case string(blades.RoleAssistant):
					msg := blades.AssistantMessage(m.Text)
					msg.Author = m.Author
					msg.Status = blades.StatusCompleted
					if err := session.Append(ctx, msg); err != nil {
						return LoadContextResponse{}, err
					}
				default:
					continue
				}
			}

			return LoadContextResponse{
				MessageCount: len(dump.Messages),
				HasState:     hasState,
			}, nil
		},
	)
}

func DecodeSaveContextResponse(raw string) (SaveContextResponse, error) {
	var resp SaveContextResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		return SaveContextResponse{}, err
	}
	return resp, nil
}

func DecodeLoadContextResponse(raw string) (LoadContextResponse, error) {
	var resp LoadContextResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		return LoadContextResponse{}, err
	}
	return resp, nil
}
