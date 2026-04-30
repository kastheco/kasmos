package linear_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kastheco/kasmos/internal/linear"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type readRequest struct {
	Query     string                 `json:"query"`
	Variables map[string]interface{} `json:"variables"`
}

func newReadServer(t *testing.T, handler func(http.ResponseWriter, *http.Request, readRequest)) (*httptest.Server, *[]readRequest) {
	t.Helper()
	var requests []readRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var got readRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&got))
		requests = append(requests, got)
		w.Header().Set("Content-Type", "application/json")
		handler(w, r, got)
	}))
	return srv, &requests
}

func TestRead_Viewer(t *testing.T) {
	srv, _ := newReadServer(t, func(w http.ResponseWriter, _ *http.Request, _ readRequest) {
		_, _ = w.Write([]byte(`{"data":{"viewer":{"id":"u_1","name":"x","email":"e"}}}`))
	})
	defer srv.Close()

	client := linear.NewClient(srv.URL, "test-key", srv.Client())
	viewer, err := client.Viewer(context.Background())
	require.NoError(t, err)
	assert.Equal(t, &linear.User{ID: "u_1", Name: "x", Email: "e"}, viewer)
}

func TestRead_TeamsPagination(t *testing.T) {
	srv, requests := newReadServer(t, func(w http.ResponseWriter, _ *http.Request, _ readRequest) {
		_, _ = w.Write([]byte(`{"data":{"teams":{"nodes":[{"id":"team_1","key":"ENG","name":"engineering"}],"pageInfo":{"hasNextPage":false,"hasPreviousPage":false,"startCursor":"s","endCursor":"e"}}}}`))
	})
	defer srv.Close()

	client := linear.NewClient(srv.URL, "test-key", srv.Client())
	_, _, err := client.Teams(context.Background(), linear.PageOptions{})
	require.NoError(t, err)
	_, _, err = client.Teams(context.Background(), linear.PageOptions{First: 100, After: "cursor_1"})
	require.NoError(t, err)

	require.Len(t, *requests, 2)
	assert.Equal(t, float64(25), (*requests)[0].Variables["first"])
	assert.NotContains(t, (*requests)[0].Variables, "after")
	assert.Equal(t, float64(50), (*requests)[1].Variables["first"])
	assert.Equal(t, "cursor_1", (*requests)[1].Variables["after"])
}

func TestRead_WorkflowStates_PageInfo(t *testing.T) {
	srv, _ := newReadServer(t, func(w http.ResponseWriter, _ *http.Request, _ readRequest) {
		_, _ = w.Write([]byte(`{"data":{"workflowStates":{"nodes":[{"id":"state_1","name":"started","type":"started"}],"pageInfo":{"hasNextPage":true,"hasPreviousPage":false,"startCursor":"start","endCursor":"end"}}}}`))
	})
	defer srv.Close()

	client := linear.NewClient(srv.URL, "test-key", srv.Client())
	states, page, err := client.WorkflowStates(context.Background(), linear.PageOptions{First: 1})
	require.NoError(t, err)
	require.Len(t, states, 1)
	assert.Equal(t, "end", page.EndCursor)
	assert.True(t, page.HasNextPage)
}

func TestRead_IssueByIdentifier(t *testing.T) {
	srv, requests := newReadServer(t, func(w http.ResponseWriter, _ *http.Request, _ readRequest) {
		_, _ = w.Write([]byte(`{"data":{"issue":{"id":"issue_1","identifier":"ENG-123","title":"ship reads","description":"body","url":"https://linear.app/acme/issue/ENG-123","priority":2,"createdAt":"2026-04-29T10:00:00Z","updatedAt":"2026-04-30T10:00:00Z","state":{"id":"state_1","name":"started","type":"started"},"team":{"id":"team_1","key":"ENG","name":"engineering"},"project":{"id":"project_1","name":"phase 1","url":"https://linear.app/acme/project/phase-1"},"assignee":{"id":"user_1","name":"x","email":"x@example.com"},"labels":{"nodes":[{"id":"label_1","name":"api","color":"#ff0000"}]}}}}`))
	})
	defer srv.Close()

	client := linear.NewClient(srv.URL, "test-key", srv.Client())
	issue, err := client.Issue(context.Background(), "ENG-123")
	require.NoError(t, err)

	require.Len(t, *requests, 1)
	assert.Equal(t, map[string]interface{}{"id": "ENG-123"}, (*requests)[0].Variables)
	assert.Equal(t, "issue_1", issue.ID)
	assert.Equal(t, "ENG-123", issue.Identifier)
	require.NotNil(t, issue.State)
	assert.Equal(t, "started", issue.State.Type)
	require.NotNil(t, issue.Team)
	assert.Equal(t, "ENG", issue.Team.Key)
	require.NotNil(t, issue.Project)
	assert.Equal(t, "phase 1", issue.Project.Name)
	require.NotNil(t, issue.Assignee)
	assert.Equal(t, "x@example.com", issue.Assignee.Email)
	assert.Equal(t, []linear.Label{{ID: "label_1", Name: "api", Color: "#ff0000"}}, issue.Labels)
}

func TestRead_IssueNotFound(t *testing.T) {
	srv, _ := newReadServer(t, func(w http.ResponseWriter, _ *http.Request, _ readRequest) {
		_, _ = w.Write([]byte(`{"data":{"issue":null}}`))
	})
	defer srv.Close()

	client := linear.NewClient(srv.URL, "test-key", srv.Client())
	issue, err := client.Issue(context.Background(), "ENG-404")
	require.Error(t, err)
	assert.Nil(t, issue)
	assert.Contains(t, err.Error(), "ENG-404")
}

func TestRead_IssuesFilterAndOrder(t *testing.T) {
	srv, requests := newReadServer(t, func(w http.ResponseWriter, _ *http.Request, _ readRequest) {
		_, _ = w.Write([]byte(`{"data":{"issues":{"nodes":[],"pageInfo":{"hasNextPage":false,"hasPreviousPage":false,"startCursor":"","endCursor":""}}}}`))
	})
	defer srv.Close()

	client := linear.NewClient(srv.URL, "test-key", srv.Client())
	_, _, err := client.Issues(context.Background(), linear.IssueQuery{
		TeamID:  "team_1",
		StateID: "state_1",
		OrderBy: "createdAt",
	})
	require.NoError(t, err)

	require.Len(t, *requests, 1)
	vars := (*requests)[0].Variables
	assert.Equal(t, float64(25), vars["first"])
	assert.Equal(t, "createdAt", vars["orderBy"])
	assert.Equal(t, map[string]interface{}{
		"team":  map[string]interface{}{"id": map[string]interface{}{"eq": "team_1"}},
		"state": map[string]interface{}{"id": map[string]interface{}{"eq": "state_1"}},
	}, vars["filter"])
}

func TestRead_IssuesEmptyConnection(t *testing.T) {
	srv, _ := newReadServer(t, func(w http.ResponseWriter, _ *http.Request, _ readRequest) {
		_, _ = w.Write([]byte(`{"data":{"issues":{"nodes":[],"pageInfo":{"hasNextPage":false,"hasPreviousPage":true,"startCursor":"s","endCursor":"e"}}}}`))
	})
	defer srv.Close()

	client := linear.NewClient(srv.URL, "test-key", srv.Client())
	issues, page, err := client.Issues(context.Background(), linear.IssueQuery{})
	require.NoError(t, err)
	assert.Empty(t, issues)
	assert.False(t, page.HasNextPage)
	assert.True(t, page.HasPreviousPage)
}

func TestRead_GraphQLErrorPropagated(t *testing.T) {
	srv, _ := newReadServer(t, func(w http.ResponseWriter, _ *http.Request, _ readRequest) {
		_, _ = w.Write([]byte(`{"errors":[{"message":"boom"}]}`))
	})
	defer srv.Close()

	client := linear.NewClient(srv.URL, "test-key", srv.Client())
	_, err := client.Viewer(context.Background())
	require.Error(t, err)

	var gqlErrs *linear.GraphQLErrors
	assert.True(t, errors.As(err, &gqlErrs))
}

func TestRead_NoAutoPagination(t *testing.T) {
	srv, requests := newReadServer(t, func(w http.ResponseWriter, _ *http.Request, _ readRequest) {
		_, _ = w.Write([]byte(`{"data":{"issues":{"nodes":[{"id":"issue_1","identifier":"ENG-1","title":"one","description":"","url":"https://linear.app/acme/issue/ENG-1","priority":0,"createdAt":"2026-04-29T10:00:00Z","updatedAt":"2026-04-30T10:00:00Z","labels":{"nodes":[]}}],"pageInfo":{"hasNextPage":true,"hasPreviousPage":false,"startCursor":"s","endCursor":"e"}}}}`))
	})
	defer srv.Close()

	client := linear.NewClient(srv.URL, "test-key", srv.Client())
	issues, page, err := client.Issues(context.Background(), linear.IssueQuery{Page: linear.PageOptions{First: 1}})
	require.NoError(t, err)
	require.Len(t, issues, 1)
	assert.True(t, page.HasNextPage)
	assert.Len(t, *requests, 1)
}
