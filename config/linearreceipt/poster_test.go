package linearreceipt

import (
	"context"
	"errors"
	"testing"

	"github.com/kastheco/kasmos/internal/linear"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewClientFromEnvReturnsErrNotConfiguredWithoutKeys(t *testing.T) {
	t.Setenv("KASMOS_LINEAR_API_KEY", "")
	t.Setenv("LINEAR_API_KEY", "")

	client, err := NewClientFromEnv()

	require.Nil(t, client)
	require.Error(t, err)
	assert.True(t, errors.Is(err, linear.ErrNotConfigured))
}

func TestNewClientFromEnvReturnsClientAdapterWithAPIKey(t *testing.T) {
	t.Setenv("KASMOS_LINEAR_API_KEY", "test-key")
	t.Setenv("LINEAR_API_KEY", "")

	client, err := NewClientFromEnv()

	require.NoError(t, err)
	require.NotNil(t, client)
	assert.Implements(t, (*CommentPoster)(nil), client)
	assert.Implements(t, (*IssueStateMutator)(nil), client)
}

func TestCommentPosterRecorderCapturesCreateCommentCalls(t *testing.T) {
	recorder := &commentPosterRecorder{}

	comment, err := recorder.CreateComment(context.Background(), "issue-123", "receipt body")

	require.NoError(t, err)
	assert.Equal(t, &linear.Comment{ID: "comment-1"}, comment)
	assert.Equal(t, 1, recorder.calls)
	assert.Equal(t, "issue-123", recorder.issueID)
	assert.Equal(t, "receipt body", recorder.body)
}

type commentPosterRecorder struct {
	calls   int
	issueID string
	body    string
}

func (r *commentPosterRecorder) CreateComment(_ context.Context, issueID, body string) (*linear.Comment, error) {
	r.calls++
	r.issueID = issueID
	r.body = body
	return &linear.Comment{ID: "comment-1"}, nil
}

var _ CommentPoster = (*commentPosterRecorder)(nil)
