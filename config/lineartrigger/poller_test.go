package lineartrigger

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
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

func TestPollerPollOnceUnlinkedLabelPlanCreatesAndPlansOnce(t *testing.T) {
	ctx := context.Background()
	h := newPollerHarness(t)
	h.poller.deps.Config.Labels.Create = ""
	h.linear.issues = []linear.Issue{{
		ID:          "lin-label-plan",
		Identifier:  "ENG-9",
		Title:       "Plan from label",
		Description: "issue body",
		URL:         "https://linear.local/ENG-9",
		Team:        &linear.Team{ID: "team-1", Key: "ENG"},
		Labels:      []linear.Label{{ID: "label-plan", Name: "kasmos-plan"}},
	}}

	stats := h.poller.PollOnce(ctx)
	require.False(t, stats.Aborted, "unexpected poll error: %v", stats.Err)
	stats = h.poller.PollOnce(ctx)
	require.False(t, stats.Aborted, "unexpected poll error: %v", stats.Err)

	entries, err := h.store.ListByStatus("proj", taskstore.StatusReady)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "eng-9", entries[0].Filename)
	signals, err := h.gateway.List("proj", taskstore.SignalPending)
	require.NoError(t, err)
	require.Len(t, signals, 1)
	assert.Equal(t, "plan_start", signals[0].SignalType)
	assert.Equal(t, "eng-9", signals[0].PlanFile)
	require.Len(t, h.linear.removedLabels, 1)
	assert.Equal(t, "lin-label-plan", h.linear.removedLabels[0].issueID)
	h.requireAuditDetailCount(t, auditlog.EventTaskLinearTriggerDispatched, "plan", 1)
}

func TestPollerPollOnceUnlinkedCommentPlanWithTaskArgCreatesAndPlansOnce(t *testing.T) {
	ctx := context.Background()
	h := newPollerHarness(t)
	h.linear.byID["lin-comment-plan"] = linear.Issue{
		ID:          "lin-comment-plan",
		Identifier:  "ENG-10",
		Title:       "Plan from comment",
		Description: "issue body",
		URL:         "https://linear.local/ENG-10",
		Team:        &linear.Team{ID: "team-1", Key: "ENG"},
	}
	_, queued, err := h.store.EnqueueLinearTrigger("proj", taskstore.LinearTriggerEntry{
		LinearIssueID:    "lin-comment-plan",
		LinearIdentifier: "ENG-10",
		CommandKind:      string(VerbPlan),
		SourceKind:       string(SourceComment),
		SourceID:         "comment-plan-custom",
		ActorID:          "actor",
		TaskArg:          "custom-plan",
		DetectedAt:       h.now,
	})
	require.NoError(t, err)
	require.True(t, queued)

	stats := h.poller.PollOnce(ctx)
	require.False(t, stats.Aborted, "unexpected poll error: %v", stats.Err)
	stats = h.poller.PollOnce(ctx)
	require.False(t, stats.Aborted, "unexpected poll error: %v", stats.Err)

	entry, err := h.store.Get("proj", "custom-plan")
	require.NoError(t, err)
	assert.Equal(t, "lin-comment-plan", entry.LinearIssueID)
	signals, err := h.gateway.List("proj", taskstore.SignalPending)
	require.NoError(t, err)
	require.Len(t, signals, 1)
	assert.Equal(t, "plan_start", signals[0].SignalType)
	assert.Equal(t, "custom-plan", signals[0].PlanFile)
	require.Len(t, h.linear.reactions, 1)
	assert.Equal(t, "comment-plan-custom", h.linear.reactions[0].commentID)
	h.requireAuditDetailCount(t, auditlog.EventTaskLinearTriggerDispatched, "plan", 1)
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

func TestPollerPollOnceReadOnlyRepliesPostCommentWhenReactionSucceeds(t *testing.T) {
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
	h.linear.comments["lin-plan"] = []linear.Comment{
		{
			ID:        "comment-help",
			Body:      "/kasmos help",
			CreatedAt: h.now.Add(time.Minute),
			User:      &linear.User{ID: "actor"},
		},
		{
			ID:        "comment-status",
			Body:      "/kasmos status",
			CreatedAt: h.now.Add(2 * time.Minute),
			User:      &linear.User{ID: "actor"},
		},
	}

	stats := h.poller.PollOnce(ctx)
	require.False(t, stats.Aborted, "unexpected poll error: %v", stats.Err)

	require.Len(t, h.linear.reactions, 2)
	require.Len(t, h.linear.createdComments, 2)
	assert.Contains(t, h.linear.createdComments[0].body, "kasmos trigger help")
	assert.Contains(t, h.linear.createdComments[1].body, "kasmos trigger status")
}

func TestPollerPollOncePollsTerminalLinkedTasksForStatusComments(t *testing.T) {
	ctx := context.Background()
	h := newPollerHarness(t)
	require.NoError(t, h.store.Create("proj", taskstore.TaskEntry{
		Filename:         "eng-1",
		Status:           taskstore.StatusDone,
		LinearIssueID:    "lin-done",
		LinearIdentifier: "ENG-1",
	}))
	h.linear.byID["lin-done"] = testIssue("lin-done")
	h.linear.comments["lin-done"] = []linear.Comment{{
		ID:        "comment-status-done",
		Body:      "/kasmos status",
		CreatedAt: h.now.Add(time.Minute),
		User:      &linear.User{ID: "other"},
	}}

	stats := h.poller.PollOnce(ctx)
	require.False(t, stats.Aborted, "unexpected poll error: %v", stats.Err)

	require.Len(t, h.linear.createdComments, 1)
	assert.Contains(t, h.linear.createdComments[0].body, "status: done")
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

func TestPollerPollOncePreservesStartGuardLabelWhenLabelStartDisabled(t *testing.T) {
	ctx := context.Background()
	h := newPollerHarness(t)
	h.poller.deps.Config.Labels.Create = ""
	h.poller.deps.Config.Labels.Plan = ""
	h.poller.deps.Config.Labels.Start = "label-start"
	h.poller.deps.Config.StartGuard.AllowLabelStart = false
	h.linear.issues = []linear.Issue{{
		ID:         "lin-start",
		Identifier: "ENG-2",
		Title:      "Start guarded",
		Team:       &linear.Team{ID: "team-1", Key: "ENG"},
		Labels:     []linear.Label{{ID: "label-start", Name: "agent-ready"}},
	}}

	stats := h.poller.PollOnce(ctx)
	require.False(t, stats.Aborted, "unexpected poll error: %v", stats.Err)

	assert.Empty(t, h.linear.removedLabels)
	triggers, err := h.store.ListUnprocessedLinearTriggers("proj", 10)
	require.NoError(t, err)
	assert.Empty(t, triggers)
}

func TestPollerPollOnceSkipsOldCommentsOnFirstPoll(t *testing.T) {
	ctx := context.Background()
	h := newPollerHarness(t)
	require.NoError(t, h.store.Create("proj", taskstore.TaskEntry{
		Filename:         "eng-1",
		Status:           taskstore.StatusReady,
		Content:          "# plan\n\n## Wave 1\n\n### Task 1: test\n",
		LinearIssueID:    "lin-old",
		LinearIdentifier: "ENG-1",
	}))
	h.linear.byID["lin-old"] = testIssue("lin-old")
	h.linear.comments["lin-old"] = []linear.Comment{{
		ID:        "old-command",
		Body:      "/kasmos plan",
		CreatedAt: h.now.Add(-time.Hour),
		User:      &linear.User{ID: "actor"},
	}}

	stats := h.poller.PollOnce(ctx)
	require.False(t, stats.Aborted, "unexpected poll error: %v", stats.Err)

	signals, err := h.gateway.List("proj", taskstore.SignalPending)
	require.NoError(t, err)
	assert.Empty(t, signals)
}

func TestPollerPollOncePagesCommentsAndProcessesCreatedAtOrderOnce(t *testing.T) {
	ctx := context.Background()
	h := newPollerHarness(t)
	require.NoError(t, h.store.Create("proj", taskstore.TaskEntry{
		Filename:         "eng-1",
		Status:           taskstore.StatusReady,
		Content:          "# plan\n\n## Wave 1\n\n### Task 1: test\n",
		LinearIssueID:    "lin-pages",
		LinearIdentifier: "ENG-1",
	}))
	h.linear.byID["lin-pages"] = testIssue("lin-pages")
	for i := 0; i < 49; i++ {
		h.linear.comments["lin-pages"] = append(h.linear.comments["lin-pages"], linear.Comment{
			ID:        "ordinary-" + strconv.Itoa(i),
			Body:      "ordinary",
			CreatedAt: h.now.Add(time.Duration(i) * time.Second),
		})
	}
	h.linear.comments["lin-pages"] = append(h.linear.comments["lin-pages"],
		linear.Comment{
			ID:        "newer-help",
			Body:      "/kasmos help",
			CreatedAt: h.now.Add(2 * time.Minute),
			User:      &linear.User{ID: "actor"},
		},
		linear.Comment{
			ID:        "older-status",
			Body:      "/kasmos status",
			CreatedAt: h.now.Add(time.Minute),
			User:      &linear.User{ID: "actor"},
		},
	)

	stats := h.poller.PollOnce(ctx)
	require.False(t, stats.Aborted, "unexpected poll error: %v", stats.Err)
	stats = h.poller.PollOnce(ctx)
	require.False(t, stats.Aborted, "unexpected poll error: %v", stats.Err)

	require.GreaterOrEqual(t, len(h.linear.commentPages), 2)
	require.Len(t, h.linear.createdComments, 2)
	assert.Contains(t, h.linear.createdComments[0].body, "kasmos trigger status")
	assert.Contains(t, h.linear.createdComments[1].body, "kasmos trigger help")
}

func TestPollerAuditDetailsUseStableTriggerFields(t *testing.T) {
	ctx := context.Background()
	h := newPollerHarness(t)
	require.NoError(t, h.store.Create("proj", taskstore.TaskEntry{
		Filename:         "eng-1",
		Status:           taskstore.StatusReady,
		Content:          "# plan\n\n## Wave 1\n\n### Task 1: test\n",
		LinearIssueID:    "lin-audit",
		LinearIdentifier: "ENG-1",
	}))
	h.linear.byID["lin-audit"] = testIssue("lin-audit")
	h.linear.comments["lin-audit"] = []linear.Comment{{
		ID:        "comment-audit",
		Body:      "/kasmos status",
		CreatedAt: h.now.Add(time.Minute),
		User:      &linear.User{ID: "actor"},
	}}

	stats := h.poller.PollOnce(ctx)
	require.False(t, stats.Aborted, "unexpected poll error: %v", stats.Err)

	require.NotEmpty(t, h.audit.events)
	for _, event := range h.audit.events {
		var detail map[string]string
		require.NoError(t, json.Unmarshal([]byte(event.Detail), &detail))
		assert.Equal(t, "status", detail["command_kind"])
		assert.Equal(t, "comment", detail["source_kind"])
		assert.Equal(t, "comment-audit", detail["source_id"])
		assert.Equal(t, "ENG-1", detail["linear_identifier"])
		assert.Equal(t, "actor", detail["actor_id"])
		assert.Contains(t, detail, "reason")
		assert.NotContains(t, detail, "verb")
		assert.NotContains(t, detail, "identifier")
	}
}

func TestPollerDrainQueuedProcessesExistingTriggerRows(t *testing.T) {
	ctx := context.Background()
	h := newPollerHarness(t)
	h.poller.deps.Config.Verbs[VerbStart] = false
	require.NoError(t, h.store.Create("proj", taskstore.TaskEntry{
		Filename:         "eng-1",
		Status:           taskstore.StatusReady,
		Content:          "# plan\n\n## Wave 1\n\n### Task 1: test\n",
		LinearIssueID:    "lin-drain",
		LinearIdentifier: "ENG-1",
	}))
	h.linear.byID["lin-drain"] = linear.Issue{
		ID:         "lin-drain",
		Identifier: "ENG-1",
		Title:      "Drain queued",
		URL:        "https://linear.local/ENG-1",
		Team:       &linear.Team{ID: "team-1", Key: "ENG"},
		Labels:     []linear.Label{{ID: "label-start", Name: "start"}},
	}
	for _, trigger := range []taskstore.LinearTriggerEntry{
		{
			LinearIssueID:    "lin-drain",
			LinearIdentifier: "ENG-1",
			CommandKind:      string(VerbStatus),
			SourceKind:       string(SourceComment),
			SourceID:         "comment-status",
			ActorID:          "other",
			DetectedAt:       h.now,
		},
		{
			LinearIssueID:    "lin-drain",
			LinearIdentifier: "ENG-1",
			CommandKind:      string(VerbPlan),
			SourceKind:       string(SourceComment),
			SourceID:         "comment-rejected",
			ActorID:          "not-allowed",
			DetectedAt:       h.now.Add(time.Second),
		},
		{
			LinearIssueID:    "lin-drain",
			LinearIdentifier: "ENG-1",
			CommandKind:      string(VerbStart),
			SourceKind:       string(SourceLabel),
			SourceID:         "label-start",
			ActorID:          "actor",
			DetectedAt:       h.now.Add(2 * time.Second),
		},
	} {
		_, queued, err := h.store.EnqueueLinearTrigger("proj", trigger)
		require.NoError(t, err)
		require.True(t, queued)
	}

	stats := h.poller.DrainQueued(ctx, 10)

	require.False(t, stats.Aborted, "unexpected drain error: %v", stats.Err)
	assert.Equal(t, 1, stats.Dispatched)
	assert.Equal(t, 1, stats.Rejected)
	assert.Equal(t, 1, stats.Ignored)
	assert.Equal(t, 0, stats.AckFailed)
	require.Len(t, h.linear.reactions, 2)
	assert.Equal(t, "comment-status", h.linear.reactions[0].commentID)
	assert.Equal(t, "eyes", h.linear.reactions[0].emoji)
	assert.Equal(t, "comment-rejected", h.linear.reactions[1].commentID)
	assert.Equal(t, "x", h.linear.reactions[1].emoji)
	require.Len(t, h.linear.createdComments, 1)
	assert.Contains(t, h.linear.createdComments[0].body, "status: ready")
	require.Len(t, h.linear.removedLabels, 1)
	assert.Equal(t, "lin-drain", h.linear.removedLabels[0].issueID)
	assert.Contains(t, h.linear.removedLabels[0].labels, "label-ack")
	remaining, err := h.store.ListUnprocessedLinearTriggers("proj", 10)
	require.NoError(t, err)
	assert.Empty(t, remaining)
}

type pollerHarness struct {
	store   *taskstore.SQLiteStore
	gateway taskstore.SignalGateway
	linear  *fakeLinearClient
	audit   *triggerRecordingLogger
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
	audit := &triggerRecordingLogger{}
	linker := linearlink.New(store, client, auditlog.NopLogger(), "proj")
	poller := NewPoller(PollerDeps{
		Project: "proj",
		Config:  cfg,
		Store:   store,
		Linker:  linker,
		Linear:  client,
		Gateway: gateway,
		Audit:   audit,
		Now:     func() time.Time { return now },
	})
	return &pollerHarness{store: store, gateway: gateway, linear: client, audit: audit, poller: poller, now: now}
}

type fakeLinearClient struct {
	issues        []linear.Issue
	byID          map[string]linear.Issue
	comments      map[string][]linear.Comment
	issueCalls    map[string]int
	commentPages  []linear.PageOptions
	removedLabels []struct {
		issueID string
		labels  []string
	}
	reactions []struct {
		commentID string
		emoji     string
	}
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

func (f *fakeLinearClient) Comments(_ context.Context, issueID string, p linear.PageOptions) ([]linear.Comment, linear.PageInfo, error) {
	f.commentPages = append(f.commentPages, p)
	all := f.comments[issueID]
	first := p.First
	if first <= 0 {
		first = 25
	}
	start := 0
	if p.After != "" {
		parsed, err := strconv.Atoi(p.After)
		if err == nil {
			start = parsed
		}
	}
	if start > len(all) {
		start = len(all)
	}
	end := start + first
	if end > len(all) {
		end = len(all)
	}
	page := append([]linear.Comment(nil), all[start:end]...)
	pageInfo := linear.PageInfo{HasNextPage: end < len(all)}
	if pageInfo.HasNextPage {
		pageInfo.EndCursor = strconv.Itoa(end)
	}
	return page, pageInfo, nil
}

func (f *fakeLinearClient) IssueLabel(_ context.Context, labelID string) (*linear.Label, error) {
	return &linear.Label{ID: labelID}, nil
}

func (f *fakeLinearClient) RemoveLabelFromIssue(_ context.Context, issueID string, labels []string) error {
	f.removedLabels = append(f.removedLabels, struct {
		issueID string
		labels  []string
	}{issueID: issueID, labels: append([]string(nil), labels...)})
	return nil
}

func (f *fakeLinearClient) CreateComment(_ context.Context, issueID, body string) (*linear.Comment, error) {
	f.createdComments = append(f.createdComments, struct {
		issueID string
		body    string
	}{issueID: issueID, body: body})
	return &linear.Comment{ID: "created"}, nil
}

func (f *fakeLinearClient) CreateCommentReaction(_ context.Context, commentID string, emoji string) error {
	f.reactions = append(f.reactions, struct {
		commentID string
		emoji     string
	}{commentID: commentID, emoji: emoji})
	return f.errReaction
}

func (f *fakeLinearClient) UpdateIssue(_ context.Context, issueID string, _ linear.UpdateIssueInput) (*linear.Issue, error) {
	issue, _ := f.Issue(context.Background(), issueID)
	return issue, nil
}

type triggerRecordingLogger struct {
	events []auditlog.Event
}

func (l *triggerRecordingLogger) Emit(event auditlog.Event) {
	l.events = append(l.events, event)
}

func (l *triggerRecordingLogger) Query(auditlog.QueryFilter) ([]auditlog.Event, error) {
	return l.events, nil
}

func (l *triggerRecordingLogger) Close() error {
	return nil
}

func (h *pollerHarness) requireAuditDetailCount(t *testing.T, kind auditlog.EventKind, needle string, want int) {
	t.Helper()
	got := 0
	for _, event := range h.audit.events {
		if event.Kind == kind && strings.Contains(event.Detail, needle) {
			got++
		}
	}
	assert.Equal(t, want, got, "kind=%s needle=%s", kind, needle)
}
