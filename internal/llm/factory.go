package llm

import (
	"context"
	"fmt"
	"os"
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

func NewFactory() *Factory {
	return &Factory{validate: validator.New()}
}

func (f *Factory) Build(ctx context.Context, cfg config.AgentLLMConfig) (blades.ModelProvider, error) {
	cfg.Provider = normalizeProvider(cfg.Provider)

	if err := f.validate.Struct(cfg); err != nil {
		return nil, fmt.Errorf("validate llm config: %w", err)
	}

	applyDefaults(&cfg)

	switch cfg.Provider {
	case "openai":
		return buildOpenAI(cfg)
	case "anthropic":
		return buildAnthropic(cfg)
	case "gemini":
		return buildGemini(ctx, cfg)
	default:
		// should be unreachable due to validator `oneof`, keep as defense-in-depth
		return nil, fmt.Errorf("unsupported llm provider: %s", cfg.Provider)
	}
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
}

func envOrValue(val, envKey string) string {
	if strings.TrimSpace(val) != "" {
		return val
	}
	return os.Getenv(envKey)
}


