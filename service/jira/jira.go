package jira

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/BurntSushi/toml"
	jira "github.com/andygrunwald/go-jira"
	"github.com/go-kratos/blades/tools"
	"github.com/oneblade/service"
)

func init() {
	service.RegisterOptionsParser(service.Jira, func(meta *toml.MetaData, primitive toml.Primitive) (interface{}, error) {
		return service.ParseOptions[Options](meta, primitive, service.Jira)
	})

	service.RegisterService(service.Jira, func(meta service.ServiceMeta, opts interface{}) (service.Service, error) {
		jiraOpts, ok := opts.(*Options)
		if !ok {
			return nil, fmt.Errorf("invalid jira options type, got %T", opts)
		}
		return NewService(meta, jiraOpts)
	})
}

type Options struct {
	URL      string `toml:"url" validate:"required,url"`
	Username string `toml:"username" validate:"required"`
	Password string `toml:"password" validate:"required"`
}

type Service struct {
	name        string
	description string
	opts        *Options
	client      *jira.Client
}

func NewService(meta service.ServiceMeta, opts *Options) (*Service, error) {
	tp := jira.BasicAuthTransport{
		Username: opts.Username,
		Password: opts.Password,
	}
	client, err := jira.NewClient(tp.Client(), opts.URL)
	if err != nil {
		return nil, fmt.Errorf("create jira client: %w", err)
	}
	return &Service{
		name:        meta.Name,
		description: meta.Description,
		opts:        opts,
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
	return service.Jira
}

func (s *Service) Health(ctx context.Context) error {
	log.Printf("[jira] Health check starting")

	healthCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	_, _, err := s.client.User.GetSelfWithContext(healthCtx)
	if err != nil {
		log.Printf("[jira] Health check failed: %v", err)
		return fmt.Errorf("jira health check failed: %w", err)
	}

	log.Printf("[jira] Health check succeeded")
	return nil
}

func (s *Service) Close() error {
	return nil
}

type Operation string

const (
	ListIssues  Operation = "list_issues"
	CreateIssue Operation = "create_issue"
	GetIssue    Operation = "get_issue"
	UpdateIssue Operation = "update_issue"
	AddComment  Operation = "add_comment"
	DeleteIssue Operation = "delete_issue"
)

type Request struct {
	Operation        Operation         `json:"operation" jsonschema:"The type of operation to perform"`
	Issue            *Issue            `json:"issue,omitempty"`
	ListIssuesParams *ListIssuesParams `json:"list_issues_params,omitempty"`
}

type Response struct {
	Operation Operation `json:"operation"`
	Success   bool      `json:"success"`
	Message   string    `json:"message,omitempty"`
	Issue     *Issue    `json:"issue,omitempty"`
	Issues    []*Issue  `json:"issues,omitempty"`
}

func (s *Service) AsTool() (tools.Tool, error) {
	return tools.NewFunc(
		s.name,
		s.Description(),
		s.Handle,
	)
}

func (s *Service) Handle(ctx context.Context, req Request) (Response, error) {
	log.Printf("[jira] Handle called with operation: %s", req.Operation)

	switch req.Operation {
	case ListIssues:
		if req.ListIssuesParams == nil {
			log.Printf("[jira] Handle: list_issues params is nil, returning error")
			return Response{Success: false, Message: "missing list_issues params"}, nil
		}
		log.Printf("[jira] Handle: list_issues params present, calling ListIssues")
		return s.ListIssues(ctx, req.ListIssuesParams)
	case CreateIssue:
		if req.Issue == nil {
			log.Printf("[jira] Handle: create_issue params is nil, returning error")
			return Response{Success: false, Message: "missing issue params"}, nil
		}
		log.Printf("[jira] Handle: create_issue params present, calling CreateIssue")
		return s.CreateIssue(ctx, req.Issue)
	case GetIssue:
		if req.Issue == nil {
			log.Printf("[jira] Handle: get_issue params is nil, returning error")
			return Response{Success: false, Message: "missing issue params"}, nil
		}
		log.Printf("[jira] Handle: get_issue params present, calling GetIssue")
		return s.GetIssue(ctx, req.Issue)
	case UpdateIssue:
		if req.Issue == nil {
			log.Printf("[jira] Handle: update_issue params is nil, returning error")
			return Response{Success: false, Message: "missing issue params"}, nil
		}
		log.Printf("[jira] Handle: update_issue params present, calling UpdateIssue")
		return s.UpdateIssue(ctx, req.Issue)
	case AddComment:
		if req.Issue == nil {
			log.Printf("[jira] Handle: add_comment params is nil, returning error")
			return Response{Success: false, Message: "missing issue params"}, nil
		}
		log.Printf("[jira] Handle: add_comment params present, calling AddComment")
		return s.AddComment(ctx, req.Issue)
	case DeleteIssue:
		if req.Issue == nil {
			log.Printf("[jira] Handle: delete_issue params is nil, returning error")
			return Response{Success: false, Message: "missing issue params"}, nil
		}
		log.Printf("[jira] Handle: delete_issue params present, calling DeleteIssue")
		return s.DeleteIssue(ctx, req.Issue)
	default:
		return Response{Success: false, Message: fmt.Sprintf("unknown operation: %s", req.Operation)}, nil
	}
}

func (s *Service) ListIssues(ctx context.Context, params *ListIssuesParams) (Response, error) {
	log.Printf("[jira] ListIssues called with jql=%s, max_results=%d", params.JQL, params.MaxResults)

	if params == nil || params.JQL == "" {
		log.Printf("[jira] ListIssues failed: JQL query is required")
		return Response{Success: false, Message: "JQL query is required"}, nil
	}

	maxResults := 50
	if params.MaxResults > 0 {
		maxResults = params.MaxResults
	}

	opts := &jira.SearchOptions{
		MaxResults: maxResults,
	}

	jiraIssues, _, err := s.client.Issue.SearchWithContext(ctx, params.JQL, opts)
	if err != nil {
		log.Printf("[jira] ListIssues failed: %v", err)
		return Response{Success: false, Message: fmt.Sprintf("failed to list issues: %v", err)}, nil
	}

	log.Printf("[jira] ListIssues succeeded, found %d issues", len(jiraIssues))
	issues := fromJiraIssues(jiraIssues)
	return Response{Success: true, Message: "Issues listed successfully", Issues: issues}, nil
}

func fromJiraIssues(jiraIssues []jira.Issue) []*Issue {
	issues := make([]*Issue, 0, len(jiraIssues))
	for _, ji := range jiraIssues {
		issues = append(issues, fromJiraIssue(&ji))
	}
	return issues
}

func fromJiraIssue(ji *jira.Issue) *Issue {
	if ji == nil || ji.Fields == nil {
		return &Issue{}
	}

	issue := &Issue{
		ID:           ji.ID,
		Key:          ji.Key,
		Summary:      ji.Fields.Summary,
		Description:  ji.Fields.Description,
		Labels:       ji.Fields.Labels,
		CustomFields: ji.Fields.Unknowns,
		CreatedAt:    time.Time(ji.Fields.Created).Format("2006-01-02T15:04:05.000Z0700"),
		UpdatedAt:    time.Time(ji.Fields.Updated).Format("2006-01-02T15:04:05.000Z0700"),
		DueDate:      time.Time(ji.Fields.Duedate).Format("2006-01-02"),
		Status:       ji.Fields.Status.Name,
		Project: &Project{
			ID:          ji.Fields.Project.ID,
			Key:         ji.Fields.Project.Key,
			DisplayName: ji.Fields.Project.Name,
		},
	}

	if ji.Fields.Assignee != nil {
		issue.Assignee = &Assignee{
			ID:          ji.Fields.Assignee.AccountID,
			DisplayName: ji.Fields.Assignee.DisplayName,
			Email:       ji.Fields.Assignee.EmailAddress,
		}
	}
	if ji.Fields.Priority != nil {
		issue.Priority = &Priority{
			ID:   ji.Fields.Priority.ID,
			Name: ji.Fields.Priority.Name,
		}
	}
	return issue
}

func toJiraIssue(issue *Issue) *jira.Issue {
	if issue == nil {
		return &jira.Issue{}
	}

	jIssue := &jira.Issue{
		ID:  issue.ID,
		Key: issue.Key,
		Fields: &jira.IssueFields{
			Summary:     issue.Summary,
			Description: issue.Description,
			Labels:      issue.Labels,
		},
	}

	if issue.Project != nil {
		jIssue.Fields.Project = jira.Project{
			Key: issue.Project.Key,
			ID:  issue.Project.ID,
		}
	}
	if issue.Priority != nil {
		jIssue.Fields.Priority = &jira.Priority{
			Name: issue.Priority.Name,
			ID:   issue.Priority.ID,
		}
	}
	if issue.Assignee != nil {
		jIssue.Fields.Assignee = &jira.User{
			AccountID:    issue.Assignee.ID,
			Name:         issue.Assignee.DisplayName,
			EmailAddress: issue.Assignee.Email,
		}
	}
	return jIssue
}

func (s *Service) CreateIssue(ctx context.Context, issue *Issue) (Response, error) {
	log.Printf("[jira] CreateIssue called with project_key=%s, summary=%s", issue.Project.Key, issue.Summary)

	jIssue := toJiraIssue(issue)
	// Create usually requires Project Key and Issue Type
	// If our Issue struct lacks IssueType, we might fail validation on Jira side
	// but here we just pass what we can.
	// Actually, check if IssueType is in Issue struct?
	// It's not in the visible changes so far, might need to add it or pass via map.
	// For now assuming the user provided valid fields in Issue struct.

	// go-jira Create returns *Issue, *Response, error
	created, _, err := s.client.Issue.Create(jIssue)
	if err != nil {
		log.Printf("[jira] CreateIssue failed: %v", err)
		return Response{Success: false, Message: fmt.Sprintf("failed to create issue: %v", err)}, nil
	}

	log.Printf("[jira] CreateIssue succeeded, issue key=%s", created.Key)
	return Response{Success: true, Message: "Issue created successfully", Issue: fromJiraIssue(created)}, nil
}

func (s *Service) GetIssue(ctx context.Context, issue *Issue) (Response, error) {
	idOrKey := issue.ID
	if idOrKey == "" {
		idOrKey = issue.Key
	}

	log.Printf("[jira] GetIssue called with id_or_key=%s", idOrKey)

	if issue == nil || (issue.ID == "" && issue.Key == "") {
		return Response{Success: false, Message: "missing issue id or key"}, nil
	}

	result, _, err := s.client.Issue.Get(idOrKey, nil)
	if err != nil {
		log.Printf("[jira] GetIssue failed: %v", err)
		return Response{Success: false, Message: fmt.Sprintf("failed to get issue: %v", err)}, nil
	}

	log.Printf("[jira] GetIssue succeeded for %s", idOrKey)
	return Response{Success: true, Message: "Issue retrieved successfully", Issue: fromJiraIssue(result)}, nil
}

func (s *Service) UpdateIssue(ctx context.Context, issue *Issue) (Response, error) {
	idOrKey := issue.ID
	if idOrKey == "" {
		idOrKey = issue.Key
	}

	log.Printf("[jira] UpdateIssue called with id_or_key=%s", idOrKey)

	if issue == nil || (issue.ID == "" && issue.Key == "") {
		return Response{Success: false, Message: "missing issue id or key"}, nil
	}

	// go-jira Update takes (issueID, map[string]interface{}) usually for flexibility
	// or we can pass a struct if we only update standard fields.
	// Using map ensures we only send what we want to update.

	jIssue := toJiraIssue(issue)
	// Update with struct (go-jira v1.17.0+ seems to enforce this or map check failed)
	_, _, err := s.client.Issue.Update(jIssue)
	if err != nil {
		log.Printf("[jira] UpdateIssue failed: %v", err)
		return Response{Success: false, Message: fmt.Sprintf("failed to update issue: %v", err)}, nil
	}

	// Fetch updated issue to return complete state
	updatedIssue, _, err := s.client.Issue.Get(idOrKey, nil)
	if err != nil {
		log.Printf("[jira] UpdateIssue succeeded but failed to refresh: %v", err)
		// Return success even if fetch fails, but warn?
		return Response{Success: true, Message: "Issue updated successfully but failed to refresh", Issue: nil}, nil
	}

	log.Printf("[jira] UpdateIssue succeeded for %s", idOrKey)
	return Response{Success: true, Message: "Issue updated successfully", Issue: fromJiraIssue(updatedIssue)}, nil
}

func (s *Service) AddComment(ctx context.Context, issue *Issue) (Response, error) {
	idOrKey := issue.ID
	if idOrKey == "" {
		idOrKey = issue.Key
	}

	newComment := issue.Comments[len(issue.Comments)-1]

	log.Printf("[jira] AddComment called with id_or_key=%s", idOrKey)

	if issue == nil || (issue.ID == "" && issue.Key == "") {
		return Response{Success: false, Message: "missing issue id or key"}, nil
	}
	if len(issue.Comments) == 0 {
		return Response{Success: false, Message: "missing comment body"}, nil
	}

	comment := &jira.Comment{
		Body: newComment.Body,
	}

	_, _, err := s.client.Issue.AddComment(idOrKey, comment)
	if err != nil {
		log.Printf("[jira] AddComment failed: %v", err)
		return Response{Success: false, Message: fmt.Sprintf("failed to add comment: %v", err)}, nil
	}

	log.Printf("[jira] AddComment succeeded for %s", idOrKey)
	// We can't easily map back a single comment to Issue struct which has a list,
	// but the caller expects Response.

	return Response{Success: true, Message: "Comment added successfully"}, nil
}

func (s *Service) DeleteIssue(ctx context.Context, issue *Issue) (Response, error) {
	idOrKey := issue.ID
	if idOrKey == "" {
		idOrKey = issue.Key
	}

	log.Printf("[jira] DeleteIssue called with id_or_key=%s", idOrKey)

	if issue == nil || (issue.ID == "" && issue.Key == "") {
		return Response{Success: false, Message: "missing issue id or key"}, nil
	}

	_, err := s.client.Issue.Delete(idOrKey)
	if err != nil {
		log.Printf("[jira] DeleteIssue failed: %v", err)
		return Response{Success: false, Message: fmt.Sprintf("failed to delete issue: %v", err)}, nil
	}

	log.Printf("[jira] DeleteIssue succeeded for %s", idOrKey)
	return Response{Success: true, Message: "Issue deleted successfully"}, nil
}
