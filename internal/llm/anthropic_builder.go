package llm

import (
	"context"

	"github.com/go-kratos/blades"
	"github.com/go-kratos/blades/contrib/anthropic"
	"github.com/oneblade/config"
)

type anthropicBuilder struct {
	baseURL string
	apiKey  string
	model   string
}

func newAnthropicBuilder() ModelBuilder {
	return &anthropicBuilder{
		model:   "claude-3",
		apiKey:  "ANTHROPIC_API_KEY",
		baseURL: "https://api.anthropic.com",
	}
}

func (b *anthropicBuilder) GetModel(cfg *config.AgentLLMConfig) string {
	return resolveModel(cfg, b.model)
}

func (b *anthropicBuilder) GetBaseURL(cfg *config.AgentLLMConfig) string {
	return resolveBaseURL(cfg, b.baseURL)
}

func (b *anthropicBuilder) Build(ctx context.Context, cfg *config.AgentLLMConfig) (blades.ModelProvider, error) {
	apiKey, err := resolveAPIKey(cfg, b.apiKey)
	if err != nil {
		return nil, err
	}

	baseURL := b.GetBaseURL(cfg)

	opts := anthropic.Config{

		APIKey:  apiKey,
		BaseURL: baseURL,
	}

	opts.MaxOutputTokens = int64(*cfg.MaxTokens)
	opts.Temperature = *cfg.Temperature

	return anthropic.NewModel(b.GetModel(cfg), opts), nil
}
