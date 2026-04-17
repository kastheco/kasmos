package taskfsm

import (
	"fmt"
	"strings"
	"time"

	"github.com/kastheco/kasmos/config/taskstate"
	"github.com/kastheco/kasmos/config/taskstore"
)

// Status represents the lifecycle state of a plan.
type Status string

// ExecutionPhase represents the persisted fine-grained execution substate.
type ExecutionPhase string

const (
	StatusReady        Status = "ready"
	StatusPlanning     Status = "planning"
	StatusImplementing Status = "implementing"
	StatusReviewing    Status = "reviewing"
	StatusVerifying    Status = "verifying"
	StatusDone         Status = "done"
	StatusCancelled    Status = "cancelled"

	ExecutionPhasePlanned                 ExecutionPhase = "planned"
	ExecutionPhaseArchitecting            ExecutionPhase = "architecting"
	ExecutionPhaseWaveRunning             ExecutionPhase = "wave_running"
	ExecutionPhaseWaveWaiting             ExecutionPhase = "wave_waiting"
	ExecutionPhaseSingleAgentImplementing ExecutionPhase = "single_agent_implementing"
	ExecutionPhaseFixing                  ExecutionPhase = "fixing"
	ExecutionPhaseReviewing               ExecutionPhase = "reviewing"
)

// Event represents a lifecycle transition trigger.
type Event string

const (
	PlanStart              Event = "plan_start"
	PlannerFinished        Event = "planner_finished"
	ImplementStart         Event = "implement_start"
	ImplementFinished      Event = "implement_finished"
	ReviewApproved         Event = "review_approved"
	ReviewChangesRequested Event = "review_changes_requested"
	VerifyApproved         Event = "verify_approved"
	VerifyFailed           Event = "verify_failed"
	RequestReview          Event = "request_review"
	StartOver              Event = "start_over"
	Reimplement            Event = "reimplement"
	Cancel                 Event = "cancel"
	Reopen                 Event = "reopen"
	MarkDone               Event = "mark_done"
)

// NormalizeExecutionPhase trims persisted execution-phase strings and returns
// the canonical enum form used by lifecycle helpers.
func NormalizeExecutionPhase(phase string) ExecutionPhase {
	return ExecutionPhase(strings.TrimSpace(phase))
}

// IsWaveExecutionPhase reports whether the phase is one of the persisted
// multi-task wave execution states.
func IsWaveExecutionPhase(phase ExecutionPhase) bool {
	switch phase {
	case ExecutionPhaseWaveRunning, ExecutionPhaseWaveWaiting:
		return true
	default:
		return false
	}
}

// IsSingleAgentImplementingPhase reports whether the phase represents a single
// coder/fixer implementation pass that can advance directly to review.
func IsSingleAgentImplementingPhase(phase ExecutionPhase) bool {
	switch phase {
	case ExecutionPhaseSingleAgentImplementing, ExecutionPhaseFixing:
		return true
	default:
		return false
	}
}

// IsUserOnly returns true if this event can only be triggered from the TUI,
// never by agent sentinel files.
func (e Event) IsUserOnly() bool {
	switch e {
	case StartOver, Reimplement, RequestReview, Cancel, Reopen, MarkDone:
		return true
	}
	return false
}

// transitionTable defines all valid state transitions.
// Key: current status → event → new status.
var transitionTable = map[Status]map[Event]Status{
	StatusReady: {
		PlanStart:      StatusPlanning,
		ImplementStart: StatusImplementing,
		MarkDone:       StatusDone,
		Cancel:         StatusCancelled,
	},
	StatusPlanning: {
		PlanStart:       StatusPlanning, // allow restart after crash/interrupt
		PlannerFinished: StatusReady,
		Cancel:          StatusCancelled,
	},
	StatusImplementing: {
		ImplementFinished: StatusReviewing,
		Cancel:            StatusCancelled,
	},
	StatusReviewing: {
		ReviewApproved:         StatusVerifying,
		ReviewChangesRequested: StatusImplementing,
		Cancel:                 StatusCancelled,
	},
	StatusVerifying: {
		VerifyApproved: StatusDone,
		VerifyFailed:   StatusImplementing,
		Cancel:         StatusCancelled,
	},
	StatusDone: {
		StartOver:     StatusPlanning,
		Reimplement:   StatusImplementing, // resume implementation without resetting branch
		RequestReview: StatusReviewing,    // retrigger review for unmerged branches
		Cancel:        StatusCancelled,    // explicit user cancellation from done
	},
	StatusCancelled: {
		Reopen: StatusPlanning,
	},
}

// ApplyTransition returns the new status for the given current status and event.
// Returns an error if the transition is not valid.
func ApplyTransition(current Status, event Event) (Status, error) {
	events, ok := transitionTable[current]
	if !ok {
		return "", fmt.Errorf("no transitions defined for status %q", current)
	}
	next, ok := events[event]
	if !ok {
		return "", fmt.Errorf("invalid transition: %q + %q", current, event)
	}
	return next, nil
}

// TransitionExecutionState returns the persisted execution metadata that should
// accompany a successful lifecycle transition.
func TransitionExecutionState(event Event, next Status) taskstore.ExecutionState {
	if next == StatusReady && event == PlannerFinished {
		return taskstore.ExecutionState{Phase: string(ExecutionPhasePlanned)}
	}
	return taskstore.ExecutionState{}
}

// TaskStateMachine is the sole writer of plan state. All plan status mutations
// must flow through Transition(). The store handles concurrency via SQLite.
type TaskStateMachine struct {
	dir     string          // legacy: retained for file rename operations (may be empty)
	store   taskstore.Store // always non-nil
	project string          // project name used with the store
	hooks   *HookRegistry   // optional; fired asynchronously after each successful transition
}

// New creates a TaskStateMachine backed by the given store.
func New(store taskstore.Store, project, dir string) *TaskStateMachine {
	return &TaskStateMachine{dir: dir, store: store, project: project}
}

// SetHooks attaches a HookRegistry that will receive TransitionEvents after
// each successful state write.
func (m *TaskStateMachine) SetHooks(h *HookRegistry) { m.hooks = h }

// Transition applies an event to a plan's current status. It reads the current
// state from the store, validates the transition, writes the new state, and returns.
// Concurrency is handled server-side via SQLite's own locking.
func (m *TaskStateMachine) Transition(planFile string, event Event) error {
	ps, err := taskstate.Load(m.store, m.project, m.dir)
	if err != nil {
		return fmt.Errorf("load plan state: %w", err)
	}
	entry, ok := ps.Entry(planFile)
	if !ok {
		return fmt.Errorf("plan not found: %s", planFile)
	}
	currentStatus := MapLegacyStatus(entry.Status)
	newStatus, err := ApplyTransition(currentStatus, event)
	if err != nil {
		return err
	}
	if err := ps.ForceSetLifecycle(planFile, taskstate.Status(newStatus), TransitionExecutionState(event, newStatus)); err != nil {
		return err
	}
	if phase, ok := phaseNameForStatus(newStatus); ok {
		if err := m.store.SetPhaseTimestamp(m.project, planFile, phase, time.Now().UTC()); err != nil {
			return fmt.Errorf("set phase timestamp: %w", err)
		}
	}
	m.hooks.FireAll(TransitionEvent{
		PlanFile:   planFile,
		FromStatus: currentStatus,
		ToStatus:   newStatus,
		Event:      event,
		Timestamp:  time.Now().UTC(),
		Project:    m.project,
	})
	return nil
}

func phaseNameForStatus(s Status) (string, bool) {
	switch s {
	case StatusPlanning:
		return "planning", true
	case StatusImplementing:
		return "implementing", true
	case StatusReviewing:
		return "reviewing", true
	case StatusVerifying:
		return "verifying", true
	case StatusDone:
		return "done", true
	default:
		return "", false
	}
}

// MapLegacyStatus converts statuses imported through the explicit legacy
// migration paths into canonical FSM statuses.
//
// Keep these aliases only as a last-resort reader boundary for already-persisted
// task-store rows created before legacy imports were normalized at ingest. New
// legacy imports map these aliases to canonical statuses before writing.
//
// Exported so callers outside taskfsm (for example the web task-actions handler)
// can normalize persisted legacy values before invoking ApplyTransition.
func MapLegacyStatus(s taskstate.Status) Status {
	switch s {
	case "in_progress":
		return StatusImplementing
	case "completed":
		return StatusDone
	default:
		return Status(s)
	}
}
