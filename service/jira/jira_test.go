package jira

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	jiraapi "github.com/andygrunwald/go-jira"
	"github.com/stretchr/testify/require"
)

func TestHandleUnknownOperation(t *testing.T) {
	svc := &Service{name: "jira"}
	resp, err := svc.Handle(context.Background(), Request{Operation: Operation("nope")})
	require.NoError(t, err)
	require.False(t, resp.Success)
	require.Contains(t, resp.Message, "unknown operation")
}

func TestHandleListIssuesMissingParams(t *testing.T) {
	svc := &Service{name: "jira"}
	resp, err := svc.Handle(context.Background(), Request{Operation: ListIssues})
	require.NoError(t, err)
	require.False(t, resp.Success)
	require.Equal(t, "missing list_issues params", resp.Message)
}

func TestHandleCreateIssueMissingParams(t *testing.T) {
	svc := &Service{name: "jira"}
	resp, err := svc.Handle(context.Background(), Request{Operation: CreateIssue})
	require.NoError(t, err)
	require.False(t, resp.Success)
	require.Equal(t, "missing create_issue params", resp.Message)
}

func TestListIssuesRequiresJQL(t *testing.T) {
	svc := &Service{name: "jira"}
	resp, err := svc.ListIssues(context.Background(), &ListIssuesParams{})
	require.NoError(t, err)
	require.False(t, resp.Success)
	require.Equal(t, "JQL query is required", resp.Message)
}

func TestListIssuesSuccess(t *testing.T) {
	svc, teardown := newTestService(t, func(mux *http.ServeMux) {
		mux.HandleFunc("/rest/api/2/search", func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodGet, r.Method)
			require.Equal(t, "project=OPS", r.URL.Query().Get("jql"))
			require.Equal(t, "10", r.URL.Query().Get("maxResults"))
			fields := r.URL.Query().Get("fields")
			require.Contains(t, fields, "id")
			require.Contains(t, fields, "key")
			require.Contains(t, fields, "summary")
			require.Contains(t, fields, "status")
			require.Contains(t, fields, "assignee")
			require.Contains(t, fields, "reporter")
			require.Contains(t, fields, "created")
			require.Contains(t, fields, "updated")
			require.Contains(t, fields, "duedate")
			fmt.Fprintf(w, searchResponseJSON)
		})
	})
	defer teardown()

	resp, err := svc.ListIssues(context.Background(), &ListIssuesParams{JQL: "project=OPS", MaxResults: 10})
	require.NoError(t, err)
	require.True(t, resp.Success)
	require.Len(t, resp.Issues, 1)
	require.Equal(t, "TEST-1", resp.Issues[0].Key)
	require.NotNil(t, resp.Issues[0].Reporter)
	require.Equal(t, "bob", resp.Issues[0].Reporter.ID)
	require.Equal(t, "Bob", resp.Issues[0].Reporter.DisplayName)
}

func TestListIssuesDefaultMaxResults(t *testing.T) {
	svc, teardown := newTestService(t, func(mux *http.ServeMux) {
		mux.HandleFunc("/rest/api/2/search", func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, "30", r.URL.Query().Get("maxResults"))
			fields := r.URL.Query().Get("fields")
			require.NotEmpty(t, fields)
			fmt.Fprintf(w, searchResponseJSON)
		})
	})
	defer teardown()

	resp, err := svc.ListIssues(context.Background(), &ListIssuesParams{JQL: "project=OPS"})
	require.NoError(t, err)
	require.True(t, resp.Success)
}

func TestCreateIssueMissingType(t *testing.T) {
	svc := &Service{name: "jira"}
	resp, err := svc.CreateIssue(context.Background(), &CreateIssueParams{
		Summary: "Test issue",
	})
	require.NoError(t, err)
	require.False(t, resp.Success)
	require.Contains(t, resp.Message, "issue type is required")
}

func TestCreateIssueMissingSummary(t *testing.T) {
	svc := &Service{name: "jira"}
	resp, err := svc.CreateIssue(context.Background(), &CreateIssueParams{
		Type: "DevOps",
	})
	require.NoError(t, err)
	require.False(t, resp.Success)
	require.Contains(t, resp.Message, "summary is required")
}

func TestCreateIssueSuccess(t *testing.T) {
	svc, teardown := newTestService(t, func(mux *http.ServeMux) {
		mux.HandleFunc("/rest/api/2/issue", func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodPost, r.Method)
			fmt.Fprintf(w, createIssueResponseJSON)
		})
	})
	defer teardown()

	resp, err := svc.CreateIssue(context.Background(), &CreateIssueParams{
		Type:    "DevOps",
		Summary: "Test issue",
	})
	require.NoError(t, err)
	require.True(t, resp.Success)
	require.NotNil(t, resp.Issue)
	require.Equal(t, "TEST-1", resp.Issue.Key)
	require.Equal(t, "Test issue", resp.Issue.Summary)
}

func TestCreateIssueWithDefaultProject(t *testing.T) {
	svc, teardown := newTestService(t, func(mux *http.ServeMux) {
		mux.HandleFunc("/rest/api/2/issue", func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodPost, r.Method)
			fmt.Fprintf(w, createIssueResponseJSON)
		})
	})
	defer teardown()

	// 不指定 Project，应该使用默认的 ProjectKey
	resp, err := svc.CreateIssue(context.Background(), &CreateIssueParams{
		Type:    "DevOps",
		Summary: "Test issue",
	})
	require.NoError(t, err)
	require.True(t, resp.Success)
}

func TestCreateIssueWithNilFields(t *testing.T) {
	// 测试当 Jira API 只返回 id 和 key，不返回 fields 的情况
	svc, teardown := newTestService(t, func(mux *http.ServeMux) {
		mux.HandleFunc("/rest/api/2/issue", func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodPost, r.Method)
			// 模拟 Jira API 创建 issue 时的最小响应（只包含 id 和 key）
			fmt.Fprintf(w, `{
				"id": "10001",
				"key": "TEST-2",
				"self": "https://test.jira.com/rest/api/2/issue/10001"
			}`)
		})
	})
	defer teardown()

	resp, err := svc.CreateIssue(context.Background(), &CreateIssueParams{
		Type:    "DevOps",
		Summary: "Test issue with nil fields",
	})
	require.NoError(t, err)
	require.True(t, resp.Success)
	require.NotNil(t, resp.Issue)
	// 验证即使 Fields 为 nil，Key 和 ID 也应该被正确返回
	require.Equal(t, "TEST-2", resp.Issue.Key)
	require.Equal(t, "10001", resp.Issue.ID)
	// 其他字段应该为空（因为 Fields 为 nil）
	require.Empty(t, resp.Issue.Summary)
}

func TestHealthSuccess(t *testing.T) {
	svc, teardown := newTestService(t, func(mux *http.ServeMux) {
		mux.HandleFunc("/rest/api/2/myself", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprintf(w, `{"self":"%s","name":"dummy"}`, r.URL.String())
		})
	})
	defer teardown()

	require.NoError(t, svc.Health(context.Background()))
}

func TestHealthFailure(t *testing.T) {
	svc, teardown := newTestService(t, func(mux *http.ServeMux) {
		mux.HandleFunc("/rest/api/2/myself", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprint(w, `{"error":"boom"}`)
		})
	})
	defer teardown()

	err := svc.Health(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "health check failed")
}

const createIssueResponseJSON = `{
	"id": "10001",
	"key": "TEST-1",
	"fields": {
		"summary": "Test issue",
		"status": {"name": "To Do"},
		"created": "2024-01-01T00:00:00.000+0000",
		"updated": "2024-01-01T00:00:00.000+0000",
		"project": {"key": "ONEPOINT", "name": "One Point Platform"},
		"issuetype": {"id": "10001", "name": "DevOps"}
	}
}`

const searchResponseJSON = `{
	"startAt": 0,
	"maxResults": 1,
	"total": 1,
	"issues": [
		{
			"id": "10001",
			"key": "TEST-1",
			"fields": {
				"summary": "sample issue",
				"status": {"name": "In Progress"},
				"created": "2024-01-01T00:00:00.000+0000",
				"updated": "2024-01-01T01:00:00.000+0000",
				"duedate": "2024-01-07",
				"assignee": {"accountId": "abc", "displayName": "Alice", "emailAddress": "alice@example.com"},
				"reporter": {"accountId": "bob", "displayName": "Bob", "emailAddress": "bob@example.com"}
			}
		}
	]
}`

func newTestService(t *testing.T, register func(mux *http.ServeMux)) (*Service, func()) {
	t.Helper()
	mux := http.NewServeMux()
	if register != nil {
		register(mux)
	}
	server := httptest.NewServer(mux)
	client, err := jiraapi.NewClient(server.Client(), server.URL)
	require.NoError(t, err)
	client.Authentication.SetBasicAuth("user", "pass")
	svc := &Service{
		name:        "jira",
		description: "test service",
		opts: &Options{
			URL:      server.URL,
			Username: "user",
			Password: "pass",
		},
		client: client,
	}
	return svc, server.Close
}
