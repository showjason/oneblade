package pagerduty

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/PagerDuty/go-pagerduty"
	"github.com/go-kratos/blades/tools"
	"github.com/oneblade/service"
)

func init() {
	service.RegisterOptionsParser(service.PagerDuty, func(meta *toml.MetaData, primitive toml.Primitive) (interface{}, error) {
		return service.ParseOptions[Options](meta, primitive, service.PagerDuty)
	})

	service.RegisterService(service.PagerDuty, func(meta service.ServiceMeta, opts interface{}) (service.Service, error) {
		pdOpts, ok := opts.(*Options)
		if !ok {
			return nil, fmt.Errorf("invalid pagerduty options type, got %T", opts)
		}
		return NewService(meta, pdOpts), nil
	})
}

type Options struct {
	APIKey string `toml:"api_key" validate:"required"`
	From   string `toml:"from"`
}

type Service struct {
	name        string
	description string
	opts        *Options
	client      *pagerduty.Client
}

func NewService(meta service.ServiceMeta, opts *Options) *Service {
	// From 字段用于 PagerDuty API 的 From header，标识执行操作的用户 email
	// 如果未配置，留空字符串，PagerDuty 可能会使用 API key 关联的用户
	return &Service{
		name:        meta.Name,
		description: meta.Description,
		opts:        opts,
		client:      pagerduty.NewClient(opts.APIKey),
	}
}

func (s *Service) Name() string {
	return s.name
}

func (s *Service) Description() string {
	return s.description
}

func (s *Service) Type() service.ServiceType {
	return service.PagerDuty
}

// === Request/Response Structures ===

type Operation string

const (
	ListIncidents       Operation = "list_incidents"
	SnoozeAlert         Operation = "snooze_alert"
	AcknowledgeIncident Operation = "acknowledge_incident"
	ResolveIncident     Operation = "resolve_incident"
	GetIncident         Operation = "get_incident"
)

type Request struct {
	Operation           Operation                  `json:"operation" jsonschema:"The type of operation to perform"`
	ListIncidents       *ListIncidentsParams       `json:"list_incidents,omitempty"`
	SnoozeAlert         *SnoozeAlertParams         `json:"snooze_alert,omitempty"`
	AcknowledgeIncident *AcknowledgeIncidentParams `json:"acknowledge_incident,omitempty"`
	ResolveIncident     *ResolveIncidentParams     `json:"resolve_incident,omitempty"`
	GetIncident         *GetIncidentParams         `json:"get_incident,omitempty"`
}

type Response struct {
	Operation   Operation  `json:"operation"`
	Success     bool       `json:"success"`
	Message     string     `json:"message,omitempty"`
	Incidents   []Incident `json:"incidents,omitempty"`
	Total       int        `json:"total,omitempty"`
	Incident    *Incident  `json:"incident,omitempty"`
	SnoozeUntil *time.Time `json:"snooze_until,omitempty"`
}

// === Params ===

type ListIncidentsParams struct {
	Since        string   `json:"since,omitempty" jsonschema:"Start time in RFC3339 format, defaults to 24 hours ago if not provided"`
	Until        string   `json:"until,omitempty" jsonschema:"End time in RFC3339 format, defaults to now if not provided"`
	ServiceIDs   []string `json:"service_ids,omitempty" jsonschema:"Filter by service IDs"`
	ServiceNames []string `json:"service_names,omitempty" jsonschema:"Filter by service names (will be converted to service IDs)"`
	Statuses     []string `json:"statuses,omitempty" jsonschema:"Filter by statuses: triggered, acknowledged, resolved"`
	Limit        int      `json:"limit,omitempty"`
}

type SnoozeAlertParams struct {
	IncidentID string `json:"incident_id" jsonschema:"required"`
	Duration   int    `json:"duration" jsonschema:"required,Snooze duration in minutes"`
}

type AcknowledgeIncidentParams struct {
	IncidentID string `json:"incident_id" jsonschema:"required"`
}

type ResolveIncidentParams struct {
	IncidentID string `json:"incident_id" jsonschema:"required"`
}

type GetIncidentParams struct {
	IncidentID string `json:"incident_id" jsonschema:"required"`
}

// Incident simplified structure
type Incident struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Status      string `json:"status"`
	Urgency     string `json:"urgency"`
	ServiceName string `json:"service_name"`
	ServiceID   string `json:"service_id"`
	CreatedAt   string `json:"created_at"`
	HTMLURL     string `json:"html_url"`
}

// === Logic ===

func (s *Service) Handle(ctx context.Context, req Request) (Response, error) {
	log.Printf("[pagerduty] Handle called with operation: %s", req.Operation)

	switch req.Operation {
	case ListIncidents:
		if req.ListIncidents == nil {
			log.Printf("[pagerduty] Handle: list_incidents params is nil, returning error")
			return Response{Success: false, Message: "missing list_incidents params"}, nil
		}
		return s.listIncidents(ctx, req.ListIncidents)
	case SnoozeAlert:
		if req.SnoozeAlert == nil {
			log.Printf("[pagerduty] Handle: snooze_alert params is nil, returning error")
			return Response{Success: false, Message: "missing snooze_alert params"}, nil
		}
		log.Printf("[pagerduty] Handle: snooze_alert params present, calling snoozeAlert")
		return s.snoozeAlert(ctx, req.SnoozeAlert)
	case AcknowledgeIncident:
		if req.AcknowledgeIncident == nil {
			log.Printf("[pagerduty] Handle: acknowledge_incident params is nil, returning error")
			return Response{Success: false, Message: "missing acknowledge_incident params"}, nil
		}
		log.Printf("[pagerduty] Handle: acknowledge_incident params present, calling acknowledgeIncident")
		return s.acknowledgeIncident(ctx, req.AcknowledgeIncident)
	case ResolveIncident:
		if req.ResolveIncident == nil {
			log.Printf("[pagerduty] Handle: resolve_incident params is nil, returning error")
			return Response{Success: false, Message: "missing resolve_incident params"}, nil
		}
		log.Printf("[pagerduty] Handle: resolve_incident params present, calling resolveIncident")
		return s.resolveIncident(ctx, req.ResolveIncident)
	case GetIncident:
		if req.GetIncident == nil {
			log.Printf("[pagerduty] Handle: get_incident params is nil, returning error")
			return Response{Success: false, Message: "missing get_incident params"}, nil
		}
		log.Printf("[pagerduty] Handle: get_incident params present, calling getIncident")
		return s.getIncident(ctx, req.GetIncident)
	default:
		return Response{Success: false, Message: fmt.Sprintf("unknown operation: %s", req.Operation)}, nil
	}
}

func (s *Service) AsTool() (tools.Tool, error) {
	return tools.NewFunc(
		"pagerduty_service",
		s.Description(),
		s.Handle,
	)
}

func (s *Service) Health(ctx context.Context) error {
	healthCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	_, err := s.client.ListAbilitiesWithContext(healthCtx)
	if err != nil {
		return fmt.Errorf("pagerduty health check failed: %w", err)
	}
	return nil
}

func (s *Service) Close() error {
	return nil
}

// === Implementations ===

func (s *Service) listIncidents(ctx context.Context, params *ListIncidentsParams) (Response, error) {
	log.Printf("[pagerduty] listIncidents called with params: limit=%d, statuses=%v, service_ids=%v", params.Limit, params.Statuses, params.ServiceIDs)

	limit := params.Limit
	if limit == 0 {
		limit = 50
	}

	// 如果 since 和 until 未提供，使用默认值（过去24小时）
	since := params.Since
	until := params.Until
	if since == "" || until == "" {
		now := time.Now()
		if until == "" {
			until = now.Format(time.RFC3339)
		}
		if since == "" {
			since = now.Add(-24 * time.Hour).Format(time.RFC3339)
		}
	}

	// 如果提供了 ServiceNames，先转换为 ServiceIDs
	serviceIDs := params.ServiceIDs
	if len(params.ServiceNames) > 0 {
		nameToIDs, err := s.resolveServiceNamesToIDs(ctx, params.ServiceNames)
		if err != nil {
			return Response{Success: false, Message: fmt.Sprintf("failed to resolve service names: %v", err)}, nil
		}
		// 合并 ServiceIDs 和从 ServiceNames 解析出的 IDs（去重）
		serviceIDMap := make(map[string]bool)
		for _, id := range serviceIDs {
			serviceIDMap[id] = true
		}
		for _, id := range nameToIDs {
			serviceIDMap[id] = true
		}
		serviceIDs = make([]string, 0, len(serviceIDMap))
		for id := range serviceIDMap {
			serviceIDs = append(serviceIDs, id)
		}
	}

	opts := pagerduty.ListIncidentsOptions{
		Since: since,
		Until: until,
		Limit: uint(limit),
	}
	if len(serviceIDs) > 0 {
		opts.ServiceIDs = serviceIDs
	}
	if len(params.Statuses) > 0 {
		opts.Statuses = params.Statuses
	}

	resp, err := s.client.ListIncidentsWithContext(ctx, opts)
	if err != nil {
		log.Printf("[pagerduty] listIncidents failed: %v", err)
		return Response{Success: false, Message: err.Error()}, nil
	}
	log.Printf("[pagerduty] listIncidents succeeded, found %d incidents", len(resp.Incidents))

	if len(resp.Incidents) == 0 {
		return Response{
			Operation: ListIncidents,
			Success:   true,
			Incidents: []Incident{},
			Total:     0,
		}, nil
	}

	incidents := make([]Incident, len(resp.Incidents))
	for i, inc := range resp.Incidents {
		incidents[i] = Incident{
			ID:          inc.ID,
			Title:       inc.Title,
			Status:      inc.Status,
			Urgency:     inc.Urgency,
			ServiceName: inc.Service.Summary,
			ServiceID:   inc.Service.ID,
			CreatedAt:   inc.CreatedAt,
			HTMLURL:     inc.HTMLURL,
		}
	}

	return Response{
		Operation: ListIncidents,
		Success:   true,
		Incidents: incidents,
		Total:     len(incidents),
	}, nil
}

// resolveServiceNamesToIDs 根据 service name 列表查询对应的 service ID 列表
func (s *Service) resolveServiceNamesToIDs(ctx context.Context, serviceNames []string) ([]string, error) {
	log.Printf("[pagerduty] resolveServiceNamesToIDs called with service_names=%v", serviceNames)

	servicesResp, err := s.client.ListServicesWithContext(ctx, pagerduty.ListServiceOptions{})
	if err != nil {
		log.Printf("[pagerduty] resolveServiceNamesToIDs failed to list services: %v", err)
		return nil, fmt.Errorf("list services: %w", err)
	}

	// 创建 name -> ID 的映射
	nameToID := make(map[string]string)
	for _, svc := range servicesResp.Services {
		nameToID[svc.Name] = svc.ID
	}

	// 根据 serviceNames 查找对应的 IDs
	var serviceIDs []string
	var notFound []string
	for _, name := range serviceNames {
		if id, ok := nameToID[name]; ok {
			serviceIDs = append(serviceIDs, id)
		} else {
			notFound = append(notFound, name)
		}
	}

	if len(notFound) > 0 {
		return serviceIDs, fmt.Errorf("service names not found: %v", notFound)
	}

	return serviceIDs, nil
}

func (s *Service) snoozeAlert(ctx context.Context, params *SnoozeAlertParams) (Response, error) {
	log.Printf("[pagerduty] snoozeAlert called with incident_id=%s, duration=%d minutes", params.IncidentID, params.Duration)

	durationSec := uint(params.Duration * 60)

	inc, err := s.client.SnoozeIncidentWithContext(ctx, params.IncidentID, durationSec)
	if err != nil {
		log.Printf("[pagerduty] snoozeAlert failed: %v", err)
		return Response{Success: false, Message: err.Error()}, nil
	}
	log.Printf("[pagerduty] snoozeAlert succeeded for incident %s", params.IncidentID)

	// Assuming inc has PendingActions or similar if we want snooze_until.
	// Or we can just calculate it for response.
	// Using empty for now or best effort.

	return Response{
		Operation: SnoozeAlert,
		Success:   true,
		Message:   fmt.Sprintf("Snoozed incident %s for %d minutes", params.IncidentID, params.Duration),
		Incident: &Incident{
			ID:     inc.ID,
			Status: inc.Status,
			// ... populate other fields if needed, but message confirms action
		},
	}, nil
}

func (s *Service) acknowledgeIncident(ctx context.Context, params *AcknowledgeIncidentParams) (Response, error) {
	log.Printf("[pagerduty] acknowledgeIncident called with incident_id=%s", params.IncidentID)

	_, err := s.client.ManageIncidentsWithContext(ctx, s.opts.From, []pagerduty.ManageIncidentsOptions{
		{
			ID:     params.IncidentID,
			Status: "acknowledged",
		},
	})

	if err != nil {
		log.Printf("[pagerduty] acknowledgeIncident failed: %v", err)
		return Response{Success: false, Message: err.Error()}, nil
	}
	log.Printf("[pagerduty] acknowledgeIncident succeeded for incident %s", params.IncidentID)

	return Response{
		Operation: AcknowledgeIncident,
		Success:   true,
		Message:   fmt.Sprintf("Acknowledged incident %s", params.IncidentID),
	}, nil
}

func (s *Service) resolveIncident(ctx context.Context, params *ResolveIncidentParams) (Response, error) {
	log.Printf("[pagerduty] resolveIncident called with incident_id=%s", params.IncidentID)

	_, err := s.client.ManageIncidentsWithContext(ctx, s.opts.From, []pagerduty.ManageIncidentsOptions{
		{
			ID:     params.IncidentID,
			Status: "resolved",
		},
	})

	if err != nil {
		log.Printf("[pagerduty] resolveIncident failed: %v", err)
		return Response{Success: false, Message: err.Error()}, nil
	}
	log.Printf("[pagerduty] resolveIncident succeeded for incident %s", params.IncidentID)

	return Response{
		Operation: ResolveIncident,
		Success:   true,
		Message:   fmt.Sprintf("Resolved incident %s", params.IncidentID),
	}, nil
}

func (s *Service) getIncident(ctx context.Context, params *GetIncidentParams) (Response, error) {
	log.Printf("[pagerduty] getIncident called with incident_id=%s", params.IncidentID)

	inc, err := s.client.GetIncidentWithContext(ctx, params.IncidentID)
	if err != nil {
		log.Printf("[pagerduty] getIncident failed: %v", err)
		return Response{Success: false, Message: err.Error()}, nil
	}
	log.Printf("[pagerduty] getIncident succeeded for incident %s", params.IncidentID)

	return Response{
		Operation: GetIncident,
		Success:   true,
		Incident: &Incident{
			ID:          inc.ID,
			Title:       inc.Title,
			Status:      inc.Status,
			Urgency:     inc.Urgency,
			ServiceName: inc.Service.Summary,
			CreatedAt:   inc.CreatedAt,
			HTMLURL:     inc.HTMLURL,
		},
	}, nil
}
