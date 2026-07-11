package app

import (
	"context"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"github.com/kastheco/kasmos/config"
	"github.com/kastheco/kasmos/config/auditlog"
	"github.com/kastheco/kasmos/config/linearreceipt"
	"github.com/kastheco/kasmos/config/taskfsm"
	"github.com/kastheco/kasmos/config/taskstate"
	"github.com/kastheco/kasmos/config/taskstore"
	"github.com/kastheco/kasmos/internal/linear"
	prsvc "github.com/kastheco/kasmos/orchestration/pr"
	"github.com/kastheco/kasmos/ui"
	"github.com/kastheco/kasmos/ui/overlay"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type countingLinearClient struct {
	mu       sync.Mutex
	comments []string
}

func (c *countingLinearClient) CreateComment(_ context.Context, _, body string) (*linear.Comment, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.comments = append(c.comments, body)
	return &linear.Comment{ID: "comment", URL: "https://linear.test/comment/comment", Body: body}, nil
}

func (c *countingLinearClient) UpdateIssue(context.Context, string, linear.UpdateIssueInput) (*linear.Issue, error) {
	return &linear.Issue{ID: "issue"}, nil
}

func (c *countingLinearClient) count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.comments)
}

func newLinearReceiptTestHome(t *testing.T, enabled bool) (*home, *countingLinearClient, string) {
	t.Helper()
	dir := t.TempDir()
	plansDir := filepath.Join(dir, "docs", "plans")
	planFile := "linear-plan"
	store := taskstore.NewTestSQLiteStore(t)
	require.NoError(t, store.Create("test", taskstore.TaskEntry{
		Filename:    planFile,
		Status:      taskstore.StatusPlanning,
		Description: "linear receipts",
		Branch:      "plan/linear-receipts",
	}))
	require.NoError(t, store.SetLinearLink("test", planFile, taskstore.LinearLink{
		LinearIssueID:    "issue-123",
		LinearIdentifier: "KAS-123",
	}))
	ps, err := taskstate.Load(store, "test", plansDir)
	require.NoError(t, err)

	client := &countingLinearClient{}
	appConfig := &config.Config{
		LinearReceipts: linearreceipt.Config{
			Enabled:       enabled,
			Events:        map[taskfsm.Event]bool{taskfsm.ImplementStart: true, taskfsm.ImplementFinished: true},
			PRReceipts:    true,
			MergeReceipts: true,
			CancelReceipt: true,
		},
	}
	var receiptHook *linearreceipt.Hook
	if enabled {
		receiptHook = linearreceipt.NewHook(appConfig.LinearReceipts, store, client, auditlog.NopLogger(), "test")
	}
	registry := taskfsm.NewHookRegistry()
	if receiptHook != nil {
		registry.Add(receiptHook, eventsFromConfig(appConfig.LinearReceipts))
	}
	fsm := taskfsm.New(store, "test", plansDir)
	fsm.SetHooks(registry)

	sp := spinner.New(spinner.WithSpinner(spinner.Dot))
	return &home{
		appConfig:         appConfig,
		taskState:         ps,
		taskStore:         store,
		taskStoreProject:  "test",
		taskStateDir:      plansDir,
		fsm:               fsm,
		linearReceiptHook: receiptHook,
		nav:               ui.NewNavigationPanel(&sp),
		tabbedWindow:      ui.NewTabbedWindow(ui.NewPreviewPane(), ui.NewInfoPane()),
		toastManager:      overlay.NewToastManager(&sp),
		activeRepoPath:    dir,
	}, client, planFile
}

func requireLinearCommentCount(t *testing.T, client *countingLinearClient, want int) {
	t.Helper()
	require.Eventually(t, func() bool {
		return client.count() == want
	}, time.Second, 10*time.Millisecond)
}

func TestLinearReceiptHook_TUIDirectTransitionPostsOnce(t *testing.T) {
	t.Parallel()
	m, client, planFile := newLinearReceiptTestHome(t, true)

	require.NoError(t, m.fsmSetImplementing(planFile))

	requireLinearCommentCount(t, client, 1)
}

func TestLinearReceiptHook_ProcessorTransitionPostsOnce(t *testing.T) {
	t.Parallel()
	m, client, planFile := newLinearReceiptTestHome(t, true)
	require.NoError(t, m.fsmSetImplementing(planFile))
	requireLinearCommentCount(t, client, 1)

	actions := m.ensureProcessor().ProcessFSMSignals([]taskfsm.Signal{{
		TaskFile: planFile,
		Event:    taskfsm.ImplementFinished,
	}})

	assert.NotEmpty(t, actions)
	requireLinearCommentCount(t, client, 2)
}

func TestLinearReceiptHook_PRCreatedPostsAfterPRURLPersisted(t *testing.T) {
	t.Parallel()
	m, client, planFile := newLinearReceiptTestHome(t, true)
	require.NotNil(t, m.linearReceiptHook)
	require.NoError(t, m.taskStore.SetPRURL("test", planFile, "https://github.test/pr/1"))

	_, cmd := m.Update(prCreatedForPlanMsg{planFile: planFile, url: "https://github.test/pr/1", outcome: prsvc.OutcomeCreated})
	require.NotNil(t, cmd)
	_ = cmd()

	entry, err := m.taskStore.Get("test", planFile)
	require.NoError(t, err)
	assert.Equal(t, "https://github.test/pr/1", entry.PRURL)
	requireLinearCommentCount(t, client, 1)
}

func TestLinearReceiptHook_DisabledDoesNotPost(t *testing.T) {
	t.Parallel()
	m, client, planFile := newLinearReceiptTestHome(t, false)

	require.NoError(t, m.fsmSetImplementing(planFile))
	_, cmd := m.Update(prCreatedForPlanMsg{planFile: planFile, url: "https://github.test/pr/1"})
	require.NotNil(t, cmd)
	_ = cmd()

	assert.Never(t, func() bool {
		return client.count() > 0
	}, 100*time.Millisecond, 10*time.Millisecond)
}

var _ tea.Model = (*home)(nil)
