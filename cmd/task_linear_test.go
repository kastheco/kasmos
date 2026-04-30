package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/kastheco/kasmos/config/auditlog"
	"github.com/kastheco/kasmos/config/linearlink"
	"github.com/kastheco/kasmos/config/taskstore"
	"github.com/kastheco/kasmos/internal/linear"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const taskLinearTestProject = "kasmos"

func TestExecuteTaskLinkLinear_Happy(t *testing.T) {
	store := newTaskLinearStore(t)
	fetcher := newTaskLinearFetcherStub()
	logger := &taskLinearRecordingLogger{}

	result, err := executeTaskLinkLinearWithLoggerFetcher(context.Background(), taskLinearTestProject, linearlink.LinkInput{
		Filename: "plan",
		IssueArg: "KAS-123",
		Reason:   "operator requested",
	}, store, logger, fetcher)

	require.NoError(t, err)
	assert.Equal(t, "KAS-123", fetcher.issueArg)
	assert.Equal(t, "issue-123", result.Link.LinearIssueID)
	assert.Equal(t, "KAS-123", result.Link.LinearIdentifier)
	assert.Equal(t, "issue-123", mustTaskLinearEntry(t, store, "plan").LinearIssueID)
	require.Len(t, logger.events, 1)
	assert.Equal(t, auditlog.EventTaskLinearLinked, logger.events[0].Kind)
	assertTaskLinearDetail(t, logger.events[0].Detail, "", "KAS-123", "operator requested")
}

func TestExecuteTaskLinkLinear_AlreadyLinked(t *testing.T) {
	store := newTaskLinearStore(t)
	require.NoError(t, store.SetLinearLink(taskLinearTestProject, "plan", taskstore.LinearLink{
		LinearIssueID:    "old-issue",
		LinearIdentifier: "KAS-1",
		LinearURL:        "https://linear.app/kasmos/issue/KAS-1/old",
	}))
	fetcher := newTaskLinearFetcherStub()
	logger := &taskLinearRecordingLogger{}

	_, err := executeTaskLinkLinearWithLoggerFetcher(context.Background(), taskLinearTestProject, linearlink.LinkInput{
		Filename: "plan",
		IssueArg: "KAS-123",
	}, store, logger, fetcher)

	require.ErrorIs(t, err, linearlink.ErrAlreadyLinked)
	assert.Empty(t, fetcher.issueArg)
	assert.Equal(t, "old-issue", mustTaskLinearEntry(t, store, "plan").LinearIssueID)
	assert.Empty(t, logger.events)
}

func TestExecuteTaskLinkLinear_ForceReplaces(t *testing.T) {
	store := newTaskLinearStore(t)
	require.NoError(t, store.SetLinearLink(taskLinearTestProject, "plan", taskstore.LinearLink{
		LinearIssueID:    "old-issue",
		LinearIdentifier: "KAS-1",
		LinearURL:        "https://linear.app/kasmos/issue/KAS-1/old",
	}))
	fetcher := newTaskLinearFetcherStub()
	logger := &taskLinearRecordingLogger{}

	result, err := executeTaskLinkLinearWithLoggerFetcher(context.Background(), taskLinearTestProject, linearlink.LinkInput{
		Filename: "plan",
		IssueArg: "KAS-123",
		Reason:   "replacement",
		Force:    true,
	}, store, logger, fetcher)

	require.NoError(t, err)
	assert.True(t, result.Replaced)
	assert.Equal(t, "KAS-123", fetcher.issueArg)
	assert.Equal(t, "issue-123", mustTaskLinearEntry(t, store, "plan").LinearIssueID)
	require.Len(t, logger.events, 1)
	assertTaskLinearDetail(t, logger.events[0].Detail, "KAS-1", "KAS-123", "replacement")
}

func TestExecuteTaskLinkLinear_DuplicateOnAnotherActiveTask(t *testing.T) {
	store := newTaskLinearStore(t)
	require.NoError(t, store.Create(taskLinearTestProject, taskstore.TaskEntry{
		Filename:  "other",
		Status:    taskstore.StatusImplementing,
		CreatedAt: time.Now(),
	}))
	require.NoError(t, store.SetLinearLink(taskLinearTestProject, "other", taskstore.LinearLink{
		LinearIssueID:    "issue-123",
		LinearIdentifier: "KAS-123",
		LinearURL:        "https://linear.app/kasmos/issue/KAS-123/conflict",
	}))
	logger := &taskLinearRecordingLogger{}

	_, err := executeTaskLinkLinearWithLoggerFetcher(context.Background(), taskLinearTestProject, linearlink.LinkInput{
		Filename: "plan",
		IssueArg: "KAS-123",
	}, store, logger, newTaskLinearFetcherStub())

	require.ErrorIs(t, err, linearlink.ErrDuplicateLink)
	assert.Empty(t, mustTaskLinearEntry(t, store, "plan").LinearIssueID)
	assert.Empty(t, logger.events)
}

func TestExecuteTaskLinkLinear_NotConfigured(t *testing.T) {
	store := newTaskLinearStore(t)
	t.Setenv("KASMOS_LINEAR_API_KEY", "")
	t.Setenv("LINEAR_API_KEY", "")

	_, err := executeTaskLinkLinear(context.Background(), taskLinearTestProject, linearlink.LinkInput{
		Filename: "plan",
		IssueArg: "KAS-123",
	}, store)

	require.ErrorIs(t, err, linear.ErrNotConfigured)
	assert.Empty(t, mustTaskLinearEntry(t, store, "plan").LinearIssueID)
}

func TestExecuteTaskLinkLinear_FetchFailure_NoWrite(t *testing.T) {
	store := newTaskLinearStore(t)
	fetcher := newTaskLinearFetcherStub()
	fetcher.issueErr = errors.New("linear unavailable")
	logger := &taskLinearRecordingLogger{}

	_, err := executeTaskLinkLinearWithLoggerFetcher(context.Background(), taskLinearTestProject, linearlink.LinkInput{
		Filename: "plan",
		IssueArg: "KAS-123",
	}, store, logger, fetcher)

	require.ErrorContains(t, err, "linear unavailable")
	assert.Empty(t, mustTaskLinearEntry(t, store, "plan").LinearIssueID)
	assert.Empty(t, logger.events)
}

func TestExecuteTaskUnlinkLinear_Happy(t *testing.T) {
	store := newTaskLinearStore(t)
	require.NoError(t, store.SetLinearLink(taskLinearTestProject, "plan", taskstore.LinearLink{
		LinearIssueID:    "issue-123",
		LinearIdentifier: "KAS-123",
		LinearURL:        "https://linear.app/kasmos/issue/KAS-123/link",
	}))
	logger := &taskLinearRecordingLogger{}

	result, err := executeTaskUnlinkLinearWithLoggerFetcher(context.Background(), taskLinearTestProject, "plan", "wrong task", store, logger, nil)

	require.NoError(t, err)
	assert.Equal(t, "KAS-123", result.Link.LinearIdentifier)
	assert.Empty(t, mustTaskLinearEntry(t, store, "plan").LinearIssueID)
	require.Len(t, logger.events, 1)
	assert.Equal(t, auditlog.EventTaskLinearUnlinked, logger.events[0].Kind)
	assertTaskLinearDetail(t, logger.events[0].Detail, "KAS-123", "", "wrong task")
}

func TestExecuteTaskUnlinkLinear_NoLink(t *testing.T) {
	store := newTaskLinearStore(t)
	logger := &taskLinearRecordingLogger{}

	result, err := executeTaskUnlinkLinearWithLoggerFetcher(context.Background(), taskLinearTestProject, "plan", "", store, logger, nil)

	require.NoError(t, err)
	assert.Empty(t, result.Link.LinearIdentifier)
	assert.Empty(t, mustTaskLinearEntry(t, store, "plan").LinearIssueID)
	assert.Empty(t, logger.events)
}

func TestLinkLinear_AuditEventOnSuccess(t *testing.T) {
	store := newTaskLinearStore(t)
	logger := &taskLinearRecordingLogger{}

	_, err := executeTaskLinkLinearWithLoggerFetcher(context.Background(), taskLinearTestProject, linearlink.LinkInput{
		Filename: "plan",
		IssueArg: "KAS-123",
	}, store, logger, newTaskLinearFetcherStub())

	require.NoError(t, err)
	require.Len(t, logger.events, 1)
	assert.Equal(t, auditlog.EventTaskLinearLinked, logger.events[0].Kind)
}

func TestLinkLinear_NoAuditOnDuplicate(t *testing.T) {
	store := newTaskLinearStore(t)
	require.NoError(t, store.Create(taskLinearTestProject, taskstore.TaskEntry{Filename: "other", Status: taskstore.StatusPlanning}))
	require.NoError(t, store.SetLinearLink(taskLinearTestProject, "other", taskstore.LinearLink{LinearIssueID: "issue-123", LinearIdentifier: "KAS-123"}))
	logger := &taskLinearRecordingLogger{}

	_, err := executeTaskLinkLinearWithLoggerFetcher(context.Background(), taskLinearTestProject, linearlink.LinkInput{
		Filename: "plan",
		IssueArg: "KAS-123",
	}, store, logger, newTaskLinearFetcherStub())

	require.ErrorIs(t, err, linearlink.ErrDuplicateLink)
	assert.Empty(t, logger.events)
}

func TestEndToEnd_LinearLinkLifecycle(t *testing.T) {
	store := newTaskLinearStore(t)
	logger := &taskLinearRecordingLogger{}

	_, err := executeTaskLinkLinearWithLoggerFetcher(context.Background(), taskLinearTestProject, linearlink.LinkInput{
		Filename: "plan",
		IssueArg: "KAS-123",
	}, store, logger, newTaskLinearFetcherStub())
	require.NoError(t, err)

	_, err = executeTaskLinkLinearWithLoggerFetcher(context.Background(), taskLinearTestProject, linearlink.LinkInput{
		Filename: "plan",
		IssueArg: "KAS-456",
	}, store, logger, taskLinearFetcherWithIssue("issue-456", "KAS-456"))
	require.ErrorIs(t, err, linearlink.ErrAlreadyLinked)

	_, err = executeTaskLinkLinearWithLoggerFetcher(context.Background(), taskLinearTestProject, linearlink.LinkInput{
		Filename: "plan",
		IssueArg: "KAS-456",
		Force:    true,
	}, store, logger, taskLinearFetcherWithIssue("issue-456", "KAS-456"))
	require.NoError(t, err)

	_, err = executeTaskUnlinkLinearWithLoggerFetcher(context.Background(), taskLinearTestProject, "plan", "", store, logger, nil)
	require.NoError(t, err)

	_, err = executeTaskLinkLinearWithLoggerFetcher(context.Background(), taskLinearTestProject, linearlink.LinkInput{
		Filename: "plan",
		IssueArg: "KAS-789",
	}, store, logger, taskLinearFetcherWithIssue("issue-789", "KAS-789"))
	require.NoError(t, err)

	require.Len(t, logger.events, 4)
	assert.Equal(t, []auditlog.EventKind{
		auditlog.EventTaskLinearLinked,
		auditlog.EventTaskLinearLinked,
		auditlog.EventTaskLinearUnlinked,
		auditlog.EventTaskLinearLinked,
	}, []auditlog.EventKind{
		logger.events[0].Kind,
		logger.events[1].Kind,
		logger.events[2].Kind,
		logger.events[3].Kind,
	})
	assert.Equal(t, "issue-789", mustTaskLinearEntry(t, store, "plan").LinearIssueID)
}

func newTaskLinearStore(t *testing.T) taskstore.Store {
	t.Helper()
	store := taskstore.NewTestSQLiteStore(t)
	require.NoError(t, store.Create(taskLinearTestProject, taskstore.TaskEntry{
		Filename:  "plan",
		Status:    taskstore.StatusPlanning,
		Branch:    "plan-branch",
		CreatedAt: time.Now(),
	}))
	return store
}

func mustTaskLinearEntry(t *testing.T, store taskstore.Store, filename string) taskstore.TaskEntry {
	t.Helper()
	entry, err := store.Get(taskLinearTestProject, filename)
	require.NoError(t, err)
	return entry
}

func newTaskLinearFetcherStub() *taskLinearFetcherStub {
	return taskLinearFetcherWithIssue("issue-123", "KAS-123")
}

func taskLinearFetcherWithIssue(issueID, identifier string) *taskLinearFetcherStub {
	return &taskLinearFetcherStub{
		issue: &linear.Issue{
			ID:         issueID,
			Identifier: identifier,
			URL:        "https://linear.app/kasmos/issue/" + identifier + "/task",
			Team:       &linear.Team{ID: "team-1", Key: "KAS", Name: "kasmos"},
			Project:    &linear.Project{ID: "project-1", Name: "kasmos"},
		},
	}
}

type taskLinearFetcherStub struct {
	issueArg       string
	issue          *linear.Issue
	issueErr       error
	commentIssueID string
	commentBody    string
	comment        *linear.Comment
	commentErr     error
}

func (f *taskLinearFetcherStub) Issue(_ context.Context, idOrIdentifier string) (*linear.Issue, error) {
	f.issueArg = idOrIdentifier
	if f.issueErr != nil {
		return nil, f.issueErr
	}
	return f.issue, nil
}

func (f *taskLinearFetcherStub) CreateComment(_ context.Context, issueID, body string) (*linear.Comment, error) {
	f.commentIssueID = issueID
	f.commentBody = body
	if f.commentErr != nil {
		return nil, f.commentErr
	}
	return f.comment, nil
}

type taskLinearRecordingLogger struct {
	events []auditlog.Event
}

func (l *taskLinearRecordingLogger) Emit(event auditlog.Event) {
	l.events = append(l.events, event)
}

func (l *taskLinearRecordingLogger) Query(auditlog.QueryFilter) ([]auditlog.Event, error) {
	return l.events, nil
}

func (l *taskLinearRecordingLogger) Close() error {
	return nil
}

func assertTaskLinearDetail(t *testing.T, raw, previous, next, reason string) {
	t.Helper()
	var detail map[string]string
	require.NoError(t, json.Unmarshal([]byte(raw), &detail))
	if previous == "" {
		assert.NotContains(t, detail, "previous_identifier")
	} else {
		assert.Equal(t, previous, detail["previous_identifier"])
	}
	if next == "" {
		assert.NotContains(t, detail, "new_identifier")
	} else {
		assert.Equal(t, next, detail["new_identifier"])
	}
	if reason == "" {
		assert.NotContains(t, detail, "reason")
	} else {
		assert.Equal(t, reason, detail["reason"])
	}
}
