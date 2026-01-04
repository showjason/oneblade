package config

import (
	"time"

	"github.com/BurntSushi/toml"
)

// LLMConfig LLM 配置
type LLMConfig struct {
	Provider    string                `toml:"provider" validate:"required"`
	Model       string                `toml:"model" validate:"required"`
	APIKey      string                `toml:"api_key" validate:"required"`
	BaseURL     string                `toml:"base_url"`
	Timeout     string                `toml:"timeout"`
	MaxTokens   int                   `toml:"max_tokens" validate:"omitempty,gt=0"`
	Temperature float64               `toml:"temperature" validate:"omitempty,gte=0,lte=2"`
	Agents      map[string]*LLMConfig `toml:"agents"` // SubAgent 特定配置
}

// Merge 合并配置（子配置继承父配置未设置的字段）
// 返回一个新的 LLMConfig，不修改原配置
func (c *LLMConfig) Merge(parent *LLMConfig) *LLMConfig {
	if parent == nil {
		return c
	}
	result := *c
	if result.Provider == "" {
		result.Provider = parent.Provider
	}
	if result.Model == "" {
		result.Model = parent.Model
	}
	if result.APIKey == "" {
		result.APIKey = parent.APIKey
	}
	if result.BaseURL == "" {
		result.BaseURL = parent.BaseURL
	}
	if result.Timeout == "" {
		result.Timeout = parent.Timeout
	}
	if result.MaxTokens == 0 {
		result.MaxTokens = parent.MaxTokens
	}
	if result.Temperature == 0 {
		result.Temperature = parent.Temperature
	}
	return &result
}

// GetAgentConfig 获取指定 agent 的 LLM 配置
// 如果 agent 未配置则返回主 LLM 配置
func (c *LLMConfig) GetAgentConfig(agentName string) *LLMConfig {
	if c.Agents == nil {
		return c
	}
	agentCfg, ok := c.Agents[agentName]
	if !ok || agentCfg == nil {
		return c
	}
	return agentCfg.Merge(c)
}

// Config 根配置结构
type Config struct {
	Server   ServerConfig             `toml:"server" validate:"required"`
	Data     DataConfig               `toml:"data"`
	LLM      LLMConfig                `toml:"llm" validate:"required"`
	Services map[string]ServiceConfig `toml:"services" validate:"required,dive"`
}

// ServerConfig 服务器配置
type ServerConfig struct {
	Addr    string        `toml:"addr" validate:"required,hostname_port"`
	Timeout time.Duration `toml:"timeout"`
}

// DataConfig 数据存储配置
type DataConfig struct {
	Database DatabaseConfig `toml:"database"`
	Redis    RedisConfig    `toml:"redis"`
}

// DatabaseConfig 数据库配置
type DatabaseConfig struct {
	Driver string `toml:"driver"`
	Source string `toml:"source"`
}

// RedisConfig Redis配置
type RedisConfig struct {
	Addr         string        `toml:"addr"`
	ReadTimeout  time.Duration `toml:"read_timeout"`
	WriteTimeout time.Duration `toml:"write_timeout"`
}

type ServiceConfig struct {
	Type        string         `toml:"type" validate:"required,oneof=prometheus pagerduty opensearch"`
	Description string         `toml:"description"`
	Enabled     bool           `toml:"enabled"`
	Options     toml.Primitive `toml:"options"`
}
