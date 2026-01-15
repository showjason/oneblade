package session

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/go-kratos/blades"
	"github.com/google/uuid"

	"github.com/oneblade/config"
	"github.com/oneblade/internal/summary"
)

// ManagedSession 是一个带“增量摘要 + 裁剪”能力的会话实现。
//
// 关键点：
// - history 本体只保留真实对话轮次（不会写入 summary）。
// - summary 存在 Session.State 中，并在 History() 返回值中作为 system message 自动注入。
// - 仅在 assistant 完成消息追加后评估触发条件。
type ManagedSession struct {
	id string

	cfg        config.ConversationConfig
	summarizer summary.Summarizer

	mu      sync.RWMutex
	state   blades.State
	history []*blades.Message
}

type ManagedSessionConfig struct {
	Conversation config.ConversationConfig
	Summarizer   summary.Summarizer
}

func NewManagedSession(cfg ManagedSessionConfig) (*ManagedSession, error) {
	if cfg.Summarizer == nil {
		return nil, fmt.Errorf("summarizer is required")
	}
	return &ManagedSession{
		id:         uuid.NewString(),
		cfg:        cfg.Conversation,
		summarizer: cfg.Summarizer,
		state:      make(blades.State),
	}, nil
}

func (s *ManagedSession) ID() string {
	return s.id
}

func (s *ManagedSession) State() blades.State {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state.Clone()
}

func (s *ManagedSession) SetState(key string, value any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state[key] = value
}

func (s *ManagedSession) History() []*blades.Message {
	s.mu.RLock()
	defer s.mu.RUnlock()

	base := make([]*blades.Message, 0, len(s.history)+1)
	if summaryText, ok := s.state[StateKeyConversationSummary].(string); ok && summaryText != "" {
		base = append(base, blades.SystemMessage(summaryText))
	}
	base = append(base, s.history...)
	return base
}

func (s *ManagedSession) Append(ctx context.Context, message *blades.Message) error {
	if message == nil {
		return nil
	}

	s.mu.Lock()
	s.history = append(s.history, message)

	// 仅在 assistant 完成消息后评估触发条件
	if message.Role != blades.RoleAssistant || message.Status != blades.StatusCompleted {
		s.mu.Unlock()
		return nil
	}

	// 更新最近一次 token 使用量（用于下一轮判断）
	s.state[StateKeyLastPromptTokens] = message.TokenUsage.InputTokens
	s.state[StateKeyLastTotalTokens] = message.TokenUsage.TotalTokens

	// 计算触发阈值（使用上一轮 prompt token 作为 proxy）
	threshold := int64(float64(s.cfg.ContextWindowTokens) * s.cfg.CompressionThreshold)
	needByTokens := message.TokenUsage.InputTokens >= threshold
	needByCount := len(s.history) > s.cfg.MaxInContextMessages

	if !(needByTokens || needByCount) {
		s.mu.Unlock()
		return nil
	}

	cutoff := len(s.history) - s.cfg.RetainRecentMessages
	if cutoff <= 0 {
		// 没有可摘要的段，避免出现负切片或空摘要。
		s.mu.Unlock()
		return nil
	}
	delta := make([]*blades.Message, cutoff)
	copy(delta, s.history[:cutoff])
	tail := make([]*blades.Message, len(s.history)-cutoff)
	copy(tail, s.history[cutoff:])

	previousSummary, _ := s.state[StateKeyConversationSummary].(string)
	s.mu.Unlock()

	newSummary, _, err := s.summarizer.Summarize(ctx, previousSummary, delta)
	if err != nil {
		return err
	}

	s.mu.Lock()
	s.state[StateKeyConversationSummary] = newSummary
	s.state[StateKeySummaryUpdatedAt] = time.Now().UTC().Format(time.RFC3339Nano)
	s.history = tail
	s.mu.Unlock()

	return nil
}
