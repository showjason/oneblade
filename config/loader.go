package config

import (
	"fmt"
	"log/slog"
	"os"
	"regexp"

	"github.com/BurntSushi/toml"
	"github.com/go-playground/validator/v10"
	"github.com/oneblade/internal/consts"
)

// Loader 配置加载器
type Loader struct {
	configPath  string
	config      *Config
	serviceMeta map[string]*toml.MetaData
	validator   *validator.Validate
}

// NewLoader 创建配置加载器
func NewLoader(configPath string) *Loader {
	loader := &Loader{
		configPath:  configPath,
		validator:   validator.New(),
		serviceMeta: make(map[string]*toml.MetaData),
	}
	return loader
}

// extractServiceMeta 提取每个 service 的元数据
func (l *Loader) extractServiceMeta(meta *toml.MetaData, cfg *Config) {
	// 为每个 service 保存元数据副本
	for name := range cfg.Services {
		l.serviceMeta[name] = meta
	}
}

// Load 加载并解析配置
func (l *Loader) Load() (*Config, error) {
	// 加载并展开环境变量
	content, err := l.loadAndExpand(l.configPath)
	if err != nil {
		return nil, fmt.Errorf("load config file %s: %w", l.configPath, err)
	}

	// 解析配置
	var cfg Config
	meta, err := toml.Decode(content, &cfg)
	if err != nil {
		return nil, fmt.Errorf("parse config file %s: %w", l.configPath, err)
	}

	// strict mode check for unknown keys
	if err := l.checkUnknownKeys(&meta); err != nil {
		return nil, err
	}

	applyDefaults(&cfg)

	// 验证配置结构
	if err := l.validate(&cfg); err != nil {
		return nil, fmt.Errorf("validate config: %w", err)
	}

	if err := validateConversation(cfg.Conversation); err != nil {
		return nil, fmt.Errorf("validate config: %w", err)
	}

	// 筛选 enabled agents 和 services
	l.filterEnabledAgents(&cfg)
	l.filterEnabledServices(&cfg)

	// 提取 service 元数据
	l.extractServiceMeta(&meta, &cfg)

	l.config = &cfg
	return &cfg, nil
}

func applyDefaults(cfg *Config) {
	if cfg.Conversation.ContextWindowTokens == 0 {
		slog.Warn("conversation.context_window_tokens is 0, using default value",
			"default", DefaultContextWindowTokens)
		cfg.Conversation.ContextWindowTokens = DefaultContextWindowTokens
	}
	if cfg.Conversation.CompressionThreshold == 0 {
		slog.Warn("conversation.compression_threshold is 0, using default value",
			"default", DefaultCompressionThreshold)
		cfg.Conversation.CompressionThreshold = DefaultCompressionThreshold
	}
	if cfg.Conversation.MaxInContextMessages == 0 {
		slog.Warn("conversation.max_in_context_messages is 0, using default value",
			"default", DefaultMaxInContextMessages)
		cfg.Conversation.MaxInContextMessages = DefaultMaxInContextMessages
	}
	if cfg.Conversation.RetainRecentMessages == 0 {
		slog.Warn("conversation.retain_recent_messages is 0, using default value",
			"default", DefaultRetainRecentMessages)
		cfg.Conversation.RetainRecentMessages = DefaultRetainRecentMessages
	}
	if cfg.Conversation.SummaryMaxOutputTokens == 0 {
		slog.Warn("conversation.summary_max_output_tokens is 0, using default value",
			"default", DefaultSummaryMaxOutputTokens)
		cfg.Conversation.SummaryMaxOutputTokens = DefaultSummaryMaxOutputTokens
	}
	if cfg.Conversation.SummaryModelAgent == "" {
		slog.Warn("conversation.summary_model_agent is empty, using default value",
			"default", consts.AgentNameOrchestrator)
		cfg.Conversation.SummaryModelAgent = consts.AgentNameOrchestrator
	}
}

func validateConversation(cfg ConversationConfig) error {
	if cfg.RetainRecentMessages >= cfg.MaxInContextMessages {
		return fmt.Errorf("conversation.retain_recent_messages must be < conversation.max_in_context_messages")
	}
	return nil
}

// checkUnknownKeys checks for undecoded keys in the config
func (l *Loader) checkUnknownKeys(meta *toml.MetaData) error {
	if undecoded := meta.Undecoded(); len(undecoded) > 0 {
		var unknown []toml.Key
		for _, k := range undecoded {
			// Ignore [services.<name>.options] as they are delayed parsed
			if len(k) >= 3 && k[0] == "services" && k[2] == "options" {
				continue
			}
			unknown = append(unknown, k)
		}
		if len(unknown) > 0 {
			return fmt.Errorf("parse config file %s: unknown keys: %v", l.configPath, unknown)
		}
	}
	return nil
}

// loadAndExpand 读取文件并展开环境变量
func (l *Loader) loadAndExpand(path string) (string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	expanded := expandEnv(string(content))
	return expanded, nil
}

// expandEnv 展开环境变量占位符
// 支持 ${VAR} 和 ${VAR:default} 语法
func expandEnv(s string) string {
	// 匹配 ${VAR} 或 ${VAR:default}
	re := regexp.MustCompile(`\$\{([^}:]+)(?::([^}]*))?\}`)

	return re.ReplaceAllStringFunc(s, func(match string) string {
		// 提取变量名和默认值
		groups := re.FindStringSubmatch(match)
		if len(groups) < 2 {
			return match
		}

		varName := groups[1]
		defaultVal := ""
		if len(groups) >= 3 {
			defaultVal = groups[2]
		}

		// 查找环境变量
		if val := os.Getenv(varName); val != "" {
			return val
		}
		return defaultVal
	})
}

// validate 验证配置
func (l *Loader) validate(cfg *Config) error {
	return l.validator.Struct(cfg)
}

// filterEnabledAgents 筛选出 enabled 的 agents
func (l *Loader) filterEnabledAgents(cfg *Config) {
	enabledAgents := make(map[string]AgentConfig)
	for name, agent := range cfg.Agents {
		if agent.Enabled {
			enabledAgents[name] = agent
		}
	}
	cfg.Agents = enabledAgents
}

// filterEnabledServices 筛选出 enabled 的 services
func (l *Loader) filterEnabledServices(cfg *Config) {
	enabledServices := make(map[string]ServiceConfig)
	for name, service := range cfg.Services {
		if service.Enabled {
			enabledServices[name] = service
		}
	}
	cfg.Services = enabledServices
}

// Get 获取当前配置
func (l *Loader) Get() (*Config, error) {
	if l.config == nil {
		return nil, fmt.Errorf("config not loaded")
	}
	return l.config, nil
}

// ConfigPath 获取配置文件路径
func (l *Loader) ConfigPath() string {
	return l.configPath
}

// GetServiceOptions 获取指定 service 的原始配置数据
func (l *Loader) GetServiceOptions(serviceName string) (toml.Primitive, *toml.MetaData, error) {
	cfg := l.config
	if cfg == nil {
		return toml.Primitive{}, nil, fmt.Errorf("config not loaded")
	}

	serviceCfg, ok := cfg.Services[serviceName]
	if !ok {
		return toml.Primitive{}, nil, fmt.Errorf("service %s not found", serviceName)
	}

	meta := l.serviceMeta[serviceName]
	if meta == nil {
		return toml.Primitive{}, nil, fmt.Errorf("no metadata for service %s", serviceName)
	}

	return serviceCfg.Options, meta, nil
}
