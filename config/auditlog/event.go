package auditlog

import "time"

// EventKind identifies the type of audit event.
type EventKind string

// String returns the string representation of the EventKind.
func (k EventKind) String() string {
	return string(k)
}

// Lifecycle events.
const (
	EventAgentSpawned   EventKind = "agent_spawned"
	EventAgentFinished  EventKind = "agent_finished"
	EventAgentKilled    EventKind = "agent_killed"
	EventAgentPaused    EventKind = "agent_paused"
	EventAgentResumed   EventKind = "agent_resumed"
	EventAgentRestarted EventKind = "agent_restarted"
)

// Plan events.
const (
	EventPlanTransition EventKind = "plan_transition"
	EventPlanCreated    EventKind = "plan_created"
	EventPlanMerged     EventKind = "plan_merged"
	EventPlanCancelled  EventKind = "plan_cancelled"
)

// Wave events.
const (
	EventWaveStarted   EventKind = "wave_started"
	EventWaveCompleted EventKind = "wave_completed"
	EventWaveFailed    EventKind = "wave_failed"
)

// Linear link events.
const (
	EventTaskLinearLinked   EventKind = "task_linear_linked"
	EventTaskLinearUnlinked EventKind = "task_linear_unlinked"
)

// Linear receipt events.
const (
	EventTaskLinearReceiptPosted EventKind = "task_linear_receipt_posted"
	EventTaskLinearReceiptFailed EventKind = "task_linear_receipt_failed"
)

// Linear trigger events (Phase 4 inbound).
const (
	EventTaskLinearTriggerReceived      EventKind = "task_linear_trigger_received"
	EventTaskLinearTriggerDispatched    EventKind = "task_linear_trigger_dispatched"
	EventTaskLinearTriggerRejected      EventKind = "task_linear_trigger_rejected"
	EventTaskLinearTriggerIgnored       EventKind = "task_linear_trigger_ignored"
	EventTaskLinearTriggerCommentFailed EventKind = "task_linear_trigger_comment_failed"
)

// Linear webhook events (Phase 5 inbound HTTP).
const (
	EventTaskLinearWebhookReceived  EventKind = "task_linear_webhook_received"
	EventTaskLinearWebhookAccepted  EventKind = "task_linear_webhook_accepted"
	EventTaskLinearWebhookDuplicate EventKind = "task_linear_webhook_duplicate"
	EventTaskLinearWebhookIgnored   EventKind = "task_linear_webhook_ignored"
	EventTaskLinearWebhookRejected  EventKind = "task_linear_webhook_rejected"
	EventTaskLinearWebhookFailed    EventKind = "task_linear_webhook_failed"
)

// Operational events.
const (
	EventPromptSent         EventKind = "prompt_sent"
	EventShellRan           EventKind = "shell_ran"
	EventGitPush            EventKind = "git_push"
	EventPRCreated          EventKind = "pr_created"
	EventPermissionDetected EventKind = "permission_detected"
	EventPermissionAnswered EventKind = "permission_answered"
	EventFSMError           EventKind = "fsm_error"
	EventError              EventKind = "error"
)

// Session lifecycle events.
const (
	EventSessionStarted EventKind = "session_started"
	EventSessionStopped EventKind = "session_stopped"
)

// Event is a single audit log entry.
type Event struct {
	ID            int64     `json:"id"`
	Kind          EventKind `json:"kind"`
	Timestamp     time.Time `json:"timestamp"`
	Project       string    `json:"project"`
	TaskFile      string    `json:"task_file"`
	InstanceTitle string    `json:"instance_title"`
	AgentType     string    `json:"agent_type"`
	WaveNumber    int       `json:"wave_number"`
	TaskNumber    int       `json:"task_number"`
	Message       string    `json:"message"`
	Detail        string    `json:"detail"` // JSON-encoded extra data
	Level         string    `json:"level"`  // info, warn, error
}
