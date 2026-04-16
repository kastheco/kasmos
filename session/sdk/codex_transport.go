// Package sdk – Codex transport.
//
// # Protocol surface
//
// CodexTransport speaks a JSON-RPC 2.0 dialect over the Codex app-server's
// stdio connection. The generic Transport interface deliberately limits the
// surface to three operations: prompt delivery, turn interruption, and
// permission approval/rejection. Codex-only advanced controls such as
// turn/steer (mid-turn prompt injection) and multi-agent delegation are out
// of scope for this pass.
//
// # TODO – deferred Codex-only controls
//
//   - turn/steer: inject a steering prompt while a turn is in progress.
//   - session/fork: branch the conversation at an earlier message index.
//   - session/reset: discard conversation history and start fresh.
//
// These methods require extensions to the Transport interface or a
// Codex-specific sub-interface; they are left for a future wave.
package sdk

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/kastheco/kasmos/session/tmux"
)

// Codex app-server JSON-RPC method names.
// All names are intentionally kept in this file and must not leak into
// session/sdk/session.go or any caller layer.
const (
	// Client → server requests.
	codexMethodTurnInput         = "turn/input"
	codexMethodTurnInterrupt     = "turn/interrupt"
	codexMethodPermissionRespond = "permission/respond"

	// Server → client notifications.
	codexNotifyTurnStarted     = "turn/started"
	codexNotifyMessageDelta    = "message/delta"
	codexNotifyToolCall        = "tool/call"
	codexNotifyToolResult      = "tool/result"
	codexNotifyPermission      = "permission/request"
	codexNotifyTurnComplete    = "turn/complete"
	codexNotifyTurnInterrupted = "turn/interrupted"
)

// codexTurnInputParams carries the user message for a new Codex turn.
type codexTurnInputParams struct {
	Text string `json:"text"`
}

// codexTurnInterruptParams is intentionally empty; the server needs no
// additional context to abort the current turn.
type codexTurnInterruptParams struct{}

// codexPermissionRespondParams forwards the operator's choice to Codex's
// numbered permission menu.
type codexPermissionRespondParams struct {
	// Response encodes the operator's decision:
	//   "allow_once"   → PermissionAllowOnce
	//   "allow_always" → PermissionAllowAlways
	//   "deny"         → PermissionReject
	Response string `json:"response"`
}

// codexNotifyTurnStartedParams carries the server-assigned turn identifier.
type codexNotifyTurnStartedParams struct {
	TurnID string `json:"turn_id"`
}

// codexNotifyMessageDeltaParams carries an incremental text chunk.
type codexNotifyMessageDeltaParams struct {
	TurnID string `json:"turn_id"`
	Text   string `json:"text"`
}

// codexNotifyToolCallParams carries a single tool invocation.
type codexNotifyToolCallParams struct {
	TurnID   string `json:"turn_id"`
	ToolName string `json:"tool_name"`
	Input    string `json:"input"` // JSON-encoded tool arguments
}

// codexNotifyToolResultParams carries the result of a tool invocation.
type codexNotifyToolResultParams struct {
	TurnID   string `json:"turn_id"`
	ToolName string `json:"tool_name"`
	Result   string `json:"result"` // JSON-encoded result
}

// codexNotifyPermissionParams carries the description of an action that
// requires operator approval before Codex can proceed.
type codexNotifyPermissionParams struct {
	TurnID      string `json:"turn_id"`
	Description string `json:"description"`
	Pattern     string `json:"pattern,omitempty"`
}

// codexNotifyTurnCompleteParams signals that the current turn finished
// normally and Codex is idle (ready for the next prompt).
type codexNotifyTurnCompleteParams struct {
	TurnID string `json:"turn_id"`
}

// codexNotifyTurnInterruptedParams signals that the current turn was
// interrupted (by the operator or internally by Codex) and Codex is idle.
type codexNotifyTurnInterruptedParams struct {
	TurnID string `json:"turn_id"`
}

// CodexTransport implements Transport for the OpenAI Codex CLI running in
// app-server mode. It drives the subprocess directly via the in-repo
// JSON-RPC client; no third-party Go SDK dependency is required.
//
// Lifecycle:
//  1. NewCodexTransport() → allocates with no process running.
//  2. Start(ctx, cfg)     → resolves executable, spawns subprocess, starts
//     the notification dispatch goroutine.
//  3. SendPrompt / Interrupt / RespondPermission → drive the running session.
//  4. Close()             → terminates the subprocess and drains resources.
type CodexTransport struct {
	process *Process
	client  *Client
	events  chan Event

	// closed is closed when Close is called; used to signal the dispatcher
	// goroutine to stop.
	closed    chan struct{}
	closeOnce sync.Once

	// mu guards currentTurnID and pendingPermission.
	mu                sync.Mutex
	currentTurnID     string
	pendingPermission bool
}

// NewCodexTransport returns an unstarted CodexTransport that satisfies the
// Transport interface.
func NewCodexTransport() Transport {
	return &CodexTransport{
		process: NewProcess(),
		events:  make(chan Event, 128),
		closed:  make(chan struct{}),
	}
}

// Start launches the Codex app-server subprocess described by cfg and begins
// reading notifications from its stdout.
//
// If cfg.Program does not already contain the "--server" flag, Start appends
// it so the Codex CLI starts in app-server (JSON-RPC over stdio) mode.
//
// Returns an error if cfg.Program is empty (or whitespace only) or if the
// subprocess cannot be started.
func (t *CodexTransport) Start(ctx context.Context, cfg LaunchConfig) error {
	if strings.TrimSpace(cfg.Program) == "" {
		return fmt.Errorf("codex transport: empty program")
	}

	// Ensure the Codex CLI runs in app-server mode.
	if !strings.Contains(cfg.Program, "--server") {
		cfg.Program = strings.TrimSpace(cfg.Program) + " --server"
	}

	stdin, stdout, err := t.process.Start(cfg)
	if err != nil {
		return fmt.Errorf("codex transport: start process: %w", err)
	}

	t.client = NewClient(stdin, stdout)
	go t.dispatchNotifications()
	return nil
}

// SendPrompt delivers a new user prompt to the running Codex turn.
// Empty or whitespace-only prompts are silently ignored so the caller does not
// accidentally start a blank turn.
func (t *CodexTransport) SendPrompt(ctx context.Context, prompt string) error {
	if strings.TrimSpace(prompt) == "" {
		return nil
	}
	if err := t.guardClient(); err != nil {
		return err
	}
	params := codexTurnInputParams{Text: prompt}
	return t.client.Call(ctx, codexMethodTurnInput, params, nil)
}

// Interrupt requests Codex to abort the current turn.
// Safe to call when no turn is in progress or after the transport has closed;
// in those cases the call is a no-op.
func (t *CodexTransport) Interrupt(ctx context.Context) error {
	if err := t.guardClient(); err != nil {
		// Transport already closed — treat as no-op rather than panic.
		return nil
	}
	return t.client.Call(ctx, codexMethodTurnInterrupt, codexTurnInterruptParams{}, nil)
}

// RespondPermission forwards the operator's decision for the most recent
// permission prompt to Codex.
//
// Safe to call when the transport has already closed; returns nil without
// panicking.
func (t *CodexTransport) RespondPermission(ctx context.Context, choice tmux.PermissionChoice) error {
	if err := t.guardClient(); err != nil {
		return nil
	}

	t.mu.Lock()
	t.pendingPermission = false
	t.mu.Unlock()

	var response string
	switch choice {
	case tmux.PermissionAllowAlways:
		response = "allow_always"
	case tmux.PermissionReject:
		response = "deny"
	default: // PermissionAllowOnce
		response = "allow_once"
	}

	params := codexPermissionRespondParams{Response: response}
	return t.client.Call(ctx, codexMethodPermissionRespond, params, nil)
}

// Events returns the read-only channel of structured events produced by the
// transport. The channel is closed when the transport is closed or the Codex
// process exits.
func (t *CodexTransport) Events() <-chan Event {
	return t.events
}

// PID returns the OS process ID of the running Codex process.
// Returns 0 before a successful Start call.
func (t *CodexTransport) PID() int {
	return t.process.PID()
}

// Close terminates the Codex subprocess and releases all resources.
// Safe to call before Start and after the process has already exited.
func (t *CodexTransport) Close() error {
	var err error
	t.closeOnce.Do(func() {
		close(t.closed)
		if t.client != nil {
			_ = t.client.Close()
		}
		err = t.process.Close()
	})
	return err
}

// guardClient returns an error if the client has not been initialised yet
// (Start was not called) or after the transport has been closed.
func (t *CodexTransport) guardClient() error {
	if t.client == nil {
		return fmt.Errorf("codex transport: not started")
	}
	select {
	case <-t.closed:
		return fmt.Errorf("codex transport: closed")
	default:
		return nil
	}
}

// dispatchNotifications is the single background goroutine that reads
// server-sent notifications from the JSON-RPC client and converts them into
// typed Events delivered on t.events.
//
// It exits when the notifications channel is closed (which happens when the
// Codex process exits or the client is closed) or when t.closed is signalled.
// In both cases t.events is closed before the goroutine returns so the session
// layer can detect end-of-stream.
func (t *CodexTransport) dispatchNotifications() {
	defer close(t.events)

	notifications := t.client.Notifications()
	for {
		select {
		case <-t.closed:
			return
		case n, ok := <-notifications:
			if !ok {
				// The Codex process exited; close the event stream.
				return
			}
			ev, err := t.translateNotification(n)
			if err != nil {
				// Unrecognised notification — emit a system event so the
				// operator can see it in the structured log.
				t.emit(Event{
					Kind:      EventSystem,
					Text:      fmt.Sprintf("codex: unknown notification %q: %v", n.Method, err),
					Timestamp: time.Now(),
				})
				continue
			}
			if ev == nil {
				// translateNotification returns nil for notifications that
				// should be silently dropped.
				continue
			}
			t.emit(*ev)
		}
	}
}

// emit sends an event on t.events without blocking. If the buffer is full the
// event is dropped to avoid stalling the dispatcher goroutine.
func (t *CodexTransport) emit(ev Event) {
	select {
	case t.events <- ev:
	default:
		// Buffer full — drop rather than deadlock.
	}
}

// translateNotification converts a raw JSON-RPC Notification from the Codex
// app-server into a typed Event.
//
// Returns:
//   - (*Event, nil)  – a valid event ready for delivery.
//   - (nil, nil)     – the notification is recognised but intentionally dropped.
//   - (nil, error)   – the notification could not be parsed.
func (t *CodexTransport) translateNotification(n Notification) (*Event, error) {
	now := time.Now()

	switch n.Method {
	case codexNotifyTurnStarted:
		var p codexNotifyTurnStartedParams
		if err := json.Unmarshal(n.Params, &p); err != nil {
			return nil, fmt.Errorf("unmarshal turn_started params: %w", err)
		}
		t.mu.Lock()
		t.currentTurnID = p.TurnID
		t.mu.Unlock()
		return &Event{
			Kind:      EventTurnStarted,
			TurnID:    p.TurnID,
			Timestamp: now,
		}, nil

	case codexNotifyMessageDelta:
		var p codexNotifyMessageDeltaParams
		if err := json.Unmarshal(n.Params, &p); err != nil {
			return nil, fmt.Errorf("unmarshal message_delta params: %w", err)
		}
		return &Event{
			Kind:      EventTextDelta,
			TurnID:    p.TurnID,
			Text:      p.Text,
			Timestamp: now,
		}, nil

	case codexNotifyToolCall:
		var p codexNotifyToolCallParams
		if err := json.Unmarshal(n.Params, &p); err != nil {
			return nil, fmt.Errorf("unmarshal tool_call params: %w", err)
		}
		return &Event{
			Kind:      EventToolCall,
			TurnID:    p.TurnID,
			ToolName:  p.ToolName,
			ToolInput: p.Input,
			Timestamp: now,
		}, nil

	case codexNotifyToolResult:
		var p codexNotifyToolResultParams
		if err := json.Unmarshal(n.Params, &p); err != nil {
			return nil, fmt.Errorf("unmarshal tool_result params: %w", err)
		}
		return &Event{
			Kind:       EventToolResult,
			TurnID:     p.TurnID,
			ToolName:   p.ToolName,
			ToolResult: p.Result,
			Timestamp:  now,
		}, nil

	case codexNotifyPermission:
		var p codexNotifyPermissionParams
		if err := json.Unmarshal(n.Params, &p); err != nil {
			return nil, fmt.Errorf("unmarshal permission_request params: %w", err)
		}
		t.mu.Lock()
		t.pendingPermission = true
		t.mu.Unlock()
		return &Event{
			Kind:                  EventPermission,
			TurnID:                p.TurnID,
			PermissionDescription: p.Description,
			PermissionPattern:     p.Pattern,
			Timestamp:             now,
		}, nil

	case codexNotifyTurnComplete:
		var p codexNotifyTurnCompleteParams
		if err := json.Unmarshal(n.Params, &p); err != nil {
			return nil, fmt.Errorf("unmarshal turn_complete params: %w", err)
		}
		// HasPrompt=true signals to the session layer that Codex is idle and
		// ready for the next user message — fixing the tmux limitation in
		// session/tmux/codex_adapter.go where DetectPrompt always returns false
		// because no reliable idle affordance has been confirmed from live pane
		// capture.
		return &Event{
			Kind:      EventTurnCompleted,
			TurnID:    p.TurnID,
			HasPrompt: true,
			Final:     true,
			Timestamp: now,
		}, nil

	case codexNotifyTurnInterrupted:
		var p codexNotifyTurnInterruptedParams
		if err := json.Unmarshal(n.Params, &p); err != nil {
			return nil, fmt.Errorf("unmarshal turn_interrupted params: %w", err)
		}
		// HasPrompt=true: Codex is idle after interruption, accepting new input.
		return &Event{
			Kind:      EventTurnInterrupted,
			TurnID:    p.TurnID,
			HasPrompt: true,
			Final:     true,
			Timestamp: now,
		}, nil

	default:
		return nil, fmt.Errorf("unrecognised method %q", n.Method)
	}
}
