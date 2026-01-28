package summary

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/go-kratos/blades"
)

type Summarizer interface {
	// Summarize 生成“完整替换”的对话摘要（中文）。
	// previousSummary 允许为空；messages 是本次需要被摘要的历史片段（不应包含 previousSummary）。
	Summarize(ctx context.Context, previousSummary string, messages []*blades.Message) (newSummary string, usage blades.TokenUsage, err error)
}

type Config struct {
	Model              blades.ModelProvider
	MaxOutputTokens    int
	MaxSummaryChars    int
	IncludeToolDetails bool
}

func (c *Config) validate() error {
	if c.Model == nil {
		return fmt.Errorf("summary model is required")
	}
	return nil
}

type modelSummarizer struct {
	model              blades.ModelProvider
	maxOutputTokens    int
	maxSummaryChars    int
	includeToolDetails bool
}

func NewSummarizer(cfg Config) (Summarizer, error) {
	if err := (&cfg).validate(); err != nil {
		return nil, err
	}
	return &modelSummarizer{
		model:              cfg.Model,
		maxOutputTokens:    cfg.MaxOutputTokens,
		maxSummaryChars:    cfg.MaxSummaryChars,
		includeToolDetails: cfg.IncludeToolDetails,
	}, nil
}

func (s *modelSummarizer) Summarize(ctx context.Context, previousSummary string, messages []*blades.Message) (string, blades.TokenUsage, error) {
	instruction := blades.SystemMessage(buildSummaryInstruction(s.maxOutputTokens, s.maxSummaryChars))
	user := blades.UserMessage(buildSummaryInput(previousSummary, messages, s.includeToolDetails))

	slog.Info("[summary] start", "previous_len", len(previousSummary), "delta_count", len(messages))

	resp, err := s.model.Generate(ctx, &blades.ModelRequest{
		Instruction: instruction,
		Messages:    []*blades.Message{user},
	})
	if err != nil {
		slog.Error("[summary] failed", "error", err)
		return "", blades.TokenUsage{}, err
	}
	if resp == nil || resp.Message == nil {
		err := fmt.Errorf("summary model returned empty response")
		slog.Error("[summary] failed", "error", err)
		return "", blades.TokenUsage{}, err
	}
	slog.Info("[summary] complete",
		"input_tokens", resp.Message.TokenUsage.InputTokens,
		"output_tokens", resp.Message.TokenUsage.OutputTokens,
	)
	return strings.TrimSpace(resp.Message.Text()), resp.Message.TokenUsage, nil
}

func buildSummaryInstruction(maxOutputTokens, maxSummaryChars int) string {
	var b strings.Builder
	b.WriteString("你是对话上下文压缩器。你的任务是把一段对话历史压缩成一份“完整替换”的中文摘要，用于后续对话继续。\n")
	b.WriteString("\n")
	b.WriteString("要求：\n")
	b.WriteString("- 只输出摘要正文，不要输出任何前后缀说明。\n")
	b.WriteString("- 摘要必须可被后续对话直接作为 system 上下文使用。\n")
	b.WriteString("- 你会收到 previous_summary（可能为空）与 delta_transcript（本次新增历史）。生成的新摘要必须合并两者，并允许 delta_transcript 纠正/更新 previous_summary 中的错误或过期信息。\n")
	b.WriteString("- 删除重复、寒暄、无关内容；保留可操作信息。\n")
	b.WriteString("- 输出结构（建议严格按此顺序）：\n")
	b.WriteString("  1) 关键事实\n")
	b.WriteString("  2) 已确认结论\n")
	b.WriteString("  3) 用户偏好与约束\n")
	b.WriteString("  4) 未解决问题\n")
	b.WriteString("  5) 后续计划\n")
	if maxSummaryChars > 0 {
		b.WriteString(fmt.Sprintf("- 长度限制：不超过 %d 个中文字符（尽量短）。\n", maxSummaryChars))
	}
	if maxOutputTokens > 0 {
		b.WriteString(fmt.Sprintf("- 输出 token 目标：不超过 %d（尽量短）。\n", maxOutputTokens))
	}
	return b.String()
}

func buildSummaryInput(previousSummary string, messages []*blades.Message, includeToolDetails bool) string {
	var b strings.Builder
	b.WriteString("previous_summary:\n")
	if strings.TrimSpace(previousSummary) == "" {
		b.WriteString("(empty)\n")
	} else {
		b.WriteString(previousSummary)
		if !strings.HasSuffix(previousSummary, "\n") {
			b.WriteString("\n")
		}
	}
	b.WriteString("\n")
	b.WriteString("delta_transcript:\n")
	b.WriteString(renderTranscript(messages, includeToolDetails))
	return b.String()
}

func renderTranscript(messages []*blades.Message, includeToolDetails bool) string {
	var b strings.Builder
	for _, m := range messages {
		if m == nil {
			continue
		}
		switch m.Role {
		case blades.RoleUser:
			b.WriteString("User: ")
			b.WriteString(strings.TrimSpace(m.Text()))
			b.WriteString("\n")
		case blades.RoleAssistant:
			b.WriteString("Assistant: ")
			b.WriteString(strings.TrimSpace(m.Text()))
			b.WriteString("\n")
		case blades.RoleSystem:
			// system 消息一般是指令，摘要中价值有限；如确实在历史段里出现，也保留为 context
			txt := strings.TrimSpace(m.Text())
			if txt != "" {
				b.WriteString("System: ")
				b.WriteString(txt)
				b.WriteString("\n")
			}
		case blades.RoleTool:
			if !includeToolDetails {
				continue
			}
			b.WriteString("Tool: ")
			txt := strings.TrimSpace(m.Text())
			if txt != "" {
				b.WriteString(txt)
				b.WriteString("\n")
				continue
			}
			// ToolPart 的 request/response 通常在 Parts 中
			for _, part := range m.Parts {
				switch v := any(part).(type) {
				case blades.ToolPart:
					if v.Name != "" {
						b.WriteString(v.Name)
					} else {
						b.WriteString("tool_call")
					}
					if strings.TrimSpace(v.Response) != "" {
						b.WriteString(" => ")
						b.WriteString(strings.TrimSpace(v.Response))
					}
					b.WriteString("\n")
				}
			}
		default:
			// unknown role: ignore
		}
	}
	return b.String()
}
