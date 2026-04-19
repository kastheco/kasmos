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

type fakeCodexProcess struct {
	t                  *testing.T
	server             *fakeCodexServer
	startCfg           LaunchConfig
	stdinW             *io.PipeWriter
	stdoutR            *io.PipeReader
	stdoutW            *io.PipeWriter
	started            bool
	closeStdoutOnStart bool
	closeOnce          sync.Once
}

func newFakeCodexProcess(t *testing.T) *fakeCodexProcess {
	t.Helper()
	return &fakeCodexProcess{t: t}
}

func (p *fakeCodexProcess) Start(cfg LaunchConfig) (io.WriteCloser, io.ReadCloser, error) {
	p.startCfg = cfg
	clientStdinR, clientStdinW := io.Pipe()
	serverStdoutR, serverStdoutW := io.Pipe()
	p.stdinW = clientStdinW
	p.stdoutR = serverStdoutR
	p.stdoutW = serverStdoutW
	p.started = true

	if !p.closeStdoutOnStart {
		p.server = newFakeCodexServer(p.t, clientStdinR, serverStdoutW)
	} else {
		_ = serverStdoutW.Close()
	}

	return clientStdinW, serverStdoutR, nil
}

func (p *fakeCodexProcess) PID() int {
	if !p.started {
		return 0
	}
	return 4242
}

func (p *fakeCodexProcess) Close() error {
	p.closeOnce.Do(func() {
		if p.stdinW != nil {
			_ = p.stdinW.Close()
		}
		if p.stdoutW != nil {
			_ = p.stdoutW.Close()
		}
		if p.stdoutR != nil {
			_ = p.stdoutR.Close()
		}
	})
	return nil
}

type fakeCodexServer struct {
	t *testing.T

	stdin  io.Reader
	stdout io.Writer

	mu                  sync.Mutex
	requests            []jsonRPCMsg
	clientNotifications []jsonRPCMsg
	responses           []jsonRPCMsg
	nextTurnID          int
	threadID            string
}

func newFakeCodexServer(t *testing.T, stdin io.Reader, stdout io.Writer) *fakeCodexServer {
	t.Helper()
	s := &fakeCodexServer{
		t:        t,
		stdin:    stdin,
		stdout:   stdout,
		threadID: "thread-1",
	}
	go s.readLoop()
	return s
}

func (s *fakeCodexServer) readLoop() {
	dec := json.NewDecoder(s.stdin)
	for {
		var msg jsonRPCMsg
		if err := dec.Decode(&msg); err != nil {
			return
		}

		s.mu.Lock()
		switch {
		case msg.Method != "" && msg.ID != nil:
			s.requests = append(s.requests, msg)
		case msg.Method != "":
			s.clientNotifications = append(s.clientNotifications, msg)
		case msg.ID != nil:
			s.responses = append(s.responses, msg)
		}
		s.mu.Unlock()

		if msg.Method != "" && msg.ID != nil {
			s.replyToRequest(msg)
		}
	}
}

func (s *fakeCodexServer) replyToRequest(msg jsonRPCMsg) {
	s.t.Helper()

	var result any = map[string]any{}
	switch msg.Method {
	case codexMethodInitialize:
		result = map[string]any{"userAgent": "kasmos-test"}
	case codexMethodThreadStart:
		result = map[string]any{
			"thread": map[string]any{"id": s.threadID},
		}
	case codexMethodTurnStart:
		s.mu.Lock()
		s.nextTurnID++
		turnID := fmt.Sprintf("turn-%d", s.nextTurnID)
		s.mu.Unlock()
		result = map[string]any{
			"turn": map[string]any{"id": turnID},
		}
	case codexMethodTurnInterrupt:
		result = map[string]any{}
	default:
		result = map[string]any{}
	}

	resp := map[string]any{
		"jsonrpc": "2.0",
		"id":      *msg.ID,
		"result":  result,
	}
	data, err := json.Marshal(resp)
	require.NoError(s.t, err)
	_, _ = io.WriteString(s.stdout, string(data)+"\n")
}

func (s *fakeCodexServer) pushNotification(method string, params any) {
	s.t.Helper()
	msg := map[string]any{
		"jsonrpc": "2.0",
		"method":  method,
		"params":  params,
	}
	data, err := json.Marshal(msg)
	require.NoError(s.t, err)
	_, _ = io.WriteString(s.stdout, string(data)+"\n")
}

func (s *fakeCodexServer) pushRequest(id int64, method string, params any) {
	s.t.Helper()
	msg := map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  method,
		"params":  params,
	}
	data, err := json.Marshal(msg)
	require.NoError(s.t, err)
	_, _ = io.WriteString(s.stdout, string(data)+"\n")
}

func (s *fakeCodexServer) waitForRequests(t *testing.T, n int) {
	t.Helper()
	require.Eventually(t, func() bool {
		s.mu.Lock()
		defer s.mu.Unlock()
		return len(s.requests) >= n
	}, time.Second, 10*time.Millisecond)
}

func (s *fakeCodexServer) waitForResponses(t *testing.T, n int) {
	t.Helper()
	require.Eventually(t, func() bool {
		s.mu.Lock()
		defer s.mu.Unlock()
		return len(s.responses) >= n
	}, time.Second, 10*time.Millisecond)
}

func (s *fakeCodexServer) requestMethods() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]string, 0, len(s.requests))
	for _, req := range s.requests {
		out = append(out, req.Method)
	}
	return out
}

func (s *fakeCodexServer) lastRequest() jsonRPCMsg {
	s.mu.Lock()
	defer s.mu.Unlock()
	require.NotZero(s.t, len(s.requests))
	return s.requests[len(s.requests)-1]
}

func (s *fakeCodexServer) lastResponse() jsonRPCMsg {
	s.mu.Lock()
	defer s.mu.Unlock()
	require.NotZero(s.t, len(s.responses))
	return s.responses[len(s.responses)-1]
}

func (s *fakeCodexServer) hasClientNotification(method string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, msg := range s.clientNotifications {
		if msg.Method == method {
			return true
		}
	}
	return false
}

func newStartedCodexTransport(t *testing.T) (*CodexTransport, *fakeCodexServer) {
	t.Helper()
	proc := newFakeCodexProcess(t)
	ct := &CodexTransport{
		process: proc,
		events:  make(chan Event, 128),
		closed:  make(chan struct{}),
	}
	require.NoError(t, ct.Start(context.Background(), LaunchConfig{
		Program: "codex --model gpt-5.4",
		WorkDir: t.TempDir(),
	}))
	t.Cleanup(func() { _ = ct.Close() })
	require.NotNil(t, proc.server)
	return ct, proc.server
}

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

func TestCodexTransport_NewCodexTransport_ImplementsTransport(t *testing.T) {
	var _ Transport = NewCodexTransport()
}

func TestCodexTransport_PID_ZeroBeforeStart(t *testing.T) {
	tr := NewCodexTransport().(*CodexTransport)
	assert.Equal(t, 0, tr.PID())
}

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

func TestCodexTransport_Start_UsesV2HandshakeAndInitialPrompt(t *testing.T) {
	proc := newFakeCodexProcess(t)
	ct := &CodexTransport{
		process: proc,
		events:  make(chan Event, 128),
		closed:  make(chan struct{}),
	}
	t.Cleanup(func() { _ = ct.Close() })

	err := ct.Start(context.Background(), LaunchConfig{
		Program:         "codex --model gpt-5.4",
		WorkDir:         t.TempDir(),
		SkipPermissions: true,
		InitialPrompt:   "expand the plan",
	})
	require.NoError(t, err)
	require.NotNil(t, proc.server)

	proc.server.waitForRequests(t, 3)
	assert.Contains(t, proc.startCfg.Program, "app-server")
	assert.Equal(t, []string{
		codexMethodInitialize,
		codexMethodThreadStart,
		codexMethodTurnStart,
	}, proc.server.requestMethods())
	assert.True(t, proc.server.hasClientNotification(codexMethodInitialized))

	req := proc.server.lastRequest()
	var params codexTurnStartParams
	require.NoError(t, json.Unmarshal(req.Params, &params))
	assert.Equal(t, "thread-1", params.ThreadID)
	require.Len(t, params.Input, 1)
	assert.Equal(t, "text", params.Input[0].Type)
	assert.Equal(t, "expand the plan", params.Input[0].Text)
}

func TestCodexTransport_Start_ProcessExitDuringInitializeReturnsError(t *testing.T) {
	proc := newFakeCodexProcess(t)
	proc.closeStdoutOnStart = true
	ct := &CodexTransport{
		process: proc,
		events:  make(chan Event, 128),
		closed:  make(chan struct{}),
	}

	err := ct.Start(context.Background(), LaunchConfig{
		Program: "codex",
		WorkDir: t.TempDir(),
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "initialize")
}

func TestCodexTransport_SendPrompt_UsesTurnStart(t *testing.T) {
	ct, srv := newStartedCodexTransport(t)

	err := ct.SendPrompt(context.Background(), "hello world")
	require.NoError(t, err)

	srv.waitForRequests(t, 3)
	req := srv.lastRequest()
	assert.Equal(t, codexMethodTurnStart, req.Method)

	var params codexTurnStartParams
	require.NoError(t, json.Unmarshal(req.Params, &params))
	assert.Equal(t, "thread-1", params.ThreadID)
	require.Len(t, params.Input, 1)
	assert.Equal(t, "hello world", params.Input[0].Text)
}

func TestCodexTransport_Interrupt_UsesThreadAndTurnID(t *testing.T) {
	ct, srv := newStartedCodexTransport(t)

	require.NoError(t, ct.SendPrompt(context.Background(), "hello"))
	require.NoError(t, ct.Interrupt(context.Background()))

	srv.waitForRequests(t, 4)
	req := srv.lastRequest()
	assert.Equal(t, codexMethodTurnInterrupt, req.Method)

	var params codexTurnInterruptParams
	require.NoError(t, json.Unmarshal(req.Params, &params))
	assert.Equal(t, "thread-1", params.ThreadID)
	assert.Equal(t, "turn-1", params.TurnID)
}

func TestCodexTransport_RespondPermission_CommandApproval(t *testing.T) {
	ct, srv := newStartedCodexTransport(t)

	srv.pushRequest(11, codexRequestCommandApproval, codexCommandApprovalRequest{
		ThreadID: "thread-1",
		TurnID:   "turn-1",
		ItemID:   "item-1",
		Command:  ptr("go test ./..."),
	})
	ev := waitForEvent(t, ct.Events(), EventPermission, time.Second)
	assert.Contains(t, ev.PermissionDescription, "go test ./...")

	err := ct.RespondPermission(context.Background(), tmux.PermissionAllowAlways)
	require.NoError(t, err)

	srv.waitForResponses(t, 1)
	resp := srv.lastResponse()
	require.NotNil(t, resp.ID)
	assert.Equal(t, int64(11), *resp.ID)
	assert.Contains(t, string(resp.Result), `"acceptForSession"`)
}

func TestCodexTransport_RespondPermission_PermissionsApproval(t *testing.T) {
	ct, srv := newStartedCodexTransport(t)

	srv.pushRequest(12, codexRequestPermissionsApproval, codexPermissionsApprovalRequest{
		ThreadID:    "thread-1",
		TurnID:      "turn-1",
		ItemID:      "item-2",
		Permissions: json.RawMessage(`{"network":{"mode":"enabled"}}`),
	})
	_ = waitForEvent(t, ct.Events(), EventPermission, time.Second)

	err := ct.RespondPermission(context.Background(), tmux.PermissionAllowAlways)
	require.NoError(t, err)

	srv.waitForResponses(t, 1)
	resp := srv.lastResponse()
	require.NotNil(t, resp.ID)
	assert.Equal(t, int64(12), *resp.ID)
	assert.Contains(t, string(resp.Result), `"scope":"session"`)
	assert.Contains(t, string(resp.Result), `"network"`)
}

func TestCodexTransport_TurnStarted_Event(t *testing.T) {
	ct, srv := newStartedCodexTransport(t)

	srv.pushNotification(codexNotifyTurnStarted, codexTurnStartedParams{
		ThreadID: "thread-1",
		Turn:     codexTurn{ID: "turn-7"},
	})

	ev := waitForEvent(t, ct.Events(), EventTurnStarted, time.Second)
	assert.Equal(t, "turn-7", ev.TurnID)
}

func TestCodexTransport_MessageDelta_Event(t *testing.T) {
	ct, srv := newStartedCodexTransport(t)

	srv.pushNotification(codexNotifyAgentMessageDelta, codexAgentMessageDeltaParams{
		ThreadID: "thread-1",
		TurnID:   "turn-7",
		ItemID:   "item-7",
		Delta:    "hello",
	})

	ev := waitForEvent(t, ct.Events(), EventTextDelta, time.Second)
	assert.Equal(t, "turn-7", ev.TurnID)
	assert.Equal(t, "hello", ev.Text)
}

func TestCodexTransport_CommandExecution_ItemEvents(t *testing.T) {
	ct, srv := newStartedCodexTransport(t)

	srv.pushNotification(codexNotifyItemStarted, codexItemNotification{
		ThreadID: "thread-1",
		TurnID:   "turn-9",
		Item: codexThreadItem{
			ID:      "item-9",
			Type:    "commandExecution",
			Command: "ls -la",
		},
	})
	call := waitForEvent(t, ct.Events(), EventToolCall, time.Second)
	assert.Equal(t, "commandExecution", call.ToolName)
	assert.Equal(t, "ls -la", call.ToolInput)

	output := "ok"
	exitCode := 0
	srv.pushNotification(codexNotifyItemCompleted, codexItemNotification{
		ThreadID: "thread-1",
		TurnID:   "turn-9",
		Item: codexThreadItem{
			ID:               "item-9",
			Type:             "commandExecution",
			Command:          "ls -la",
			AggregatedOutput: &output,
			ExitCode:         &exitCode,
		},
	})
	result := waitForEvent(t, ct.Events(), EventToolResult, time.Second)
	assert.Equal(t, "commandExecution", result.ToolName)
	assert.Contains(t, result.ToolResult, "exit_code=0")
	assert.Contains(t, result.ToolResult, "ok")
}

func TestCodexTransport_TurnCompleted_HasPrompt(t *testing.T) {
	ct, srv := newStartedCodexTransport(t)

	srv.pushNotification(codexNotifyTurnCompleted, codexTurnCompletedParams{
		ThreadID: "thread-1",
		Turn:     codexTurn{ID: "turn-3"},
	})

	ev := waitForEvent(t, ct.Events(), EventTurnCompleted, time.Second)
	assert.True(t, ev.HasPrompt)
	assert.True(t, ev.Final)
}

func TestCodexTransport_ErrorNotification_EmitsSystemEvent(t *testing.T) {
	ct, srv := newStartedCodexTransport(t)

	srv.pushNotification(codexNotifyError, codexErrorNotification{
		ThreadID:  "thread-1",
		TurnID:    "turn-3",
		Error:     codexTurnError{Message: "boom"},
		WillRetry: false,
	})

	ev := waitForEvent(t, ct.Events(), EventSystem, time.Second)
	assert.Contains(t, ev.Text, "boom")
	assert.True(t, ev.HasPrompt)
}

func TestCodexTransport_BenignNotifications_AreSilentlyIgnored(t *testing.T) {
	ct, srv := newStartedCodexTransport(t)

	srv.pushNotification(codexNotifyHookStarted, map[string]any{"hook": "started"})
	srv.pushNotification(codexNotifyHookCompleted, map[string]any{"hook": "completed"})
	srv.pushNotification(codexNotifyFileChangeOutputDelta, map[string]any{"itemId": "item-1"})

	assert.Empty(t, collectEvents(t, ct.Events(), 150*time.Millisecond))
}

func TestCodexTransport_UnsupportedServerRequest_EmitsSystemEvent(t *testing.T) {
	ct, srv := newStartedCodexTransport(t)

	srv.pushRequest(99, "item/tool/call", map[string]any{"tool": "custom"})

	ev := waitForEvent(t, ct.Events(), EventSystem, time.Second)
	assert.Contains(t, ev.Text, "item/tool/call")
	srv.waitForResponses(t, 1)
}

func TestCodexTransport_EventsChannelClosedAfterProcessExit(t *testing.T) {
	proc := newFakeCodexProcess(t)
	ct := &CodexTransport{
		process: proc,
		events:  make(chan Event, 128),
		closed:  make(chan struct{}),
	}
	require.NoError(t, ct.Start(context.Background(), LaunchConfig{
		Program: "codex",
		WorkDir: t.TempDir(),
	}))
	t.Cleanup(func() { _ = ct.Close() })

	_ = proc.Close()

	select {
	case _, ok := <-ct.Events():
		if ok {
			for range ct.Events() {
			}
		}
	case <-time.After(2 * time.Second):
		t.Fatal("event channel not closed after process exit")
	}
}

func TestCodexTransport_Close_Idempotent(t *testing.T) {
	ct, _ := newStartedCodexTransport(t)
	assert.NoError(t, ct.Close())
	assert.NoError(t, ct.Close())
	assert.NoError(t, ct.Close())
}

func TestCodexTransport_MultipleNotifications_AllDelivered(t *testing.T) {
	ct, srv := newStartedCodexTransport(t)

	srv.pushNotification(codexNotifyTurnStarted, codexTurnStartedParams{
		ThreadID: "thread-1",
		Turn:     codexTurn{ID: "turn-5"},
	})
	srv.pushNotification(codexNotifyAgentMessageDelta, codexAgentMessageDeltaParams{
		ThreadID: "thread-1",
		TurnID:   "turn-5",
		ItemID:   "item-5",
		Delta:    "chunk1",
	})
	srv.pushNotification(codexNotifyAgentMessageDelta, codexAgentMessageDeltaParams{
		ThreadID: "thread-1",
		TurnID:   "turn-5",
		ItemID:   "item-5",
		Delta:    "chunk2",
	})
	srv.pushNotification(codexNotifyTurnCompleted, codexTurnCompletedParams{
		ThreadID: "thread-1",
		Turn:     codexTurn{ID: "turn-5"},
	})

	events := collectEvents(t, ct.Events(), 200*time.Millisecond)
	kinds := make([]EventKind, 0, len(events))
	var deltas []string
	for _, ev := range events {
		kinds = append(kinds, ev.Kind)
		if ev.Kind == EventTextDelta {
			deltas = append(deltas, ev.Text)
		}
	}

	assert.Contains(t, kinds, EventTurnStarted)
	assert.Contains(t, kinds, EventTextDelta)
	assert.Contains(t, kinds, EventTurnCompleted)
	assert.ElementsMatch(t, []string{"chunk1", "chunk2"}, deltas)
}

func ptr(s string) *string { return &s }

// TestCodexTransport_Start_FastServiceTier verifies that when SpeedTier is "fast",
// the thread/start request includes serviceTier="fast".
func TestCodexTransport_Start_FastServiceTier(t *testing.T) {
	proc := newFakeCodexProcess(t)
	ct := &CodexTransport{
		process: proc,
		events:  make(chan Event, 128),
		closed:  make(chan struct{}),
	}
	t.Cleanup(func() { _ = ct.Close() })

	err := ct.Start(context.Background(), LaunchConfig{
		Program:   "codex --model gpt-5.4",
		WorkDir:   t.TempDir(),
		SpeedTier: "fast",
	})
	require.NoError(t, err)
	require.NotNil(t, proc.server)

	proc.server.waitForRequests(t, 2) // initialize + thread/start

	// Find the thread/start request and verify serviceTier.
	proc.server.mu.Lock()
	var threadStartMsg jsonRPCMsg
	for _, req := range proc.server.requests {
		if req.Method == codexMethodThreadStart {
			threadStartMsg = req
			break
		}
	}
	proc.server.mu.Unlock()

	require.Equal(t, codexMethodThreadStart, threadStartMsg.Method)
	var params codexThreadStartParams
	require.NoError(t, json.Unmarshal(threadStartMsg.Params, &params))
	assert.Equal(t, "fast", params.ServiceTier)
}

// TestCodexTransport_Start_DefaultServiceTierOmitted verifies that when SpeedTier is
// empty, the thread/start request omits the serviceTier field entirely.
func TestCodexTransport_Start_DefaultServiceTierOmitted(t *testing.T) {
	_, server := newStartedCodexTransport(t)

	server.waitForRequests(t, 2) // initialize + thread/start

	server.mu.Lock()
	var threadStartMsg jsonRPCMsg
	for _, req := range server.requests {
		if req.Method == codexMethodThreadStart {
			threadStartMsg = req
			break
		}
	}
	server.mu.Unlock()

	require.Equal(t, codexMethodThreadStart, threadStartMsg.Method)
	// serviceTier must be absent when SpeedTier is default.
	assert.NotContains(t, string(threadStartMsg.Params), "serviceTier")
}
