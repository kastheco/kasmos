package lineartrigger

import (
	"context"
	"testing"
	"time"

	"github.com/kastheco/kasmos/config/auditlog"
	"github.com/kastheco/kasmos/config/linearlink"
	"github.com/kastheco/kasmos/config/taskstore"
	"github.com/kastheco/kasmos/internal/linear"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPollerPollOnceIdempotentLabelCreateAcrossTwoCycles(t *testing.T) {
	ctx := context.Background()
	h := newPollerHarness(t)
	h.linear.issues = []linear.Issue{testIssue("lin-create")}

	stats := h.poller.PollOnce(ctx)
	require.False(t, stats.Aborted, "unexpected poll error: %v", stats.Err)
	stats = h.poller.PollOnce(ctx)
	require.False(t, stats.Aborted, "unexpected poll error: %v", stats.Err)

	entries, err := h.store.ListByStatus("proj", taskstore.StatusReady)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "eng-1", entries[0].Filename)
}

func TestPollerPollOnceIdempotentCommentPlanAcrossTwoCycles(t *testing.T) {
	ctx := context.Background()
	h := newPollerHarness(t)
	require.NoError(t, h.store.Create("proj", taskstore.TaskEntry{
		Filename:         "eng-1",
		Status:           taskstore.StatusReady,
		Content:          "# plan\n\n## Wave 1\n\n### Task 1: test\n",
		LinearIssueID:    "lin-plan",
		LinearIdentifier: "ENG-1",
		Branch:           "linear/eng-1",
	}))
	h.linear.byID["lin-plan"] = testIssue("lin-plan")
	h.linear.comments["lin-plan"] = []linear.Comment{{
		ID:        "comment-1",
		Body:      "/kasmos plan",
		CreatedAt: h.now.Add(time.Minute),
		User:      &linear.User{ID: "actor", Email: "actor@example.com"},
	}}

	stats := h.poller.PollOnce(ctx)
	require.False(t, stats.Aborted, "unexpected poll error: %v", stats.Err)
	stats = h.poller.PollOnce(ctx)
	require.False(t, stats.Aborted, "unexpected poll error: %v", stats.Err)

	signals, err := h.gateway.List("proj", taskstore.SignalPending)
	require.NoError(t, err)
	require.Len(t, signals, 1)
	assert.Equal(t, "plan_start", signals[0].SignalType)
	assert.Equal(t, "eng-1", signals[0].PlanFile)
}

func TestPollerPollOnceRateLimitAbortsWithoutPartialEnqueue(t *testing.T) {
	ctx := context.Background()
	h := newPollerHarness(t)
	h.linear.errIssues = &linear.RateLimitError{StatusCode: 429}

	stats := h.poller.PollOnce(ctx)

	assert.True(t, stats.Aborted)
	triggers, err := h.store.ListUnprocessedLinearTriggers("proj", 10)
	require.NoError(t, err)
	assert.Empty(t, triggers)
}

func TestPollerPollOnceReactionUnsupportedFallsBackToComment(t *testing.T) {
	ctx := context.Background()
	h := newPollerHarness(t)
	require.NoError(t, h.store.Create("proj", taskstore.TaskEntry{
		Filename:         "eng-1",
		Status:           taskstore.StatusReady,
		Content:          "# plan\n\n## Wave 1\n\n### Task 1: test\n",
		LinearIssueID:    "lin-plan",
		LinearIdentifier: "ENG-1",
	}))
	h.linear.byID["lin-plan"] = testIssue("lin-plan")
	h.linear.comments["lin-plan"] = []linear.Comment{{
		ID:        "comment-1",
		Body:      "/kasmos status",
		CreatedAt: h.now.Add(time.Minute),
		User:      &linear.User{ID: "other"},
	}}
	h.linear.errReaction = &linear.ReactionsUnsupportedError{}

	stats := h.poller.PollOnce(ctx)
	require.False(t, stats.Aborted, "unexpected poll error: %v", stats.Err)

	require.Len(t, h.linear.createdComments, 1)
	assert.Contains(t, h.linear.createdComments[0].body, "kasmos trigger status")
}

func TestPollerPollOnceLastSeenCommentAdvancesMonotonically(t *testing.T) {
	ctx := context.Background()
	h := newPollerHarness(t)
	require.NoError(t, h.store.Create("proj", taskstore.TaskEntry{
		Filename:         "eng-1",
		Status:           taskstore.StatusReady,
		LinearIssueID:    "lin-cursor",
		LinearIdentifier: "ENG-1",
	}))
	first := h.now.Add(time.Minute)
	second := h.now.Add(2 * time.Minute)
	h.linear.comments["lin-cursor"] = []linear.Comment{
		{ID: "c2", Body: "ordinary", CreatedAt: second},
		{ID: "c1", Body: "ordinary", CreatedAt: first},
	}

	stats := h.poller.PollOnce(ctx)
	require.False(t, stats.Aborted, "unexpected poll error: %v", stats.Err)
	got, err := h.store.LastSeenCommentAt("proj", "lin-cursor")
	require.NoError(t, err)
	assert.Equal(t, second.UTC(), got.UTC())

	h.linear.comments["lin-cursor"] = nil
	stats = h.poller.PollOnce(ctx)
	require.False(t, stats.Aborted, "unexpected poll error: %v", stats.Err)
	gotAgain, err := h.store.LastSeenCommentAt("proj", "lin-cursor")
	require.NoError(t, err)
	assert.Equal(t, second.UTC(), gotAgain.UTC())
}

type pollerHarness struct {
	store   *taskstore.SQLiteStore
	gateway taskstore.SignalGateway
	linear  *fakeLinearClient
	poller  *Poller
	now     time.Time
}

func newPollerHarness(t *testing.T) *pollerHarness {
	t.Helper()
	store, err := taskstore.NewSQLiteStore(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })
	gateway, err := taskstore.NewSQLiteSignalGateway(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = gateway.Close() })
	now := time.Date(2026, 4, 30, 12, 0, 0, 0, time.UTC)
	client := newFakeLinearClient()
	cfg := testConfig()
	linker := linearlink.New(store, client, auditlog.NopLogger(), "proj")
	poller := NewPoller(PollerDeps{
		Project: "proj",
		Config:  cfg,
		Store:   store,
		Linker:  linker,
		Linear:  client,
		Gateway: gateway,
		Audit:   auditlog.NopLogger(),
		Now:     func() time.Time { return now },
	})
	return &pollerHarness{store: store, gateway: gateway, linear: client, poller: poller, now: now}
}

type fakeLinearClient struct {
	issues          []linear.Issue
	byID            map[string]linear.Issue
	comments        map[string][]linear.Comment
	issueCalls      map[string]int
	createdComments []struct {
		issueID string
		body    string
	}
	errIssues   error
	errReaction error
}

func newFakeLinearClient() *fakeLinearClient {
	return &fakeLinearClient{
		byID:       map[string]linear.Issue{},
		comments:   map[string][]linear.Comment{},
		issueCalls: map[string]int{},
	}
}

func (f *fakeLinearClient) Issue(_ context.Context, idOrIdentifier string) (*linear.Issue, error) {
	f.issueCalls[idOrIdentifier]++
	if issue, ok := f.byID[idOrIdentifier]; ok {
		return &issue, nil
	}
	for _, issue := range f.issues {
		if issue.ID == idOrIdentifier || issue.Identifier == idOrIdentifier {
			return &issue, nil
		}
	}
	issue := testIssue(idOrIdentifier)
	return &issue, nil
}

func (f *fakeLinearClient) Issues(_ context.Context, q linear.IssueQuery) ([]linear.Issue, linear.PageInfo, error) {
	if f.errIssues != nil {
		return nil, linear.PageInfo{}, f.errIssues
	}
	out := []linear.Issue{}
	for _, issue := range f.issues {
		if q.LabelID != "" && !issueHasLabel(issue, q.LabelID) {
			continue
		}
		out = append(out, issue)
	}
	return out, linear.PageInfo{}, nil
}

func (f *fakeLinearClient) Comments(_ context.Context, issueID string, _ linear.PageOptions) ([]linear.Comment, linear.PageInfo, error) {
	return append([]linear.Comment(nil), f.comments[issueID]...), linear.PageInfo{}, nil
}

func (f *fakeLinearClient) IssueLabel(_ context.Context, labelID string) (*linear.Label, error) {
	return &linear.Label{ID: labelID}, nil
}

func (f *fakeLinearClient) RemoveLabelFromIssue(_ context.Context, _ string, _ []string) error {
	return nil
}

func (f *fakeLinearClient) CreateComment(_ context.Context, issueID, body string) (*linear.Comment, error) {
	f.createdComments = append(f.createdComments, struct {
		issueID string
		body    string
	}{issueID: issueID, body: body})
	return &linear.Comment{ID: "created"}, nil
}

func (f *fakeLinearClient) CreateCommentReaction(_ context.Context, _ string, _ string) error {
	return f.errReaction
}

func (f *fakeLinearClient) UpdateIssue(_ context.Context, issueID string, _ linear.UpdateIssueInput) (*linear.Issue, error) {
	issue, _ := f.Issue(context.Background(), issueID)
	return issue, nil
}
