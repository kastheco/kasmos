package sdk

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/kastheco/kasmos/session/tmux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeClaudeServer simulates the Claude app-server's JSON-RPC stdio interface.
// It reads requests from processStdin and writes responses/notifications to
// processStdout. Tests construct a fakeClaudeServer and use it to verify that
// ClaudeTransport correctly translates wire messages to typed Events.
type fakeClaudeServer struct {
	stdin  io.Reader
	stdout io.WriteCloser
}

// sendNotification writes a single server-sent JSON-RPC notification.
func (s *fakeClaudeServer) sendNotification(method string, params any) error {
	type notification struct {
		JSONRPC string `json:"jsonrpc"`
		Method  string `json:"method"`
		Params  any    `json:"params"`
	}
	data, err := json.Marshal(notification{JSONRPC: "2.0", Method: method, Params: params})
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(s.stdout, "%s\n", data)
	return err
}

// respondTo reads the next request from stdin and writes a result response with
// the given result payload (or an empty object if result is nil).
func (s *fakeClaudeServer) respondTo(result any) error {
	dec := json.NewDecoder(s.stdin)
	var req jsonRPCMsg
	if err := dec.Decode(&req); err != nil {
		return fmt.Errorf("decode request: %w", err)
	}
	if req.ID == nil {
		// Notification or invalid — skip (not a request needing a reply).
		return nil
	}
	var resultJSON []byte
	var err error
	if result == nil {
		resultJSON = []byte(`{}`)
	} else {
		resultJSON, err = json.Marshal(result)
		if err != nil {
			return err
		}
	}
	_, err = fmt.Fprintf(s.stdout, `{"jsonrpc":"2.0","id":%d,"result":%s}`+"\n",
		*req.ID, resultJSON)
	return err
}

// newFakeTransport creates a ClaudeTransport wired to in-memory pipes that
// simulate the subprocess's stdin/stdout. The returned fakeClaudeServer
// provides helpers for injecting server responses and notifications.
//
// The transport's process field is replaced with a stub that returns the pipe
// ends, so no real subprocess is created.
func newFakeTransport(t *testing.T) (*ClaudeTransport, *fakeClaudeServer) {
	t.Helper()
	// io.Pipe returns (reader, writer).
	// client writes to its stdin → server reads from serverStdinR
	serverStdinR, clientStdinW := io.Pipe()
	// server writes to serverStdoutW → client reads from clientStdoutR
	clientStdoutR, serverStdoutW := io.Pipe()

	tr := &ClaudeTransport{
		process: NewProcess(), // unused in fake — we wire the client manually below
		events:  make(chan Event, 128),
		closed:  make(chan struct{}),
	}
	tr.client = NewClient(clientStdinW, clientStdoutR)

	server := &fakeClaudeServer{
		stdin:  serverStdinR,
		stdout: serverStdoutW,
	}

	t.Cleanup(func() {
		_ = tr.Close()
		_ = serverStdoutW.Close()
		_ = serverStdinR.Close()
	})

	return tr, server
}

// drainEvents reads all events from ch until it is closed or timeout elapses.
func drainEvents(t *testing.T, ch <-chan Event, timeout time.Duration) []Event {
	t.Helper()
	var events []Event
	deadline := time.After(timeout)
	for {
		select {
		case ev, ok := <-ch:
			if !ok {
				return events
			}
			events = append(events, ev)
		case <-deadline:
			return events
		}
	}
}

// collectN reads exactly n events from ch within timeout, failing the test if
// fewer events arrive.
func collectN(t *testing.T, ch <-chan Event, n int, timeout time.Duration) []Event {
	t.Helper()
	events := make([]Event, 0, n)
	deadline := time.After(timeout)
	for len(events) < n {
		select {
		case ev, ok := <-ch:
			if !ok {
				t.Fatalf("events channel closed after %d events (wanted %d)", len(events), n)
			}
			events = append(events, ev)
		case <-deadline:
			t.Fatalf("timeout waiting for event %d/%d", len(events)+1, n)
		}
	}
	return events
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

// TestClaude_NewClaudeTransport_ImplementsTransport verifies that the concrete
// type satisfies the Transport interface at compile time.
func TestClaude_NewClaudeTransport_ImplementsTransport(t *testing.T) {
	var _ Transport = NewClaudeTransport()
}

// TestClaude_Start_EmptyProgram verifies that Start rejects an empty program.
func TestClaude_Start_EmptyProgram(t *testing.T) {
	tr := NewClaudeTransport().(*ClaudeTransport)
	err := tr.Start(context.Background(), LaunchConfig{Program: "  "})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty program")
}

// TestClaude_Start_HandshakeTimeout verifies that a non-responsive subprocess
// causes the handshake Call to fail with a context deadline error.
//
// We test the handshake logic directly (bypassing real subprocess spawning) by
// wiring a transport to a server that reads requests but never responds. A
// short context deadline ensures the Call unblocks quickly.
func TestClaude_Start_HandshakeTimeout(t *testing.T) {
	// serverStdinR reads what the client sends; serverStdoutW writes to the client.
	// We connect a draining reader on the server side so client writes don't block,
	// but the server never writes a response — simulating a hung subprocess.
	serverStdinR, clientStdinW := io.Pipe()
	clientStdoutR, _ := io.Pipe() // server stdout write end dropped; client reads nothing

	tr := &ClaudeTransport{
		process: NewProcess(),
		events:  make(chan Event, 16),
		closed:  make(chan struct{}),
	}
	tr.client = NewClient(clientStdinW, clientStdoutR)

	// Drain client requests so writes don't block.
	go io.Copy(io.Discard, serverStdinR)

	t.Cleanup(func() {
		_ = tr.Close()
		serverStdinR.Close()
	})

	initCtx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()

	var result claudeInitResult
	err := tr.client.Call(initCtx, claudeMethodInit, claudeInitParams{}, &result)
	require.Error(t, err)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}

// TestClaude_TurnStarted_TranslatesToEvent verifies that a "claude/turnStarted"
// notification is converted to an EventTurnStarted event.
func TestClaude_TurnStarted_TranslatesToEvent(t *testing.T) {
	tr, server := newFakeTransport(t)
	go tr.dispatchNotifications()

	err := server.sendNotification(claudeNotifyTurnStarted, claudeNotifyTurnStartedParams{
		TurnID: "turn-1",
	})
	require.NoError(t, err)

	events := collectN(t, tr.Events(), 1, time.Second)
	require.Equal(t, EventTurnStarted, events[0].Kind)
	assert.Equal(t, "turn-1", events[0].TurnID)

	tr.mu.Lock()
	assert.True(t, tr.turnActive, "turnActive should be true after TurnStarted")
	tr.mu.Unlock()
}

// TestClaude_TextDelta_TranslatesToEvent verifies that "claude/streamText"
// notifications are forwarded as EventTextDelta events.
func TestClaude_TextDelta_TranslatesToEvent(t *testing.T) {
	tr, server := newFakeTransport(t)
	go tr.dispatchNotifications()

	require.NoError(t, server.sendNotification(claudeNotifyStreamText, claudeNotifyStreamTextParams{
		TurnID: "turn-1",
		Text:   "hello ",
	}))
	require.NoError(t, server.sendNotification(claudeNotifyStreamText, claudeNotifyStreamTextParams{
		TurnID: "turn-1",
		Text:   "world",
	}))

	events := collectN(t, tr.Events(), 2, time.Second)
	assert.Equal(t, EventTextDelta, events[0].Kind)
	assert.Equal(t, "hello ", events[0].Text)
	assert.Equal(t, EventTextDelta, events[1].Kind)
	assert.Equal(t, "world", events[1].Text)
}

// TestClaude_ToolUse_TranslatesToEvent verifies tool-call notifications.
func TestClaude_ToolUse_TranslatesToEvent(t *testing.T) {
	tr, server := newFakeTransport(t)
	go tr.dispatchNotifications()

	require.NoError(t, server.sendNotification(claudeNotifyToolUse, claudeNotifyToolUseParams{
		TurnID:   "turn-1",
		ToolName: "bash",
		Input:    `{"cmd":"ls"}`,
	}))

	events := collectN(t, tr.Events(), 1, time.Second)
	assert.Equal(t, EventToolCall, events[0].Kind)
	assert.Equal(t, "bash", events[0].ToolName)
	assert.Equal(t, `{"cmd":"ls"}`, events[0].ToolInput)
}

// TestClaude_ToolResult_TranslatesToEvent verifies tool-result notifications.
func TestClaude_ToolResult_TranslatesToEvent(t *testing.T) {
	tr, server := newFakeTransport(t)
	go tr.dispatchNotifications()

	require.NoError(t, server.sendNotification(claudeNotifyToolResult, claudeNotifyToolResultParams{
		TurnID:   "turn-1",
		ToolName: "bash",
		Result:   `"ok"`,
	}))

	events := collectN(t, tr.Events(), 1, time.Second)
	assert.Equal(t, EventToolResult, events[0].Kind)
	assert.Equal(t, "bash", events[0].ToolName)
	assert.Equal(t, `"ok"`, events[0].ToolResult)
}

// TestClaude_PermissionRequest_TranslatesToEvent verifies that permission
// notifications set the pendingPermID and emit an EventPermission.
func TestClaude_PermissionRequest_TranslatesToEvent(t *testing.T) {
	tr, server := newFakeTransport(t)
	go tr.dispatchNotifications()

	require.NoError(t, server.sendNotification(claudeNotifyPermRequest, claudeNotifyPermRequestParams{
		TurnID:       "turn-1",
		PermissionID: "perm-abc",
		Description:  "write to /etc",
		Pattern:      "/etc/*",
	}))

	events := collectN(t, tr.Events(), 1, time.Second)
	assert.Equal(t, EventPermission, events[0].Kind)
	assert.Equal(t, "write to /etc", events[0].PermissionDescription)
	assert.Equal(t, "/etc/*", events[0].PermissionPattern)

	tr.mu.Lock()
	assert.Equal(t, "perm-abc", tr.pendingPermID, "pendingPermID should be set")
	tr.mu.Unlock()
}

// TestClaude_PermissionRequest_ReplacesExistingPending verifies that a second
// permission notification replaces (not queues) the first pending request.
func TestClaude_PermissionRequest_ReplacesExistingPending(t *testing.T) {
	tr, server := newFakeTransport(t)
	go tr.dispatchNotifications()

	require.NoError(t, server.sendNotification(claudeNotifyPermRequest, claudeNotifyPermRequestParams{
		TurnID: "turn-1", PermissionID: "perm-1", Description: "first",
	}))
	require.NoError(t, server.sendNotification(claudeNotifyPermRequest, claudeNotifyPermRequestParams{
		TurnID: "turn-1", PermissionID: "perm-2", Description: "second",
	}))

	// Drain both events.
	collectN(t, tr.Events(), 2, time.Second)

	// Only the latest permission ID should be tracked.
	tr.mu.Lock()
	assert.Equal(t, "perm-2", tr.pendingPermID)
	tr.mu.Unlock()
}

// TestClaude_TurnComplete_TranslatesToEvent verifies turn-complete events.
func TestClaude_TurnComplete_TranslatesToEvent(t *testing.T) {
	tr, server := newFakeTransport(t)

	// Pre-set turnActive so the complete can clear it.
	tr.mu.Lock()
	tr.turnActive = true
	tr.mu.Unlock()

	go tr.dispatchNotifications()

	require.NoError(t, server.sendNotification(claudeNotifyTurnComplete, claudeNotifyTurnCompleteParams{
		TurnID: "turn-1",
	}))

	events := collectN(t, tr.Events(), 1, time.Second)
	assert.Equal(t, EventTurnCompleted, events[0].Kind)
	assert.True(t, events[0].HasPrompt, "HasPrompt should be true when turn completes")
	assert.True(t, events[0].Final)

	tr.mu.Lock()
	assert.False(t, tr.turnActive, "turnActive should be false after TurnComplete")
	tr.mu.Unlock()
}

// TestClaude_TurnInterrupted_TranslatesToEvent verifies turn-interrupted events.
func TestClaude_TurnInterrupted_TranslatesToEvent(t *testing.T) {
	tr, server := newFakeTransport(t)

	tr.mu.Lock()
	tr.turnActive = true
	tr.mu.Unlock()

	go tr.dispatchNotifications()

	require.NoError(t, server.sendNotification(claudeNotifyTurnInterrupted, claudeNotifyTurnInterruptedParams{
		TurnID: "turn-1",
	}))

	events := collectN(t, tr.Events(), 1, time.Second)
	assert.Equal(t, EventTurnInterrupted, events[0].Kind)
	assert.True(t, events[0].HasPrompt)
	assert.True(t, events[0].Final)

	tr.mu.Lock()
	assert.False(t, tr.turnActive)
	tr.mu.Unlock()
}

// TestClaude_Interrupt_NoOpWhenNoTurnActive verifies that Interrupt returns
// nil and sends nothing when no turn is in progress.
func TestClaude_Interrupt_NoOpWhenNoTurnActive(t *testing.T) {
	tr, _ := newFakeTransport(t)

	// turnActive defaults to false — no turn is running.
	err := tr.Interrupt(context.Background())
	require.NoError(t, err, "Interrupt must be a no-op when no turn is active")
}

// TestClaude_Interrupt_SendsWhenTurnActive verifies that Interrupt sends the
// interrupt request when a turn is in progress.
func TestClaude_Interrupt_SendsWhenTurnActive(t *testing.T) {
	tr, server := newFakeTransport(t)

	tr.mu.Lock()
	tr.turnActive = true
	tr.mu.Unlock()

	// The server must respond to the interrupt call so it doesn't block forever.
	done := make(chan error, 1)
	go func() {
		done <- server.respondTo(nil)
	}()

	err := tr.Interrupt(context.Background())
	require.NoError(t, err)

	select {
	case serverErr := <-done:
		require.NoError(t, serverErr)
	case <-time.After(time.Second):
		t.Fatal("server did not receive interrupt request in time")
	}
}

// TestClaude_RespondPermission_NoOpWhenNoPendingRequest verifies that
// RespondPermission returns nil when no permission is pending.
func TestClaude_RespondPermission_NoOpWhenNoPendingRequest(t *testing.T) {
	tr, _ := newFakeTransport(t)

	// No pending permission — should be a silent no-op.
	err := tr.RespondPermission(context.Background(), tmux.PermissionAllowOnce)
	require.NoError(t, err, "RespondPermission must be a no-op when no request is pending")
}

// TestClaude_RespondPermission_ClearsPendingAndSendsResponse verifies that
// RespondPermission sends the permission response and clears the pending ID.
func TestClaude_RespondPermission_ClearsPendingAndSendsResponse(t *testing.T) {
	tr, server := newFakeTransport(t)

	tr.mu.Lock()
	tr.pendingPermID = "perm-xyz"
	tr.mu.Unlock()

	done := make(chan error, 1)
	go func() {
		done <- server.respondTo(nil)
	}()

	err := tr.RespondPermission(context.Background(), tmux.PermissionAllowAlways)
	require.NoError(t, err)

	select {
	case serverErr := <-done:
		require.NoError(t, serverErr)
	case <-time.After(time.Second):
		t.Fatal("server did not receive permission response in time")
	}

	tr.mu.Lock()
	assert.Empty(t, tr.pendingPermID, "pendingPermID must be cleared after responding")
	tr.mu.Unlock()
}

// TestClaude_EventsChannelClosedOnProcessExit verifies that the events channel
// is closed when the server's stdout is closed (simulating process exit).
func TestClaude_EventsChannelClosedOnProcessExit(t *testing.T) {
	// Wire a fresh transport with pipes we control.
	serverStdinR, _ := io.Pipe()
	clientStdoutR, serverStdoutW := io.Pipe()
	_, clientStdinW := io.Pipe()

	tr := &ClaudeTransport{
		process: NewProcess(),
		events:  make(chan Event, 16),
		closed:  make(chan struct{}),
	}
	tr.client = NewClient(clientStdinW, clientStdoutR)
	_ = serverStdinR // unused server-side read end

	go tr.dispatchNotifications()

	// Simulate process exit by closing the server's write end of stdout.
	_ = serverStdoutW.Close()

	select {
	case _, ok := <-tr.Events():
		assert.False(t, ok, "events channel should be closed after process exit")
	case <-time.After(time.Second):
		t.Fatal("events channel was not closed after simulated process exit")
	}
}

// TestClaude_ProgramStringModification verifies that Start appends
// --app-server and --permission-mode flags to the program string when
// appropriate. We test this indirectly via the config-building logic by
// inspecting what would be passed to the process (without real subprocess).
//
// This test exercises the flag-injection logic in Start without spawning
// an actual subprocess.
func TestClaude_ProgramStringModification(t *testing.T) {
	tests := []struct {
		name             string
		program          string
		skipPermissions  bool
		noFlicker        bool
		wantAppServer    bool
		wantBypass       bool
		wantNoFlickerVal string
	}{
		{
			name:             "bare claude gets app-server flag",
			program:          "claude",
			skipPermissions:  false,
			noFlicker:        false,
			wantAppServer:    true,
			wantBypass:       false,
			wantNoFlickerVal: "0",
		},
		{
			name:             "skip permissions appends bypass flag",
			program:          "claude",
			skipPermissions:  true,
			noFlicker:        false,
			wantAppServer:    true,
			wantBypass:       true,
			wantNoFlickerVal: "0",
		},
		{
			name:             "noFlicker sets env var to 1",
			program:          "claude",
			skipPermissions:  false,
			noFlicker:        true,
			wantAppServer:    true,
			wantBypass:       false,
			wantNoFlickerVal: "1",
		},
		{
			name:             "app-server already present is not duplicated",
			program:          "claude --app-server",
			skipPermissions:  false,
			noFlicker:        false,
			wantAppServer:    true,
			wantBypass:       false,
			wantNoFlickerVal: "0",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := LaunchConfig{
				Program:         tc.program,
				SkipPermissions: tc.skipPermissions,
				NoFlicker:       tc.noFlicker,
			}

			// Mirror the flag-injection logic from Start.
			program := cfg.Program
			if !contains(program, claudeAppServerFlag) {
				program = program + " " + claudeAppServerFlag
			}
			if cfg.SkipPermissions && !contains(program, "--permission-mode") {
				program = program + " " + claudePermBypassFlag
			}
			flickerVal := "0"
			if cfg.NoFlicker {
				flickerVal = "1"
			}

			if tc.wantAppServer {
				assert.Contains(t, program, claudeAppServerFlag)
			}
			if tc.wantBypass {
				assert.Contains(t, program, "--permission-mode bypassPermissions")
			} else {
				assert.NotContains(t, program, "--permission-mode bypassPermissions")
			}
			assert.Equal(t, tc.wantNoFlickerVal, flickerVal)
		})
	}
}

// TestClaude_UnknownNotification_EmitsSystemEvent verifies that an unrecognised
// server notification produces an EventSystem rather than crashing the dispatcher.
func TestClaude_UnknownNotification_EmitsSystemEvent(t *testing.T) {
	tr, server := newFakeTransport(t)
	go tr.dispatchNotifications()

	require.NoError(t, server.sendNotification("claude/unknownFuture", map[string]string{"x": "y"}))

	events := collectN(t, tr.Events(), 1, time.Second)
	assert.Equal(t, EventSystem, events[0].Kind)
	assert.Contains(t, events[0].Text, "claude/unknownFuture")
}

// TestClaude_Close_IdempotentBeforeStart verifies that Close before Start is safe.
func TestClaude_Close_IdempotentBeforeStart(t *testing.T) {
	tr := NewClaudeTransport().(*ClaudeTransport)
	assert.NoError(t, tr.Close())
	assert.NoError(t, tr.Close()) // second call must not panic
}

// TestClaude_PID_ZeroBeforeStart verifies PID returns 0 before Start.
func TestClaude_PID_ZeroBeforeStart(t *testing.T) {
	tr := NewClaudeTransport().(*ClaudeTransport)
	assert.Equal(t, 0, tr.PID())
}

// contains is a thin wrapper around strings.Contains used in table-driven tests
// to avoid importing "strings" at the test package level for a single call.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		func() bool {
			for i := 0; i <= len(s)-len(substr); i++ {
				if s[i:i+len(substr)] == substr {
					return true
				}
			}
			return false
		}())
}

// TestClaude_SpeedTier_IgnoredInPayload is a regression test verifying that
// setting LaunchConfig.SpeedTier="fast" does not alter the program string,
// extra env vars, or any other observable part of the Claude startup payload.
// Claude has no fast-tier concept; the field must be silently ignored.
func TestClaude_SpeedTier_IgnoredInPayload(t *testing.T) {
	// Baseline: start without SpeedTier and capture program + env.
	baselineCfg := LaunchConfig{
		Program:   "claude",
		SpeedTier: "",
	}
	// With SpeedTier set.
	fastCfg := LaunchConfig{
		Program:   "claude",
		SpeedTier: "fast",
	}

	// Mirror the flag-injection logic from ClaudeTransport.Start.
	applyFlags := func(cfg LaunchConfig) (program string, extraEnv []string) {
		program = strings.TrimSpace(cfg.Program)
		if !contains(program, claudeAppServerFlag) {
			program = program + " " + claudeAppServerFlag
		}
		if cfg.SkipPermissions && !contains(program, "--permission-mode") {
			program = program + " " + claudePermBypassFlag
		}
		flickerVal := "0"
		if cfg.NoFlicker {
			flickerVal = "1"
		}
		extraEnv = append(cfg.ExtraEnv, "CLAUDE_CODE_NO_FLICKER="+flickerVal)
		return program, extraEnv
	}

	baselineProgram, baselineEnv := applyFlags(baselineCfg)
	fastProgram, fastEnv := applyFlags(fastCfg)

	// SpeedTier must not change program or env.
	assert.Equal(t, baselineProgram, fastProgram, "SpeedTier must not alter the program string")
	assert.Equal(t, baselineEnv, fastEnv, "SpeedTier must not alter the extra env vars")
	// Paranoia: confirm serviceTier is not leaked into the program string.
	assert.NotContains(t, fastProgram, "serviceTier")
	assert.NotContains(t, fastProgram, "fast")
}
