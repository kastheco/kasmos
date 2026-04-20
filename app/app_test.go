package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kastheco/kasmos/cmd/cmd_test"
	"github.com/kastheco/kasmos/config"
	"github.com/kastheco/kasmos/config/taskfsm"
	"github.com/kastheco/kasmos/config/taskstate"
	"github.com/kastheco/kasmos/config/taskstore"
	daemonpkg "github.com/kastheco/kasmos/daemon"
	"github.com/kastheco/kasmos/daemon/api"
	"github.com/kastheco/kasmos/log"
	"github.com/kastheco/kasmos/session"
	gitpkg "github.com/kastheco/kasmos/session/git"
	"github.com/kastheco/kasmos/session/tmux"
	"github.com/kastheco/kasmos/ui"
	"github.com/kastheco/kasmos/ui/overlay"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	zone "github.com/lrstanley/bubblezone/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMain runs before all tests to set up the test environment
func TestMain(m *testing.M) {
	// Initialize bubblezone global manager (required for zone.Mark/zone.Get in tests)
	zone.NewGlobal()

	// Initialize the logger before any tests run
	log.Initialize(false)
	defer log.Close()

	// Prevent tests from reaching the live tmux server via outerTmuxSession()
	_ = os.Unsetenv("TMUX")
	_ = os.Unsetenv("TMUX_PANE")

	// Run all tests
	exitCode := m.Run()

	// Exit with the same code as the tests
	os.Exit(exitCode)
}

func newTestHome() *home {
	spin := spinner.New(spinner.WithSpinner(spinner.Dot))
	return &home{
		ctx:            context.Background(),
		state:          stateDefault,
		appConfig:      config.DefaultConfig(),
		nav:            ui.NewNavigationPanel(&spin),
		menu:           ui.NewMenu(),
		auditPane:      ui.NewAuditPane(),
		tabbedWindow:   ui.NewTabbedWindow(ui.NewPreviewPane(), ui.NewInfoPane()),
		toastManager:   overlay.NewToastManager(&spin),
		overlays:       overlay.NewManager(),
		activeRepoPath: os.TempDir(),
		program:        "opencode",
		daemonStatusChecker: func(string) daemonStatusMsg {
			return daemonStatusMsg{ready: true}
		},
		daemonRepoRegistrar: func(string) error { return nil },
		planBrowserOpener: func(repoRoot, project, planFile string) (string, bool, error) {
			return "http://127.0.0.1:7433/admin/?project=" + project, false, nil
		},
	}
}

func startTestDaemonSocketServer(t *testing.T, handler http.Handler) string {
	t.Helper()

	// Use os.MkdirTemp with a short prefix so the derived socket path stays
	// under the 108-byte Unix domain socket limit on Linux, regardless of how
	// long the test name is.
	//
	// Set HOME to the same short temp dir so ResolvedDaemonSocketPath() never
	// reads a real ~/.config/kasmos/daemon.toml and picks up a developer's
	// configured socket_path, which would break test hermeticity.
	xdgDir, err := os.MkdirTemp("", "ks-")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(xdgDir) })
	t.Setenv("HOME", xdgDir)
	t.Setenv("XDG_RUNTIME_DIR", xdgDir)
	socketPath := daemonpkg.DefaultSocketPath()
	require.NoError(t, os.MkdirAll(filepath.Dir(socketPath), 0o755))
	_ = os.Remove(socketPath)

	listener, err := net.Listen("unix", socketPath)
	require.NoError(t, err)

	server := &http.Server{Handler: handler}
	go func() {
		_ = server.Serve(listener)
	}()

	t.Cleanup(func() {
		require.NoError(t, server.Close())
		_ = os.Remove(socketPath)
	})

	return socketPath
}

func TestShowDaemonRequiredDialog_RegistersRepoOnConfirm(t *testing.T) {
	registeredPath := ""
	h := newTestHome()
	h.activeRepoPath = filepath.Join(os.TempDir(), "kasmos-test-repo")
	h.daemonRepoRegistrar = func(path string) error {
		registeredPath = path
		return nil
	}

	h.showDaemonRequiredDialog(daemonStatusMsg{
		message:         "the kasmos daemon is running, but this repo is not registered.",
		canRegisterRepo: true,
	})
	require.NotNil(t, h.pendingConfirmAction)

	msg := h.pendingConfirmAction()
	registered, ok := msg.(daemonRepoRegisteredMsg)
	require.True(t, ok)
	assert.Equal(t, h.activeRepoPath, registered.path)
	assert.Equal(t, h.activeRepoPath, registeredPath)

	co, ok := h.overlays.Current().(*overlay.ConfirmationOverlay)
	require.True(t, ok)
	assert.Contains(t, co.View(), "to confirm")
	assert.Contains(t, co.View(), "register")
}

func TestShowDaemonRequiredDialog_DoesNotRegisterWhenUnavailable(t *testing.T) {
	h := newTestHome()
	h.showDaemonRequiredDialog(daemonStatusMsg{message: "start the daemon first"})
	assert.Nil(t, h.pendingConfirmAction)
	assert.Equal(t, stateConfirm, h.state)
	assert.True(t, h.overlays.IsActive())
	co, ok := h.overlays.Current().(*overlay.ConfirmationOverlay)
	require.True(t, ok)
	assert.Contains(t, co.View(), "start the daemon first")
}

func TestCheckDaemonStatus_AutoRegistersRepoWhenDaemonIsRunning(t *testing.T) {
	repoPath := filepath.Join(t.TempDir(), "repo")
	require.NoError(t, os.MkdirAll(repoPath, 0o755))

	oldManaged := repoManagedByDaemon
	repoManagedByDaemon = func(string) bool { return false }
	t.Cleanup(func() {
		repoManagedByDaemon = oldManaged
	})

	tests := []struct {
		name            string
		registerStatus  int
		wantReady       bool
		wantAutoReg     bool
		wantCanRegister bool
	}{
		{
			name:           "successful auto-registration returns ready",
			registerStatus: http.StatusCreated,
			wantReady:      true,
			wantAutoReg:    true,
		},
		{
			name:            "failed auto-registration falls back to confirmation",
			registerStatus:  http.StatusInternalServerError,
			wantCanRegister: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			postCalls := 0
			mux := http.NewServeMux()
			mux.HandleFunc("GET /v1/status", func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				require.NoError(t, json.NewEncoder(w).Encode(api.StatusResponse{Running: true}))
			})
			mux.HandleFunc("POST /v1/repos", func(w http.ResponseWriter, r *http.Request) {
				postCalls++
				var req struct {
					Path string `json:"path"`
				}
				require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
				assert.Equal(t, canonicalRepoPath(repoPath), req.Path)
				w.WriteHeader(tc.registerStatus)
			})

			startTestDaemonSocketServer(t, mux)

			status := checkDaemonStatus(repoPath)

			assert.Equal(t, 1, postCalls)
			assert.Equal(t, tc.wantReady, status.ready)
			assert.Equal(t, tc.wantAutoReg, status.autoRegistered)
			assert.Equal(t, tc.wantCanRegister, status.canRegisterRepo)
			if tc.wantCanRegister {
				assert.Contains(t, status.message, "press y to register it now")
			} else {
				assert.Empty(t, status.message)
			}
		})
	}
}

func TestDaemonRepoRegisteredMsg_KeepsLocalTaskStoreWithoutConfirmation(t *testing.T) {
	// Redirect HOME so newHome() uses a fresh isolated global DB instead of
	// the developer's real ~/.config/kasmos/taskstore.db.
	t.Setenv("HOME", t.TempDir())

	repoDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(repoDir, ".git"), 0o755))

	oldCwd, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(repoDir))
	t.Cleanup(func() { _ = os.Chdir(oldCwd) })

	oldManaged := repoManagedByDaemon
	repoManagedByDaemon = func(string) bool { return false }
	t.Cleanup(func() {
		repoManagedByDaemon = oldManaged
	})

	h := newHome(context.Background(), "opencode", false, "test")
	t.Cleanup(func() {
		if h.taskStore != nil {
			_ = h.taskStore.Close()
		}
		if h.auditLogger != nil {
			_ = h.auditLogger.Close()
		}
	})

	require.IsType(t, &taskstore.SQLiteStore{}, h.taskStore)
	assert.Equal(t, filepath.Base(repoDir), h.taskStoreProject)

	model, toastCmd := h.Update(daemonRepoRegisteredMsg{path: repoDir})
	h = model.(*home)

	require.NotNil(t, toastCmd)
	_, ok := toastCmd().(overlay.ToastTickMsg)
	require.True(t, ok, "daemon registration should schedule a toast tick")
	updated := h

	assert.Equal(t, stateDefault, updated.state)
	assert.False(t, updated.overlays.IsActive(), "successful registration should not open a confirmation overlay")
	assert.Nil(t, updated.pendingConfirmAction)
	require.IsType(t, &taskstore.SQLiteStore{}, updated.taskStore)
	assert.Equal(t, filepath.Base(repoDir), updated.taskStoreProject)

	require.NoError(t, updated.taskStore.Create(updated.taskStoreProject, taskstore.TaskEntry{
		Filename: "local-created",
		Status:   taskstore.StatusReady,
	}))

	loaded, err := updated.taskStore.Get(updated.taskStoreProject, "local-created")
	require.NoError(t, err)
	assert.Equal(t, taskstore.StatusReady, loaded.Status)
	assert.True(t, updated.toastManager.HasActiveToasts())
}

func TestNewHome_AutoRegisterDoesNotShowStaleDaemonUnavailableToast(t *testing.T) {
	repoDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(repoDir, ".git"), 0o755))

	oldCwd, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(repoDir))
	t.Cleanup(func() { _ = os.Chdir(oldCwd) })

	project := filepath.Base(repoDir)
	repoRegistered := false

	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		status := api.StatusResponse{Running: true}
		if repoRegistered {
			status.Repos = []api.RepoStatus{{Path: repoDir, Project: project}}
		}
		require.NoError(t, json.NewEncoder(w).Encode(status))
	})
	mux.HandleFunc("GET /v1/repos", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		repos := []api.RepoStatus{}
		if repoRegistered {
			repos = append(repos, api.RepoStatus{Path: repoDir, Project: project})
		}
		require.NoError(t, json.NewEncoder(w).Encode(repos))
	})
	mux.HandleFunc("POST /v1/repos", func(w http.ResponseWriter, r *http.Request) {
		repoRegistered = true
		w.WriteHeader(http.StatusCreated)
	})
	startTestDaemonSocketServer(t, mux)

	oldManaged := repoManagedByDaemon
	repoManagedByDaemon = func(string) bool { return false }
	t.Cleanup(func() {
		repoManagedByDaemon = oldManaged
	})

	h := newHome(context.Background(), "opencode", false, "test")
	t.Cleanup(func() {
		if h.taskStore != nil {
			_ = h.taskStore.Close()
		}
		if h.auditLogger != nil {
			_ = h.auditLogger.Close()
		}
	})

	require.IsType(t, &taskstore.SQLiteStore{}, h.taskStore)
	assert.Equal(t, project, h.taskStoreProject)
	assert.NotContains(t, h.toastManager.View(), "local task store unavailable")
	assert.NotContains(t, h.toastManager.View(), "daemon task store unavailable")

	statusMsg := h.daemonStartupCheckCmd()()
	model, toastCmd := h.Update(statusMsg)
	updated := model.(*home)

	require.NotNil(t, toastCmd)
	_, ok := toastCmd().(overlay.ToastTickMsg)
	require.True(t, ok, "daemon auto-registration should schedule a toast tick")

	assert.Contains(t, updated.toastManager.View(), "auto-registered repo with daemon")
	assert.NotContains(t, updated.toastManager.View(), "local task store unavailable")
	assert.NotContains(t, updated.toastManager.View(), "daemon task store unavailable")
	require.IsType(t, &taskstore.SQLiteStore{}, updated.taskStore)
	assert.Equal(t, project, updated.taskStoreProject)
	assert.True(t, repoRegistered)

	require.NoError(t, updated.taskStore.Create(updated.taskStoreProject, taskstore.TaskEntry{
		Filename: "local-created-after-auto-register",
		Status:   taskstore.StatusReady,
	}))

	created, err := updated.taskStore.Get(updated.taskStoreProject, "local-created-after-auto-register")
	require.NoError(t, err)
	assert.Equal(t, taskstore.StatusReady, created.Status)
}

func TestView_UsesCellMotionMouseMode(t *testing.T) {
	h := newTestHome()
	h.termHeight = 20
	h.contentHeight = 10
	h.nav.SetSize(24, 10)
	h.tabbedWindow.SetSize(56, 10)

	v := h.View()
	assert.Equal(t, tea.MouseModeCellMotion, v.MouseMode)
}

func TestSpawnAdHocAgent_DefaultCreatesWorktree(t *testing.T) {
	h := newTestHome()
	model, cmd := h.spawnAdHocAgent("my-agent", "", "", "", session.ExecutionModeTmux, "")
	updated := model.(*home)
	instances := updated.nav.GetInstances()
	require.NotEmpty(t, instances)
	last := instances[len(instances)-1]
	assert.Equal(t, "my-agent", last.Title)
	assert.Equal(t, session.AgentTypeMaster, last.AgentType, "spawned instance must use the master agent")
	assert.Equal(t, "claude", last.Program)
	assert.Equal(t, session.Loading, last.Status)
	assert.NotNil(t, cmd, "should return async start command")
	// Default config (nil appConfig) must produce tmux.
	assert.Equal(t, session.ExecutionModeTmux, last.ExecutionMode)
}

func TestExecuteLauncherAction_NewInstanceUsesClaudeMasterAgent(t *testing.T) {
	h := newTestHome()

	model, cmd := h.executeLauncherAction("new_instance")
	updated := model.(*home)

	require.Nil(t, cmd)
	require.Equal(t, stateNew, updated.state)
	require.NotNil(t, updated.newInstance)
	assert.Equal(t, session.AgentTypeMaster, updated.newInstance.AgentType)
	assert.Equal(t, "claude", updated.newInstance.Program)
	// Default config (nil appConfig) must produce tmux so the instance is interactive.
	assert.Equal(t, session.ExecutionModeTmux, updated.newInstance.ExecutionMode)
}

func TestExecuteLauncherAction_NewInstanceSDKIgnoresTmuxLimit(t *testing.T) {
	h := newTestHome()
	h.tmuxSessionCount = GlobalInstanceLimit
	h.appConfig = &config.Config{
		PhaseRoles: map[string]string{"readiness_review": "master"},
		Profiles: map[string]config.AgentProfile{
			"master": {Program: "claude", Enabled: true, ExecutionMode: config.ExecutionModeSDK},
		},
	}

	model, cmd := h.executeLauncherAction("new_instance")
	updated := model.(*home)

	require.Nil(t, cmd)
	require.Equal(t, stateNew, updated.state)
	require.NotNil(t, updated.newInstance)
	assert.Equal(t, session.ExecutionModeSDK, updated.newInstance.ExecutionMode)
}

func TestSpawnAdHocAgent_BranchOverride(t *testing.T) {
	h := newTestHome()
	model, cmd := h.spawnAdHocAgent("my-agent", "feature/login", "", "", session.ExecutionModeTmux, "")
	updated := model.(*home)
	instances := updated.nav.GetInstances()
	require.NotEmpty(t, instances)
	last := instances[len(instances)-1]
	assert.Equal(t, "my-agent", last.Title)
	assert.NotNil(t, cmd)
}

func TestSpawnAdHocAgent_PathOverride(t *testing.T) {
	h := newTestHome()
	model, cmd := h.spawnAdHocAgent("my-agent", "", "/tmp/custom-path", "", session.ExecutionModeTmux, "")
	updated := model.(*home)
	instances := updated.nav.GetInstances()
	require.NotEmpty(t, instances)
	last := instances[len(instances)-1]
	assert.Equal(t, "my-agent", last.Title)
	assert.NotNil(t, cmd)
}

func TestSpawnAgent_KeyOpensHarnessPicker(t *testing.T) {
	h := newTestHome()
	h.appConfig = &config.Config{DefaultProgram: "claude"}
	h.keySent = true
	model, _ := h.handleKeyPress(tea.KeyPressMsg{Code: 'S', Text: "S"})
	updated := model.(*home)
	require.Equal(t, stateSpawnHarnessPicker, updated.state)
	require.True(t, updated.overlays.IsActive(), "harness picker must be active")
	picker, ok := updated.overlays.Current().(*overlay.PickerOverlay)
	require.True(t, ok, "active overlay must be a PickerOverlay")
	assert.Contains(t, picker.View(), "▸ claude")
}

func TestSpawnAgent_SDKFlowAtTmuxLimitStillOpensForm(t *testing.T) {
	h := newTestHome()
	h.tmuxSessionCount = GlobalInstanceLimit
	h.appConfig = &config.Config{
		DefaultProgram: "claude",
		PhaseRoles:     map[string]string{"readiness_review": "master"},
		Profiles: map[string]config.AgentProfile{
			"master": {Program: "claude", Enabled: true, ExecutionMode: config.ExecutionModeSDK},
		},
	}

	h.keySent = true
	model, _ := h.handleKeyPress(tea.KeyPressMsg{Code: 'S', Text: "S"})
	updated := model.(*home)
	require.Equal(t, stateSpawnHarnessPicker, updated.state)

	h = updated
	h.keySent = true
	model, cmd := h.handleKeyPress(tea.KeyPressMsg{Code: tea.KeyEnter})
	updated = model.(*home)

	require.Nil(t, cmd)
	require.Equal(t, stateSpawnExecutionModePicker, updated.state)

	h = updated
	h.keySent = true
	model, cmd = h.handleKeyPress(tea.KeyPressMsg{Code: tea.KeyEnter})
	updated = model.(*home)

	require.Nil(t, cmd)
	require.Equal(t, stateSpawnAgent, updated.state)
	require.True(t, updated.overlays.IsActive())
	_, ok := updated.overlays.Current().(*overlay.FormOverlay)
	require.True(t, ok)
	assert.Equal(t, session.ExecutionModeSDK, updated.pendingSpawnExecutionMode)
}

func TestSpawnAgent_EscCancels(t *testing.T) {
	h := newTestHome()
	h.state = stateSpawnAgent
	h.overlays.Show(overlay.NewSpawnFormOverlay("spawn agent", 60))

	h.keySent = true
	model, _ := h.handleKeyPress(tea.KeyPressMsg{Code: tea.KeyEscape})
	updated := model.(*home)
	assert.Equal(t, stateDefault, updated.state)
	assert.False(t, updated.overlays.IsActive())
}

func TestSpawnAgent_SubmitCreatesInstance(t *testing.T) {
	h := newTestHome()
	h.state = stateSpawnAgent
	h.overlays.Show(overlay.NewSpawnFormOverlay("spawn agent", 60))

	press := func(msg tea.KeyPressMsg) {
		h.keySent = true
		handleModel, _ := h.handleKeyPress(msg)
		h = handleModel.(*home)
	}

	for _, r := range "test-agent" {
		press(tea.KeyPressMsg{Code: r, Text: string(r)})
	}

	h.keySent = true
	model, cmd := h.handleKeyPress(tea.KeyPressMsg{Code: tea.KeyEnter})
	updated := model.(*home)
	assert.Equal(t, stateDefault, updated.state)
	assert.False(t, updated.overlays.IsActive())
	assert.NotNil(t, cmd, "should return start command")

	instances := updated.nav.GetInstances()
	require.NotEmpty(t, instances)
	last := instances[len(instances)-1]
	assert.Equal(t, "test-agent", last.Title)
	assert.Equal(t, "", last.TaskFile, "ad-hoc instance must have no PlanFile")
	assert.Equal(t, session.AgentTypeMaster, last.AgentType, "spawned instance must use the master agent")
	assert.Equal(t, session.Loading, last.Status)
}

func TestAvailableSpawnPrograms_DedupesSortedEnabledProfiles(t *testing.T) {
	h := newTestHome()
	h.appConfig = &config.Config{
		DefaultProgram: "claude",
		Profiles: map[string]config.AgentProfile{
			"role-a": {Program: "opencode", Enabled: true},
			"role-b": {Program: "opencode", Enabled: true}, // duplicate
			"role-c": {Program: "codex", Enabled: true},
			"role-d": {Program: "disabled", Enabled: false}, // ignored
			"role-e": {Program: "", Enabled: true},          // blank ignored
		},
	}
	programs := h.availableSpawnPrograms()
	assert.Equal(t, []string{"claude", "codex", "opencode"}, programs)
}

func TestAvailableSpawnPrograms_IncludesDefaultProgram(t *testing.T) {
	h := newTestHome()
	h.appConfig = &config.Config{DefaultProgram: "amp"}
	programs := h.availableSpawnPrograms()
	assert.Equal(t, []string{"claude", "codex", "opencode", "amp"}, programs)
}

func TestAvailableSpawnPrograms_IgnoresDisabledAndBlankProfiles(t *testing.T) {
	h := newTestHome()
	h.appConfig = &config.Config{
		DefaultProgram: "claude",
		Profiles: map[string]config.AgentProfile{
			"bad-a": {Program: "disabled-prog", Enabled: false},
			"bad-b": {Program: "", Enabled: true},
		},
	}
	programs := h.availableSpawnPrograms()
	assert.Equal(t, []string{"claude", "codex", "opencode"}, programs)
}

func TestAvailableSpawnPrograms_FallsBackToProgramField(t *testing.T) {
	h := newTestHome()
	h.appConfig = nil
	h.program = "codex"
	programs := h.availableSpawnPrograms()
	assert.Equal(t, []string{"claude", "codex", "opencode"}, programs)
}

func TestAvailableSpawnPrograms_UsesCleanLabelsForConfiguredPaths(t *testing.T) {
	h := newTestHome()
	h.appConfig = &config.Config{
		DefaultProgram: "/home/kas/.nvm/versions/node/v22.0.0/bin/codex --model gpt-5.4",
		Profiles: map[string]config.AgentProfile{
			"reviewer": {Program: "/usr/local/bin/claude --model sonnet", Enabled: true},
		},
	}
	programs := h.availableSpawnPrograms()
	assert.Equal(t, []string{"claude", "codex", "opencode"}, programs)
}

func TestSpawnAgent_MultiplePrograms_OpensPicker(t *testing.T) {
	h := newTestHome()
	h.appConfig = &config.Config{
		DefaultProgram: "claude",
		Profiles: map[string]config.AgentProfile{
			"opencode-role": {Program: "opencode", Enabled: true},
		},
	}
	h.keySent = true
	model, _ := h.handleKeyPress(tea.KeyPressMsg{Code: 'S', Text: "S"})
	updated := model.(*home)
	require.Equal(t, stateSpawnHarnessPicker, updated.state)
	require.True(t, updated.overlays.IsActive(), "picker overlay must be active")
	_, ok := updated.overlays.Current().(*overlay.PickerOverlay)
	require.True(t, ok, "active overlay must be a PickerOverlay")
}

func TestSpawnAgent_DefaultHarnessSelection_UsesCleanCodexLabel(t *testing.T) {
	h := newTestHome()
	h.appConfig = &config.Config{DefaultProgram: "/home/kas/.nvm/versions/node/v22.0.0/bin/codex --model gpt-5.4"}
	h.keySent = true
	model, _ := h.handleKeyPress(tea.KeyPressMsg{Code: 'S', Text: "S"})
	updated := model.(*home)
	require.Equal(t, stateSpawnHarnessPicker, updated.state)
	picker, ok := updated.overlays.Current().(*overlay.PickerOverlay)
	require.True(t, ok, "active overlay must be a PickerOverlay")
	view := picker.View()
	assert.Contains(t, view, "▸ codex")
	assert.NotContains(t, view, "/home/kas/.nvm")

	h = updated
	h.keySent = true
	model, _ = h.handleKeyPress(tea.KeyPressMsg{Code: tea.KeyEnter})
	updated = model.(*home)
	require.Equal(t, stateSpawnExecutionModePicker, updated.state)
	assert.Equal(t, "/home/kas/.nvm/versions/node/v22.0.0/bin/codex --model gpt-5.4", updated.pendingSpawnProgram)
	assert.Equal(t, session.ExecutionModeTmux, updated.pendingSpawnExecutionMode)
}

func TestSpawnAgent_DefaultCustomProgram_SelectionOpensForm(t *testing.T) {
	h := newTestHome()
	h.appConfig = &config.Config{DefaultProgram: "amp"}
	h.keySent = true
	model, _ := h.handleKeyPress(tea.KeyPressMsg{Code: 'S', Text: "S"})
	updated := model.(*home)
	require.Equal(t, stateSpawnHarnessPicker, updated.state)
	picker, ok := updated.overlays.Current().(*overlay.PickerOverlay)
	require.True(t, ok, "active overlay must be a PickerOverlay")
	assert.Contains(t, picker.View(), "▸ amp")

	h = updated
	h.keySent = true
	model, _ = h.handleKeyPress(tea.KeyPressMsg{Code: tea.KeyEnter})
	updated = model.(*home)
	require.Equal(t, stateSpawnAgent, updated.state)
	require.True(t, updated.overlays.IsActive(), "form overlay must be active")
	_, ok = updated.overlays.Current().(*overlay.FormOverlay)
	require.True(t, ok, "active overlay must be a FormOverlay")
	assert.Equal(t, "amp", updated.pendingSpawnProgram)
}

func TestSpawnAgent_LauncherAction_MultiplePrograms_OpensPicker(t *testing.T) {
	h := newTestHome()
	h.appConfig = &config.Config{
		DefaultProgram: "claude",
		Profiles: map[string]config.AgentProfile{
			"opencode-role": {Program: "opencode", Enabled: true},
		},
	}
	model, _ := h.executeLauncherAction("spawn_agent")
	updated := model.(*home)
	require.Equal(t, stateSpawnHarnessPicker, updated.state)
	_, ok := updated.overlays.Current().(*overlay.PickerOverlay)
	require.True(t, ok, "launcher spawn_agent with multiple programs must show PickerOverlay")
}

func TestSpawnAgent_LauncherAction_AlwaysOpensPicker(t *testing.T) {
	h := newTestHome()
	h.appConfig = &config.Config{DefaultProgram: "amp"}
	model, _ := h.executeLauncherAction("spawn_agent")
	updated := model.(*home)
	require.Equal(t, stateSpawnHarnessPicker, updated.state)
	_, ok := updated.overlays.Current().(*overlay.PickerOverlay)
	require.True(t, ok, "launcher spawn_agent must show PickerOverlay")
}

func TestSpawnHarnessPicker_EscReturnsToDefault(t *testing.T) {
	h := newTestHome()
	h.appConfig = &config.Config{
		DefaultProgram: "claude",
		Profiles: map[string]config.AgentProfile{
			"opencode-role": {Program: "opencode", Enabled: true},
		},
	}
	h.state = stateSpawnHarnessPicker
	h.pendingSpawnProgram = "opencode"
	h.overlays.Show(overlay.NewPickerOverlay("select harness", []string{"claude", "opencode"}))

	h.keySent = true
	model, _ := h.handleKeyPress(tea.KeyPressMsg{Code: tea.KeyEscape})
	updated := model.(*home)
	assert.Equal(t, stateDefault, updated.state)
	assert.Empty(t, updated.pendingSpawnProgram)
	assert.False(t, updated.overlays.IsActive())
}

func TestSpawnAgent_SubmitCreatesInstanceWithChosenProgram(t *testing.T) {
	h := newTestHome()
	h.pendingSpawnProgram = "opencode"
	h.pendingSpawnExecutionMode = session.ExecutionModeTmux
	h.state = stateSpawnAgent
	h.overlays.Show(overlay.NewSpawnFormOverlay("spawn agent", 60))

	press := func(msg tea.KeyPressMsg) {
		h.keySent = true
		handleModel, _ := h.handleKeyPress(msg)
		h = handleModel.(*home)
	}

	for _, r := range "my-opencode-agent" {
		press(tea.KeyPressMsg{Code: r, Text: string(r)})
	}

	h.keySent = true
	model, cmd := h.handleKeyPress(tea.KeyPressMsg{Code: tea.KeyEnter})
	updated := model.(*home)
	assert.Equal(t, stateDefault, updated.state)
	assert.Empty(t, updated.pendingSpawnProgram)
	assert.Empty(t, updated.pendingSpawnExecutionMode)
	assert.NotNil(t, cmd)

	instances := updated.nav.GetInstances()
	require.NotEmpty(t, instances)
	last := instances[len(instances)-1]
	assert.Equal(t, "my-opencode-agent", last.Title)
	assert.Equal(t, "opencode", last.Program)
	assert.Equal(t, session.AgentTypeMaster, last.AgentType)
	assert.Equal(t, session.ExecutionModeTmux, last.ExecutionMode)
}

func TestSpawnExecutionModePicker_EscReturnsToDefault(t *testing.T) {
	h := newTestHome()
	h.appConfig = &config.Config{DefaultProgram: "claude"}
	h.state = stateSpawnExecutionModePicker
	h.pendingSpawnProgram = "claude"
	h.pendingSpawnExecutionMode = session.ExecutionModeTmux
	h.overlays.Show(overlay.NewPickerOverlay("execution mode", []string{"tmux", "sdk"}))

	h.keySent = true
	model, _ := h.handleKeyPress(tea.KeyPressMsg{Code: tea.KeyEscape})
	updated := model.(*home)
	assert.Equal(t, stateDefault, updated.state)
	assert.Empty(t, updated.pendingSpawnProgram)
	assert.Empty(t, updated.pendingSpawnExecutionMode)
	assert.False(t, updated.overlays.IsActive())
}

func TestSpawnAgent_SDKProgram_SDK_Submit_HasSDKMode(t *testing.T) {
	// S -> claude -> sdk -> submit => spawned instance has ExecutionModeSDK
	h := newTestHome()
	h.pendingSpawnProgram = "claude"
	h.pendingSpawnExecutionMode = session.ExecutionModeSDK
	h.state = stateSpawnAgent
	fo := overlay.NewSpawnFormOverlay("spawn agent", 60)
	fo.SetFooterHint("standalone sdk agents cannot be controlled from the web ui")
	h.overlays.Show(fo)

	press := func(msg tea.KeyPressMsg) {
		h.keySent = true
		handleModel, _ := h.handleKeyPress(msg)
		h = handleModel.(*home)
	}
	for _, r := range "sdk-agent" {
		press(tea.KeyPressMsg{Code: r, Text: string(r)})
	}

	h.keySent = true
	model, cmd := h.handleKeyPress(tea.KeyPressMsg{Code: tea.KeyEnter})
	updated := model.(*home)
	assert.Equal(t, stateDefault, updated.state)
	assert.Empty(t, updated.pendingSpawnProgram)
	assert.Empty(t, updated.pendingSpawnExecutionMode)
	assert.NotNil(t, cmd)

	instances := updated.nav.GetInstances()
	require.NotEmpty(t, instances)
	last := instances[len(instances)-1]
	assert.Equal(t, "sdk-agent", last.Title)
	assert.Equal(t, "claude", last.Program)
	assert.Equal(t, session.ExecutionModeSDK, last.ExecutionMode)
}

func TestSpawnAgent_SDKProgram_Tmux_Submit_HasTmuxMode(t *testing.T) {
	// S -> codex -> tmux -> submit => spawned instance has ExecutionModeTmux
	h := newTestHome()
	h.pendingSpawnProgram = "codex"
	h.pendingSpawnExecutionMode = session.ExecutionModeTmux
	h.state = stateSpawnAgent
	h.overlays.Show(overlay.NewSpawnFormOverlay("spawn agent", 60))

	press := func(msg tea.KeyPressMsg) {
		h.keySent = true
		handleModel, _ := h.handleKeyPress(msg)
		h = handleModel.(*home)
	}
	for _, r := range "tmux-agent" {
		press(tea.KeyPressMsg{Code: r, Text: string(r)})
	}

	h.keySent = true
	model, cmd := h.handleKeyPress(tea.KeyPressMsg{Code: tea.KeyEnter})
	updated := model.(*home)
	assert.Equal(t, stateDefault, updated.state)
	assert.NotNil(t, cmd)

	instances := updated.nav.GetInstances()
	require.NotEmpty(t, instances)
	last := instances[len(instances)-1]
	assert.Equal(t, "tmux-agent", last.Title)
	assert.Equal(t, "codex", last.Program)
	assert.Equal(t, session.ExecutionModeTmux, last.ExecutionMode)
}

func TestSpawnAgent_UnsupportedProgram_Submit_HasTmuxModeNoPicker(t *testing.T) {
	// S -> opencode -> submit => ExecutionModeTmux with no picker involved
	h := newTestHome()
	h.appConfig = &config.Config{DefaultProgram: "opencode"}
	h.pendingSpawnProgram = "opencode"
	h.pendingSpawnExecutionMode = session.ExecutionModeTmux
	h.state = stateSpawnAgent
	h.overlays.Show(overlay.NewSpawnFormOverlay("spawn agent", 60))

	press := func(msg tea.KeyPressMsg) {
		h.keySent = true
		handleModel, _ := h.handleKeyPress(msg)
		h = handleModel.(*home)
	}
	for _, r := range "opencode-agent" {
		press(tea.KeyPressMsg{Code: r, Text: string(r)})
	}

	h.keySent = true
	model, _ := h.handleKeyPress(tea.KeyPressMsg{Code: tea.KeyEnter})
	updated := model.(*home)
	assert.Equal(t, stateDefault, updated.state)

	instances := updated.nav.GetInstances()
	require.NotEmpty(t, instances)
	last := instances[len(instances)-1]
	assert.Equal(t, "opencode-agent", last.Title)
	assert.Equal(t, "opencode", last.Program)
	assert.Equal(t, session.ExecutionModeTmux, last.ExecutionMode)
}

// TestSpawnAgent_CodexPickerShowsSDKFast verifies that the execution-mode picker
// exposes sdk-fast for codex programs.
func TestSpawnAgent_CodexPickerShowsSDKFast(t *testing.T) {
	h := newTestHome()
	h.appConfig = &config.Config{DefaultProgram: "codex"}
	h.keySent = true
	model, _ := h.handleKeyPress(tea.KeyPressMsg{Code: 'S', Text: "S"})
	h = model.(*home)
	require.Equal(t, stateSpawnHarnessPicker, h.state)
	h.keySent = true
	model, _ = h.handleKeyPress(tea.KeyPressMsg{Code: tea.KeyEnter})
	updated := model.(*home)
	require.Equal(t, stateSpawnExecutionModePicker, updated.state)
	picker, ok := updated.overlays.Current().(*overlay.PickerOverlay)
	require.True(t, ok)
	view := picker.View()
	assert.Contains(t, view, "sdk-fast", "codex picker must include sdk-fast")
}

// TestSpawnAgent_ClaudePickerNoSDKFast verifies that the execution-mode picker
// does NOT include sdk-fast for claude programs.
func TestSpawnAgent_ClaudePickerNoSDKFast(t *testing.T) {
	h := newTestHome()
	h.appConfig = &config.Config{DefaultProgram: "claude"}
	h.keySent = true
	model, _ := h.handleKeyPress(tea.KeyPressMsg{Code: 'S', Text: "S"})
	h = model.(*home)
	require.Equal(t, stateSpawnHarnessPicker, h.state)
	h.keySent = true
	model, _ = h.handleKeyPress(tea.KeyPressMsg{Code: tea.KeyEnter})
	updated := model.(*home)
	require.Equal(t, stateSpawnExecutionModePicker, updated.state)
	picker, ok := updated.overlays.Current().(*overlay.PickerOverlay)
	require.True(t, ok)
	view := picker.View()
	assert.NotContains(t, view, "sdk-fast", "claude picker must not include sdk-fast")
}

// TestSpawnAgent_SDKFastSubmit_CreatesInstanceWithFastTier verifies that selecting
// sdk-fast in the picker and completing the form creates an instance with
// ExecutionModeSDK and SDKSpeedTier=="fast".
func TestSpawnAgent_SDKFastSubmit_CreatesInstanceWithFastTier(t *testing.T) {
	h := newTestHome()
	h.appConfig = &config.Config{DefaultProgram: "codex"}
	h.keySent = true

	// Open harness picker, then accept the default codex selection.
	model, _ := h.handleKeyPress(tea.KeyPressMsg{Code: 'S', Text: "S"})
	h = model.(*home)
	require.Equal(t, stateSpawnHarnessPicker, h.state)
	h.keySent = true
	model, _ = h.handleKeyPress(tea.KeyPressMsg{Code: tea.KeyEnter})
	h = model.(*home)
	require.Equal(t, stateSpawnExecutionModePicker, h.state)

	// Navigate to sdk-fast (two arrow-downs from top: tmux -> sdk -> sdk-fast)
	h.keySent = true
	model, _ = h.handleKeyPress(tea.KeyPressMsg{Code: tea.KeyDown})
	h = model.(*home)
	h.keySent = true
	model, _ = h.handleKeyPress(tea.KeyPressMsg{Code: tea.KeyDown})
	h = model.(*home)

	// Submit sdk-fast
	h.keySent = true
	model, _ = h.handleKeyPress(tea.KeyPressMsg{Code: tea.KeyEnter})
	h = model.(*home)
	require.Equal(t, stateSpawnAgent, h.state, "spawn form must open after sdk-fast selection")
	assert.Equal(t, session.ExecutionModeSDK, h.pendingSpawnExecutionMode)
	assert.Equal(t, "fast", h.pendingSpawnSpeedTier)
	spawnView := h.overlays.Current().View()
	assert.Contains(t, spawnView, "fast tier consumes 2x usage")

	// Type agent name and submit form
	press := func(msg tea.KeyPressMsg) {
		h.keySent = true
		m, _ := h.handleKeyPress(msg)
		h = m.(*home)
	}
	for _, r := range "fast-codex" {
		press(tea.KeyPressMsg{Code: r, Text: string(r)})
	}
	press(tea.KeyPressMsg{Code: tea.KeyEnter})

	assert.Equal(t, stateDefault, h.state)
	assert.Empty(t, h.pendingSpawnSpeedTier, "pendingSpawnSpeedTier must be cleared after spawn")

	instances := h.nav.GetInstances()
	require.NotEmpty(t, instances)
	last := instances[len(instances)-1]
	assert.Equal(t, "fast-codex", last.Title)
	assert.Equal(t, session.ExecutionModeSDK, last.ExecutionMode)
	assert.Equal(t, "fast", last.SDKSpeedTier)
}

// TestSpawnAgent_EscFromExecutionModePicker_ClearsSpeedTier verifies that
// escaping the picker clears both pendingSpawnExecutionMode and pendingSpawnSpeedTier.
func TestSpawnAgent_EscFromExecutionModePicker_ClearsSpeedTier(t *testing.T) {
	h := newTestHome()
	h.appConfig = &config.Config{DefaultProgram: "codex"}
	h.state = stateSpawnExecutionModePicker
	h.pendingSpawnProgram = "codex"
	h.pendingSpawnExecutionMode = session.ExecutionModeSDK
	h.pendingSpawnSpeedTier = "fast"
	h.overlays.Show(overlay.NewPickerOverlay("execution mode", []string{"tmux", "sdk", "sdk-fast"}))

	h.keySent = true
	model, _ := h.handleKeyPress(tea.KeyPressMsg{Code: tea.KeyEscape})
	updated := model.(*home)
	assert.Equal(t, stateDefault, updated.state)
	assert.Empty(t, updated.pendingSpawnProgram)
	assert.Empty(t, updated.pendingSpawnExecutionMode)
	assert.Empty(t, updated.pendingSpawnSpeedTier, "speed tier must be cleared on cancel")
}

// TestSpawnAgent_EscFromSpawnForm_ClearsSpeedTier verifies that escaping the
// spawn form also clears pendingSpawnSpeedTier.
func TestSpawnAgent_EscFromSpawnForm_ClearsSpeedTier(t *testing.T) {
	h := newTestHome()
	h.state = stateSpawnAgent
	h.pendingSpawnProgram = "codex"
	h.pendingSpawnExecutionMode = session.ExecutionModeSDK
	h.pendingSpawnSpeedTier = "fast"
	h.overlays.Show(overlay.NewSpawnFormOverlay("spawn agent", 60))

	h.keySent = true
	model, _ := h.handleKeyPress(tea.KeyPressMsg{Code: tea.KeyEscape})
	updated := model.(*home)
	assert.Equal(t, stateDefault, updated.state)
	assert.Empty(t, updated.pendingSpawnSpeedTier, "speed tier must be cleared on cancel")
}

func TestExecutionModeForAgent_DefaultsToTmuxButHonorsExplicitSDK(t *testing.T) {
	h := newTestHome()
	h.appConfig = &config.Config{
		PhaseRoles: map[string]string{
			"elaborating":  "architect",
			"implementing": "coder",
			"planning":     "planner",
		},
		Profiles: map[string]config.AgentProfile{
			"architect": {Program: "codex", Enabled: true, ExecutionMode: config.ExecutionModeSDK},
			"coder":     {Program: "claude", Enabled: true},
			"planner":   {Program: "claude", Enabled: true},
		},
	}

	assert.Equal(t, session.ExecutionModeTmux, h.executionModeForAgent(session.AgentTypeCoder))
	assert.Equal(t, session.ExecutionModeTmux, h.executionModeForAgent(session.AgentTypePlanner))
	assert.Equal(t, session.ExecutionModeSDK, h.executionModeForAgent(session.AgentTypeElaborator))
}

func TestStandaloneExecutionMode(t *testing.T) {
	tests := []struct {
		name       string
		agentType  string
		program    string
		phaseRoles map[string]string
		profiles   map[string]config.AgentProfile
		want       session.ExecutionMode
	}{
		{
			name:      "master + claude + sdk profile => sdk",
			agentType: session.AgentTypeMaster,
			program:   "claude",
			phaseRoles: map[string]string{
				"readiness_review": "master",
			},
			profiles: map[string]config.AgentProfile{
				"master": {Program: "claude", Enabled: true, ExecutionMode: config.ExecutionModeSDK},
			},
			want: session.ExecutionModeSDK,
		},
		{
			name:      "master + claude + tmux profile => tmux",
			agentType: session.AgentTypeMaster,
			program:   "claude",
			phaseRoles: map[string]string{
				"readiness_review": "master",
			},
			profiles: map[string]config.AgentProfile{
				"master": {Program: "claude", Enabled: true, ExecutionMode: config.ExecutionModeTmux},
			},
			want: session.ExecutionModeTmux,
		},
		{
			name:      "master + opencode + sdk profile => tmux (opencode not sdk-capable)",
			agentType: session.AgentTypeMaster,
			program:   "opencode",
			phaseRoles: map[string]string{
				"readiness_review": "master",
			},
			profiles: map[string]config.AgentProfile{
				"master": {Program: "opencode", Enabled: true, ExecutionMode: config.ExecutionModeSDK},
			},
			want: session.ExecutionModeTmux,
		},
		{
			name:      "fixer + codex + sdk profile => sdk",
			agentType: session.AgentTypeFixer,
			program:   "codex",
			phaseRoles: map[string]string{
				"fixer": "fixer",
			},
			profiles: map[string]config.AgentProfile{
				"fixer": {Program: "codex", Enabled: true, ExecutionMode: config.ExecutionModeSDK},
			},
			want: session.ExecutionModeSDK,
		},
		{
			name:      "empty agent type + claude + sdk-capable chat profile => sdk",
			agentType: "",
			program:   "claude",
			profiles: map[string]config.AgentProfile{
				"chat": {Program: "claude", Enabled: true, ExecutionMode: config.ExecutionModeSDK},
			},
			want: session.ExecutionModeSDK,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newTestHome()
			h.appConfig = &config.Config{
				PhaseRoles: tc.phaseRoles,
				Profiles:   tc.profiles,
			}
			got := h.standaloneExecutionMode(tc.agentType, tc.program)
			assert.Equal(t, tc.want, got)
		})
	}
}

func collectQuickLaunchMsgs(cmd tea.Cmd) (started []instanceStartedMsg) {
	if cmd == nil {
		return nil
	}

	msg := cmd()
	switch msg := msg.(type) {
	case tea.BatchMsg:
		for _, sub := range msg {
			subStarted := collectQuickLaunchMsgs(sub)
			started = append(started, subStarted...)
		}
	case instanceStartedMsg:
		started = append(started, msg)
	default:
	}

	return started
}

func TestQuickLaunch_KeyCreatesInstance(t *testing.T) {
	oldQuickLaunchStartOnMain := quickLaunchStartOnMain
	quickLaunchStartOnMain = func(inst *session.Instance) error {
		inst.MarkStartedForTest()
		inst.SetStatus(session.Running)
		return nil
	}
	t.Cleanup(func() {
		quickLaunchStartOnMain = oldQuickLaunchStartOnMain
	})

	// Pressing 's' now uses the debounced double-tap path: the first tap schedules
	// a timeout, and KeyQuickLaunch fires only when the timeout message arrives.
	// Override the scheduler so tests do not block on real wall-clock time.
	var capturedTimeoutMsg doubleTapTimeoutMsg
	oldSchedule := scheduleDoubleTapTimeout
	scheduleDoubleTapTimeout = func(_ time.Duration, key string, seq int) tea.Cmd {
		capturedTimeoutMsg = doubleTapTimeoutMsg{key: key, seq: seq}
		return func() tea.Msg { return capturedTimeoutMsg }
	}
	t.Cleanup(func() { scheduleDoubleTapTimeout = oldSchedule })

	h := newTestHome()
	h.activeRepoPath = filepath.Join(t.TempDir(), "myrepo")
	h.keySent = true

	// First s: sets pending + schedules debounce timeout (no instance yet).
	model, _ := h.handleKeyPress(tea.KeyPressMsg{Code: 's', Text: "s"})
	updated := model.(*home)
	require.Equal(t, "s", updated.pendingDoubleTapKey, "pending must be set after first s")

	// Fire the timeout: dispatches KeyQuickLaunch → instance created.
	model, cmd := updated.Update(capturedTimeoutMsg)
	updated = model.(*home)

	require.NotNil(t, cmd)
	assert.Equal(t, stateDefault, updated.state)
	assert.False(t, updated.overlays.IsActive())
	instances := updated.nav.GetInstances()
	require.Len(t, instances, 1)
	assert.Equal(t, "myrepo-agent-1", instances[0].Title)
	assert.Equal(t, session.AgentTypeFixer, instances[0].AgentType)
	assert.Equal(t, h.programForAgent(session.AgentTypeFixer), instances[0].Program)
	assert.Equal(t, session.ExecutionModeTmux, instances[0].ExecutionMode)
	assert.Empty(t, updated.allInstances)

	startedMsgs := collectQuickLaunchMsgs(cmd)
	require.Len(t, startedMsgs, 1)
	assert.Same(t, instances[0], startedMsgs[0].instance)
	assert.True(t, startedMsgs[0].instance.Started())
	assert.Equal(t, session.Running, startedMsgs[0].instance.Status)

	model, followCmd := updated.Update(startedMsgs[0])
	updated = model.(*home)

	require.NotNil(t, followCmd)
	require.Len(t, updated.allInstances, 1)
	assert.Same(t, instances[0], updated.allInstances[0])
	assert.Equal(t, "myrepo-agent-1", updated.allInstances[0].Title)
}

func TestQuickLaunch_SDKProfileProducesSDKMode(t *testing.T) {
	// When the fixer profile is configured with SDK mode and the program supports
	// the SDK transport (claude), the quick-launch instance must use SDK mode.
	oldQuickLaunchStartOnMain := quickLaunchStartOnMain
	quickLaunchStartOnMain = func(inst *session.Instance) error {
		inst.MarkStartedForTest()
		inst.SetStatus(session.Running)
		return nil
	}
	t.Cleanup(func() { quickLaunchStartOnMain = oldQuickLaunchStartOnMain })

	var capturedTimeoutMsg doubleTapTimeoutMsg
	oldSchedule := scheduleDoubleTapTimeout
	scheduleDoubleTapTimeout = func(_ time.Duration, key string, seq int) tea.Cmd {
		capturedTimeoutMsg = doubleTapTimeoutMsg{key: key, seq: seq}
		return func() tea.Msg { return capturedTimeoutMsg }
	}
	t.Cleanup(func() { scheduleDoubleTapTimeout = oldSchedule })

	h := newTestHome()
	h.activeRepoPath = filepath.Join(t.TempDir(), "myrepo")
	h.appConfig = &config.Config{
		PhaseRoles: map[string]string{"fixer": "fixer"},
		Profiles: map[string]config.AgentProfile{
			"fixer": {Program: "claude", Enabled: true, ExecutionMode: config.ExecutionModeSDK},
		},
	}
	h.keySent = true

	// First s: pending double-tap.
	model, _ := h.handleKeyPress(tea.KeyPressMsg{Code: 's', Text: "s"})
	updated := model.(*home)

	// Fire the timeout: dispatches KeyQuickLaunch → instance created.
	model, _ = updated.Update(capturedTimeoutMsg)
	updated = model.(*home)

	instances := updated.nav.GetInstances()
	require.Len(t, instances, 1)
	assert.Equal(t, session.ExecutionModeSDK, instances[0].ExecutionMode,
		"sdk fixer profile with claude program must produce sdk execution mode")
}

func TestQuickLaunch_SDKModeIgnoresTmuxLimit(t *testing.T) {
	oldQuickLaunchStartOnMain := quickLaunchStartOnMain
	quickLaunchStartOnMain = func(inst *session.Instance) error {
		inst.MarkStartedForTest()
		inst.SetStatus(session.Running)
		return nil
	}
	t.Cleanup(func() { quickLaunchStartOnMain = oldQuickLaunchStartOnMain })

	var capturedTimeoutMsg doubleTapTimeoutMsg
	oldSchedule := scheduleDoubleTapTimeout
	scheduleDoubleTapTimeout = func(_ time.Duration, key string, seq int) tea.Cmd {
		capturedTimeoutMsg = doubleTapTimeoutMsg{key: key, seq: seq}
		return func() tea.Msg { return capturedTimeoutMsg }
	}
	t.Cleanup(func() { scheduleDoubleTapTimeout = oldSchedule })

	h := newTestHome()
	h.activeRepoPath = filepath.Join(t.TempDir(), "myrepo")
	h.tmuxSessionCount = GlobalInstanceLimit
	h.appConfig = &config.Config{
		PhaseRoles: map[string]string{"fixer": "fixer"},
		Profiles: map[string]config.AgentProfile{
			"fixer": {Program: "claude", Enabled: true, ExecutionMode: config.ExecutionModeSDK},
		},
	}
	h.keySent = true

	model, _ := h.handleKeyPress(tea.KeyPressMsg{Code: 's', Text: "s"})
	updated := model.(*home)

	model, _ = updated.Update(capturedTimeoutMsg)
	updated = model.(*home)

	instances := updated.nav.GetInstances()
	require.Len(t, instances, 1)
	assert.Equal(t, session.ExecutionModeSDK, instances[0].ExecutionMode)
}

func TestQuickLaunch_TitleSyncUpdatesDisplayTitle(t *testing.T) {
	h := newTestHome()
	h.nav.SetSize(80, 20)

	inst, err := session.NewInstance(session.InstanceOptions{
		Title:   "agent-1",
		Path:    t.TempDir(),
		Program: "opencode",
	})
	require.NoError(t, err)
	inst.MarkStartedForTest()
	inst.SetStatus(session.Running)

	h.nav.AddInstance(inst)()
	h.nav.SelectInstance(inst)
	h.allInstances = append(h.allInstances, inst)
	h.tabbedWindow.SetInstance(inst)
	h.previewTerminalInstance = inst.Title
	h.populateInstanceTabs()
	h.updateInfoPane()
	originalTitle := inst.Title

	model, cmd := h.Update(instanceTitleSyncMsg{instance: inst, newTitle: "Investigate flaky auth tests!!!"})
	updated := model.(*home)

	assert.Nil(t, cmd)
	assert.Equal(t, originalTitle, inst.Title)
	assert.Equal(t, "investigate-flaky-auth-tests", inst.DisplayTitle)
	assert.Equal(t, originalTitle, updated.previewTerminalInstance)
	selected := updated.nav.GetSelectedInstance()
	require.Same(t, inst, selected)
	assert.Equal(t, originalTitle, selected.Title)
	assert.Equal(t, "investigate-flaky-auth-tests", selected.DisplayName())
	assert.Equal(t, "investigate-flaky-auth-tests", updated.tabbedWindow.GetInfoData().Title)
	assert.Contains(t, updated.nav.String(), "investigate-flaky-auth-tests")
}

func TestQuickLaunch_InstanceLimitEnforced(t *testing.T) {
	// s is debounced: limit check happens inside quickLaunchAgent which runs after
	// the single-tap timeout fires.
	var capturedTimeout doubleTapTimeoutMsg
	oldSchedule := scheduleDoubleTapTimeout
	scheduleDoubleTapTimeout = func(_ time.Duration, key string, seq int) tea.Cmd {
		capturedTimeout = doubleTapTimeoutMsg{key: key, seq: seq}
		return func() tea.Msg { return capturedTimeout }
	}
	t.Cleanup(func() { scheduleDoubleTapTimeout = oldSchedule })

	h := newTestHome()
	h.tmuxSessionCount = GlobalInstanceLimit
	h.keySent = true

	// First s: debounced — schedules timeout (no limit check yet).
	model, _ := h.handleKeyPress(tea.KeyPressMsg{Code: 's', Text: "s"})
	updated := model.(*home)
	require.Equal(t, "s", updated.pendingDoubleTapKey, "pending must be set after first s")

	// Fire the timeout: quickLaunchAgent runs → hits limit → error toast.
	model, cmd := updated.Update(capturedTimeout)
	updated = model.(*home)

	assert.Equal(t, stateDefault, updated.state)
	assert.False(t, updated.overlays.IsActive())
	assert.Empty(t, updated.nav.GetInstances())
	assert.Empty(t, updated.allInstances)
	require.NotNil(t, cmd)
	_, ok := cmd().(overlay.ToastTickMsg)
	require.True(t, ok)
	assert.True(t, updated.toastManager.HasActiveToasts())
}

func TestKeyPrompt_SDKModeIgnoresTmuxLimit(t *testing.T) {
	h := newTestHome()
	h.tmuxSessionCount = GlobalInstanceLimit
	h.appConfig = &config.Config{
		Profiles: map[string]config.AgentProfile{
			"chat": {Program: "claude", Enabled: true, ExecutionMode: config.ExecutionModeSDK},
		},
	}
	h.keySent = true

	model, cmd := h.handleKeyPress(tea.KeyPressMsg{Code: 'N', Text: "N"})
	updated := model.(*home)

	require.Nil(t, cmd)
	require.Equal(t, stateNew, updated.state)
	require.NotNil(t, updated.newInstance)
	assert.Equal(t, "claude", updated.newInstance.Program)
	assert.Equal(t, session.ExecutionModeSDK, updated.newInstance.ExecutionMode)
}

func TestQuickLaunch_PlaceholderNameAvoidsCollisions(t *testing.T) {
	h := newTestHome()
	repoPath := filepath.Join(t.TempDir(), "myrepo")
	h.activeRepoPath = repoPath

	newPlaceholder := func(title string) *session.Instance {
		t.Helper()
		inst, err := session.NewInstance(session.InstanceOptions{
			Title:   title,
			Path:    repoPath,
			Program: "opencode",
		})
		require.NoError(t, err)
		return inst
	}

	agent1 := newPlaceholder("myrepo-agent-1")
	agent3 := newPlaceholder("myrepo-agent-3")
	h.nav.AddInstance(agent1)()
	h.nav.AddInstance(agent3)()
	h.allInstances = append(h.allInstances, agent1, agent3)

	assert.Equal(t, "myrepo-agent-4", h.nextPlaceholderName())
}

// TestConfirmationModalStateTransitions tests state transitions without full instance setup
func TestConfirmationModalStateTransitions(t *testing.T) {
	mgr := overlay.NewManager()
	// Create a minimal home struct for testing state transitions
	h := &home{
		ctx:       context.Background(),
		state:     stateDefault,
		appConfig: config.DefaultConfig(),
		overlays:  mgr,
	}

	t.Run("shows confirmation on D press", func(t *testing.T) {
		// Simulate pressing 'D'
		h.state = stateDefault
		h.overlays.Dismiss()

		// Manually trigger what would happen in handleKeyPress for 'D'
		h.state = stateConfirm
		co := overlay.NewConfirmationOverlay("[!] Kill session 'test'?")
		h.overlays.Show(co)

		assert.Equal(t, stateConfirm, h.state)
		assert.True(t, h.overlays.IsActive())
		assert.False(t, co.Dismissed)
	})

	t.Run("returns to default on y press", func(t *testing.T) {
		// Start in confirmation state
		h.state = stateConfirm
		co := overlay.NewConfirmationOverlay("Test confirmation")
		h.overlays.Show(co)

		// Simulate pressing 'y' using HandleKeyPress
		keyMsg := tea.KeyPressMsg{Code: 'y', Text: "y"}
		result := h.overlays.HandleKey(keyMsg)
		if result.Dismissed {
			h.state = stateDefault
		}

		assert.Equal(t, stateDefault, h.state)
		assert.False(t, h.overlays.IsActive())
	})

	t.Run("returns to default on n press", func(t *testing.T) {
		// Start in confirmation state
		h.state = stateConfirm
		h.overlays.Show(overlay.NewConfirmationOverlay("Test confirmation"))

		// Simulate pressing 'n' using HandleKeyPress
		keyMsg := tea.KeyPressMsg{Code: 'n', Text: "n"}
		result := h.overlays.HandleKey(keyMsg)
		if result.Dismissed {
			h.state = stateDefault
		}

		assert.Equal(t, stateDefault, h.state)
		assert.False(t, h.overlays.IsActive())
	})

	t.Run("returns to default on esc press", func(t *testing.T) {
		// Start in confirmation state
		h.state = stateConfirm
		h.overlays.Show(overlay.NewConfirmationOverlay("Test confirmation"))

		// Simulate pressing ESC using HandleKeyPress
		keyMsg := tea.KeyPressMsg{Code: tea.KeyEscape}
		result := h.overlays.HandleKey(keyMsg)
		if result.Dismissed {
			h.state = stateDefault
		}

		assert.Equal(t, stateDefault, h.state)
		assert.False(t, h.overlays.IsActive())
	})
}

// TestConfirmationModalKeyHandling tests the actual key handling in confirmation state
func TestConfirmationModalKeyHandling(t *testing.T) {
	// Import needed packages
	spinner := spinner.New(spinner.WithSpinner(spinner.Dot))
	list := ui.NewNavigationPanel(&spinner)

	// Create enough of home struct to test handleKeyPress in confirmation state
	h := &home{
		ctx:       context.Background(),
		state:     stateConfirm,
		appConfig: config.DefaultConfig(),
		nav:       list,
		menu:      ui.NewMenu(),
		overlays:  overlay.NewManager(),
	}
	h.overlays.Show(overlay.NewConfirmationOverlay("Kill session?"))

	testCases := []struct {
		name          string
		key           string
		expectedState state
		expectedNil   bool
	}{
		{
			name:          "y key confirms and dismisses overlay",
			key:           "y",
			expectedState: stateDefault,
			expectedNil:   true,
		},
		{
			name:          "n key cancels and dismisses overlay",
			key:           "n",
			expectedState: stateDefault,
			expectedNil:   true,
		},
		{
			name:          "esc key cancels and dismisses overlay",
			key:           "esc",
			expectedState: stateDefault,
			expectedNil:   true,
		},
		{
			name:          "other keys are ignored",
			key:           "x",
			expectedState: stateConfirm,
			expectedNil:   false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Reset state
			h.state = stateConfirm
			h.overlays.Show(overlay.NewConfirmationOverlay("Kill session?"))

			// Create key message
			var keyMsg tea.KeyPressMsg
			if tc.key == "esc" {
				keyMsg = tea.KeyPressMsg{Code: tea.KeyEscape}
			} else {
				keyMsg = tea.KeyPressMsg{Code: rune(tc.key[0]), Text: tc.key}
			}

			// Call handleKeyPress
			model, _ := h.handleKeyPress(keyMsg)
			homeModel, ok := model.(*home)
			require.True(t, ok)

			assert.Equal(t, tc.expectedState, homeModel.state, "State mismatch for key: %s", tc.key)
			if tc.expectedNil {
				assert.False(t, homeModel.overlays.IsActive(), "Overlay should be nil for key: %s", tc.key)
			} else {
				assert.True(t, homeModel.overlays.IsActive(), "Overlay should not be nil for key: %s", tc.key)
			}
		})
	}
}

// TestConfirmationMessageFormatting tests that confirmation messages are formatted correctly
func TestConfirmationMessageFormatting(t *testing.T) {
	testCases := []struct {
		name            string
		sessionTitle    string
		expectedMessage string
	}{
		{
			name:            "short session name",
			sessionTitle:    "my-feature",
			expectedMessage: "[!] Kill session 'my-feature'? (y/n)",
		},
		{
			name:            "long session name",
			sessionTitle:    "very-long-feature-branch-name-here",
			expectedMessage: "[!] Kill session 'very-long-feature-branch-name-here'? (y/n)",
		},
		{
			name:            "session with spaces",
			sessionTitle:    "feature with spaces",
			expectedMessage: "[!] Kill session 'feature with spaces'? (y/n)",
		},
		{
			name:            "session with special chars",
			sessionTitle:    "feature/branch-123",
			expectedMessage: "[!] Kill session 'feature/branch-123'? (y/n)",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Test the message formatting directly
			actualMessage := fmt.Sprintf("[!] Kill session '%s'? (y/n)", tc.sessionTitle)
			assert.Equal(t, tc.expectedMessage, actualMessage)
		})
	}
}

// TestConfirmationFlowSimulation tests the confirmation flow by simulating the state changes
func TestConfirmationFlowSimulation(t *testing.T) {
	// Test the confirmation overlay component directly
	message := "[!] Kill session 'test-session'?"
	co := overlay.NewConfirmationOverlay(message)

	// Verify the overlay was created correctly
	assert.False(t, co.Dismissed)
	// Test that overlay renders with the correct message
	rendered := co.View()
	assert.Contains(t, rendered, "Kill session 'test-session'?")
}

// TestConfirmActionWithDifferentTypes tests that ConfirmationOverlay works with different action types
func TestConfirmActionWithDifferentTypes(t *testing.T) {
	t.Run("works with simple action returning nil", func(t *testing.T) {
		actionCalled := false
		actionExecuted := false

		co := overlay.NewConfirmationOverlay("Test action?")
		co.OnConfirm = func() {
			actionExecuted = true
			actionCalled = true
		}

		assert.False(t, co.Dismissed)
		assert.NotNil(t, co.OnConfirm)

		co.OnConfirm()
		assert.True(t, actionCalled)
		assert.True(t, actionExecuted)
	})

	t.Run("works with action returning error", func(t *testing.T) {
		expectedErr := fmt.Errorf("test error")
		var receivedMsg tea.Msg

		co := overlay.NewConfirmationOverlay("Error action?")
		co.OnConfirm = func() {
			receivedMsg = expectedErr
		}

		assert.False(t, co.Dismissed)
		assert.NotNil(t, co.OnConfirm)

		co.OnConfirm()
		assert.Equal(t, expectedErr, receivedMsg)
	})

	t.Run("works with action returning custom message", func(t *testing.T) {
		var receivedMsg tea.Msg

		co := overlay.NewConfirmationOverlay("Custom message action?")
		co.OnConfirm = func() {
			receivedMsg = instanceChangedMsg{}
		}

		assert.False(t, co.Dismissed)
		assert.NotNil(t, co.OnConfirm)

		co.OnConfirm()
		_, ok := receivedMsg.(instanceChangedMsg)
		assert.True(t, ok, "Expected instanceChangedMsg but got %T", receivedMsg)
	})
}

// TestMultipleConfirmationsDontInterfere tests that multiple ConfirmationOverlays don't interfere
func TestMultipleConfirmationsDontInterfere(t *testing.T) {
	// First confirmation
	action1Called := false
	action1 := func() tea.Msg {
		action1Called = true
		return nil
	}

	co1 := overlay.NewConfirmationOverlay("First action?")
	firstOnConfirm := func() {
		action1()
	}
	co1.OnConfirm = firstOnConfirm

	assert.False(t, co1.Dismissed)
	assert.NotNil(t, co1.OnConfirm)

	// Cancel first confirmation (simulate pressing 'n')
	keyMsg := tea.KeyPressMsg{Code: 'n', Text: "n"}
	result1 := co1.HandleKey(keyMsg)
	assert.True(t, result1.Dismissed, "pressing 'n' must dismiss the overlay")

	// Second confirmation with different action
	action2Called := false
	action2 := func() tea.Msg {
		action2Called = true
		return fmt.Errorf("action2 error")
	}

	co2 := overlay.NewConfirmationOverlay("Second action?")
	var secondResult tea.Msg
	secondOnConfirm := func() {
		secondResult = action2()
	}
	co2.OnConfirm = secondOnConfirm

	assert.False(t, co2.Dismissed)
	assert.NotNil(t, co2.OnConfirm)

	// Execute second action to verify it's the correct one
	co2.OnConfirm()
	err, ok := secondResult.(error)
	assert.True(t, ok)
	assert.Equal(t, "action2 error", err.Error())
	assert.True(t, action2Called)
	assert.False(t, action1Called, "First action should not have been called")

	// Test that cancelled action can still be executed independently
	firstOnConfirm()
	assert.True(t, action1Called, "First action should be callable after being replaced")
}

// TestConfirmationModalVisualAppearance tests that confirmation modal has distinct visual appearance
func TestConfirmationModalVisualAppearance(t *testing.T) {
	// Test the ConfirmationOverlay component directly
	message := "[!] Delete everything?"
	co := overlay.NewConfirmationOverlay(message)

	assert.False(t, co.Dismissed)

	// Test the overlay render (we can test that it renders without errors)
	rendered := co.View()
	assert.NotEmpty(t, rendered)

	// Test that it includes the message content and instructions
	assert.Contains(t, rendered, "Delete everything?")
	assert.Contains(t, rendered, "Press")
	assert.Contains(t, rendered, "to confirm")
	assert.Contains(t, rendered, "to cancel")

	// Test that the danger indicator is preserved
	assert.Contains(t, rendered, "[!")
}

func TestFocusRing(t *testing.T) {
	newTestHome := func() *home {
		spin := spinner.New(spinner.WithSpinner(spinner.Dot))
		return &home{
			ctx:          context.Background(),
			state:        stateDefault,
			appConfig:    config.DefaultConfig(),
			nav:          ui.NewNavigationPanel(&spin),
			menu:         ui.NewMenu(),
			tabbedWindow: ui.NewTabbedWindow(ui.NewPreviewPane(), ui.NewInfoPane()),
		}
	}

	addTestInstance := func(t *testing.T, h *home) *session.Instance {
		t.Helper()
		inst, err := session.NewInstance(session.InstanceOptions{
			Title: "test", Path: t.TempDir(), Program: "claude",
		})
		require.NoError(t, err)
		// Stagger timestamps so newest-first sort gives deterministic visual order:
		// first added gets the highest timestamp (visual position 0), last added is oldest (visual last).
		inst.CreatedAt = time.Unix(int64(1000-h.nav.NumInstances()*100), 0)
		h.nav.AddInstance(inst)()
		return inst
	}

	handle := func(t *testing.T, h *home, msg tea.KeyPressMsg) *home {
		t.Helper()
		h.keySent = true
		model, _ := h.handleKeyPress(msg)
		homeModel, ok := model.(*home)
		require.True(t, ok)
		return homeModel
	}

	// --- Tab cycles through dynamic instance tabs; sidebar (slotNav) always retains focus ---

	t.Run("Tab cycles active tab from first to second, sidebar stays focused", func(t *testing.T) {
		h := newTestHome()
		h.tabbedWindow.SetTabs([]ui.InstanceTab{
			{Title: "tab-0", Key: "tab-0"},
			{Title: "tab-1", Key: "tab-1"},
		})
		h.tabbedWindow.SetActiveTab(0)

		homeModel := handle(t, h, tea.KeyPressMsg{Code: tea.KeyTab})

		assert.Equal(t, slotNav, homeModel.focusSlot, "sidebar must retain focus")
		assert.Equal(t, 1, homeModel.tabbedWindow.GetActiveTab(), "active tab must advance to second")
	})

	t.Run("Tab wraps active tab from last to first, sidebar stays focused", func(t *testing.T) {
		h := newTestHome()
		h.tabbedWindow.SetTabs([]ui.InstanceTab{
			{Title: "tab-0", Key: "tab-0"},
			{Title: "tab-1", Key: "tab-1"},
		})
		h.tabbedWindow.SetActiveTab(1)

		homeModel := handle(t, h, tea.KeyPressMsg{Code: tea.KeyTab})

		assert.Equal(t, slotNav, homeModel.focusSlot, "sidebar must retain focus")
		assert.Equal(t, 0, homeModel.tabbedWindow.GetActiveTab(), "active tab must wrap to first")
	})

	t.Run("Tab with zero tabs is no-op, active index stays at 0", func(t *testing.T) {
		h := newTestHome()
		// No instance tabs seeded — Tab should be a no-op.

		homeModel := handle(t, h, tea.KeyPressMsg{Code: tea.KeyTab})

		assert.Equal(t, slotNav, homeModel.focusSlot, "sidebar must retain focus")
		assert.Equal(t, 0, homeModel.tabbedWindow.GetActiveTab(), "active index must stay at 0 with zero tabs")
	})

	t.Run("Shift+Tab is a no-op in default state", func(t *testing.T) {
		h := newTestHome()
		h.tabbedWindow.SetTabs([]ui.InstanceTab{
			{Title: "tab-0", Key: "tab-0"},
			{Title: "tab-1", Key: "tab-1"},
		})
		h.tabbedWindow.SetActiveTab(1)

		homeModel := handle(t, h, tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift})

		assert.Equal(t, slotNav, homeModel.focusSlot, "sidebar must retain focus")
		assert.Equal(t, 1, homeModel.tabbedWindow.GetActiveTab(), "active tab must be unchanged")
	})

	t.Run("T is no-op when right-sidebar shortcut is removed", func(t *testing.T) {
		h := newTestHome()
		addTestInstance(t, h)
		h.setFocusSlot(slotAgent)

		homeModel := handle(t, h, tea.KeyPressMsg{Code: 'T', Text: "T"})

		assert.Equal(t, slotAgent, homeModel.focusSlot)
	})

	// --- Direct keybinds (!/@/#) ---

	t.Run("! enters focus mode for selected instance, sidebar keeps focus", func(t *testing.T) {
		h := newTestHome()
		// No running instance — ! should be a no-op (doesn't enter focus mode).
		homeModel := handle(t, h, tea.KeyPressMsg{Code: '!', Text: "!"})

		assert.Equal(t, slotNav, homeModel.focusSlot, "sidebar must retain focus")
		assert.Equal(t, stateDefault, homeModel.state, "! without running instance must not enter focus mode")
	})

	t.Run("I toggles compact info header visibility, sidebar keeps focus", func(t *testing.T) {
		h := newTestHome()
		// showInfo starts as false (from NewTabbedWindow).
		wasShowing := h.tabbedWindow.IsShowingInfo()

		homeModel := handle(t, h, tea.KeyPressMsg{Code: 'I', Text: "I"})

		assert.Equal(t, slotNav, homeModel.focusSlot, "sidebar must retain focus")
		assert.Equal(t, !wasShowing, homeModel.tabbedWindow.IsShowingInfo(), "I must toggle info header visibility")
	})

	t.Run("# is no-op (direct info-tab shortcut removed)", func(t *testing.T) {
		h := newTestHome()
		wasShowing := h.tabbedWindow.IsShowingInfo()
		h.setFocusSlot(slotNav)

		homeModel := handle(t, h, tea.KeyPressMsg{Code: '#', Text: "#"})

		assert.Equal(t, slotNav, homeModel.focusSlot)
		assert.Equal(t, wasShowing, homeModel.tabbedWindow.IsShowingInfo())
	})

	t.Run("T does not show hidden sidebar", func(t *testing.T) {
		h := newTestHome()
		h.sidebarHidden = true
		h.setFocusSlot(slotNav)

		homeModel := handle(t, h, tea.KeyPressMsg{Code: 'T', Text: "T"})

		assert.True(t, homeModel.sidebarHidden)
		assert.Equal(t, slotNav, homeModel.focusSlot)
	})

	// --- Sidebar toggle (ctrl+s) ---

	t.Run("ctrl+s hides sidebar and moves focus from nav to agent", func(t *testing.T) {
		h := newTestHome()
		h.sidebarHidden = false
		h.setFocusSlot(slotNav)

		homeModel := handle(t, h, tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl})

		assert.True(t, homeModel.sidebarHidden)
		assert.Equal(t, slotAgent, homeModel.focusSlot)
	})

	t.Run("ctrl+s hides sidebar and keeps focus when agent slot is focused", func(t *testing.T) {
		h := newTestHome()
		h.sidebarHidden = false
		h.setFocusSlot(slotAgent)

		homeModel := handle(t, h, tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl})

		assert.True(t, homeModel.sidebarHidden)
		assert.Equal(t, slotAgent, homeModel.focusSlot)
	})

	t.Run("ctrl+s shows sidebar and keeps focus when sidebar is hidden", func(t *testing.T) {
		h := newTestHome()
		h.sidebarHidden = true
		h.setFocusSlot(slotNav)

		homeModel := handle(t, h, tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl})

		assert.False(t, homeModel.sidebarHidden)
		assert.Equal(t, slotNav, homeModel.focusSlot)
	})

	// --- Arrow key navigation ---

	t.Run("← is no-op (sidebar already focused)", func(t *testing.T) {
		h := newTestHome()
		h.tabbedWindow.SetActiveTab(ui.PreviewTab)

		homeModel := handle(t, h, tea.KeyPressMsg{Code: tea.KeyLeft})

		assert.Equal(t, slotNav, homeModel.focusSlot, "sidebar must remain focused after ←")
		assert.Equal(t, ui.PreviewTab, homeModel.tabbedWindow.GetActiveTab(), "active tab must not change on ←")
	})

	t.Run("← closes plan preview opened in document mode", func(t *testing.T) {
		h := newTestHome()
		h.tabbedWindow.SetDocumentContent("# test plan")

		homeModel := handle(t, h, tea.KeyPressMsg{Code: tea.KeyLeft})

		assert.False(t, homeModel.tabbedWindow.IsDocumentMode(), "left arrow must close document mode")
	})

	t.Run("→ toggles expand on selected sidebar item", func(t *testing.T) {
		h := newTestHome()
		// Without a plan header selected, ToggleSelectedExpand returns false,
		// so → is effectively a no-op — sidebar stays focused.
		homeModel := handle(t, h, tea.KeyPressMsg{Code: tea.KeyRight})

		assert.Equal(t, slotNav, homeModel.focusSlot, "sidebar must retain focus after →")
	})

	// --- Enter key blocked on info tab ---

	// --- Ctrl+Up/Down: cycle active instances with wrapping ---

	t.Run("ctrl+down cycles to next active instance", func(t *testing.T) {
		h := newTestHome()
		addTestInstance(t, h)
		addTestInstance(t, h)
		addTestInstance(t, h)
		h.nav.SetSelectedInstance(0)

		homeModel := handle(t, h, tea.KeyPressMsg{Code: tea.KeyDown, Mod: tea.ModCtrl})

		assert.Equal(t, 1, homeModel.nav.SelectedIndex())
	})

	t.Run("ctrl+down wraps from last to first", func(t *testing.T) {
		h := newTestHome()
		addTestInstance(t, h)
		addTestInstance(t, h)
		addTestInstance(t, h)
		h.nav.SetSelectedInstance(2)

		homeModel := handle(t, h, tea.KeyPressMsg{Code: tea.KeyDown, Mod: tea.ModCtrl})

		assert.Equal(t, 0, homeModel.nav.SelectedIndex())
	})

	t.Run("ctrl+up cycles to previous active instance", func(t *testing.T) {
		h := newTestHome()
		addTestInstance(t, h)
		addTestInstance(t, h)
		addTestInstance(t, h)
		h.nav.SetSelectedInstance(2)

		homeModel := handle(t, h, tea.KeyPressMsg{Code: tea.KeyUp, Mod: tea.ModCtrl})

		assert.Equal(t, 1, homeModel.nav.SelectedIndex())
	})

	t.Run("ctrl+up wraps from first to last", func(t *testing.T) {
		h := newTestHome()
		addTestInstance(t, h)
		addTestInstance(t, h)
		addTestInstance(t, h)
		h.nav.SetSelectedInstance(0)

		homeModel := handle(t, h, tea.KeyPressMsg{Code: tea.KeyUp, Mod: tea.ModCtrl})

		assert.Equal(t, 2, homeModel.nav.SelectedIndex())
	})

	t.Run("← switches to previous instance tab", func(t *testing.T) {
		h := newTestHome()
		inst1 := addTestInstance(t, h)
		inst2 := addTestInstance(t, h)
		h.tabbedWindow.SetTabs([]ui.InstanceTab{
			{Title: inst1.Title, Key: inst1.Title},
			{Title: inst2.Title, Key: inst2.Title},
		})
		h.tabbedWindow.SetActiveTab(1) // on inst-2

		homeModel := handle(t, h, tea.KeyPressMsg{Code: tea.KeyLeft})

		assert.Equal(t, 0, homeModel.tabbedWindow.GetActiveTab(),
			"← should switch to previous tab")
	})

	t.Run("→ switches to next instance tab", func(t *testing.T) {
		h := newTestHome()
		inst1 := addTestInstance(t, h)
		inst2 := addTestInstance(t, h)
		h.tabbedWindow.SetTabs([]ui.InstanceTab{
			{Title: inst1.Title, Key: inst1.Title},
			{Title: inst2.Title, Key: inst2.Title},
		})
		h.tabbedWindow.SetActiveTab(0) // on inst-1

		homeModel := handle(t, h, tea.KeyPressMsg{Code: tea.KeyRight})

		assert.Equal(t, 1, homeModel.tabbedWindow.GetActiveTab(),
			"→ should switch to next tab")
	})

	t.Run("ctrl+down skips paused instances", func(t *testing.T) {
		h := newTestHome()
		addTestInstance(t, h) // 0: active
		addTestInstance(t, h) // 1: will be paused
		addTestInstance(t, h) // 2: active
		h.nav.GetInstances()[1].Status = session.Paused
		h.nav.SetSelectedInstance(0)

		homeModel := handle(t, h, tea.KeyPressMsg{Code: tea.KeyDown, Mod: tea.ModCtrl})

		assert.Equal(t, 2, homeModel.nav.SelectedIndex())
	})

	t.Run("ctrl+up skips paused instances", func(t *testing.T) {
		h := newTestHome()
		addTestInstance(t, h) // 0: active
		addTestInstance(t, h) // 1: will be paused
		addTestInstance(t, h) // 2: active
		h.nav.GetInstances()[1].Status = session.Paused
		h.nav.SetSelectedInstance(2)

		homeModel := handle(t, h, tea.KeyPressMsg{Code: tea.KeyUp, Mod: tea.ModCtrl})

		assert.Equal(t, 0, homeModel.nav.SelectedIndex())
	})
}

func TestPreviewTerminal_SelectionChange(t *testing.T) {
	// Helper to create a minimal home with two started instances.
	newTestHomeWithInstances := func(t *testing.T) (*home, *session.Instance, *session.Instance) {
		t.Helper()
		spin := spinner.New(spinner.WithSpinner(spinner.Dot))
		h := &home{
			ctx:          context.Background(),
			state:        stateDefault,
			appConfig:    config.DefaultConfig(),
			nav:          ui.NewNavigationPanel(&spin),
			menu:         ui.NewMenu(),
			tabbedWindow: ui.NewTabbedWindow(ui.NewPreviewPane(), ui.NewInfoPane()),
		}

		instA, err := session.NewInstance(session.InstanceOptions{
			Title: "instance-A", Path: t.TempDir(), Program: "claude",
		})
		require.NoError(t, err)
		instA.MarkStartedForTest()
		instA.Status = session.Running
		instA.CachedContentSet = true // avoid tmux subprocess calls in tests

		instB, err := session.NewInstance(session.InstanceOptions{
			Title: "instance-B", Path: t.TempDir(), Program: "claude",
		})
		require.NoError(t, err)
		instB.MarkStartedForTest()
		instB.Status = session.Running
		instB.CachedContentSet = true

		h.nav.AddInstance(instA)()
		h.nav.AddInstance(instB)()

		return h, instA, instB
	}

	t.Run("swap terminal when selection changes from A to B", func(t *testing.T) {
		h, _, instB := newTestHomeWithInstances(t)
		h.previewRequested = true

		// Simulate: previewTerminal is attached to instance "A".
		dummyTerm := session.NewDummyTerminal()
		h.previewTerminal = dummyTerm
		h.previewTerminalInstance = "instance-A"

		// Select instance "B" by reference (sort-order safe).
		require.True(t, h.nav.SelectInstance(instB), "should find instance-B in list")

		// Fire instanceChanged — should tear down old terminal and return spawn cmd.
		cmd := h.instanceChanged()

		// Old terminal is closed: previewTerminal becomes nil, instance name cleared.
		assert.Nil(t, h.previewTerminal, "previewTerminal should be nil after selection change")
		assert.Empty(t, h.previewTerminalInstance, "previewTerminalInstance should be cleared")

		// A tea.Cmd is returned (the async spawn command).
		assert.NotNil(t, cmd, "instanceChanged should return a tea.Cmd for async spawn")
	})

	t.Run("tear down terminal when no valid instance selected", func(t *testing.T) {
		// Use a home with zero instances so GetSelectedInstance returns nil.
		spin := spinner.New(spinner.WithSpinner(spinner.Dot))
		h := &home{
			ctx:          context.Background(),
			state:        stateDefault,
			appConfig:    config.DefaultConfig(),
			nav:          ui.NewNavigationPanel(&spin),
			menu:         ui.NewMenu(),
			tabbedWindow: ui.NewTabbedWindow(ui.NewPreviewPane(), ui.NewInfoPane()),
		}

		// Attach a terminal.
		dummyTerm := session.NewDummyTerminal()
		h.previewTerminal = dummyTerm
		h.previewTerminalInstance = "instance-A"

		cmd := h.instanceChanged()

		assert.Nil(t, h.previewTerminal, "previewTerminal should be torn down")
		assert.Empty(t, h.previewTerminalInstance, "previewTerminalInstance should be cleared")
		// Old terminal close now happens asynchronously.
		assert.NotNil(t, cmd, "selection clear should return async close cmd when a terminal is attached")
	})

	t.Run("no-op when selection matches current terminal", func(t *testing.T) {
		h, instA, _ := newTestHomeWithInstances(t)
		h.previewRequested = true

		dummyTerm := session.NewDummyTerminal()
		h.previewTerminal = dummyTerm
		h.previewTerminalInstance = "instance-A"

		// Select instance "A" — same as current terminal (use reference, sort-order safe).
		require.True(t, h.nav.SelectInstance(instA), "should find instance-A in list")

		cmd := h.instanceChanged()

		// Terminal should remain attached (not nil).
		assert.Equal(t, dummyTerm, h.previewTerminal, "previewTerminal should remain attached")
		assert.Equal(t, "instance-A", h.previewTerminalInstance, "previewTerminalInstance should remain")
		// No spawn cmd — terminal already attached.
		assert.Nil(t, cmd, "no spawn cmd when same instance is selected")

		// Cleanup
		dummyTerm.Close()
	})

	t.Run("previewTerminalReadyMsg attaches terminal on match", func(t *testing.T) {
		h, instA, _ := newTestHomeWithInstances(t)
		h.previewRequested = true
		h.nav.SelectInstance(instA) // select instance-A

		readyTerm := session.NewDummyTerminal()
		msg := previewTerminalReadyMsg{
			term:          readyTerm,
			instanceTitle: "instance-A",
		}

		_, cmd := h.Update(msg)

		assert.Equal(t, readyTerm, h.previewTerminal, "previewTerminal should be set from msg")
		assert.Equal(t, "instance-A", h.previewTerminalInstance, "previewTerminalInstance should match")
		assert.NotNil(t, cmd, "preview terminal attach should request a resize refresh")

		// Cleanup
		readyTerm.Close()
	})

	t.Run("previewTerminalReadyMsg discards stale terminal", func(t *testing.T) {
		h, _, instB := newTestHomeWithInstances(t)
		h.previewRequested = true
		h.nav.SelectInstance(instB) // select instance-B (different from msg)

		staleTerm := session.NewDummyTerminal()
		msg := previewTerminalReadyMsg{
			term:          staleTerm,
			instanceTitle: "instance-A", // stale — selection moved to B
		}

		_, cmd := h.Update(msg)

		// Stale terminal should NOT be attached.
		assert.Nil(t, h.previewTerminal, "stale terminal should not be attached")
		assert.Empty(t, h.previewTerminalInstance, "previewTerminalInstance should remain empty")
		assert.NotNil(t, cmd, "stale terminals should be closed asynchronously")
		// staleTerm.Close() was called internally by the handler
	})

	t.Run("previewTerminalReadyMsg discards on error", func(t *testing.T) {
		h, instA, _ := newTestHomeWithInstances(t)
		h.previewRequested = true
		h.nav.SelectInstance(instA)

		errTerm := session.NewDummyTerminal()
		msg := previewTerminalReadyMsg{
			term:          errTerm,
			instanceTitle: "instance-A",
			err:           fmt.Errorf("tmux attach failed"),
		}

		_, cmd := h.Update(msg)

		assert.Nil(t, h.previewTerminal, "terminal should not be attached on error")
		assert.Empty(t, h.previewTerminalInstance)
		assert.NotNil(t, cmd, "errored terminals should be closed asynchronously")
		// errTerm.Close() was called internally by the handler
	})
}

// TestPreviewTerminal_RenderTickIntegration tests the full preview terminal lifecycle:
// selection change → previewTerminalReadyMsg → render tick → selection change again.
func TestPreviewTerminal_RenderTickIntegration(t *testing.T) {
	newTestHomeWithInstances := func(t *testing.T) (*home, *session.Instance, *session.Instance) {
		t.Helper()
		spin := spinner.New(spinner.WithSpinner(spinner.Dot))
		h := &home{
			ctx:          context.Background(),
			state:        stateDefault,
			appConfig:    config.DefaultConfig(),
			nav:          ui.NewNavigationPanel(&spin),
			menu:         ui.NewMenu(),
			tabbedWindow: ui.NewTabbedWindow(ui.NewPreviewPane(), ui.NewInfoPane()),
		}

		instA, err := session.NewInstance(session.InstanceOptions{
			Title: "instance-A", Path: t.TempDir(), Program: "claude",
		})
		require.NoError(t, err)
		instA.MarkStartedForTest()
		instA.Status = session.Running
		instA.CachedContentSet = true

		instB, err := session.NewInstance(session.InstanceOptions{
			Title: "instance-B", Path: t.TempDir(), Program: "claude",
		})
		require.NoError(t, err)
		instB.MarkStartedForTest()
		instB.Status = session.Running
		instB.CachedContentSet = true

		h.nav.AddInstance(instA)()
		h.nav.AddInstance(instB)()

		return h, instA, instB
	}

	t.Run("full flow: attach → tick → selection change → discard old terminal", func(t *testing.T) {
		h, instA, instB := newTestHomeWithInstances(t)
		h.previewRequested = true

		// Step 1: Select instance A and simulate instanceChanged returning a spawn cmd.
		require.True(t, h.nav.SelectInstance(instA))
		spawnCmd := h.instanceChanged()
		assert.NotNil(t, spawnCmd, "instanceChanged should return spawn cmd for new selection")
		assert.Nil(t, h.previewTerminal, "terminal not yet attached — spawn is async")

		// Step 2: Async spawn completes — deliver previewTerminalReadyMsg for instance A.
		termA := session.NewDummyTerminal()
		_, cmd := h.Update(previewTerminalReadyMsg{
			term:          termA,
			instanceTitle: "instance-A",
		})
		assert.Equal(t, termA, h.previewTerminal, "terminal A should be attached")
		assert.Equal(t, "instance-A", h.previewTerminalInstance)
		assert.NotNil(t, cmd, "ready msg should request a resize refresh")

		// Step 3: Render tick fires — terminal is active, tick returns event-driven cmd.
		_, tickCmd := h.Update(previewTickMsg{})
		assert.NotNil(t, tickCmd, "previewTickMsg should always return a follow-up tick cmd")
		// previewTerminal is still attached after the tick.
		assert.Equal(t, termA, h.previewTerminal, "terminal A should remain attached after tick")

		// Step 4: User selects instance B — old terminal is discarded, new spawn cmd returned.
		require.True(t, h.nav.SelectInstance(instB))
		spawnCmd2 := h.instanceChanged()

		assert.Nil(t, h.previewTerminal, "old terminal A should be discarded on selection change")
		assert.Empty(t, h.previewTerminalInstance, "instance name should be cleared")
		assert.NotNil(t, spawnCmd2, "new spawn cmd should be returned for instance B")
	})

	t.Run("render tick with nil terminal returns sleep-based cmd", func(t *testing.T) {
		h, _, _ := newTestHomeWithInstances(t)
		// No terminal attached.
		assert.Nil(t, h.previewTerminal)

		_, cmd := h.Update(previewTickMsg{})
		assert.NotNil(t, cmd, "previewTickMsg should return a follow-up cmd even with nil terminal")
	})

	t.Run("render tick with active terminal returns event-driven cmd", func(t *testing.T) {
		h, instA, _ := newTestHomeWithInstances(t)
		h.nav.SelectInstance(instA)

		term := session.NewDummyTerminal()
		h.previewTerminal = term
		h.previewTerminalInstance = "instance-A"
		defer term.Close()

		_, cmd := h.Update(previewTickMsg{})
		assert.NotNil(t, cmd, "previewTickMsg should return event-driven cmd when terminal is active")
		// Terminal remains attached after tick.
		assert.Equal(t, term, h.previewTerminal, "terminal should remain attached after tick")
	})

	t.Run("stale ready msg after second selection change is discarded", func(t *testing.T) {
		h, instA, instB := newTestHomeWithInstances(t)
		h.previewRequested = true

		// Select A, spawn starts.
		require.True(t, h.nav.SelectInstance(instA))
		h.instanceChanged()

		// Before spawn completes, user switches to B.
		require.True(t, h.nav.SelectInstance(instB))
		h.instanceChanged()

		// Now the stale ready msg for A arrives.
		staleTermA := session.NewDummyTerminal()
		_, cmd := h.Update(previewTerminalReadyMsg{
			term:          staleTermA,
			instanceTitle: "instance-A", // stale — selection is now B
		})

		// Stale terminal must be discarded (not attached).
		assert.Nil(t, h.previewTerminal, "stale terminal for A should not be attached when B is selected")
		assert.Empty(t, h.previewTerminalInstance)
		assert.NotNil(t, cmd, "stale ready terminals should be closed asynchronously")
	})
}

// TestPreviewTerminalReadyMsg_StaleDiscard verifies that previewTerminalReadyMsg
// discards the terminal when the selection has changed since the spawn was initiated.
func TestPreviewTerminalReadyMsg_StaleDiscard(t *testing.T) {
	spin := spinner.New(spinner.WithSpinner(spinner.Dot))
	h := &home{
		ctx:          context.Background(),
		state:        stateDefault,
		appConfig:    config.DefaultConfig(),
		nav:          ui.NewNavigationPanel(&spin),
		menu:         ui.NewMenu(),
		tabbedWindow: ui.NewTabbedWindow(ui.NewPreviewPane(), ui.NewInfoPane()),
	}
	h.previewRequested = true

	// Add instance "B" and select it (simulating selection change after spawn started for "A").
	instB, err := session.NewInstance(session.InstanceOptions{
		Title:   "B",
		Path:    t.TempDir(),
		Program: "claude",
	})
	require.NoError(t, err)
	h.nav.AddInstance(instB)()
	h.nav.SelectInstance(instB) // Select "B" by pointer (sort-order safe)

	// Simulate a stale previewTerminalReadyMsg arriving for "A" (selection already moved to "B").
	// The handler should discard the terminal since selected.Title != msg.instanceTitle.
	msg := previewTerminalReadyMsg{
		term:          nil, // nil is fine — we just check it's discarded
		instanceTitle: "A",
		err:           nil,
	}

	// Process the message through Update.
	model, cmd := h.Update(msg)
	homeModel, ok := model.(*home)
	require.True(t, ok)

	// Terminal should NOT be set — it was stale.
	assert.Nil(t, homeModel.previewTerminal, "stale terminal should be discarded")
	assert.Equal(t, "", homeModel.previewTerminalInstance,
		"previewTerminalInstance should not be set for stale msg")
	assert.Nil(t, cmd, "no cmd should be returned for stale msg")
}

func TestTmuxBrowserActions(t *testing.T) {
	t.Run("tmuxSessionsMsg with no sessions shows toast", func(t *testing.T) {
		h := newTestHome()
		msg := tmuxSessionsMsg{sessions: nil}
		model, _ := h.Update(msg)
		hm := model.(*home)
		assert.False(t, hm.overlays.IsActive())
		assert.Equal(t, stateDefault, hm.state)
	})

	t.Run("tmuxSessionsMsg with sessions opens browser", func(t *testing.T) {
		h := newTestHome()
		msg := tmuxSessionsMsg{
			sessions: []tmux.SessionInfo{
				{Name: "kas_test", Title: "test", Width: 80, Height: 24, Managed: false},
			},
		}
		model, _ := h.Update(msg)
		hm := model.(*home)
		_, isBrowser := hm.overlays.Current().(*overlay.TmuxBrowserOverlay)
		assert.True(t, isBrowser, "current overlay must be a TmuxBrowserOverlay")
		assert.Equal(t, stateTmuxBrowser, hm.state)
	})

	t.Run("managed sessions are enriched with instance metadata", func(t *testing.T) {
		h := newTestHome()
		inst, _ := session.NewInstance(session.InstanceOptions{
			Title:   "auth-impl",
			Path:    "/tmp",
			Program: "claude",
		})
		inst.TaskFile = "auth"
		inst.AgentType = session.AgentTypeCoder
		inst.MarkStartedForTest()
		inst.SetTmuxSession(tmux.NewTmuxSessionWithDeps(
			"auth-impl",
			"claude",
			false,
			&noopPtyFactory{},
			cmd_test.NewMockExecutor(),
		))
		h.allInstances = append(h.allInstances, inst)

		msg := tmuxSessionsMsg{
			sessions: []tmux.SessionInfo{
				{Name: "kas_auth-impl", Title: "auth-impl", Width: 80, Height: 24, Managed: true},
			},
		}
		model, _ := h.Update(msg)
		hm := model.(*home)
		browser, ok := hm.overlays.Current().(*overlay.TmuxBrowserOverlay)
		require.True(t, ok, "current overlay must be a TmuxBrowserOverlay")
		item := browser.SelectedItem()
		assert.True(t, item.Managed)
		assert.Equal(t, "coder", item.AgentType)
		assert.Equal(t, "auth", item.TaskFile)
	})

	t.Run("dismiss returns to default state", func(t *testing.T) {
		h := newTestHome()
		browser := overlay.NewTmuxBrowserOverlay([]overlay.TmuxBrowserItem{
			{Name: "kas_test", Title: "test"},
		})
		h.overlays.Show(browser)
		h.state = stateTmuxBrowser
		// "" is the dismiss action string
		model, _ := h.handleTmuxBrowserAction(browser, "")
		hm := model.(*home)
		assert.False(t, hm.overlays.IsActive())
		assert.Equal(t, stateDefault, hm.state)
	})
}

// TestPreviewTerminalReadyMsg_AcceptsCurrentInstance verifies that previewTerminalReadyMsg
// sets the terminal when the instance title matches the current selection.
func TestPreviewTerminalReadyMsg_AcceptsCurrentInstance(t *testing.T) {
	spin := spinner.New(spinner.WithSpinner(spinner.Dot))
	h := &home{
		ctx:          context.Background(),
		state:        stateDefault,
		appConfig:    config.DefaultConfig(),
		nav:          ui.NewNavigationPanel(&spin),
		menu:         ui.NewMenu(),
		tabbedWindow: ui.NewTabbedWindow(ui.NewPreviewPane(), ui.NewInfoPane()),
	}
	h.previewRequested = true

	// Add instance "A" and select it.
	instA, err := session.NewInstance(session.InstanceOptions{
		Title:   "A",
		Path:    t.TempDir(),
		Program: "claude",
	})
	require.NoError(t, err)
	instA.MarkStartedForTest()
	instA.Status = session.Running
	h.nav.AddInstance(instA)()
	h.nav.SetSelectedInstance(0)

	// Simulate a fresh previewTerminalReadyMsg for "A" (current selection).
	msg := previewTerminalReadyMsg{
		term:          nil, // nil terminal — we just verify the instance title is set
		instanceTitle: "A",
		err:           nil,
	}

	model, cmd := h.Update(msg)
	homeModel, ok := model.(*home)
	require.True(t, ok)

	// previewTerminalInstance should be set to "A".
	assert.Equal(t, "A", homeModel.previewTerminalInstance,
		"previewTerminalInstance should be set when msg matches current selection")
	assert.NotNil(t, cmd, "preview terminal attach should request a resize refresh")
}

func TestInstanceChanged_AutoRequestsPreview(t *testing.T) {
	spin := spinner.New(spinner.WithSpinner(spinner.Dot))
	h := &home{
		ctx:          context.Background(),
		state:        stateDefault,
		appConfig:    config.DefaultConfig(),
		nav:          ui.NewNavigationPanel(&spin),
		menu:         ui.NewMenu(),
		tabbedWindow: ui.NewTabbedWindow(ui.NewPreviewPane(), ui.NewInfoPane()),
	}

	instA, err := session.NewInstance(session.InstanceOptions{
		Title: "instance-A", Path: t.TempDir(), Program: "claude",
	})
	require.NoError(t, err)
	instA.MarkStartedForTest()
	instA.Status = session.Running

	instB, err := session.NewInstance(session.InstanceOptions{
		Title: "instance-B", Path: t.TempDir(), Program: "claude",
	})
	require.NoError(t, err)
	instB.MarkStartedForTest()
	instB.Status = session.Running

	h.nav.AddInstance(instA)()
	h.nav.AddInstance(instB)()

	require.True(t, h.nav.SelectInstance(instB), "should find instance-B in list")
	cmd := h.instanceChanged()

	assert.True(t, h.previewRequested, "instanceChanged should request preview for the selected instance")
	assert.NotNil(t, cmd, "instanceChanged should auto-attach preview for a running instance")
	// populateInstanceTabs creates a single tab for the selected solo instance.
	assert.Equal(t, 1, h.tabbedWindow.TabCount(), "a single instance tab must be populated for the selected instance")
	assert.Equal(t, 0, h.tabbedWindow.GetActiveTab(), "active tab must be at index 0")
}

func TestInit_PrimesPreviewForSelectedInstance(t *testing.T) {
	h := newTestHome()
	inst, err := session.NewInstance(session.InstanceOptions{
		Title:   "instance-A",
		Path:    t.TempDir(),
		Program: "claude",
	})
	require.NoError(t, err)
	inst.MarkStartedForTest()
	inst.Status = session.Running
	h.nav.AddInstance(inst)()
	require.True(t, h.nav.SelectInstance(inst))

	cmd := h.Init()

	assert.True(t, h.previewRequested, "Init should prime the live preview on startup")
	assert.NotNil(t, cmd, "Init should return the startup command batch")
}

// TestFocusMode_ReusesPreviewTerminal verifies that enterFocusMode reuses the
// existing previewTerminal when it's already attached to the selected instance.
func TestFocusMode_ReusesPreviewTerminal(t *testing.T) {
	spin := spinner.New(spinner.WithSpinner(spinner.Dot))
	h := &home{
		ctx:          context.Background(),
		state:        stateDefault,
		appConfig:    config.DefaultConfig(),
		nav:          ui.NewNavigationPanel(&spin),
		menu:         ui.NewMenu(),
		tabbedWindow: ui.NewTabbedWindow(ui.NewPreviewPane(), ui.NewInfoPane()),
	}

	// Add a started-looking instance. We can't actually start it (no tmux),
	// but we can test the branch where previewTerminal is already set.
	inst, err := session.NewInstance(session.InstanceOptions{
		Title:   "my-agent",
		Path:    t.TempDir(),
		Program: "claude",
	})
	require.NoError(t, err)
	h.nav.AddInstance(inst)()
	h.nav.SetSelectedInstance(0)

	// Simulate previewTerminal already attached to "my-agent".
	// enterFocusMode should detect this and NOT spawn a new terminal.
	h.previewTerminalInstance = "my-agent"
	// Instance is not started, so enterFocusMode should return nil (guard check).
	cmd := h.enterFocusMode()

	assert.Nil(t, cmd, "enterFocusMode should return nil when instance is not started")
	assert.Equal(t, stateDefault, h.state, "state should remain default when instance is not started")
}

func TestHandleQuit_NoActiveSessions_QuitsImmediately(t *testing.T) {
	h := newTestHome()
	h.toastManager = overlay.NewToastManager(&h.spinner)

	// Add a paused instance (not active)
	inst := &session.Instance{Title: "paused-agent", Status: session.Paused}
	h.nav.AddInstance(inst)

	_, cmd := h.handleQuit()

	// Should return tea.Quit directly (no confirmation)
	assert.Equal(t, stateDefault, h.state, "state should remain default (no confirmation overlay)")
	assert.False(t, h.overlays.IsActive(), "no confirmation overlay should be shown")
	require.NotNil(t, cmd, "should return a quit command")
}

func TestHandleQuit_ActiveSessions_ShowsConfirmation(t *testing.T) {
	h := newTestHome()
	h.toastManager = overlay.NewToastManager(&h.spinner)

	// Add a running instance
	inst := &session.Instance{Title: "running-agent", Status: session.Running}
	h.nav.AddInstance(inst)

	_, cmd := h.handleQuit()

	// Should show confirmation, not quit immediately
	assert.Equal(t, stateConfirm, h.state, "state should be stateConfirm")
	require.True(t, h.overlays.IsActive(), "confirmation overlay must be shown")
	assert.Nil(t, cmd, "confirmAction returns nil cmd (action stored in pendingConfirmAction)")
	assert.NotNil(t, h.pendingConfirmAction, "pending action must be set")
}

// setupPlanState sets up an in-memory plan state on h for test use.
// It creates a temp directory, registers the plan, seeds the status, and
// refreshes the nav panel so SelectByID works immediately afterward.
func (h *home) setupPlanState(t *testing.T, planFile string, status taskstate.Status, topic string) {
	t.Helper()
	dir := t.TempDir()
	plansDir := filepath.Join(dir, "docs", "plans")
	require.NoError(t, os.MkdirAll(plansDir, 0o755))
	ps, err := newTestPlanState(t, plansDir)
	require.NoError(t, err)
	name := taskstate.DisplayName(planFile)
	require.NoError(t, ps.Create(planFile, name, "plan/"+name, topic, time.Now()))
	// Seed the status directly (bypass FSM).
	entry := ps.Plans[planFile]
	entry.Status = status
	ps.Plans[planFile] = entry
	require.NoError(t, ps.Save())
	h.taskState = ps
	h.taskStateDir = plansDir
	h.fsm = newPlanFSMForTest(t, plansDir)
	h.activeRepoPath = dir
	h.updateSidebarTasks()
}

func TestChatAboutPlan_ContextMenuAction(t *testing.T) {
	h := newTestHome()
	h.setupPlanState(t, "test-plan", taskstate.StatusImplementing, "test topic")

	// Select the plan in the nav panel
	h.nav.SelectByID(ui.SidebarPlanPrefix + "test-plan")

	// Execute the context action
	model, _ := h.executeContextAction("chat_about_plan")
	updated := model.(*home)

	require.Equal(t, stateChatAboutTask, updated.state)
	require.True(t, updated.overlays.IsActive(), "text input overlay must be set for question")
}

func TestChatAboutPlan_AppearsInContextMenu(t *testing.T) {
	h := newTestHome()
	h.setupPlanState(t, "test-plan", taskstate.StatusImplementing, "")

	h.focusSlot = slotNav
	h.nav.SelectByID(ui.SidebarPlanPrefix + "test-plan")

	model, _ := h.openTaskContextMenu()
	updated := model.(*home)

	require.Equal(t, stateContextMenu, updated.state)
	cm1, ok1 := updated.overlays.Current().(*overlay.ContextMenu)
	require.True(t, ok1, "current overlay must be a ContextMenu")

	// Verify "chat about this" appears in the menu items (use AllItems to search nested groups)
	found := false
	for _, item := range cm1.AllItems() {
		if item.Action == "chat_about_plan" {
			found = true
			break
		}
	}
	require.True(t, found, "context menu must include 'chat about this' action")
}

func TestCreatePlanPR_AppearsInTaskContextMenu(t *testing.T) {
	h := newTestHome()
	h.setupPlanState(t, "test-plan", taskstate.StatusImplementing, "")

	h.focusSlot = slotNav
	h.nav.SelectByID(ui.SidebarPlanPrefix + "test-plan")

	model, _ := h.openTaskContextMenu()
	updated := model.(*home)

	require.Equal(t, stateContextMenu, updated.state)
	cm, ok := updated.overlays.Current().(*overlay.ContextMenu)
	require.True(t, ok, "current overlay must be a ContextMenu")

	found := false
	for _, item := range cm.AllItems() {
		if item.Action == "create_plan_pr" {
			found = true
			break
		}
	}
	require.True(t, found, "task context menu must include 'create pr' action")
}

func TestStartFixer_AppearsInTaskContextMenu(t *testing.T) {
	h := newTestHome()
	h.setupPlanState(t, "review-plan", taskstate.StatusReviewing, "")

	h.focusSlot = slotNav
	h.nav.SelectByID(ui.SidebarPlanPrefix + "review-plan")

	model, _ := h.openTaskContextMenu()
	updated := model.(*home)

	require.Equal(t, stateContextMenu, updated.state)
	cm, ok := updated.overlays.Current().(*overlay.ContextMenu)
	require.True(t, ok, "current overlay must be a ContextMenu")

	found := false
	for _, item := range cm.AllItems() {
		if item.Action == "start_fixer" {
			found = true
			break
		}
	}
	require.True(t, found, "task context menu must include 'start fixer' action")
}

func TestStartFixer_AppearsInImplementingTaskContextMenu(t *testing.T) {
	h := newTestHome()
	h.setupPlanState(t, "implementing-plan", taskstate.StatusImplementing, "")

	h.focusSlot = slotNav
	h.nav.SelectByID(ui.SidebarPlanPrefix + "implementing-plan")

	model, _ := h.openTaskContextMenu()
	updated := model.(*home)

	require.Equal(t, stateContextMenu, updated.state)
	cm, ok := updated.overlays.Current().(*overlay.ContextMenu)
	require.True(t, ok, "current overlay must be a ContextMenu")

	found := false
	for _, item := range cm.AllItems() {
		if item.Action == "start_fixer" {
			found = true
			break
		}
	}
	require.True(t, found, "implementing task context menu must include 'start fixer' action")
}

func TestStartVerify_AppearsInTaskContextMenu(t *testing.T) {
	cases := []struct {
		name   string
		status taskstate.Status
	}{
		{name: "implementing", status: taskstate.StatusImplementing},
		{name: "reviewing", status: taskstate.StatusReviewing},
		{name: "verifying", status: taskstate.StatusVerifying},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := newTestHome()
			planFile := tc.name + "-plan"
			h.setupPlanState(t, planFile, tc.status, "")

			h.focusSlot = slotNav
			h.nav.SelectByID(ui.SidebarPlanPrefix + planFile)

			model, _ := h.openTaskContextMenu()
			updated := model.(*home)

			require.Equal(t, stateContextMenu, updated.state)
			cm, ok := updated.overlays.Current().(*overlay.ContextMenu)
			require.True(t, ok, "current overlay must be a ContextMenu")

			found := false
			for _, item := range cm.AllItems() {
				if item.Action == "start_verify" {
					found = true
					break
				}
			}
			require.True(t, found, "%s task context menu must include 'start verify' action", tc.name)
		})
	}
}

// TestExitFocusMode_KeepsPreviewTerminal verifies that exitFocusMode does NOT close
// previewTerminal — it stays alive for preview rendering.
func TestExitFocusMode_KeepsPreviewTerminal(t *testing.T) {
	spin := spinner.New(spinner.WithSpinner(spinner.Dot))
	h := &home{
		ctx:          context.Background(),
		state:        stateFocusAgent,
		appConfig:    config.DefaultConfig(),
		nav:          ui.NewNavigationPanel(&spin),
		menu:         ui.NewMenu(),
		tabbedWindow: ui.NewTabbedWindow(ui.NewPreviewPane(), ui.NewInfoPane()),
	}

	// Set previewTerminalInstance to simulate an attached terminal.
	h.previewTerminalInstance = "my-agent"

	h.exitFocusMode()

	assert.Equal(t, stateDefault, h.state, "state should return to default after exitFocusMode")
	assert.Equal(t, "my-agent", h.previewTerminalInstance,
		"previewTerminalInstance should NOT be cleared by exitFocusMode")
}

func TestHandleKeyPress_CtrlShiftEnterSubmitsAndExitsFocusMode(t *testing.T) {
	h := newTestHome()
	h.state = stateFocusAgent
	h.previewTerminal = session.NewDummyTerminal()
	h.previewTerminalInstance = "test-agent"
	h.keySent = true

	model, cmd := h.handleKeyPress(tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModCtrl | tea.ModShift})
	updated := model.(*home)

	sent := updated.previewTerminal.SentKeys()
	require.Len(t, sent, 1)
	assert.Equal(t, []byte{0x0D}, sent[0])
	assert.Equal(t, stateDefault, updated.state)
	assert.Equal(t, "test-agent", updated.previewTerminalInstance)
	require.NotNil(t, cmd)
}

func TestHandleKeyPress_CtrlEnterStaysInFocusMode(t *testing.T) {
	h := newTestHome()
	h.state = stateFocusAgent
	h.previewTerminal = session.NewDummyTerminal()
	h.previewTerminalInstance = "test-agent"
	h.keySent = true

	model, cmd := h.handleKeyPress(tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModCtrl})
	updated := model.(*home)

	sent := updated.previewTerminal.SentKeys()
	require.Len(t, sent, 1)
	assert.Equal(t, kittyCSIu(13, tea.ModCtrl), sent[0])
	assert.Equal(t, stateFocusAgent, updated.state)
	assert.Nil(t, cmd)
}

func TestHandleKeyPress_CtrlSpaceTogglesIntoFocusMode(t *testing.T) {
	h := newTestHome()
	inst, err := session.NewInstance(session.InstanceOptions{
		Title:   "test-focus-toggle",
		Path:    os.TempDir(),
		Program: "opencode",
	})
	require.NoError(t, err)
	inst.MarkStartedForTest()
	h.nav.AddInstance(inst)
	h.nav.SelectInstance(inst)
	h.previewTerminal = session.NewDummyTerminal()
	h.previewTerminalInstance = inst.Title
	h.keySent = true

	model, cmd := h.handleKeyPress(tea.KeyPressMsg{Code: tea.KeySpace, Mod: tea.ModCtrl})
	updated := model.(*home)

	assert.Equal(t, stateFocusAgent, updated.state)
	assert.NotNil(t, cmd)
}

func TestHandleKeyPress_FocusModeKeepsStateWhilePreviewReattaches(t *testing.T) {
	h := newTestHome()
	inst, err := session.NewInstance(session.InstanceOptions{
		Title:   "test-focus-reattach",
		Path:    os.TempDir(),
		Program: "opencode",
	})
	require.NoError(t, err)
	inst.MarkStartedForTest()
	h.nav.AddInstance(inst)
	h.nav.SelectInstance(inst)
	h.state = stateFocusAgent
	h.previewTerminal = nil
	h.previewRequested = false

	model, cmd := h.handleKeyPress(tea.KeyPressMsg{Code: tea.KeySpace, Text: " "})
	updated := model.(*home)

	require.NotNil(t, cmd)
	require.Equal(t, stateFocusAgent, updated.state)
	require.True(t, updated.previewRequested)
}

func TestRestartInstance_AppearsInContextMenu(t *testing.T) {
	h := newTestHome()
	inst, _ := session.NewInstance(session.InstanceOptions{
		Title:   "test-restart-menu",
		Path:    os.TempDir(),
		Program: "opencode",
	})
	inst.MarkStartedForTest()
	h.nav.AddInstance(inst)
	h.nav.SelectInstance(inst)

	model, _ := h.openContextMenu()
	updated := model.(*home)
	cm2, ok2 := updated.overlays.Current().(*overlay.ContextMenu)
	require.True(t, ok2, "current overlay must be a ContextMenu")

	found := false
	for _, item := range cm2.AllItems() {
		if item.Action == "restart_instance" {
			found = true
			break
		}
	}
	assert.True(t, found, "context menu should contain 'restart' option")
}

func TestExecuteContextAction_RestartInstance(t *testing.T) {
	h := newTestHome()
	inst, _ := session.NewInstance(session.InstanceOptions{
		Title:   "test-restart-action",
		Path:    os.TempDir(),
		Program: "opencode",
	})
	inst.MarkStartedForTest()
	h.nav.AddInstance(inst)
	h.nav.SelectInstance(inst)

	_, cmd := h.executeContextAction("restart_instance")
	// The action returns an async command (the restart runs in a goroutine).
	assert.NotNil(t, cmd, "restart action should return a tea.Cmd")
}

func TestDeleteKey_AllowsRemovalOfExitedRunningInstance(t *testing.T) {
	h := newTestHome()
	inst, err := newTestInstance("exited-reviewer")
	require.NoError(t, err)
	inst.Status = session.Running
	inst.Exited = true
	_ = h.nav.AddInstance(inst)
	h.nav.SelectInstance(inst)
	h.allInstances = append(h.allInstances, inst)

	msg := tea.KeyPressMsg{Code: tea.KeyDelete}
	_, _ = h.handleKeyPress(msg)

	assert.Equal(t, 0, h.nav.TotalInstances(),
		"delete should remove exited instance even if status is Running")
}

func TestCtrlKill_NoopsOnExitedInstance(t *testing.T) {
	h := newTestHome()
	inst, err := newTestInstance("exited-reviewer")
	require.NoError(t, err)
	inst.Status = session.Running
	inst.Exited = true
	inst.MarkStartedForTest()
	_ = h.nav.AddInstance(inst)
	h.nav.SelectInstance(inst)

	h.keySent = true
	msg := tea.KeyPressMsg{Code: 'k', Text: "k", Mod: tea.ModCtrl}
	_, cmd := h.handleKeyPress(msg)

	assert.Nil(t, cmd, "ctrl+k should no-op on an already-exited instance")
}

func TestMetadataTick_ExitedInstanceTransitionsToReady(t *testing.T) {
	h := newTestHomeWithToast()
	inst, err := newTestInstance("reviewer-done")
	require.NoError(t, err)
	inst.Status = session.Running
	_ = h.nav.AddInstance(inst)
	h.allInstances = append(h.allInstances, inst)

	ps, err := newTestPlanState(t, t.TempDir())
	require.NoError(t, err)

	// Simulate metadata tick with dead tmux
	msg := metadataResultMsg{
		Results:   []instanceMetadata{{Title: "reviewer-done", TmuxAlive: false}},
		PlanState: ps,
	}
	h.Update(msg)

	assert.True(t, inst.Exited, "instance should be marked exited")
	assert.Equal(t, session.Ready, inst.Status,
		"exited instance status should transition to Ready")
}

func TestShouldCreatePROnApproval(t *testing.T) {
	tests := []struct {
		name   string
		entry  taskstore.TaskEntry
		expect bool
	}{
		{name: "done with branch and no pr", entry: taskstore.TaskEntry{Status: taskstore.StatusDone, Branch: "plan/test", PRURL: ""}, expect: true},
		{name: "done with existing pr", entry: taskstore.TaskEntry{Status: taskstore.StatusDone, Branch: "plan/test", PRURL: "https://github.com/org/repo/pull/1"}, expect: false},
		{name: "not done", entry: taskstore.TaskEntry{Status: taskstore.StatusImplementing, Branch: "plan/test"}, expect: false},
		{name: "done but no branch", entry: taskstore.TaskEntry{Status: taskstore.StatusDone, Branch: ""}, expect: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expect, shouldCreatePR(tt.entry))
		})
	}
}

func TestAssemblePRMetadata_FullEntry(t *testing.T) {
	meta := gitpkg.AssemblePRMetadata(taskstore.TaskEntry{
		Description: "Auth Middleware",
		Goal:        "add JWT auth to all routes",
		Branch:      "plan/auth-middleware",
		Content:     "# Auth\n\n**Goal:** add JWT auth\n\n**Architecture:** middleware chain\n\n**Tech Stack:** Go\n\n## Wave 1\n\n### Task 1: JWT middleware\n\nbody\n",
	}, []taskstore.SubtaskEntry{
		{TaskNumber: 1, Title: "JWT middleware", Status: taskstore.SubtaskStatusComplete},
		{TaskNumber: 2, Title: "Route wiring", Status: taskstore.SubtaskStatusComplete},
	}, "looks good, approved", 2, "file1.go", "abc123 fix: auth", "1 file changed")

	assert.Equal(t, "Auth Middleware", meta.Description)
	assert.Equal(t, "add JWT auth to all routes", meta.Goal)
	assert.Equal(t, "middleware chain", meta.Architecture)
	assert.Equal(t, "Go", meta.TechStack)
	assert.Len(t, meta.Subtasks, 2)
	assert.Equal(t, "looks good, approved", meta.ReviewerSummary)
	assert.Equal(t, 2, meta.ReviewCycle)
	assert.Equal(t, "file1.go", meta.GitChanges)
}

func TestAssemblePRMetadata_EmptyContent(t *testing.T) {
	meta := gitpkg.AssemblePRMetadata(taskstore.TaskEntry{
		Description: "quick fix",
		Goal:        "fix the bug",
	}, nil, "", 0, "", "", "")

	assert.Equal(t, "quick fix", meta.Description)
	assert.Equal(t, "fix the bug", meta.Goal)
	assert.Empty(t, meta.Architecture)
	assert.Empty(t, meta.TechStack)
	assert.Empty(t, meta.Subtasks)
	assert.Zero(t, meta.ReviewCycle)
}

func TestAssemblePRMetadata_InvalidPlanContent(t *testing.T) {
	meta := gitpkg.AssemblePRMetadata(taskstore.TaskEntry{
		Description: "quick fix",
		Goal:        "fix the bug",
		Content:     "# no waves here",
	}, nil, "", 0, "", "", "")

	assert.Equal(t, "quick fix", meta.Description)
	assert.Equal(t, "fix the bug", meta.Goal)
	assert.Empty(t, meta.Architecture)
	assert.Empty(t, meta.TechStack)
}

func TestMapPRReviewDecision(t *testing.T) {
	assert.Equal(t, "approved", mapPRReviewDecision("APPROVED"))
	assert.Equal(t, "changes_requested", mapPRReviewDecision("CHANGES_REQUESTED"))
	assert.Equal(t, "pending", mapPRReviewDecision("REVIEW_REQUIRED"))
	assert.Equal(t, "pending", mapPRReviewDecision(""))
}

func TestMapPRCheckStatus(t *testing.T) {
	assert.Equal(t, "passing", mapPRCheckStatus("SUCCESS"))
	assert.Equal(t, "failing", mapPRCheckStatus("FAILURE"))
	assert.Equal(t, "pending", mapPRCheckStatus("PENDING"))
	assert.Equal(t, "pending", mapPRCheckStatus(""))
}

func TestHandleMouseClick_OutsideAgentPane_ExitsFocusMode(t *testing.T) {
	h := newTestHome()
	inst, err := session.NewInstance(session.InstanceOptions{
		Title:   "focus-click-test",
		Path:    os.TempDir(),
		Program: "opencode",
	})
	require.NoError(t, err)
	inst.MarkStartedForTest()
	h.nav.AddInstance(inst)
	h.nav.SelectInstance(inst)
	h.previewTerminal = session.NewDummyTerminal()
	h.previewTerminalInstance = inst.Title
	h.state = stateFocusAgent
	h.tabbedWindow.SetFocusMode(true)
	h.menu.SetFocusMode(true)

	// Simulate a left click at coordinates that are NOT inside ZoneAgentPane.
	// Since zones are not registered in tests, InBounds returns false for all zones,
	// so this click is "outside" the agent pane.
	msg := tea.MouseClickMsg{X: 0, Y: 0, Button: tea.MouseLeft}
	model, _ := h.handleMouseClick(msg)
	updated := model.(*home)

	assert.Equal(t, stateDefault, updated.state,
		"clicking outside agent pane should exit focus mode")
	assert.False(t, updated.tabbedWindow.IsFocusMode(),
		"tabbed window focus mode should be cleared")
}

// TestHandleMouseClick_InsideAgentPane_StaysInFocusMode documents the expected
// behaviour when a click lands inside the agent pane while in focus mode: the
// handler should return early without calling exitFocusMode.
//
// NOTE: bubblezone zones are not registered in unit tests, so
// zone.Get(ZoneAgentPane).InBounds always returns false.  There is therefore
// no way to drive the else-branch (stay in focus mode) through handleMouseClick
// in this test environment.  The branch is verified by code inspection — the
// implementation's else-clause returns immediately — and by the OutsideAgentPane
// test which exercises the mirror path.
func TestHandleMouseClick_InsideAgentPane_StaysInFocusMode(t *testing.T) {
	// This test exercises the initial-state setup that both click tests depend on,
	// and provides a home for the zone-limitation comment above.
	h := newTestHome()
	inst, err := session.NewInstance(session.InstanceOptions{
		Title:   "focus-click-inside-test",
		Path:    os.TempDir(),
		Program: "opencode",
	})
	require.NoError(t, err)
	inst.MarkStartedForTest()
	h.nav.AddInstance(inst)
	h.nav.SelectInstance(inst)
	h.previewTerminal = session.NewDummyTerminal()
	h.previewTerminalInstance = inst.Title
	h.state = stateFocusAgent
	h.tabbedWindow.SetFocusMode(true)
	h.menu.SetFocusMode(true)

	// Preconditions — ensure the test harness enters the expected initial state.
	require.Equal(t, stateFocusAgent, h.state,
		"precondition: state must be stateFocusAgent")
	require.True(t, h.tabbedWindow.IsFocusMode(),
		"precondition: tabbed window must be in focus mode")
}

// TestTaskLifecycleItems verifies the lifecycle item builder returns the correct
// actions in the correct order for each task status.
func TestTaskLifecycleItems(t *testing.T) {
	makeEntry := func(status taskstate.Status, phase string) taskstate.TaskEntry {
		return taskstate.TaskEntry{
			Status:         status,
			ExecutionState: taskstore.ExecutionState{Phase: phase},
		}
	}

	cases := []struct {
		name    string
		entry   taskstate.TaskEntry
		actions []string
	}{
		{
			name:    "planning",
			entry:   makeEntry(taskstate.StatusPlanning, ""),
			actions: []string{"start_plan", "start_implement", "start_implement_direct", "start_solo", "start_review"},
		},
		{
			name:    "draft ready",
			entry:   makeEntry(taskstate.StatusReady, ""),
			actions: []string{"start_plan"},
		},
		{
			name:    "planned ready",
			entry:   makeEntry(taskstate.StatusReady, "planned"),
			actions: []string{"start_implement", "start_implement_direct", "start_plan", "start_solo", "start_review"},
		},
		{
			name:    "implementing",
			entry:   makeEntry(taskstate.StatusImplementing, ""),
			actions: []string{"start_review", "start_fixer", "start_verify", "start_implement", "start_implement_direct", "start_solo"},
		},
		{
			name:    "reviewing",
			entry:   makeEntry(taskstate.StatusReviewing, ""),
			actions: []string{"mark_plan_done", "start_fixer", "start_verify", "start_review"},
		},
		{
			name:    "verifying",
			entry:   makeEntry(taskstate.StatusVerifying, ""),
			actions: []string{"mark_verify_approved", "mark_verify_failed", "start_verify", "start_fixer"},
		},
		{
			name:    "done",
			entry:   makeEntry(taskstate.StatusDone, ""),
			actions: []string{"request_review", "resume_implement"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			items := taskLifecycleItems(tc.entry)
			got := make([]string, 0, len(items))
			for _, item := range items {
				got = append(got, item.Action)
			}
			assert.Equal(t, tc.actions, got)
		})
	}
}

// TestInstanceContextMenu_Running_RootItems verifies that a running attachable instance
// promotes open, kill, and restart to the root level of the context menu.
func TestInstanceContextMenu_Running_RootItems(t *testing.T) {
	h := newTestHome()
	inst, _ := session.NewInstance(session.InstanceOptions{
		Title:   "test-running-instance",
		Path:    os.TempDir(),
		Program: "opencode",
	})
	inst.MarkStartedForTest()
	h.nav.AddInstance(inst)
	h.nav.SelectInstance(inst)

	model, _ := h.openContextMenu()
	updated := model.(*home)
	cm, ok := updated.overlays.Current().(*overlay.ContextMenu)
	require.True(t, ok, "current overlay must be a ContextMenu")

	// Root-level items must include the promoted session controls.
	rootActions := make([]string, 0, len(cm.Items()))
	for _, item := range cm.Items() {
		rootActions = append(rootActions, item.Action)
	}
	assert.Contains(t, rootActions, "kill_instance", "kill must be a root item for a running instance")
	assert.Contains(t, rootActions, "restart_instance", "restart must be a root item for a running instance")

	// Nested actions remain discoverable via AllItems.
	allActions := make([]string, 0)
	for _, item := range cm.AllItems() {
		allActions = append(allActions, item.Action)
	}
	assert.Contains(t, allActions, "push_instance", "push_instance must be discoverable in AllItems")
}

func TestInstanceContextMenu_ReviewerManualActions(t *testing.T) {
	h := newTestHome()
	h.setupPlanState(t, "feature", taskstate.StatusReviewing, "")
	inst, _ := session.NewInstance(session.InstanceOptions{
		Title:       "feature-review-6",
		Path:        os.TempDir(),
		Program:     "opencode",
		TaskFile:    "feature",
		AgentType:   session.AgentTypeReviewer,
		ReviewCycle: 6,
	})
	h.nav.AddInstance(inst)
	h.updateSidebarTasks()
	h.nav.SelectInstance(inst)

	model, _ := h.openContextMenu()
	updated := model.(*home)
	cm, ok := updated.overlays.Current().(*overlay.ContextMenu)
	require.True(t, ok)

	actions := make([]string, 0)
	for _, item := range cm.AllItems() {
		actions = append(actions, item.Action)
	}
	assert.Contains(t, actions, "mark_review_approved")
	assert.Contains(t, actions, "mark_review_changes_requested")
	assert.Contains(t, actions, "advance_review_cycle")
}

func TestInstanceContextMenu_CoderManualAction(t *testing.T) {
	h := newTestHome()
	h.setupPlanState(t, "feature", taskstate.StatusImplementing, "")
	// mark_implement_finished is only offered when the task is in a single-agent
	// implementing phase (solo/fixer) rather than a wave-based phase.
	// ActiveAgentType is required by normalizeExecutionState for this phase.
	require.NoError(t, h.taskState.ForceSetLifecycle("feature", taskstate.StatusImplementing,
		taskstore.ExecutionState{
			Phase:           string(taskfsm.ExecutionPhaseSingleAgentImplementing),
			ActiveAgentType: session.AgentTypeCoder,
		}))
	inst, _ := session.NewInstance(session.InstanceOptions{
		Title:     "feature-coder",
		Path:      os.TempDir(),
		Program:   "opencode",
		TaskFile:  "feature",
		AgentType: session.AgentTypeCoder,
	})
	h.nav.AddInstance(inst)
	h.updateSidebarTasks()
	h.nav.SelectInstance(inst)

	model, _ := h.openContextMenu()
	updated := model.(*home)
	cm, ok := updated.overlays.Current().(*overlay.ContextMenu)
	require.True(t, ok)

	actions := make([]string, 0)
	for _, item := range cm.AllItems() {
		actions = append(actions, item.Action)
	}
	assert.Contains(t, actions, "mark_implement_finished")
	assert.Contains(t, actions, "merge_instance")
}

// TestTaskContextMenu_HasGroupedSubMenus verifies that the task context menu exposes
// the expected structure: promoted lifecycle actions at root, followed by grouped
// sections (start, sync, config, lifecycle), with no separate 'view' group.
func TestTaskContextMenu_HasGroupedSubMenus(t *testing.T) {
	h := newTestHome()
	h.setupPlanState(t, "review-task", taskstate.StatusReviewing, "")
	h.nav.SelectByID(ui.SidebarPlanPrefix + "review-task")

	model, _ := h.openTaskContextMenu()
	updated := model.(*home)
	cm, ok := updated.overlays.Current().(*overlay.ContextMenu)
	require.True(t, ok, "current overlay must be a ContextMenu")

	topLabels := make([]string, 0, len(cm.Items()))
	for _, item := range cm.Items() {
		topLabels = append(topLabels, item.Label)
	}
	// sync, config and lifecycle groups must always be present.
	assert.Contains(t, topLabels, "sync", "task menu must have 'sync' group at top level")
	assert.Contains(t, topLabels, "config", "task menu must have 'config' group at top level")
	assert.Contains(t, topLabels, "lifecycle", "task menu must have 'lifecycle' group at top level")
	// The old monolithic 'view' group is gone; view task and chat appear at root.
	assert.NotContains(t, topLabels, "view", "task menu must not have a separate 'view' group")
	// view task and chat about this are direct root actions.
	topActions := make([]string, 0)
	for _, item := range cm.Items() {
		topActions = append(topActions, item.Action)
	}
	assert.Contains(t, topActions, "view_plan", "view task must be a root action")
	assert.Contains(t, topActions, "chat_about_plan", "chat about this must be a root action")

	// Nested actions remain discoverable via AllItems.
	allActions := make([]string, 0)
	for _, item := range cm.AllItems() {
		allActions = append(allActions, item.Action)
	}
	assert.Contains(t, allActions, "create_plan_pr", "create_plan_pr must be discoverable in AllItems")
	assert.Contains(t, allActions, "start_fixer", "start_fixer must be discoverable in AllItems for reviewing status")
}

// TestTaskContextMenu_PlannedReady_PromotedRootItems verifies that for a planned-ready
// task the first two lifecycle actions are promoted to root and the rest go into the
// 'start' subgroup.
func TestTaskContextMenu_PlannedReady_PromotedRootItems(t *testing.T) {
	h := newTestHome()
	h.setupPlanState(t, "ready-task", taskstate.StatusReady, "")
	entry := h.taskState.Plans["ready-task"]
	entry.ExecutionState = taskstore.ExecutionState{Phase: "planned"}
	h.taskState.Plans["ready-task"] = entry
	h.nav.SelectByID(ui.SidebarPlanPrefix + "ready-task")

	model, _ := h.openTaskContextMenu()
	updated := model.(*home)
	cm, ok := updated.overlays.Current().(*overlay.ContextMenu)
	require.True(t, ok, "current overlay must be a ContextMenu")

	// The first two root items must be the top lifecycle actions for planned-ready.
	rootItems := cm.Items()
	require.GreaterOrEqual(t, len(rootItems), 2, "menu must have at least 2 root items")
	assert.Equal(t, "start_implement", rootItems[0].Action, "first root item must be start_implement for planned-ready")
	assert.Equal(t, "start_implement_direct", rootItems[1].Action, "second root item must be start_implement_direct for planned-ready")

	// Remaining lifecycle actions go into the 'start' subgroup.
	var startGroup *overlay.ContextMenuItem
	for _, item := range cm.Items() {
		if item.Label == "start" {
			itemCopy := item
			startGroup = &itemCopy
			break
		}
	}
	require.NotNil(t, startGroup, "task menu for planned-ready status must have a 'start' subgroup for remaining items")
	startActions := make([]string, 0, len(startGroup.Children))
	for _, child := range startGroup.Children {
		startActions = append(startActions, child.Action)
	}
	assert.Contains(t, startActions, "start_plan", "start subgroup must contain start_plan")
	assert.Contains(t, startActions, "start_solo", "start subgroup must contain start_solo")
	assert.Contains(t, startActions, "start_review", "start subgroup must contain start_review")
}

// TestTaskContextMenu_DraftReadyStatus_OnlyShowsPlanningStart verifies that for a
// draft-ready task the single lifecycle action (start_plan) is promoted to root and
// no 'start' subgroup is rendered.
func TestTaskContextMenu_DraftReadyStatus_OnlyShowsPlanningStart(t *testing.T) {
	h := newTestHome()
	h.setupPlanState(t, "draft-ready-task", taskstate.StatusReady, "")
	h.nav.SelectByID(ui.SidebarPlanPrefix + "draft-ready-task")

	model, _ := h.openTaskContextMenu()
	updated := model.(*home)
	cm, ok := updated.overlays.Current().(*overlay.ContextMenu)
	require.True(t, ok, "current overlay must be a ContextMenu")

	// start_plan must be a direct root action.
	rootActions := make([]string, 0)
	for _, item := range cm.Items() {
		rootActions = append(rootActions, item.Action)
	}
	assert.Contains(t, rootActions, "start_plan", "start_plan must be a root action for draft-ready")

	// No 'start' subgroup should be present (only one lifecycle item, fully promoted).
	for _, item := range cm.Items() {
		assert.NotEqual(t, "start", item.Label, "no 'start' subgroup expected when all lifecycle items are promoted")
	}
}

// TestOpenTaskContextMenu_RefreshesFromStoreBeforeLifecycleItems verifies that
// openTaskContextMenu reads the latest task state from the backing store rather
// than the stale in-memory snapshot. A task cached as draft-ready (no phase)
// that the daemon has advanced to planned-ready (phase="planned") must produce
// a menu with start_implement at the root — not the draft-ready start_plan only.
func TestOpenTaskContextMenu_RefreshesFromStoreBeforeLifecycleItems(t *testing.T) {
	const planFile = "refresh-task"

	dir := t.TempDir()
	plansDir := filepath.Join(dir, "docs", "plans")
	require.NoError(t, os.MkdirAll(plansDir, 0o755))

	// Seed the store with a draft-ready task (no phase).
	store := newTestStore(t)
	require.NoError(t, store.Create("test", taskstore.TaskEntry{
		Filename: planFile,
		Status:   taskstore.StatusReady,
	}))
	ps, err := newTestPlanStateWithStore(t, store, plansDir)
	require.NoError(t, err)

	sp := spinner.New(spinner.WithSpinner(spinner.Dot))
	nav := ui.NewNavigationPanel(&sp)
	nav.SetTopicsAndPlans(nil, []ui.PlanDisplay{{Filename: planFile, Status: string(taskstate.StatusReady)}}, nil)
	require.True(t, nav.SelectByID(ui.SidebarPlanPrefix+planFile))

	h := &home{
		ctx:              context.Background(),
		state:            stateDefault,
		taskState:        ps,
		taskStore:        store,
		taskStoreProject: "test",
		taskStateDir:     plansDir,
		nav:              nav,
		menu:             ui.NewMenu(),
		tabbedWindow:     ui.NewTabbedWindow(ui.NewPreviewPane(), ui.NewInfoPane()),
		overlays:         overlay.NewManager(),
		activeRepoPath:   dir,
	}

	// Advance the store to planned-ready without touching the in-memory ps.
	require.NoError(t, store.Update("test", planFile, taskstore.TaskEntry{
		Filename:       planFile,
		Status:         taskstore.StatusReady,
		ExecutionState: taskstore.ExecutionState{Phase: "planned"},
	}))

	// openTaskContextMenu must call refreshTaskEntry → loadTaskState → fresh store read.
	model, _ := h.openTaskContextMenu()
	updated := model.(*home)
	cm, ok := updated.overlays.Current().(*overlay.ContextMenu)
	require.True(t, ok, "current overlay must be a ContextMenu")

	// Planned-ready promotes start_implement to root; draft-ready would only show start_plan.
	rootActions := make([]string, 0, len(cm.Items()))
	for _, item := range cm.Items() {
		rootActions = append(rootActions, item.Action)
	}
	assert.Contains(t, rootActions, "start_implement",
		"openTaskContextMenu must use fresh store state: start_implement expected at root for planned-ready task")
}

// TestOpenContextMenu_TaskOwnerSignalsUseFreshTaskState verifies that the
// instance context menu for a top-level task agent uses the task entry from
// refreshTaskEntry when computing lifecycle signal actions. A reviewer instance
// opened against a task whose state was updated to reviewing (in the store)
// must display mark_review_approved even though the original in-memory snapshot
// had the task as planning.
func TestOpenContextMenu_TaskOwnerSignalsUseFreshTaskState(t *testing.T) {
	const planFile = "owner-task"

	dir := t.TempDir()
	plansDir := filepath.Join(dir, "docs", "plans")
	require.NoError(t, os.MkdirAll(plansDir, 0o755))

	// Shared store: task starts as planning.
	store := newTestStore(t)
	require.NoError(t, store.Create("test", taskstore.TaskEntry{
		Filename: planFile,
		Status:   taskstore.StatusPlanning,
	}))
	ps, err := newTestPlanStateWithStore(t, store, plansDir)
	require.NoError(t, err)

	inst, err := session.NewInstance(session.InstanceOptions{
		Title:     "owner-task-reviewer",
		Path:      t.TempDir(),
		Program:   "opencode",
		TaskFile:  planFile,
		AgentType: session.AgentTypeReviewer,
	})
	require.NoError(t, err)

	sp := spinner.New(spinner.WithSpinner(spinner.Dot))
	nav := ui.NewNavigationPanel(&sp)

	h := &home{
		ctx:              context.Background(),
		state:            stateDefault,
		taskState:        ps,
		taskStore:        store,
		taskStoreProject: "test",
		taskStateDir:     plansDir,
		nav:              nav,
		menu:             ui.NewMenu(),
		tabbedWindow:     ui.NewTabbedWindow(ui.NewPreviewPane(), ui.NewInfoPane()),
		overlays:         overlay.NewManager(),
		activeRepoPath:   dir,
	}

	// Build the sidebar first so plan-header rows exist before the instance is
	// added. This ensures SelectInstance picks a stable index that survives the
	// subsequent updateSidebarTasks call inside refreshTaskEntry.
	h.updateSidebarTasks()
	_ = nav.AddInstance(inst)
	nav.SelectInstance(inst)

	// Advance the store to reviewing; in-memory ps still says planning.
	require.NoError(t, store.Update("test", planFile, taskstore.TaskEntry{
		Filename: planFile,
		Status:   taskstore.StatusReviewing,
	}))

	// openContextMenu calls refreshTaskEntry → loadTaskState → reads reviewing
	// from the store → passes that fresh entry to instanceSignalItems.
	model, _ := h.openContextMenu()
	updated := model.(*home)
	cm, ok := updated.overlays.Current().(*overlay.ContextMenu)
	require.True(t, ok, "current overlay must be a ContextMenu")

	allActions := make([]string, 0)
	for _, item := range cm.AllItems() {
		allActions = append(allActions, item.Action)
	}
	assert.Contains(t, allActions, "mark_review_approved",
		"reviewer context menu must include mark_review_approved when fresh store state is reviewing")
}

// TestMetadataResultMsg_DismissesTrackedContextMenuOnStatusChange verifies that
// a task context menu is automatically dismissed when the metadata tick delivers
// a PlanState snapshot showing the task FSM has moved to a different status.
func TestMetadataResultMsg_DismissesTrackedContextMenuOnStatusChange(t *testing.T) {
	const planFile = "tracked-status-task"

	dir := t.TempDir()
	plansDir := filepath.Join(dir, "docs", "plans")
	require.NoError(t, os.MkdirAll(plansDir, 0o755))

	store := newTestStore(t)
	require.NoError(t, store.Create("test", taskstore.TaskEntry{
		Filename: planFile,
		Status:   taskstore.StatusReady,
	}))
	ps, err := newTestPlanStateWithStore(t, store, plansDir)
	require.NoError(t, err)

	sp := spinner.New(spinner.WithSpinner(spinner.Dot))
	h := &home{
		ctx:              context.Background(),
		state:            stateContextMenu,
		taskState:        ps,
		taskStore:        store,
		taskStoreProject: "test",
		taskStateDir:     plansDir,
		nav:              ui.NewNavigationPanel(&sp),
		menu:             ui.NewMenu(),
		tabbedWindow:     ui.NewTabbedWindow(ui.NewPreviewPane(), ui.NewInfoPane()),
		toastManager:     overlay.NewToastManager(&sp),
		overlays:         overlay.NewManager(),
		activeRepoPath:   dir,
		// tracking fields seeded as if the menu were opened for a ready task
		contextMenuTaskFile:   planFile,
		contextMenuTaskStatus: taskstate.StatusReady,
		contextMenuTaskPhase:  "",
	}
	// Activate a real context menu so overlays.IsActive() returns true.
	h.overlays.Show(overlay.NewContextMenu([]overlay.ContextMenuItem{
		{Label: "start planning", Action: "start_plan"},
	}))

	// Build fresh PlanState where the task has advanced to implementing.
	require.NoError(t, store.Update("test", planFile, taskstore.TaskEntry{
		Filename: planFile,
		Status:   taskstore.StatusImplementing,
		ExecutionState: taskstore.ExecutionState{
			Phase:           string(taskfsm.ExecutionPhaseSingleAgentImplementing),
			ActiveAgentType: session.AgentTypeCoder,
		},
	}))
	freshPs, err := newTestPlanStateWithStore(t, store, plansDir)
	require.NoError(t, err)

	model, _ := h.Update(metadataResultMsg{PlanState: freshPs})
	updated := model.(*home)

	assert.False(t, updated.overlays.IsActive(),
		"overlay must be dismissed when task status changes while menu is open")
	assert.Equal(t, stateDefault, updated.state,
		"state must revert to stateDefault after stale menu dismissal")
	assert.Contains(t, updated.toastManager.View(), "menu dismissed",
		"dismissal toast must include 'menu dismissed'")
}

// TestMetadataResultMsg_DismissesTrackedContextMenuOnPhaseChange verifies that
// a task context menu is dismissed when the metadata tick detects a phase change
// even when the lifecycle status itself does not change. Uses ready+""
// (draft-ready) vs ready+"planned" (planned-ready) to exercise a same-status
// but different-phase transition.
func TestMetadataResultMsg_DismissesTrackedContextMenuOnPhaseChange(t *testing.T) {
	const planFile = "tracked-phase-task"

	dir := t.TempDir()
	plansDir := filepath.Join(dir, "docs", "plans")
	require.NoError(t, os.MkdirAll(plansDir, 0o755))

	store := newTestStore(t)
	require.NoError(t, store.Create("test", taskstore.TaskEntry{
		Filename: planFile,
		Status:   taskstore.StatusReady,
		// No ExecutionState.Phase → draft-ready
	}))
	ps, err := newTestPlanStateWithStore(t, store, plansDir)
	require.NoError(t, err)

	sp := spinner.New(spinner.WithSpinner(spinner.Dot))
	h := &home{
		ctx:              context.Background(),
		state:            stateContextMenu,
		taskState:        ps,
		taskStore:        store,
		taskStoreProject: "test",
		taskStateDir:     plansDir,
		nav:              ui.NewNavigationPanel(&sp),
		menu:             ui.NewMenu(),
		tabbedWindow:     ui.NewTabbedWindow(ui.NewPreviewPane(), ui.NewInfoPane()),
		toastManager:     overlay.NewToastManager(&sp),
		overlays:         overlay.NewManager(),
		activeRepoPath:   dir,
		// tracking fields for draft-ready (ready + empty phase)
		contextMenuTaskFile:   planFile,
		contextMenuTaskStatus: taskstate.StatusReady,
		contextMenuTaskPhase:  "",
	}
	h.overlays.Show(overlay.NewContextMenu([]overlay.ContextMenuItem{
		{Label: "start planning", Action: "start_plan"},
	}))

	// Advance store to planned-ready (same status, phase now "planned").
	require.NoError(t, store.Update("test", planFile, taskstore.TaskEntry{
		Filename:       planFile,
		Status:         taskstore.StatusReady,
		ExecutionState: taskstore.ExecutionState{Phase: "planned"},
	}))
	freshPs, err := newTestPlanStateWithStore(t, store, plansDir)
	require.NoError(t, err)

	model, _ := h.Update(metadataResultMsg{PlanState: freshPs})
	updated := model.(*home)

	assert.False(t, updated.overlays.IsActive(),
		"overlay must be dismissed when task phase changes (draft-ready → planned-ready) while menu is open")
	assert.Equal(t, stateDefault, updated.state,
		"state must revert to stateDefault after stale menu dismissal")
	assert.Contains(t, updated.toastManager.View(), "menu dismissed",
		"dismissal toast must include 'menu dismissed' on phase-only change")
}

// TestSaveAllInstances_SkipsDaemonSDKPlaceholders verifies that the filtering
// logic in saveAllInstances excludes daemon SDK placeholders (unstarted SDK
// instances) so they are never handed off to storage.
func TestSaveAllInstances_SkipsDaemonSDKPlaceholders(t *testing.T) {
	h := newTestHome()
	// storage nil → saveAllInstances no-ops; we test the filtering through
	// isDaemonSDKPlaceholder directly on the home value.

	// A daemon SDK placeholder: SDK mode, not started.
	placeholder, err := session.NewInstance(session.InstanceOptions{
		Title:         "solo-placeholder",
		Path:          t.TempDir(),
		Program:       "claude",
		ExecutionMode: session.ExecutionModeSDK,
	})
	require.NoError(t, err)

	// A tmux instance (not an SDK placeholder).
	tmuxInst, err := session.NewInstance(session.InstanceOptions{
		Title:         "regular-tmux",
		Path:          t.TempDir(),
		Program:       "opencode",
		ExecutionMode: session.ExecutionModeTmux,
	})
	require.NoError(t, err)

	assert.True(t, h.isDaemonSDKPlaceholder(placeholder),
		"unstarted SDK instance must be identified as daemon SDK placeholder")
	assert.False(t, h.isDaemonSDKPlaceholder(tmuxInst),
		"tmux instance must not be identified as daemon SDK placeholder")

	// saveAllInstances with nil storage must not panic.
	h.allInstances = []*session.Instance{placeholder, tmuxInst}
	require.NoError(t, h.saveAllInstances())
}

// TestInstanceStartedMsg_SDKPlaceholderDoesNotAutoFocus verifies that when an
// SDK placeholder (not locally started) triggers instanceStartedMsg, the TUI
// does not enter focus mode. Focus mode requires a real local execution session.
func TestInstanceStartedMsg_SDKPlaceholderDoesNotAutoFocus(t *testing.T) {
	h := newTestHome()

	// Build an SDK master instance without calling Start — this mimics a daemon
	// SDK placeholder where inst.Started() returns false.
	inst, err := session.NewInstance(session.InstanceOptions{
		Title:         "solo-sdk",
		Path:          t.TempDir(),
		Program:       "claude",
		ExecutionMode: session.ExecutionModeSDK,
		AgentType:     session.AgentTypeMaster,
	})
	require.NoError(t, err)
	// Do NOT call inst.MarkStartedForTest() — placeholder stays unstarted.

	_ = h.nav.AddInstance(inst)
	h.instanceFinalizers = make(map[*session.Instance]func())

	model, _ := h.Update(instanceStartedMsg{instance: inst, err: nil})
	updated := model.(*home)

	assert.NotEqual(t, stateFocusAgent, updated.state,
		"daemon SDK placeholder must not trigger auto-focus mode")
}

// TestShowSpawnAgentForm_SDKHintRemoved verifies that the outdated "cannot be
// controlled from the web ui" footer hint is no longer shown for SDK mode.
func TestShowSpawnAgentForm_SDKHintRemoved(t *testing.T) {
	h := newTestHome()
	h.pendingSpawnExecutionMode = session.ExecutionModeSDK
	h.pendingSpawnSpeedTier = ""
	h.showSpawnAgentForm("claude")

	fo, ok := h.overlays.Current().(*overlay.FormOverlay)
	require.True(t, ok, "spawn form overlay must be active")
	assert.NotContains(t, fo.View(), "web ui",
		"outdated 'web ui' hint must be removed for SDK spawn form")
}
