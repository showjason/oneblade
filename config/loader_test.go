package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createTempConfig 创建临时配置文件并返回路径
func createTempConfig(t *testing.T, content string) string {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.toml")
	err := os.WriteFile(configPath, []byte(content), 0644)
	require.NoError(t, err)
	return configPath
}

// setupEnvVars 设置环境变量并返回清理函数
func setupEnvVars(t *testing.T, vars map[string]string) func() {
	originalVars := make(map[string]string)
	for k, v := range vars {
		originalVars[k] = os.Getenv(k)
		os.Setenv(k, v)
	}
	return func() {
		for k, originalVal := range originalVars {
			if originalVal == "" {
				os.Unsetenv(k)
			} else {
				os.Setenv(k, originalVal)
			}
		}
	}
}

// containsAny 检查字符串是否包含任意一个子字符串
func containsAny(s string, substrings []string) bool {
	for _, substr := range substrings {
		if strings.Contains(s, substr) {
			return true
		}
	}
	return false
}

// testLLMConfig 测试用的最小 LLM 配置
const testLLMConfig = `
[llm.agents.test_agent]
provider = "openai"
model = "gpt-4"
api_key = "test-api-key"
`

// TestNewLoader 测试创建配置加载器
// 验证加载器能正确创建并保存配置文件路径
func TestNewLoader(t *testing.T) {
	loader := NewLoader("./test.toml")
	assert.NotNil(t, loader)
	assert.Equal(t, "./test.toml", loader.ConfigPath())
}

// TestLoader_Load_ValidConfig 测试加载有效配置
// 验证配置能正确解析并包含所有预期的字段和值
func TestLoader_Load_ValidConfig(t *testing.T) {
	configContent := testLLMConfig + `
[server]
addr = "localhost:8080"
timeout = "30s"

[data.database]
driver = "mysql"
source = "user:pass@tcp(localhost:3306)/db"

[data.redis]
addr = "localhost:6379"
read_timeout = "5s"
write_timeout = "5s"

[services.prometheus]
type = "prometheus"
enabled = true

[services.pagerduty]
type = "pagerduty"
enabled = false
`
	configPath := createTempConfig(t, configContent)

	loader := NewLoader(configPath)

	cfg, err := loader.Load()
	require.NoError(t, err)
	assert.NotNil(t, cfg)
	assert.Equal(t, "localhost:8080", cfg.Server.Addr)
	assert.True(t, cfg.Services["prometheus"].Enabled)
	assert.False(t, cfg.Services["pagerduty"].Enabled)
}

// TestLoader_Load_FileNotFound 测试文件不存在的情况
// 验证错误消息包含文件路径和明确的错误信息
func TestLoader_Load_FileNotFound(t *testing.T) {
	nonExistentPath := "/nonexistent/config.toml"
	loader := NewLoader(nonExistentPath)

	_, err := loader.Load()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "load config file")
	assert.Contains(t, err.Error(), nonExistentPath)
}

// TestLoader_Load_InvalidTOML 测试无效的 TOML 格式
// 验证解析错误能被正确捕获并返回明确的错误信息
func TestLoader_Load_InvalidTOML(t *testing.T) {
	configContent := `invalid toml content [`
	configPath := createTempConfig(t, configContent)

	loader := NewLoader(configPath)

	_, err := loader.Load()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse config file")
	assert.Contains(t, err.Error(), configPath)
}

// TestLoader_Load_WithEnvVars 测试环境变量替换功能
// 使用表驱动测试覆盖多种环境变量场景
func TestLoader_Load_WithEnvVars(t *testing.T) {
	tests := []struct {
		name            string
		configContent   string
		envVars         map[string]string
		expectedAddr    string
		expectedTimeout time.Duration
	}{
		{
			name: "使用默认值",
			configContent: testLLMConfig + `
[server]
addr = "${SERVER_ADDR:localhost:8080}"
timeout = "${TIMEOUT:30s}"

[services.prometheus]
type = "prometheus"
enabled = true
`,
			envVars:         map[string]string{},
			expectedAddr:    "localhost:8080",
			expectedTimeout: 30 * time.Second,
		},
		{
			name: "环境变量覆盖默认值",
			configContent: testLLMConfig + `
[server]
addr = "${SERVER_ADDR:localhost:8080}"
timeout = "${TIMEOUT:30s}"

[services.prometheus]
type = "prometheus"
enabled = true
`,
			envVars:         map[string]string{"SERVER_ADDR": "0.0.0.0:9090", "TIMEOUT": "60s"},
			expectedAddr:    "0.0.0.0:9090",
			expectedTimeout: 60 * time.Second,
		},
		{
			name: "部分环境变量设置",
			configContent: testLLMConfig + `
[server]
addr = "${SERVER_ADDR:localhost:8080}"
timeout = "${TIMEOUT:30s}"

[services.prometheus]
type = "prometheus"
enabled = true
`,
			envVars:         map[string]string{"SERVER_ADDR": "127.0.0.1:8080"},
			expectedAddr:    "127.0.0.1:8080",
			expectedTimeout: 30 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configPath := createTempConfig(t, tt.configContent)
			cleanup := setupEnvVars(t, tt.envVars)
			defer cleanup()

			loader := NewLoader(configPath)

			cfg, err := loader.Load()
			require.NoError(t, err)
			assert.Equal(t, tt.expectedAddr, cfg.Server.Addr)
			assert.Equal(t, tt.expectedTimeout, cfg.Server.Timeout)
		})
	}
}

// Test_expandEnv 测试环境变量展开功能
// 覆盖各种场景：简单变量、默认值、多个变量、边界情况等
func Test_expandEnv(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		envVars  map[string]string
		expected string
	}{
		{
			name:     "simple variable",
			input:    "Hello ${NAME}",
			envVars:  map[string]string{"NAME": "World"},
			expected: "Hello World",
		},
		{
			name:     "variable with default",
			input:    "Hello ${NAME:Guest}",
			envVars:  map[string]string{},
			expected: "Hello Guest",
		},
		{
			name:     "variable with value overrides default",
			input:    "Hello ${NAME:Guest}",
			envVars:  map[string]string{"NAME": "World"},
			expected: "Hello World",
		},
		{
			name:     "multiple variables",
			input:    "${HOST}:${PORT:8080}",
			envVars:  map[string]string{"HOST": "localhost"},
			expected: "localhost:8080",
		},
		{
			name:     "no variable",
			input:    "plain text",
			envVars:  map[string]string{},
			expected: "plain text",
		},
		{
			name:     "empty default",
			input:    "Value: ${VAR:}",
			envVars:  map[string]string{},
			expected: "Value: ",
		},
		{
			name:     "multiple consecutive variables",
			input:    "${A}${B}${C}",
			envVars:  map[string]string{"A": "1", "B": "2", "C": "3"},
			expected: "123",
		},
		{
			name:     "variable with special characters in value",
			input:    "Path: ${PATH}",
			envVars:  map[string]string{"PATH": "/usr/bin:/usr/local/bin"},
			expected: "Path: /usr/bin:/usr/local/bin",
		},
		{
			name:     "variable name with underscore",
			input:    "${MY_VAR:default}",
			envVars:  map[string]string{"MY_VAR": "value"},
			expected: "value",
		},
		{
			name:     "variable with colon in default",
			input:    "${HOST:localhost:8080}",
			envVars:  map[string]string{},
			expected: "localhost:8080",
		},
		{
			name:     "unclosed brace",
			input:    "${VAR",
			envVars:  map[string]string{},
			expected: "${VAR",
		},
		{
			name:     "variable not set without default",
			input:    "Value: ${NOT_SET}",
			envVars:  map[string]string{},
			expected: "Value: ",
		},
		{
			name:     "nested braces in text",
			input:    "Text ${VAR} more text",
			envVars:  map[string]string{"VAR": "value"},
			expected: "Text value more text",
		},
		{
			name:     "variable with empty value uses default",
			input:    "${VAR:default}",
			envVars:  map[string]string{"VAR": ""},
			expected: "default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleanup := setupEnvVars(t, tt.envVars)
			defer cleanup()

			result := expandEnv(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestLoader_Load_DuplicateConfig 测试重复配置的处理
// 验证重复配置能被正确检测并返回错误
func TestLoader_Load_DuplicateConfig(t *testing.T) {
	t.Run("重复的顶级配置段", func(t *testing.T) {
		// TOML 解析器会在解析阶段检测重复的键并报错
		configContent := testLLMConfig + `
[server]
addr = "localhost:8080"

[server]
addr = "localhost:9090"

[services.prometheus]
type = "prometheus"
enabled = true
`
		configPath := createTempConfig(t, configContent)

		loader := NewLoader(configPath)

		_, err := loader.Load()
		// TOML 库会在解析阶段检测到重复配置并报错
		require.Error(t, err)
		assert.Contains(t, err.Error(), "parse config file")
		assert.Contains(t, err.Error(), "already been defined")
		assert.NotNil(t, loader)
	})

	t.Run("无重复的有效配置", func(t *testing.T) {
		configContent := testLLMConfig + `
[server]
addr = "localhost:8080"
timeout = "30s"

[data.database]
driver = "mysql"

[data.redis]
addr = "localhost:6379"

[services.prometheus]
type = "prometheus"
enabled = true
`
		configPath := createTempConfig(t, configContent)

		loader := NewLoader(configPath)

		_, err := loader.Load()
		require.NoError(t, err)
		assert.NotNil(t, loader)
	})
}

// TestLoader_GetServiceOptions 测试获取服务选项
// 覆盖正常获取、不存在 service、service 没有 options 等场景
func TestLoader_GetServiceOptions(t *testing.T) {
	t.Run("正常获取 service options", func(t *testing.T) {
		configContent := testLLMConfig + `
[server]
addr = "localhost:8080"

[services.prometheus]
type = "prometheus"
enabled = true

[services.prometheus.options]
address = "http://localhost:9090"
timeout = "30s"

[services.pagerduty]
type = "pagerduty"
enabled = false

[services.pagerduty.options]
api_key = "test-key"
`
		configPath := createTempConfig(t, configContent)

		loader := NewLoader(configPath)

		_, err := loader.Load()
		require.NoError(t, err)

		// Get prometheus service options
		primitive, meta, err := loader.GetServiceOptions("prometheus")
		require.NoError(t, err)
		assert.NotNil(t, meta)
		assert.NotNil(t, primitive)

		// Get pagerduty service options
		primitive2, meta2, err := loader.GetServiceOptions("pagerduty")
		require.NoError(t, err)
		assert.NotNil(t, meta2)
		assert.NotNil(t, primitive2)
	})

	t.Run("不存在的 service", func(t *testing.T) {
		configContent := testLLMConfig + `
[server]
addr = "localhost:8080"

[services.prometheus]
type = "prometheus"
enabled = true
`
		configPath := createTempConfig(t, configContent)

		loader := NewLoader(configPath)

		_, err := loader.Load()
		require.NoError(t, err)

		_, _, err = loader.GetServiceOptions("nonexistent")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("service 没有 options", func(t *testing.T) {
		configContent := testLLMConfig + `
[server]
addr = "localhost:8080"

[services.prometheus]
type = "prometheus"
enabled = true
`
		configPath := createTempConfig(t, configContent)

		loader := NewLoader(configPath)

		_, err := loader.Load()
		require.NoError(t, err)

		// Service 存在但没有 options，应该能正常返回（primitive 可能为空）
		primitive, meta, err := loader.GetServiceOptions("prometheus")
		require.NoError(t, err)
		assert.NotNil(t, meta)
		// primitive 可能为空，这是正常的
		_ = primitive
	})
}

// TestLoader_Get 测试获取当前配置
// 验证加载前返回 nil，加载后返回正确的配置
func TestLoader_Get(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.toml")

	configContent := testLLMConfig + `
[server]
addr = "localhost:8080"

[services.prometheus]
type = "prometheus"
enabled = true
`
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	loader := NewLoader(configPath)

	// Before loading, Get should return error
	_, err = loader.Get()
	require.NoError(t, err)
	require.Error(t, err)

	// After loading
	_, err = loader.Load()
	require.NoError(t, err)

	cfg, err := loader.Get()
	require.NoError(t, err)
	assert.NotNil(t, cfg)
	assert.Equal(t, "localhost:8080", cfg.Server.Addr)
}

// TestLoader_Load_ValidationError 测试配置验证失败的情况
// 覆盖 required、hostname_port、oneof 等验证规则
func TestLoader_Load_ValidationError(t *testing.T) {
	t.Run("缺少必需的 server.addr", func(t *testing.T) {
		configContent := testLLMConfig + `
[server]
timeout = "30s"

[services.prometheus]
type = "prometheus"
enabled = true
`
		configPath := createTempConfig(t, configContent)

		loader := NewLoader(configPath)

		_, err := loader.Load()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "validate config")
		// 验证错误应该包含 Addr 字段的信息
		assert.True(t, strings.Contains(err.Error(), "Addr") || strings.Contains(err.Error(), "addr"),
			"错误消息应该包含 Addr 或 addr: %s", err.Error())
	})

	t.Run("无效的 hostname_port 格式", func(t *testing.T) {
		configContent := testLLMConfig + `
[server]
addr = "invalid-address"
timeout = "30s"

[services.prometheus]
type = "prometheus"
enabled = true
`
		configPath := createTempConfig(t, configContent)

		loader := NewLoader(configPath)

		_, err := loader.Load()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "validate config")
	})

	t.Run("有效的 hostname_port 格式", func(t *testing.T) {
		configContent := testLLMConfig + `
[server]
addr = "localhost:8080"
timeout = "30s"

[services.prometheus]
type = "prometheus"
enabled = true
`
		configPath := createTempConfig(t, configContent)

		loader := NewLoader(configPath)

		_, err := loader.Load()
		require.NoError(t, err)
	})

	t.Run("service type 不在允许的列表中", func(t *testing.T) {
		configContent := testLLMConfig + `
[server]
addr = "localhost:8080"

[services.invalid]
type = "invalid_type"
enabled = true
`
		configPath := createTempConfig(t, configContent)

		loader := NewLoader(configPath)

		_, err := loader.Load()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "validate config")
	})

	t.Run("service 缺少 type", func(t *testing.T) {
		configContent := testLLMConfig + `
[server]
addr = "localhost:8080"

[services.prometheus]
enabled = true
`
		configPath := createTempConfig(t, configContent)

		loader := NewLoader(configPath)

		_, err := loader.Load()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "validate config")
	})

	t.Run("有效的 service type", func(t *testing.T) {
		configContent := testLLMConfig + `
[server]
addr = "localhost:8080"

[services.prometheus]
type = "prometheus"
enabled = true

[services.pagerduty]
type = "pagerduty"
enabled = false

[services.opensearch]
type = "opensearch"
enabled = true
`
		configPath := createTempConfig(t, configContent)

		loader := NewLoader(configPath)

		_, err := loader.Load()
		require.NoError(t, err)
	})
}
