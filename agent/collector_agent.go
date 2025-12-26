package agent

import (
	"github.com/go-kratos/blades"
	"github.com/go-kratos/blades/tools"

	"github.com/oneblade/collector"
)

// DataCollectionAgentConfig 数据采集 Agent 配置
type DataCollectionAgentConfig struct {
	Model      blades.ModelProvider
	Collectors []collector.Collector
}

// NewDataCollectionAgent 创建统一数据采集 Agent
func NewDataCollectionAgent(acfg DataCollectionAgentConfig) (blades.Agent, error) {
	// 创建各个 Collector Tool
	var tools []tools.Tool
	for _, c := range acfg.Collectors {
		tool, err := c.AsTool()
		if err != nil {
			return nil, err
		}
		tools = append(tools, tool)
	}

	return blades.NewAgent(
		"data_collection_agent",
		blades.WithDescription("负责从 Prometheus、PagerDuty、OpenSearch 采集和分析数据的 Agent"),
		blades.WithInstruction(`你是一个 SRE 数据采集与分析专家。

你拥有三个数据采集工具:
1. **query_prometheus** - 查询 Prometheus 指标数据
2. **query_pagerduty** - 查询 PagerDuty 告警信息
3. **query_opensearch** - 查询 OpenSearch 日志数据

你的职责:
1. 根据用户需求确定需要查询哪些数据源
2. 构建正确的查询语句 (PromQL/PagerDuty API/OpenSearch DSL)
3. 并行或顺序调用需要的采集工具
4. 分析和汇总采集到的数据

**Prometheus 常用查询:**
- CPU 使用率: rate(node_cpu_seconds_total{mode!='idle'}[5m])
- 内存使用率: 1 - (node_memory_MemAvailable_bytes / node_memory_MemTotal_bytes)
- 磁盘使用率: 1 - (node_filesystem_avail_bytes / node_filesystem_size_bytes)
- 网络流量: rate(node_network_receive_bytes_total[5m])

**PagerDuty 查询注意:**
- 按服务过滤告警
- 按严重程度分类 (Critical, Warning, Info)
- 统计告警响应时间和解决时间

**OpenSearch 常用查询:**
- 错误日志: level:ERROR OR level:error
- 特定服务: service:api-gateway AND level:ERROR

根据任务需求智能选择和组合工具，提供全面的数据分析结果。`),
		blades.WithModel(acfg.Model),
		blades.WithTools(tools...),
	)
}
