package service

import (
	"context"

	"github.com/go-kratos/blades/tools"
)

type ServiceType string

const (
	PagerDuty  ServiceType = "pagerduty"
	Prometheus ServiceType = "prometheus"
	OpenSearch ServiceType = "opensearch"
)

type Service interface {
	Name() ServiceType
	Description() string
	AsTool() (tools.Tool, error)
	Health(ctx context.Context) error
	Close() error
}
