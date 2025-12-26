package collector

import (
	"context"

	"github.com/go-kratos/blades/tools"
)

type CollectorType string

const (
	CollectorPrometheus CollectorType = "prometheus"
	CollectorPagerDuty  CollectorType = "pagerduty"
	CollectorOpenSearch CollectorType = "opensearch"
)

// Collector 采集器接口
// - 不同数据源有不同的采集方式（Query/API），所以不定义统一的 Query 方法
// - 每个 Collector 自己实现 Handle 方法，参数类型可不同
// - 通过 tools.NewFunc 转换为 Tool 供 Agent 使用
type Collector interface {
	// Name 返回采集器名称（如 "prometheus", "pagerduty", "opensearch"）
	Name() CollectorType

	// Description 返回采集器描述，供 LLM 理解如何使用
	Description() string

	// tool 将 Collector 转换为 Tool
	AsTool() (tools.Tool, error)

	// Health 健康检查
	Health(ctx context.Context) error

	// Close 关闭连接
	Close() error
}
