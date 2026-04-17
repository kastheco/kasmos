// Package sdk – Codex transport.
//
// # Protocol surface
//
// CodexTransport speaks the current Codex App Server protocol over stdio.
// The documented client flow is:
//
//  1. initialize
//  2. initialized
//  3. thread/start
//  4. turn/start for each prompt
//
// The generic Transport interface deliberately limits the client surface to
// prompt delivery, turn interruption, and permission approval/rejection.
// Codex-only controls such as turn/steer, thread/fork, and review mode are
// intentionally left out for a future wave.
package sdk

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/kastheco/kasmos/session/tmux"
)

const (
	// Client -> server startup and turn lifecycle.
	codexMethodInitialize    = "initialize"
	codexMethodInitialized   = "initialized"
	codexMethodThreadStart   = "thread/start"
	codexMethodTurnStart     = "turn/start"
	codexMethodTurnSteer     = "turn/steer"
	codexMethodTurnInterrupt = "turn/interrupt"

	// Server -> client notifications.
	codexNotifyError             = "error"
	codexNotifyThreadStarted     = "thread/started"
	codexNotifyTurnStarted       = "turn/started"
	codexNotifyTurnCompleted     = "turn/completed"
	codexNotifyItemStarted       = "item/started"
	codexNotifyItemCompleted     = "item/completed"
	codexNotifyAgentMessageDelta = "item/agentMessage/delta"

	// Known-benign notifications emitted by codex-cli 0.121 that have no
	// observable effect on our rendering. We swallow them silently rather
	// than surfacing "[system: codex: unknown notification ...]" lines into
	// the agent pane on every tick.
	codexNotifyMcpStartupStatus         = "mcpServer/startupStatus/updated"
	codexNotifyThreadStatus             = "thread/status/changed"
	codexNotifyAccountRateLimits        = "account/rateLimits/updated"
	codexNotifyThreadTokenUsage         = "thread/tokenUsage/updated"
	codexNotifyServerRequestDone        = "serverRequest/resolved"
	codexNotifyTurnPlanUpdated          = "turn/plan/updated"
	codexNotifyTurnDiffUpdated          = "turn/diff/updated"
	codexNotifyCommandExecOutputDelta   = "item/commandExecution/outputDelta"

	// Server -> client requests.
	codexRequestCommandApproval     = "item/commandExecution/requestApproval"
	codexRequestPermissionsApproval = "item/permissions/requestApproval"

	// Known server requests we cannot service. We reply with JSON-RPC
	// method-not-found and suppress the pane-visible error line that
	// would otherwise fire every time codex asked.
	codexRequestElicitation = "mcpServer/elicitation/request"

	// codexHandshakeTimeout covers initialize + initialized + thread/start.
	// thread/start synchronously sets up codex's HTTP MCP clients for any
	// server configured in .codex/config.toml, and measurements on codex-cli
	// 0.121 show this takes 15-18s against a local kasmos MCP endpoint
	// (codex's own MCP init blocks thread/start, not kasmos — the kasmos
	// server itself goes starting→ready in <50ms). 45s leaves >2x headroom
	// over the observed ceiling without hiding a truly wedged handshake.
	codexHandshakeTimeout = 45 * time.Second
)

type codexProcess interface {
	Start(cfg LaunchConfig) (io.WriteCloser, io.ReadCloser, error)
	PID() int
	Close() error
}

type codexClientInfo struct {
	Name    string `json:"name"`
	Title   string `json:"title,omitempty"`
	Version string `json:"version"`
}

type codexInitializeParams struct {
	ClientInfo codexClientInfo `json:"clientInfo"`
}

type codexInitializeResult struct {
	UserAgent string `json:"userAgent,omitempty"`
}

type codexThreadStartParams struct {
	Cwd            string `json:"cwd,omitempty"`
	ApprovalPolicy any    `json:"approvalPolicy,omitempty"`
	Sandbox        any    `json:"sandbox,omitempty"`
}

type codexThreadStartResult struct {
	Thread codexThread `json:"thread"`
}

type codexThread struct {
	ID string `json:"id"`
}

type codexUserInput struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

type codexTurnStartParams struct {
	ThreadID string           `json:"threadId"`
	Input    []codexUserInput `json:"input"`
}

// codexTurnSteerParams injects input into an already-running turn rather
// than starting a new one. expectedTurnId must match the active turn id
// or codex fails the request, so the caller is responsible for reading
// currentTurnID under the transport mutex before making the call.
type codexTurnSteerParams struct {
	ThreadID       string           `json:"threadId"`
	ExpectedTurnID string           `json:"expectedTurnId"`
	Input          []codexUserInput `json:"input"`
}

type codexTurnStartResult struct {
	Turn codexTurn `json:"turn"`
}

type codexTurn struct {
	ID string `json:"id"`
}

type codexTurnStartedParams struct {
	ThreadID string    `json:"threadId"`
	Turn     codexTurn `json:"turn"`
}

type codexTurnCompletedParams struct {
	ThreadID string    `json:"threadId"`
	Turn     codexTurn `json:"turn"`
}

type codexAgentMessageDeltaParams struct {
	Delta    string `json:"delta"`
	ItemID   string `json:"itemId"`
	ThreadID string `json:"threadId"`
	TurnID   string `json:"turnId"`
}

type codexTurnInterruptParams struct {
	ThreadID string `json:"threadId"`
	TurnID   string `json:"turnId"`
}

type codexErrorNotification struct {
	Error     codexTurnError `json:"error"`
	ThreadID  string         `json:"threadId"`
	TurnID    string         `json:"turnId"`
	WillRetry bool           `json:"willRetry"`
}

type codexTurnError struct {
	Message string `json:"message"`
}

type codexItemNotification struct {
	Item     codexThreadItem `json:"item"`
	ThreadID string          `json:"threadId"`
	TurnID   string          `json:"turnId"`
}

type codexThreadItem struct {
	ID               string          `json:"id"`
	Type             string          `json:"type"`
	Tool             string          `json:"tool,omitempty"`
	Arguments        json.RawMessage `json:"arguments,omitempty"`
	Result           json.RawMessage `json:"result,omitempty"`
	Command          string          `json:"command,omitempty"`
	AggregatedOutput *string         `json:"aggregatedOutput,omitempty"`
	ExitCode         *int            `json:"exitCode,omitempty"`
	ContentItems     json.RawMessage `json:"contentItems,omitempty"`
	Changes          json.RawMessage `json:"changes,omitempty"`
}

type codexCommandApprovalRequest struct {
	ApprovalID *string `json:"approvalId,omitempty"`
	Command    *string `json:"command,omitempty"`
	Cwd        *string `json:"cwd,omitempty"`
	ItemID     string  `json:"itemId"`
	Reason     *string `json:"reason,omitempty"`
	ThreadID   string  `json:"threadId"`
	TurnID     string  `json:"turnId"`
}

type codexPermissionsApprovalRequest struct {
	ItemID      string          `json:"itemId"`
	Permissions json.RawMessage `json:"permissions"`
	Reason      *string         `json:"reason,omitempty"`
	ThreadID    string          `json:"threadId"`
	TurnID      string          `json:"turnId"`
}

type codexPendingApprovalKind string

const (
	codexApprovalCommand     codexPendingApprovalKind = "command"
	codexApprovalPermissions codexPendingApprovalKind = "permissions"
)

type codexPendingApproval struct {
	ID          int64
	Kind        codexPendingApprovalKind
	Permissions json.RawMessage
	// Description and Pattern mirror the rendered EventPermission payload so
	// PendingPermission() can answer without re-walking the approval params.
	// Populated at request time; cleared when RespondPermission fires.
	Description string
	Pattern     string
}

// CodexTransport implements Transport for the OpenAI Codex CLI running in
// app-server mode.
type CodexTransport struct {
	process codexProcess
	client  *Client
	events  chan Event

	closed    chan struct{}
	closeOnce sync.Once

	// autoApprove mirrors LaunchConfig.SkipPermissions for the lifetime of
	// this transport. When true, we reply "accept" to every approval
	// server-request from codex without surfacing a pane modal, matching
	// the user's declared intent when SkipPermissions was set. Codex
	// sometimes fires these even with approvalPolicy="never" — e.g. when
	// its sandbox blocks an OS-level call (AF_UNIX pipes, network) and it
	// wants the operator to re-run unsandboxed.
	autoApprove bool

	mu              sync.Mutex
	threadID        string
	currentTurnID   string
	pendingApproval *codexPendingApproval
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

// Start launches the Codex app-server subprocess, performs the documented
// initialize/initialized/thread-start handshake, and optionally starts the
// first turn with cfg.InitialPrompt.
func (t *CodexTransport) Start(ctx context.Context, cfg LaunchConfig) error {
	if strings.TrimSpace(cfg.Program) == "" {
		return fmt.Errorf("codex transport: empty program")
	}

	if !strings.Contains(cfg.Program, "app-server") && !strings.Contains(cfg.Program, "--server") {
		cfg.Program = strings.TrimSpace(cfg.Program) + " app-server"
	}

	stdin, stdout, err := t.process.Start(cfg)
	if err != nil {
		return fmt.Errorf("codex transport: start process: %w", err)
	}
	t.client = NewClient(stdin, stdout)

	t.autoApprove = cfg.SkipPermissions

	if err := t.startHandshake(ctx, cfg); err != nil {
		_ = t.client.Close()
		_ = t.process.Close()
		return err
	}

	go t.dispatchMessages()

	if cfg.InitialPrompt != "" {
		if err := t.SendPrompt(ctx, cfg.InitialPrompt); err != nil {
			_ = t.Close()
			return fmt.Errorf("codex transport: initial prompt: %w", err)
		}
	}
	return nil
}

func (t *CodexTransport) startHandshake(ctx context.Context, cfg LaunchConfig) error {
	initCtx, cancel := context.WithTimeout(ctx, codexHandshakeTimeout)
	defer cancel()

	var initResult codexInitializeResult
	if err := t.client.Call(initCtx, codexMethodInitialize, codexInitializeParams{
		ClientInfo: codexClientInfo{
			Name:    "kasmos",
			Title:   "kasmos",
			Version: codexClientVersion(),
		},
	}, &initResult); err != nil {
		return fmt.Errorf("codex transport: initialize: %w", err)
	}
	if err := t.client.Notify(initCtx, codexMethodInitialized, nil); err != nil {
		return fmt.Errorf("codex transport: initialized: %w", err)
	}

	threadParams := codexThreadStartParams{
		Cwd: cfg.WorkDir,
	}
	if cfg.SkipPermissions {
		threadParams.ApprovalPolicy = "never"
		threadParams.Sandbox = "danger-full-access"
	}

	var threadResult codexThreadStartResult
	if err := t.client.Call(initCtx, codexMethodThreadStart, threadParams, &threadResult); err != nil {
		return fmt.Errorf("codex transport: thread/start: %w", err)
	}
	if strings.TrimSpace(threadResult.Thread.ID) == "" {
		return fmt.Errorf("codex transport: thread/start returned empty thread id")
	}

	t.mu.Lock()
	t.threadID = threadResult.Thread.ID
	t.mu.Unlock()
	return nil
}

// SendPrompt starts a new Codex turn for the given prompt.
func (t *CodexTransport) SendPrompt(ctx context.Context, prompt string) error {
	if strings.TrimSpace(prompt) == "" {
		return nil
	}
	if err := t.guardClient(); err != nil {
		return err
	}

	t.mu.Lock()
	threadID := t.threadID
	activeTurnID := t.currentTurnID
	t.mu.Unlock()
	if strings.TrimSpace(threadID) == "" {
		return fmt.Errorf("codex transport: no thread id")
	}

	input := []codexUserInput{{Type: "text", Text: prompt}}

	// Steer the running turn when one exists; only start a fresh turn
	// when the agent is idle. Without this, user-typed prompts during an
	// active turn called turn/start, which codex rejects or silently
	// ignores because the thread already has an in-flight turn — i.e.
	// "interactive input doesn't appear to do anything".
	if strings.TrimSpace(activeTurnID) != "" {
		err := t.client.Call(ctx, codexMethodTurnSteer, codexTurnSteerParams{
			ThreadID:       threadID,
			ExpectedTurnID: activeTurnID,
			Input:          input,
		}, nil)
		if err == nil {
			return nil
		}
		// Fall through to turn/start if steer failed. A likely cause is
		// expectedTurnId drift (the original turn completed between the
		// mutex read and the call) — in that case starting a new turn is
		// the right fallback rather than silently dropping the prompt.
		// Emit the transient failure as a system event so the user can
		// see in the pane that a steer was retried as a fresh turn.
		t.emit(Event{
			Kind:      EventSystem,
			Text:      fmt.Sprintf("turn/steer failed (%v), starting new turn", err),
			Timestamp: time.Now(),
		})
	}

	var turnResult codexTurnStartResult
	err := t.client.Call(ctx, codexMethodTurnStart, codexTurnStartParams{
		ThreadID: threadID,
		Input:    input,
	}, &turnResult)
	if err != nil {
		return fmt.Errorf("codex transport: turn/start: %w", err)
	}
	if strings.TrimSpace(turnResult.Turn.ID) != "" {
		t.mu.Lock()
		t.currentTurnID = turnResult.Turn.ID
		t.mu.Unlock()
	}
	return nil
}

// Interrupt requests Codex to abort the current turn.
func (t *CodexTransport) Interrupt(ctx context.Context) error {
	if err := t.guardClient(); err != nil {
		return nil
	}

	t.mu.Lock()
	threadID := t.threadID
	turnID := t.currentTurnID
	t.mu.Unlock()
	if threadID == "" || turnID == "" {
		return nil
	}

	return t.client.Call(ctx, codexMethodTurnInterrupt, codexTurnInterruptParams{
		ThreadID: threadID,
		TurnID:   turnID,
	}, nil)
}

// PendingPermission returns the description + pattern of the pending
// approval, if any. Returns ok=false when no approval is in flight.
func (t *CodexTransport) PendingPermission() (description, pattern string, ok bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.pendingApproval == nil {
		return "", "", false
	}
	return t.pendingApproval.Description, t.pendingApproval.Pattern, true
}

// RespondPermission replies to the most recent pending approval request.
func (t *CodexTransport) RespondPermission(ctx context.Context, choice tmux.PermissionChoice) error {
	if err := t.guardClient(); err != nil {
		return nil
	}

	t.mu.Lock()
	pending := t.pendingApproval
	t.pendingApproval = nil
	t.mu.Unlock()
	if pending == nil {
		return nil
	}

	switch pending.Kind {
	case codexApprovalCommand:
		return t.client.Reply(ctx, pending.ID, map[string]any{
			"decision": codexCommandApprovalDecision(choice),
		})
	case codexApprovalPermissions:
		return t.client.Reply(ctx, pending.ID, codexPermissionsApprovalResponse(choice, pending.Permissions))
	default:
		return t.client.ReplyError(ctx, pending.ID, -32601, "unsupported approval type")
	}
}

// Events returns the read-only channel of structured events produced by the
// transport.
func (t *CodexTransport) Events() <-chan Event {
	return t.events
}

// PID returns the OS process ID of the running Codex process.
func (t *CodexTransport) PID() int {
	return t.process.PID()
}

// Close terminates the Codex subprocess and releases all resources.
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

func (t *CodexTransport) dispatchMessages() {
	defer close(t.events)

	notifications := t.client.Notifications()
	requests := t.client.Requests()
	for {
		select {
		case <-t.closed:
			return
		case n, ok := <-notifications:
			if !ok {
				return
			}
			ev, err := t.translateNotification(n)
			if err != nil {
				t.emit(Event{
					Kind:      EventSystem,
					Text:      fmt.Sprintf("codex: unknown notification %q: %v", n.Method, err),
					Timestamp: time.Now(),
				})
				continue
			}
			if ev != nil {
				t.emit(*ev)
			}
		case req, ok := <-requests:
			if !ok {
				return
			}
			if err := t.handleServerRequest(req); err != nil {
				t.emit(Event{
					Kind:      EventSystem,
					Text:      fmt.Sprintf("codex: server request %q failed: %v", req.Method, err),
					Timestamp: time.Now(),
				})
			}
		}
	}
}

func (t *CodexTransport) emit(ev Event) {
	select {
	case t.events <- ev:
	default:
	}
}

func (t *CodexTransport) translateNotification(n Notification) (*Event, error) {
	now := time.Now()

	switch n.Method {
	case codexNotifyThreadStarted,
		codexNotifyMcpStartupStatus,
		codexNotifyThreadStatus,
		codexNotifyAccountRateLimits,
		codexNotifyThreadTokenUsage,
		codexNotifyServerRequestDone,
		codexNotifyTurnPlanUpdated,
		codexNotifyTurnDiffUpdated,
		codexNotifyCommandExecOutputDelta:
		return nil, nil

	case codexNotifyTurnStarted:
		var p codexTurnStartedParams
		if err := json.Unmarshal(n.Params, &p); err != nil {
			return nil, fmt.Errorf("unmarshal turn/started params: %w", err)
		}
		t.mu.Lock()
		t.currentTurnID = p.Turn.ID
		t.mu.Unlock()
		return &Event{
			Kind:      EventTurnStarted,
			TurnID:    p.Turn.ID,
			Timestamp: now,
		}, nil

	case codexNotifyAgentMessageDelta:
		var p codexAgentMessageDeltaParams
		if err := json.Unmarshal(n.Params, &p); err != nil {
			return nil, fmt.Errorf("unmarshal item/agentMessage/delta params: %w", err)
		}
		return &Event{
			Kind:      EventTextDelta,
			TurnID:    p.TurnID,
			Text:      p.Delta,
			Timestamp: now,
		}, nil

	case codexNotifyItemStarted:
		var p codexItemNotification
		if err := json.Unmarshal(n.Params, &p); err != nil {
			return nil, fmt.Errorf("unmarshal item/started params: %w", err)
		}
		return translateCodexItemEvent(p, false, now), nil

	case codexNotifyItemCompleted:
		var p codexItemNotification
		if err := json.Unmarshal(n.Params, &p); err != nil {
			return nil, fmt.Errorf("unmarshal item/completed params: %w", err)
		}
		return translateCodexItemEvent(p, true, now), nil

	case codexNotifyTurnCompleted:
		var p codexTurnCompletedParams
		if err := json.Unmarshal(n.Params, &p); err != nil {
			return nil, fmt.Errorf("unmarshal turn/completed params: %w", err)
		}
		t.mu.Lock()
		if t.currentTurnID == p.Turn.ID {
			t.currentTurnID = ""
		}
		t.mu.Unlock()
		return &Event{
			Kind:      EventTurnCompleted,
			TurnID:    p.Turn.ID,
			HasPrompt: true,
			Final:     true,
			Timestamp: now,
		}, nil

	case codexNotifyError:
		var p codexErrorNotification
		if err := json.Unmarshal(n.Params, &p); err != nil {
			return nil, fmt.Errorf("unmarshal error params: %w", err)
		}
		t.mu.Lock()
		if p.TurnID != "" && t.currentTurnID == p.TurnID && !p.WillRetry {
			t.currentTurnID = ""
		}
		t.mu.Unlock()
		return &Event{
			Kind:      EventSystem,
			TurnID:    p.TurnID,
			Text:      "codex error: " + strings.TrimSpace(p.Error.Message),
			HasPrompt: !p.WillRetry,
			Final:     !p.WillRetry,
			Timestamp: now,
		}, nil

	default:
		return nil, fmt.Errorf("unrecognised method %q", n.Method)
	}
}

func (t *CodexTransport) handleServerRequest(req ServerRequest) error {
	now := time.Now()

	switch req.Method {
	case codexRequestCommandApproval:
		var p codexCommandApprovalRequest
		if err := json.Unmarshal(req.Params, &p); err != nil {
			return fmt.Errorf("unmarshal %s: %w", req.Method, err)
		}
		if t.autoApprove {
			// Daemon-spawned agents run with SkipPermissions=true — the
			// operator has declared up front that these should run
			// unattended. Reply "accept" directly instead of stalling the
			// turn on a modal the user has no reason to see. Codex fires
			// these even with approvalPolicy="never" when its sandbox
			// blocks an OS-level call (tsx ipc pipe, http, etc.).
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			return t.client.Reply(ctx, req.ID, map[string]any{"decision": "accept"})
		}
		desc := codexCommandApprovalDescription(p)
		t.mu.Lock()
		t.pendingApproval = &codexPendingApproval{
			ID:          req.ID,
			Kind:        codexApprovalCommand,
			Description: desc,
		}
		t.mu.Unlock()
		t.emit(Event{
			Kind:                  EventPermission,
			TurnID:                p.TurnID,
			PermissionDescription: desc,
			Timestamp:             now,
		})
		return nil

	case codexRequestPermissionsApproval:
		var p codexPermissionsApprovalRequest
		if err := json.Unmarshal(req.Params, &p); err != nil {
			return fmt.Errorf("unmarshal %s: %w", req.Method, err)
		}
		if t.autoApprove {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			return t.client.Reply(ctx, req.ID, codexPermissionsApprovalResponse(tmux.PermissionAllowAlways, p.Permissions))
		}
		desc := codexPermissionsApprovalDescription(p)
		t.mu.Lock()
		t.pendingApproval = &codexPendingApproval{
			ID:          req.ID,
			Kind:        codexApprovalPermissions,
			Permissions: p.Permissions,
			Description: desc,
		}
		t.mu.Unlock()
		t.emit(Event{
			Kind:                  EventPermission,
			TurnID:                p.TurnID,
			PermissionDescription: desc,
			Timestamp:             now,
		})
		return nil

	case codexRequestElicitation:
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := t.client.ReplyError(ctx, req.ID, -32601, "elicitation not supported"); err != nil {
			return fmt.Errorf("reject %s: %w", req.Method, err)
		}
		return nil

	default:
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := t.client.ReplyError(ctx, req.ID, -32601, "unsupported by kasmos"); err != nil {
			return fmt.Errorf("reject unsupported %s: %w", req.Method, err)
		}
		return fmt.Errorf("unsupported server request %q", req.Method)
	}
}

func translateCodexItemEvent(p codexItemNotification, completed bool, now time.Time) *Event {
	switch p.Item.Type {
	case "commandExecution":
		if !completed {
			return &Event{
				Kind:      EventToolCall,
				TurnID:    p.TurnID,
				ToolName:  "commandExecution",
				ToolInput: p.Item.Command,
				Timestamp: now,
			}
		}
		return &Event{
			Kind:       EventToolResult,
			TurnID:     p.TurnID,
			ToolName:   "commandExecution",
			ToolResult: codexCommandExecutionResult(p.Item),
			Timestamp:  now,
		}

	case "mcpToolCall", "dynamicToolCall", "collabAgentToolCall":
		if !completed {
			return &Event{
				Kind:      EventToolCall,
				TurnID:    p.TurnID,
				ToolName:  codexToolName(p.Item),
				ToolInput: codexRawString(p.Item.Arguments),
				Timestamp: now,
			}
		}
		return &Event{
			Kind:       EventToolResult,
			TurnID:     p.TurnID,
			ToolName:   codexToolName(p.Item),
			ToolResult: codexToolResult(p.Item),
			Timestamp:  now,
		}

	case "fileChange":
		if !completed {
			return &Event{
				Kind:      EventToolCall,
				TurnID:    p.TurnID,
				ToolName:  "fileChange",
				ToolInput: codexRawString(p.Item.Changes),
				Timestamp: now,
			}
		}
		return &Event{
			Kind:       EventToolResult,
			TurnID:     p.TurnID,
			ToolName:   "fileChange",
			ToolResult: codexRawString(p.Item.Changes),
			Timestamp:  now,
		}

	default:
		return nil
	}
}

func codexCommandExecutionDescription(item codexThreadItem) string {
	if strings.TrimSpace(item.Command) != "" {
		return item.Command
	}
	if item.ExitCode != nil {
		return fmt.Sprintf("exit_code=%d", *item.ExitCode)
	}
	return ""
}

func codexCommandExecutionResult(item codexThreadItem) string {
	if item.AggregatedOutput != nil && strings.TrimSpace(*item.AggregatedOutput) != "" {
		if item.ExitCode != nil {
			return fmt.Sprintf("exit_code=%d output=%s", *item.ExitCode, strings.TrimSpace(*item.AggregatedOutput))
		}
		return strings.TrimSpace(*item.AggregatedOutput)
	}
	return codexCommandExecutionDescription(item)
}

func codexToolName(item codexThreadItem) string {
	if strings.TrimSpace(item.Tool) != "" {
		return item.Tool
	}
	return item.Type
}

func codexToolResult(item codexThreadItem) string {
	// Codex 0.121 emits `result: null` on mcpToolCall items whose payload is
	// carried in `contentItems` instead (e.g. MCP tools that return text
	// content blocks). Treat the literal JSON null as "no result" so we fall
	// through to contentItems rather than rendering "[result: null]".
	if codexRawHasContent(item.Result) {
		return codexRawString(item.Result)
	}
	if codexRawHasContent(item.ContentItems) {
		return codexRawString(item.ContentItems)
	}
	return ""
}

// codexRawHasContent reports whether raw holds a JSON value other than the
// null literal or whitespace. Used to decide whether a RawMessage should be
// treated as "present" for rendering priority.
func codexRawHasContent(raw json.RawMessage) bool {
	trimmed := strings.TrimSpace(string(raw))
	return trimmed != "" && trimmed != "null"
}

func codexRawString(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	return strings.TrimSpace(string(raw))
}

func codexCommandApprovalDescription(p codexCommandApprovalRequest) string {
	parts := []string{}
	if p.Reason != nil && strings.TrimSpace(*p.Reason) != "" {
		parts = append(parts, strings.TrimSpace(*p.Reason))
	}
	if p.Command != nil && strings.TrimSpace(*p.Command) != "" {
		parts = append(parts, strings.TrimSpace(*p.Command))
	}
	if p.Cwd != nil && strings.TrimSpace(*p.Cwd) != "" {
		parts = append(parts, "cwd="+strings.TrimSpace(*p.Cwd))
	}
	if len(parts) == 0 {
		return "codex requested command approval"
	}
	return strings.Join(parts, " | ")
}

func codexPermissionsApprovalDescription(p codexPermissionsApprovalRequest) string {
	if p.Reason != nil && strings.TrimSpace(*p.Reason) != "" {
		return strings.TrimSpace(*p.Reason)
	}
	if len(p.Permissions) > 0 {
		return "codex requested permissions: " + codexRawString(p.Permissions)
	}
	return "codex requested additional permissions"
}

func codexCommandApprovalDecision(choice tmux.PermissionChoice) any {
	switch choice {
	case tmux.PermissionAllowAlways:
		return "acceptForSession"
	case tmux.PermissionReject:
		return "decline"
	default:
		return "accept"
	}
}

func codexPermissionsApprovalResponse(choice tmux.PermissionChoice, requested json.RawMessage) map[string]any {
	perms := map[string]any{}
	if choice != tmux.PermissionReject && len(requested) > 0 {
		_ = json.Unmarshal(requested, &perms)
	}

	scope := "turn"
	if choice == tmux.PermissionAllowAlways {
		scope = "session"
	}
	return map[string]any{
		"permissions": perms,
		"scope":       scope,
	}
}

func codexClientVersion() string {
	if bi, ok := debug.ReadBuildInfo(); ok {
		if strings.TrimSpace(bi.Main.Version) != "" && bi.Main.Version != "(devel)" {
			return bi.Main.Version
		}
	}
	return "dev"
}
