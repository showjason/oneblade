package jira

const defaultMaxResults = 30

const ProjectKey = "ONEPOINT"

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
	Operation         Operation          `json:"operation" jsonschema:"The type of operation to perform"`
	Issue             *Issue             `json:"issue,omitempty" jsonschema:"Required for get_issue, update_issue, add_comment, delete_issue"`
	CreateIssueParams *CreateIssueParams `json:"create_issue_params,omitempty" jsonschema:"Required when operation is create_issue"`
	ListIssuesParams  *ListIssuesParams  `json:"list_issues_params,omitempty" jsonschema:"Required when operation is list_issues"`
}

type Response struct {
	Operation Operation `json:"operation"`
	Success   bool      `json:"success"`
	Message   string    `json:"message,omitempty"`
	Issue     *Issue    `json:"issue,omitempty"`
	Issues    []*Issue  `json:"issues,omitempty"`
}

type Issue struct {
	ID           string                 `json:"id,omitempty"`
	Key          string                 `json:"key,omitempty"`
	Summary      string                 `json:"summary,omitempty"`
	Description  string                 `json:"description,omitempty"`
	Status       string                 `json:"status,omitempty"`
	Assignee     *Assignee              `json:"assignee,omitempty"`
	Reporter     *Reporter              `json:"reporter,omitempty"`
	Labels       []string               `json:"labels,omitempty"`
	CustomFields map[string]interface{} `json:"custom_fields,omitempty"`
	Comments     []Comment              `json:"comments,omitempty"`
	CreatedAt    string                 `json:"created_at"`
	UpdatedAt    string                 `json:"updated_at"`
	DueDate      string                 `json:"due_date,omitempty"`
	Priority     *Priority              `json:"priority,omitempty"`
	Project      string                 `json:"project,omitempty"`
	Type         string                 `json:"type,omitempty"`
}

type Assignee struct {
	DisplayName string `json:"display_name"`
	Email       string `json:"email"`
}

type Reporter struct {
	DisplayName string `json:"display_name"`
	Email       string `json:"email"`
}

type Comment struct {
	Body      string  `json:"body"`
	Author    *Author `json:"author,omitempty"`
	CreatedAt string  `json:"created_at"`
	UpdatedAt string  `json:"updated_at"`
}

type Author struct {
	DisplayName string `json:"display_name"`
	Email       string `json:"email"`
}

type Priority struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type CustomField struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Value       interface{} `json:"value"`
}

type ListIssuesParams struct {
	JQL        string `json:"jql" jsonschema:"JQL query string to search issues"`
	MaxResults int    `json:"max_results,omitempty" jsonschema:"Maximum number of results to return"`
}

type CreateIssueParams struct {
	Project     string    `json:"project,omitempty" jsonschema:"Project key (default: ONEPOINT if not specified)"`
	Type        string    `json:"type" jsonschema:"Issue type name or id (e.g., 'DevOps' or '10001')"`
	Summary     string    `json:"summary" jsonschema:"Issue summary/title, required"`
	Description string    `json:"description,omitempty" jsonschema:"Issue description"`
	Labels      []string  `json:"labels,omitempty" jsonschema:"Issue labels"`
	Priority    *Priority `json:"priority,omitempty" jsonschema:"Issue priority"`
	Assignee    *Assignee `json:"assignee,omitempty" jsonschema:"Issue assignee"`
}
