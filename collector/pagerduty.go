package collector

import (
	"context"
	"fmt"

	"github.com/PagerDuty/go-pagerduty"
	"github.com/go-kratos/blades/tools"
	"github.com/oneblade/config"
)

// PagerDutyCollector PagerDuty 采集器
type PagerDutyCollector struct {
	apiKey string
	client *pagerduty.Client
}

// NewPagerDutyCollectorFromOptions 从配置选项创建 PagerDuty 采集器
func NewPagerDutyCollectorFromOptions(opts *config.PagerDutyOptions) *PagerDutyCollector {
	return &PagerDutyCollector{
		apiKey: opts.APIKey,
		client: pagerduty.NewClient(opts.APIKey),
	}
}

func (c *PagerDutyCollector) Name() CollectorType {
	return CollectorPagerDuty
}

func (c *PagerDutyCollector) Description() string {
	return "Fetch incidents or alerts from PagerDuty. Use to check active alerts and incident history."
}

// PagerDutyQueryInput PagerDuty 查询参数
type PagerDutyQueryInput struct {
	ResourceType string   `json:"resource_type" jsonschema:"description=Resource to fetch: incidents or alerts,enum=incidents,enum=alerts"`
	Since        string   `json:"since" jsonschema:"description=Start time in RFC3339 format"`
	Until        string   `json:"until" jsonschema:"description=End time in RFC3339 format"`
	ServiceIDs   []string `json:"service_ids,omitempty" jsonschema:"description=Filter by service IDs"`
	Statuses     []string `json:"statuses,omitempty" jsonschema:"description=Filter by statuses: triggered, acknowledged, resolved"`
	Limit        int      `json:"limit,omitempty" jsonschema:"description=Maximum number of results"`
}

// PagerDutyQueryOutput PagerDuty 查询结果
type PagerDutyQueryOutput struct {
	Incidents []pagerduty.Incident `json:"incidents"`
	Total     int                  `json:"total"`
}

// Handle 处理 PagerDuty 查询请求
func (c *PagerDutyCollector) Handle(ctx context.Context, input PagerDutyQueryInput) (PagerDutyQueryOutput, error) {
	limit := input.Limit
	if limit == 0 {
		limit = 50
	}

	opts := pagerduty.ListIncidentsOptions{
		Since: input.Since,
		Until: input.Until,
		Limit: uint(limit),
	}

	if len(input.ServiceIDs) > 0 {
		opts.ServiceIDs = input.ServiceIDs
	}
	if len(input.Statuses) > 0 {
		opts.Statuses = input.Statuses
	}

	resp, err := c.client.ListIncidentsWithContext(ctx, opts)
	if err != nil {
		return PagerDutyQueryOutput{}, fmt.Errorf("pagerduty api: %w", err)
	}

	return PagerDutyQueryOutput{
		Incidents: resp.Incidents,
		Total:     len(resp.Incidents),
	}, nil
}

func (c *PagerDutyCollector) AsTool() (tools.Tool, error) {
	return tools.NewFunc(
		string(c.Name()),
		c.Description(),
		c.Handle,
	)
}

func (c *PagerDutyCollector) Health(ctx context.Context) error {
	_, err := c.client.ListAbilitiesWithContext(ctx)
	return err
}

func (c *PagerDutyCollector) Close() error {
	return nil
}

func init() {
	RegisterCollector(CollectorPagerDuty, func(opts interface{}) (Collector, error) {
		pdOpts, ok := opts.(*config.PagerDutyOptions)
		if !ok {
			panic(fmt.Errorf("invalid pagerduty options type, got %T", opts))
		}
		return NewPagerDutyCollectorFromOptions(pdOpts), nil
	})
}
