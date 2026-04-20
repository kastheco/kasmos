package ui

import (
	"fmt"
	"github.com/kastheco/kasmos/cmd/cmd_test"
	"github.com/kastheco/kasmos/log"
	"github.com/kastheco/kasmos/session"
	"github.com/kastheco/kasmos/session/sdk"
	"github.com/kastheco/kasmos/session/tmux"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/stretchr/testify/require"
)

// testSetup holds common test setup data
type testSetup struct {
	workdir     string
	instance    *session.Instance
	sessionName string
	cleanupFn   func()
}

// setupTestEnvironment creates a common test environment with git repo and instance
func setupTestEnvironment(t *testing.T, cmdExec cmd_test.MockCmdExec) *testSetup {
	t.Helper()

	// Initialize logging
	log.Initialize(false)

	// Set up a temp working directory
	workdir := t.TempDir()

	// Initialize git repository
	setupGitRepo(t, workdir)

	// Create unique session name
	random := time.Now().UnixNano() % 10000000
	sessionName := fmt.Sprintf("test-preview-%s-%d-%d", t.Name(), time.Now().UnixNano(), random)

	// Create instance
	instance, err := session.NewInstance(session.InstanceOptions{
		Title:   sessionName,
		Path:    workdir,
		Program: "bash",
		AutoYes: false,
	})
	require.NoError(t, err)

	// Create MockPtyFactory
	ptyFactory := &MockPtyFactory{
		t:       t,
		cmdExec: cmdExec,
	}

	// Set up tmux session with mocks
	tmuxSession := tmux.NewTmuxSessionWithDeps(sessionName, "bash", false, ptyFactory, cmdExec)
	instance.SetTmuxSession(tmuxSession)

	// Start the tmux session
	err = instance.Start(true)
	require.NoError(t, err)

	// Create cleanup function
	cleanupFn := func() {
		if instance != nil {
			_ = instance.Kill() // Ignore errors during cleanup
		}
		log.Close()
	}

	return &testSetup{
		workdir:     workdir,
		instance:    instance,
		sessionName: sessionName,
		cleanupFn:   cleanupFn,
	}
}

// setupGitRepo initializes a git repository in the given directory
func setupGitRepo(t *testing.T, workdir string) {
	t.Helper()

	// Initialize git repository
	initCmd := exec.Command("git", "init")
	initCmd.Dir = workdir
	err := initCmd.Run()
	require.NoError(t, err)

	// Create basic git config (local to this repo only)
	configCmd := exec.Command("git", "config", "--local", "user.email", "test@example.com")
	configCmd.Dir = workdir
	err = configCmd.Run()
	require.NoError(t, err)

	configCmd = exec.Command("git", "config", "--local", "user.name", "Test User")
	configCmd.Dir = workdir
	err = configCmd.Run()
	require.NoError(t, err)

	// Create and commit a test file
	testFile := filepath.Join(workdir, "test.txt")
	err = os.WriteFile(testFile, []byte("test content"), 0644)
	require.NoError(t, err)

	addCmd := exec.Command("git", "add", "test.txt")
	addCmd.Dir = workdir
	err = addCmd.Run()
	require.NoError(t, err)

	commitCmd := exec.Command("git", "commit", "-m", "initial commit")
	commitCmd.Dir = workdir
	err = commitCmd.Run()
	require.NoError(t, err)
}

// TestPreviewScrolling tests the scrolling functionality in the preview pane
func TestPreviewScrolling(t *testing.T) {
	// Track what commands were executed and their order
	var executedCommands []string
	inCopyMode := false
	scrollPosition := 0 // 0 = bottom, positive = scrolled up
	sessionCreated := false

	// Create test content with line numbers for scrolling
	const numLines = 100
	lines := make([]string, numLines+1)
	lines[0] = "$ seq 100" // Command that was run
	for i := 1; i <= numLines; i++ {
		lines[i] = fmt.Sprintf("%d", i)
	}
	fullContent := strings.Join(lines, "\n")

	// Mock command execution
	cmdExec := cmd_test.MockCmdExec{
		RunFunc: func(cmd *exec.Cmd) error {
			cmdStr := cmd.String()
			executedCommands = append(executedCommands, cmdStr)

			// Handle tmux session creation and existence checking
			if strings.Contains(cmdStr, "has-session") {
				if sessionCreated {
					return nil // Session exists
				} else {
					return fmt.Errorf("session does not exist")
				}
			}

			// Handle session creation
			if strings.Contains(cmdStr, "new-session") {
				sessionCreated = true
				return nil
			}

			// Handle attach-session
			if strings.Contains(cmdStr, "attach-session") {
				return nil
			}

			// Handle copy mode commands
			if strings.Contains(cmdStr, "copy-mode") {
				inCopyMode = true
			}
			if strings.Contains(cmdStr, "send-keys") && strings.Contains(cmdStr, "q") {
				inCopyMode = false
				scrollPosition = 0 // Reset position when exiting copy mode
			}
			if strings.Contains(cmdStr, "send-keys") && strings.Contains(cmdStr, "Up") {
				if inCopyMode {
					scrollPosition++
				}
			}
			if strings.Contains(cmdStr, "send-keys") && strings.Contains(cmdStr, "Down") {
				if inCopyMode && scrollPosition > 0 {
					scrollPosition--
				}
			}

			return nil
		},
		OutputFunc: func(cmd *exec.Cmd) ([]byte, error) {
			cmdStr := cmd.String()

			// Handle capture-pane commands
			if strings.Contains(cmdStr, "capture-pane") {
				// Check if this is a request for cursor position
				if strings.Contains(cmdStr, "display-message") && strings.Contains(cmdStr, "copy_cursor_y") {
					var buf []byte
					buf = fmt.Appendf(buf, "%d", scrollPosition)
					return buf, nil
				}

				// Check if this is a copy mode capture with full history (-S -)
				if strings.Contains(cmdStr, "-S -") {
					// Always return the full content for PreviewFullHistory
					return []byte(fullContent), nil
				}

				// Regular capture for normal preview mode - show the last 20 lines
				const visibleLines = 20
				startLine := max(0, numLines+1-visibleLines)
				visibleContent := strings.Join(lines[startLine:], "\n")
				return []byte(visibleContent), nil
			}

			return []byte(""), nil
		},
	}

	// Setup test environment
	setup := setupTestEnvironment(t, cmdExec)
	defer setup.cleanupFn()

	// Simulate running a command that produces lots of output
	err := setup.instance.SendKeys("seq 100")
	require.NoError(t, err)
	err = setup.instance.SendKeys("") // Simulate pressing Enter
	require.NoError(t, err)

	// Create the preview pane
	previewPane := NewPreviewPane()
	previewPane.SetSize(80, 30) // Set reasonable size for testing

	// Step 1: Check initial content - should show normal preview mode
	err = previewPane.UpdateContent(setup.instance)
	require.NoError(t, err)

	// Verify we're not in scrolling mode initially
	require.False(t, previewPane.isScrolling, "Should not be in scrolling mode initially")

	// Step 2: Check that PreviewFullHistory returns all content
	fullHistory, err := setup.instance.PreviewFullHistory()
	require.NoError(t, err)

	// Verify that the full history contains both the command and early output
	require.Contains(t, fullHistory, "$ seq 100", "Full history should contain the command")
	require.Contains(t, fullHistory, "1", "Full history should contain earliest output")

	// Step 3: Enter scroll mode
	err = previewPane.ScrollUp(setup.instance)
	require.NoError(t, err)

	// Verify we entered scrolling mode
	require.True(t, previewPane.isScrolling, "Should be in scrolling mode after ScrollUp")

	// Step 4: Get the content directly from the viewport
	viewportContent := previewPane.viewport.View()
	t.Logf("Viewport content: %q", viewportContent)

	// With proper implementation, the viewport should have the full history content
	// Note: The viewport will be positioned at the bottom initially, so we need to scroll up

	// Step 5: Scroll up multiple times to get to the top
	for range 50 {
		err = previewPane.ScrollUp(setup.instance)
		require.NoError(t, err)
	}

	// Now get the viewport content after scrolling up
	viewportAfterScrollUp := previewPane.viewport.View()
	t.Logf("Viewport after scrolling up: %q", viewportAfterScrollUp)

	// Step 6: Scroll down multiple times
	for range 25 {
		err = previewPane.ScrollDown(setup.instance)
		require.NoError(t, err)
	}

	// Get updated viewport content after scrolling down
	viewportAfterScrollDown := previewPane.viewport.View()
	t.Logf("Viewport after scrolling down: %q", viewportAfterScrollDown)

	// Step 7: Reset to normal mode
	err = previewPane.ResetToNormalMode(setup.instance)
	require.NoError(t, err)

	// Verify we exited scrolling mode
	require.False(t, previewPane.isScrolling, "Should not be in scrolling mode after reset")
}

// MockPtyFactory for testing tmux sessions
type MockPtyFactory struct {
	t       *testing.T
	cmdExec cmd_test.MockCmdExec

	// Array of commands and the corresponding file handles representing PTYs.
	cmds  []*exec.Cmd
	files []*os.File
}

func (pt *MockPtyFactory) Start(cmd *exec.Cmd) (*os.File, error) {
	filePath := filepath.Join(pt.t.TempDir(), fmt.Sprintf("pty-%s-%d", pt.t.Name(), len(pt.cmds)))
	f, err := os.OpenFile(filePath, os.O_CREATE|os.O_RDWR, 0644)
	if err == nil {
		pt.cmds = append(pt.cmds, cmd)
		pt.files = append(pt.files, f)

		// Execute the command through our mock to trigger session creation logic
		_ = pt.cmdExec.Run(cmd)
	}
	return f, err
}

func (pt *MockPtyFactory) Close() {}

// TestPreviewContentWithoutScrolling tests that the preview pane correctly displays content
// pushed via SetRawContent (the VT emulator path) without requiring scrolling.
// In the new architecture, UpdateContent no longer fetches content in normal mode —
// live content arrives via SetRawContent from the VT emulator tick loop.
func TestPreviewContentWithoutScrolling(t *testing.T) {
	// Create the preview pane
	previewPane := NewPreviewPane()
	previewPane.SetSize(80, 30)

	// Simulate the VT emulator pushing rendered content via SetRawContent.
	expectedContent := "$ echo test\ntest"
	previewPane.SetRawContent(expectedContent)

	// Verify we're not in scrolling mode
	require.False(t, previewPane.isScrolling, "Should not be in scrolling mode")

	// Verify that the preview state is not in fallback mode
	require.False(t, previewPane.previewState.fallback, "Preview should not be in fallback mode")

	// Verify that the preview state contains the expected content
	require.Equal(t, expectedContent, previewPane.previewState.text, "Preview state should contain the expected content")

	// Verify the rendered string contains the content
	renderedString := previewPane.String()
	require.Contains(t, renderedString, "test", "Rendered preview should contain the test content")
}

func TestPreviewPaneSetSize_ReservesScrollbarColumn(t *testing.T) {
	previewPane := NewPreviewPane()
	previewPane.SetSize(80, 24)

	require.Equal(t, 80, previewPane.width)
	require.Equal(t, 79, previewPane.viewport.Width())
	require.Equal(t, 24, previewPane.viewport.Height())
}

func TestPreviewPaneViewportUpdate_DocumentModeHandlesNativeKeys(t *testing.T) {
	previewPane := NewPreviewPane()
	previewPane.SetSize(30, 5)
	previewPane.SetDocumentContent(testDocumentLines(40))

	before := previewPane.viewport.View()
	cmd := previewPane.ViewportUpdate(tea.KeyPressMsg{Code: tea.KeyPgDown})
	after := previewPane.viewport.View()

	require.Nil(t, cmd)
	require.NotEqual(t, before, after)
}

func TestPreviewPaneViewportUpdate_NoOpOutsideScrollableModes(t *testing.T) {
	previewPane := NewPreviewPane()
	previewPane.SetSize(30, 5)
	previewPane.SetRawContent("plain preview")

	cmd := previewPane.ViewportUpdate(tea.KeyPressMsg{Code: tea.KeyPgDown})

	require.Nil(t, cmd)
	require.False(t, previewPane.IsDocumentMode())
	require.False(t, previewPane.isScrolling)
}

func TestPreviewPaneSetRawContent_PreservesScrollMode(t *testing.T) {
	previewPane := NewPreviewPane()
	previewPane.SetSize(30, 5)
	previewPane.viewport.SetContent(testDocumentLines(40) + "\nESC to exit scroll mode")
	previewPane.viewport.GotoBottom()
	previewPane.viewport.ScrollUp(3)
	previewPane.isScrolling = true

	before := previewPane.viewport.View()

	previewPane.SetRawContent("live terminal update")

	after := previewPane.viewport.View()
	require.True(t, previewPane.isScrolling)
	require.True(t, previewPane.isRawTerminal)
	require.Equal(t, "live terminal update", previewPane.previewState.text)
	require.Equal(t, before, after)
}

func TestPreviewPaneString_RendersScrollbarOnlyWhenScrollable(t *testing.T) {
	t.Run("shows scrollbar for long document", func(t *testing.T) {
		previewPane := NewPreviewPane()
		previewPane.SetSize(30, 6)
		previewPane.SetDocumentContent(testDocumentLines(60))

		rendered := previewPane.String()
		require.Contains(t, rendered, "▐")
		require.Contains(t, rendered, "│")
	})

	t.Run("hides scrollbar when content fits", func(t *testing.T) {
		previewPane := NewPreviewPane()
		previewPane.SetSize(30, 6)
		previewPane.SetDocumentContent(testDocumentLines(2))

		rendered := previewPane.String()
		require.NotContains(t, rendered, "▐")
		require.NotContains(t, rendered, "│")
	})
}

func TestPreviewPaneString_CentersFallbackContentInShortHeight(t *testing.T) {
	previewPane := NewPreviewPane()
	previewPane.SetSize(20, 8)
	previewPane.setFallbackContent("X")

	rendered := previewPane.String()
	lines := strings.Split(rendered, "\n")

	markerLine := -1
	for i, line := range lines {
		if strings.Contains(line, "X") {
			markerLine = i
			break
		}
	}

	require.Equal(t, 3, markerLine)
}

// TestPreviewPane_RawTerminalContent_NoEllipsis verifies that content pushed via
// SetRawContent (the VT emulator path) is rendered without clipping or an
// ellipsis marker, even when the number of lines exactly matches the pane height.
// Previously, String() unconditionally subtracted 1 from the available height,
// causing the last line of every embedded terminal frame to be dropped and
// replaced with "...".
func TestPreviewPane_RawTerminalContent_NoEllipsis(t *testing.T) {
	const rows = 24
	previewPane := NewPreviewPane()
	previewPane.SetSize(80, rows)

	// Build a rows-line string that simulates a VT-emulator snapshot.
	// Each line is uniquely identifiable so we can check the last one is present.
	lineStrs := make([]string, rows)
	for i := range rows {
		lineStrs[i] = fmt.Sprintf("terminal line %d", i+1)
	}
	rawContent := strings.Join(lineStrs, "\n")

	previewPane.SetRawContent(rawContent)

	rendered := previewPane.String()
	plain := stripPreviewANSI(rendered)

	// The last line must appear in the output.
	lastLine := fmt.Sprintf("terminal line %d", rows)
	require.Contains(t, plain, lastLine,
		"last VT-emulator line should appear in rendered output (no clipping)")

	// No ellipsis should be injected by the preview pane.
	require.NotContains(t, plain, "...",
		"preview pane must not inject '...' for raw terminal content")
}

func testDocumentLines(n int) string {
	var b strings.Builder
	for i := 1; i <= n; i++ {
		if i > 1 {
			b.WriteByte('\n')
		}
		_, _ = fmt.Fprintf(&b, "line %d", i)
	}
	return b.String()
}

// TestPreviewPane_SDKUpdateContent_RendersCachedContent verifies that when an
// SDK instance has cached capture content, UpdateContent shows it in the pane.
func TestPreviewPane_SDKUpdateContent_RendersCachedContent(t *testing.T) {
	previewPane := NewPreviewPane()
	previewPane.SetSize(80, 24)

	inst := &session.Instance{
		Status:           session.Running,
		ExecutionMode:    session.ExecutionModeSDK,
		CachedContent:    "sdk agent line",
		CachedContentSet: true,
	}
	require.NoError(t, previewPane.UpdateContent(inst))
	require.False(t, previewPane.previewState.fallback)
	require.Equal(t, "sdk agent line", previewPane.previewState.text)
}

// TestPreviewPane_SDKUpdateContent_ClearsStaleContentWhenCacheEmpty is a
// regression test for the bug where selecting a newly spawned SDK instance
// whose capture is empty left the previously selected instance's preview on
// screen.  UpdateContent must clear/replace the previewState so the pane
// always reflects the selected instance, not a stale predecessor.
func TestPreviewPane_SDKUpdateContent_ClearsStaleContentWhenCacheEmpty(t *testing.T) {
	previewPane := NewPreviewPane()
	previewPane.SetSize(80, 24)

	// Seed the pane with visible preview content from a different (prior)
	// instance — simulating a switch from a tmux or populated-SDK row.
	previewPane.SetRawContent("previous instance content")
	require.Equal(t, "previous instance content", previewPane.previewState.text)

	cases := []struct {
		name     string
		instance *session.Instance
	}{
		{
			name: "empty string",
			instance: &session.Instance{
				Status:           session.Running,
				ExecutionMode:    session.ExecutionModeSDK,
				CachedContent:    "",
				CachedContentSet: true,
			},
		},
		{
			name: "cache never set",
			instance: &session.Instance{
				Status:           session.Running,
				ExecutionMode:    session.ExecutionModeSDK,
				CachedContentSet: false,
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			previewPane.SetRawContent("previous instance content")
			require.NoError(t, previewPane.UpdateContent(tc.instance))
			require.False(t, previewPane.previewState.fallback,
				"empty SDK capture should render the empty SDK view, not the fallback banner")
			require.NotContains(t, previewPane.previewState.text, "previous instance content",
				"empty SDK capture must not leave stale content from the previously selected instance")
			require.Contains(t, previewPane.previewState.text, "> send a message to the agent",
				"empty SDK capture should show the interactive footer")
		})
	}
}

// TestPreviewPane_SDKUpdateContent_ClearsScrollModeWhenCacheEmpty verifies
// that switching into an SDK instance with no cached capture clears any
// inherited scroll-mode state, so the viewport does not continue consuming
// scroll keys against stale content from the previously selected instance.
func TestPreviewPane_SDKUpdateContent_ClearsScrollModeWhenCacheEmpty(t *testing.T) {
	previewPane := NewPreviewPane()
	previewPane.SetSize(80, 24)

	// Simulate a prior tmux instance that had engaged scroll mode.
	previewPane.isScrolling = true
	previewPane.viewport.SetContent("scrolled-tmux-content")
	require.True(t, previewPane.isScrolling)

	inst := &session.Instance{
		Status:           session.Running,
		ExecutionMode:    session.ExecutionModeSDK,
		CachedContentSet: false,
	}
	require.NoError(t, previewPane.UpdateContent(inst))

	require.False(t, previewPane.isScrolling,
		"entering the SDK empty-cache fallback path must drop inherited scroll-mode state")
	require.False(t, previewPane.previewState.fallback,
		"empty SDK capture should render the empty SDK view, not the fallback banner")
	require.Contains(t, previewPane.previewState.text, "> send a message to the agent",
		"empty SDK capture should show the interactive footer after clearing stale scroll state")
}

// fakeSDKSession is a no-op ExecutionSession that also implements
// presentationProvider. Used to inject structured turn data into an Instance
// without starting a real process.
type fakeSDKSession struct {
	turns []*sdk.PresentationTurn
}

func (f *fakeSDKSession) Start(string) error                                 { return nil }
func (f *fakeSDKSession) Restore() error                                     { return nil }
func (f *fakeSDKSession) Close() error                                       { return nil }
func (f *fakeSDKSession) DoesSessionExist() bool                             { return true }
func (f *fakeSDKSession) SendKeys(string) error                              { return nil }
func (f *fakeSDKSession) TapEnter() error                                    { return nil }
func (f *fakeSDKSession) SendPermissionResponse(tmux.PermissionChoice) error { return nil }
func (f *fakeSDKSession) CapturePaneContent() (string, error)                { return "", nil }
func (f *fakeSDKSession) CapturePaneContentWithOptions(string, string) (string, error) {
	return "", nil
}
func (f *fakeSDKSession) HasUpdated() (bool, bool) { return false, false }
func (f *fakeSDKSession) HasUpdatedWithContent() (bool, bool, string, bool) {
	return false, false, "", false
}
func (f *fakeSDKSession) GetPanePID() (int, error)                     { return 0, fmt.Errorf("no pane") }
func (f *fakeSDKSession) Attach() (chan struct{}, error)               { return nil, nil }
func (f *fakeSDKSession) DetachSafely() error                          { return nil }
func (f *fakeSDKSession) SetDetachedSize(int, int) error               { return nil }
func (f *fakeSDKSession) GetSanitizedName() string                     { return "" }
func (f *fakeSDKSession) SetAgentType(string)                          {}
func (f *fakeSDKSession) SetInitialPrompt(string)                      {}
func (f *fakeSDKSession) SetNoFlicker(bool)                            {}
func (f *fakeSDKSession) SetTaskEnv(int, int, int)                     {}
func (f *fakeSDKSession) SetProject(string)                            {}
func (f *fakeSDKSession) SetSessionTitle(string)                       {}
func (f *fakeSDKSession) SetTitleFunc(func(string, time.Time, string)) {}
func (f *fakeSDKSession) SetSDKSpeedTier(string)                       {}

// CapturePresentation implements the optional presentationProvider interface.
func (f *fakeSDKSession) CapturePresentation() []*sdk.PresentationTurn {
	return f.turns
}

// newSDKInstanceWithTurns creates a minimal SDK-mode instance injected with the
// given presentation turns via SetExecutionSessionForTest.
func newSDKInstanceWithTurns(t *testing.T, turns []*sdk.PresentationTurn) *session.Instance {
	t.Helper()
	inst, err := session.NewInstance(session.InstanceOptions{
		Title:   "test-sdk-instance",
		Path:    t.TempDir(),
		Program: "bash",
	})
	require.NoError(t, err)
	inst.ExecutionMode = session.ExecutionModeSDK
	inst.SetExecutionSessionForTest(&fakeSDKSession{turns: turns})
	inst.MarkStartedForTest()
	return inst
}

// TestPreviewPane_SDKPresentation_RendersTurnHierarchy verifies that UpdateContent
// prefers the structured presentation model over the flat cache and renders turns
// with the variant-c colour hierarchy from docs/agent-sdk-pane-mockups.md.
func TestPreviewPane_SDKPresentation_RendersTurnHierarchy(t *testing.T) {
	pane := NewPreviewPane()
	pane.SetSize(80, 40)

	now := time.Now()
	turn := &sdk.PresentationTurn{
		ID:        "t1",
		Number:    1,
		StartedAt: now,
		Rows: []sdk.PresentationRow{
			{Kind: sdk.RowTool, Text: "• read_file main.go", Timestamp: now},
			{Kind: sdk.RowResult, Text: "→ 42 lines", Timestamp: now},
			{Kind: sdk.RowResponse, Timestamp: now},
			{Kind: sdk.RowProse, Text: "assistant text", Timestamp: now},
		},
	}

	inst := newSDKInstanceWithTurns(t, []*sdk.PresentationTurn{turn})
	require.NoError(t, pane.UpdateContent(inst))

	require.False(t, pane.previewState.fallback, "pane must not be in fallback with presentation turns")

	rendered := pane.previewState.text

	// Tool rows use ColorSubtle.
	require.Contains(t, rendered, lipgloss.NewStyle().Foreground(ColorSubtle).Render("• read_file main.go"),
		"tool row must be rendered in ColorSubtle")

	// Successful result rows use ColorMuted (quieter than tools).
	require.Contains(t, rendered, lipgloss.NewStyle().Foreground(ColorMuted).Render("→ 42 lines"),
		"ok-result row must be rendered in ColorMuted")

	// Prose rows use ColorText.
	require.Contains(t, rendered, lipgloss.NewStyle().Foreground(ColorText).Render("assistant text"),
		"prose row must be rendered in ColorText")

	plain := stripPreviewANSI(rendered)
	require.NotContains(t, plain, "response",
		"RowResponse must not emit a visible label")
	require.Regexp(t, `(?s)42 lines\s+─{8,}\s+assistant text`, plain,
		"RowResponse must render a divider rule between result and prose")

	// Composer footer must be present.
	require.Contains(t, rendered, "> send a message to the agent",
		"composer footer must appear after the turn timeline")
}

// TestPreviewPane_SDKPresentation_ErrorResultColor verifies that result rows with
// IsError=true are rendered in ColorLove (distinct from ok results in ColorMuted).
func TestPreviewPane_SDKPresentation_ErrorResultColor(t *testing.T) {
	pane := NewPreviewPane()
	pane.SetSize(80, 40)

	now := time.Now()
	turn := &sdk.PresentationTurn{
		ID:        "t1",
		Number:    1,
		StartedAt: now,
		Rows: []sdk.PresentationRow{
			{Kind: sdk.RowResult, Text: "✗ tool failed", Timestamp: now, IsError: true},
		},
	}

	inst := newSDKInstanceWithTurns(t, []*sdk.PresentationTurn{turn})
	require.NoError(t, pane.UpdateContent(inst))

	rendered := pane.previewState.text
	require.Contains(t, rendered, lipgloss.NewStyle().Foreground(ColorLove).Render("✗ tool failed"),
		"error result must be rendered in ColorLove")
	require.NotContains(t, rendered, lipgloss.NewStyle().Foreground(ColorMuted).Render("✗ tool failed"),
		"error result must not be rendered in ColorMuted")
}

// TestPreviewPane_SDKPresentation_PermissionColor checks that permission rows use ColorRose.
func TestPreviewPane_SDKPresentation_PermissionColor(t *testing.T) {
	pane := NewPreviewPane()
	pane.SetSize(80, 40)

	now := time.Now()
	turn := &sdk.PresentationTurn{
		ID:        "t1",
		Number:    1,
		StartedAt: now,
		Rows: []sdk.PresentationRow{
			{Kind: sdk.RowPermission, Text: "[permission: write /etc/hosts]", Timestamp: now},
		},
	}

	inst := newSDKInstanceWithTurns(t, []*sdk.PresentationTurn{turn})
	require.NoError(t, pane.UpdateContent(inst))

	rendered := pane.previewState.text
	require.Contains(t, rendered,
		lipgloss.NewStyle().Foreground(ColorRose).Render("[permission: write /etc/hosts]"),
		"permission row must be rendered in ColorRose")
}

// TestPreviewPane_SDKPresentation_FallsBackToCachedContent verifies that when the
// presentation model returns no turns but cached flat content is available, the
// pane uses the flat cache (preserving the existing fallback contract).
func TestPreviewPane_SDKPresentation_FallsBackToCachedContent(t *testing.T) {
	pane := NewPreviewPane()
	pane.SetSize(80, 24)

	// Instance has no presentation turns but has cached content.
	inst, err := session.NewInstance(session.InstanceOptions{
		Title:   "test-sdk-no-turns",
		Path:    t.TempDir(),
		Program: "bash",
	})
	require.NoError(t, err)
	inst.ExecutionMode = session.ExecutionModeSDK
	// Inject a session with empty turns (nil slice) so CapturePresentation() returns nil.
	inst.SetExecutionSessionForTest(&fakeSDKSession{turns: nil})
	inst.MarkStartedForTest()
	inst.CachedContent = "flat cached output"
	inst.CachedContentSet = true

	require.NoError(t, pane.UpdateContent(inst))
	require.False(t, pane.previewState.fallback)
	require.Equal(t, "flat cached output", pane.previewState.text,
		"pane must fall back to flat cached content when no turns are present")
}

// TestPreviewPane_SDKPresentation_ShowsComposerWhenNoOutput verifies that an
// SDK instance with no turns and no cache still renders the empty interactive
// footer instead of falling back to the generic banner.
func TestPreviewPane_SDKPresentation_ShowsComposerWhenNoOutput(t *testing.T) {
	pane := NewPreviewPane()
	pane.SetSize(80, 24)

	inst := newSDKInstanceWithTurns(t, nil)
	require.NoError(t, pane.UpdateContent(inst))

	require.False(t, pane.previewState.fallback,
		"no turns and no cache should render the empty SDK view")
	require.Contains(t, pane.previewState.text, "> send a message to the agent",
		"empty SDK preview should still show the composer footer")
}

func TestPreviewPane_SDKPresentation_UsesCachedPresentationForPlaceholder(t *testing.T) {
	pane := NewPreviewPane()
	pane.SetSize(80, 24)

	inst, err := session.NewInstance(session.InstanceOptions{
		Title:   "daemon-sdk-placeholder",
		Path:    t.TempDir(),
		Program: "bash",
	})
	require.NoError(t, err)
	inst.ExecutionMode = session.ExecutionModeSDK
	inst.SetCachedPresentation([]*sdk.PresentationTurn{
		{
			ID:     "t1",
			Number: 1,
			Rows: []sdk.PresentationRow{
				{Kind: sdk.RowResponse},
				{Kind: sdk.RowProse, Text: "placeholder structured output"},
			},
		},
	})

	require.NoError(t, pane.UpdateContent(inst))
	require.False(t, pane.previewState.fallback)
	require.Contains(t, pane.previewState.text, "placeholder structured output")
}

func TestPreviewPane_SDKPresentation_FocusedComposerShowsTypedText(t *testing.T) {
	pane := NewPreviewPane()
	pane.SetSize(80, 24)

	inst := newSDKInstanceWithTurns(t, nil)
	require.NoError(t, pane.UpdateContent(inst))
	pane.SetSDKFocusMode(true)
	pane.AppendSDKComposerText("hello")
	require.NoError(t, pane.UpdateContent(inst))

	require.Contains(t, pane.previewState.text, "> hello█")
	require.Contains(t, pane.previewState.text,
		lipgloss.NewStyle().Foreground(lipgloss.Color("#ffffff")).Render("> hello█"))
	require.Contains(t, pane.previewState.text, "shift+enter newline")
}

func TestPreviewPane_SDKPresentation_UserHistoryUsesFoam(t *testing.T) {
	pane := NewPreviewPane()
	pane.SetSize(80, 24)

	now := time.Now()
	inst := newSDKInstanceWithTurns(t, []*sdk.PresentationTurn{{
		ID:        "t1",
		Number:    1,
		StartedAt: now,
		Rows: []sdk.PresentationRow{{
			Kind:      sdk.RowUser,
			Text:      "show logs",
			Timestamp: now,
		}},
	}})
	require.NoError(t, pane.UpdateContent(inst))

	require.Contains(t, pane.previewState.text,
		lipgloss.NewStyle().Foreground(ColorFoam).Render("you: show logs"))
}

func TestPreviewPane_SDKPresentation_ShowsFooterMetadataBeforeHints(t *testing.T) {
	pane := NewPreviewPane()
	pane.SetSize(80, 24)

	inst := newSDKInstanceWithTurns(t, nil)
	inst.Program = "codex -m gpt-5.4 -c reasoning.effort=xhigh"
	inst.SDKSpeedTier = "fast"
	require.NoError(t, pane.UpdateContent(inst))

	plain := stripPreviewANSI(pane.previewState.text)
	require.Contains(t, plain, "gpt-5.4 xhigh fast (2x) • enter send")
	require.NotContains(t, plain, "gpt-5.4 xhigh fast (2x)\nenter send")
}

// TestPreviewPane_SDKPresentation_ClearsInheritedScrollModeOnInstanceSwitch
// verifies that switching from a previously scrolled instance to an SDK
// instance with structured turns exits scroll mode so the new timeline renders
// instead of stale viewport content.
func TestPreviewPane_SDKPresentation_ClearsInheritedScrollModeOnInstanceSwitch(t *testing.T) {
	pane := NewPreviewPane()
	pane.SetSize(80, 24)

	prev := &session.Instance{
		Title:            "previous-instance",
		Status:           session.Running,
		ExecutionMode:    session.ExecutionModeSDK,
		CachedContent:    "old content",
		CachedContentSet: true,
	}
	require.NoError(t, pane.UpdateContent(prev))

	pane.isScrolling = true
	pane.viewport.SetContent("stale scrolled content")

	now := time.Now()
	turn := &sdk.PresentationTurn{
		ID:        "t1",
		Number:    1,
		StartedAt: now,
		Rows: []sdk.PresentationRow{
			{Kind: sdk.RowResponse, Timestamp: now},
			{Kind: sdk.RowProse, Text: "fresh structured output", Timestamp: now},
		},
	}
	inst := newSDKInstanceWithTurns(t, []*sdk.PresentationTurn{turn})

	require.NoError(t, pane.UpdateContent(inst))
	require.False(t, pane.isScrolling,
		"switching to a different instance must drop inherited scroll mode")

	rendered := stripPreviewANSI(pane.String())
	require.Contains(t, rendered, "fresh structured output",
		"pane must render the new structured timeline after the instance switch")
	require.NotContains(t, rendered, "stale scrolled content",
		"pane must not keep rendering stale viewport content from the previous instance")
}

// TestPreviewPane_SDKPresentation_ComposerFooterPresent checks that the composer
// footer appears even when the turn list is empty (though in practice the
// structured path only fires when len(turns) > 0, test the helper directly).
func TestPreviewPane_SDKPresentation_ComposerFooterPresent(t *testing.T) {
	output := renderSDKPresentation(nil, 80)
	require.Contains(t, output, "> send a message to the agent",
		"renderSDKPresentation must include composer footer prompt")
	require.Contains(t, output, "shift+enter newline",
		"renderSDKPresentation must include keyboard hint line")
}

func TestPreviewPane_SDKScrollMode_UsesStructuredPresentationWhenFlatHistoryEmpty(t *testing.T) {
	pane := NewPreviewPane()
	pane.SetSize(80, 12)

	now := time.Now()
	turn := &sdk.PresentationTurn{
		ID:        "t1",
		Number:    1,
		StartedAt: now,
		Rows: []sdk.PresentationRow{
			{Kind: sdk.RowTool, Text: "• read_file plan.md", Timestamp: now},
			{Kind: sdk.RowResponse, Timestamp: now},
			{Kind: sdk.RowProse, Text: "structured history survives scroll mode", Timestamp: now},
		},
	}
	inst := newSDKInstanceWithTurns(t, []*sdk.PresentationTurn{turn})

	require.NoError(t, pane.UpdateContent(inst))
	require.NoError(t, pane.ScrollUp(inst))
	require.True(t, pane.isScrolling)
	require.Contains(t, pane.viewport.View(), "structured history survives scroll mode")
}

// TestPreviewPane_SDKPresentation_RunningTurnHeaderIndicator verifies that an
// open (running) turn's header includes the "• running" indicator.
func TestPreviewPane_SDKPresentation_RunningTurnHeaderIndicator(t *testing.T) {
	pane := NewPreviewPane()
	pane.SetSize(80, 40)

	now := time.Now()
	turn := &sdk.PresentationTurn{
		ID:        "t1",
		Number:    1,
		StartedAt: now.Add(-5 * time.Second),
		// No CompletedAt, not Interrupted → Running() == true.
	}

	inst := newSDKInstanceWithTurns(t, []*sdk.PresentationTurn{turn})
	require.NoError(t, pane.UpdateContent(inst))

	rendered := pane.previewState.text
	require.Contains(t, rendered, "• running",
		"running turn must display '• running' indicator in pane header")
}

// TestPreviewPane_SDKPresentation_InterruptedStatusColorGold verifies that
// RowStatus rows (added by TurnInterrupted events) are rendered in ColorGold,
// not ColorMuted.
func TestPreviewPane_SDKPresentation_InterruptedStatusColorGold(t *testing.T) {
	pane := NewPreviewPane()
	pane.SetSize(80, 40)

	now := time.Now()
	turn := &sdk.PresentationTurn{
		ID:          "t1",
		Number:      1,
		StartedAt:   now.Add(-5 * time.Second),
		Interrupted: true,
		Rows: []sdk.PresentationRow{
			{Kind: sdk.RowStatus, Text: "[interrupted]", Timestamp: now},
		},
	}

	inst := newSDKInstanceWithTurns(t, []*sdk.PresentationTurn{turn})
	require.NoError(t, pane.UpdateContent(inst))

	rendered := pane.previewState.text
	require.Contains(t, rendered,
		lipgloss.NewStyle().Foreground(ColorGold).Render("[interrupted]"),
		"interrupted RowStatus row must be rendered in ColorGold")
	require.NotContains(t, rendered,
		lipgloss.NewStyle().Foreground(ColorMuted).Render("[interrupted]"),
		"interrupted RowStatus must not be rendered in ColorMuted")
}

// TestPreviewPane_SDKPresentation_NarrowPane_MinimalHeader verifies that when
// the pane width is below narrowPaneThreshold (40), the turn header is reduced
// to just "turn N" — no elapsed time, tool count, or running label.
func TestPreviewPane_SDKPresentation_NarrowPane_MinimalHeader(t *testing.T) {
	pane := NewPreviewPane()
	pane.SetSize(30, 40) // width=30 < narrowPaneThreshold=40

	now := time.Now()
	turn := &sdk.PresentationTurn{
		ID:        "t1",
		Number:    3,
		StartedAt: now.Add(-10 * time.Second),
		ToolCount: 5,
		// No CompletedAt → Running.
	}

	inst := newSDKInstanceWithTurns(t, []*sdk.PresentationTurn{turn})
	require.NoError(t, pane.UpdateContent(inst))

	rendered := pane.previewState.text
	// Minimal label must appear.
	require.Contains(t, rendered, "turn 3")
	// Verbose details must be suppressed.
	require.NotContains(t, rendered, "10s", "elapsed must be suppressed in narrow mode")
	require.NotContains(t, rendered, "5 tool", "tool count must be suppressed in narrow mode")
	require.NotContains(t, rendered, "• running", "running label must be suppressed in narrow mode")
}

// TestPreviewPane_SDKPresentation_NarrowPane_NoDoubleSpacer verifies that
// narrow pane mode (width < narrowPaneThreshold) uses single-newline separators
// between turns, not the double-newline used in normal mode.
func TestPreviewPane_SDKPresentation_NarrowPane_NoDoubleSpacer(t *testing.T) {
	now := time.Now()
	turns := []*sdk.PresentationTurn{
		{
			ID: "t1", Number: 1, StartedAt: now, CompletedAt: now,
			Rows: []sdk.PresentationRow{{Kind: sdk.RowProse, Text: "first", Timestamp: now}},
		},
		{
			ID: "t2", Number: 2, StartedAt: now, CompletedAt: now,
			Rows: []sdk.PresentationRow{{Kind: sdk.RowProse, Text: "second", Timestamp: now}},
		},
	}
	rendered := renderSDKPresentation(turns, 30) // narrow width
	require.NotContains(t, rendered, "\n\n",
		"narrow mode must not insert double-newline separators between turns")
}

// TestPreviewPane_SDKPresentation_NormalPane_DoubleSpacerPreserved verifies
// that normal width (>= narrowPaneThreshold) keeps double-newline turn separation.
func TestPreviewPane_SDKPresentation_NormalPane_DoubleSpacerPreserved(t *testing.T) {
	now := time.Now()
	turns := []*sdk.PresentationTurn{
		{
			ID: "t1", Number: 1, StartedAt: now, CompletedAt: now,
			Rows: []sdk.PresentationRow{{Kind: sdk.RowProse, Text: "alpha", Timestamp: now}},
		},
		{
			ID: "t2", Number: 2, StartedAt: now, CompletedAt: now,
			Rows: []sdk.PresentationRow{{Kind: sdk.RowProse, Text: "beta", Timestamp: now}},
		},
	}
	rendered := renderSDKPresentation(turns, 80) // normal width
	// Double newline must separate the two turns.
	idxAlpha := strings.Index(rendered, "alpha")
	idxBeta := strings.Index(rendered, "beta")
	require.True(t, idxAlpha >= 0 && idxBeta > idxAlpha)
	between := rendered[idxAlpha:idxBeta]
	require.Contains(t, between, "\n\n",
		"normal mode must separate turns with double newline")
}

// TestPreviewPane_SDKPresentation_FlatScrollbackPreservesContract verifies that
// an SDK instance's structured presentation does not bleed into the flat cache,
// ensuring that CachedContent and structured turns remain independent paths.
func TestPreviewPane_SDKPresentation_FlatScrollbackPreservesContract(t *testing.T) {
	pane := NewPreviewPane()
	pane.SetSize(80, 24)

	now := time.Now()
	turn := &sdk.PresentationTurn{
		ID:        "t1",
		Number:    1,
		StartedAt: now,
		Rows: []sdk.PresentationRow{
			{Kind: sdk.RowProse, Text: "structured text", Timestamp: now},
		},
	}

	// Instance has both structured turns and flat cached content.
	inst := newSDKInstanceWithTurns(t, []*sdk.PresentationTurn{turn})
	inst.CachedContent = "flat cached content"
	inst.CachedContentSet = true

	// Normal mode must prefer the structured presentation path.
	require.NoError(t, pane.UpdateContent(inst))
	require.False(t, pane.previewState.fallback)
	require.Contains(t, pane.previewState.text, "structured text",
		"normal mode must use structured presentation when turns are present")
	require.NotContains(t, pane.previewState.text, "flat cached content",
		"structured presentation must not include flat cache text")
}

// TestPreviewPane_SDKPresentation_MultipleTurnsSeparatedByBlankLines verifies
// that consecutive turns are separated by blank lines in the rendered output.
func TestPreviewPane_SDKPresentation_MultipleTurnsSeparatedByBlankLines(t *testing.T) {
	pane := NewPreviewPane()
	pane.SetSize(80, 60)

	now := time.Now()
	turns := []*sdk.PresentationTurn{
		{
			ID: "t1", Number: 1, StartedAt: now,
			Rows: []sdk.PresentationRow{
				{Kind: sdk.RowProse, Text: "first turn prose", Timestamp: now},
			},
		},
		{
			ID: "t2", Number: 2, StartedAt: now,
			Rows: []sdk.PresentationRow{
				{Kind: sdk.RowProse, Text: "second turn prose", Timestamp: now},
			},
		},
	}

	inst := newSDKInstanceWithTurns(t, turns)
	require.NoError(t, pane.UpdateContent(inst))

	rendered := pane.previewState.text
	// Both turns must appear in the output.
	require.Contains(t, rendered, "first turn prose")
	require.Contains(t, rendered, "second turn prose")

	// They must be separated by at least one blank line (\n\n).
	idxFirst := strings.Index(rendered, "first turn prose")
	idxSecond := strings.Index(rendered, "second turn prose")
	require.True(t, idxFirst >= 0 && idxSecond > idxFirst)
	between := rendered[idxFirst:idxSecond]
	require.Contains(t, between, "\n\n",
		"turns must be separated by a blank line")
}
