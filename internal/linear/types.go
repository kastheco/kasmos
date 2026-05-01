package linear

import "time"

// Issue is a Linear issue with the stable fields phase-1 callers need.
type Issue struct {
	ID          string         `json:"id"`
	Identifier  string         `json:"identifier"` // e.g. "ENG-123"
	Title       string         `json:"title"`
	Description string         `json:"description"`
	URL         string         `json:"url"`
	Priority    int            `json:"priority"`
	State       *WorkflowState `json:"state,omitempty"`
	Team        *Team          `json:"team,omitempty"`
	Project     *Project       `json:"project,omitempty"`
	Assignee    *User          `json:"assignee,omitempty"`
	Labels      []Label        `json:"-"` // populated from labels.nodes connection
	CreatedAt   time.Time      `json:"createdAt"`
	UpdatedAt   time.Time      `json:"updatedAt"`
}

type Team struct {
	ID   string `json:"id"`
	Key  string `json:"key"`
	Name string `json:"name"`
}

type WorkflowState struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"` // e.g. "started", "completed", "canceled", "backlog"
}

type Project struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	URL  string `json:"url"`
}

type Label struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color"`
}

type User struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

// PageInfo mirrors Linear's pageInfo connection field.
type PageInfo struct {
	HasNextPage     bool   `json:"hasNextPage"`
	HasPreviousPage bool   `json:"hasPreviousPage"`
	StartCursor     string `json:"startCursor"`
	EndCursor       string `json:"endCursor"`
}

// PageOptions carries cursor-based pagination input. Zero First defaults to 25
// (Linear's documented default). Callers must pass an explicit large value to
// scan a workspace; the adapter never auto-paginates.
type PageOptions struct {
	First int    // 1..50; 0 -> 25
	After string // cursor from previous PageInfo.EndCursor
}

// IssueQuery is the input shape for Issues(). Empty IDs disable that filter.
type IssueQuery struct {
	Page         PageOptions
	TeamID       string
	StateID      string
	LabelID      string
	UpdatedSince *time.Time
	OrderBy      string // "updatedAt" (default) or "createdAt"
}

// CreateIssueInput is the input shape for CreateIssue. Title and TeamID are
// required; any other zero/nil field is omitted from the mutation variables.
type CreateIssueInput struct {
	Title       string
	TeamID      string
	Description string
	StateID     string
	AssigneeID  string
	ProjectID   string
	Priority    *int
	LabelIDs    []string
}

// UpdateIssueInput is the sparse input shape for UpdateIssue. Only fields whose
// pointers are non-nil are sent in the mutation variables; this lets callers
// clear values via empty pointers and leave others untouched.
type UpdateIssueInput struct {
	Title       *string
	Description *string
	StateID     *string
	AssigneeID  *string
	ProjectID   *string
	Priority    *int
	LabelIDs    *[]string
}

// Comment is the Linear comment shape used for trigger polling and writes.
type Comment struct {
	ID        string    `json:"id"`
	URL       string    `json:"url"`
	Body      string    `json:"body"`
	IssueID   string    `json:"-"` // populated by Comments() from query context
	User      *User     `json:"user,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}
