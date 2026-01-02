package opensearch

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/go-kratos/blades/tools"
	"github.com/oneblade/service"
	"github.com/opensearch-project/opensearch-go/v2"
	"github.com/opensearch-project/opensearch-go/v2/opensearchapi"
)

func init() {
	service.RegisterOptionsParser(service.OpenSearch, func(meta *toml.MetaData, primitive toml.Primitive) (interface{}, error) {
		return service.ParseOptions[Options](meta, primitive, service.OpenSearch)
	})

	service.RegisterService(service.OpenSearch, func(meta service.ServiceMeta, opts interface{}) (service.Service, error) {
		osOpts, ok := opts.(*Options)
		if !ok {
			return nil, fmt.Errorf("invalid opensearch options type, got %T", opts)
		}
		return NewService(meta, osOpts)
	})
}

type Options struct {
	Addresses []string `toml:"addresses" validate:"required,min=1,dive,url"`
	Username  string   `toml:"username"`
	Password  string   `toml:"password"`
	Index     string   `toml:"index" validate:"required"`
}

type Service struct {
	name        string
	description string
	addresses   []string
	username    string
	password    string
	index       string
	client      *opensearch.Client
}

func NewService(meta service.ServiceMeta, opts *Options) (*Service, error) {
	client, err := opensearch.NewClient(opensearch.Config{
		Addresses: opts.Addresses,
		Username:  opts.Username,
		Password:  opts.Password,
	})
	if err != nil {
		return nil, fmt.Errorf("create opensearch client: %w", err)
	}

	return &Service{
		name:        meta.Name,
		description: meta.Description,
		addresses:   opts.Addresses,
		username:    opts.Username,
		password:    opts.Password,
		index:       opts.Index,
		client:      client,
	}, nil
}

func (s *Service) Name() string {
	return s.name
}

func (s *Service) Description() string {
	return s.description
}

func (s *Service) Type() service.ServiceType {
	return service.OpenSearch
}

// === Request/Response Structures ===

type Operation string

const (
	Search Operation = "search"
)

type Request struct {
	Operation Operation     `json:"operation" jsonschema:"The type of operation to perform"`
	Search    *SearchParams `json:"search,omitempty"`
}

type Response struct {
	Operation Operation              `json:"operation"`
	Success   bool                   `json:"success"`
	Message   string                 `json:"message,omitempty"`
	Data      map[string]interface{} `json:"data,omitempty"`
}

// === Params ===

type SearchParams struct {
	Index string          `json:"index,omitempty" jsonschema:"Index pattern to search, defaults to configured index"`
	Body  json.RawMessage `json:"body" jsonschema:"OpenSearch DSL query body in JSON format, required"`
}

// === Logic ===

func (s *Service) Handle(ctx context.Context, req Request) (Response, error) {
	switch req.Operation {
	case Search:
		if req.Search == nil {
			return Response{Success: false, Message: "missing search params"}, nil
		}
		return s.search(ctx, req.Search)
	default:
		return Response{Success: false, Message: fmt.Sprintf("unknown operation: %s", req.Operation)}, nil
	}
}

func (s *Service) AsTool() (tools.Tool, error) {
	return tools.NewFunc(
		"opensearch_service",
		s.Description(),
		s.Handle,
	)
}

func (s *Service) Health(ctx context.Context) error {
	healthCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	req := opensearchapi.PingRequest{}
	res, err := req.Do(healthCtx, s.client)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.IsError() {
		return fmt.Errorf("opensearch health check failed: %s", res.Status())
	}
	return nil
}

func (s *Service) Close() error {
	return nil
}

// === Implementations ===

func (s *Service) search(ctx context.Context, params *SearchParams) (Response, error) {
	index := params.Index
	if index == "" {
		index = s.index
	}

	if len(params.Body) == 0 {
		return Response{Success: false, Message: "body is required for opensearch query"}, nil
	}

	req := opensearchapi.SearchRequest{
		Index: []string{index},
		Body:  strings.NewReader(string(params.Body)),
	}

	res, err := req.Do(ctx, s.client)
	if err != nil {
		return Response{Success: false, Message: fmt.Sprintf("opensearch search: %v", err)}, nil
	}
	defer res.Body.Close()

	if res.IsError() {
		return Response{Success: false, Message: fmt.Sprintf("opensearch error: %s", res.String())}, nil
	}

	var result map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		return Response{Success: false, Message: fmt.Sprintf("decode response: %v", err)}, nil
	}

	return Response{
		Operation: Search,
		Success:   true,
		Data:      result,
	}, nil
}
