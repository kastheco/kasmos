package app

import (
	"context"
	"net/http"
	"regexp"
	"strings"
	"testing"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"github.com/kastheco/kasmos/config"
	"github.com/kastheco/kasmos/config/taskstate"
	daemonpkg "github.com/kastheco/kasmos/daemon"
	"github.com/kastheco/kasmos/session"
	"github.com/kastheco/kasmos/session/tmux"
	"github.com/kastheco/kasmos/ui"
	"github.com/kastheco/kasmos/ui/overlay"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testANSISequenceRE = regexp.MustCompile(`\x1b\[[0-9;?]*[ -/]*[@-~]`)

func permissionOverlayClickTarget(t *testing.T, view, needle string) (int, int) {
	t.Helper()
	for y, line := range strings.Split(view, "\n") {
		clean := overlayStripANSIForTest(line)
		x := strings.Index(clean, needle)
		if x >= 0 {
			return x, y
		}
	}
	require.FailNowf(t, "missing target", "could not find %q in view", needle)
	return 0, 0
}

func overlayStripANSIForTest(s string) string {
	return testANSISequenceRE.ReplaceAllString(s, "")
}

// newTestHomeWithCache returns a home with a real permissionStore backed by an in-memory SQLite DB.
func newTestHomeWithCache(t *testing.T) *home {
	t.Helper()
	permStore, err := config.NewSQLitePermissionStore(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { permStore.Close() })

	spin := spinner.New(spinner.WithSpinner(spinner.Dot))
	return &home{
		ctx:               context.Background(),
		state:             stateDefault,
		appConfig:         config.DefaultConfig(),
		nav:               ui.NewNavigationPanel(&spin),
		menu:              ui.NewMenu(),
		tabbedWindow:      ui.NewTabbedWindow(ui.NewPreviewPane(), ui.NewInfoPane()),
		toastManager:      overlay.NewToastManager(&spin),
		overlays:          overlay.NewManager(),
		activeRepoPath:    t.TempDir(),
		program:           "opencode",
		permissionStore:   permStore,
		permissionHandled: make(map[*session.Instance]string),
	}
}

// requirePreOverlaySaved asserts that a pre-overlay nav row has been captured
// and matches the expected nav row id.
func requirePreOverlaySaved(t *testing.T, m *home, wantID string) {
	t.Helper()
	require.True(t, m.preOverlayCaptured, "preOverlayCaptured must be set")
	require.Equal(t, wantID, m.preOverlayNavID, "preOverlayNavID must match captured row id")
}

// assertPreOverlayCleared asserts both pre-overlay fields are back to zero.
func assertPreOverlayCleared(t *testing.T, m *home) {
	t.Helper()
	assert.False(t, m.preOverlayCaptured, "preOverlayCaptured must be cleared")
	assert.Empty(t, m.preOverlayNavID, "preOverlayNavID must be cleared")
}

// collectAutoApproveMsgs runs a tea.Cmd recursively and collects all permissionAutoApproveMsg values.
func collectAutoApproveMsgs(cmd tea.Cmd) []permissionAutoApproveMsg {
	if cmd == nil {
		return nil
	}
	msg := cmd()
	var results []permissionAutoApproveMsg
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, sub := range batch {
			results = append(results, collectAutoApproveMsgs(sub)...)
		}
	} else if pam, ok := msg.(permissionAutoApproveMsg); ok {
		results = append(results, pam)
	}
	return results
}

// --- Update() cycle integration tests ---

// TestUpdate_PermissionPromptDetection_ShowsOverlay exercises the real metadata-tick
// detection path through Update(), rather than manually setting m.state.
func TestUpdate_PermissionPromptDetection_ShowsOverlay(t *testing.T) {
	m := newTestHomeWithCache(t)
	inst := &session.Instance{Title: "test-agent", Program: "opencode"}
	inst.MarkStartedForTest()
	m.nav.AddInstance(inst)()

	pp := &session.PermissionPrompt{Pattern: "/opt/*", Description: "Access /opt"}
	msg := metadataResultMsg{
		Results: []instanceMetadata{
			{Title: "test-agent", PermissionPrompt: pp},
		},
	}

	_, _ = m.Update(msg)

	assert.Equal(t, statePermission, m.state)
	require.NotNil(t, m.overlays.Current(), "permission overlay must be active")
	_, ok := m.overlays.Current().(*overlay.PermissionOverlay)
	assert.True(t, ok, "active overlay must be a PermissionOverlay")
	assert.NotNil(t, m.pendingPermissionInstance)
}

// TestUpdate_PermissionAutoApprove_FiresOnCachedPattern verifies that a cached pattern
// fires permissionAutoApproveMsg (not the modal) on the first tick.
func TestUpdate_PermissionAutoApprove_FiresOnCachedPattern(t *testing.T) {
	m := newTestHomeWithCache(t)
	m.permissionStore.Remember(m.activeProject(), "/opt/*")

	inst := &session.Instance{Title: "test-agent", Program: "opencode"}
	inst.MarkStartedForTest()
	m.nav.AddInstance(inst)()

	pp := &session.PermissionPrompt{Pattern: "/opt/*", Description: "Access /opt"}
	msg := metadataResultMsg{
		Results: []instanceMetadata{
			{Title: "test-agent", PermissionPrompt: pp},
		},
	}

	_, cmd := m.Update(msg)
	approvals := collectAutoApproveMsgs(cmd)

	assert.Len(t, approvals, 1, "first tick should queue exactly one auto-approve")
	assert.Equal(t, stateDefault, m.state, "auto-approve should not change state")
}

func TestUpdate_PermissionPromptDetection_ShowsOverlayForClaude(t *testing.T) {
	m := newTestHomeWithCache(t)
	inst := &session.Instance{Title: "test-agent", Program: "claude"}
	inst.MarkStartedForTest()
	m.nav.AddInstance(inst)()

	pp := &session.PermissionPrompt{Pattern: "Bash", Description: "Allow tool Bash?"}
	msg := metadataResultMsg{
		Results: []instanceMetadata{
			{Title: "test-agent", PermissionPrompt: pp},
		},
	}

	_, _ = m.Update(msg)

	assert.Equal(t, statePermission, m.state)
	require.NotNil(t, m.overlays.Current(), "permission overlay must be active")
	_, ok := m.overlays.Current().(*overlay.PermissionOverlay)
	assert.True(t, ok, "active overlay must be a PermissionOverlay")
	assert.Equal(t, inst, m.pendingPermissionInstance)
	assert.Equal(t, "Bash", m.pendingPermissionPattern)
	assert.Equal(t, "Allow tool Bash?", m.pendingPermissionDesc)
}

func TestUpdate_PermissionAutoApprove_FiresOnCachedClaudePattern(t *testing.T) {
	m := newTestHomeWithCache(t)
	m.permissionStore.Remember(m.activeProject(), "Bash")

	inst := &session.Instance{Title: "test-agent", Program: "claude"}
	inst.MarkStartedForTest()
	m.nav.AddInstance(inst)()

	pp := &session.PermissionPrompt{Pattern: "Bash", Description: "Allow tool Bash?"}
	msg := metadataResultMsg{
		Results: []instanceMetadata{
			{Title: "test-agent", PermissionPrompt: pp},
		},
	}

	_, cmd := m.Update(msg)
	approvals := collectAutoApproveMsgs(cmd)

	assert.Len(t, approvals, 1, "first claude tick should queue exactly one auto-approve")
	assert.Equal(t, inst, approvals[0].instance)
	assert.Equal(t, stateDefault, m.state, "auto-approve should not change state")
}

// TestUpdate_PermissionAutoApprove_DeduplicatesOnMultipleTicks is the critical regression test:
// a second metadata tick with the same prompt (before opencode clears it) must NOT fire
// a second auto-approve, which would corrupt opencode's input state.
func TestUpdate_PermissionAutoApprove_DeduplicatesOnMultipleTicks(t *testing.T) {
	m := newTestHomeWithCache(t)
	m.permissionStore.Remember(m.activeProject(), "/opt/*")

	inst := &session.Instance{Title: "test-agent", Program: "opencode"}
	inst.MarkStartedForTest()
	m.nav.AddInstance(inst)()

	pp := &session.PermissionPrompt{Pattern: "/opt/*", Description: "Access /opt"}
	msg := metadataResultMsg{
		Results: []instanceMetadata{
			{Title: "test-agent", PermissionPrompt: pp},
		},
	}

	// First tick — should fire once.
	_, cmd1 := m.Update(msg)
	approvals1 := collectAutoApproveMsgs(cmd1)
	assert.Len(t, approvals1, 1, "first tick should queue one auto-approve")

	// Second tick — pane still shows the prompt (opencode hasn't processed the keys yet).
	// Must NOT fire again.
	_, cmd2 := m.Update(msg)
	approvals2 := collectAutoApproveMsgs(cmd2)
	assert.Len(t, approvals2, 0, "second tick must not queue a duplicate auto-approve")
}

func TestUpdate_PermissionAutoApprove_DeduplicatesClaudeOnMultipleTicks(t *testing.T) {
	m := newTestHomeWithCache(t)
	m.permissionStore.Remember(m.activeProject(), "Bash")

	inst := &session.Instance{Title: "test-agent", Program: "claude"}
	inst.MarkStartedForTest()
	m.nav.AddInstance(inst)()

	pp := &session.PermissionPrompt{Pattern: "Bash", Description: "Allow tool Bash?"}
	msg := metadataResultMsg{
		Results: []instanceMetadata{
			{Title: "test-agent", PermissionPrompt: pp},
		},
	}

	_, cmd1 := m.Update(msg)
	approvals1 := collectAutoApproveMsgs(cmd1)
	assert.Len(t, approvals1, 1, "first claude tick should queue one auto-approve")

	_, cmd2 := m.Update(msg)
	approvals2 := collectAutoApproveMsgs(cmd2)
	assert.Len(t, approvals2, 0, "second claude tick must not queue a duplicate auto-approve")
}

// TestUpdate_PermissionAutoApprove_ClearsGuardWhenPromptGone verifies that once the
// permission prompt disappears from the pane the deduplication guard is cleared,
// allowing a future prompt to trigger auto-approve again.
func TestUpdate_PermissionAutoApprove_ClearsGuardWhenPromptGone(t *testing.T) {
	m := newTestHomeWithCache(t)
	m.permissionStore.Remember(m.activeProject(), "/opt/*")

	inst := &session.Instance{Title: "test-agent", Program: "opencode"}
	inst.MarkStartedForTest()
	m.nav.AddInstance(inst)()

	pp := &session.PermissionPrompt{Pattern: "/opt/*", Description: "Access /opt"}
	withPrompt := metadataResultMsg{
		Results: []instanceMetadata{
			{Title: "test-agent", PermissionPrompt: pp},
		},
	}
	noPrompt := metadataResultMsg{
		Results: []instanceMetadata{
			{Title: "test-agent", PermissionPrompt: nil},
		},
	}

	// First tick — fires, guard set.
	_, _ = m.Update(withPrompt)

	// Prompt clears — guard should be removed.
	_, _ = m.Update(noPrompt)

	// New prompt (e.g. a second permission request later) — should fire again.
	_, cmd3 := m.Update(withPrompt)
	approvals := collectAutoApproveMsgs(cmd3)
	assert.Len(t, approvals, 1, "should fire again after guard is cleared")
}

// TestUpdate_PermissionAutoApprove_DifferentPromptBypassesGuard verifies that when
// a new permission prompt with a different cache key appears before the pane clears,
// the dedup guard does not block auto-approval of the new prompt.
func TestUpdate_PermissionAutoApprove_DifferentPromptBypassesGuard(t *testing.T) {
	m := newTestHomeWithCache(t)
	m.permissionStore.Remember(m.activeProject(), "Bash: ls -la")
	m.permissionStore.Remember(m.activeProject(), "Bash: git status")

	inst := &session.Instance{Title: "test-agent", Program: "claude"}
	inst.MarkStartedForTest()
	m.nav.AddInstance(inst)()

	ppA := &session.PermissionPrompt{Pattern: "Bash: ls -la", Description: "Bash: ls -la"}
	ppB := &session.PermissionPrompt{Pattern: "Bash: git status", Description: "Bash: git status"}

	tickA := metadataResultMsg{
		Results: []instanceMetadata{
			{Title: "test-agent", PermissionPrompt: ppA},
		},
	}
	tickB := metadataResultMsg{
		Results: []instanceMetadata{
			{Title: "test-agent", PermissionPrompt: ppB},
		},
	}

	// First tick — prompt A fires, guard set.
	_, cmd1 := m.Update(tickA)
	assert.Len(t, collectAutoApproveMsgs(cmd1), 1, "prompt A should auto-approve")

	// Second tick — prompt B appears (no nil gap). Guard key differs so it must fire.
	_, cmd2 := m.Update(tickB)
	assert.Len(t, collectAutoApproveMsgs(cmd2), 1,
		"prompt B should auto-approve even without a nil gap, because the cache key differs")
}

// TestHandleKeyPress_PermissionEnter_SendsResponse verifies that pressing Enter while
// in statePermission triggers a SendPermissionResponse cmd and returns to stateDefault.
func TestHandleKeyPress_PermissionEnter_SendsResponse(t *testing.T) {
	m := newTestHomeWithCache(t)
	inst := &session.Instance{Title: "test-agent", Program: "opencode"}
	inst.MarkStartedForTest()
	m.nav.AddInstance(inst)()

	// Set up statePermission (overlay open, instance pending)
	m.state = statePermission
	m.overlays.Show(overlay.NewPermissionOverlay(inst.Title, "Access /opt", "/opt/*"))
	m.pendingPermissionInstance = inst

	// Enter confirms with current selection (default: allow always)
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

	assert.Equal(t, stateDefault, m.state, "enter should return to stateDefault")
	assert.NotNil(t, cmd, "enter should return a permission response cmd")
}

func TestHandleKeyPress_PermissionEnter_DaemonPlaceholderRoutesResponse(t *testing.T) {
	m := newTestHomeWithCache(t)
	m.taskStoreProject = "myproj"
	inst, err := session.NewInstance(session.InstanceOptions{
		Title:         "sdk-agent",
		Path:          t.TempDir(),
		Program:       "codex",
		ExecutionMode: session.ExecutionModeSDK,
	})
	require.NoError(t, err)
	m.nav.AddInstance(inst)()

	m.state = statePermission
	m.overlays.Show(overlay.NewPermissionOverlay(inst.Title, "Access /opt", "/opt/*"))
	m.pendingPermissionInstance = inst
	m.pendingPermissionPattern = "/opt/*"
	m.pendingPermissionDesc = "Access /opt"

	var (
		gotProject string
		gotTitle   string
		gotChoice  tmux.PermissionChoice
	)
	origClient := newDaemonActionClient
	newDaemonActionClient = func() daemonActionClient {
		return &stubDaemonActionClient{
			sendPermissionResponseFunc: func(project, title string, choice tmux.PermissionChoice) error {
				gotProject = project
				gotTitle = title
				gotChoice = choice
				return nil
			},
		}
	}
	t.Cleanup(func() { newDaemonActionClient = origClient })

	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.NotNil(t, cmd)
	assert.Equal(t, stateDefault, m.state)
	require.Nil(t, cmd())
	assert.Equal(t, "myproj", gotProject)
	assert.Equal(t, "sdk-agent", gotTitle)
	assert.Equal(t, tmux.PermissionAllowAlways, gotChoice)
}

type stubDaemonActionClient struct {
	sendPromptFunc             func(project, title, prompt string) error
	killFunc                   func(project, title string) error
	sendPermissionResponseFunc func(project, title string, choice tmux.PermissionChoice) error
}

func (s *stubDaemonActionClient) SendInstancePrompt(project, title, prompt string) error {
	if s.sendPromptFunc != nil {
		return s.sendPromptFunc(project, title, prompt)
	}
	return nil
}

func (s *stubDaemonActionClient) KillInstance(project, title string) error {
	if s.killFunc != nil {
		return s.killFunc(project, title)
	}
	return nil
}

func (s *stubDaemonActionClient) SendInstancePermissionResponse(project, title string, choice tmux.PermissionChoice) error {
	if s.sendPermissionResponseFunc != nil {
		return s.sendPermissionResponseFunc(project, title, choice)
	}
	return nil
}

func TestDaemonRouteSend_LoadingPlaceholderRetriesStatusRace(t *testing.T) {
	origClient := newDaemonActionClient
	t.Cleanup(func() { newDaemonActionClient = origClient })

	attempts := 0
	newDaemonActionClient = func() daemonActionClient {
		return &stubDaemonActionClient{
			sendPromptFunc: func(project, title, prompt string) error {
				attempts++
				if attempts == 1 {
					return &daemonpkg.ClientStatusError{
						Method:     http.MethodPost,
						Path:       "/v1/repos/myproj/instances/sdk-agent/send",
						StatusCode: http.StatusNotFound,
						Message:    "not found",
					}
				}
				return nil
			},
		}
	}

	inst, err := session.NewInstance(session.InstanceOptions{
		Title:         "sdk-agent",
		Path:          t.TempDir(),
		Program:       "claude",
		ExecutionMode: session.ExecutionModeSDK,
	})
	require.NoError(t, err)
	inst.SetStatus(session.Loading)

	m := &home{taskStoreProject: "myproj"}
	handled, err := m.daemonRouteSend(inst, "hello")
	require.True(t, handled)
	require.NoError(t, err)
	assert.Equal(t, 2, attempts, "daemonRouteSend should retry once the daemon registers the loading placeholder")
}

// TestHandleKeyPress_PermissionEsc_DismissesWithoutSending verifies that Esc closes
// the modal without sending any keys to the tmux pane.
func TestHandleKeyPress_PermissionEsc_DismissesWithoutSending(t *testing.T) {
	m := newTestHomeWithCache(t)
	inst := &session.Instance{Title: "test-agent", Program: "opencode"}
	inst.MarkStartedForTest()
	m.nav.AddInstance(inst)()

	m.state = statePermission
	m.overlays.Show(overlay.NewPermissionOverlay(inst.Title, "Access /opt", "/opt/*"))
	m.pendingPermissionInstance = inst

	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})

	assert.Equal(t, stateDefault, m.state, "esc should return to stateDefault")
	assert.False(t, m.overlays.IsActive(), "overlay should be cleared")
	assert.Nil(t, m.pendingPermissionInstance, "pending instance should be cleared")
	// Esc must not send any permission response (nil cmd is fine; no auto-approve)
	approvals := collectAutoApproveMsgs(cmd)
	assert.Len(t, approvals, 0, "esc should not trigger any auto-approve")
}

func TestHandleMouseClick_PermissionOverlay_InsideClickSendsResponse(t *testing.T) {
	m := newTestHomeWithCache(t)
	inst := &session.Instance{Title: "test-agent", Program: "opencode"}
	inst.MarkStartedForTest()
	m.nav.AddInstance(inst)()

	po := overlay.NewPermissionOverlay(inst.Title, "Access /opt", "/opt/*")
	m.state = statePermission
	m.overlays.ShowPositioned(po, 0, 0, false)
	m.pendingPermissionInstance = inst
	m.pendingPermissionPattern = "/opt/*"
	m.pendingPermissionDesc = "Access /opt"

	x, y := permissionOverlayClickTarget(t, po.View(), "allow once")
	_, cmd := m.Update(tea.MouseClickMsg{X: x, Y: y, Button: tea.MouseLeft})

	assert.Equal(t, stateDefault, m.state)
	assert.False(t, m.overlays.IsActive())
	assert.Nil(t, m.pendingPermissionInstance)
	assert.Empty(t, m.pendingPermissionPattern)
	assert.Empty(t, m.pendingPermissionDesc)
	assert.NotNil(t, cmd)
}

func TestHandleMouseClick_PermissionOverlay_OutsideClickDismissesWithoutSending(t *testing.T) {
	m := newTestHomeWithCache(t)
	inst := &session.Instance{Title: "test-agent", Program: "opencode"}
	inst.MarkStartedForTest()
	m.nav.AddInstance(inst)()

	po := overlay.NewPermissionOverlay(inst.Title, "Access /opt", "/opt/*")
	m.state = statePermission
	m.overlays.ShowPositioned(po, 5, 5, false)
	m.pendingPermissionInstance = inst
	m.pendingPermissionPattern = "/opt/*"
	m.pendingPermissionDesc = "Access /opt"

	_, cmd := m.Update(tea.MouseClickMsg{X: 0, Y: 0, Button: tea.MouseLeft})

	assert.Equal(t, stateDefault, m.state)
	assert.False(t, m.overlays.IsActive())
	assert.Nil(t, m.pendingPermissionInstance)
	assert.Empty(t, m.pendingPermissionPattern)
	assert.Empty(t, m.pendingPermissionDesc)
	approvals := collectAutoApproveMsgs(cmd)
	assert.Len(t, approvals, 0)
}

// TestPermissionOverlay_PatternExposed verifies the overlay exposes its pattern
// so app_input.go can read it on confirm without re-parsing CachedContent.
func TestPermissionOverlay_PatternExposed(t *testing.T) {
	po := overlay.NewPermissionOverlay("test", "Access /opt", "/opt/*")
	assert.Equal(t, "/opt/*", po.Pattern())
}

// --- Legacy unit tests (kept for overlay component coverage) ---

func TestPermissionDetection_ShowsOverlayForOpenCode(t *testing.T) {
	m := newTestHomeWithCache(t)
	inst := &session.Instance{
		Title:   "test-agent",
		Program: "opencode",
	}
	inst.MarkStartedForTest()
	m.nav.AddInstance(inst)()
	m.nav.SetSelectedInstance(0)

	// Simulate metadata tick with permission prompt detected
	inst.CachedContent = "△ Permission required\n  ← Access external directory /opt\n\nPatterns\n\n- /opt/*\n\n Allow once   Allow always   Reject\n"
	inst.CachedContentSet = true

	pp := session.ParsePermissionPrompt(inst.CachedContent, inst.Program)
	assert.NotNil(t, pp)

	// Simulate the detection path
	m.overlays.Show(overlay.NewPermissionOverlay(inst.Title, pp.Description, pp.Pattern))
	m.pendingPermissionInstance = inst
	m.state = statePermission

	assert.Equal(t, statePermission, m.state)
	assert.True(t, m.overlays.IsActive(), "permission overlay must be active")
}

func TestPermissionOverlay_ArrowKeysNavigate(t *testing.T) {
	po := overlay.NewPermissionOverlay("test", "Access /opt", "/opt/*")

	// Default is "allow always" (index 1)
	assert.Equal(t, overlay.PermissionAllowAlways, po.Choice())

	// Right → "reject"
	po.HandleKey(tea.KeyPressMsg{Code: tea.KeyRight})
	assert.Equal(t, overlay.PermissionReject, po.Choice())

	// Right at end → stays on "reject"
	po.HandleKey(tea.KeyPressMsg{Code: tea.KeyRight})
	assert.Equal(t, overlay.PermissionReject, po.Choice())

	// Left → back to "allow always"
	po.HandleKey(tea.KeyPressMsg{Code: tea.KeyLeft})
	assert.Equal(t, overlay.PermissionAllowAlways, po.Choice())

	// Left → "allow once"
	po.HandleKey(tea.KeyPressMsg{Code: tea.KeyLeft})
	assert.Equal(t, overlay.PermissionAllowOnce, po.Choice())
}

func TestPermissionOverlay_EnterConfirms(t *testing.T) {
	po := overlay.NewPermissionOverlay("test", "Access /opt", "/opt/*")
	result := po.HandleKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	assert.True(t, result.Dismissed)
	assert.True(t, result.Submitted)
	assert.Equal(t, overlay.PermissionAllowAlways, po.Choice()) // default is "allow always"
}

func TestPermissionOverlay_EscDismisses(t *testing.T) {
	po := overlay.NewPermissionOverlay("test", "Access /opt", "/opt/*")
	result := po.HandleKey(tea.KeyPressMsg{Code: tea.KeyEscape})
	assert.True(t, result.Dismissed)
	assert.False(t, result.Submitted)
}

func TestPermissionCache_AutoApprovesCachedPattern(t *testing.T) {
	m := newTestHomeWithCache(t)
	m.permissionStore.Remember("test-project", "/opt/*")
	assert.True(t, m.permissionStore.IsAllowedAlways("test-project", "/opt/*"))
}

// TestUpdate_PermissionAutoApprove_DescriptionOnly verifies that prompts without
// a Pattern (e.g. bash command permissions) still auto-approve when the description
// has been cached via "allow always".
func TestUpdate_PermissionAutoApprove_DescriptionOnly(t *testing.T) {
	m := newTestHomeWithCache(t)
	// Cache by description (no pattern).
	m.permissionStore.Remember(m.activeProject(), "Execute bash command")

	inst := &session.Instance{Title: "test-agent", Program: "opencode"}
	inst.MarkStartedForTest()
	m.nav.AddInstance(inst)()

	pp := &session.PermissionPrompt{Pattern: "", Description: "Execute bash command"}
	msg := metadataResultMsg{
		Results: []instanceMetadata{
			{Title: "test-agent", PermissionPrompt: pp},
		},
	}

	_, cmd := m.Update(msg)
	approvals := collectAutoApproveMsgs(cmd)

	assert.Len(t, approvals, 1, "description-only prompt should auto-approve when cached")
	assert.Equal(t, stateDefault, m.state, "auto-approve should not change state")
}

// TestUpdate_PermissionPrompt_DefersInFocusMode verifies that a permission
// prompt is deferred (not shown) while the user is in focus/interactive mode.
func TestUpdate_PermissionPrompt_DefersInFocusMode(t *testing.T) {
	m := newTestHomeWithCache(t)
	inst := &session.Instance{Title: "test-agent", Program: "opencode"}
	inst.MarkStartedForTest()
	m.nav.AddInstance(inst)()

	// Start in focus mode
	m.state = stateFocusAgent
	m.tabbedWindow.SetFocusMode(true)
	m.menu.SetFocusMode(true)

	pp := &session.PermissionPrompt{Pattern: "/opt/*", Description: "Access /opt"}
	msg := metadataResultMsg{
		Results: []instanceMetadata{
			{Title: "test-agent", PermissionPrompt: pp},
		},
	}

	_, _ = m.Update(msg)

	assert.Equal(t, stateFocusAgent, m.state, "focus mode must not be interrupted")
	assert.True(t, m.tabbedWindow.IsFocusMode(), "tabbed window should remain in focus mode")
	assert.Len(t, m.deferredPermissionPrompts, 1, "permission should be queued for later")
}

// TestUpdate_PermissionPrompt_FocusesInstanceBeforeOverlay verifies that when a
// permission prompt is detected for a non-selected instance, that instance becomes
// selected before the overlay is shown — so the user sees the agent output behind it.
func TestUpdate_PermissionPrompt_FocusesInstanceBeforeOverlay(t *testing.T) {
	m := newTestHomeWithCache(t)

	// Add two instances; select the first one.
	inst1 := &session.Instance{Title: "agent-1", Program: "opencode"}
	inst1.MarkStartedForTest()
	m.nav.AddInstance(inst1)()

	inst2 := &session.Instance{Title: "agent-2", Program: "opencode"}
	inst2.MarkStartedForTest()
	m.nav.AddInstance(inst2)()

	m.nav.SetSelectedInstance(0)
	assert.Equal(t, inst1, m.nav.GetSelectedInstance(), "precondition: agent-1 selected")

	// Permission prompt fires on agent-2 (not the selected one).
	pp := &session.PermissionPrompt{Pattern: "/opt/*", Description: "Access /opt"}
	msg := metadataResultMsg{
		Results: []instanceMetadata{
			{Title: "agent-1"},
			{Title: "agent-2", PermissionPrompt: pp},
		},
	}

	_, _ = m.Update(msg)

	assert.Equal(t, statePermission, m.state)
	// The nav selection should have switched to agent-2.
	assert.Equal(t, inst2, m.nav.GetSelectedInstance(),
		"permission overlay should auto-focus the instance that triggered it")
}

// --- Focus-restoration tests ---

// TestFinishPermission_RestoresFocus_AfterAllowOnce verifies that when a permission
// prompt fires on a non-selected instance (switching nav focus to it), dismissing
// the overlay by choosing "allow once" restores the original selection.
func TestFinishPermission_RestoresFocus_AfterAllowOnce(t *testing.T) {
	m := newTestHomeWithCache(t)

	inst1 := &session.Instance{Title: "agent-1", Program: "opencode"}
	inst1.MarkStartedForTest()
	m.nav.AddInstance(inst1)()

	inst2 := &session.Instance{Title: "agent-2", Program: "opencode"}
	inst2.MarkStartedForTest()
	m.nav.AddInstance(inst2)()

	// inst1 is the original selection.
	m.nav.SelectInstance(inst1)
	require.Equal(t, inst1, m.nav.GetSelectedInstance(), "precondition: agent-1 selected")
	wantID := m.nav.GetSelectedID()

	// Permission fires on inst2 — the detection path saves inst1 and focuses inst2.
	pp := &session.PermissionPrompt{Pattern: "/opt/*", Description: "Access /opt"}
	_, _ = m.Update(metadataResultMsg{
		Results: []instanceMetadata{
			{Title: "agent-1"},
			{Title: "agent-2", PermissionPrompt: pp},
		},
	})

	require.Equal(t, statePermission, m.state)
	requirePreOverlaySaved(t, m, wantID)
	require.Equal(t, inst2, m.nav.GetSelectedInstance(), "overlay focused inst2")

	// Navigate to "allow once" (default is "allow always"; press ← once).
	_, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyLeft})
	// Confirm.
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd != nil {
		cmd() // drain SendPermissionResponse
	}

	assert.Equal(t, stateDefault, m.state)
	assert.Equal(t, inst1, m.nav.GetSelectedInstance(), "original instance should be restored")
	assertPreOverlayCleared(t, m)
}

// TestFinishPermission_RestoresFocus_AfterEscape verifies that dismissing a permission
// overlay via Escape also restores the original nav selection.
func TestFinishPermission_RestoresFocus_AfterEscape(t *testing.T) {
	m := newTestHomeWithCache(t)

	inst1 := &session.Instance{Title: "agent-1", Program: "opencode"}
	inst1.MarkStartedForTest()
	m.nav.AddInstance(inst1)()

	inst2 := &session.Instance{Title: "agent-2", Program: "opencode"}
	inst2.MarkStartedForTest()
	m.nav.AddInstance(inst2)()

	m.nav.SelectInstance(inst1)
	require.Equal(t, inst1, m.nav.GetSelectedInstance(), "precondition: agent-1 selected")
	wantID := m.nav.GetSelectedID()

	// Trigger permission on inst2.
	pp := &session.PermissionPrompt{Pattern: "/opt/*", Description: "Access /opt"}
	_, _ = m.Update(metadataResultMsg{
		Results: []instanceMetadata{
			{Title: "agent-1"},
			{Title: "agent-2", PermissionPrompt: pp},
		},
	})

	require.Equal(t, statePermission, m.state)
	requirePreOverlaySaved(t, m, wantID)

	// Dismiss with Escape (no response sent).
	_, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})

	assert.Equal(t, stateDefault, m.state)
	assert.Equal(t, inst1, m.nav.GetSelectedInstance(), "original instance should be restored after escape")
	assertPreOverlayCleared(t, m)
}

// TestFinishPermission_RestoresFocus_QueuedPrompts verifies that when two permission
// prompts are queued (e.g., after leaving focus mode), responding to each in turn
// preserves the very first nav selection — not the intermediate one — and only
// restores once the last prompt is dismissed.
func TestFinishPermission_RestoresFocus_QueuedPrompts(t *testing.T) {
	m := newTestHomeWithCache(t)

	inst1 := &session.Instance{Title: "agent-1", Program: "opencode"}
	inst1.MarkStartedForTest()
	m.nav.AddInstance(inst1)()

	inst2 := &session.Instance{Title: "agent-2", Program: "opencode"}
	inst2.MarkStartedForTest()
	m.nav.AddInstance(inst2)()

	inst3 := &session.Instance{Title: "agent-3", Program: "opencode"}
	inst3.MarkStartedForTest()
	m.nav.AddInstance(inst3)()

	// inst1 is the original selection.
	m.nav.SelectInstance(inst1)
	require.Equal(t, inst1, m.nav.GetSelectedInstance(), "precondition: agent-1 selected")
	wantID := m.nav.GetSelectedID()

	// Simulate two deferred prompts arriving while in focus mode (one for each of inst2/inst3).
	// showPermissionPrompt's first-write-wins save captures inst1's row id on
	// the first call and must preserve it across subsequent prompts.
	m.deferredPermissionPrompts = []deferredPermissionPrompt{
		{instance: inst3, pattern: "/usr/*", desc: "Access /usr"},
	}

	// Show the first deferred prompt (inst2) via showPermissionPrompt, which also
	// applies the first-write-wins save.
	deferred2 := deferredPermissionPrompt{instance: inst2, pattern: "/opt/*", desc: "Access /opt"}
	_ = m.showPermissionPrompt(deferred2)

	requirePreOverlaySaved(t, m, wantID)
	require.Equal(t, statePermission, m.state)
	require.Len(t, m.deferredPermissionPrompts, 1, "inst3 still deferred")

	// Answer inst2 overlay — deferred queue still has inst3, so no restoration yet.
	_, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

	requirePreOverlaySaved(t, m, wantID)
	require.Equal(t, stateDefault, m.state)

	// Drain and show inst3's deferred prompt — first-write-wins must NOT overwrite the saved id.
	deferred3 := m.deferredPermissionPrompts[0]
	m.deferredPermissionPrompts = m.deferredPermissionPrompts[1:]
	_ = m.showPermissionPrompt(deferred3)

	requirePreOverlaySaved(t, m, wantID)
	require.Equal(t, statePermission, m.state)
	require.Empty(t, m.deferredPermissionPrompts, "queue now empty")

	// Answer inst3 — queue empty, must restore inst1.
	_, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

	assert.Equal(t, inst1, m.nav.GetSelectedInstance(), "original instance restored after all prompts answered")
	assertPreOverlayCleared(t, m)
}

// TestFinishPermission_NoRestore_WhenSameInstance verifies that when the permission
// prompt fires on the already-selected instance, no pre-overlay row is captured
// and no restoration occurs — the selection stays on that instance without
// triggering unnecessary instanceChanged() side effects.
func TestFinishPermission_NoRestore_WhenSameInstance(t *testing.T) {
	m := newTestHomeWithCache(t)

	inst1 := &session.Instance{Title: "agent-1", Program: "opencode"}
	inst1.MarkStartedForTest()
	m.nav.AddInstance(inst1)()

	// inst1 is both the original selection and the one with the permission prompt.
	m.nav.SelectInstance(inst1)
	require.Equal(t, inst1, m.nav.GetSelectedInstance(), "precondition: agent-1 selected")

	pp := &session.PermissionPrompt{Pattern: "/opt/*", Description: "Access /opt"}
	_, _ = m.Update(metadataResultMsg{
		Results: []instanceMetadata{
			{Title: "agent-1", PermissionPrompt: pp},
		},
	})

	require.Equal(t, statePermission, m.state)
	// Same-instance prompt: pre-overlay row must NOT be captured.
	require.False(t, m.preOverlayCaptured, "preOverlayCaptured should not be set for same-instance prompt")
	require.Empty(t, m.preOverlayNavID, "preOverlayNavID should remain empty")

	// Answer the overlay.
	_, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

	// Selection unchanged; no crash, no restoration path triggered.
	assert.Equal(t, stateDefault, m.state)
	assert.Equal(t, inst1, m.nav.GetSelectedInstance(), "selection should remain on inst1")
	assertPreOverlayCleared(t, m)
}

// TestFinishPermission_NoRestore_WhenSavedRowRemoved verifies that if the saved
// original row disappears from the nav before the overlay is dismissed, the
// restoration attempt fails gracefully (no panic, no stale preOverlay fields).
func TestFinishPermission_NoRestore_WhenSavedRowRemoved(t *testing.T) {
	m := newTestHomeWithCache(t)

	inst1 := &session.Instance{Title: "agent-1", Program: "opencode"}
	inst1.MarkStartedForTest()
	m.nav.AddInstance(inst1)()

	inst2 := &session.Instance{Title: "agent-2", Program: "opencode"}
	inst2.MarkStartedForTest()
	m.nav.AddInstance(inst2)()

	// Manually set up the overlay state with inst1's row captured as the original.
	m.nav.SelectInstance(inst1)
	m.preOverlayNavID = m.nav.GetSelectedID()
	m.preOverlayCaptured = true
	m.nav.SelectInstance(inst2)
	m.state = statePermission
	m.overlays.Show(overlay.NewPermissionOverlay(inst2.Title, "Access /opt", "/opt/*"))
	m.pendingPermissionInstance = inst2
	m.pendingPermissionPattern = "/opt/*"
	m.pendingPermissionDesc = "Access /opt"

	// Remove inst1 from the nav before the overlay is dismissed.
	m.nav.RemoveByTitle("agent-1")
	require.NotContains(t, m.nav.GetInstances(), inst1, "precondition: inst1 removed from nav")

	// Dismiss the overlay — SelectByID(<inst1 row>) will return false; must not panic.
	require.NotPanics(t, func() {
		_, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	})

	assert.Equal(t, stateDefault, m.state)
	assertPreOverlayCleared(t, m)
}

// TestFinishPermission_RestoresFocus_DeferredPrompt verifies the end-to-end deferred
// flow: permission prompt arrives while the user is in focus mode, the user exits
// focus mode, the prompt is shown, and on response the original selection is restored.
func TestFinishPermission_RestoresFocus_DeferredPrompt(t *testing.T) {
	m := newTestHomeWithCache(t)

	inst1 := &session.Instance{Title: "agent-1", Program: "opencode"}
	inst1.MarkStartedForTest()
	m.nav.AddInstance(inst1)()

	inst2 := &session.Instance{Title: "agent-2", Program: "opencode"}
	inst2.MarkStartedForTest()
	m.nav.AddInstance(inst2)()

	// Enter focus mode with inst1 selected.
	m.nav.SelectInstance(inst1)
	m.state = stateFocusAgent
	m.tabbedWindow.SetFocusMode(true)
	m.menu.SetFocusMode(true)
	require.Equal(t, inst1, m.nav.GetSelectedInstance(), "precondition: agent-1 selected in focus mode")
	wantID := m.nav.GetSelectedID()

	// A permission prompt arrives while in focus mode — it should be deferred.
	pp := &session.PermissionPrompt{Pattern: "/opt/*", Description: "Access /opt"}
	_, _ = m.Update(metadataResultMsg{
		Results: []instanceMetadata{
			{Title: "agent-1"},
			{Title: "agent-2", PermissionPrompt: pp},
		},
	})

	require.Equal(t, stateFocusAgent, m.state, "focus mode must not be interrupted")
	require.Len(t, m.deferredPermissionPrompts, 1, "prompt should be deferred")
	require.False(t, m.preOverlayCaptured, "preOverlay not yet captured (prompt still deferred)")

	// User exits focus mode.
	m.exitFocusMode()
	require.Equal(t, stateDefault, m.state)

	// Simulate drainDeferredDialogs showing the deferred prompt.
	deferred := m.deferredPermissionPrompts[0]
	m.deferredPermissionPrompts = m.deferredPermissionPrompts[1:]
	_ = m.showPermissionPrompt(deferred)

	require.Equal(t, statePermission, m.state)
	requirePreOverlaySaved(t, m, wantID)
	require.Equal(t, inst2, m.nav.GetSelectedInstance(), "nav should focus inst2 for overlay context")

	// Respond to the prompt.
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd != nil {
		cmd()
	}

	assert.Equal(t, stateDefault, m.state)
	assert.Equal(t, inst1, m.nav.GetSelectedInstance(), "original selection restored after deferred prompt answered")
	assertPreOverlayCleared(t, m)
}

// TestUpdate_DeferredPromptClearedBeforeDisplay verifies that when a deferred
// permission prompt is resolved externally (PermissionPrompt becomes nil) before
// it is shown, the stale entry is removed from the deferred queue. This ensures
// that the last real prompt still restores the original selection and that
// drainDeferredDialogs does not surface a bogus overlay.
func TestUpdate_DeferredPromptClearedBeforeDisplay(t *testing.T) {
	m := newTestHomeWithCache(t)

	inst1 := &session.Instance{Title: "agent-1", Program: "opencode"}
	inst1.MarkStartedForTest()
	m.nav.AddInstance(inst1)()

	inst2 := &session.Instance{Title: "agent-2", Program: "opencode"}
	inst2.MarkStartedForTest()
	m.nav.AddInstance(inst2)()

	inst3 := &session.Instance{Title: "agent-3", Program: "opencode"}
	inst3.MarkStartedForTest()
	m.nav.AddInstance(inst3)()

	// inst1 is the original selection.
	m.nav.SelectInstance(inst1)
	require.Equal(t, inst1, m.nav.GetSelectedInstance(), "precondition: agent-1 selected")
	wantID := m.nav.GetSelectedID()

	// Enter focus mode so prompts are deferred.
	m.state = stateFocusAgent
	m.tabbedWindow.SetFocusMode(true)
	m.menu.SetFocusMode(true)

	// Two permission prompts arrive while in focus mode.
	pp2 := &session.PermissionPrompt{Pattern: "/opt/*", Description: "Access /opt"}
	pp3 := &session.PermissionPrompt{Pattern: "/usr/*", Description: "Access /usr"}

	_, _ = m.Update(metadataResultMsg{
		Results: []instanceMetadata{
			{Title: "agent-1"},
			{Title: "agent-2", PermissionPrompt: pp2},
			{Title: "agent-3", PermissionPrompt: pp3},
		},
	})

	require.Equal(t, stateFocusAgent, m.state, "focus mode must not be interrupted")
	require.Len(t, m.deferredPermissionPrompts, 2, "both prompts should be deferred")

	// inst2's permission clears externally before the user leaves focus mode.
	_, _ = m.Update(metadataResultMsg{
		Results: []instanceMetadata{
			{Title: "agent-1"},
			{Title: "agent-2", PermissionPrompt: nil},
			{Title: "agent-3", PermissionPrompt: pp3},
		},
	})

	require.Len(t, m.deferredPermissionPrompts, 1, "stale inst2 entry must be removed from deferred queue")
	assert.Equal(t, inst3, m.deferredPermissionPrompts[0].instance, "remaining entry should be for inst3")

	// Exit focus mode.
	m.exitFocusMode()
	require.Equal(t, stateDefault, m.state)

	// Drain the remaining deferred prompt (inst3).
	deferred := m.deferredPermissionPrompts[0]
	m.deferredPermissionPrompts = m.deferredPermissionPrompts[1:]
	_ = m.showPermissionPrompt(deferred)

	require.Equal(t, statePermission, m.state)
	requirePreOverlaySaved(t, m, wantID)
	require.Equal(t, inst3, m.nav.GetSelectedInstance(), "nav focused inst3 for overlay")

	// Answer the prompt.
	_, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

	assert.Equal(t, stateDefault, m.state)
	assert.Equal(t, inst1, m.nav.GetSelectedInstance(), "original selection restored after last real prompt")
	assertPreOverlayCleared(t, m)
}

// TestUpdate_DeferredPromptClearedBeforeDisplay_NewPromptRestoresFocus is a
// regression test for the clearDeferredPermissionPrompt path. It exercises the
// scenario where every deferred entry is cleared externally (the queue drains to
// empty), focus mode ends with nothing to show, and then a brand-new prompt from
// a third instance arrives. The original selection must still be restored after
// that new prompt is answered. Without clearDeferredPermissionPrompt, the stale
// deferred entry would have been drained first, showing a bogus overlay that
// would corrupt the preOverlay fields and break focus restoration.
func TestUpdate_DeferredPromptClearedBeforeDisplay_NewPromptRestoresFocus(t *testing.T) {
	m := newTestHomeWithCache(t)

	inst1 := &session.Instance{Title: "agent-1", Program: "opencode"}
	inst1.MarkStartedForTest()
	m.nav.AddInstance(inst1)()

	inst2 := &session.Instance{Title: "agent-2", Program: "opencode"}
	inst2.MarkStartedForTest()
	m.nav.AddInstance(inst2)()

	inst3 := &session.Instance{Title: "agent-3", Program: "opencode"}
	inst3.MarkStartedForTest()
	m.nav.AddInstance(inst3)()

	// inst1 is the original selection.
	m.nav.SelectInstance(inst1)
	require.Equal(t, inst1, m.nav.GetSelectedInstance(), "precondition: agent-1 selected")
	wantID := m.nav.GetSelectedID()

	// Enter focus mode so prompts are deferred.
	m.state = stateFocusAgent
	m.tabbedWindow.SetFocusMode(true)
	m.menu.SetFocusMode(true)

	// inst2 gets a permission prompt while in focus mode — added to deferred queue.
	pp2 := &session.PermissionPrompt{Pattern: "/opt/*", Description: "Access /opt"}
	_, _ = m.Update(metadataResultMsg{
		Results: []instanceMetadata{
			{Title: "agent-1"},
			{Title: "agent-2", PermissionPrompt: pp2},
			{Title: "agent-3"},
		},
	})

	require.Equal(t, stateFocusAgent, m.state, "focus mode must not be interrupted")
	require.Len(t, m.deferredPermissionPrompts, 1, "inst2 prompt should be deferred")

	// inst2's permission clears externally before the user exits focus mode.
	// This exercises clearDeferredPermissionPrompt via the metadata-clear path.
	_, _ = m.Update(metadataResultMsg{
		Results: []instanceMetadata{
			{Title: "agent-1"},
			{Title: "agent-2", PermissionPrompt: nil},
			{Title: "agent-3"},
		},
	})

	require.Empty(t, m.deferredPermissionPrompts, "stale inst2 entry must be removed — queue now empty")

	// Exit focus mode — no deferred prompts to drain.
	m.exitFocusMode()
	require.Equal(t, stateDefault, m.state)
	assertPreOverlayCleared(t, m)
	assert.Equal(t, inst1, m.nav.GetSelectedInstance(), "inst1 still selected after exiting focus mode")

	// A new permission prompt arrives from inst3 (not deferred — we're in default state).
	pp3 := &session.PermissionPrompt{Pattern: "/usr/*", Description: "Access /usr"}
	_, _ = m.Update(metadataResultMsg{
		Results: []instanceMetadata{
			{Title: "agent-1"},
			{Title: "agent-2"},
			{Title: "agent-3", PermissionPrompt: pp3},
		},
	})

	require.Equal(t, statePermission, m.state, "permission overlay shown for inst3")
	requirePreOverlaySaved(t, m, wantID)
	require.Equal(t, inst3, m.nav.GetSelectedInstance(), "nav switched to inst3 for overlay")

	// Answer the prompt.
	_, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

	assert.Equal(t, stateDefault, m.state)
	assert.Equal(t, inst1, m.nav.GetSelectedInstance(), "original inst1 selection restored after answering inst3")
	assertPreOverlayCleared(t, m)
}

// TestFinishPermission_RestoresFocus_NonInstanceRow_Immediate verifies that when
// the user is on a non-instance nav row (a plan header with no attached agent)
// and a permission prompt arrives via the immediate detection path, dismissing
// the overlay restores selection to that plan header row — not to the prompted
// instance. This exercises the GetSelectedID / SelectByID path which handles
// non-instance rows where GetSelectedInstance() returns nil.
func TestFinishPermission_RestoresFocus_NonInstanceRow_Immediate(t *testing.T) {
	m := newTestHomeWithCache(t)

	// Seed a persistent plan-state entry so the plan header survives the
	// updateSidebarTasks() rebuild that runs at the end of the metadata tick.
	planFile := "plan-alpha"
	m.setupPlanState(t, planFile, taskstate.StatusReady, "")

	// Add the instance whose permission prompt will fire. It has no TaskFile,
	// so it renders under the solo "agents" header.
	inst2 := &session.Instance{Title: "agent-2", Program: "opencode"}
	inst2.MarkStartedForTest()
	m.nav.AddInstance(inst2)()

	// Select the plan header — GetSelectedInstance() is nil, but the row id is set.
	require.True(t, m.nav.SelectByID(ui.SidebarPlanPrefix+planFile), "plan header must be selectable")
	require.Nil(t, m.nav.GetSelectedInstance(), "precondition: plan header selected, no instance")
	wantID := m.nav.GetSelectedID()
	require.NotEmpty(t, wantID, "plan header must expose a stable row id")

	// Permission fires on inst2 — the detection path must capture the plan row
	// id via GetSelectedID (not the nil instance pointer) and focus inst2.
	pp := &session.PermissionPrompt{Pattern: "/opt/*", Description: "Access /opt"}
	_, _ = m.Update(metadataResultMsg{
		Results: []instanceMetadata{
			{Title: "agent-2", PermissionPrompt: pp},
		},
	})

	require.Equal(t, statePermission, m.state)
	requirePreOverlaySaved(t, m, wantID)
	require.Equal(t, inst2, m.nav.GetSelectedInstance(), "overlay focused inst2")

	// Answer the overlay (allow once).
	_, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyLeft})
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd != nil {
		cmd()
	}

	// Plan header row must be restored — not the prompted instance row.
	assert.Equal(t, stateDefault, m.state)
	assert.Equal(t, wantID, m.nav.GetSelectedID(), "plan header row id should be restored")
	assert.Nil(t, m.nav.GetSelectedInstance(), "selected row should still be the plan header (no instance)")
	assertPreOverlayCleared(t, m)
}

// TestFinishPermission_RestoresFocus_NonInstanceRow_Deferred verifies the same
// guarantee for the deferred path: a permission prompt that arrives while the
// user is in focus mode on a non-instance nav row must, once drained and
// answered, restore the original plan-header selection.
func TestFinishPermission_RestoresFocus_NonInstanceRow_Deferred(t *testing.T) {
	m := newTestHomeWithCache(t)

	// Seed a persistent plan-state entry so the plan header survives the
	// updateSidebarTasks() rebuild that runs at the end of the metadata tick.
	planFile := "plan-beta"
	m.setupPlanState(t, planFile, taskstate.StatusReady, "")

	// Add the instance whose deferred permission prompt will fire.
	inst2 := &session.Instance{Title: "agent-2", Program: "opencode"}
	inst2.MarkStartedForTest()
	m.nav.AddInstance(inst2)()

	// Select the plan header first — pre-overlay row is a non-instance row.
	require.True(t, m.nav.SelectByID(ui.SidebarPlanPrefix+planFile), "plan header must be selectable")
	require.Nil(t, m.nav.GetSelectedInstance(), "precondition: plan header selected")
	wantID := m.nav.GetSelectedID()
	require.NotEmpty(t, wantID)

	// Enter focus mode — any permission prompts must be deferred.
	m.state = stateFocusAgent
	m.tabbedWindow.SetFocusMode(true)
	m.menu.SetFocusMode(true)

	// A permission prompt arrives while in focus mode — it should be deferred.
	pp := &session.PermissionPrompt{Pattern: "/opt/*", Description: "Access /opt"}
	_, _ = m.Update(metadataResultMsg{
		Results: []instanceMetadata{
			{Title: "agent-2", PermissionPrompt: pp},
		},
	})

	require.Equal(t, stateFocusAgent, m.state, "focus mode must not be interrupted")
	require.Len(t, m.deferredPermissionPrompts, 1, "prompt should be deferred")
	require.False(t, m.preOverlayCaptured, "preOverlay not yet captured (prompt still deferred)")

	// User exits focus mode — the plan header is still selected because focus
	// mode did not change the nav row.
	m.exitFocusMode()
	require.Equal(t, stateDefault, m.state)
	require.Equal(t, wantID, m.nav.GetSelectedID(), "plan header still selected after exiting focus mode")

	// Drain and show the deferred prompt via showPermissionPrompt, which must
	// capture the plan header row id — not nil — before focusing inst2.
	deferred := m.deferredPermissionPrompts[0]
	m.deferredPermissionPrompts = m.deferredPermissionPrompts[1:]
	_ = m.showPermissionPrompt(deferred)

	require.Equal(t, statePermission, m.state)
	requirePreOverlaySaved(t, m, wantID)
	require.Equal(t, inst2, m.nav.GetSelectedInstance(), "nav focused inst2 for overlay context")

	// Respond to the prompt.
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd != nil {
		cmd()
	}

	// Plan header row must be restored — not the prompted instance row.
	assert.Equal(t, stateDefault, m.state)
	assert.Equal(t, wantID, m.nav.GetSelectedID(), "plan header row id restored after deferred prompt answered")
	assert.Nil(t, m.nav.GetSelectedInstance(), "selected row should still be the plan header")
	assertPreOverlayCleared(t, m)
}
