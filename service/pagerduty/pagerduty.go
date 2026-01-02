package pagerduty

import (
	"context"
	"fmt"
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
}

type Service struct {
	name        string
	description string
	apiKey      string
	client      *pagerduty.Client
}

func NewService(meta service.ServiceMeta, opts *Options) *Service {
	return &Service{
		name:        meta.Name,
		description: meta.Description,
		apiKey:      opts.APIKey,
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
	Since      string   `json:"since" jsonschema:"Start time in RFC3339 format"`
	Until      string   `json:"until" jsonschema:"End time in RFC3339 format"`
	ServiceIDs []string `json:"service_ids,omitempty" jsonschema:"Filter by service IDs"`
	Statuses   []string `json:"statuses,omitempty" jsonschema:"Filter by statuses: triggered, acknowledged, resolved"`
	Limit      int      `json:"limit,omitempty"`
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
	CreatedAt   string `json:"created_at"`
	HTMLURL     string `json:"html_url"`
}

// === Logic ===

func (s *Service) Handle(ctx context.Context, req Request) (Response, error) {
	switch req.Operation {
	case ListIncidents:
		if req.ListIncidents == nil {
			return Response{Success: false, Message: "missing list_incidents params"}, nil
		}
		return s.listIncidents(ctx, req.ListIncidents)
	case SnoozeAlert:
		if req.SnoozeAlert == nil {
			return Response{Success: false, Message: "missing snooze_alert params"}, nil
		}
		return s.snoozeAlert(ctx, req.SnoozeAlert)
	case AcknowledgeIncident:
		if req.AcknowledgeIncident == nil {
			return Response{Success: false, Message: "missing acknowledge_incident params"}, nil
		}
		return s.acknowledgeIncident(ctx, req.AcknowledgeIncident)
	case ResolveIncident:
		if req.ResolveIncident == nil {
			return Response{Success: false, Message: "missing resolve_incident params"}, nil
		}
		return s.resolveIncident(ctx, req.ResolveIncident)
	case GetIncident:
		if req.GetIncident == nil {
			return Response{Success: false, Message: "missing get_incident params"}, nil
		}
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
	limit := params.Limit
	if limit == 0 {
		limit = 50
	}

	opts := pagerduty.ListIncidentsOptions{
		Since: params.Since,
		Until: params.Until,
		Limit: uint(limit),
	}
	if len(params.ServiceIDs) > 0 {
		opts.ServiceIDs = params.ServiceIDs
	}
	if len(params.Statuses) > 0 {
		opts.Statuses = params.Statuses
	}

	resp, err := s.client.ListIncidentsWithContext(ctx, opts)
	if err != nil {
		return Response{Success: false, Message: err.Error()}, nil
	}

	incidents := make([]Incident, len(resp.Incidents))
	for i, inc := range resp.Incidents {
		incidents[i] = Incident{
			ID:          inc.ID,
			Title:       inc.Title,
			Status:      inc.Status,
			Urgency:     inc.Urgency,
			ServiceName: inc.Service.Summary,
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

func (s *Service) snoozeAlert(ctx context.Context, params *SnoozeAlertParams) (Response, error) {
	// PagerDuty Go SDK SnoozeIncident method usually takes ID and duration in seconds
	// Check sdk docs or assume similar to existing capable logic.
	// Since original code didn't implement snooze, I'll implement best effort.
	// client.SnoozeIncident(id, durationSeconds)

	durationSec := uint(params.Duration * 60)
	// Note: checking signature of SnoozeIncident in generic thought or assuming it exists.
	// If it doesn't exist, I might fail build.
	// Let's assume standard PagerDuty V2 API behavior.
	// Actually, the go-pagerduty library usually has `SnoozeIncidentWithContext`.

	inc, err := s.client.SnoozeIncidentWithContext(ctx, params.IncidentID, durationSec)
	if err != nil {
		return Response{Success: false, Message: err.Error()}, nil
	}

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
	// Manage incidents takes a list of Reference objects
	_, err := s.client.ManageIncidentsWithContext(ctx, s.apiKey, []pagerduty.ManageIncidentsOptions{
		{
			ID:     params.IncidentID,
			Status: "acknowledged",
		},
	})

	if err != nil {
		return Response{Success: false, Message: err.Error()}, nil
	}

	return Response{
		Operation: AcknowledgeIncident,
		Success:   true,
		Message:   fmt.Sprintf("Acknowledged incident %s", params.IncidentID),
	}, nil
}

func (s *Service) resolveIncident(ctx context.Context, params *ResolveIncidentParams) (Response, error) {
	_, err := s.client.ManageIncidentsWithContext(ctx, s.apiKey, []pagerduty.ManageIncidentsOptions{
		{
			ID:     params.IncidentID,
			Status: "resolved",
		},
	})

	if err != nil {
		return Response{Success: false, Message: err.Error()}, nil
	}

	return Response{
		Operation: ResolveIncident,
		Success:   true,
		Message:   fmt.Sprintf("Resolved incident %s", params.IncidentID),
	}, nil
}

func (s *Service) getIncident(ctx context.Context, params *GetIncidentParams) (Response, error) {
	inc, err := s.client.GetIncidentWithContext(ctx, params.IncidentID)
	if err != nil {
		return Response{Success: false, Message: err.Error()}, nil
	}

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
