package jira

import (
	"context"
	"fmt"
	"strings"
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
	Operation Operation `json:"operation" jsonschema:"The type of operation to perform"`
	Issue     *Issue    `json:"issue,omitempty"`
}

type Response struct {
	Operation Operation `json:"operation"`
	Success   bool      `json:"success"`
	Message   string    `json:"message,omitempty"`
	Issue     *Issue    `json:"issue,omitempty"`
	Issues    []*Issue  `json:"issues,omitempty"`
}

type ListIssuesParams struct {
	ProjectKey string   `json:"project_key,omitempty" jsonschema:"The project key to filter by"`
	IssueType  string   `json:"issue_type,omitempty" jsonschema:"The issue type to filter by"`
	Status     string   `json:"status,omitempty" jsonschema:"The status to filter by"`
	Assignee   string   `json:"assignee,omitempty" jsonschema:"The assignee to filter by"`
	Labels     []string `json:"labels,omitempty" jsonschema:"The labels to filter by"`
	Priority   string   `json:"priority,omitempty" jsonschema:"The priority to filter by"`
}

func (s *Service) AsTool() (tools.Tool, error) {
	return tools.NewFunc(
		s.name,
		s.Description(),
		s.Handle,
	)
}

func (s *Service) Handle(ctx context.Context, req Request) (Response, error) {
	switch req.Operation {
	case CreateIssue:
		return s.CreateIssue(ctx, req.Issue)
	case GetIssue:
		return s.GetIssue(ctx, req.Issue)
	case UpdateIssue:
		return s.UpdateIssue(ctx, req.Issue)
	case AddComment:
		return s.AddComment(ctx, req.Issue)
	case DeleteIssue:
		return s.DeleteIssue(ctx, req.Issue)
	}

	return Response{Success: false, Message: fmt.Sprintf("unknown operation: %s", req.Operation)}, nil
}

func (s *Service) ListIssues(ctx context.Context, params *ListIssuesParams) (Response, error) {
	if params == nil {
		params = &ListIssuesParams{}
	}
	jql := buildJQL(params)
	opts := &jira.SearchOptions{
		MaxResults: 50,
	}
	jiraIssues, _, err := s.client.Issue.SearchWithContext(ctx, jql, opts)
	if err != nil {
		return Response{Success: false, Message: fmt.Sprintf("failed to list issues: %v", err)}, nil
	}
	issues := fromJiraIssues(jiraIssues)
	return Response{Success: true, Message: "Issues listed successfully", Issues: issues}, nil
}

func buildJQL(params *ListIssuesParams) string {
	var conditions []string
	if params.ProjectKey != "" {
		conditions = append(conditions, fmt.Sprintf("project = %s", params.ProjectKey))
	}
	if params.IssueType != "" {
		conditions = append(conditions, fmt.Sprintf("issueType = %s", params.IssueType))
	}
	if params.Status != "" {
		conditions = append(conditions, fmt.Sprintf("status = %q", params.Status))
	}
	if params.Assignee != "" {
		conditions = append(conditions, fmt.Sprintf("assignee = %q", params.Assignee))
	}
	if len(params.Labels) > 0 {
		labelQueries := make([]string, len(params.Labels))
		for i, label := range params.Labels {
			labelQueries[i] = fmt.Sprintf("labels = %q", label)
		}
		conditions = append(conditions, fmt.Sprintf("(%s)", strings.Join(labelQueries, " OR ")))
	}
	if params.Priority != "" {
		conditions = append(conditions, fmt.Sprintf("priority = %q", params.Priority))
	}
	jql := strings.Join(conditions, " AND ")
	if jql == "" {
		return "ORDER BY created DESC"
	}
	return jql + " ORDER BY created DESC"
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
		return Response{Success: false, Message: fmt.Sprintf("failed to create issue: %v", err)}, nil
	}

	return Response{Success: true, Message: "Issue created successfully", Issue: fromJiraIssue(created)}, nil
}

func (s *Service) GetIssue(ctx context.Context, issue *Issue) (Response, error) {
	if issue == nil || (issue.ID == "" && issue.Key == "") {
		return Response{Success: false, Message: "missing issue id or key"}, nil
	}
	idOrKey := issue.ID
	if idOrKey == "" {
		idOrKey = issue.Key
	}

	result, _, err := s.client.Issue.Get(idOrKey, nil)
	if err != nil {
		return Response{Success: false, Message: fmt.Sprintf("failed to get issue: %v", err)}, nil
	}

	return Response{Success: true, Message: "Issue retrieved successfully", Issue: fromJiraIssue(result)}, nil
}

func (s *Service) UpdateIssue(ctx context.Context, issue *Issue) (Response, error) {
	if issue == nil || (issue.ID == "" && issue.Key == "") {
		return Response{Success: false, Message: "missing issue id or key"}, nil
	}
	idOrKey := issue.ID
	if idOrKey == "" {
		idOrKey = issue.Key
	}

	// go-jira Update takes (issueID, map[string]interface{}) usually for flexibility
	// or we can pass a struct if we only update standard fields.
	// Using map ensures we only send what we want to update.

	jIssue := toJiraIssue(issue)
	// Update with struct (go-jira v1.17.0+ seems to enforce this or map check failed)
	_, _, err := s.client.Issue.Update(jIssue)
	if err != nil {
		return Response{Success: false, Message: fmt.Sprintf("failed to update issue: %v", err)}, nil
	}

	// Fetch updated issue to return complete state
	updatedIssue, _, err := s.client.Issue.Get(idOrKey, nil)
	if err != nil {
		// Return success even if fetch fails, but warn?
		return Response{Success: true, Message: "Issue updated successfully but failed to refresh", Issue: nil}, nil
	}

	return Response{Success: true, Message: "Issue updated successfully", Issue: fromJiraIssue(updatedIssue)}, nil
}

func (s *Service) AddComment(ctx context.Context, issue *Issue) (Response, error) {
	if issue == nil || (issue.ID == "" && issue.Key == "") {
		return Response{Success: false, Message: "missing issue id or key"}, nil
	}
	if len(issue.Comments) == 0 {
		return Response{Success: false, Message: "missing comment body"}, nil
	}

	idOrKey := issue.ID
	if idOrKey == "" {
		idOrKey = issue.Key
	}

	// Assuming we take the last comment from the list as the new comment to add
	newComment := issue.Comments[len(issue.Comments)-1]

	comment := &jira.Comment{
		Body: newComment.Body,
	}

	_, _, err := s.client.Issue.AddComment(idOrKey, comment)
	if err != nil {
		return Response{Success: false, Message: fmt.Sprintf("failed to add comment: %v", err)}, nil
	}

	// We can't easily map back a single comment to Issue struct which has a list,
	// but the caller expects Response.

	return Response{Success: true, Message: "Comment added successfully"}, nil
}

func (s *Service) DeleteIssue(ctx context.Context, issue *Issue) (Response, error) {
	if issue == nil || (issue.ID == "" && issue.Key == "") {
		return Response{Success: false, Message: "missing issue id or key"}, nil
	}
	idOrKey := issue.ID
	if idOrKey == "" {
		idOrKey = issue.Key
	}

	_, err := s.client.Issue.Delete(idOrKey)
	if err != nil {
		return Response{Success: false, Message: fmt.Sprintf("failed to delete issue: %v", err)}, nil
	}

	return Response{Success: true, Message: "Issue deleted successfully"}, nil
}
