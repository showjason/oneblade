package app

import (
	"fmt"
	"os"

	"github.com/go-kratos/blades"
	"github.com/go-kratos/blades/contrib/openai"

	"github.com/oneblade/config"
)

// buildModel 构建 LLM Model
func buildModel(cfg *config.LLMConfig) (blades.ModelProvider, error) {
	if cfg == nil {
		return nil, fmt.Errorf("LLM config is nil")
	}

	// 目前只支持 openai provider
	switch cfg.Provider {
	case "openai", "":
		return buildOpenAIModel(cfg)
	default:
		return nil, fmt.Errorf("unsupported LLM provider: %s", cfg.Provider)
	}
}

// buildOpenAIModel 构建 OpenAI Model
func buildOpenAIModel(cfg *config.LLMConfig) (blades.ModelProvider, error) {
	apiKey := cfg.APIKey
	if apiKey == "" {
		// 如果配置文件中没有设置，尝试从环境变量读取
		apiKey = os.Getenv("OPENAI_API_KEY")
	}

	if apiKey == "" {
		return nil, fmt.Errorf("OpenAI API key not configured")
	}

	modelName := cfg.Model
	if modelName == "" {
		modelName = "gpt-4"
	}

	opts := openai.Config{
		APIKey: apiKey,
	}

	// 设置 BaseURL（如果配置了）
	if cfg.BaseURL != "" {
		opts.BaseURL = cfg.BaseURL
	}

	// Note: Timeout 可以通过 http.Client 设置，但 openai.Config 不直接支持
	// 如需支持超时，需要自定义 http.Client

	return openai.NewModel(modelName, opts), nil
}
