package llm

import (
	"context"

	"github.com/go-kratos/blades"
	"github.com/go-kratos/blades/contrib/openai"
	"github.com/oneblade/config"
)

type openaiBuilder struct {
	baseURL string
	model   string
	apiKey  string
}

func newOpenAIBuilder() ModelBuilder {
	return &openaiBuilder{
		model:   "gpt-4",
		baseURL: "https://api.openai.com/v1",
		apiKey:  "OPENAI_API_KEY",
	}
}

func (b *openaiBuilder) GetModel(cfg *config.AgentLLMConfig) string {
	return resolveModel(cfg, b.model)
}

func (b *openaiBuilder) GetBaseURL(cfg *config.AgentLLMConfig) string {
	return resolveBaseURL(cfg, b.baseURL)
}

func (b *openaiBuilder) Build(ctx context.Context, cfg *config.AgentLLMConfig) (blades.ModelProvider, error) {
	apiKey, err := resolveAPIKey(cfg, b.apiKey)
	if err != nil {
		return nil, err
	}

	baseURL := b.GetBaseURL(cfg)

	opts := openai.Config{
		APIKey:  apiKey,
		BaseURL: baseURL,
	}

	opts.MaxOutputTokens = int64(*cfg.MaxTokens)
	opts.Temperature = *cfg.Temperature

	return openai.NewModel(b.GetModel(cfg), opts), nil
}
