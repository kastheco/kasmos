package linearreceipt

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/kastheco/kasmos/config/auditlog"
	"github.com/kastheco/kasmos/config/taskfsm"
	"github.com/kastheco/kasmos/config/taskstore"
	"github.com/kastheco/kasmos/internal/linear"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testProject = "kasmos"
	testTask    = "linear-task"
)

type mockLinearClient struct {
	createErr error
	updateErr error

	mu             sync.Mutex
	createComments []createCommentCall
	updateIssues   []updateIssueCall
	ops            []string
}

type createCommentCall struct {
	issueID string
	body    string
}

type updateIssueCall struct {
	issueID string
	input   linear.UpdateIssueInput
}

func (m *mockLinearClient) CreateComment(_ context.Context, issueID, body string) (*linear.Comment, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ops = append(m.ops, "create_comment")
	m.createComments = append(m.createComments, createCommentCall{issueID: issueID, body: body})
	if m.createErr != nil {
		return nil, m.createErr
	}
	return &linear.Comment{ID: "comment-1", URL: "https://linear.app/comment/comment-1", Body: body}, nil
}

func (m *mockLinearClient) UpdateIssue(_ context.Context, issueID string, in linear.UpdateIssueInput) (*linear.Issue, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ops = append(m.ops, "update_issue")
	m.updateIssues = append(m.updateIssues, updateIssueCall{issueID: issueID, input: in})
	if m.updateErr != nil {
		return nil, m.updateErr
	}
	return &linear.Issue{ID: issueID}, nil
}

func (m *mockLinearClient) createCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.createComments)
}

func (m *mockLinearClient) updateCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.updateIssues)
}

func (m *mockLinearClient) firstCreate() createCommentCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.createComments[0]
}

func (m *mockLinearClient) firstUpdate() updateIssueCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.updateIssues[0]
}

func (m *mockLinearClient) operations() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	ops := make([]string, len(m.ops))
	copy(ops, m.ops)
	return ops
}

type recordingAuditLogger struct {
	mu     sync.Mutex
	events []auditlog.Event
}

func (l *recordingAuditLogger) Emit(event auditlog.Event) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.events = append(l.events, event)
}

func (l *recordingAuditLogger) Query(filter auditlog.QueryFilter) ([]auditlog.Event, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	events := make([]auditlog.Event, 0, len(l.events))
	for _, event := range l.events {
		if filter.Project != "" && event.Project != filter.Project {
			continue
		}
		if filter.TaskFile != "" && event.TaskFile != filter.TaskFile {
			continue
		}
		if len(filter.Kinds) > 0 {
			matched := false
			for _, kind := range filter.Kinds {
				if event.Kind == kind {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}
		events = append(events, event)
	}
	return events, nil
}

func (l *recordingAuditLogger) Close() error { return nil }

func (l *recordingAuditLogger) all() []auditlog.Event {
	l.mu.Lock()
	defer l.mu.Unlock()
	events := make([]auditlog.Event, len(l.events))
	copy(events, l.events)
	return events
}

func enabledConfig(events ...taskfsm.Event) Config {
	cfg := Config{
		Enabled:       true,
		Events:        map[taskfsm.Event]bool{},
		StateMap:      map[taskstore.Status]string{},
		PRReceipts:    true,
		MergeReceipts: true,
		CancelReceipt: true,
	}
	for _, event := range events {
		cfg.Events[event] = true
	}
	return cfg
}

func newLinkedHook(t *testing.T, cfg Config, client *mockLinearClient, logger auditlog.Logger) (*Hook, taskstore.Store) {
	t.Helper()
	store := taskstore.NewTestStore(t)
	require.NoError(t, store.Create(testProject, taskstore.TaskEntry{
		Filename:             testTask,
		Status:               taskstore.StatusReviewing,
		Branch:               "feature/linear-receipts",
		LinearIssueID:        "issue-1",
		LinearIdentifier:     "KAS-123",
		LinearURL:            "https://linear.app/kas/issue/KAS-123",
		PRURL:                "https://github.com/kastheco/kasmos/pull/123",
		LatestReviewFeedback: "please adjust",
	}))
	hook := NewHook(cfg, store, client, logger, testProject)
	hook.now = func() time.Time { return time.Date(2026, 4, 30, 12, 0, 0, 0, time.UTC) }
	return hook, store
}

func lifecycleEvent() taskfsm.TransitionEvent {
	return taskfsm.TransitionEvent{
		PlanFile:   testTask,
		FromStatus: taskfsm.StatusImplementing,
		ToStatus:   taskfsm.StatusReviewing,
		Event:      taskfsm.ImplementFinished,
		Project:    testProject,
		Timestamp:  time.Date(2026, 4, 30, 11, 59, 0, 0, time.UTC),
	}
}

func TestHookNameStable(t *testing.T) {
	hook := NewHook(Config{}, nil, nil, nil, testProject)
	assert.Equal(t, "linearreceipt", hook.Name())
}

func TestHookRunPostsLifecycleReceiptForLinkedAllowedEvent(t *testing.T) {
	client := &mockLinearClient{}
	audit := &recordingAuditLogger{}
	hook, _ := newLinkedHook(t, enabledConfig(taskfsm.ImplementFinished), client, audit)

	err := hook.Run(context.Background(), lifecycleEvent())

	require.NoError(t, err)
	require.Equal(t, 1, client.createCount())
	call := client.firstCreate()
	assert.Equal(t, "issue-1", call.issueID)
	assert.Contains(t, call.body, "event: implement_finished")
	assert.Contains(t, call.body, "status: implementing -> reviewing")
	events := audit.all()
	require.Len(t, events, 1)
	assert.Equal(t, auditlog.EventTaskLinearReceiptPosted, events[0].Kind)
	assert.Equal(t, testProject, events[0].Project)
	assert.Equal(t, testTask, events[0].TaskFile)
}

func TestHookRunSkipsUnlinkedTask(t *testing.T) {
	store := taskstore.NewTestStore(t)
	require.NoError(t, store.Create(testProject, taskstore.TaskEntry{Filename: testTask, Status: taskstore.StatusReviewing}))
	client := &mockLinearClient{}
	hook := NewHook(enabledConfig(taskfsm.ImplementFinished), store, client, nil, testProject)

	err := hook.Run(context.Background(), lifecycleEvent())

	require.NoError(t, err)
	assert.Equal(t, 0, client.createCount())
}

func TestHookRunSkipsDisabledConfig(t *testing.T) {
	client := &mockLinearClient{}
	hook, _ := newLinkedHook(t, Config{}, client, nil)

	err := hook.Run(context.Background(), lifecycleEvent())

	require.NoError(t, err)
	assert.Equal(t, 0, client.createCount())
}

func TestHookRunSkipsEventNotInAllowlist(t *testing.T) {
	client := &mockLinearClient{}
	hook, _ := newLinkedHook(t, enabledConfig(taskfsm.PlanStart), client, nil)

	err := hook.Run(context.Background(), lifecycleEvent())

	require.NoError(t, err)
	assert.Equal(t, 0, client.createCount())
}

func TestHookRunErrNotConfiguredIsSilent(t *testing.T) {
	client := &mockLinearClient{createErr: linear.ErrNotConfigured}
	audit := &recordingAuditLogger{}
	hook, _ := newLinkedHook(t, enabledConfig(taskfsm.ImplementFinished), client, audit)

	err := hook.Run(context.Background(), lifecycleEvent())

	require.NoError(t, err)
	assert.Equal(t, 1, client.createCount())
	assert.Empty(t, audit.all())
}

func TestHookRunRateLimitEmitsWarnAuditAndReturnsError(t *testing.T) {
	rateLimitErr := &linear.RateLimitError{StatusCode: 429, GraphQLCode: "RATE_LIMITED", Remaining: 0}
	client := &mockLinearClient{createErr: rateLimitErr}
	audit := &recordingAuditLogger{}
	hook, _ := newLinkedHook(t, enabledConfig(taskfsm.ImplementFinished), client, audit)

	err := hook.Run(context.Background(), lifecycleEvent())

	require.ErrorIs(t, err, rateLimitErr)
	events := audit.all()
	require.Len(t, events, 1)
	assert.Equal(t, auditlog.EventTaskLinearReceiptFailed, events[0].Kind)
	assert.Equal(t, "warn", events[0].Level)
	var detail map[string]string
	require.NoError(t, json.Unmarshal([]byte(events[0].Detail), &detail))
	assert.Equal(t, "implement_finished", detail["event"])
	assert.Equal(t, "implementing", detail["from"])
	assert.Equal(t, "reviewing", detail["to"])
	assert.Equal(t, "KAS-123", detail["linear_identifier"])
	assert.Contains(t, detail["error"], "rate limited")
}

func TestHookRunStateMapUpdatesBeforeComment(t *testing.T) {
	client := &mockLinearClient{}
	cfg := enabledConfig(taskfsm.ImplementFinished)
	cfg.StateMap[taskstore.StatusReviewing] = "state-reviewing"
	hook, _ := newLinkedHook(t, cfg, client, nil)

	err := hook.Run(context.Background(), lifecycleEvent())

	require.NoError(t, err)
	require.Equal(t, 1, client.updateCount())
	assert.Equal(t, "issue-1", client.firstUpdate().issueID)
	require.NotNil(t, client.firstUpdate().input.StateID)
	assert.Equal(t, "state-reviewing", *client.firstUpdate().input.StateID)
	assert.Equal(t, 1, client.createCount())
	assert.Equal(t, []string{"update_issue", "create_comment"}, client.operations())
}

func TestHookRunStateMapUpdateFailureDoesNotBlockComment(t *testing.T) {
	client := &mockLinearClient{updateErr: errors.New("update failed")}
	cfg := enabledConfig(taskfsm.ImplementFinished)
	cfg.StateMap[taskstore.StatusReviewing] = "state-reviewing"
	hook, _ := newLinkedHook(t, cfg, client, nil)

	err := hook.Run(context.Background(), lifecycleEvent())

	require.NoError(t, err)
	assert.Equal(t, 1, client.updateCount())
	assert.Equal(t, 1, client.createCount())
}

func TestHookRunSkipsEmptyStateMapTarget(t *testing.T) {
	client := &mockLinearClient{}
	cfg := enabledConfig(taskfsm.ImplementFinished)
	cfg.StateMap[taskstore.StatusDone] = "state-done"
	hook, _ := newLinkedHook(t, cfg, client, nil)

	err := hook.Run(context.Background(), lifecycleEvent())

	require.NoError(t, err)
	assert.Equal(t, 0, client.updateCount())
	assert.Equal(t, 1, client.createCount())
}

func TestHookRunRecoversFormatterPanic(t *testing.T) {
	client := &mockLinearClient{}
	hook, _ := newLinkedHook(t, enabledConfig(taskfsm.ImplementFinished), client, nil)
	original := lifecycleTemplate
	lifecycleTemplate = receiptTemplate{}
	t.Cleanup(func() { lifecycleTemplate = original })

	err := hook.Run(context.Background(), lifecycleEvent())

	require.NoError(t, err)
	assert.Equal(t, 0, client.createCount())
}
