package config

import (
	"time"

	"github.com/BurntSushi/toml"
)

// AgentLLMConfig 单个 Agent 的 LLM 配置
//
// 说明：
// - 本项目采用“严格 per-agent 配置”策略：不做继承/合并。
// - required 字段缺失应在启动时直接报错；可选字段缺失由运行时填充默认值。
type AgentLLMConfig struct {
	Provider    string   `toml:"provider" validate:"required,oneof=openai gemini anthropic"`
	Model       string   `toml:"model" validate:"required"`
	APIKey      string   `toml:"api_key"` // 可选：允许从环境变量读取（运行时校验）
	BaseURL     string   `toml:"base_url"`
	Timeout     string   `toml:"timeout"`
	MaxTokens   *int     `toml:"max_tokens" validate:"omitempty,gt=0"`
	Temperature *float64 `toml:"temperature" validate:"omitempty,gte=0,lte=2"`
}

// LLMConfig LLM 配置（仅包含每个 agent 的独立配置）
type LLMConfig struct {
	Agents map[string]AgentLLMConfig `toml:"agents" validate:"required,min=1,dive"`
}

// GetAgentStrict 获取指定 agent 的 LLM 配置（严格模式，不做继承）
// 若 agent 未配置则返回错误。
func (c *LLMConfig) GetAgentStrict(agentName string) (*AgentLLMConfig, error) {
	if c == nil || c.Agents == nil {
		return nil, ErrAgentLLMConfigNotFound(agentName)
	}
	cfg, ok := c.Agents[agentName]
	if !ok {
		return nil, ErrAgentLLMConfigNotFound(agentName)
	}
	return &cfg, nil
}

// ErrAgentLLMConfigNotFound 缺失 agent 配置错误
func ErrAgentLLMConfigNotFound(agentName string) error {
	return &AgentLLMConfigNotFoundError{AgentName: agentName}
}

type AgentLLMConfigNotFoundError struct {
	AgentName string
}

func (e *AgentLLMConfigNotFoundError) Error() string {
	return "llm config not found for agent: " + e.AgentName
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
