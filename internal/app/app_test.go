package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createTempConfig 创建临时配置文件
func createTempConfig(t *testing.T, content string) string {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.toml")
	err := os.WriteFile(configPath, []byte(content), 0644)
	require.NoError(t, err)
	return configPath
}

// TestApplication_Initialize_AgentConfigs 验证多个 Agent 配置是否被正确隔离加载
// 这是一个关键测试，用于验证 Go 1.22+ 的 for 循环变量作用域特性（即每次迭代创建新变量）是否生效，
// 确保每个 agent 只有独立的配置指针，不会发生只会指向最后一个元素的闭包问题。
func TestApplication_Initialize_AgentConfigs(t *testing.T) {
	// 定义两个 agent，拥有不同的 API Key，用于验证配置是否混淆
	configContent := `
[server]
addr = "localhost:8080"
timeout = "30s"

# 此测试中我们不需要真实的 service 连接，但应用会校验 validConfig
# 所有的 services 都可以 disable 以减少副作用(如果允许的话)，或者提供 mockable 的配置
# 这里我们开启 Prometheus 但给一个假的地址，Service 初始化通过即可（只要不发生连接panic）
[services.prometheus]
type = "prometheus"
enabled = true

[services.prometheus.options]
address = "http://localhost:9090"
timeout = "1s"

# Orchestrator (必须开启)
[agents.orchestrator]
enabled = true
[agents.orchestrator.llm]
provider = "openai"
model = "gpt-4"
api_key = "key-orchestrator"

# Service Agent (必须开启至少一个子agent)
[agents.service_agent]
enabled = true
[agents.service_agent.llm]
provider = "openai"
model = "gpt-3.5"
api_key = "key-service-agent"

# Report Agent (另一个子agent)
[agents.report_agent]
enabled = true
[agents.report_agent.llm]
provider = "openai"
model = "gpt-4-turbo"
api_key = "key-report-agent"
`
	configPath := createTempConfig(t, configContent)

	app, err := NewApplication(configPath)
	require.NoError(t, err)

	// 注意：Initialize 会尝试构建 Model 和 Service。
	// 为了确保测试运行环境不需要真实依赖，我们需要确保 LLM Factory 和 Service Factory 不会因为连不上网而 Panic 或 阻塞太久。
	// 这里的测试主要关注 config loadding 到 app.agents 的映射过程。
	// 如果 Initialize 因为网络原因失败，我们可能需要 mock。
	// 但鉴于我们只想测试 a.agents 的正确性，我们可以部分只运行 Initialize 的前几步，
	// 或者，我们只需要断言 Initialize 返回错误（比如连不上 LLM），但 a.agents 已经被正确填充了。
	// 查看 app.go 源码，a.agents 是在 Initialize 的 Step 2 完成后就填充了。
	// 如果后面的 initServices 失败，err 会返回，但 app 实例的状态已经被修改。

	// 尝试初始化
	// 即使因为缺少真实 API Key 导致 build model 失败，agents map 应该已经被填充
	_ = app.Initialize(context.Background())

	// 验证 agents map 是否已填充
	require.NotEmpty(t, app.agents, "Agents map should be populated even if later initialization steps fail")

	// 获取配置
	orchCfg, ok := app.agents["orchestrator"]
	require.True(t, ok)
	svcCfg, ok := app.agents["service_agent"]
	require.True(t, ok)
	reportCfg, ok := app.agents["report_agent"]
	require.True(t, ok)

	// 验证指针地址不同 (证明没有复用同一个循环变量地址)
	assert.True(t, orchCfg != svcCfg, "Agent config pointers should be different")
	assert.True(t, svcCfg != reportCfg, "Agent config pointers should be different")

	// 验证内容不同 (证明值被正确拷贝/读取)
	assert.Equal(t, "key-orchestrator", orchCfg.LLM.APIKey)
	assert.Equal(t, "key-service-agent", svcCfg.LLM.APIKey)
	assert.Equal(t, "key-report-agent", reportCfg.LLM.APIKey)
}

// TestApplication_Initialize_ValidationRules 验证应用启动规则校验
func TestApplication_Initialize_ValidationRules(t *testing.T) {
	baseConfig := `
[server]
addr = "localhost:8080"
[services.prometheus]
type = "prometheus"
enabled = false
`

	t.Run("缺少 orchestrator", func(t *testing.T) {
		configContent := baseConfig + `
[agents.service_agent]
enabled = true
[agents.service_agent.llm]
provider = "openai"
model = "gpt-4"
`
		configPath := createTempConfig(t, configContent)
		app, _ := NewApplication(configPath)
		err := app.Initialize(context.Background())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "orchestrator agent orchestrator is required")
	})

	t.Run("orchestrator 未开启", func(t *testing.T) {
		configContent := baseConfig + `
[agents.orchestrator]
enabled = false
[agents.orchestrator.llm]
provider = "openai"
model = "gpt-4"

[agents.service_agent]
enabled = true
[agents.service_agent.llm]
provider = "openai"
model = "gpt-4"
`
		configPath := createTempConfig(t, configContent)
		app, _ := NewApplication(configPath)
		err := app.Initialize(context.Background())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "orchestrator agent orchestrator is required but not found")
	})

	t.Run("没有开启任何子 agent", func(t *testing.T) {
		configContent := baseConfig + `
[agents.orchestrator]
enabled = true
[agents.orchestrator.llm]
provider = "openai"
model = "gpt-4"

[agents.service_agent]
enabled = false
[agents.service_agent.llm]
provider = "openai"
model = "gpt-4"
`
		configPath := createTempConfig(t, configContent)
		app, _ := NewApplication(configPath)
		err := app.Initialize(context.Background())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "at least one sub agent")
	})
}

// TestApplication_Run_NotInitialized 测试未初始化直接运行
func TestApplication_Run_NotInitialized(t *testing.T) {
	app, _ := NewApplication("dummy.toml")
	_, err := app.Run(context.Background(), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")
}

// TestApplication_Shutdown 测试关闭流程
// 这是一个基础测试，验证 Shutdown 不会 Panic 且能处理 nil 成员
func TestApplication_Shutdown(t *testing.T) {
	app, _ := NewApplication("dummy.toml")
	// 即使没有 Initialize，Shutdown 也应该安全返回（或返回错误但不 Panic）
	// 在当前实现中，未初始化的 registry/models 为 nil，Shutdown 会检查 nil
	err := app.Shutdown(context.Background())
	assert.NoError(t, err)
}
