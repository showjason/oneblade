package prometheus

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/go-kratos/blades/tools"
	"github.com/oneblade/service"
	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
)

func init() {
	service.RegisterOptionsParser(service.Prometheus, func(meta *toml.MetaData, primitive toml.Primitive) (interface{}, error) {
		return service.ParseOptions[Options](meta, primitive, service.Prometheus)
	})

	service.RegisterService(service.Prometheus, func(meta service.ServiceMeta, opts interface{}) (service.Service, error) {
		promOpts, ok := opts.(*Options)
		if !ok {
			return nil, fmt.Errorf("invalid prometheus options type, got %T", opts)
		}
		return NewService(meta, promOpts)
	})
}

type Options struct {
	Address string        `toml:"address" validate:"required,url"`
	Timeout time.Duration `toml:"timeout"`
}

type Service struct {
	name        string
	description string
	address     string
	timeout     time.Duration
	client      api.Client
	api         v1.API
}

func NewService(meta service.ServiceMeta, opts *Options) (*Service, error) {
	client, err := api.NewClient(api.Config{Address: opts.Address})
	if err != nil {
		return nil, fmt.Errorf("create prometheus client: %w", err)
	}

	return &Service{
		name:        meta.Name,
		description: meta.Description,
		address:     opts.Address,
		timeout:     opts.Timeout,
		client:      client,
		api:         v1.NewAPI(client),
	}, nil
}

func (s *Service) Name() string {
	return s.name
}

func (s *Service) Description() string {
	return s.description
}

func (s *Service) Type() service.ServiceType {
	return service.Prometheus
}

// === Request/Response Structures ===

type Operation string

const (
	QueryRange   Operation = "query_range"
	QueryInstant Operation = "query_instant"
)

type Request struct {
	Operation    Operation           `json:"operation" jsonschema:"The type of operation to perform"`
	QueryRange   *QueryRangeParams   `json:"query_range,omitempty"`
	QueryInstant *QueryInstantParams `json:"query_instant,omitempty"`
}

type Response struct {
	Operation Operation   `json:"operation"`
	Success   bool        `json:"success"`
	Message   string      `json:"message,omitempty"`
	Data      interface{} `json:"data,omitempty"`
	Warnings  []string    `json:"warnings,omitempty"`
}

// === Params ===

type QueryRangeParams struct {
	PromQL    string `json:"promql" jsonschema:"PromQL query expression"`
	StartTime string `json:"start_time" jsonschema:"Start time in RFC3339 format"`
	EndTime   string `json:"end_time" jsonschema:"End time in RFC3339 format"`
	Step      string `json:"step,omitempty" jsonschema:"Query step duration, e.g. 1m or 5m"`
}

type QueryInstantParams struct {
	PromQL string `json:"promql" jsonschema:"PromQL query expression"`
	Time   string `json:"time,omitempty" jsonschema:"Evaluation time in RFC3339 format"`
}

// === Logic ===

func (s *Service) Handle(ctx context.Context, req Request) (Response, error) {
	log.Printf("[prometheus] Handle called with operation: %s", req.Operation)

	timeout := s.timeout
	if timeout == 0 {
		timeout = 60 * time.Second
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	switch req.Operation {
	case QueryRange:
		if req.QueryRange == nil {
			log.Printf("[prometheus] Handle: query_range params is nil, returning error")
			return Response{Success: false, Message: "missing query_range params"}, nil
		}
		return s.queryRange(ctx, req.QueryRange)
	case QueryInstant:
		if req.QueryInstant == nil {
			log.Printf("[prometheus] Handle: query_instant params is nil, returning error")
			return Response{Success: false, Message: "missing query_instant params"}, nil
		}
		return s.queryInstant(ctx, req.QueryInstant)
	default:
		log.Printf("[prometheus] Handle: unknown operation %s", req.Operation)
		return Response{Success: false, Message: fmt.Sprintf("unknown operation: %s", req.Operation)}, nil
	}
}

func (s *Service) AsTool() (tools.Tool, error) {
	return tools.NewFunc(
		"prometheus_service",
		s.Description(),
		s.Handle,
	)
}

func (s *Service) Health(ctx context.Context) error {
	log.Printf("[prometheus] Health check started")

	healthCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	_, err := s.api.Config(healthCtx)
	if err != nil {
		log.Printf("[prometheus] Health check failed: %v", err)
		return fmt.Errorf("prometheus health check failed: %w", err)
	}
	log.Printf("[prometheus] Health check succeeded")
	return nil
}

func (s *Service) Close() error {
	return nil
}

// === Implementations ===

func (s *Service) queryRange(ctx context.Context, params *QueryRangeParams) (Response, error) {
	log.Printf("[prometheus] queryRange called with promql=%s, start_time=%s, end_time=%s, step=%s", params.PromQL, params.StartTime, params.EndTime, params.Step)

	start, err := time.Parse(time.RFC3339, params.StartTime)
	if err != nil {
		log.Printf("[prometheus] queryRange failed to parse start_time: %v", err)
		return Response{Success: false, Message: fmt.Sprintf("parse start_time: %v", err)}, nil
	}

	end, err := time.Parse(time.RFC3339, params.EndTime)
	if err != nil {
		log.Printf("[prometheus] queryRange failed to parse end_time: %v", err)
		return Response{Success: false, Message: fmt.Sprintf("parse end_time: %v", err)}, nil
	}

	step := time.Minute
	if params.Step != "" {
		step, err = time.ParseDuration(params.Step)
		if err != nil {
			log.Printf("[prometheus] queryRange failed to parse step: %v", err)
			return Response{Success: false, Message: fmt.Sprintf("parse step: %v", err)}, nil
		}
	}

	result, warnings, err := s.api.QueryRange(ctx, params.PromQL, v1.Range{
		Start: start,
		End:   end,
		Step:  step,
	})
	if err != nil {
		log.Printf("[prometheus] queryRange failed: %v", err)
		return Response{Success: false, Message: err.Error()}, nil
	}
	log.Printf("[prometheus] queryRange succeeded with %d warnings", len(warnings))

	return Response{
		Operation: QueryRange,
		Success:   true,
		Data:      result,
		Warnings:  warnings,
	}, nil
}

func (s *Service) queryInstant(ctx context.Context, params *QueryInstantParams) (Response, error) {
	log.Printf("[prometheus] queryInstant called with promql=%s, time=%s", params.PromQL, params.Time)

	ts := time.Now()
	if params.Time != "" {
		var err error
		ts, err = time.Parse(time.RFC3339, params.Time)
		if err != nil {
			log.Printf("[prometheus] queryInstant failed to parse time: %v", err)
			return Response{Success: false, Message: fmt.Sprintf("parse time: %v", err)}, nil
		}
	}

	result, warnings, err := s.api.Query(ctx, params.PromQL, ts)
	if err != nil {
		log.Printf("[prometheus] queryInstant failed: %v", err)
		return Response{Success: false, Message: err.Error()}, nil
	}
	log.Printf("[prometheus] queryInstant succeeded with %d warnings", len(warnings))

	return Response{
		Operation: QueryInstant,
		Success:   true,
		Data:      result,
		Warnings:  warnings,
	}, nil
}
