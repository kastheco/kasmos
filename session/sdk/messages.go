package sdk

import "time"

// EventKind identifies the type of a structured event emitted by a Transport.
type EventKind string

const (
	// EventTurnStarted is emitted when the agent begins a new response turn.
	EventTurnStarted EventKind = "turn_started"
	// EventTextDelta carries an incremental text chunk from the agent's response.
	EventTextDelta EventKind = "text_delta"
	// EventToolCall is emitted when the agent invokes a tool.
	EventToolCall EventKind = "tool_call"
	// EventToolResult carries the result returned from a tool invocation.
	EventToolResult EventKind = "tool_result"
	// EventPermission is emitted when the agent requests a permission approval
	// from the operator.
	EventPermission EventKind = "permission"
	// EventTurnCompleted is emitted when the agent finishes a turn normally.
	EventTurnCompleted EventKind = "turn_completed"
	// EventTurnInterrupted is emitted when an in-progress agent turn is
	// interrupted (either by the operator or the agent itself).
	EventTurnInterrupted EventKind = "turn_interrupted"
	// EventSystem carries system-level messages such as startup, shutdown, or
	// non-fatal errors from the transport layer.
	EventSystem EventKind = "system"
)

// Event is a program-agnostic structured event produced by a Transport.
//
// Only fields relevant to the EventKind are populated; all others are zero.
// The design is intentionally flat so session/sdk/session.go can convert events
// to preview text without reflection or type switches over a large union type.
type Event struct {
	// Kind classifies the event.
	Kind EventKind

	// TurnID uniquely identifies the agent turn this event belongs to.
	// Set on EventTurnStarted and carried through to EventTurnCompleted /
	// EventTurnInterrupted so the consumer can correlate events.
	TurnID string

	// Text is the content for EventTextDelta (incremental text chunk) and
	// EventSystem (human-readable status or error message).
	Text string

	// ToolName identifies the tool being called (EventToolCall) or whose
	// result is being returned (EventToolResult).
	ToolName string

	// ToolInput is the JSON-encoded arguments for EventToolCall.
	ToolInput string

	// ToolResult is the JSON-encoded result for EventToolResult.
	ToolResult string

	// PermissionDescription is a human-readable description of the action
	// requiring approval (EventPermission).
	PermissionDescription string

	// PermissionPattern is the glob or regex pattern being approved
	// (EventPermission).
	PermissionPattern string

	// HasPrompt is true when the agent is idle and ready to accept a new prompt.
	// Consumers can use this flag to update the "waiting for input" indicator.
	HasPrompt bool

	// Final is true when this event marks the definitive end of a turn or stream.
	// After a Final event the consumer should not expect further events for the
	// same TurnID.
	Final bool

	// Timestamp records when the event was produced by the transport layer.
	Timestamp time.Time
}
