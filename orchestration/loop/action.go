// Package loop defines the action types emitted by the signal processor and
// the AgentSpawner interface used by the daemon and TUI to perform side effects.
package loop

import (
	"context"

	"github.com/kastheco/kasmos/config/taskfsm"
)

// Action is a sealed interface representing a side-effect the caller must
// perform. Implementations carry just enough data for the caller to act.
type Action interface {
	// Kind returns a stable snake_case string identifier for the action type.
	Kind() string
	sealedAction()
}

// SpawnReviewerAction instructs the caller to launch a reviewer agent for the
// given plan file.
type SpawnReviewerAction struct {
	PlanFile   string
	ReviewBody string
}

func (SpawnReviewerAction) Kind() string  { return "spawn_reviewer" }
func (SpawnReviewerAction) sealedAction() {}

// SpawnCoderAction instructs the caller to launch a coder agent.
type SpawnCoderAction struct {
	PlanFile string
	Feedback string
}

func (SpawnCoderAction) Kind() string  { return "spawn_coder" }
func (SpawnCoderAction) sealedAction() {}

// SpawnPlannerAction instructs the caller to launch a planner agent for the
// given plan file. Emitted when a plan_start gateway signal is processed; the
// daemon executes it by building a planner prompt from the task store entry
// and spawning a planner session on the main branch (no worktree).
type SpawnPlannerAction struct {
	PlanFile string
}

func (SpawnPlannerAction) Kind() string  { return "spawn_planner" }
func (SpawnPlannerAction) sealedAction() {}

// ReviewChangesAction signals a validated review-changes transition and carries
// the reviewer feedback so callers can perform side effects only after the FSM
// accepted the signal.
type ReviewChangesAction struct {
	PlanFile string
	Feedback string
}

func (ReviewChangesAction) Kind() string  { return "review_changes" }
func (ReviewChangesAction) sealedAction() {}

// SpawnElaboratorAction instructs the caller to launch the architect pass.
type SpawnElaboratorAction struct {
	PlanFile string
}

func (SpawnElaboratorAction) Kind() string  { return "spawn_elaborator" }
func (SpawnElaboratorAction) sealedAction() {}

// AdvanceWaveAction instructs the caller to advance the plan to the next wave.
type AdvanceWaveAction struct {
	PlanFile string
	Wave     int
}

func (AdvanceWaveAction) Kind() string  { return "advance_wave" }
func (AdvanceWaveAction) sealedAction() {}

// ReviewApprovedAction is emitted whenever a ReviewApproved FSM signal is
// processed, regardless of whether a PR will be created. It carries the review
// body so callers can perform side effects (audit log, toast, ClickUp progress,
// reviewer pause) independently of PR-creation eligibility.
type ReviewApprovedAction struct {
	PlanFile   string
	ReviewBody string
}

func (ReviewApprovedAction) Kind() string  { return "review_approved" }
func (ReviewApprovedAction) sealedAction() {}

// CreatePRAction instructs the caller to open a pull request for the plan.
// It is only emitted when the plan has a branch and no PR URL yet.
// Callers that need to react to approval unconditionally should handle
// ReviewApprovedAction instead.
type CreatePRAction struct {
	PlanFile   string
	ReviewBody string
}

func (CreatePRAction) Kind() string  { return "create_pr" }
func (CreatePRAction) sealedAction() {}

// PlannerCompleteAction signals that the planner agent has finished and the
// plan is ready for implementation.
type PlannerCompleteAction struct {
	PlanFile string
}

func (PlannerCompleteAction) Kind() string  { return "planner_complete" }
func (PlannerCompleteAction) sealedAction() {}

// AutoImplementAction instructs the caller to automatically start
// implementation for a freshly planned task when auto_advance is enabled.
type AutoImplementAction struct {
	PlanFile string
}

func (AutoImplementAction) Kind() string  { return "auto_implement" }
func (AutoImplementAction) sealedAction() {}

// TaskCompleteAction signals that an individual task within a wave is done.
type TaskCompleteAction struct {
	PlanFile   string
	TaskNumber int
	WaveNumber int
}

func (TaskCompleteAction) Kind() string  { return "task_complete" }
func (TaskCompleteAction) sealedAction() {}

// IncrementReviewCycleAction instructs the caller to bump the review cycle
// counter for the plan (used to gate re-review after changes requested).
type IncrementReviewCycleAction struct {
	PlanFile string
}

func (IncrementReviewCycleAction) Kind() string  { return "increment_review_cycle" }
func (IncrementReviewCycleAction) sealedAction() {}

// ReviewCycleLimitAction signals that the review-fix loop has hit its
// configured maximum cycle count. The caller should notify the user
// and leave the plan in reviewing state without spawning a fixer.
type ReviewCycleLimitAction struct {
	PlanFile string
	Cycle    int
	Limit    int
}

func (ReviewCycleLimitAction) Kind() string  { return "review_cycle_limit" }
func (ReviewCycleLimitAction) sealedAction() {}

// SpawnFixerAction instructs the caller to launch a fixer agent to address
// reviewer feedback, whether it came from an in-app review loop or a PR review.
type SpawnFixerAction struct {
	PlanFile string
	Feedback string
}

func (SpawnFixerAction) Kind() string  { return "spawn_fixer" }
func (SpawnFixerAction) sealedAction() {}

// SpawnMasterAction instructs the caller to launch the master agent for the
// holistic readiness review phase. It is emitted by the processor when a
// reviewer approves and AutoReadinessReview is enabled.
type SpawnMasterAction struct {
	PlanFile string
}

func (SpawnMasterAction) Kind() string  { return "spawn_master" }
func (SpawnMasterAction) sealedAction() {}

// VerifyApprovedAction is emitted when a VerifyApproved FSM signal is
// processed (verifying → done). It signals that the verification phase passed
// and the task is done. CreatePRAction is emitted immediately after when
// eligible. Callers that need to react to reviewer approval unconditionally
// should handle ReviewApprovedAction instead.
type VerifyApprovedAction struct {
	PlanFile   string
	ReviewBody string
}

func (VerifyApprovedAction) Kind() string  { return "verify_approved" }
func (VerifyApprovedAction) sealedAction() {}

// VerifyFailedAction is emitted when a VerifyFailed FSM signal is processed
// (verifying → implementing). It carries fixer feedback so callers can surface
// the failure and dispatch a fixer agent when AutoReviewFix is enabled.
type VerifyFailedAction struct {
	PlanFile string
	Feedback string
}

func (VerifyFailedAction) Kind() string  { return "verify_failed" }
func (VerifyFailedAction) sealedAction() {}

// PausePlanAgentAction instructs the caller to pause (kill) the running agent
// of the given type for the plan.
type PausePlanAgentAction struct {
	PlanFile  string
	AgentType string
}

func (PausePlanAgentAction) Kind() string  { return "pause_plan_agent" }
func (PausePlanAgentAction) sealedAction() {}

// TransitionAction is emitted for audit/logging hooks whenever a state
// transition should be recorded. It does not imply a spawning side effect.
type TransitionAction struct {
	PlanFile string
	Event    taskfsm.Event
}

func (TransitionAction) Kind() string  { return "transition" }
func (TransitionAction) sealedAction() {}

// ---------------------------------------------------------------------------
// AgentSpawner
// ---------------------------------------------------------------------------

// SpawnOpts carries the parameters needed to launch any agent type.
type SpawnOpts struct {
	// PlanFile is the path to the plan markdown file relative to the repo root.
	PlanFile string
	// AgentType identifies the agent role: coder, reviewer, architect-pass, etc.
	AgentType string
	// RepoPath is the absolute filesystem path to the repository root.
	RepoPath string
	// Project is the project name (basename of RepoPath). Used by the daemon
	// to associate running instances with their originating repository.
	Project string
	// Branch is the git branch for the plan's shared worktree.
	Branch string
	// Prompt is the initial prompt delivered to the agent on startup.
	Prompt string
	// Program is the agent executable command (e.g. "opencode", "claude").
	Program string
	// Feedback is forwarded to coder agents as review feedback (may be empty).
	Feedback string
	// Wave is the current wave number (used to set KASMOS_WAVE env var).
	Wave int
	// ReviewCycle is the 1-indexed review/fix round used for reviewer/fixer
	// titles and prompts. Zero means "not a review-cycle agent".
	ReviewCycle int
}

// AgentSpawner abstracts tmux session management so the daemon and TUI can
// share the same SignalProcessor logic while using different backends.
type AgentSpawner interface {
	// SpawnPlanner launches a planner agent for the given plan on the main branch.
	SpawnPlanner(ctx context.Context, opts SpawnOpts) error
	// SpawnReviewer launches a reviewer agent for the given plan.
	SpawnReviewer(ctx context.Context, opts SpawnOpts) error
	// SpawnCoder launches a coder agent for the given plan.
	SpawnCoder(ctx context.Context, opts SpawnOpts) error
	// SpawnElaborator launches the architect pass for the given plan.
	SpawnElaborator(ctx context.Context, opts SpawnOpts) error
	// SpawnFixer launches a fixer agent to address PR review feedback.
	SpawnFixer(ctx context.Context, opts SpawnOpts) error
	// SpawnMaster launches the master agent for the holistic readiness review.
	// Any running master or reviewer instance for the plan is killed first.
	SpawnMaster(ctx context.Context, opts SpawnOpts) error
	// KillAgent stops the running agent of the given type for the plan.
	// repoPath is the absolute path to the repository root and is required to
	// disambiguate agents across multiple registered repos that share the same
	// plan filename.
	KillAgent(repoPath, planFile, agentType string) error
}
