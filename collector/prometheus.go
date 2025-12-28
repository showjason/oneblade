package collector

import (
	"context"
	"fmt"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/go-kratos/blades/tools"
	"github.com/oneblade/utils"
	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
)

// PrometheusOptions Prometheus 采集器选项
type PrometheusOptions struct {
	Address string        `toml:"address" validate:"required,url"`
	Timeout utils.Duration `toml:"timeout"`
}

// PrometheusCollector Prometheus 采集器
type PrometheusCollector struct {
	address string
	timeout time.Duration
	client  api.Client
	api     v1.API
}

// NewPrometheusCollectorFromOptions 从配置选项创建 Prometheus 采集器
func NewPrometheusCollectorFromOptions(opts *PrometheusOptions) (*PrometheusCollector, error) {
	client, err := api.NewClient(api.Config{Address: opts.Address})
	if err != nil {
		return nil, fmt.Errorf("create prometheus client: %w", err)
	}

	return &PrometheusCollector{
		address: opts.Address,
		timeout: opts.Timeout.Duration,
		client:  client,
		api:     v1.NewAPI(client),
	}, nil
}

func (c *PrometheusCollector) Name() CollectorType {
	return CollectorPrometheus
}

func (c *PrometheusCollector) Description() string {
	return "Query metrics from Prometheus using PromQL. Returns time-series data."
}

// PrometheusQueryInput Prometheus 查询参数（给 LLM 使用）
type PrometheusQueryInput struct {
	PromQL    string `json:"promql" jsonschema:"description=PromQL query expression"`
	StartTime string `json:"start_time" jsonschema:"description=Start time in RFC3339 format"`
	EndTime   string `json:"end_time" jsonschema:"description=End time in RFC3339 format"`
	Step      string `json:"step,omitempty" jsonschema:"description=Query step duration (e.g. 1m, 5m)"`
}

// PrometheusQueryOutput Prometheus 查询结果
type PrometheusQueryOutput struct {
	Data     interface{} `json:"data"`
	Warnings []string    `json:"warnings,omitempty"`
}

// Handle 处理 Prometheus 查询请求
func (c *PrometheusCollector) Handle(ctx context.Context, input PrometheusQueryInput) (PrometheusQueryOutput, error) {
	start, err := time.Parse(time.RFC3339, input.StartTime)
	if err != nil {
		return PrometheusQueryOutput{}, fmt.Errorf("parse start_time: %w", err)
	}

	end, err := time.Parse(time.RFC3339, input.EndTime)
	if err != nil {
		return PrometheusQueryOutput{}, fmt.Errorf("parse end_time: %w", err)
	}

	step := time.Minute
	if input.Step != "" {
		step, err = time.ParseDuration(input.Step)
		if err != nil {
			return PrometheusQueryOutput{}, fmt.Errorf("parse step: %w", err)
		}
	}

	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	result, warnings, err := c.api.QueryRange(ctx, input.PromQL, v1.Range{
		Start: start,
		End:   end,
		Step:  step,
	})
	if err != nil {
		return PrometheusQueryOutput{}, fmt.Errorf("prometheus query: %w", err)
	}

	return PrometheusQueryOutput{
		Data:     result,
		Warnings: warnings,
	}, nil
}

func (c *PrometheusCollector) AsTool() (tools.Tool, error) {
	return tools.NewFunc(
		string(c.Name()),
		c.Description(),
		c.Handle,
	)
}

func (c *PrometheusCollector) Health(ctx context.Context) error {
	_, err := c.api.Config(ctx)
	return err
}

func (c *PrometheusCollector) Close() error {
	return nil
}

func init() {
	// 注册解析器（使用闭包调用泛型函数）
	RegisterOptionsParser(CollectorPrometheus, func(meta *toml.MetaData, primitive toml.Primitive) (interface{}, error) {
		return ParseOptions[PrometheusOptions](meta, primitive, "prometheus")
	})

	// 注册 collector 工厂
	RegisterCollector(CollectorPrometheus, func(opts interface{}) (Collector, error) {
		promOpts, ok := opts.(*PrometheusOptions)
		if !ok {
			return nil, fmt.Errorf("invalid prometheus options type, got %T", opts)
		}
		return NewPrometheusCollectorFromOptions(promOpts)
	})
}
