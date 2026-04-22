package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"charm.land/bubbles/v2/spinner"
	"github.com/kastheco/kasmos/config"
	"github.com/kastheco/kasmos/config/taskstate"
	"github.com/kastheco/kasmos/orchestration"
	"github.com/kastheco/kasmos/session"
	"github.com/kastheco/kasmos/ui"
	"github.com/kastheco/kasmos/ui/overlay"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewHomeInitializesNavigationPanel(t *testing.T) {
	// serial: calls production newHome constructor which connects to shared config/db
	h := newHome(context.Background(), "opencode", false, "")
	require.NotNil(t, h.nav)
}

func TestUpdateSidebarTasks_CancelledTaskAppearsInHistory(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	plansDir := filepath.Join(dir, "docs", "plans")
	require.NoError(t, os.MkdirAll(plansDir, 0o755))

	store, ps, fsm := newSharedStoreForTest(t, plansDir)
	planFile := "cancelled-feature.md"
	require.NoError(t, ps.Register(planFile, "cancelled feature", "plan/cancelled-feature", time.Now()))
	seedPlanStatus(t, ps, planFile, taskstate.StatusCancelled)

	sp := spinner.New(spinner.WithSpinner(spinner.Dot))
	h := &home{
		ctx:                   context.Background(),
		state:                 stateDefault,
		appConfig:             config.DefaultConfig(),
		nav:                   ui.NewNavigationPanel(&sp),
		menu:                  ui.NewMenu(),
		tabbedWindow:          ui.NewTabbedWindow(ui.NewPreviewPane(), ui.NewInfoPane()),
		toastManager:          overlay.NewToastManager(&sp),
		overlays:              overlay.NewManager(),
		taskState:             ps,
		taskStore:             store,
		taskStoreProject:      "test",
		taskStateDir:          plansDir,
		fsm:                   fsm,
		pendingReviewFeedback: make(map[string]string),
		plannerPrompted:       make(map[string]bool),
		coderPushPrompted:     make(map[string]bool),
		waveOrchestrators:     make(map[string]*orchestration.WaveOrchestrator),
		instanceFinalizers:    make(map[*session.Instance]func()),
		activeRepoPath:        dir,
		program:               "claude",
	}

	h.updateSidebarTasks()

	// Cancelled plans live under history and become selectable when that section is expanded.
	require.True(t, h.nav.SelectByID(ui.SidebarPlanHistoryToggle), "history toggle must exist")
	require.True(t, h.nav.ToggleSelectedExpand(), "history toggle must expand")

	ok := h.nav.SelectByID(ui.SidebarPlanPrefix + planFile)
	require.True(t, ok, "cancelled plan must produce a selectable history row")
	assert.True(t, h.nav.IsSelectedPlanHeader(), "selected cancelled history row must satisfy IsSelectedPlanHeader")
}
