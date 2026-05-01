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

type writeGraphQLRequest struct {
	Query     string                 `json:"query"`
	Variables map[string]interface{} `json:"variables"`
}

func TestWrite_CreateIssue_Success(t *testing.T) {
	var got writeGraphQLRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, json.NewDecoder(r.Body).Decode(&got))
		_, _ = w.Write([]byte(`{"data":{"issueCreate":{"success":true,"issue":{"id":"issue-1","identifier":"ENG-7","title":"X","url":"https://linear.app/acme/issue/ENG-7/x","priority":0,"createdAt":"2026-04-30T12:00:00Z","updatedAt":"2026-04-30T12:01:00Z","state":{"id":"state-1","name":"Todo","type":"backlog"}}}}}`))
	}))
	defer srv.Close()

	client := linear.NewClient(srv.URL, "test-key", srv.Client())
	issue, err := client.CreateIssue(context.Background(), linear.CreateIssueInput{Title: "X", TeamID: "T"})
	require.NoError(t, err)

	assert.Contains(t, got.Query, "mutation IssueCreate")
	require.Equal(t, map[string]interface{}{"title": "X", "teamId": "T"}, got.Variables["input"])
	require.NotNil(t, issue)
	assert.Equal(t, "ENG-7", issue.Identifier)
	assert.Equal(t, "https://linear.app/acme/issue/ENG-7/x", issue.URL)
	require.NotNil(t, issue.State)
	assert.Equal(t, "Todo", issue.State.Name)
	assert.False(t, issue.CreatedAt.IsZero())
}

func TestWrite_CreateIssue_AllFields(t *testing.T) {
	priority := 2
	var got writeGraphQLRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, json.NewDecoder(r.Body).Decode(&got))
		_, _ = w.Write([]byte(`{"data":{"issueCreate":{"success":true,"issue":{"id":"issue-1","identifier":"ENG-7","title":"X","url":"https://linear.app/acme/issue/ENG-7/x","priority":2,"createdAt":"2026-04-30T12:00:00Z","updatedAt":"2026-04-30T12:01:00Z"}}}}`))
	}))
	defer srv.Close()

	client := linear.NewClient(srv.URL, "test-key", srv.Client())
	_, err := client.CreateIssue(context.Background(), linear.CreateIssueInput{
		Title:       "X",
		TeamID:      "team-1",
		Description: "details",
		StateID:     "state-1",
		AssigneeID:  "user-1",
		ProjectID:   "project-1",
		Priority:    &priority,
		LabelIDs:    []string{"label-1", "label-2"},
	})
	require.NoError(t, err)

	input := got.Variables["input"].(map[string]interface{})
	assert.Equal(t, "X", input["title"])
	assert.Equal(t, "team-1", input["teamId"])
	assert.Equal(t, "details", input["description"])
	assert.Equal(t, "state-1", input["stateId"])
	assert.Equal(t, "user-1", input["assigneeId"])
	assert.Equal(t, "project-1", input["projectId"])
	assert.Equal(t, float64(2), input["priority"])
	assert.Equal(t, []interface{}{"label-1", "label-2"}, input["labelIds"])
}

func TestWrite_CreateIssue_TitleRequired(t *testing.T) {
	requests := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
	}))
	defer srv.Close()

	client := linear.NewClient(srv.URL, "test-key", srv.Client())
	_, err := client.CreateIssue(context.Background(), linear.CreateIssueInput{Title: " ", TeamID: "team-1"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "title required")
	assert.Equal(t, 0, requests)
}

func TestWrite_CreateIssue_TeamIDRequired(t *testing.T) {
	requests := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
	}))
	defer srv.Close()

	client := linear.NewClient(srv.URL, "test-key", srv.Client())
	_, err := client.CreateIssue(context.Background(), linear.CreateIssueInput{Title: "X", TeamID: "\t"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "teamID required")
	assert.Equal(t, 0, requests)
}

func TestWrite_UpdateIssue_SparseInput(t *testing.T) {
	title := "renamed"
	var got writeGraphQLRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, json.NewDecoder(r.Body).Decode(&got))
		_, _ = w.Write([]byte(`{"data":{"issueUpdate":{"success":true,"issue":{"id":"issue-1","identifier":"ENG-7","title":"renamed","url":"https://linear.app/acme/issue/ENG-7/x","priority":0,"updatedAt":"2026-04-30T12:01:00Z"}}}}`))
	}))
	defer srv.Close()

	client := linear.NewClient(srv.URL, "test-key", srv.Client())
	_, err := client.UpdateIssue(context.Background(), "issue-1", linear.UpdateIssueInput{Title: &title})
	require.NoError(t, err)

	assert.Equal(t, "issue-1", got.Variables["id"])
	input := got.Variables["input"].(map[string]interface{})
	assert.Equal(t, map[string]interface{}{"title": "renamed"}, input)
}

func TestWrite_UpdateIssue_LabelsCleared(t *testing.T) {
	labels := []string{}
	var got writeGraphQLRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, json.NewDecoder(r.Body).Decode(&got))
		_, _ = w.Write([]byte(`{"data":{"issueUpdate":{"success":true,"issue":{"id":"issue-1","identifier":"ENG-7","title":"X","url":"https://linear.app/acme/issue/ENG-7/x","priority":0,"updatedAt":"2026-04-30T12:01:00Z"}}}}`))
	}))
	defer srv.Close()

	client := linear.NewClient(srv.URL, "test-key", srv.Client())
	_, err := client.UpdateIssue(context.Background(), "issue-1", linear.UpdateIssueInput{LabelIDs: &labels})
	require.NoError(t, err)

	input := got.Variables["input"].(map[string]interface{})
	assert.Equal(t, []interface{}{}, input["labelIds"])
}

func TestWrite_RemoveLabelFromIssue(t *testing.T) {
	var got writeGraphQLRequest
	requests := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		require.NoError(t, json.NewDecoder(r.Body).Decode(&got))
		_, _ = w.Write([]byte(`{"data":{"issueUpdate":{"success":true,"issue":{"id":"issue-1","identifier":"ENG-7","title":"X","url":"https://linear.app/acme/issue/ENG-7/x","priority":0,"updatedAt":"2026-04-30T12:01:00Z"}}}}`))
	}))
	defer srv.Close()

	client := linear.NewClient(srv.URL, "test-key", srv.Client())
	err := client.RemoveLabelFromIssue(context.Background(), "issue-1", []string{"label-keep"})
	require.NoError(t, err)

	assert.Equal(t, 1, requests)
	assert.Contains(t, got.Query, "mutation IssueUpdate")
	assert.Equal(t, "issue-1", got.Variables["id"])
	input := got.Variables["input"].(map[string]interface{})
	assert.Equal(t, []interface{}{"label-keep"}, input["labelIds"])
}

func TestWrite_RemoveLabelFromIssueAllowsEmptySurvivors(t *testing.T) {
	var got writeGraphQLRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, json.NewDecoder(r.Body).Decode(&got))
		_, _ = w.Write([]byte(`{"data":{"issueUpdate":{"success":true,"issue":{"id":"issue-1","identifier":"ENG-7","title":"X","url":"https://linear.app/acme/issue/ENG-7/x","priority":0,"updatedAt":"2026-04-30T12:01:00Z"}}}}`))
	}))
	defer srv.Close()

	client := linear.NewClient(srv.URL, "test-key", srv.Client())
	err := client.RemoveLabelFromIssue(context.Background(), "issue-1", []string{})
	require.NoError(t, err)

	input := got.Variables["input"].(map[string]interface{})
	assert.Equal(t, []interface{}{}, input["labelIds"])
}

func TestWrite_UpdateIssue_NoFieldsError(t *testing.T) {
	requests := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
	}))
	defer srv.Close()

	client := linear.NewClient(srv.URL, "test-key", srv.Client())
	_, err := client.UpdateIssue(context.Background(), "issue-1", linear.UpdateIssueInput{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "at least one field")
	assert.Equal(t, 0, requests)
}

func TestWrite_UpdateIssue_AcceptsIdentifier(t *testing.T) {
	title := "renamed"
	var got writeGraphQLRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, json.NewDecoder(r.Body).Decode(&got))
		_, _ = w.Write([]byte(`{"data":{"issueUpdate":{"success":true,"issue":{"id":"issue-1","identifier":"ENG-7","title":"renamed","url":"https://linear.app/acme/issue/ENG-7/x","priority":0,"updatedAt":"2026-04-30T12:01:00Z"}}}}`))
	}))
	defer srv.Close()

	client := linear.NewClient(srv.URL, "test-key", srv.Client())
	_, err := client.UpdateIssue(context.Background(), "ENG-7", linear.UpdateIssueInput{Title: &title})
	require.NoError(t, err)
	assert.Equal(t, "ENG-7", got.Variables["id"])
}

func TestWrite_CreateIssue_SuccessFalse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":{"issueCreate":{"success":false,"issue":null}}}`))
	}))
	defer srv.Close()

	client := linear.NewClient(srv.URL, "test-key", srv.Client())
	_, err := client.CreateIssue(context.Background(), linear.CreateIssueInput{Title: "X", TeamID: "team-1"})
	require.Error(t, err)

	var mutationErr *linear.MutationFailedError
	require.True(t, errors.As(err, &mutationErr))
	assert.Equal(t, "issueCreate", mutationErr.OperationName)
}

func TestWrite_CreateComment_Success(t *testing.T) {
	var got writeGraphQLRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, json.NewDecoder(r.Body).Decode(&got))
		_, _ = w.Write([]byte(`{"data":{"commentCreate":{"success":true,"comment":{"id":"comment-1","url":"https://linear.app/acme/comment/comment-1","body":"hello"}}}}`))
	}))
	defer srv.Close()

	client := linear.NewClient(srv.URL, "test-key", srv.Client())
	comment, err := client.CreateComment(context.Background(), "issue-1", "hello")
	require.NoError(t, err)

	assert.Contains(t, got.Query, "mutation CommentCreate")
	require.Equal(t, map[string]interface{}{"issueId": "issue-1", "body": "hello"}, got.Variables["input"])
	require.NotNil(t, comment)
	assert.Equal(t, "comment-1", comment.ID)
	assert.Equal(t, "https://linear.app/acme/comment/comment-1", comment.URL)
	assert.Equal(t, "hello", comment.Body)
}

func TestWrite_CreateComment_BodyRequired(t *testing.T) {
	requests := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
	}))
	defer srv.Close()

	client := linear.NewClient(srv.URL, "test-key", srv.Client())
	_, err := client.CreateComment(context.Background(), "issue-1", " ")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "body required")
	assert.Equal(t, 0, requests)
}

func TestWrite_CreateComment_IssueIDRequired(t *testing.T) {
	requests := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
	}))
	defer srv.Close()

	client := linear.NewClient(srv.URL, "test-key", srv.Client())
	_, err := client.CreateComment(context.Background(), "\n", "hello")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "issueID required")
	assert.Equal(t, 0, requests)
}

func TestWrite_GraphQLErrorPropagated(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":{"issueCreate":{"success":false,"issue":null}},"errors":[{"message":"invalid input","path":["issueCreate"],"extensions":{"code":"BAD_USER_INPUT"}}]}`))
	}))
	defer srv.Close()

	client := linear.NewClient(srv.URL, "test-key", srv.Client())
	_, err := client.CreateIssue(context.Background(), linear.CreateIssueInput{Title: "X", TeamID: "team-1"})
	require.Error(t, err)

	var gqlErrs *linear.GraphQLErrors
	require.True(t, errors.As(err, &gqlErrs))
	var mutationErr *linear.MutationFailedError
	assert.False(t, errors.As(err, &mutationErr))
}

func TestWrite_RateLimitPropagated(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`rate limited`))
	}))
	defer srv.Close()

	client := linear.NewClient(srv.URL, "test-key", srv.Client())
	_, err := client.CreateComment(context.Background(), "issue-1", "hello")
	require.Error(t, err)

	var rateLimitErr *linear.RateLimitError
	require.True(t, errors.As(err, &rateLimitErr))
	assert.Equal(t, http.StatusTooManyRequests, rateLimitErr.StatusCode)
}

func TestWrite_CreateCommentReactionSuccess(t *testing.T) {
	var got writeGraphQLRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, json.NewDecoder(r.Body).Decode(&got))
		_, _ = w.Write([]byte(`{"data":{"reactionCreate":{"success":true,"reaction":{"id":"reaction-1","emoji":"eyes"}}}}`))
	}))
	defer srv.Close()

	client := linear.NewClient(srv.URL, "test-key", srv.Client())
	err := client.CreateCommentReaction(context.Background(), "comment-1", "eyes")
	require.NoError(t, err)

	assert.Contains(t, got.Query, "mutation CommentReactionCreate")
	require.Equal(t, map[string]interface{}{"commentId": "comment-1", "emoji": "eyes"}, got.Variables["input"])
}

func TestWrite_CreateCommentReactionFeatureNotAccessibleGraphQL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"errors":[{"message":"feature unavailable","extensions":{"code":"FEATURE_NOT_ACCESSIBLE"}}]}`))
	}))
	defer srv.Close()

	client := linear.NewClient(srv.URL, "test-key", srv.Client())
	err := client.CreateCommentReaction(context.Background(), "comment-1", "eyes")
	require.Error(t, err)

	var unsupported *linear.ReactionsUnsupportedError
	require.True(t, errors.As(err, &unsupported))
}

func TestWrite_CreateCommentReactionFeatureNotAccessibleUserError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":{"reactionCreate":{"success":false,"reaction":null,"userErrors":[{"code":"FEATURE_NOT_ACCESSIBLE","message":"upgrade required"}]}}}`))
	}))
	defer srv.Close()

	client := linear.NewClient(srv.URL, "test-key", srv.Client())
	err := client.CreateCommentReaction(context.Background(), "comment-1", "eyes")
	require.Error(t, err)

	var unsupported *linear.ReactionsUnsupportedError
	require.True(t, errors.As(err, &unsupported))
	assert.Contains(t, unsupported.Error(), "upgrade required")
}

func TestWrite_CreateCommentReactionHTTP403Unsupported(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`forbidden`))
	}))
	defer srv.Close()

	client := linear.NewClient(srv.URL, "test-key", srv.Client())
	err := client.CreateCommentReaction(context.Background(), "comment-1", "eyes")
	require.Error(t, err)

	var unsupported *linear.ReactionsUnsupportedError
	require.True(t, errors.As(err, &unsupported))
}

func TestWrite_CreateCommentReactionRateLimitPropagated(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`rate limited`))
	}))
	defer srv.Close()

	client := linear.NewClient(srv.URL, "test-key", srv.Client())
	err := client.CreateCommentReaction(context.Background(), "comment-1", "eyes")
	require.Error(t, err)

	var rateLimitErr *linear.RateLimitError
	require.True(t, errors.As(err, &rateLimitErr))
	assert.Equal(t, http.StatusTooManyRequests, rateLimitErr.StatusCode)
	var unsupported *linear.ReactionsUnsupportedError
	assert.False(t, errors.As(err, &unsupported))
}

func TestWrite_CreateCommentReactionHTTPErrorPropagated(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(`bad gateway`))
	}))
	defer srv.Close()

	client := linear.NewClient(srv.URL, "test-key", srv.Client())
	err := client.CreateCommentReaction(context.Background(), "comment-1", "eyes")
	require.Error(t, err)

	var httpErr *linear.HTTPError
	require.True(t, errors.As(err, &httpErr))
	assert.Equal(t, http.StatusBadGateway, httpErr.StatusCode)
	var unsupported *linear.ReactionsUnsupportedError
	assert.False(t, errors.As(err, &unsupported))
}
