package config

import (
	"fmt"
	"sync"

	"github.com/BurntSushi/toml"
)

// CollectorOptionsParser Collector 配置解析器接口
// 每个 Collector 类型实现此接口来解析自己的 Options
type CollectorOptionsParser interface {
	// ParseOptions 解析 TOML Primitive 到具体的配置结构
	ParseOptions(meta *toml.MetaData, primitive toml.Primitive) (interface{}, error)
}

// CollectorRegistry 管理 Collector 配置解析器
type CollectorRegistry struct {
	mu      sync.RWMutex
	parsers map[string]CollectorOptionsParser
}

// NewCollectorRegistry 创建并初始化包含默认解析器的 Registry
func NewCollectorRegistry() *CollectorRegistry {
	r := &CollectorRegistry{
		parsers: make(map[string]CollectorOptionsParser),
	}
	r.RegisterParser("prometheus", &PrometheusOptionsParser{})
	r.RegisterParser("pagerduty", &PagerDutyOptionsParser{})
	r.RegisterParser("opensearch", &OpenSearchOptionsParser{})
	return r
}

// RegisterParser 注册解析器
func (r *CollectorRegistry) RegisterParser(collectorType string, parser CollectorOptionsParser) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.parsers[collectorType] = parser
}

// ParseOptions 解析选项
func (r *CollectorRegistry) ParseOptions(collectorType string, meta *toml.MetaData, primitive toml.Primitive) (interface{}, error) {
	r.mu.RLock()
	parser, ok := r.parsers[collectorType]
	r.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("no parser registered for collector type: %s", collectorType)
	}
	return parser.ParseOptions(meta, primitive)
}

// PrometheusOptionsParser Prometheus 配置解析器
type PrometheusOptionsParser struct{}

func (p *PrometheusOptionsParser) ParseOptions(meta *toml.MetaData, primitive toml.Primitive) (interface{}, error) {
	var opts PrometheusOptions
	if err := meta.PrimitiveDecode(primitive, &opts); err != nil {
		return nil, fmt.Errorf("decode prometheus options: %w", err)
	}
	return &opts, nil
}

// PagerDutyOptionsParser PagerDuty 配置解析器
type PagerDutyOptionsParser struct{}

func (p *PagerDutyOptionsParser) ParseOptions(meta *toml.MetaData, primitive toml.Primitive) (interface{}, error) {
	var opts PagerDutyOptions
	if err := meta.PrimitiveDecode(primitive, &opts); err != nil {
		return nil, fmt.Errorf("decode pagerduty options: %w", err)
	}
	return &opts, nil
}

// OpenSearchOptionsParser OpenSearch 配置解析器
type OpenSearchOptionsParser struct{}

func (p *OpenSearchOptionsParser) ParseOptions(meta *toml.MetaData, primitive toml.Primitive) (interface{}, error) {
	var opts OpenSearchOptions
	if err := meta.PrimitiveDecode(primitive, &opts); err != nil {
		return nil, fmt.Errorf("decode opensearch options: %w", err)
	}
	return &opts, nil
}
