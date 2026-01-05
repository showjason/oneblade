package llm

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/go-kratos/blades"
	"github.com/go-kratos/blades/contrib/gemini"
	"google.golang.org/genai"

	"github.com/oneblade/config"
)

func buildGemini(ctx context.Context, cfg config.AgentLLMConfig) (blades.ModelProvider, error) {
	apiKey := firstNonEmpty(strings.TrimSpace(cfg.APIKey), os.Getenv("GEMINI_API_KEY"), os.Getenv("GOOGLE_API_KEY"))
	if apiKey == "" {
		// genai client also supports env vars, but we enforce strict startup validation here.
		return nil, fmt.Errorf("gemini api key not configured (api_key or GEMINI_API_KEY/GOOGLE_API_KEY)")
	}

	var opts gemini.Config
	opts.ClientConfig = genai.ClientConfig{
		APIKey: apiKey,
	}

	// applyDefaults 已确保这些字段不为 nil
	opts.MaxOutputTokens = int32(*cfg.MaxTokens)
	opts.Temperature = float32(*cfg.Temperature)

	return gemini.NewModel(ctx, cfg.Model, opts)
}
