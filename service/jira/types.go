package jira

type Issue struct {
	ID           string                 `json:"id,omitempty"`
	Key          string                 `json:"key,omitempty"`
	Summary      string                 `json:"summary,omitempty"`
	Description  string                 `json:"description,omitempty"`
	Status       string                 `json:"status,omitempty"`
	Assignee     *Assignee              `json:"assignee,omitempty"`
	Labels       []string               `json:"labels,omitempty"`
	CustomFields map[string]interface{} `json:"custom_fields,omitempty"`
	Attachments  []Attachment           `json:"attachments,omitempty"`
	Comments     []Comment              `json:"comments,omitempty"`
	CreatedAt    string                 `json:"created_at"`
	UpdatedAt    string                 `json:"updated_at"`
	DueDate      string                 `json:"due_date,omitempty"`
	Priority     *Priority              `json:"priority,omitempty"`
	Project      *Project               `json:"project,omitempty"`
}

type Assignee struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
	Email       string `json:"email"`
}

type Attachment struct {
	ID       string `json:"id"`
	Filename string `json:"filename"`
	MimeType string `json:"mime_type"`
	Size     int    `json:"size"`
}

type Comment struct {
	ID        string  `json:"id"`
	Body      string  `json:"body"`
	Author    *Author `json:"author,omitempty"`
	CreatedAt string  `json:"created_at"`
	UpdatedAt string  `json:"updated_at"`
}

type Author struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
	Email       string `json:"email"`
}

type Priority struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type Project struct {
	ID          string `json:"id"`
	Key         string `json:"key"`
	DisplayName string `json:"display_name"`
	Description string `json:"description,omitempty"`
}

type CustomField struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Value       interface{} `json:"value"`
}

type ListIssuesParams struct {
	JQL        string `json:"jql" jsonschema:"JQL query string to search issues"`
	MaxResults int    `json:"max_results,omitempty" jsonschema:"Maximum number of results to return"`
}
