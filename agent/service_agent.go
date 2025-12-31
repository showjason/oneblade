package agent

import (
	"github.com/go-kratos/blades"
	"github.com/go-kratos/blades/tools"

	"github.com/oneblade/service"
)

// ServiceAgentConfig Service Agent 配置
type ServiceAgentConfig struct {
	Model    blades.ModelProvider
	Services []service.Service
}

// NewServiceAgent 创建统一服务 Tool Agent
func NewServiceAgent(cfg ServiceAgentConfig) (blades.Agent, error) {
	// 创建各个 Service Tool
	var serviceTools []tools.Tool
	for _, s := range cfg.Services {
		tool, err := s.AsTool()
		if err != nil {
			return nil, err
		}
		serviceTools = append(serviceTools, tool)
	}

	return blades.NewAgent(
		"service_agent",
		blades.WithDescription("负责与 Prometheus、PagerDuty、OpenSearch 等服务交互的 Agent，提供数据采集和操作能力"),
		blades.WithInstruction(`你是一个 SRE 服务交互专家。

你拥有以下服务的操作工具:
1. **prometheus_service** - Prometheus 监控数据查询 (支持 query_range/query_instant)
2. **pagerduty_service** - PagerDuty 告警管理 (支持 list/get/acknowledge/resolve/snooze)
3. **opensearch_service** - OpenSearch 日志查询 (支持 DSL search)

你的职责:
1. 根据用户意图，确定需要操作的服务和具体操作类型
2. 构建正确的请求参数
3. 调用工具并解析结果

**使用示例:**

*   **Prometheus**:
    *   查询CPU: {"operation": "query_range", "query_range": {"promql": "rate(node_cpu_seconds_total[5m])", ...}}
*   **PagerDuty**:
    *   列出告警: {"operation": "list_incidents", "list_incidents": {"limit": 10}}
    *   解决告警: {"operation": "resolve_incident", "resolve_incident": {"incident_id": "P12345"}}
	*   确认告警: {"operation": "acknowledge_incident", "acknowledge_incident": {"incident_id": "P12345"}}
	*   抑制告警: {"operation": "snooze_alert", "snooze_alert": {"incident_id": "P12345", "duration": 3600}}
	*   获取告警详情: {"operation": "get_incident", "get_incident": {"incident_id": "P12345"}}
*   **OpenSearch**:
    *   查询错误: {"operation": "search", "search": {"body": {"query": {"match": {"level": "ERROR"}}}}}

请根据需求灵活组合使用这些工具。`),
		blades.WithModel(cfg.Model),
		blades.WithTools(serviceTools...),
	)
}
