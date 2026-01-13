package llm

import (
	"context"

	"github.com/go-kratos/blades"
	"github.com/go-kratos/blades/contrib/gemini"
	"google.golang.org/genai"

	"github.com/oneblade/config"
)

type geminiBuilder struct {
	baseURL string
	model   string
	apiKey  string
}

func newGeminiBuilder() ModelBuilder {
	return &geminiBuilder{
		baseURL: "https://generativelanguage.googleapis.com/",
		model:   "gemini-2.5-flash",
		apiKey:  "GEMINI_API_KEY,GOOGLE_API_KEY",
	}
}

func (b *geminiBuilder) GetModel(cfg *config.AgentLLMConfig) string {
	return resolveModel(cfg, b.model)
}

func (b *geminiBuilder) GetBaseURL(cfg *config.AgentLLMConfig) string {
	return cfg.BaseURL
}

func (b *geminiBuilder) Build(ctx context.Context, cfg *config.AgentLLMConfig) (blades.ModelProvider, error) {
	apiKey, err := resolveAPIKey(cfg, b.apiKey)
	if err != nil {
		return nil, err
	}

	var opts gemini.Config
	opts.ClientConfig = genai.ClientConfig{
		APIKey: apiKey,
	}
	opts.MaxOutputTokens = int32(*cfg.MaxTokens)
	opts.Temperature = float32(*cfg.Temperature)

	return gemini.NewModel(ctx, b.GetModel(cfg), opts)
}
