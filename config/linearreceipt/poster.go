package linearreceipt

import (
	"context"

	"github.com/kastheco/kasmos/internal/linear"
)

// CommentPoster is the subset of internal/linear.Client used by the receipt hook.
type CommentPoster interface {
	CreateComment(ctx context.Context, issueID, body string) (*linear.Comment, error)
}

// IssueStateMutator is the subset used for opt-in workflow-state mutation.
type IssueStateMutator interface {
	UpdateIssue(ctx context.Context, issueID string, in linear.UpdateIssueInput) (*linear.Issue, error)
}

// ClientAdapter combines both seams -- the live *linear.Client satisfies it.
type ClientAdapter interface {
	CommentPoster
	IssueStateMutator
}

var _ ClientAdapter = (*linear.Client)(nil)

// NewClientFromEnv returns a ClientAdapter built from KASMOS_LINEAR_API_KEY /
// LINEAR_API_KEY via linear.ConfigFromEnv. Returns (nil, linear.ErrNotConfigured)
// when no key is set so callers can degrade silently.
func NewClientFromEnv() (ClientAdapter, error) {
	cfg, err := linear.ConfigFromEnv()
	if err != nil {
		return nil, err
	}
	return linear.NewClientFromConfig(cfg), nil
}
