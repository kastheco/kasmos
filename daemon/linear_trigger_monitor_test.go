package daemon

import (
	"bytes"
	"context"
	"database/sql"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kastheco/kasmos/config/auditlog"
	"github.com/kastheco/kasmos/config/linearlink"
	"github.com/kastheco/kasmos/config/lineartrigger"
	"github.com/kastheco/kasmos/config/taskstore"
	"github.com/kastheco/kasmos/daemon/api"
	"github.com/kastheco/kasmos/internal/linear"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

func TestLinearTriggerMonitor_LabelCreateIdempotentAcrossTwoCycles(t *testing.T) {
	store := newLinearTriggerMonitorStore(t)
	client := &linearTriggerMonitorLinearClient{issue: monitorIssue()}
	repos := NewRepoManager()
	repos.repos = []RepoEntry{monitorRepoEntry(t, store, client, monitorTriggerConfig())}
	monitor := NewLinearTriggerMonitor(LinearTriggerMonitorConfig{PollInterval: time.Minute}, repos, api.NewEventBroadcaster(), slog.Default())

	monitor.pollOnce(context.Background())
	monitor.pollOnce(context.Background())

	entries, err := store.ListByStatus("proj", taskstore.StatusReady)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "lin-123", entries[0].Filename)
	assert.Equal(t, "issue-1", entries[0].LinearIssueID)
	assert.Equal(t, 2, client.issuesCalls)
	assert.Equal(t, 2, client.issueCalls)
}

func TestLinearTriggerMonitor_MissingCredentialsWarningLogsOnce(t *testing.T) {
	store := newLinearTriggerMonitorStore(t)
	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelWarn}))
	repos := NewRepoManager()
	repos.repos = []RepoEntry{{
		Path:                "/tmp/proj",
		Project:             "proj",
		Store:               store,
		LinearTriggerConfig: monitorTriggerConfig(),
	}}
	monitor := NewLinearTriggerMonitor(LinearTriggerMonitorConfig{PollInterval: time.Minute}, repos, api.NewEventBroadcaster(), logger)

	for i := 0; i < 3; i++ {
		monitor.pollOnce(context.Background())
	}

	assert.Equal(t, 1, strings.Count(logBuf.String(), "Linear API credentials are configured"))
}

func TestLinearTriggerMonitor_RateLimitDoesNotAdvanceStateAndNextCycleResumes(t *testing.T) {
	store := newLinearTriggerMonitorStore(t)
	client := &linearTriggerMonitorLinearClient{
		issue:     monitorIssue(),
		issuesErr: &linear.RateLimitError{StatusCode: 429},
	}
	repos := NewRepoManager()
	repos.repos = []RepoEntry{monitorRepoEntry(t, store, client, monitorTriggerConfig())}
	monitor := NewLinearTriggerMonitor(LinearTriggerMonitorConfig{PollInterval: time.Minute}, repos, api.NewEventBroadcaster(), slog.Default())

	monitor.pollOnce(context.Background())
	entries, err := store.ListByStatus("proj", taskstore.StatusReady)
	require.NoError(t, err)
	assert.Empty(t, entries)

	client.issuesErr = nil
	monitor.pollOnce(context.Background())
	entries, err = store.ListByStatus("proj", taskstore.StatusReady)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "issue-1", entries[0].LinearIssueID)
}

func TestLinearTriggerMonitor_DisabledRepoSkipsPoller(t *testing.T) {
	store := newLinearTriggerMonitorStore(t)
	cfg := monitorTriggerConfig()
	cfg.Enabled = false
	client := &linearTriggerMonitorLinearClient{issue: monitorIssue()}
	repos := NewRepoManager()
	repos.repos = []RepoEntry{monitorRepoEntry(t, store, client, cfg)}
	monitor := NewLinearTriggerMonitor(LinearTriggerMonitorConfig{PollInterval: time.Minute}, repos, api.NewEventBroadcaster(), slog.Default())

	monitor.pollOnce(context.Background())

	assert.Zero(t, client.issuesCalls)
	assert.Zero(t, client.issueCalls)
}

func TestLinearTriggerMonitor_RunExitsOnContextCancel(t *testing.T) {
	store := newLinearTriggerMonitorStore(t)
	repos := NewRepoManager()
	repos.repos = []RepoEntry{{
		Path:                "/tmp/proj",
		Project:             "proj",
		Store:               store,
		LinearTriggerConfig: monitorTriggerConfig(),
	}}
	monitor := NewLinearTriggerMonitor(LinearTriggerMonitorConfig{PollInterval: time.Minute}, repos, api.NewEventBroadcaster(), slog.Default())
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- monitor.Run(ctx) }()

	cancel()

	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(time.Second):
		t.Fatal("monitor did not exit within 1s of cancellation")
	}
}

func TestLinearTriggerMonitor_UsesSmallestEnabledRepoIntervalClamped(t *testing.T) {
	repos := NewRepoManager()
	fast := monitorTriggerConfig()
	fast.PollInterval = 5 * time.Second
	slow := monitorTriggerConfig()
	slow.PollInterval = 45 * time.Second
	disabled := monitorTriggerConfig()
	disabled.Enabled = false
	disabled.PollInterval = time.Second
	repos.repos = []RepoEntry{
		{Project: "fast", LinearTriggerConfig: fast},
		{Project: "slow", LinearTriggerConfig: slow},
		{Project: "disabled", LinearTriggerConfig: disabled},
	}
	monitor := NewLinearTriggerMonitor(LinearTriggerMonitorConfig{PollInterval: time.Minute}, repos, nil, slog.Default())

	assert.Equal(t, 15*time.Second, monitor.pollInterval())
}

func TestLinearTriggerMonitor_PollOnceProducesGuardedTriggerAuditRows(t *testing.T) {
	ctx := context.Background()
	db, err := sql.Open("sqlite", "file:daemon-linear-trigger-e2e?mode=memory&cache=shared&_pragma=busy_timeout(30000)&_pragma=foreign_keys(on)")
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	store, err := taskstore.NewSQLiteStoreFromDB(db)
	require.NoError(t, err)
	gateway, err := taskstore.NewSQLiteSignalGatewayFromDB(db)
	require.NoError(t, err)
	audit, err := auditlog.NewSQLiteLoggerFromDB(db)
	require.NoError(t, err)

	cfg := monitorTriggerConfig()
	cfg.Actor = lineartrigger.ActorPolicy{AllowedUserIDs: []string{"allowed-user"}}
	now := time.Date(2026, 4, 30, 12, 0, 0, 0, time.UTC)
	client := &linearTriggerMonitorLinearClient{issue: monitorIssue(), comments: map[string][]linear.Comment{}}
	linker := linearlink.New(store, client, audit, "proj")
	poller := lineartrigger.NewPoller(lineartrigger.PollerDeps{
		Project: "proj",
		Config:  cfg,
		Store:   store,
		Linker:  linker,
		Linear:  client,
		Gateway: gateway,
		Audit:   audit,
		Service: lineartrigger.NewService("proj", cfg, store, nil, nil, nil),
		Now:     func() time.Time { return now },
		Logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	repos := NewRepoManager()
	repos.repos = []RepoEntry{{
		Path:                "/tmp/proj",
		Project:             "proj",
		Store:               store,
		SignalGateway:       gateway,
		LinearTriggerPoller: poller,
		LinearTriggerConfig: cfg,
	}}
	monitor := NewLinearTriggerMonitor(LinearTriggerMonitorConfig{PollInterval: time.Minute}, repos, api.NewEventBroadcaster(), slog.New(slog.NewTextHandler(io.Discard, nil)))

	monitor.pollOnce(ctx)
	client.comments["issue-1"] = []linear.Comment{{
		ID:        "comment-plan",
		Body:      "/kasmos plan",
		CreatedAt: now.Add(time.Minute),
		User:      &linear.User{ID: "allowed-user", Email: "allowed@example.com"},
	}}
	monitor.pollOnce(ctx)
	client.comments["issue-1"] = append(client.comments["issue-1"], linear.Comment{
		ID:        "comment-start-rejected",
		Body:      "/kasmos start",
		CreatedAt: now.Add(2 * time.Minute),
		User:      &linear.User{ID: "blocked-user", Email: "blocked@example.com"},
	})
	monitor.pollOnce(ctx)

	requireMonitorAuditCount(t, audit, auditlog.EventTaskLinearTriggerDispatched, "create", 1)
	requireMonitorAuditCount(t, audit, auditlog.EventTaskLinearTriggerDispatched, "plan", 1)
	requireMonitorAuditCount(t, audit, auditlog.EventTaskLinearTriggerRejected, "actor_not_allowed", 1)
	signals, err := gateway.List("proj", taskstore.SignalPending)
	require.NoError(t, err)
	require.Len(t, signals, 1)
	assert.Equal(t, "plan_start", signals[0].SignalType)
}

func TestLinearTriggerMonitor_PollOnceDrainsWebhookQueuedRows(t *testing.T) {
	ctx := context.Background()
	db, err := sql.Open("sqlite", "file:daemon-linear-webhook-drain?mode=memory&cache=shared&_pragma=busy_timeout(30000)&_pragma=foreign_keys(on)")
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	store, err := taskstore.NewSQLiteStoreFromDB(db)
	require.NoError(t, err)
	gateway, err := taskstore.NewSQLiteSignalGatewayFromDB(db)
	require.NoError(t, err)
	audit, err := auditlog.NewSQLiteLoggerFromDB(db)
	require.NoError(t, err)

	cfg := monitorTriggerConfig()
	cfg.Labels = lineartrigger.LabelMap{}
	cfg.Actor = lineartrigger.ActorPolicy{AllowedUserIDs: []string{"allowed-user"}}
	now := time.Date(2026, 4, 30, 12, 0, 0, 0, time.UTC)
	client := &linearTriggerMonitorLinearClient{
		issue:    monitorIssue(),
		comments: map[string][]linear.Comment{},
	}
	client.issue.Labels = nil
	linker := linearlink.New(store, client, audit, "proj")
	poller := lineartrigger.NewPoller(lineartrigger.PollerDeps{
		Project: "proj",
		Config:  cfg,
		Store:   store,
		Linker:  linker,
		Linear:  client,
		Gateway: gateway,
		Audit:   audit,
		Service: lineartrigger.NewService("proj", cfg, store, nil, nil, nil),
		Now:     func() time.Time { return now },
		Logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	repos := NewRepoManager()
	repos.repos = []RepoEntry{{
		Path:                "/tmp/proj",
		Project:             "proj",
		Store:               store,
		SignalGateway:       gateway,
		LinearTriggerPoller: poller,
		LinearTriggerConfig: cfg,
	}}
	_, queued, err := store.EnqueueLinearTrigger("proj", taskstore.LinearTriggerEntry{
		LinearIssueID:    "issue-1",
		LinearIdentifier: "LIN-123",
		CommandKind:      string(lineartrigger.VerbPlan),
		SourceKind:       string(lineartrigger.SourceComment),
		SourceID:         "webhook-comment-plan",
		ActorID:          "allowed-user",
		TaskArg:          "webhook-plan",
		DetectedAt:       now,
	})
	require.NoError(t, err)
	require.True(t, queued)
	monitor := NewLinearTriggerMonitor(LinearTriggerMonitorConfig{PollInterval: time.Minute}, repos, api.NewEventBroadcaster(), slog.New(slog.NewTextHandler(io.Discard, nil)))

	monitor.pollOnce(ctx)

	remaining, err := store.ListUnprocessedLinearTriggers("proj", 10)
	require.NoError(t, err)
	assert.Empty(t, remaining)
	requireMonitorAuditCount(t, audit, auditlog.EventTaskLinearTriggerDispatched, "plan", 1)
	signals, err := gateway.List("proj", taskstore.SignalPending)
	require.NoError(t, err)
	require.Len(t, signals, 1)
	assert.Equal(t, "plan_start", signals[0].SignalType)
	assert.Equal(t, "webhook-plan", signals[0].PlanFile)
}

func newLinearTriggerMonitorStore(t *testing.T) *taskstore.SQLiteStore {
	t.Helper()
	store, err := taskstore.NewSQLiteStore(filepath.Join(t.TempDir(), "taskstore.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func monitorRepoEntry(t *testing.T, store taskstore.Store, client *linearTriggerMonitorLinearClient, cfg lineartrigger.Config) RepoEntry {
	t.Helper()
	linker := linearlink.New(store, client, nil, "proj")
	poller := lineartrigger.NewPoller(lineartrigger.PollerDeps{
		Project: "proj",
		Config:  cfg,
		Store:   store,
		Linker:  linker,
		Linear:  client,
		Service: lineartrigger.NewService("proj", cfg, store, nil, nil, nil),
		Now:     time.Now,
		Logger:  slog.Default(),
	})
	return RepoEntry{
		Path:                "/tmp/proj",
		Project:             "proj",
		Store:               store,
		LinearTriggerPoller: poller,
		LinearTriggerConfig: cfg,
	}
}

func monitorTriggerConfig() lineartrigger.Config {
	return lineartrigger.Config{
		Enabled:          true,
		PollInterval:     time.Minute,
		MaxIssuesPerPoll: 10,
		Routes: []lineartrigger.Route{{
			TeamID:       "team-1",
			Topic:        "linear",
			BranchPrefix: "linear/",
		}},
		Verbs: map[lineartrigger.Verb]bool{
			lineartrigger.VerbCreate: true,
			lineartrigger.VerbPlan:   true,
			lineartrigger.VerbStart:  true,
			lineartrigger.VerbLink:   true,
			lineartrigger.VerbHelp:   true,
			lineartrigger.VerbStatus: true,
		},
		Labels: lineartrigger.LabelMap{Create: "label-create"},
	}
}

func requireMonitorAuditCount(t *testing.T, logger *auditlog.SQLiteLogger, kind auditlog.EventKind, needle string, want int) {
	t.Helper()
	events, err := logger.Query(auditlog.QueryFilter{
		Project: "proj",
		Kinds:   []auditlog.EventKind{kind},
		Limit:   100,
	})
	require.NoError(t, err)
	got := 0
	for _, event := range events {
		if strings.Contains(event.Detail, needle) {
			got++
		}
	}
	assert.Equal(t, want, got, "kind=%s needle=%s", kind, needle)
}

func monitorIssue() linear.Issue {
	return linear.Issue{
		ID:          "issue-1",
		Identifier:  "LIN-123",
		Title:       "Linear trigger task",
		Description: "Create this task",
		URL:         "https://linear.app/acme/issue/LIN-123",
		Team:        &linear.Team{ID: "team-1"},
		Labels:      []linear.Label{{ID: "label-create", Name: "kasmos create"}},
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
}

type linearTriggerMonitorLinearClient struct {
	issue       linear.Issue
	comments    map[string][]linear.Comment
	issuesErr   error
	issuesCalls int
	issueCalls  int
}

func (c *linearTriggerMonitorLinearClient) Issue(_ context.Context, _ string) (*linear.Issue, error) {
	c.issueCalls++
	return &c.issue, nil
}

func (c *linearTriggerMonitorLinearClient) Issues(_ context.Context, _ linear.IssueQuery) ([]linear.Issue, linear.PageInfo, error) {
	c.issuesCalls++
	if c.issuesErr != nil {
		return nil, linear.PageInfo{}, c.issuesErr
	}
	return []linear.Issue{c.issue}, linear.PageInfo{}, nil
}

func (c *linearTriggerMonitorLinearClient) Comments(_ context.Context, issueID string, _ linear.PageOptions) ([]linear.Comment, linear.PageInfo, error) {
	return append([]linear.Comment(nil), c.comments[issueID]...), linear.PageInfo{}, nil
}

func (c *linearTriggerMonitorLinearClient) IssueLabel(_ context.Context, labelID string) (*linear.Label, error) {
	return &linear.Label{ID: labelID}, nil
}

func (c *linearTriggerMonitorLinearClient) RemoveLabelFromIssue(_ context.Context, _ string, _ []string) error {
	return nil
}

func (c *linearTriggerMonitorLinearClient) CreateComment(_ context.Context, _, _ string) (*linear.Comment, error) {
	return &linear.Comment{ID: "comment-1"}, nil
}

func (c *linearTriggerMonitorLinearClient) CreateCommentReaction(_ context.Context, _, _ string) error {
	return nil
}

func (c *linearTriggerMonitorLinearClient) UpdateIssue(_ context.Context, _ string, _ linear.UpdateIssueInput) (*linear.Issue, error) {
	return &c.issue, nil
}
