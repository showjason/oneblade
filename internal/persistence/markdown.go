package persistence

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/go-kratos/blades"
)

const (
	markdownHeaderV1     = "<!-- oneblade-session:v1 -->"
	beginSessionJSONDump = "<!-- BEGIN_ONEBLADE_SESSION_JSON -->"
	endSessionJSONDump   = "<!-- END_ONEBLADE_SESSION_JSON -->"
)

type SessionDumpV1 struct {
	SessionID string         `json:"session_id"`
	State     map[string]any `json:"state,omitempty"`
	Messages  []MessageV1    `json:"messages"`
}

type MessageV1 struct {
	Role   string `json:"role"`
	Author string `json:"author,omitempty"`
	Text   string `json:"text"`
}

func BuildDumpV1(session blades.Session) (SessionDumpV1, error) {
	if session == nil {
		return SessionDumpV1{}, fmt.Errorf("session is nil")
	}

	history := session.History()
	messages := make([]MessageV1, 0, len(history))
	for _, m := range history {
		if m == nil {
			continue
		}
		switch m.Role {
		case blades.RoleUser, blades.RoleAssistant:
			messages = append(messages, MessageV1{
				Role:   string(m.Role),
				Author: m.Author,
				Text:   m.Text(),
			})
		}
	}

	return SessionDumpV1{
		SessionID: session.ID(),
		State:     session.State(),
		Messages:  messages,
	}, nil
}

func EncodeMarkdownV1(dump SessionDumpV1, title string) ([]byte, error) {
	body, err := json.MarshalIndent(dump, "", "  ")
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	buf.WriteString(markdownHeaderV1)
	buf.WriteString("\n\n")

	if strings.TrimSpace(title) != "" {
		buf.WriteString("# ")
		buf.WriteString(strings.TrimSpace(title))
		buf.WriteString("\n\n")
	}

	if len(dump.Messages) > 0 {
		buf.WriteString("## Conversation\n\n")
		for _, m := range dump.Messages {
			role := strings.TrimSpace(m.Role)
			if role == "" {
				role = "unknown"
			}
			buf.WriteString("### ")
			buf.WriteString(role)
			if strings.TrimSpace(m.Author) != "" {
				buf.WriteString(" (")
				buf.WriteString(strings.TrimSpace(m.Author))
				buf.WriteString(")")
			}
			buf.WriteString("\n\n")
			buf.WriteString("```text\n")
			buf.WriteString(m.Text)
			buf.WriteString("\n```\n\n")
		}
	}

	buf.WriteString(beginSessionJSONDump)
	buf.WriteString("\n")
	buf.Write(body)
	buf.WriteString("\n")
	buf.WriteString(endSessionJSONDump)
	buf.WriteString("\n")

	return buf.Bytes(), nil
}

func DecodeMarkdownV1(markdown []byte) (SessionDumpV1, error) {
	content := string(markdown)

	begin := strings.Index(content, beginSessionJSONDump)
	if begin < 0 {
		return SessionDumpV1{}, fmt.Errorf("missing json dump begin marker")
	}
	begin += len(beginSessionJSONDump)

	end := strings.Index(content, endSessionJSONDump)
	if end < 0 || end < begin {
		return SessionDumpV1{}, fmt.Errorf("missing json dump end marker")
	}

	raw := strings.TrimSpace(content[begin:end])
	var dump SessionDumpV1
	if err := json.Unmarshal([]byte(raw), &dump); err != nil {
		return SessionDumpV1{}, err
	}
	return dump, nil
}

