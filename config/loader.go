package config

import (
	"fmt"
	"os"
	"regexp"

	"github.com/BurntSushi/toml"
	"github.com/go-playground/validator/v10"
)

// Loader 配置加载器
type Loader struct {
	configPath  string
	config      *Config
	serviceMeta map[string]*toml.MetaData
	validator   *validator.Validate
}

// NewLoader 创建配置加载器
// configPath: 配置文件路径 (如 "./config.toml")
func NewLoader(configPath string) (*Loader, error) {
	loader := &Loader{
		configPath:  configPath,
		validator:   validator.New(),
		serviceMeta: make(map[string]*toml.MetaData),
	}
	return loader, nil
}

// extractServiceMeta 提取每个 service 的元数据
func (l *Loader) extractServiceMeta(meta *toml.MetaData, cfg *Config) {
	// 为每个 service 保存元数据副本
	for name := range cfg.Services {
		metaCopy := *meta
		l.serviceMeta[name] = &metaCopy
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

	// 严格模式：禁止存在未解码的配置项，避免拼写错误/过期配置被静默忽略。
	if undecoded := meta.Undecoded(); len(undecoded) > 0 {
		// Note: `[services.<name>.options]` 采用 toml.Primitive 延迟解析，
		// meta.Undecoded() 会把 options 下的 leaf keys 视为未解码，这是预期行为。
		// 因此这里需要忽略该路径下的 keys。
		var unknown []toml.Key
		for _, k := range undecoded {
			// Expected undecoded key example: ["services","prometheus","options","address"]
			if len(k) >= 3 && k[0] == "services" && k[2] == "options" {
				continue
			}
			unknown = append(unknown, k)
		}
		if len(unknown) > 0 {
			return nil, fmt.Errorf("parse config file %s: unknown keys: %v", l.configPath, unknown)
		}
	}

	// 验证配置
	if err := l.validate(&cfg); err != nil {
		return nil, fmt.Errorf("validate config: %w", err)
	}

	// 提取 service 元数据
	l.extractServiceMeta(&meta, &cfg)

	l.config = &cfg
	return &cfg, nil
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
		var zero toml.Primitive
		return zero, nil, fmt.Errorf("config not loaded")
	}

	serviceCfg, ok := cfg.Services[serviceName]
	if !ok {
		var zero toml.Primitive
		return zero, nil, fmt.Errorf("service %s not found", serviceName)
	}

	meta := l.serviceMeta[serviceName]
	if meta == nil {
		var zero toml.Primitive
		return zero, nil, fmt.Errorf("no metadata for service %s", serviceName)
	}

	return serviceCfg.Options, meta, nil
}
