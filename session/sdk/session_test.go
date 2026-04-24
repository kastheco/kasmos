package sdk

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/kastheco/kasmos/session/tmux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockTransport is a minimal Transport implementation for testing.
type mockTransport struct {
	mu             sync.Mutex
	startErr       error
	sendPromptErr  error
	interruptErr   error
	respondPermErr error
	started        bool
	pid            int
	events         chan Event
	closed         bool

	lastPrompt     string
	interruptCnt   int
	lastPermChoice tmux.PermissionChoice
}

func newMockTransport() *mockTransport {
	return &mockTransport{
		pid:    12345,
		events: make(chan Event, 64),
	}
}

func (m *mockTransport) Start(_ context.Context, _ LaunchConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.startErr != nil {
		return m.startErr
	}
	m.started = true
	return nil
}

func (m *mockTransport) SendPrompt(_ context.Context, prompt string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.lastPrompt = prompt
	return m.sendPromptErr
}

func (m *mockTransport) Interrupt(_ context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.interruptCnt++
	return m.interruptErr
}

func (m *mockTransport) RespondPermission(_ context.Context, choice tmux.PermissionChoice) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.lastPermChoice = choice
	return m.respondPermErr
}

func (m *mockTransport) PendingPermission() (description, pattern string, ok bool) {
	return "", "", false
}

func (m *mockTransport) Events() <-chan Event {
	return m.events
}

func (m *mockTransport) PID() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.started {
		return 0
	}
	return m.pid
}

func (m *mockTransport) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.closed {
		m.closed = true
		close(m.events)
	}
	return nil
}

// injectTransport replaces the transport factory for a test.
// Returns a restore function.
func injectTransport(tr Transport) func() {
	orig := transportFactory
	transportFactory = func(_ string) (Transport, bool) {
		return tr, true
	}
	return func() { transportFactory = orig }
}

// --- tests ---

func TestNew_ReturnsNonNil(t *testing.T) {
	s := New("name", "claude", false)
	require.NotNil(t, s)
}

func TestNew_ImplementsExecutionSession(t *testing.T) {
	var _ ExecutionSessionIface = New("name", "claude", false)
}

func TestSDKSession_DoesSessionExist_BeforeStart(t *testing.T) {
	s := New("name", "claude", false)
	assert.False(t, s.DoesSessionExist())
}

func TestSDKSession_DoesSessionExist_AfterStart(t *testing.T) {
	mock := newMockTransport()
	restore := injectTransport(mock)
	defer restore()

	s := New("name", "claude", false)
	require.NoError(t, s.Start(t.TempDir()))
	assert.True(t, s.DoesSessionExist())
}

func TestSDKSession_DoesSessionExist_AfterClose(t *testing.T) {
	mock := newMockTransport()
	restore := injectTransport(mock)
	defer restore()

	s := New("name", "claude", false)
	require.NoError(t, s.Start(t.TempDir()))
	require.NoError(t, s.Close())
	// Close must flip alive immediately — DoesSessionExist() should not
	// depend on the event-consumer goroutine observing the closed channel.
	assert.False(t, s.DoesSessionExist())
}

func TestSDKSession_Start_Unsupported_ReturnsError(t *testing.T) {
	s := New("name", "opencode", false)
	err := s.Start(t.TempDir())
	assert.Error(t, err)
}

func TestSDKSession_Start_TransportError_ReturnsError(t *testing.T) {
	mock := newMockTransport()
	mock.startErr = errors.New("transport failed")
	restore := injectTransport(mock)
	defer restore()

	s := New("name", "claude", false)
	err := s.Start(t.TempDir())
	assert.Error(t, err)
	assert.False(t, s.DoesSessionExist())
}

func TestSDKSession_Attach_ReturnsErrInteractiveOnly(t *testing.T) {
	s := New("name", "claude", false)
	_, err := s.Attach()
	assert.ErrorIs(t, err, ErrInteractiveOnly)
}

func TestSDKSession_DetachSafely_ReturnsErrInteractiveOnly(t *testing.T) {
	s := New("name", "claude", false)
	err := s.DetachSafely()
	assert.ErrorIs(t, err, ErrInteractiveOnly)
}

func TestSDKSession_SetDetachedSize_ReturnsErrInteractiveOnly(t *testing.T) {
	s := New("name", "claude", false)
	err := s.SetDetachedSize(80, 24)
	assert.ErrorIs(t, err, ErrInteractiveOnly)
}

func TestSDKSession_SendKeys_BuffersText(t *testing.T) {
	mock := newMockTransport()
	restore := injectTransport(mock)
	defer restore()

	s := New("name", "claude", false)
	require.NoError(t, s.Start(t.TempDir()))
	require.NoError(t, s.SendKeys("hello"))
	// Text should be buffered — not yet sent to transport.
	mock.mu.Lock()
	assert.Equal(t, "", mock.lastPrompt)
	mock.mu.Unlock()
}

func TestSDKSession_TapEnter_SubmitsBufferedPrompt(t *testing.T) {
	mock := newMockTransport()
	restore := injectTransport(mock)
	defer restore()

	s := New("name", "claude", false)
	require.NoError(t, s.Start(t.TempDir()))
	require.NoError(t, s.SendKeys("hello world"))
	require.NoError(t, s.TapEnter())

	mock.mu.Lock()
	assert.Equal(t, "hello world", mock.lastPrompt)
	mock.mu.Unlock()
}

func TestSDKSession_TapEnter_RecordsUserPromptInPresentation(t *testing.T) {
	mock := newMockTransport()
	restore := injectTransport(mock)
	defer restore()

	s := New("name", "claude", false)
	require.NoError(t, s.Start(t.TempDir()))
	require.NoError(t, s.SendKeys("show logs"))
	require.NoError(t, s.TapEnter())

	turns := s.CapturePresentation()
	require.Len(t, turns, 1)
	require.Len(t, turns[0].Rows, 1)
	assert.Equal(t, RowUser, turns[0].Rows[0].Kind)
	assert.Equal(t, "show logs", turns[0].Rows[0].Text)
}

func TestSDKSession_TapEnter_ClearsBuffer(t *testing.T) {
	mock := newMockTransport()
	restore := injectTransport(mock)
	defer restore()

	s := New("name", "claude", false)
	require.NoError(t, s.Start(t.TempDir()))
	require.NoError(t, s.SendKeys("first"))
	require.NoError(t, s.TapEnter())
	// Second TapEnter should send empty prompt.
	require.NoError(t, s.TapEnter())
	mock.mu.Lock()
	assert.Equal(t, "", mock.lastPrompt)
	mock.mu.Unlock()
}

func TestSDKSession_SendKeys_Interrupt(t *testing.T) {
	mock := newMockTransport()
	restore := injectTransport(mock)
	defer restore()

	s := New("name", "claude", false)
	require.NoError(t, s.Start(t.TempDir()))
	require.NoError(t, s.SendKeys("\x03"))

	mock.mu.Lock()
	assert.Equal(t, 1, mock.interruptCnt)
	mock.mu.Unlock()
}

func TestSDKSession_SendPermissionResponse_Forwards(t *testing.T) {
	mock := newMockTransport()
	restore := injectTransport(mock)
	defer restore()

	s := New("name", "claude", false)
	require.NoError(t, s.Start(t.TempDir()))
	err := s.SendPermissionResponse(tmux.PermissionAllowOnce)
	assert.NoError(t, err)
	mock.mu.Lock()
	assert.Equal(t, tmux.PermissionAllowOnce, mock.lastPermChoice)
	mock.mu.Unlock()
}

func TestSDKSession_SendPermissionResponse_BeforeStart_ReturnsErrInteractiveOnly(t *testing.T) {
	s := New("name", "claude", false)
	err := s.SendPermissionResponse(tmux.PermissionAllowOnce)
	assert.ErrorIs(t, err, ErrInteractiveOnly)
}

func TestSDKSession_CapturePaneContent_Empty(t *testing.T) {
	mock := newMockTransport()
	restore := injectTransport(mock)
	defer restore()

	s := New("name", "claude", false)
	require.NoError(t, s.Start(t.TempDir()))
	content, err := s.CapturePaneContent()
	require.NoError(t, err)
	assert.Equal(t, "", content)
}

func TestSDKSession_CapturePaneContent_ShowsEvents(t *testing.T) {
	mock := newMockTransport()
	restore := injectTransport(mock)
	defer restore()

	s := New("name", "claude", false)
	require.NoError(t, s.Start(t.TempDir()))

	// Send an event directly through the mock's events channel.
	mock.events <- Event{Kind: EventTextDelta, Text: "hello"}
	// Give the goroutine time to process.
	time.Sleep(20 * time.Millisecond)

	content, err := s.CapturePaneContent()
	require.NoError(t, err)
	assert.Contains(t, content, "hello")
}

func TestSDKSession_CapturePaneContentWithOptions_DashDash_ReturnsAll(t *testing.T) {
	mock := newMockTransport()
	restore := injectTransport(mock)
	defer restore()

	s := New("name", "claude", false)
	require.NoError(t, s.Start(t.TempDir()))
	mock.events <- Event{Kind: EventTextDelta, Text: "hello"}
	time.Sleep(20 * time.Millisecond)

	all, err := s.CapturePaneContentWithOptions("-", "-")
	require.NoError(t, err)
	full, _ := s.CapturePaneContent()
	assert.Equal(t, full, all)
}

func TestSDKSession_HasUpdated_InitiallyFalse(t *testing.T) {
	mock := newMockTransport()
	restore := injectTransport(mock)
	defer restore()

	s := New("name", "claude", false)
	require.NoError(t, s.Start(t.TempDir()))
	updated, _ := s.HasUpdated()
	assert.False(t, updated)
}

func TestSDKSession_HasUpdated_TrueAfterEvent(t *testing.T) {
	mock := newMockTransport()
	restore := injectTransport(mock)
	defer restore()

	s := New("name", "claude", false)
	require.NoError(t, s.Start(t.TempDir()))
	// Drain initial state.
	s.HasUpdated()

	mock.events <- Event{Kind: EventTextDelta, Text: "new content"}
	time.Sleep(20 * time.Millisecond)

	updated, _ := s.HasUpdated()
	assert.True(t, updated)
}

func TestSDKSession_HasPrompt_TrueAfterHasPromptEvent(t *testing.T) {
	mock := newMockTransport()
	restore := injectTransport(mock)
	defer restore()

	s := New("name", "claude", false)
	require.NoError(t, s.Start(t.TempDir()))

	mock.events <- Event{Kind: EventTurnCompleted, HasPrompt: true}
	time.Sleep(20 * time.Millisecond)

	_, hasPrompt := s.HasUpdated()
	assert.True(t, hasPrompt)
}

func TestSDKSession_HasPrompt_ClearedOnTurnStarted(t *testing.T) {
	mock := newMockTransport()
	restore := injectTransport(mock)
	defer restore()

	s := New("name", "claude", false)
	require.NoError(t, s.Start(t.TempDir()))

	mock.events <- Event{Kind: EventTurnCompleted, HasPrompt: true}
	time.Sleep(10 * time.Millisecond)
	mock.events <- Event{Kind: EventTurnStarted}
	time.Sleep(10 * time.Millisecond)

	_, hasPrompt := s.HasUpdated()
	assert.False(t, hasPrompt)
}

func TestSDKSession_GetPanePID_BeforeStart_ReturnsZero(t *testing.T) {
	s := New("name", "claude", false)
	pid, err := s.GetPanePID()
	assert.NoError(t, err)
	assert.Equal(t, 0, pid)
}

func TestSDKSession_GetPanePID_AfterStart(t *testing.T) {
	mock := newMockTransport()
	restore := injectTransport(mock)
	defer restore()

	s := New("name", "claude", false)
	require.NoError(t, s.Start(t.TempDir()))
	pid, err := s.GetPanePID()
	require.NoError(t, err)
	assert.Equal(t, 12345, pid)
}

func TestSDKSession_GetSanitizedName(t *testing.T) {
	s := New("my session.name", "claude", false)
	assert.Equal(t, "mysession_name", s.GetSanitizedName())
}

func TestSDKSession_Restore_NoOp(t *testing.T) {
	s := New("name", "claude", false)
	assert.NoError(t, s.Restore())
}

func TestSDKSession_Close_BeforeStart_NoError(t *testing.T) {
	s := New("name", "claude", false)
	assert.NoError(t, s.Close())
}

func TestSDKSession_BuilderSetters_NoPanic(t *testing.T) {
	s := New("name", "claude", false)
	s.SetAgentType("coder")
	s.SetInitialPrompt("do the thing")
	s.SetNoFlicker(true)
	s.SetTaskEnv(1, 2, 3)
	s.SetProject("myproject")
	s.SetSessionTitle("title")
	s.SetTitleFunc(func(workDir string, beforeStart time.Time, title string) {})
}

func TestSDKSession_CapturePresentation_Empty(t *testing.T) {
	mock := newMockTransport()
	restore := injectTransport(mock)
	defer restore()

	s := New("name", "claude", false)
	require.NoError(t, s.Start(t.TempDir()))
	turns := s.CapturePresentation()
	assert.Nil(t, turns)
}

func TestSDKSession_CapturePresentation_AfterTurnStarted(t *testing.T) {
	mock := newMockTransport()
	restore := injectTransport(mock)
	defer restore()

	s := New("name", "claude", false)
	require.NoError(t, s.Start(t.TempDir()))

	mock.events <- Event{Kind: EventTurnStarted, TurnID: "t1"}
	mock.events <- Event{Kind: EventTextDelta, Text: "hello"}

	var turns []*PresentationTurn
	require.Eventually(t, func() bool {
		turns = s.CapturePresentation()
		return len(turns) == 1 && turns[0].ID == "t1"
	}, time.Second, 10*time.Millisecond)

	assert.Equal(t, 1, turns[0].Number)
	assert.Equal(t, "t1", turns[0].ID)
}

func TestSDKSession_SetSDKSpeedTier_PropagatesViaLaunchConfig(t *testing.T) {
	var capturedCfg LaunchConfig
	orig := transportFactory
	transportFactory = func(_ string) (Transport, bool) {
		mt := newMockTransport()
		// Wrap to capture the LaunchConfig passed to Start.
		return &captureCfgTransport{Transport: mt, onStart: func(cfg LaunchConfig) { capturedCfg = cfg }}, true
	}
	defer func() { transportFactory = orig }()

	s := New("name", "codex", false)
	s.SetSDKSpeedTier("fast")
	require.NoError(t, s.Start(t.TempDir()))

	assert.Equal(t, "fast", capturedCfg.SpeedTier)
}

func TestSDKSession_ImplementsIface(t *testing.T) {
	var _ ExecutionSessionIface = New("name", "claude", false)
}

// captureCfgTransport wraps a Transport and calls onStart with the LaunchConfig
// passed to Start.  Used to inspect what parameters the Session passes through.
type captureCfgTransport struct {
	Transport
	onStart func(LaunchConfig)
}

func (c *captureCfgTransport) Start(ctx context.Context, cfg LaunchConfig) error {
	c.onStart(cfg)
	return c.Transport.Start(ctx, cfg)
}

// Compile-time assertion: *Session must satisfy the shell runner interface.
var _ interface{ RunShellCommand(context.Context, string) error } = (*Session)(nil)

// stubShellRunner is a test seam for RunShellCommand that avoids spawning a
// real subprocess.
func stubShellRunner(exitCode int, output string, truncated bool, runErr error) shellRunner {
	return func(_ context.Context, _, _ string, _ []string) (int, string, bool, error) {
		return exitCode, output, truncated, runErr
	}
}

func startedSession(t *testing.T) (*Session, *mockTransport) {
	t.Helper()
	mock := newMockTransport()
	restore := injectTransport(mock)
	t.Cleanup(restore)
	s := New("name", "claude", false)
	require.NoError(t, s.Start(t.TempDir()))
	return s, mock
}

func TestRunShellCommand_SuccessAddsCorrectTurn(t *testing.T) {
	orig := runCommandSeam
	runCommandSeam = stubShellRunner(0, "hello\nworld", false, nil)
	t.Cleanup(func() { runCommandSeam = orig })

	s, _ := startedSession(t)
	err := s.RunShellCommand(context.Background(), "echo")
	require.NoError(t, err)

	turns := s.CapturePresentation()
	require.Len(t, turns, 1, "expected exactly one turn")
	turn := turns[0]
	kinds := make([]PresentationRowKind, len(turn.Rows))
	for i, r := range turn.Rows {
		kinds[i] = r.Kind
	}
	require.Equal(t, []PresentationRowKind{RowUser, RowResponse, RowProse, RowProse}, kinds)
	assert.Equal(t, "! echo", turn.Rows[0].Text)
	assert.Equal(t, "hello", turn.Rows[2].Text)
	assert.Equal(t, "world", turn.Rows[3].Text)
}

func TestRunShellCommand_NonZeroExitAndTruncatedAddsStatusRow(t *testing.T) {
	orig := runCommandSeam
	runCommandSeam = stubShellRunner(2, "bad output", true, nil)
	t.Cleanup(func() { runCommandSeam = orig })

	s, _ := startedSession(t)
	err := s.RunShellCommand(context.Background(), "bad-cmd")
	require.NoError(t, err)

	turns := s.CapturePresentation()
	require.Len(t, turns, 1)
	turn := turns[0]
	lastRow := turn.Rows[len(turn.Rows)-1]
	assert.Equal(t, RowStatus, lastRow.Kind)
	assert.Contains(t, lastRow.Text, "exit 2")
	assert.Contains(t, lastRow.Text, "truncated")
}

func TestRunShellCommand_DoesNotInterruptOpenAgentTurn(t *testing.T) {
	orig := runCommandSeam
	runCommandSeam = stubShellRunner(0, "ok", false, nil)
	t.Cleanup(func() { runCommandSeam = orig })

	s, mock := startedSession(t)

	// Seed an open agent turn via event.
	mock.events <- Event{Kind: EventTurnStarted, TurnID: "agent-turn-1"}
	mock.events <- Event{Kind: EventTextDelta, Text: "thinking..."}
	// Wait for goroutine to process events.
	require.Eventually(t, func() bool {
		turns := s.CapturePresentation()
		return len(turns) == 1
	}, time.Second, 5*time.Millisecond)

	err := s.RunShellCommand(context.Background(), "ls")
	require.NoError(t, err)

	turns := s.CapturePresentation()
	// First turn should be the agent turn (not interrupted), second is shell turn.
	require.GreaterOrEqual(t, len(turns), 2)
	agentTurn := turns[0]
	assert.False(t, agentTurn.Interrupted, "open agent turn must not be interrupted by RunShellCommand")
	assert.Equal(t, "agent-turn-1", agentTurn.ID)
}
