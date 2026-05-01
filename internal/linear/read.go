package linear

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

const (
	queryViewer = `query Viewer { viewer { id name email } }`

	queryUsers = `query Users($first: Int!, $after: String) {
	        users(first: $first, after: $after) {
	            nodes { id name email }
	            pageInfo { hasNextPage hasPreviousPage startCursor endCursor }
	        }
	    }`

	queryTeams = `query Teams($first: Int!, $after: String) {
	        teams(first: $first, after: $after) {
	            nodes { id key name }
            pageInfo { hasNextPage hasPreviousPage startCursor endCursor }
        }
    }`

	queryWorkflowStates = `query WorkflowStates($first: Int!, $after: String) {
        workflowStates(first: $first, after: $after) {
            nodes { id name type }
            pageInfo { hasNextPage hasPreviousPage startCursor endCursor }
        }
    }`

	queryProjects = `query Projects($first: Int!, $after: String) {
        projects(first: $first, after: $after) {
            nodes { id name url }
            pageInfo { hasNextPage hasPreviousPage startCursor endCursor }
        }
    }`

	queryLabels = `query Labels($first: Int!, $after: String) {
	        issueLabels(first: $first, after: $after) {
	            nodes { id name color }
	            pageInfo { hasNextPage hasPreviousPage startCursor endCursor }
	        }
	    }`

	queryIssueLabel = `query IssueLabel($id: String!) {
	        issueLabel(id: $id) { id name color }
	    }`

	queryIssue = `query Issue($id: String!) {
	        issue(id: $id) {
	            id identifier title description url priority createdAt updatedAt
            state { id name type }
            team { id key name }
            project { id name url }
            assignee { id name email }
            labels(first: 50) { nodes { id name color } }
        }
    }`

	queryIssues = `query Issues($first: Int!, $after: String, $filter: IssueFilter, $orderBy: PaginationOrderBy) {
        issues(first: $first, after: $after, filter: $filter, orderBy: $orderBy) {
            nodes {
                id identifier title description url priority createdAt updatedAt
                state { id name type }
                team { id key name }
                project { id name url }
                assignee { id name email }
                labels(first: 50) { nodes { id name color } }
            }
	            pageInfo { hasNextPage hasPreviousPage startCursor endCursor }
	        }
	    }`

	queryComments = `query Comments($issueId: String!, $first: Int!, $after: String) {
	        issue(id: $issueId) {
	            id
	            comments(first: $first, after: $after, orderBy: createdAt) {
	                nodes {
	                    id url body createdAt updatedAt
	                    user { id name email }
	                }
	                pageInfo { hasNextPage hasPreviousPage startCursor endCursor }
	            }
	        }
	    }`
)

// Viewer returns the authenticated user. Useful for credential smoke tests.
func (c *Client) Viewer(ctx context.Context) (*User, error) {
	var data struct {
		Viewer User `json:"viewer"`
	}
	if err := c.Do(ctx, queryViewer, nil, &data); err != nil {
		return nil, err
	}
	return &data.Viewer, nil
}

// Users returns a paginated list of users in the workspace.
func (c *Client) Users(ctx context.Context, p PageOptions) ([]User, PageInfo, error) {
	return readConnection[User](ctx, c, queryUsers, "users", p)
}

// Teams returns a paginated list of teams in the workspace.
func (c *Client) Teams(ctx context.Context, p PageOptions) ([]Team, PageInfo, error) {
	return readConnection[Team](ctx, c, queryTeams, "teams", p)
}

// WorkflowStates returns a paginated list of workflow states in the workspace.
func (c *Client) WorkflowStates(ctx context.Context, p PageOptions) ([]WorkflowState, PageInfo, error) {
	return readConnection[WorkflowState](ctx, c, queryWorkflowStates, "workflowStates", p)
}

// Projects returns a paginated list of projects in the workspace.
func (c *Client) Projects(ctx context.Context, p PageOptions) ([]Project, PageInfo, error) {
	return readConnection[Project](ctx, c, queryProjects, "projects", p)
}

// Labels returns a paginated list of issue labels in the workspace.
func (c *Client) Labels(ctx context.Context, p PageOptions) ([]Label, PageInfo, error) {
	return readConnection[Label](ctx, c, queryLabels, "issueLabels", p)
}

// IssueLabel returns one label by UUID. Unknown labels return (nil, nil).
func (c *Client) IssueLabel(ctx context.Context, labelID string) (*Label, error) {
	if labelID == "" {
		return nil, fmt.Errorf("linear: IssueLabel: labelID required")
	}
	var data struct {
		IssueLabel *Label `json:"issueLabel"`
	}
	if err := c.Do(ctx, queryIssueLabel, map[string]interface{}{"id": labelID}, &data); err != nil {
		return nil, err
	}
	return data.IssueLabel, nil
}

// Issue accepts either a UUID or an identifier such as "ENG-123". Linear's
// issue(id:) field accepts both shapes.
func (c *Client) Issue(ctx context.Context, idOrIdentifier string) (*Issue, error) {
	if idOrIdentifier == "" {
		return nil, fmt.Errorf("linear: Issue: id required")
	}
	var data struct {
		Issue *issueWithLabels `json:"issue"`
	}
	vars := map[string]interface{}{"id": idOrIdentifier}
	if err := c.Do(ctx, queryIssue, vars, &data); err != nil {
		return nil, err
	}
	if data.Issue == nil {
		return nil, fmt.Errorf("linear: issue %q not found", idOrIdentifier)
	}
	return data.Issue.flatten(), nil
}

// Issues runs a paginated, filtered issue search. Empty filter fields are omitted.
func (c *Client) Issues(ctx context.Context, q IssueQuery) ([]Issue, PageInfo, error) {
	first, after := normalizePage(q.Page)
	vars := map[string]interface{}{"first": first}
	if after != "" {
		vars["after"] = after
	}
	filter := buildIssueFilter(q)
	if filter != nil {
		vars["filter"] = filter
	}
	if order := normalizeOrder(q.OrderBy); order != "" {
		vars["orderBy"] = order
	}

	var data struct {
		Issues struct {
			Nodes    []issueWithLabels `json:"nodes"`
			PageInfo PageInfo          `json:"pageInfo"`
		} `json:"issues"`
	}
	if err := c.Do(ctx, queryIssues, vars, &data); err != nil {
		return nil, PageInfo{}, err
	}

	issues := make([]Issue, len(data.Issues.Nodes))
	for i := range data.Issues.Nodes {
		issues[i] = *data.Issues.Nodes[i].flatten()
	}
	return issues, data.Issues.PageInfo, nil
}

// Comments returns a paginated list of comments for a single issue, ordered by
// createdAt ascending. issueID may be a UUID or identifier; the query uses
// issue(id:) which accepts both.
func (c *Client) Comments(ctx context.Context, issueID string, p PageOptions) ([]Comment, PageInfo, error) {
	if issueID == "" {
		return nil, PageInfo{}, fmt.Errorf("linear: Comments: issueID required")
	}
	first, after := normalizePage(p)
	vars := map[string]interface{}{"issueId": issueID, "first": first}
	if after != "" {
		vars["after"] = after
	}
	var data struct {
		Issue *struct {
			ID       string `json:"id"`
			Comments struct {
				Nodes    []Comment `json:"nodes"`
				PageInfo PageInfo  `json:"pageInfo"`
			} `json:"comments"`
		} `json:"issue"`
	}
	if err := c.Do(ctx, queryComments, vars, &data); err != nil {
		return nil, PageInfo{}, err
	}
	if data.Issue == nil {
		return nil, PageInfo{}, fmt.Errorf("linear: Comments: issue %q not found", issueID)
	}
	for i := range data.Issue.Comments.Nodes {
		data.Issue.Comments.Nodes[i].IssueID = data.Issue.ID
	}
	return data.Issue.Comments.Nodes, data.Issue.Comments.PageInfo, nil
}

func readConnection[T any](ctx context.Context, c *Client, query, field string, p PageOptions) ([]T, PageInfo, error) {
	first, after := normalizePage(p)
	vars := map[string]interface{}{"first": first}
	if after != "" {
		vars["after"] = after
	}

	var data map[string]json.RawMessage
	if err := c.Do(ctx, query, vars, &data); err != nil {
		return nil, PageInfo{}, err
	}
	raw, ok := data[field]
	if !ok {
		return nil, PageInfo{}, fmt.Errorf("linear: missing connection %q", field)
	}

	var conn struct {
		Nodes    []T      `json:"nodes"`
		PageInfo PageInfo `json:"pageInfo"`
	}
	if err := json.Unmarshal(raw, &conn); err != nil {
		return nil, PageInfo{}, fmt.Errorf("linear: decode connection %q: %w", field, err)
	}
	return conn.Nodes, conn.PageInfo, nil
}

func normalizePage(p PageOptions) (int, string) {
	n := p.First
	if n <= 0 {
		n = 25
	}
	if n > 50 {
		n = 50
	}
	return n, p.After
}

func buildIssueFilter(q IssueQuery) map[string]interface{} {
	f := map[string]interface{}{}
	if q.TeamID != "" {
		f["team"] = map[string]interface{}{"id": map[string]interface{}{"eq": q.TeamID}}
	}
	if q.StateID != "" {
		f["state"] = map[string]interface{}{"id": map[string]interface{}{"eq": q.StateID}}
	}
	if q.LabelID != "" {
		f["labels"] = map[string]interface{}{"id": map[string]interface{}{"eq": q.LabelID}}
	}
	if q.UpdatedSince != nil {
		f["updatedAt"] = map[string]interface{}{"gte": q.UpdatedSince.Format(time.RFC3339)}
	}
	if len(f) == 0 {
		return nil
	}
	return f
}

func normalizeOrder(s string) string {
	switch s {
	case "", "updatedAt":
		return "updatedAt"
	case "createdAt":
		return "createdAt"
	default:
		return ""
	}
}

type issueWithLabels struct {
	Issue
	LabelsConnection struct {
		Nodes []Label `json:"nodes"`
	} `json:"labels"`
}

func (i *issueWithLabels) flatten() *Issue {
	i.Issue.Labels = i.LabelsConnection.Nodes
	return &i.Issue
}
