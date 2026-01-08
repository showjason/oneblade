package llm

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-kratos/blades"
	"github.com/go-playground/validator/v10"

	"github.com/oneblade/config"
)

// Factory builds a blades.ModelProvider from a per-agent LLM config.
//
// It applies defaults for optional fields, validates required fields, and
// dispatches to provider-specific builders.
type Factory struct {
	validate *validator.Validate
}

// builderRegistry 存储所有 provider 的 builder
var builderRegistry = map[string]ModelBuilder{
	"openai":    newOpenAIBuilder(),
	"anthropic": newAnthropicBuilder(),
	"gemini":    newGeminiBuilder(),
}

func NewFactory() *Factory {
	return &Factory{validate: validator.New()}
}

func (f *Factory) Build(ctx context.Context, cfg config.AgentLLMConfig) (blades.ModelProvider, error) {
	cfg.Provider = normalizeProvider(cfg.Provider)

	if err := f.validate.Struct(cfg); err != nil {
		return nil, fmt.Errorf("validate llm config: %w", err)
	}

	applyDefaults(&cfg)

	builder, ok := builderRegistry[cfg.Provider]
	if !ok {
		return nil, fmt.Errorf("unsupported llm provider: %s", cfg.Provider)
	}

	return builder.Build(ctx, &cfg)
}

func normalizeProvider(p string) string {
	return strings.ToLower(strings.TrimSpace(p))
}

func applyDefaults(cfg *config.AgentLLMConfig) {
	// Provider-specific defaults can be applied in provider builders.
	// Keep common defaults here minimal and unsurprising.
	if cfg.Timeout == "" {
		cfg.Timeout = "60s"
	}

	// 为可选字段设置默认值，避免各 builder 中重复的 nil 检查
	if cfg.MaxTokens == nil {
		defaultMaxTokens := 2048
		cfg.MaxTokens = &defaultMaxTokens
	}
	if cfg.Temperature == nil {
		defaultTemperature := 0.7
		cfg.Temperature = &defaultTemperature
	}
}

// firstNonEmpty 返回第一个非空字符串（去除首尾空格后）
// 用于按优先级顺序检查配置值和环境变量
func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
