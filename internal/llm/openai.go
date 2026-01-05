package llm

import (
	"fmt"

	"github.com/go-kratos/blades"
	"github.com/go-kratos/blades/contrib/openai"

	"github.com/oneblade/config"
)

func buildOpenAI(cfg config.AgentLLMConfig) (blades.ModelProvider, error) {
	apiKey := envOrValue(cfg.APIKey, "OPENAI_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("openai api key not configured (api_key or OPENAI_API_KEY)")
	}

	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.openai.com/v1"
	}

	opts := openai.Config{
		APIKey:  apiKey,
		BaseURL: cfg.BaseURL,
	}

	if cfg.MaxTokens != nil {
		opts.MaxOutputTokens = int64(*cfg.MaxTokens)
	}
	if cfg.Temperature != nil {
		opts.Temperature = *cfg.Temperature
	}

	return openai.NewModel(cfg.Model, opts), nil
}


