package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sync"

	"github.com/BurntSushi/toml"
	"github.com/go-playground/validator/v10"
	"github.com/joho/godotenv"
)

// Loader 配置加载器
type Loader struct {
	baseDir   string
	env       string
	config    *Config
	meta      *toml.MetaData
	mu        sync.RWMutex
	validator *validator.Validate
	registry  *CollectorRegistry
}

// NewLoader 创建配置加载器
// baseDir: 配置文件目录路径 (如 "./configs")
func NewLoader(baseDir string) (*Loader, error) {
	// 加载 .env 文件（如果存在）
	envPath := filepath.Join(baseDir, ".env")
	if _, err := os.Stat(envPath); err == nil {
		if err := godotenv.Load(envPath); err != nil {
			return nil, fmt.Errorf("load .env file: %w", err)
		}
	}

	// 确定环境
	env := os.Getenv("APP_ENV")
	if env == "" {
		env = "dev"
	}

	return &Loader{
		baseDir:   baseDir,
		env:       env,
		validator: validator.New(),
		registry:  NewCollectorRegistry(),
	}, nil
}

// Load 加载并解析配置
func (l *Loader) Load() (*Config, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	// 1. 加载基础配置
	basePath := filepath.Join(l.baseDir, "config.toml")
	baseContent, err := l.loadAndExpand(basePath)
	if err != nil {
		return nil, fmt.Errorf("load base config: %w", err)
	}

	// 解析基础配置
	var cfg Config
	meta, err := toml.Decode(baseContent, &cfg)
	if err != nil {
		return nil, fmt.Errorf("parse base config: %w", err)
	}

	// 2. 加载环境特定配置并合并
	envPath := filepath.Join(l.baseDir, fmt.Sprintf("config.%s.toml", l.env))
	if _, err := os.Stat(envPath); err == nil {
		envContent, err := l.loadAndExpand(envPath)
		if err != nil {
			return nil, fmt.Errorf("load env config: %w", err)
		}

		// 解析并合并环境配置
		if _, err := toml.Decode(envContent, &cfg); err != nil {
			return nil, fmt.Errorf("parse env config: %w", err)
		}
	}

	// 验证配置
	if err := l.validate(&cfg); err != nil {
		return nil, fmt.Errorf("validate config: %w", err)
	}

	l.config = &cfg
	l.meta = &meta
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

// Get 线程安全地获取当前配置
func (l *Loader) Get() *Config {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.config
}

// GetMeta 获取 TOML 元数据（用于延迟解析 Primitive）
func (l *Loader) GetMeta() *toml.MetaData {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.meta
}

// Env 获取当前环境
func (l *Loader) Env() string {
	return l.env
}

// BaseDir 获取配置目录
func (l *Loader) BaseDir() string {
	return l.baseDir
}

// ParseCollectorOptions 解析 Collector 的 Options 到具体类型
// 代理给内部的 registry
func (l *Loader) ParseCollectorOptions(collectorType string, meta *toml.MetaData, primitive toml.Primitive) (interface{}, error) {
	return l.registry.ParseOptions(collectorType, meta, primitive)
}
