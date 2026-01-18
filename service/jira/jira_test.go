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
			fmt.Fprintf(w, searchResponseJSON)
		})
	})
	defer teardown()

	resp, err := svc.ListIssues(context.Background(), &ListIssuesParams{JQL: "project=OPS", MaxResults: 10})
	require.NoError(t, err)
	require.True(t, resp.Success)
	require.Len(t, resp.Issues, 1)
	require.Equal(t, "TEST-1", resp.Issues[0].Key)
	require.Equal(t, "Ops", resp.Issues[0].Project.DisplayName)
}

func TestListIssuesDefaultMaxResults(t *testing.T) {
	svc, teardown := newTestService(t, func(mux *http.ServeMux) {
		mux.HandleFunc("/rest/api/2/search", func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, "50", r.URL.Query().Get("maxResults"))
			fmt.Fprintf(w, searchResponseJSON)
		})
	})
	defer teardown()

	resp, err := svc.ListIssues(context.Background(), &ListIssuesParams{JQL: "project=OPS"})
	require.NoError(t, err)
	require.True(t, resp.Success)
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
				"description": "desc",
				"labels": ["devops"],
				"project": {"id": "200", "key": "OPS", "name": "Ops"},
				"status": {"name": "In Progress"},
				"created": "2024-01-01T00:00:00.000+0000",
				"updated": "2024-01-01T01:00:00.000+0000",
				"duedate": "2024-01-07",
				"priority": {"id": "1", "name": "High"},
				"assignee": {"accountId": "abc", "displayName": "Alice", "emailAddress": "alice@example.com"}
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
