package sdk

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/kastheco/kasmos/session/tmux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeCodexServer simulates the Codex app-server over a pair of in-memory
// pipes. It reads JSON-RPC requests from its "stdin" and can push
// notifications to the client via pushNotification.
type fakeCodexServer struct {
	t        *testing.T
	stdin    io.Reader // requests arrive here
	stdout   io.Writer // notifications are written here
	mu       sync.Mutex
	requests []jsonRPCMsg
}

func newFakeCodexServer(t *testing.T, stdin io.Reader, stdout io.Writer) *fakeCodexServer {
	t.Helper()
	s := &fakeCodexServer{t: t, stdin: stdin, stdout: stdout}
	go s.readLoop()
	return s
}

// readLoop drains incoming requests so the client's write calls don't block.
func (s *fakeCodexServer) readLoop() {
	dec := json.NewDecoder(s.stdin)
	for {
		var msg jsonRPCMsg
		if err := dec.Decode(&msg); err != nil {
			return
		}
		s.mu.Lock()
		s.requests = append(s.requests, msg)
		s.mu.Unlock()

		// Acknowledge request-response calls with an empty result so the
		// client's Call unblocks.
		if msg.ID != nil {
			resp := fmt.Sprintf(`{"jsonrpc":"2.0","id":%d,"result":{}}`+"\n", *msg.ID)
			_, _ = io.WriteString(s.stdout, resp)
		}
	}
}

// pushNotification writes a server-sent notification to the client's stdout.
func (s *fakeCodexServer) pushNotification(method string, params any) {
	s.t.Helper()
	data, err := json.Marshal(params)
	if err != nil {
		s.t.Fatalf("pushNotification: marshal params: %v", err)
	}
	line := fmt.Sprintf(`{"jsonrpc":"2.0","method":%q,"params":%s}`+"\n", method, data)
	_, _ = io.WriteString(s.stdout, line)
}

// lastRequest returns the most recently received request, or panics if none.
func (s *fakeCodexServer) lastRequest() jsonRPCMsg {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.requests) == 0 {
		s.t.Fatal("no requests received by fake server")
	}
	return s.requests[len(s.requests)-1]
}

// collectRequest returns the most recently received request method.
func (s *fakeCodexServer) lastMethod() string {
	return s.lastRequest().Method
}

// newCodexTransportWithFakeServer creates a CodexTransport wired to a
// fakeCodexServer via in-memory pipes and calls Start on it.
// Returns the transport and the fake server.
func newCodexTransportWithFakeServer(t *testing.T) (*CodexTransport, *fakeCodexServer, io.Closer) {
	t.Helper()

	// client-side stdin/stdout ↔ server-side stdin/stdout
	clientStdinR, clientStdinW := io.Pipe()   // transport writes → server reads
	serverStdoutR, serverStdoutW := io.Pipe() // server writes → transport reads

	srv := newFakeCodexServer(t, clientStdinR, serverStdoutW)

	ct := &CodexTransport{
		process: NewProcess(),
		events:  make(chan Event, 128),
		closed:  make(chan struct{}),
	}
	ct.client = NewClient(clientStdinW, serverStdoutR)
	go ct.dispatchNotifications()

	closer := &multiCloser{closers: []io.Closer{
		clientStdinW,
		serverStdoutW,
	}}

	t.Cleanup(func() {
		_ = ct.Close()
		_ = closer.Close()
	})

	return ct, srv, closer
}

// multiCloser closes multiple io.Closers in order.
type multiCloser struct {
	closers []io.Closer
}

func (m *multiCloser) Close() error {
	for _, c := range m.closers {
		_ = c.Close()
	}
	return nil
}

// collectEvents drains all events from ch until it is closed or the deadline
// elapses.
func collectEvents(t *testing.T, ch <-chan Event, timeout time.Duration) []Event {
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

// waitForEvent blocks until an event of the given kind arrives on ch or the
// deadline elapses.
func waitForEvent(t *testing.T, ch <-chan Event, kind EventKind, timeout time.Duration) Event {
	t.Helper()
	deadline := time.After(timeout)
	for {
		select {
		case ev, ok := <-ch:
			if !ok {
				t.Fatalf("event channel closed before %q event arrived", kind)
			}
			if ev.Kind == kind {
				return ev
			}
		case <-deadline:
			t.Fatalf("timed out waiting for %q event", kind)
		}
	}
}

// ---------------------------------------------------------------------------
// Construction
// ---------------------------------------------------------------------------

func TestCodexTransport_NewCodexTransport_ImplementsTransport(t *testing.T) {
	var _ Transport = NewCodexTransport()
}

func TestCodexTransport_PID_ZeroBeforeStart(t *testing.T) {
	tr := NewCodexTransport().(*CodexTransport)
	assert.Equal(t, 0, tr.PID())
}

// ---------------------------------------------------------------------------
// Start validation
// ---------------------------------------------------------------------------

func TestCodexTransport_Start_EmptyProgramReturnsError(t *testing.T) {
	tr := NewCodexTransport()
	err := tr.Start(context.Background(), LaunchConfig{Program: ""})
	require.Error(t, err)
	assert.Contains(t, strings.ToLower(err.Error()), "empty program")
}

func TestCodexTransport_Start_WhitespaceProgramReturnsError(t *testing.T) {
	tr := NewCodexTransport()
	err := tr.Start(context.Background(), LaunchConfig{Program: "   \t  "})
	require.Error(t, err)
	assert.Contains(t, strings.ToLower(err.Error()), "empty program")
}

// ---------------------------------------------------------------------------
// SendPrompt
// ---------------------------------------------------------------------------

func TestCodexTransport_SendPrompt_SendsCorrectMethod(t *testing.T) {
	ct, srv, _ := newCodexTransportWithFakeServer(t)

	err := ct.SendPrompt(context.Background(), "hello world")
	require.NoError(t, err)

	// Give the server a moment to receive the request.
	time.Sleep(10 * time.Millisecond)
	assert.Equal(t, codexMethodTurnInput, srv.lastMethod())
}

func TestCodexTransport_SendPrompt_EmptyIgnored(t *testing.T) {
	ct, srv, _ := newCodexTransportWithFakeServer(t)

	err := ct.SendPrompt(context.Background(), "")
	require.NoError(t, err)

	err = ct.SendPrompt(context.Background(), "   ")
	require.NoError(t, err)

	// No requests should have been sent.
	time.Sleep(10 * time.Millisecond)
	srv.mu.Lock()
	count := len(srv.requests)
	srv.mu.Unlock()
	assert.Equal(t, 0, count, "empty/whitespace prompts must not produce requests")
}

func TestCodexTransport_SendPrompt_TextInPayload(t *testing.T) {
	ct, srv, _ := newCodexTransportWithFakeServer(t)

	const wantText = "write a hello world program"
	err := ct.SendPrompt(context.Background(), wantText)
	require.NoError(t, err)

	time.Sleep(10 * time.Millisecond)
	req := srv.lastRequest()
	var params codexTurnInputParams
	require.NoError(t, json.Unmarshal(req.Params, &params))
	assert.Equal(t, wantText, params.Text)
}

// ---------------------------------------------------------------------------
// Interrupt
// ---------------------------------------------------------------------------

func TestCodexTransport_Interrupt_SendsCorrectMethod(t *testing.T) {
	ct, srv, _ := newCodexTransportWithFakeServer(t)

	err := ct.Interrupt(context.Background())
	require.NoError(t, err)

	time.Sleep(10 * time.Millisecond)
	assert.Equal(t, codexMethodTurnInterrupt, srv.lastMethod())
}

func TestCodexTransport_Interrupt_NoopAfterClose(t *testing.T) {
	ct, _, _ := newCodexTransportWithFakeServer(t)
	_ = ct.Close()

	// Must not panic; error is ignored by contract.
	err := ct.Interrupt(context.Background())
	assert.NoError(t, err, "Interrupt after Close must not panic or error")
}

// ---------------------------------------------------------------------------
// RespondPermission
// ---------------------------------------------------------------------------

func TestCodexTransport_RespondPermission_AllowOnce(t *testing.T) {
	ct, srv, _ := newCodexTransportWithFakeServer(t)

	err := ct.RespondPermission(context.Background(), tmux.PermissionAllowOnce)
	require.NoError(t, err)

	time.Sleep(10 * time.Millisecond)
	req := srv.lastRequest()
	assert.Equal(t, codexMethodPermissionRespond, req.Method)
	var params codexPermissionRespondParams
	require.NoError(t, json.Unmarshal(req.Params, &params))
	assert.Equal(t, "allow_once", params.Response)
}

func TestCodexTransport_RespondPermission_AllowAlways(t *testing.T) {
	ct, srv, _ := newCodexTransportWithFakeServer(t)

	err := ct.RespondPermission(context.Background(), tmux.PermissionAllowAlways)
	require.NoError(t, err)

	time.Sleep(10 * time.Millisecond)
	var params codexPermissionRespondParams
	require.NoError(t, json.Unmarshal(srv.lastRequest().Params, &params))
	assert.Equal(t, "allow_always", params.Response)
}

func TestCodexTransport_RespondPermission_Reject(t *testing.T) {
	ct, srv, _ := newCodexTransportWithFakeServer(t)

	err := ct.RespondPermission(context.Background(), tmux.PermissionReject)
	require.NoError(t, err)

	time.Sleep(10 * time.Millisecond)
	var params codexPermissionRespondParams
	require.NoError(t, json.Unmarshal(srv.lastRequest().Params, &params))
	assert.Equal(t, "deny", params.Response)
}

func TestCodexTransport_RespondPermission_NoopAfterClose(t *testing.T) {
	ct, _, _ := newCodexTransportWithFakeServer(t)
	_ = ct.Close()

	// Must not panic.
	err := ct.RespondPermission(context.Background(), tmux.PermissionAllowOnce)
	assert.NoError(t, err, "RespondPermission after Close must not panic or error")
}

// ---------------------------------------------------------------------------
// Notification → Event translation
// ---------------------------------------------------------------------------

func TestCodexTransport_TurnStarted_Event(t *testing.T) {
	ct, srv, _ := newCodexTransportWithFakeServer(t)

	srv.pushNotification(codexNotifyTurnStarted, codexNotifyTurnStartedParams{TurnID: "t1"})

	ev := waitForEvent(t, ct.Events(), EventTurnStarted, time.Second)
	assert.Equal(t, "t1", ev.TurnID)
	assert.False(t, ev.HasPrompt, "turn started should not set HasPrompt")
}

func TestCodexTransport_MessageDelta_Event(t *testing.T) {
	ct, srv, _ := newCodexTransportWithFakeServer(t)

	srv.pushNotification(codexNotifyMessageDelta, codexNotifyMessageDeltaParams{
		TurnID: "t1", Text: "hello",
	})

	ev := waitForEvent(t, ct.Events(), EventTextDelta, time.Second)
	assert.Equal(t, "t1", ev.TurnID)
	assert.Equal(t, "hello", ev.Text)
}

func TestCodexTransport_ToolCall_Event(t *testing.T) {
	ct, srv, _ := newCodexTransportWithFakeServer(t)

	srv.pushNotification(codexNotifyToolCall, codexNotifyToolCallParams{
		TurnID: "t1", ToolName: "shell", Input: `{"cmd":"ls"}`,
	})

	ev := waitForEvent(t, ct.Events(), EventToolCall, time.Second)
	assert.Equal(t, "shell", ev.ToolName)
	assert.Equal(t, `{"cmd":"ls"}`, ev.ToolInput)
}

func TestCodexTransport_ToolResult_Event(t *testing.T) {
	ct, srv, _ := newCodexTransportWithFakeServer(t)

	srv.pushNotification(codexNotifyToolResult, codexNotifyToolResultParams{
		TurnID: "t1", ToolName: "shell", Result: `{"out":"file.go"}`,
	})

	ev := waitForEvent(t, ct.Events(), EventToolResult, time.Second)
	assert.Equal(t, "shell", ev.ToolName)
	assert.Equal(t, `{"out":"file.go"}`, ev.ToolResult)
}

func TestCodexTransport_Permission_Event(t *testing.T) {
	ct, srv, _ := newCodexTransportWithFakeServer(t)

	srv.pushNotification(codexNotifyPermission, codexNotifyPermissionParams{
		TurnID: "t1", Description: "write /etc/hosts", Pattern: "/etc/*",
	})

	ev := waitForEvent(t, ct.Events(), EventPermission, time.Second)
	assert.Equal(t, "write /etc/hosts", ev.PermissionDescription)
	assert.Equal(t, "/etc/*", ev.PermissionPattern)
}

func TestCodexTransport_TurnComplete_HasPrompt(t *testing.T) {
	ct, srv, _ := newCodexTransportWithFakeServer(t)

	srv.pushNotification(codexNotifyTurnComplete, codexNotifyTurnCompleteParams{TurnID: "t1"})

	ev := waitForEvent(t, ct.Events(), EventTurnCompleted, time.Second)
	assert.True(t, ev.HasPrompt, "turn/complete must set HasPrompt=true so session layer detects readiness")
	assert.True(t, ev.Final)
}

func TestCodexTransport_TurnInterrupted_HasPrompt(t *testing.T) {
	ct, srv, _ := newCodexTransportWithFakeServer(t)

	srv.pushNotification(codexNotifyTurnInterrupted, codexNotifyTurnInterruptedParams{TurnID: "t1"})

	ev := waitForEvent(t, ct.Events(), EventTurnInterrupted, time.Second)
	assert.True(t, ev.HasPrompt, "turn/interrupted must set HasPrompt=true")
	assert.True(t, ev.Final)
}

// ---------------------------------------------------------------------------
// Edge cases
// ---------------------------------------------------------------------------

func TestCodexTransport_UnknownNotification_EmitsSystemEvent(t *testing.T) {
	ct, srv, _ := newCodexTransportWithFakeServer(t)

	// Push a notification the transport does not recognise.
	srv.pushNotification("unknown/method", map[string]any{"x": 1})

	ev := waitForEvent(t, ct.Events(), EventSystem, time.Second)
	assert.Contains(t, ev.Text, "unknown/method")
}

func TestCodexTransport_EventsChannelClosedAfterProcessExit(t *testing.T) {
	ct, _, closer := newCodexTransportWithFakeServer(t)

	// Close the server-stdout pipe to simulate process exit.
	_ = closer.Close()

	// The events channel must be closed, not block forever.
	select {
	case _, ok := <-ct.Events():
		if ok {
			// Drain any buffered events until closed.
			for range ct.Events() {
			}
		}
		// Channel eventually closed — pass.
	case <-time.After(2 * time.Second):
		t.Fatal("event channel not closed after process exit")
	}
}

func TestCodexTransport_Close_Idempotent(t *testing.T) {
	ct, _, _ := newCodexTransportWithFakeServer(t)

	// Multiple Close calls must not panic.
	assert.NoError(t, ct.Close())
	assert.NoError(t, ct.Close())
	assert.NoError(t, ct.Close())
}

func TestCodexTransport_MultipleNotifications_AllDelivered(t *testing.T) {
	ct, srv, _ := newCodexTransportWithFakeServer(t)

	srv.pushNotification(codexNotifyTurnStarted, codexNotifyTurnStartedParams{TurnID: "t2"})
	srv.pushNotification(codexNotifyMessageDelta, codexNotifyMessageDeltaParams{TurnID: "t2", Text: "chunk1"})
	srv.pushNotification(codexNotifyMessageDelta, codexNotifyMessageDeltaParams{TurnID: "t2", Text: "chunk2"})
	srv.pushNotification(codexNotifyTurnComplete, codexNotifyTurnCompleteParams{TurnID: "t2"})

	events := collectEvents(t, ct.Events(), 200*time.Millisecond)

	kinds := make([]EventKind, 0, len(events))
	for _, ev := range events {
		kinds = append(kinds, ev.Kind)
	}

	assert.Contains(t, kinds, EventTurnStarted)
	assert.Contains(t, kinds, EventTextDelta)
	assert.Contains(t, kinds, EventTurnCompleted)

	// The two text-delta events must both be present.
	var deltas []string
	for _, ev := range events {
		if ev.Kind == EventTextDelta {
			deltas = append(deltas, ev.Text)
		}
	}
	assert.ElementsMatch(t, []string{"chunk1", "chunk2"}, deltas)
}
