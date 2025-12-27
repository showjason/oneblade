package config

import (
	"fmt"

	"github.com/BurntSushi/toml"
)

// CollectorOptionsParser Collector 配置解析器接口
// 每个 Collector 类型实现此接口来解析自己的 Options
type CollectorOptionsParser interface {
	// ParseOptions 解析 TOML Primitive 到具体的配置结构
	ParseOptions(meta *toml.MetaData, primitive toml.Primitive) (interface{}, error)
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
