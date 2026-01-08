package llm

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/go-kratos/blades"
	"github.com/oneblade/config"
)

// ModelBuilder 定义模型构建器接口
type ModelBuilder interface {
	GetModel(cfg *config.AgentLLMConfig) string
	GetBaseURL(cfg *config.AgentLLMConfig) string
	Build(ctx context.Context, cfg *config.AgentLLMConfig) (blades.ModelProvider, error)
}

func resolveModel(cfg *config.AgentLLMConfig, model string) string {
	if model == "" || strings.TrimSpace(model) == "" {
		return model
	}
	return cfg.Model
}

// getAPIKey 通用 API Key 获取逻辑
func resolveAPIKey(cfg *config.AgentLLMConfig, apiKey string) (string, error) {
	apiKeyVal := firstNonEmpty(strings.TrimSpace(cfg.APIKey), os.Getenv(apiKey))
	if apiKeyVal == "" {
		return "", fmt.Errorf("%s api key not configured (api_key or %s)", cfg.Model, apiKey)
	}
	return apiKeyVal, nil
}

// setBaseURL 设置默认 BaseURL
func resolveBaseURL(cfg *config.AgentLLMConfig, defaultURL string) string {
	if cfg.BaseURL == "" || strings.TrimSpace(cfg.BaseURL) == "" {
		return defaultURL
	}
	return cfg.BaseURL
}
