package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/BurntSushi/toml"
	"github.com/go-playground/validator/v10"
	"github.com/joho/godotenv"
)

// Loader 配置加载器
type Loader struct {
	baseDir       string
	config        *Config
	collectorMeta map[string]*toml.MetaData
	mu            sync.RWMutex
	validator     *validator.Validate
}

// NewLoader 创建配置加载器
// baseDir: 配置文件目录路径 (如 "./configs")
func NewLoader(baseDir string) (*Loader, error) {
	loader := &Loader{
		baseDir:       baseDir,
		validator:     validator.New(),
		collectorMeta: make(map[string]*toml.MetaData),
	}
	return loader, nil
}

// scanConfigFiles 扫描配置目录获取所有 .toml 文件
func scanConfigFiles(baseDir string) ([]string, error) {
	var files []string

	err := filepath.Walk(baseDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		// 只处理 .toml 文件，排除 .env 文件
		if strings.HasSuffix(info.Name(), ".toml") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("scan config directory: %w", err)
	}

	if len(files) == 0 {
		return nil, fmt.Errorf("no .toml config files found in %s", baseDir)
	}

	// 按文件名排序，确保加载顺序一致
	sort.Strings(files)
	return files, nil
}

// checkNonCollectorConfig 检查非 collector 配置是否重复定义
func checkNonCollectorConfig(meta toml.MetaData, defined map[string]string, filePath string) error {
	keys := meta.Keys()
	// 使用 set 记录当前文件中已处理的顶级配置段
	processedInFile := make(map[string]bool)

	for _, key := range keys {
		if len(key) == 0 {
			continue
		}
		// 检查顶级配置段：app, server, data
		topKey := key[0]
		if topKey == "app" || topKey == "server" || topKey == "data" {
			// 跳过当前文件中已处理的配置段
			if processedInFile[topKey] {
				continue
			}
			processedInFile[topKey] = true

			if prevFile, exists := defined[topKey]; exists {
				return fmt.Errorf("duplicate configuration \"%s\" defined in both \"%s\" and \"%s\"",
					topKey, filepath.Base(prevFile), filepath.Base(filePath))
			}
			defined[topKey] = filePath
		}
	}
	return nil
}

// mergeCollectors 合并 collector 配置并检查重复
func mergeCollectors(
	target map[string]CollectorConfig,
	source map[string]CollectorConfig,
	filePath string,
	defined map[string]string,
	collectorMeta map[string]*toml.MetaData,
	meta *toml.MetaData,
) error {
	for name, cfg := range source {
		if prevFile, exists := defined[name]; exists {
			return fmt.Errorf("duplicate collector \"%s\" defined in both \"%s\" and \"%s\"",
				name, filepath.Base(prevFile), filepath.Base(filePath))
		}
		target[name] = cfg
		defined[name] = filePath
		collectorMeta[name] = meta
	}
	return nil
}

// Load 加载并解析配置
func (l *Loader) Load() (*Config, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	// 加载 .env 文件（如果存在）
	// 注意：godotenv.Load() 不会覆盖已存在的环境变量（如通过 export 设置的）
	// 因此系统环境变量优先级更高
	envPath := filepath.Join(l.baseDir, ".env")
	if _, err := os.Stat(envPath); err == nil {
		if err := godotenv.Load(envPath); err != nil {
			return nil, fmt.Errorf("load .env file: %w", err)
		}
	}

	// 扫描配置目录获取所有 .toml 文件
	files, err := scanConfigFiles(l.baseDir)
	if err != nil {
		return nil, err
	}

	// 初始化配置和记录 map
	var cfg Config
	cfg.Collectors = make(map[string]CollectorConfig)
	l.collectorMeta = make(map[string]*toml.MetaData)

	nonCollectorConfig := make(map[string]string) // 记录非 collector 配置段首次定义的文件
	collectorConfig := make(map[string]string)    // 记录 collector 首次定义的文件

	// 遍历每个配置文件
	for _, filePath := range files {
		// 加载并展开环境变量
		content, err := l.loadAndExpand(filePath)
		if err != nil {
			return nil, fmt.Errorf("load config file %s: %w", filepath.Base(filePath), err)
		}

		// 解析到临时配置结构
		var tempCfg Config
		meta, err := toml.Decode(content, &tempCfg)
		if err != nil {
			return nil, fmt.Errorf("parse config file %s: %w", filepath.Base(filePath), err)
		}

		// 检查非 collector 配置的重复定义
		if err := checkNonCollectorConfig(meta, nonCollectorConfig, filePath); err != nil {
			return nil, err
		}

		// 合并非 collector 配置（只取首次定义）
		if _, exists := nonCollectorConfig["app"]; exists && nonCollectorConfig["app"] == filePath {
			cfg.App = tempCfg.App
		}
		if _, exists := nonCollectorConfig["server"]; exists && nonCollectorConfig["server"] == filePath {
			cfg.Server = tempCfg.Server
		}
		if _, exists := nonCollectorConfig["data"]; exists && nonCollectorConfig["data"] == filePath {
			cfg.Data = tempCfg.Data
		}

		// 合并 collector 配置
		if tempCfg.Collectors != nil {
			metaCopy := meta
			if err := mergeCollectors(cfg.Collectors, tempCfg.Collectors, filePath, collectorConfig, l.collectorMeta, &metaCopy); err != nil {
				return nil, err
			}
		}
	}

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

// Get 线程安全地获取当前配置
func (l *Loader) Get() *Config {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.config
}

// GetCollectorMeta 获取指定 collector 的 TOML 元数据（用于延迟解析 Primitive）
func (l *Loader) GetCollectorMeta(collectorName string) *toml.MetaData {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.collectorMeta[collectorName]
}

// BaseDir 获取配置目录
func (l *Loader) BaseDir() string {
	return l.baseDir
}

// GetCollectorOptions 获取指定 collector 的原始配置数据
func (l *Loader) GetCollectorOptions(collectorName string) (toml.Primitive, *toml.MetaData, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()

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
