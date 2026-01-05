package llm

import (
	"fmt"
	"os"
	"strings"

	"github.com/go-kratos/blades"
	"github.com/go-kratos/blades/contrib/openai"

	"github.com/oneblade/config"
)

func buildOpenAI(cfg config.AgentLLMConfig) (blades.ModelProvider, error) {
	apiKey := firstNonEmpty(strings.TrimSpace(cfg.APIKey), os.Getenv("OPENAI_API_KEY"))
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

	// applyDefaults 已确保这些字段不为 nil
	opts.MaxOutputTokens = int64(*cfg.MaxTokens)
	opts.Temperature = *cfg.Temperature

	return openai.NewModel(cfg.Model, opts), nil
}
