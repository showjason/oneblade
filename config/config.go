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

// ConversationConfig 会话级上下文控制配置
//
// 说明：
// - 这些配置不属于“某个 agent 的模型参数”，而是会话历史管理策略。
// - Loader 负责填充默认值并做跨字段约束校验（如 RetainRecentMessages < MaxInContextMessages）。
type ConversationConfig struct {
	// ContextWindowTokens 模型上下文窗口（token）
	ContextWindowTokens int `toml:"context_window_tokens" validate:"omitempty,gt=0"`
	// CompressionThreshold 触发压缩阈值 (0, 1]
	CompressionThreshold float64 `toml:"compression_threshold" validate:"omitempty,gt=0,lte=1"`
	// MaxInContextMessages 最大消息数阈值
	MaxInContextMessages int `toml:"max_in_context_messages" validate:"omitempty,gt=0"`
	// RetainRecentMessages 触发压缩后保留的尾部消息数
	RetainRecentMessages int `toml:"retain_recent_messages" validate:"omitempty,gt=0"`
	// SummaryMaxOutputTokens 摘要生成的最大输出 token
	SummaryMaxOutputTokens int `toml:"summary_max_output_tokens" validate:"omitempty,gt=0"`
	// SummaryModelAgent 使用哪个已注册的 agent 模型进行摘要（通过 ModelRegistry 获取）
	SummaryModelAgent string `toml:"summary_model_agent" validate:"omitempty"`
}

// ConversationConfig 默认值常量
const (
	DefaultContextWindowTokens    = 128000
	DefaultCompressionThreshold   = 0.8
	DefaultMaxInContextMessages   = 50
	DefaultRetainRecentMessages   = 16
	DefaultSummaryMaxOutputTokens = 768
)

// Config 根配置结构
type Config struct {
	Server       ServerConfig             `toml:"server" validate:"required"`
	Data         DataConfig               `toml:"data"`
	Log          LogConfig                `toml:"log"`
	Conversation ConversationConfig       `toml:"conversation"`
	Agents       map[string]AgentConfig   `toml:"agents" validate:"required,dive"`
	Services     map[string]ServiceConfig `toml:"services" validate:"required,dive"`
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
