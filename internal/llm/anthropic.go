package llm

import (
	"fmt"

	"github.com/go-kratos/blades"
	"github.com/go-kratos/blades/contrib/anthropic"

	"github.com/oneblade/config"
)

func buildAnthropic(cfg config.AgentLLMConfig) (blades.ModelProvider, error) {
	apiKey := envOrValue(cfg.APIKey, "ANTHROPIC_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("anthropic api key not configured (api_key or ANTHROPIC_API_KEY)")
	}

	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.anthropic.com"
	}

	opts := anthropic.Config{
		APIKey:  apiKey,
		BaseURL: cfg.BaseURL,
	}

	// applyDefaults 已确保这些字段不为 nil
	opts.MaxOutputTokens = int64(*cfg.MaxTokens)
	opts.Temperature = *cfg.Temperature

	return anthropic.NewModel(cfg.Model, opts), nil
}
