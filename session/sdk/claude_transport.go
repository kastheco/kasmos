// Package sdk – Claude transport.
//
// # Protocol surface
//
// ClaudeTransport drives Claude Code in app-server mode via JSON-RPC 2.0 over
// stdio. The subprocess is launched with the "--app-server" flag, which puts
// Claude into a non-interactive mode where it speaks the JSON-RPC dialect
// defined by the constants below.
//
// All method names and wire struct types are intentionally private to this file
// so Claude-specific protocol shapes never leak into the generic session/sdk
// substrate.
//
// # Startup handshake
//
// After spawning the subprocess Start sends a "claude/initialize" request and
// waits up to handshakeTimeout for the response. A timeout or error closes the
// subprocess and returns a wrapped startup error so the session layer can
// surface it to the operator.
//
// # Initial prompt injection
//
// When LaunchConfig.InitialPrompt is non-empty it is delivered via
// "claude/sendMessage" immediately after the successful handshake rather than
// being baked into the CLI argv. This is more reliable than positional-argument
// injection because it avoids shell-quoting edge cases and lets the caller
// know if the delivery itself fails.
//
// # Permission handling
//
// Multiple consecutive permission notifications (before an answer arrives)
// replace the pending prompt rather than being queued — only the most recent
// permission request is tracked. RespondPermission returns nil without error
// when no request is pending to avoid spamming the app log.
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

// Claude wire protocol constants — kept local to this file.
const (
	// Client → server requests.
	claudeMethodInit        = "claude/initialize"
	claudeMethodSendMessage = "claude/sendMessage"
	claudeMethodInterrupt   = "claude/interrupt"
	claudeMethodPermRespond = "claude/permissionRespond"

	// Server → client notifications.
	claudeNotifyTurnStarted     = "claude/turnStarted"
	claudeNotifyStreamText      = "claude/streamText"
	claudeNotifyToolUse         = "claude/toolUse"
	claudeNotifyToolResult      = "claude/toolResult"
	claudeNotifyPermRequest     = "claude/permissionRequest"
	claudeNotifyTurnComplete    = "claude/turnComplete"
	claudeNotifyTurnInterrupted = "claude/turnInterrupted"

	// claudeAppServerFlag enables the Claude app-server JSON-RPC mode.
	claudeAppServerFlag = "--app-server"

	// claudePermBypassFlag tells Claude to skip permission prompts.
	claudePermBypassFlag = "--permission-mode bypassPermissions"

	// handshakeTimeout is how long Start waits for the init response before
	// declaring a startup failure and terminating the subprocess.
	handshakeTimeout = 10 * time.Second
)

// --- Wire request/response structs ------------------------------------------
// These are intentionally unexported and confined to this file.

type claudeInitParams struct{}

type claudeInitResult struct {
	// Version carries the Claude app-server protocol version string.
	Version string `json:"version,omitempty"`
}

type claudeSendMessageParams struct {
	Text string `json:"text"`
}

type claudeInterruptParams struct{}

type claudePermRespondParams struct {
	// PermissionID is the opaque identifier returned in the permission request.
	PermissionID string `json:"permission_id"`
	// Response encodes the operator's choice:
	//   "allow_once"   → PermissionAllowOnce
	//   "allow_always" → PermissionAllowAlways
	//   "deny"         → PermissionReject
	Response string `json:"response"`
}

// --- Notification param structs ----------------------------------------------

type claudeNotifyTurnStartedParams struct {
	TurnID string `json:"turn_id"`
}

type claudeNotifyStreamTextParams struct {
	TurnID string `json:"turn_id"`
	Text   string `json:"text"`
}

type claudeNotifyToolUseParams struct {
	TurnID   string `json:"turn_id"`
	ToolName string `json:"tool_name"`
	Input    string `json:"input"` // JSON-encoded tool arguments
}

type claudeNotifyToolResultParams struct {
	TurnID   string `json:"turn_id"`
	ToolName string `json:"tool_name"`
	Result   string `json:"result"` // JSON-encoded result
}

type claudeNotifyPermRequestParams struct {
	TurnID       string `json:"turn_id"`
	PermissionID string `json:"permission_id"`
	Description  string `json:"description"`
	Pattern      string `json:"pattern,omitempty"`
}

type claudeNotifyTurnCompleteParams struct {
	TurnID string `json:"turn_id"`
}

type claudeNotifyTurnInterruptedParams struct {
	TurnID string `json:"turn_id"`
}

// ----------------------------------------------------------------------------

// ClaudeTransport implements Transport for Claude Code running in app-server
// mode. It drives the subprocess directly via the in-repo JSON-RPC client; no
// third-party Go SDK dependency is required.
//
// Lifecycle:
//  1. NewClaudeTransport() → allocates with no process running.
//  2. Start(ctx, cfg)      → resolves executable, spawns subprocess, performs
//     the init handshake, optionally delivers the initial prompt, then starts
//     the notification dispatch goroutine.
//  3. SendPrompt / Interrupt / RespondPermission → drive the running session.
//  4. Close()              → terminates the subprocess and releases resources.
type ClaudeTransport struct {
	process *Process
	client  *Client
	events  chan Event

	// closed is closed when Close is called; used to signal the dispatcher to stop.
	closed    chan struct{}
	closeOnce sync.Once

	// mu guards currentTurnID, pendingPermID, and turnActive.
	mu            sync.Mutex
	currentTurnID string
	// pendingPermID is the permission_id of the most recent unanswered permission
	// request. It is the empty string when no permission is pending. Multiple
	// consecutive requests replace the previous value without queuing.
	pendingPermID string
	// turnActive is true while the agent is processing a response turn.
	turnActive bool
}

// NewClaudeTransport returns an unstarted ClaudeTransport that satisfies the
// Transport interface.
func NewClaudeTransport() Transport {
	return &ClaudeTransport{
		process: NewProcess(),
		events:  make(chan Event, 128),
		closed:  make(chan struct{}),
	}
}

// Start launches the Claude app-server subprocess described by cfg.
//
// Start automatically:
//   - Appends "--app-server" to cfg.Program so Claude enters JSON-RPC mode.
//   - Appends "--permission-mode bypassPermissions" when cfg.SkipPermissions is true.
//   - Injects CLAUDE_CODE_NO_FLICKER into the child environment (1 when
//     cfg.NoFlicker is true, 0 otherwise).
//   - Performs the "claude/initialize" handshake with a 10-second timeout.
//   - Delivers cfg.InitialPrompt via SendPrompt after a successful handshake
//     (rather than via CLI argv injection).
//
// Returns an error if cfg.Program is empty, the subprocess cannot start, or
// the handshake times out. On handshake failure the subprocess is terminated
// before the error is returned.
func (t *ClaudeTransport) Start(ctx context.Context, cfg LaunchConfig) error {
	if strings.TrimSpace(cfg.Program) == "" {
		return fmt.Errorf("claude transport: empty program")
	}

	// Build the effective program string with Claude-specific flags.
	program := strings.TrimSpace(cfg.Program)
	if !strings.Contains(program, claudeAppServerFlag) {
		program = program + " " + claudeAppServerFlag
	}
	if cfg.SkipPermissions && !strings.Contains(program, "--permission-mode") {
		program = program + " " + claudePermBypassFlag
	}

	// Inject Claude-specific environment variables via ExtraEnv.
	flickerVal := "0"
	if cfg.NoFlicker {
		flickerVal = "1"
	}
	cfg.ExtraEnv = append(cfg.ExtraEnv, "CLAUDE_CODE_NO_FLICKER="+flickerVal)
	cfg.Program = program

	stdin, stdout, err := t.process.Start(cfg)
	if err != nil {
		return fmt.Errorf("claude transport: start process: %w", err)
	}

	t.client = NewClient(stdin, stdout)

	// Perform the init handshake with a bounded timeout. On failure, clean up
	// the subprocess immediately so the caller does not have to call Close.
	initCtx, cancel := context.WithTimeout(ctx, handshakeTimeout)
	defer cancel()

	var initResult claudeInitResult
	if err := t.client.Call(initCtx, claudeMethodInit, claudeInitParams{}, &initResult); err != nil {
		_ = t.client.Close()
		_ = t.process.Close()
		return fmt.Errorf("claude transport: handshake: %w", err)
	}

	// Start the notification dispatch goroutine before sending the initial
	// prompt so we do not lose any notifications emitted as a result of it.
	go t.dispatchNotifications()

	// Deliver the initial prompt now that the handshake has succeeded.
	if cfg.InitialPrompt != "" {
		if err := t.SendPrompt(ctx, cfg.InitialPrompt); err != nil {
			// Non-fatal: the session is up; the caller can retry.
			t.emit(Event{
				Kind:      EventSystem,
				Text:      fmt.Sprintf("claude: initial prompt delivery failed: %v", err),
				Timestamp: time.Now(),
			})
		}
	}

	return nil
}

// SendPrompt delivers a new user prompt to the running Claude session.
// Empty or whitespace-only prompts are silently ignored.
func (t *ClaudeTransport) SendPrompt(ctx context.Context, prompt string) error {
	if strings.TrimSpace(prompt) == "" {
		return nil
	}
	if err := t.guardClient(); err != nil {
		return err
	}
	params := claudeSendMessageParams{Text: prompt}
	return t.client.Call(ctx, claudeMethodSendMessage, params, nil)
}

// Interrupt requests Claude to abort the current turn.
// Safe to call when no turn is in progress — in that case it is a no-op.
func (t *ClaudeTransport) Interrupt(ctx context.Context) error {
	t.mu.Lock()
	active := t.turnActive
	t.mu.Unlock()
	if !active {
		// No turn running — idempotent no-op as required by the spec.
		return nil
	}
	if err := t.guardClient(); err != nil {
		// Transport closed — treat as no-op.
		return nil
	}
	return t.client.Call(ctx, claudeMethodInterrupt, claudeInterruptParams{}, nil)
}

// RespondPermission forwards the operator's decision for the most recent
// permission request to Claude.
//
// Returns nil without error when no permission request is pending to avoid
// spamming the app log with spurious errors.
func (t *ClaudeTransport) RespondPermission(ctx context.Context, choice tmux.PermissionChoice) error {
	t.mu.Lock()
	permID := t.pendingPermID
	if permID == "" {
		t.mu.Unlock()
		// No pending permission request — silent no-op.
		return nil
	}
	t.pendingPermID = ""
	t.mu.Unlock()

	if err := t.guardClient(); err != nil {
		return nil
	}

	var response string
	switch choice {
	case tmux.PermissionAllowAlways:
		response = "allow_always"
	case tmux.PermissionReject:
		response = "deny"
	default: // PermissionAllowOnce
		response = "allow_once"
	}

	params := claudePermRespondParams{
		PermissionID: permID,
		Response:     response,
	}
	return t.client.Call(ctx, claudeMethodPermRespond, params, nil)
}

// Events returns the read-only channel of structured events produced by the
// transport. The channel is closed when the transport is closed or the Claude
// process exits.
func (t *ClaudeTransport) Events() <-chan Event {
	return t.events
}

// PID returns the OS process ID of the running Claude process.
// Returns 0 before a successful Start call.
func (t *ClaudeTransport) PID() int {
	return t.process.PID()
}

// Close terminates the Claude subprocess and releases all resources.
// Safe to call before Start and after the process has already exited.
func (t *ClaudeTransport) Close() error {
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
func (t *ClaudeTransport) guardClient() error {
	if t.client == nil {
		return fmt.Errorf("claude transport: not started")
	}
	select {
	case <-t.closed:
		return fmt.Errorf("claude transport: closed")
	default:
		return nil
	}
}

// dispatchNotifications is the single background goroutine that reads
// server-sent notifications from the JSON-RPC client and converts them into
// typed Events delivered on t.events.
//
// It exits when the notifications channel is closed (the Claude process exited
// or the client was closed) or when t.closed is signalled. In both cases
// t.events is closed before the goroutine returns so the session layer can
// detect end-of-stream.
func (t *ClaudeTransport) dispatchNotifications() {
	defer close(t.events)

	notifications := t.client.Notifications()
	for {
		select {
		case <-t.closed:
			return
		case n, ok := <-notifications:
			if !ok {
				// Claude process exited — close the event stream.
				return
			}
			ev, err := t.translateNotification(n)
			if err != nil {
				t.emit(Event{
					Kind:      EventSystem,
					Text:      fmt.Sprintf("claude: unknown notification %q: %v", n.Method, err),
					Timestamp: time.Now(),
				})
				continue
			}
			if ev == nil {
				continue
			}
			t.emit(*ev)
		}
	}
}

// emit sends an event on t.events without blocking. If the buffer is full the
// event is dropped rather than stalling the dispatcher goroutine.
func (t *ClaudeTransport) emit(ev Event) {
	select {
	case t.events <- ev:
	default:
		// Buffer full — drop rather than deadlock.
	}
}

// translateNotification converts a raw JSON-RPC Notification from the Claude
// app-server into a typed Event.
//
// Returns:
//   - (*Event, nil)  – a valid event ready for delivery.
//   - (nil, nil)     – the notification is recognised but intentionally dropped.
//   - (nil, error)   – the notification is unrecognised or malformed.
func (t *ClaudeTransport) translateNotification(n Notification) (*Event, error) {
	now := time.Now()

	switch n.Method {
	case claudeNotifyTurnStarted:
		var p claudeNotifyTurnStartedParams
		if err := json.Unmarshal(n.Params, &p); err != nil {
			return nil, fmt.Errorf("unmarshal turn_started params: %w", err)
		}
		t.mu.Lock()
		t.currentTurnID = p.TurnID
		t.turnActive = true
		t.mu.Unlock()
		return &Event{
			Kind:      EventTurnStarted,
			TurnID:    p.TurnID,
			Timestamp: now,
		}, nil

	case claudeNotifyStreamText:
		var p claudeNotifyStreamTextParams
		if err := json.Unmarshal(n.Params, &p); err != nil {
			return nil, fmt.Errorf("unmarshal stream_text params: %w", err)
		}
		return &Event{
			Kind:      EventTextDelta,
			TurnID:    p.TurnID,
			Text:      p.Text,
			Timestamp: now,
		}, nil

	case claudeNotifyToolUse:
		var p claudeNotifyToolUseParams
		if err := json.Unmarshal(n.Params, &p); err != nil {
			return nil, fmt.Errorf("unmarshal tool_use params: %w", err)
		}
		return &Event{
			Kind:      EventToolCall,
			TurnID:    p.TurnID,
			ToolName:  p.ToolName,
			ToolInput: p.Input,
			Timestamp: now,
		}, nil

	case claudeNotifyToolResult:
		var p claudeNotifyToolResultParams
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

	case claudeNotifyPermRequest:
		var p claudeNotifyPermRequestParams
		if err := json.Unmarshal(n.Params, &p); err != nil {
			return nil, fmt.Errorf("unmarshal permission_request params: %w", err)
		}
		// Replace (not queue) the pending permission request with the newest one.
		t.mu.Lock()
		t.pendingPermID = p.PermissionID
		t.mu.Unlock()
		return &Event{
			Kind:                  EventPermission,
			TurnID:                p.TurnID,
			PermissionDescription: p.Description,
			PermissionPattern:     p.Pattern,
			Timestamp:             now,
		}, nil

	case claudeNotifyTurnComplete:
		var p claudeNotifyTurnCompleteParams
		if err := json.Unmarshal(n.Params, &p); err != nil {
			return nil, fmt.Errorf("unmarshal turn_complete params: %w", err)
		}
		t.mu.Lock()
		t.turnActive = false
		t.mu.Unlock()
		return &Event{
			Kind:      EventTurnCompleted,
			TurnID:    p.TurnID,
			HasPrompt: true,
			Final:     true,
			Timestamp: now,
		}, nil

	case claudeNotifyTurnInterrupted:
		var p claudeNotifyTurnInterruptedParams
		if err := json.Unmarshal(n.Params, &p); err != nil {
			return nil, fmt.Errorf("unmarshal turn_interrupted params: %w", err)
		}
		t.mu.Lock()
		t.turnActive = false
		t.mu.Unlock()
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
