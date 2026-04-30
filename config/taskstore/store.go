// Package taskstore provides a Store interface for task state persistence,
// with a SQLite implementation for direct DB access and an HTTP implementation
// for client-server communication.
package taskstore

import (
	"errors"
	"fmt"
	"time"
)

// ErrNotFound marks task-store missing-resource errors. Store implementations
// may preserve their historical Error strings while still matching this
// sentinel through errors.Is.
var ErrNotFound = errors.New("not found")

type notFoundError struct {
	msg string
}

func newNotFoundError(format string, args ...any) error {
	return notFoundError{msg: fmt.Sprintf(format, args...)}
}

func (e notFoundError) Error() string {
	return e.msg
}

func (e notFoundError) Is(target error) bool {
	return target == ErrNotFound
}

// ExecutionState captures finer-grained execution lifecycle metadata.
type ExecutionState struct {
	Phase           string `json:"execution_phase,omitempty"`
	ActiveAgentType string `json:"active_agent_type,omitempty"`
	ActiveWave      int    `json:"active_wave,omitempty"`
}

// PRReviewEntry holds a persisted PR review record for a single plan.
type PRReviewEntry struct {
	ReviewID        int       `json:"review_id"`
	ReviewState     string    `json:"review_state"`
	ReviewBody      string    `json:"review_body"`
	ReviewerLogin   string    `json:"reviewer_login"`
	ReactionPosted  bool      `json:"reaction_posted"`
	FixerDispatched bool      `json:"fixer_dispatched"`
	CreatedAt       time.Time `json:"created_at,omitempty"`
}

// LinearTriggerEntry is one persisted inbound Linear trigger.
type LinearTriggerEntry struct {
	ID               int64     `json:"id"`
	LinearIssueID    string    `json:"linear_issue_id"`
	LinearIdentifier string    `json:"linear_identifier"`
	CommandKind      string    `json:"command_kind"`
	SourceKind       string    `json:"source_kind"`
	SourceID         string    `json:"source_id"`
	ActorID          string    `json:"actor_id"`
	ActorEmail       string    `json:"actor_email"`
	TaskArg          string    `json:"task_arg"`
	DetectedAt       time.Time `json:"detected_at"`
	Processed        bool      `json:"processed"`
	ProcessedAt      time.Time `json:"processed_at"`
	Outcome          string    `json:"outcome"`
	RejectionReason  string    `json:"rejection_reason"`
	TargetFilename   string    `json:"target_filename"`
	AckState         string    `json:"ack_state"`
}

// Status represents the lifecycle state of a plan.
// These constants mirror taskstate.Status to keep taskstore self-contained
// and avoid circular imports.
type Status string

const (
	StatusReady        Status = "ready"
	StatusDone         Status = "done"
	StatusReviewing    Status = "reviewing"
	StatusVerifying    Status = "verifying"
	StatusCancelled    Status = "cancelled"
	StatusPlanning     Status = "planning"
	StatusImplementing Status = "implementing"
)

// TaskEntry holds the persisted metadata for a single plan.
type TaskEntry struct {
	ExecutionState       ExecutionState `json:"execution_state,omitempty"`
	Filename             string         `json:"filename"`
	Status               Status         `json:"status"`
	Description          string         `json:"description,omitempty"`
	Branch               string         `json:"branch,omitempty"`
	Topic                string         `json:"topic,omitempty"`
	CreatedAt            time.Time      `json:"created_at,omitempty"`
	Implemented          string         `json:"implemented,omitempty"`
	PlanningAt           time.Time      `json:"planning_at,omitempty"`
	ImplementingAt       time.Time      `json:"implementing_at,omitempty"`
	ReviewingAt          time.Time      `json:"reviewing_at,omitempty"`
	VerifyingAt          time.Time      `json:"verifying_at,omitempty"`
	DoneAt               time.Time      `json:"done_at,omitempty"`
	Goal                 string         `json:"goal,omitempty"`
	Content              string         `json:"content,omitempty"`
	ClickUpTaskID        string         `json:"clickup_task_id,omitempty"`
	LinearIssueID        string         `json:"linear_issue_id,omitempty"`
	LinearIdentifier     string         `json:"linear_identifier,omitempty"`
	LinearURL            string         `json:"linear_url,omitempty"`
	LinearTeamKey        string         `json:"linear_team_key,omitempty"`
	LinearProjectID      string         `json:"linear_project_id,omitempty"`
	ReviewCycle          int            `json:"review_cycle,omitempty"`
	LatestReviewFeedback string         `json:"latest_review_feedback,omitempty"`
	PRURL                string         `json:"pr_url,omitempty"`
	PRReviewDecision     string         `json:"pr_review_decision,omitempty"`
	PRCheckStatus        string         `json:"pr_check_status,omitempty"`
}

// LinearLink holds the persisted Linear issue coordinates for a task.
type LinearLink struct {
	LinearIssueID    string `json:"linear_issue_id,omitempty"`
	LinearIdentifier string `json:"linear_identifier,omitempty"`
	LinearURL        string `json:"linear_url,omitempty"`
	LinearTeamKey    string `json:"linear_team_key,omitempty"`
	LinearProjectID  string `json:"linear_project_id,omitempty"`
	IssueID          string `json:"-"`
	Identifier       string `json:"-"`
	URL              string `json:"-"`
	TeamKey          string `json:"-"`
	ProjectID        string `json:"-"`
}

func normalizedLinearLink(link LinearLink) LinearLink {
	if link.LinearIssueID == "" {
		link.LinearIssueID = link.IssueID
	}
	if link.LinearIdentifier == "" {
		link.LinearIdentifier = link.Identifier
	}
	if link.LinearURL == "" {
		link.LinearURL = link.URL
	}
	if link.LinearTeamKey == "" {
		link.LinearTeamKey = link.TeamKey
	}
	if link.LinearProjectID == "" {
		link.LinearProjectID = link.ProjectID
	}
	return link
}

// ExecutionStateWriter persists execution lifecycle metadata without rewriting
// the rest of a task entry.
type ExecutionStateWriter interface {
	SetExecutionState(project, filename string, state ExecutionState) error
}

// LinearTriggerStore persists inbound Linear trigger events.
type LinearTriggerStore interface {
	// EnqueueLinearTrigger inserts a row using INSERT ... ON CONFLICT DO NOTHING.
	// Returns queued=true when a new row landed; false when the unique key was
	// already present (replay-safe).
	EnqueueLinearTrigger(project string, e LinearTriggerEntry) (queued bool, err error)
	// MarkLinearTriggerDispatched marks an enqueued row as successfully dispatched.
	// targetFilename is the resulting kasmos task file (empty for help/status).
	MarkLinearTriggerDispatched(project string, id int64, targetFilename string) error
	// MarkLinearTriggerRejected records why dispatch was refused.
	MarkLinearTriggerRejected(project string, id int64, reason string) error
	// MarkLinearTriggerIgnored records that a recognised event did not produce a dispatch.
	MarkLinearTriggerIgnored(project string, id int64, reason string) error
	// MarkLinearTriggerFailed records a non-rejection error (e.g. mid-dispatch crash).
	MarkLinearTriggerFailed(project string, id int64, reason string) error
	// MarkLinearTriggerAck records ack outcome ("acked" or "ack_failed").
	MarkLinearTriggerAck(project string, id int64, ackState string) error
	// ListUnprocessedLinearTriggers returns rows with processed=0 in detected_at ASC order, capped at limit.
	ListUnprocessedLinearTriggers(project string, limit int) ([]LinearTriggerEntry, error)
	// LastSeenCommentAt returns the cursor for an issue, or zero time when unknown.
	LastSeenCommentAt(project, linearIssueID string) (time.Time, error)
	// SetLastSeenCommentAt updates the cursor monotonically; SET only when at > current.
	SetLastSeenCommentAt(project, linearIssueID string, at time.Time) error
}

// SubtaskStatus represents the lifecycle state of a subtask.
type SubtaskStatus string

const (
	SubtaskStatusPending  SubtaskStatus = "pending"
	SubtaskStatusRunning  SubtaskStatus = "running"
	SubtaskStatusComplete SubtaskStatus = "complete"
	SubtaskStatusFailed   SubtaskStatus = "failed"
	SubtaskStatusClosed   SubtaskStatus = "closed"
	SubtaskStatusDone     SubtaskStatus = "done"
	SubtaskStatusBlocked  SubtaskStatus = "blocked"
	SubtaskStatusInReview SubtaskStatus = "in_review"
)

// SubtaskEntry holds a persisted subtask for a single plan.
type SubtaskEntry struct {
	TaskNumber int           `json:"task_number"`
	Title      string        `json:"title"`
	Status     SubtaskStatus `json:"status"`
}

// TopicEntry holds the persisted metadata for a topic grouping.
type TopicEntry struct {
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

// Store is the interface for plan state persistence. Implementations include
// SQLiteStore (direct DB access, used by the server) and HTTPStore (client
// that talks to the server over HTTP).
type Store interface {
	// Plan CRUD
	Create(project string, entry TaskEntry) error
	Get(project, filename string) (TaskEntry, error)
	Update(project, filename string, entry TaskEntry) error
	Rename(project, oldFilename, newFilename string) error
	Delete(project, filename string) error

	// Content access
	GetContent(project, filename string) (string, error)
	SetContent(project, filename, content string) error

	// Subtasks
	SetSubtasks(project, filename string, subtasks []SubtaskEntry) error
	GetSubtasks(project, filename string) ([]SubtaskEntry, error)
	UpdateSubtaskStatus(project, filename string, taskNumber int, status SubtaskStatus) error

	// Phase timestamps
	SetPhaseTimestamp(project, filename, phase string, ts time.Time) error

	// ClickUp integration
	SetClickUpTaskID(project, filename, taskID string) error

	// Linear integration
	SetLinearLink(project, filename string, link LinearLink) error
	SetLinearLinkIfNoActiveDuplicate(project, filename string, link LinearLink, statuses ...Status) (string, error)
	ClearLinearLink(project, filename string) error
	FindLinkedTask(project, issueID string, statuses ...Status) (string, error)

	// Review cycle
	IncrementReviewCycle(project, filename string) error

	// Plan goals
	SetPlanGoal(project, filename, goal string) error

	// PR metadata
	SetPRURL(project, filename, url string) error
	SetPRState(project, filename, reviewDecision, checkStatus string) error

	// PR reviews
	RecordPRReview(project, filename string, reviewID int, state, body, reviewer string) error
	IsReviewProcessed(project, filename string, reviewID int) bool
	MarkReviewReacted(project, filename string, reviewID int) error
	MarkReviewFixerDispatched(project, filename string, reviewID int) error
	ListPendingReviews(project, filename string) ([]PRReviewEntry, error)

	// Linear triggers
	LinearTriggerStore

	// Queries
	List(project string) ([]TaskEntry, error)
	ListByStatus(project string, statuses ...Status) ([]TaskEntry, error)
	ListByTopic(project, topic string) ([]TaskEntry, error)

	// Topics
	ListTopics(project string) ([]TopicEntry, error)
	CreateTopic(project string, entry TopicEntry) error

	// Health
	Ping() error

	// Close releases any resources held by the store.
	Close() error
}
