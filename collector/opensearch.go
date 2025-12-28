package collector

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/go-kratos/blades/tools"
	"github.com/opensearch-project/opensearch-go/v2"
	"github.com/opensearch-project/opensearch-go/v2/opensearchapi"
)

// OpenSearchOptions OpenSearch 采集器选项
type OpenSearchOptions struct {
	Addresses []string `toml:"addresses" validate:"required,min=1,dive,url"`
	Username  string   `toml:"username"`
	Password  string   `toml:"password"`
	Index     string   `toml:"index" validate:"required"`
}

// OpenSearchCollector OpenSearch 采集器
type OpenSearchCollector struct {
	addresses []string
	username  string
	password  string
	index     string
	client    *opensearch.Client
}

// NewOpenSearchCollectorFromOptions 从配置选项创建 OpenSearch 采集器
func NewOpenSearchCollectorFromOptions(opts *OpenSearchOptions) (*OpenSearchCollector, error) {
	client, err := opensearch.NewClient(opensearch.Config{
		Addresses: opts.Addresses,
		Username:  opts.Username,
		Password:  opts.Password,
	})
	if err != nil {
		return nil, fmt.Errorf("create opensearch client: %w", err)
	}

	return &OpenSearchCollector{
		addresses: opts.Addresses,
		username:  opts.Username,
		password:  opts.Password,
		index:     opts.Index,
		client:    client,
	}, nil
}

func (c *OpenSearchCollector) Name() CollectorType {
	return CollectorOpenSearch
}

func (c *OpenSearchCollector) Description() string {
	return "Execute a raw OpenSearch/Elasticsearch DSL query. The 'body' parameter must contain the full JSON query object (including 'query', 'size', 'aggs' etc)."
}

// OpenSearchQueryInput OpenSearch 查询参数
type OpenSearchQueryInput struct {
	Index string          `json:"index,omitempty" jsonschema:"description=Index pattern to search (default: configured index)"`
	Body  json.RawMessage `json:"body" jsonschema:"description=OpenSearch DSL query body (JSON)"`
}

// OpenSearchQueryOutput OpenSearch 查询结果
type OpenSearchQueryOutput map[string]interface{}

// Handle 处理 OpenSearch 查询请求
func (c *OpenSearchCollector) Handle(ctx context.Context, input OpenSearchQueryInput) (OpenSearchQueryOutput, error) {
	index := input.Index
	if index == "" {
		index = c.index
	}

	if len(input.Body) == 0 {
		return nil, fmt.Errorf("body is required for opensearch query")
	}

	req := opensearchapi.SearchRequest{
		Index: []string{index},
		Body:  strings.NewReader(string(input.Body)),
	}

	res, err := req.Do(ctx, c.client)
	if err != nil {
		return nil, fmt.Errorf("opensearch search: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return nil, fmt.Errorf("opensearch error: %s", res.String())
	}

	var result map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return result, nil
}

func (c *OpenSearchCollector) AsTool() (tools.Tool, error) {
	return tools.NewFunc(
		string(c.Name()),
		c.Description(),
		c.Handle,
	)
}

func (c *OpenSearchCollector) Health(ctx context.Context) error {
	res, err := c.client.Ping()
	if err != nil {
		return err
	}
	defer res.Body.Close()
	return nil
}

func (c *OpenSearchCollector) Close() error {
	return nil
}

func init() {
	// 注册解析器（使用闭包调用泛型函数）
	RegisterOptionsParser(CollectorOpenSearch, func(meta *toml.MetaData, primitive toml.Primitive) (interface{}, error) {
		return ParseOptions[OpenSearchOptions](meta, primitive, "opensearch")
	})

	// 注册 collector 工厂
	RegisterCollector(CollectorOpenSearch, func(opts interface{}) (Collector, error) {
		osOpts, ok := opts.(*OpenSearchOptions)
		if !ok {
			return nil, fmt.Errorf("invalid opensearch options type, got %T", opts)
		}
		return NewOpenSearchCollectorFromOptions(osOpts)
	})
}
