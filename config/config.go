package config

import (
	"fmt"
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
	APIKey      string   `toml:"api_key"`
	BaseURL     string   `toml:"base_url"`
	Timeout     string   `toml:"timeout"`
	MaxTokens   *int     `toml:"max_tokens" validate:"omitempty,gt=0"`
	Temperature *float64 `toml:"temperature" validate:"omitempty,gte=0,lte=2"`
}

// AgentConfig Agent 配置
// 包含 enabled 开关和 LLM 配置
type AgentConfig struct {
	Enabled bool           `toml:"enabled"`
	LLM     AgentLLMConfig `toml:"llm"`
}

func (c *Config) GetAgentConfig(agentName string) (*AgentConfig, error) {
	if len(c.Agents) == 0 {
		return nil, fmt.Errorf("no agents configured")
	}
	acfg, ok := c.Agents[agentName]
	if !ok {
		return nil, fmt.Errorf("agent %s not found", agentName)
	}
	return &acfg, nil
}

// Config 根配置结构
type Config struct {
	Server   ServerConfig             `toml:"server" validate:"required"`
	Data     DataConfig               `toml:"data"`
	Log      LogConfig                `toml:"log"`
	Agents   map[string]AgentConfig   `toml:"agents" validate:"required,dive"`
	Services map[string]ServiceConfig `toml:"services" validate:"required,dive"`
}

// LogConfig 日志配置
type LogConfig struct {
	Level  string `toml:"level" validate:"omitempty,oneof=debug info warn error"`
	Format string `toml:"format" validate:"omitempty,oneof=text json"`
	Output string `toml:"output"`
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
