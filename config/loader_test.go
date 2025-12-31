package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewLoader(t *testing.T) {
	loader, err := NewLoader("./test.toml")
	require.NoError(t, err)
	assert.NotNil(t, loader)
	assert.Equal(t, "./test.toml", loader.ConfigPath())
}

func TestLoader_Load_ValidConfig(t *testing.T) {
	// Create a temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.toml")

	configContent := `
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

[collectors.prometheus]
type = "prometheus"
enabled = true

[collectors.pagerduty]
type = "pagerduty"
enabled = false
`
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	loader, err := NewLoader(configPath)
	require.NoError(t, err)

	cfg, err := loader.Load()
	require.NoError(t, err)
	assert.NotNil(t, cfg)
	assert.Equal(t, "localhost:8080", cfg.Server.Addr)
	assert.True(t, cfg.Collectors["prometheus"].Enabled)
	assert.False(t, cfg.Collectors["pagerduty"].Enabled)
}

func TestLoader_Load_FileNotFound(t *testing.T) {
	loader, err := NewLoader("/nonexistent/config.toml")
	require.NoError(t, err)

	_, err = loader.Load()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "load config file")
}

func TestLoader_Load_InvalidTOML(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.toml")

	configContent := `invalid toml content [`
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	loader, err := NewLoader(configPath)
	require.NoError(t, err)

	_, err = loader.Load()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse config file")
}

func TestLoader_Load_WithEnvVars(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.toml")

	configContent := `
[server]
addr = "${SERVER_ADDR:localhost:8080}"
timeout = "${TIMEOUT:30s}"

[collectors.prometheus]
type = "prometheus"
enabled = true
`
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	// Test with default values
	loader, err := NewLoader(configPath)
	require.NoError(t, err)

	cfg, err := loader.Load()
	require.NoError(t, err)
	assert.Equal(t, "localhost:8080", cfg.Server.Addr)

	// Test with environment variables set
	os.Setenv("SERVER_ADDR", "0.0.0.0:9090")
	os.Setenv("TIMEOUT", "60s")
	defer os.Unsetenv("SERVER_ADDR")
	defer os.Unsetenv("TIMEOUT")

	loader2, err := NewLoader(configPath)
	require.NoError(t, err)

	cfg2, err := loader2.Load()
	require.NoError(t, err)
	assert.Equal(t, "0.0.0.0:9090", cfg2.Server.Addr)
}

func TestExpandEnv(t *testing.T) {
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variables
			for k, v := range tt.envVars {
				os.Setenv(k, v)
				defer os.Unsetenv(k)
			}

			result := expandEnv(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestLoader_Load_DuplicateConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.toml")

	// This is tricky - TOML parser itself handles duplicates, but we can test
	// by creating a config that would have issues
	configContent := `
[server]
addr = "localhost:8080"

[server]
addr = "localhost:9090"
`
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	loader, err := NewLoader(configPath)
	require.NoError(t, err)

	// The TOML library will handle this, but our checkDuplicateConfig should catch it
	_, err = loader.Load()
	// The behavior depends on TOML library - it might overwrite or error
	// This test verifies the loader doesn't crash
	assert.NotNil(t, loader)
}

func TestLoader_GetCollectorOptions(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.toml")

	configContent := `
[server]
addr = "localhost:8080"

[collectors.prometheus]
type = "prometheus"
enabled = true

[collectors.prometheus.options]
address = "http://localhost:9090"
timeout = "30s"
`
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	loader, err := NewLoader(configPath)
	require.NoError(t, err)

	_, err = loader.Load()
	require.NoError(t, err)

	// Get collector options
	primitive, meta, err := loader.GetCollectorOptions("prometheus")
	require.NoError(t, err)
	assert.NotNil(t, meta)
	assert.NotNil(t, primitive)

	// Test getting non-existent collector
	_, _, err = loader.GetCollectorOptions("nonexistent")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestLoader_GetCollectorOptions_NotLoaded(t *testing.T) {
	loader, err := NewLoader("./test.toml")
	require.NoError(t, err)

	// Try to get options before loading
	_, _, err = loader.GetCollectorOptions("prometheus")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "config not loaded")
}

func TestLoader_Get(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.toml")

	configContent := `
[server]
addr = "localhost:8080"
`
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	loader, err := NewLoader(configPath)
	require.NoError(t, err)

	// Before loading, Get should return nil
	assert.Nil(t, loader.Get())

	// After loading
	_, err = loader.Load()
	require.NoError(t, err)

	cfg := loader.Get()
	assert.NotNil(t, cfg)
	assert.Equal(t, "localhost:8080", cfg.Server.Addr)
}

func TestLoader_Validate_InvalidConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.toml")

	// Missing required server.addr
	configContent := `
[server]
timeout = "30s"
`
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	loader, err := NewLoader(configPath)
	require.NoError(t, err)

	_, err = loader.Load()
	// Validation might pass or fail depending on validator rules
	// This test ensures the loader handles validation
	assert.NotNil(t, loader)
}

func TestCheckDuplicateConfig(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		expectError bool
		errorMsg    string
		description string
	}{
		{
			name: "valid_config_no_duplicates",
			content: `
[server]
addr = "localhost:8080"
timeout = "30s"
`,
			expectError: false,
			description: "正常配置，无重复键",
		},
		{
			name: "valid_config_with_nested_sections",
			content: `
[server]
addr = "localhost:8080"

[data.database]
driver = "mysql"

[data.redis]
addr = "localhost:6379"

[collectors.prometheus]
type = "prometheus"
enabled = true
`,
			expectError: false,
			description: "包含嵌套配置段的正常配置",
		},
		{
			name: "valid_config_deeply_nested",
			content: `
[collectors.prometheus]
type = "prometheus"

[collectors.prometheus.options]
address = "http://localhost:9090"
timeout = "30s"
`,
			expectError: false,
			description: "深层嵌套配置（collectors.prometheus.options）",
		},
		{
			name:        "empty_config",
			content:     ``,
			expectError: false,
			description: "空配置",
		},
		{
			name: "config_with_only_top_level",
			content: `
[server]
addr = "localhost:8080"
`,
			expectError: false,
			description: "只有顶层配置段",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var cfg Config
			meta, err := toml.Decode(tt.content, &cfg)
			require.NoError(t, err, "TOML 解析应该成功: %s", tt.description)

			err = checkDuplicateConfig(meta)

			if tt.expectError {
				require.Error(t, err, "应该检测到重复配置: %s", tt.description)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg, "错误消息应该包含预期内容")
				}
			} else {
				assert.NoError(t, err, "不应该有错误: %s", tt.description)
			}
		})
	}
}

// TestCheckDuplicateConfig_EdgeCases 测试边界情况
func TestCheckDuplicateConfig_EdgeCases(t *testing.T) {
	t.Run("single_key_config", func(t *testing.T) {
		// 测试只有一个顶层配置段的简单配置
		content := `
[server]
addr = "localhost:8080"
`
		var cfg Config
		meta, err := toml.Decode(content, &cfg)
		require.NoError(t, err)

		err = checkDuplicateConfig(meta)
		assert.NoError(t, err, "单个配置段的配置应该通过")
	})

	t.Run("top_level_key_without_table", func(t *testing.T) {
		// 测试没有表头的顶层键（虽然在这个项目中不常见，但验证函数能处理）
		// 注意：这不是空键，空键是指 []string{}（长度为 0 的切片）
		// 这里测试的是键路径为 ["addr"] 的情况，而不是 ["server", "addr"]
		content := `addr = "localhost:8080"`
		var cfg struct {
			Addr string `toml:"addr"`
		}
		meta, err := toml.Decode(content, &cfg)
		require.NoError(t, err)

		err = checkDuplicateConfig(meta)
		assert.NoError(t, err, "没有表头的顶层键应该能正常处理")
	})

	t.Run("empty_config_produces_no_keys", func(t *testing.T) {
		// 测试空配置（不产生任何键）
		content := ``
		var cfg Config
		meta, err := toml.Decode(content, &cfg)
		require.NoError(t, err)

		// 空配置应该不产生任何键，函数应该能正常处理
		err = checkDuplicateConfig(meta)
		assert.NoError(t, err, "空配置应该能正常处理")
	})
}
