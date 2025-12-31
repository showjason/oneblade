package collector

import (
	"context"
	"fmt"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/PagerDuty/go-pagerduty"
	"github.com/go-kratos/blades/tools"
)

func init() {
	// 注册解析器（使用闭包调用泛型函数）
	RegisterOptionsParser(CollectorPagerDuty, func(meta *toml.MetaData, primitive toml.Primitive) (interface{}, error) {
		return ParseOptions[PagerDutyOptions](meta, primitive, CollectorPagerDuty)
	})

	// 注册 collector 工厂
	RegisterCollector(CollectorPagerDuty, func(opts interface{}) (Collector, error) {
		pdOpts, ok := opts.(*PagerDutyOptions)
		if !ok {
			return nil, fmt.Errorf("invalid pagerduty options type, got %T", opts)
		}
		return NewPagerDutyCollectorFromOptions(pdOpts), nil
	})
}

// PagerDutyOptions PagerDuty 采集器选项
type PagerDutyOptions struct {
	APIKey string `toml:"api_key" validate:"required"`
}

// PagerDutyCollector PagerDuty 采集器
type PagerDutyCollector struct {
	apiKey string
	client *pagerduty.Client
}

// NewPagerDutyCollectorFromOptions 从配置选项创建 PagerDuty 采集器
func NewPagerDutyCollectorFromOptions(opts *PagerDutyOptions) *PagerDutyCollector {
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
	ResourceType string   `json:"resource_type" jsonschema:"Resource to fetch: incidents or alerts"`
	Since        string   `json:"since" jsonschema:"Start time in RFC3339 format"`
	Until        string   `json:"until" jsonschema:"End time in RFC3339 format"`
	ServiceIDs   []string `json:"service_ids,omitempty" jsonschema:"Filter by service IDs"`
	Statuses     []string `json:"statuses,omitempty" jsonschema:"Filter by statuses: triggered, acknowledged, resolved"`
	Limit        int      `json:"limit,omitempty" jsonschema:"Maximum number of results"`
}

// PagerDutyIncident simplified incident structure to avoid cycle
type PagerDutyIncident struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Status      string `json:"status"`
	Urgency     string `json:"urgency"`
	ServiceName string `json:"service_name"`
	CreatedAt   string `json:"created_at"`
	HTMLURL     string `json:"html_url"`
}

// PagerDutyQueryOutput PagerDuty 查询结果
type PagerDutyQueryOutput struct {
	Incidents []PagerDutyIncident `json:"incidents"`
	Total     int                 `json:"total"`
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

	// Convert to simplified incident type to avoid cycle
	incidents := make([]PagerDutyIncident, len(resp.Incidents))
	for i, inc := range resp.Incidents {
		incidents[i] = PagerDutyIncident{
			ID:          inc.ID,
			Title:       inc.Title,
			Status:      inc.Status,
			Urgency:     inc.Urgency,
			ServiceName: inc.Service.Summary,
			CreatedAt:   inc.CreatedAt,
			HTMLURL:     inc.HTMLURL,
		}
	}

	return PagerDutyQueryOutput{
		Incidents: incidents,
		Total:     len(incidents),
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
	healthCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	_, err := c.client.ListAbilitiesWithContext(healthCtx)
	if err != nil {
		return fmt.Errorf("pagerduty health check failed: %w", err)
	}
	return nil
}

func (c *PagerDutyCollector) Close() error {
	return nil
}
