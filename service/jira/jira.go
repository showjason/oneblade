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
		if req.CreateIssueParams == nil {
			log.Printf("[jira] Handle: create_issue params is nil, returning error")
			return Response{Success: false, Message: "missing create_issue params"}, nil
		}
		log.Printf("[jira] Handle: create_issue params present, calling CreateIssue")
		return s.CreateIssue(ctx, req.CreateIssueParams)
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

	maxResults := defaultMaxResults
	if params.MaxResults > 0 {
		maxResults = params.MaxResults
	}

	requiredFields := []string{
		"id", "key", "summary", "status",
		"assignee", "reporter", "priority",
		"created", "updated", "duedate",
	}

	opts := &jira.SearchOptions{
		MaxResults: maxResults,
		Fields:     requiredFields,
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
	if ji == nil {
		return &Issue{}
	}

	// Key 和 ID 在顶层，即使 Fields 为 nil 也应该保留
	issue := &Issue{
		ID:  ji.ID,
		Key: ji.Key,
	}

	// 如果 Fields 为 nil，只返回 Key 和 ID
	if ji.Fields == nil {
		return issue
	}

	// Fields 存在，只填充必需字段（对应 types.go 中定义的常量）
	issue.Summary = ji.Fields.Summary
	issue.Status = ji.Fields.Status.Name
	issue.CreatedAt = time.Time(ji.Fields.Created).Format("2006-01-02T15:04:05.000Z0700")
	issue.UpdatedAt = time.Time(ji.Fields.Updated).Format("2006-01-02T15:04:05.000Z0700")
	issue.DueDate = time.Time(ji.Fields.Duedate).Format("2006-01-02")

	if ji.Fields.Assignee != nil {
		issue.Assignee = &Assignee{
			DisplayName: ji.Fields.Assignee.DisplayName,
			Email:       ji.Fields.Assignee.EmailAddress,
		}
	}
	if ji.Fields.Reporter != nil {
		issue.Reporter = &Reporter{
			DisplayName: ji.Fields.Reporter.DisplayName,
			Email:       ji.Fields.Reporter.EmailAddress,
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
	if issue.Priority != nil {
		jIssue.Fields.Priority = &jira.Priority{
			Name: issue.Priority.Name,
		}
	}
	if issue.Assignee != nil {
		jIssue.Fields.Assignee = &jira.User{
			Name:         issue.Assignee.DisplayName,
			EmailAddress: issue.Assignee.Email,
		}
	}
	return jIssue
}

func (s *Service) CreateIssue(ctx context.Context, params *CreateIssueParams) (Response, error) {
	log.Printf("[jira] CreateIssue called with project=%s, type=%s, summary=%s", params.Project, params.Type, params.Summary)

	if params.Type == "" {
		return Response{Success: false, Message: "issue type is required for creating issue"}, nil
	}
	if params.Summary == "" {
		return Response{Success: false, Message: "summary is required for creating issue"}, nil
	}

	projectKey := params.Project
	if projectKey == "" {
		projectKey = ProjectKey
	}

	buildJiraIssue := func() *jira.Issue {
		jIssue := &jira.Issue{
			Fields: &jira.IssueFields{
				Project: jira.Project{
					Key: projectKey,
				},
				Type: jira.IssueType{
					Name: params.Type, // 支持 name 或 id，Jira API 会自动处理
				},
				Summary:     params.Summary,
				Description: params.Description,
				Labels:      params.Labels,
			},
		}

		if params.Priority != nil {
			jIssue.Fields.Priority = &jira.Priority{
				Name: params.Priority.Name,
			}
		}

		if params.Assignee != nil {
			jIssue.Fields.Assignee = &jira.User{
				Name:         params.Assignee.DisplayName,
				EmailAddress: params.Assignee.Email,
			}
		}

		return jIssue
	}

	// 创建 issue（使用 CreateWithContext 以支持 context）
	jIssue := buildJiraIssue()
	created, _, err := s.client.Issue.CreateWithContext(ctx, jIssue)
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

func (s *Service) updateStatus(idOrKey, targetStatusName string) error {
	transitions, _, err := s.client.Issue.GetTransitions(idOrKey)
	if err != nil {
		return fmt.Errorf("failed to get transitions: %w", err)
	}

	var transitionID string
	for _, t := range transitions {
		if t.To.Name == targetStatusName {
			transitionID = t.ID
			break
		}
	}

	if transitionID == "" {
		return fmt.Errorf("no transition found to status: %s", targetStatusName)
	}

	_, err = s.client.Issue.DoTransition(idOrKey, transitionID)
	return err
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

	if issue.Status != "" {
		err := s.updateStatus(idOrKey, issue.Status)
		if err != nil {
			log.Printf("[jira] UpdateIssue failed to update status: %v", err)
			return Response{Success: false, Message: fmt.Sprintf("failed to update status: %v", err)}, nil
		}
	}

	hasOtherFields := issue.Summary != "" || issue.Description != "" || issue.Assignee != nil ||
		issue.Priority != nil || len(issue.Labels) > 0

	if hasOtherFields {
		_, _, err := s.client.Issue.Update(toJiraIssue(issue))
		if err != nil {
			log.Printf("[jira] UpdateIssue failed: %v", err)
			return Response{Success: false, Message: fmt.Sprintf("failed to update issue: %v", err)}, nil
		}
	}

	log.Printf("[jira] UpdateIssue succeeded for %s", idOrKey)
	return Response{Success: true, Message: "Issue updated successfully"}, nil
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
