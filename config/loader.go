package config

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/go-playground/validator/v10"
)

// Loader 配置加载器
type Loader struct {
	configPath    string
	config        *Config
	collectorMeta map[string]*toml.MetaData
	validator     *validator.Validate
}

// NewLoader 创建配置加载器
// configPath: 配置文件路径 (如 "./config.toml")
func NewLoader(configPath string) (*Loader, error) {
	loader := &Loader{
		configPath:    configPath,
		validator:     validator.New(),
		collectorMeta: make(map[string]*toml.MetaData),
	}
	return loader, nil
}

// checkDuplicateConfig 检查所有配置段是否重复定义（包括嵌套配置）
func checkDuplicateConfig(meta toml.MetaData) error {
	keys := meta.Keys()
	defined := make(map[string]bool)

	for _, key := range keys {
		if len(key) == 0 {
			continue
		}

		// 构建完整的配置路径，例如：
		// - "server"
		// - "data.database"
		// - "data.redis"
		// - "collectors.prometheus"
		// - "collectors.prometheus.options"
		configPath := strings.Join(key, ".")

		// 检查是否已定义
		if defined[configPath] {
			return fmt.Errorf("duplicate configuration \"%s\" defined", configPath)
		}

		defined[configPath] = true
	}
	return nil
}

// extractCollectorMeta 提取每个 collector 的元数据
func (l *Loader) extractCollectorMeta(meta *toml.MetaData, cfg *Config) {
	// 为每个 collector 保存元数据副本
	for name := range cfg.Collectors {
		metaCopy := *meta
		l.collectorMeta[name] = &metaCopy
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

	// 检查重复配置
	if err := checkDuplicateConfig(meta); err != nil {
		return nil, err
	}

	// 初始化 Collectors map（如果为 nil）
	if cfg.Collectors == nil {
		cfg.Collectors = make(map[string]CollectorConfig)
	}

	// 提取 collector 元数据
	l.extractCollectorMeta(&meta, &cfg)

	// 验证配置
	if err := l.validate(&cfg); err != nil {
		return nil, fmt.Errorf("validate config: %w", err)
	}

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
func (l *Loader) Get() *Config {
	return l.config
}

// GetCollectorMeta 获取指定 collector 的 TOML 元数据（用于延迟解析 Primitive）
func (l *Loader) GetCollectorMeta(collectorName string) *toml.MetaData {
	return l.collectorMeta[collectorName]
}

// ConfigPath 获取配置文件路径
func (l *Loader) ConfigPath() string {
	return l.configPath
}

// GetCollectorOptions 获取指定 collector 的原始配置数据
func (l *Loader) GetCollectorOptions(collectorName string) (toml.Primitive, *toml.MetaData, error) {
	cfg := l.config
	if cfg == nil {
		var zero toml.Primitive
		return zero, nil, fmt.Errorf("config not loaded")
	}

	collectorCfg, ok := cfg.Collectors[collectorName]
	if !ok {
		var zero toml.Primitive
		return zero, nil, fmt.Errorf("collector %s not found", collectorName)
	}

	meta := l.collectorMeta[collectorName]
	if meta == nil {
		var zero toml.Primitive
		return zero, nil, fmt.Errorf("no metadata for collector %s", collectorName)
	}

	return collectorCfg.Options, meta, nil
}
