package session

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/kastheco/kasmos/session/common"
	"github.com/kastheco/kasmos/session/git"
	"github.com/kastheco/kasmos/session/sdk"
)

// Status represents the current state of an instance.
type Status int

const (
	// Running indicates the instance is active and the agent is working.
	Running Status = iota
	// Ready indicates the instance is idle and waiting for user input.
	Ready
	// Loading indicates the instance is starting up or initialising.
	Loading
	// Paused indicates the instance is paused — the worktree has been removed but the branch is preserved.
	Paused
)

// AgentType constants identify the role of an agent session within a plan lifecycle.
const (
	AgentTypePlanner           = "planner"
	AgentTypeCoder             = "coder"
	AgentTypeReviewer          = "reviewer"
	AgentTypeFixer             = "fixer"
	AgentTypeElaborator        = "architect"
	AgentTypeArchitectBaseline = "architect-baseline"
	AgentTypeMaster            = "master"
)

// Instance represents a managed agent session with its associated execution backend and git state.
type Instance struct {
	// Title is the stable tmux session identifier for this instance.
	Title string
	// DisplayTitle is an optional user-facing label shown in the UI.
	DisplayTitle string
	// Path is the workspace directory for the instance.
	Path string
	// Branch is the git branch associated with this instance.
	Branch string
	// Status is the current lifecycle state of the instance.
	Status Status
	// Program is the agent command to execute within the session.
	Program string
	// ExecutionMode determines how the agent process is hosted (tmux or sdk).
	ExecutionMode ExecutionMode
	// Height is the terminal height in rows.
	Height int
	// Width is the terminal width in columns.
	Width int
	// CreatedAt records when the instance was first created.
	CreatedAt time.Time
	// UpdatedAt records the most recent update timestamp.
	UpdatedAt time.Time
	// AutoYes causes the instance to auto-confirm prompts.
	AutoYes bool
	// SkipPermissions enables the --permission-mode bypassPermissions flag for Claude.
	SkipPermissions bool
	// TaskFile is the plan file this instance is implementing (empty for ad-hoc sessions).
	TaskFile string
	// Topic is the topic group this instance belongs to.
	Topic string
	// AgentType identifies the role within a plan lifecycle: planner, coder, reviewer, fixer, architect, master, or empty.
	AgentType string
	// TaskNumber is the 1-indexed task number within a plan wave (0 = not a wave task).
	TaskNumber int
	// WaveNumber is the 1-indexed wave number this task belongs to (0 = not a wave task).
	WaveNumber int
	// PeerCount is the number of concurrent sibling tasks in the same wave (0 = not a wave task).
	PeerCount int
	// WaveTaskIndex is the 1-indexed position of this task within its wave (0 = unknown / non-wave task).
	WaveTaskIndex int
	// WaveTaskCount is the total number of tasks in this task's wave (0 = unknown / non-wave task).
	WaveTaskCount int
	// IsReviewer is a compatibility mirror for older persisted instance data.
	// New runtime logic should use AgentType == AgentTypeReviewer.
	IsReviewer bool
	// ImplementationComplete is set when the coder finishes and the plan transitions to review.
	ImplementationComplete bool
	// SoloAgent is true for instances launched as standalone agents outside the orchestration lifecycle.
	SoloAgent bool
	// Exited is true when the instance's tmux session has terminated unexpectedly.
	Exited bool
	// QueuedPrompt is delivered to the session on first transition to Ready. Cleared after delivery.
	QueuedPrompt string

	// sharedWorktree indicates the instance shares a topic worktree and should not clean it up.
	sharedWorktree bool
	// LoadingStage tracks the current startup progress step for the UI.
	LoadingStage int
	// LoadingTotal is the total count of startup stages.
	LoadingTotal int
	// LoadingMessage describes the current startup step shown in the UI.
	LoadingMessage string

	// Notified is true after the instance completes (Running→Ready) until the user selects it.
	Notified bool

	// LastActiveAt records the most recent time the instance entered Running or Loading state.
	LastActiveAt time.Time

	// PromptDetected is true when the agent program is waiting for user input.
	// Persists across status transitions to prevent UI flicker.
	PromptDetected bool

	// AwaitingWork is set when a QueuedPrompt is dispatched and cleared when the agent goes Running.
	// The wave orchestrator uses this to avoid treating early idle prompts as task completion.
	AwaitingWork bool

	// ReviewCycle is the 1-indexed count of review/fix cycles for this instance (0 = not a cycle instance).
	ReviewCycle int

	// ClaudeNoFlicker controls whether CLAUDE_CODE_NO_FLICKER=1 is set for the agent process.
	// Defaults to false (CLAUDE_CODE_NO_FLICKER=0) so prompt detection works in spawned agents.
	ClaudeNoFlicker bool

	// SDKSpeedTier is the session-scoped speed tier for SDK sessions ("", "flex", or "fast").
	// Only meaningful when ExecutionMode is SDK and the program is Codex.
	// The Codex transport forwards this as serviceTier on thread/start.
	SDKSpeedTier string

	// SDKTranscriptLimitsSet is true when SDKTranscriptMaxBytes/MaxTurns were explicitly configured.
	// Guards against forwarding zero-value (unlimited) limits from call sites that never set them.
	SDKTranscriptLimitsSet bool
	// SDKTranscriptMaxBytes is the byte cap forwarded to the SDK renderer (0 = no byte limit / unlimited).
	// A zero value here only takes effect when SDKTranscriptLimitsSet is true; otherwise
	// SDKTranscriptLimitsSet=false leaves the renderer at its compiled defaults.
	SDKTranscriptMaxBytes int64
	// SDKTranscriptMaxTurns is the turn cap forwarded to the SDK renderer (0 = no turn limit / unlimited).
	// Same SDKTranscriptLimitsSet guard applies.
	SDKTranscriptMaxTurns int64
	// RendererStats holds the last collected renderer stats for this instance (cached, not persisted).
	RendererStats sdk.RendererStats

	// HasWorked is true once the agent produces at least one content update after receiving its task.
	// Prevents permission prompts or early returns from prematurely completing a wave.
	HasWorked bool

	// PermissionBlocked is true when the agent is waiting on a permission prompt.
	// Runtime-only: not persisted. Prevents permission dialogs from looking like completion prompts.
	PermissionBlocked bool

	// CompletionPromptSince records when the current completion-eligible prompt state began.
	// Runtime-only: not persisted. Used to enforce the stability window before auto-advancing.
	CompletionPromptSince time.Time

	// CPUPercent is the last sampled CPU utilisation of the agent process.
	CPUPercent float64
	// MemMB is the last sampled memory usage of the agent process in megabytes.
	MemMB float64

	// LastActivity is the most recently detected agent activity event (ephemeral, not persisted).
	LastActivity *Activity

	// CachedContent is the last tmux pane capture, kept to avoid redundant subprocess calls.
	CachedContent string
	// CachedContentSet is true once CachedContent has been populated for the first time.
	CachedContentSet bool
	// CachedPresentation stores structured SDK preview turns for daemon-managed
	// placeholder instances that have no local execution session.
	CachedPresentation []*sdk.PresentationTurn

	// started is true once Start() has been called successfully.
	started bool
	// executionSession manages the underlying process host (tmux or sdk) for this instance.
	executionSession ExecutionSession
	// gitWorktree manages the git worktree associated with this instance.
	gitWorktree *git.GitWorktree
}

// ToInstanceData converts an Instance to its JSON-serialisable form for persistence.
// UpdatedAt is always refreshed to the current time.
func (i *Instance) ToInstanceData() InstanceData {
	data := InstanceData{
		Title:                  i.Title,
		DisplayTitle:           i.DisplayTitle,
		Path:                   i.Path,
		Branch:                 i.Branch,
		Status:                 i.Status,
		Height:                 i.Height,
		Width:                  i.Width,
		CreatedAt:              i.CreatedAt,
		UpdatedAt:              time.Now(),
		Program:                i.Program,
		ExecutionMode:          NormalizeExecutionMode(i.ExecutionMode),
		AutoYes:                i.AutoYes,
		SkipPermissions:        i.SkipPermissions,
		TaskFile:               i.TaskFile,
		AgentType:              i.AgentType,
		TaskNumber:             i.TaskNumber,
		WaveNumber:             i.WaveNumber,
		PeerCount:              i.PeerCount,
		WaveTaskIndex:          i.WaveTaskIndex,
		WaveTaskCount:          i.WaveTaskCount,
		ImplementationComplete: i.ImplementationComplete,
		SoloAgent:              i.SoloAgent,
		QueuedPrompt:           i.QueuedPrompt,
		ReviewCycle:            i.ReviewCycle,
		ClaudeNoFlicker:        i.ClaudeNoFlicker,
		SDKSpeedTier:           i.SDKSpeedTier,
	}

	if i.gitWorktree != nil {
		data.Worktree = GitWorktreeData{
			RepoPath:      i.gitWorktree.GetRepoPath(),
			WorktreePath:  i.gitWorktree.GetWorktreePath(),
			SessionName:   i.Title,
			BranchName:    i.gitWorktree.GetBranchName(),
			BaseCommitSHA: i.gitWorktree.GetBaseCommitSHA(),
		}
	}

	return data
}

// FromInstanceData reconstructs an Instance from its serialised form.
// The execution mode is resolved (via ResolveExecutionMode) so that the
// Instance.ExecutionMode always reflects the actual process host — if the
// requested mode is SDK but the program is unsupported, tmux is used instead.
// For paused instances the execution session is prepared but not started.
// For live instances the session is reattached; dead sessions are marked Exited.
func FromInstanceData(data InstanceData) (*Instance, error) {
	// Resolve: normalise + SDK-unsupported-program fallback.
	mode := ResolveExecutionMode(data.ExecutionMode, data.Program)

	agentType := data.AgentType
	if agentType == "" && data.IsReviewer {
		agentType = AgentTypeReviewer
	}

	branch := data.Branch
	if branch == "" {
		branch = data.Worktree.BranchName
	}
	sharedWorktree := isSharedTaskWorktree(data.Worktree, agentType)

	// Only restore a GitWorktree if the persisted data contains valid worktree
	// info. Main-branch / planner instances have an empty Worktree block and
	// must keep gitWorktree == nil so Resume() knows to skip worktree ops.
	var restoredWorktree *git.GitWorktree
	if data.Worktree.RepoPath != "" && data.Worktree.BranchName != "" {
		restoredWorktree = git.NewGitWorktreeFromStorage(
			data.Worktree.RepoPath,
			data.Worktree.WorktreePath,
			data.Worktree.SessionName,
			data.Worktree.BranchName,
			data.Worktree.BaseCommitSHA,
		)
	}

	// Normalize stored speed tier — only preserve for codex sdk sessions.
	sdkSpeedTier := ""
	if mode == ExecutionModeSDK && common.DetectProgramKind(data.Program) == common.ProgramCodex {
		sdkSpeedTier = NormalizeSDKSpeedTier(data.SDKSpeedTier)
	}

	instance := &Instance{
		Title:                  data.Title,
		DisplayTitle:           data.DisplayTitle,
		Path:                   data.Path,
		Branch:                 branch,
		Status:                 data.Status,
		Height:                 data.Height,
		Width:                  data.Width,
		CreatedAt:              data.CreatedAt,
		UpdatedAt:              data.UpdatedAt,
		Program:                data.Program,
		ExecutionMode:          mode,
		SkipPermissions:        data.SkipPermissions,
		TaskFile:               data.TaskFile,
		AgentType:              agentType,
		TaskNumber:             data.TaskNumber,
		WaveNumber:             data.WaveNumber,
		PeerCount:              data.PeerCount,
		WaveTaskIndex:          data.WaveTaskIndex,
		WaveTaskCount:          data.WaveTaskCount,
		IsReviewer:             agentType == AgentTypeReviewer,
		ImplementationComplete: data.ImplementationComplete,
		SoloAgent:              data.SoloAgent,
		QueuedPrompt:           data.QueuedPrompt,
		ReviewCycle:            data.ReviewCycle,
		ClaudeNoFlicker:        data.ClaudeNoFlicker,
		SDKSpeedTier:           sdkSpeedTier,
		sharedWorktree:         sharedWorktree,
		gitWorktree:            restoredWorktree,
	}

	if instance.Paused() {
		// Paused instances keep the session struct ready but do not reattach.
		instance.started = true
		es := NewExecutionSession(mode, instance.Title, instance.Program, instance.SkipPermissions)
		es.SetAgentType(instance.AgentType)
		es.SetSDKSpeedTier(instance.SDKSpeedTier)
		instance.executionSession = es
		return instance, nil
	}

	// Build the execution session handle and check liveness before attempting a full restore.
	es := NewExecutionSession(mode, instance.Title, instance.Program, instance.SkipPermissions)
	es.SetAgentType(instance.AgentType)
	es.SetSDKSpeedTier(instance.SDKSpeedTier)
	instance.executionSession = es

	if !es.DoesSessionExist() {
		// The session is gone — mark as exited so the UI can display it as dead.
		instance.started = true
		instance.Exited = true
		// Restore this silently: a persisted dead instance should still appear as
		// finished/notified in the UI, but must not re-fire the desktop
		// notification every time the app reloads state.
		instance.Status = Ready
		instance.Notified = true
		return instance, nil
	}

	// Session is alive — restore the full attachment via Start(false).
	if err := instance.Start(false); err != nil {
		return nil, err
	}

	return instance, nil
}

// DisplayName returns the user-facing label for the instance.
func (i *Instance) DisplayName() string {
	if strings.TrimSpace(i.DisplayTitle) != "" {
		return i.DisplayTitle
	}
	return i.Title
}

// IdentityKey returns a stable in-memory identity for UI selection and preview
// caches. Titles are user-editable and not globally unique for ad-hoc agents,
// so title-only comparisons can collide between distinct instances.
func (i *Instance) IdentityKey() string {
	if i == nil {
		return ""
	}
	mode := NormalizeExecutionMode(i.ExecutionMode)
	if !i.CreatedAt.IsZero() {
		return fmt.Sprintf(
			"%s|%s|%d|%s",
			strings.TrimSpace(i.Title),
			mode,
			i.CreatedAt.UnixNano(),
			strings.TrimSpace(i.Path),
		)
	}
	return fmt.Sprintf(
		"%s|%s|%s|%p",
		strings.TrimSpace(i.Title),
		mode,
		strings.TrimSpace(i.Path),
		i,
	)
}

func isSharedTaskWorktree(data GitWorktreeData, agentType string) bool {
	if data.RepoPath == "" || data.WorktreePath == "" || data.BranchName == "" {
		return false
	}
	switch agentType {
	case AgentTypeCoder, AgentTypeReviewer, AgentTypeFixer:
		return data.WorktreePath == git.TaskWorktreePath(data.RepoPath, data.BranchName)
	default:
		return false
	}
}

// InstanceOptions holds the configuration values for creating a new Instance.
type InstanceOptions struct {
	// Title is the stable session identifier.
	Title string
	// Path is the workspace directory.
	Path string
	// Program is the agent command to run (e.g. "claude", "opencode").
	Program string
	// ExecutionMode selects the process host backend (tmux or sdk).
	// Empty string defaults to ExecutionModeTmux.
	ExecutionMode ExecutionMode
	// AutoYes enables automatic confirmation of agent prompts.
	AutoYes bool
	// SkipPermissions enables --permission-mode bypassPermissions for Claude.
	SkipPermissions bool
	// TaskFile binds this instance to a plan from plan-state.
	TaskFile string
	// AgentType is the role of this instance within a plan: planner, coder, reviewer, fixer, architect, master, or empty.
	AgentType string
	// TaskNumber is the 1-indexed task number within a plan wave (0 = not a wave task).
	TaskNumber int
	// WaveNumber is the 1-indexed wave this task belongs to (0 = not a wave task).
	WaveNumber int
	// PeerCount is the number of concurrent sibling tasks in the same wave.
	PeerCount int
	// WaveTaskIndex is the 1-indexed position of this task within its wave (0 = unknown / non-wave task).
	WaveTaskIndex int
	// WaveTaskCount is the total number of tasks in this task's wave (0 = unknown / non-wave task).
	WaveTaskCount int
	// ReviewCycle is the 1-indexed review/fix cycle number (0 = not a cycle instance).
	ReviewCycle int
	// ClaudeNoFlicker controls whether CLAUDE_CODE_NO_FLICKER=1 is set for the agent process.
	// Defaults to false (CLAUDE_CODE_NO_FLICKER=0) so prompt detection works in spawned agents.
	ClaudeNoFlicker bool
	// SDKSpeedTier is the session-scoped speed tier ("", "flex", or "fast").
	// Only applied when ExecutionMode resolves to SDK and the program is Codex.
	SDKSpeedTier string
	// SDKTranscriptLimitsSet must be true to forward MaxBytes/MaxTurns to the renderer.
	SDKTranscriptLimitsSet bool
	// SDKTranscriptMaxBytes caps the SDK renderer's in-process byte usage (0 = renderer default).
	SDKTranscriptMaxBytes int64
	// SDKTranscriptMaxTurns caps the number of completed turns retained by the renderer (0 = renderer default).
	SDKTranscriptMaxTurns int64
}

// NewInstance constructs a new unstarted Instance from the given options.
// The workspace path is resolved to an absolute path before storage.
func NewInstance(opts InstanceOptions) (*Instance, error) {
	now := time.Now()

	absPath, err := filepath.Abs(opts.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path: %w", err)
	}

	resolvedMode := ResolveExecutionMode(opts.ExecutionMode, opts.Program)

	// SDK tiers are only available for Codex SDK sessions. If the resolved mode is
	// not SDK or the program is not Codex, ignore the tier.
	sdkSpeedTier := ""
	if resolvedMode == ExecutionModeSDK && common.DetectProgramKind(opts.Program) == common.ProgramCodex {
		sdkSpeedTier = NormalizeSDKSpeedTier(opts.SDKSpeedTier)
	}

	return &Instance{
		Title:                  opts.Title,
		Status:                 Ready,
		Path:                   absPath,
		Program:                opts.Program,
		ExecutionMode:          resolvedMode,
		Height:                 0,
		Width:                  0,
		CreatedAt:              now,
		UpdatedAt:              now,
		AutoYes:                opts.AutoYes,
		SkipPermissions:        opts.SkipPermissions,
		TaskFile:               opts.TaskFile,
		AgentType:              opts.AgentType,
		IsReviewer:             opts.AgentType == AgentTypeReviewer,
		TaskNumber:             opts.TaskNumber,
		WaveNumber:             opts.WaveNumber,
		PeerCount:              opts.PeerCount,
		WaveTaskIndex:          opts.WaveTaskIndex,
		WaveTaskCount:          opts.WaveTaskCount,
		ReviewCycle:            opts.ReviewCycle,
		ClaudeNoFlicker:        opts.ClaudeNoFlicker,
		SDKSpeedTier:           sdkSpeedTier,
		SDKTranscriptLimitsSet: opts.SDKTranscriptLimitsSet,
		SDKTranscriptMaxBytes:  opts.SDKTranscriptMaxBytes,
		SDKTranscriptMaxTurns:  opts.SDKTranscriptMaxTurns,
	}, nil
}

// BindSharedTaskWorktree attaches shared-worktree metadata to an instance that
// is being restored or manually re-adopted.
func (i *Instance) BindSharedTaskWorktree(repoPath, branch string) {
	if repoPath == "" || branch == "" {
		return
	}
	i.Branch = branch
	i.gitWorktree = git.NewSharedTaskWorktree(repoPath, branch)
	i.sharedWorktree = true
}

// RepoName returns the repository name for this instance.
// For instances without a git worktree (e.g. planner sessions on the main branch),
// the repo name is derived from the workspace path.
func (i *Instance) RepoName() (string, error) {
	if !i.started {
		return "", fmt.Errorf("cannot get repo name for instance that has not been started")
	}
	if i.gitWorktree == nil {
		return filepath.Base(i.Path), nil
	}
	return i.gitWorktree.GetRepoName(), nil
}

// GetRepoPath returns the repository root path, or empty string if the instance is not started
// or has no git worktree.
func (i *Instance) GetRepoPath() string {
	if !i.started || i.gitWorktree == nil {
		return ""
	}
	return i.gitWorktree.GetRepoPath()
}

// GetWorktreePath returns the worktree directory path, or empty string if unavailable.
func (i *Instance) GetWorktreePath() string {
	if i.gitWorktree == nil {
		return ""
	}
	return i.gitWorktree.GetWorktreePath()
}

// SetStatus transitions the instance to the given status and triggers associated side-effects:
// desktop notification on Running→Ready, timestamp refresh on Running/Loading, and
// AwaitingWork clear on Running.
func (i *Instance) SetStatus(status Status) {
	if i.Status == Running && status == Ready {
		i.Notified = true
		// Wave task instances are managed collectively by the orchestrator.
		// Only send desktop notifications for actual completion, not transient
		// idle/quiet Ready states during long-running agent work.
		if i.shouldSendCompletionNotification() {
			SendNotification("kas", fmt.Sprintf("'%s' has finished", i.Title))
		}
	}

	if status == Running || status == Loading {
		i.LastActiveAt = time.Now()
		i.PromptDetected = false
		i.Notified = false
		i.PermissionBlocked = false
		i.CompletionPromptSince = time.Time{}
	}

	if status == Running {
		i.AwaitingWork = false
	}

	i.Status = status
}

func (i *Instance) shouldSendCompletionNotification() bool {
	if i.TaskNumber != 0 {
		return false
	}
	return i.Exited || i.ImplementationComplete
}

// setLoadingProgress updates the loading stage and message shown during startup.
func (i *Instance) setLoadingProgress(stage int, message string) {
	i.LoadingStage = stage
	i.LoadingMessage = message
}

// Started reports whether the instance has been started via Start().
func (i *Instance) Started() bool {
	return i.started
}

// SetTitle updates the instance title. Returns an error if the instance has already started,
// since the title is used as the tmux session name and cannot be changed after creation.
func (i *Instance) SetTitle(title string) error {
	if i.started {
		return fmt.Errorf("cannot change title of a started instance")
	}
	i.Title = title
	return nil
}

// Paused reports whether the instance is in the Paused state.
func (i *Instance) Paused() bool {
	return i.Status == Paused
}

// TmuxAlive reports whether the underlying execution session is still running.
// The method name is preserved for backward compatibility with callers that
// use it to check session liveness (the semantics are unchanged).
func (i *Instance) TmuxAlive() bool {
	if i.executionSession == nil {
		return false
	}
	return i.executionSession.DoesSessionExist()
}
