package session

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/kastheco/kasmos/log"
	"github.com/kastheco/kasmos/session/git"
	"github.com/kastheco/kasmos/session/sdk"
	"github.com/kastheco/kasmos/session/tmux"
)

type localImagePromptSession interface {
	SendPromptWithLocalImages(prompt string, imagePaths []string) error
}

// Preview returns the current pane content as a string.
// Returns an empty string if the instance has not been started or is paused.
func (i *Instance) Preview() (string, error) {
	if !i.started || i.Status == Paused {
		return "", nil
	}
	if NormalizeExecutionMode(i.ExecutionMode) == ExecutionModeSDK {
		if turns := i.CapturePresentation(); len(turns) > 0 {
			width := i.Width
			if width <= 0 {
				width = 80
			}
			return sdk.RenderPresentation(turns, width), nil
		}
	}
	return i.executionSession.CapturePaneContent()
}

// HasUpdated reports whether the pane content has changed since the last check.
// Returns (false, false) if the instance has not been started.
func (i *Instance) HasUpdated() (updated bool, hasPrompt bool) {
	if !i.started {
		return false, false
	}
	return i.executionSession.HasUpdated()
}

// NewEmbeddedTerminalForInstance creates an embedded terminal emulator connected
// to this instance's tmux PTY for zero-latency interactive focus mode.
// Returns ErrInteractiveOnly for SDK instances (no live terminal to attach to).
func (i *Instance) NewEmbeddedTerminalForInstance(cols, rows int) (*EmbeddedTerminal, error) {
	if NormalizeExecutionMode(i.ExecutionMode) == ExecutionModeSDK {
		return nil, ErrInteractiveOnly
	}
	if !i.started || i.executionSession == nil {
		return nil, fmt.Errorf("instance not started")
	}
	sessionName := i.executionSession.GetSanitizedName()
	return NewEmbeddedTerminal(sessionName, cols, rows)
}

// TapEnter sends an enter keypress to the pane when AutoYes is enabled.
// No-op if the instance is not started or AutoYes is false.
func (i *Instance) TapEnter() {
	if !i.started || !i.AutoYes {
		return
	}
	if err := i.executionSession.TapEnter(); err != nil {
		log.ErrorLog.Printf("error tapping enter: %v", err)
	}
}

// Attach connects the caller to the instance's execution session.
// Returns an error if the instance has not been started.
// Returns ErrInteractiveOnly for headless instances.
func (i *Instance) Attach() (chan struct{}, error) {
	if !i.started {
		return nil, fmt.Errorf("cannot attach instance that has not been started")
	}
	return i.executionSession.Attach()
}

// SetPreviewSize resizes the detached pane to the given dimensions.
// Returns an error if the instance is not started or is paused.
//
// SDK-backed sessions have no pty to resize, so SetDetachedSize on them
// returns ErrInteractiveOnly. Window-resize callers iterate every
// instance and we don't want a dozen "interactive operation requires
// tmux execution" lines per resize event in the log — swallow that one
// expected error so the log stays readable.
func (i *Instance) SetPreviewSize(width, height int) error {
	if !i.started || i.Status == Paused {
		return fmt.Errorf("cannot set preview size for instance that has not been started or " +
			"is paused")
	}
	if NormalizeExecutionMode(i.ExecutionMode) == ExecutionModeSDK {
		return nil
	}
	return i.executionSession.SetDetachedSize(width, height)
}

// GetGitWorktree returns the git worktree associated with this instance.
// Returns an error if the instance has not been started, has no worktree
// (e.g. started on main branch), or has an empty branch name.
func (i *Instance) GetGitWorktree() (*git.GitWorktree, error) {
	if !i.started {
		return nil, fmt.Errorf("cannot get git worktree for instance that has not been started")
	}
	if i.gitWorktree == nil || i.gitWorktree.GetBranchName() == "" {
		return nil, fmt.Errorf("instance '%s' has no git worktree branch (started on main branch or branch not set)", i.Title)
	}
	return i.gitWorktree, nil
}

// SendPrompt sends a text prompt followed by an enter keypress to the agent pane.
// Returns an error if the instance is not started or the execution session is nil.
// The 100ms sleep between SendKeys and TapEnter is skipped for SDK sessions
// because the SDK backend buffers the prompt internally and does not need it.
func (i *Instance) SendPrompt(prompt string) error {
	if !i.started {
		return fmt.Errorf("instance not started")
	}
	if i.executionSession == nil {
		return fmt.Errorf("execution session not initialized")
	}
	if err := i.executionSession.SendKeys(prompt); err != nil {
		return fmt.Errorf("error sending keys to session: %w", err)
	}
	// Brief pause to prevent the carriage return from being misinterpreted by tmux.
	// Skip for SDK mode — the backend buffers the prompt and the sleep is unnecessary.
	if NormalizeExecutionMode(i.ExecutionMode) == ExecutionModeTmux {
		time.Sleep(100 * time.Millisecond)
	}
	if err := i.executionSession.TapEnter(); err != nil {
		return fmt.Errorf("error tapping enter: %w", err)
	}
	return nil
}

// SendPromptWithLocalImages sends a prompt with one or more local image attachments.
// SDK-backed sessions can attach images directly; tmux-backed sessions fall back to
// text-only prompt delivery when no images are supplied.
func (i *Instance) SendPromptWithLocalImages(prompt string, imagePaths []string) error {
	if !i.started {
		return fmt.Errorf("instance not started")
	}
	if i.executionSession == nil {
		return fmt.Errorf("execution session not initialized")
	}
	if len(imagePaths) == 0 {
		return i.SendPrompt(prompt)
	}
	sender, ok := i.executionSession.(localImagePromptSession)
	if !ok {
		return fmt.Errorf("image prompts are not supported for %s", strings.TrimSpace(i.Program))
	}
	return sender.SendPromptWithLocalImages(prompt, imagePaths)
}

// PreviewFullHistory captures the complete pane output including the full scrollback buffer.
// Returns an empty string if the instance is not started or is paused.
func (i *Instance) PreviewFullHistory() (string, error) {
	if !i.started || i.Status == Paused {
		return "", nil
	}
	return i.executionSession.CapturePaneContentWithOptions("-", "-")
}

// PreviewRange captures a line-range slice of the pane output.
// start and end follow tmux -S/-E semantics ("-" = beginning/end, integers are
// 0-based line offsets, negative values count from the end).
// Returns an empty string if the instance is not started or is paused.
func (i *Instance) PreviewRange(start, end string) (string, error) {
	if !i.started || i.Status == Paused {
		return "", nil
	}
	return i.executionSession.CapturePaneContentWithOptions(start, end)
}

// Interrupt sends a ctrl-C interrupt to the agent.
// For SDK sessions this calls the transport's Interrupt; for tmux sessions it
// routes through SendKeys("\x03").
// Returns an error if the instance is not started.
func (i *Instance) Interrupt() error {
	if !i.started {
		return fmt.Errorf("instance not started")
	}
	return i.executionSession.SendKeys("\x03")
}

// CapturePresentation returns the structured turn-grouped presentation model
// for SDK-backed instances. It prefers a live local execution session when one
// is available, otherwise it falls back to any cached daemon presentation turns
// stored on the instance placeholder.
func (i *Instance) CapturePresentation() []*sdk.PresentationTurn {
	if i.executionSession != nil {
		if pp, ok := i.executionSession.(presentationProvider); ok {
			return pp.CapturePresentation()
		}
	}
	return sdk.ClonePresentationTurns(i.CachedPresentation)
}

// SetCachedPresentation stores a deep copy of daemon-fetched SDK presentation
// turns so placeholder instances can render structured output without a local
// execution session.
func (i *Instance) SetCachedPresentation(turns []*sdk.PresentationTurn) {
	i.CachedPresentation = sdk.ClonePresentationTurns(turns)
}

// SetExecutionSessionForTest replaces the execution session without starting the
// instance. Intended for use in tests that need to inject a custom session
// (e.g. a mock presentationProvider) without spawning real processes.
func (i *Instance) SetExecutionSessionForTest(exec ExecutionSession) {
	i.executionSession = exec
}

// SetTmuxSession replaces the tmux session handle. Intended for use in tests only.
// The TmuxSession is wrapped in the internal tmuxExecutionSession adapter so that
// the Instance can be used through the ExecutionSession interface.
func (i *Instance) SetTmuxSession(session *tmux.TmuxSession) {
	i.executionSession = &tmuxExecutionSession{s: session}
}

// MarkStartedForTest sets the started flag without spawning a real session.
// Use only in tests that need to simulate a running instance.
func (i *Instance) MarkStartedForTest() {
	i.started = true
}

// MarkStartedDeadForTest marks the instance as started with a no-op execution
// session that reports the underlying process as dead (DoesSessionExist →
// false).  This lets tests simulate an already-exited agent without starting a
// real tmux session or subprocess — required on CI hosts where tmux is not
// available.  Metadata collection calls (HasUpdatedWithContent, GetPanePID)
// return zero values so MonitorRunningInstances treats the instance as
// quiescent-but-exited.
func (i *Instance) MarkStartedDeadForTest() {
	i.started = true
	i.executionSession = deadExecutionSession{}
}

// deadExecutionSession is a no-op ExecutionSession used by
// MarkStartedDeadForTest.  It reports the session as non-existent and returns
// zero values for all metadata queries.
type deadExecutionSession struct{}

func (deadExecutionSession) Start(string) error     { return nil }
func (deadExecutionSession) Restore() error         { return nil }
func (deadExecutionSession) Close() error           { return nil }
func (deadExecutionSession) DoesSessionExist() bool { return false }
func (deadExecutionSession) SendKeys(string) error  { return nil }
func (deadExecutionSession) TapEnter() error        { return nil }
func (deadExecutionSession) SendPermissionResponse(tmux.PermissionChoice) error {
	return nil
}
func (deadExecutionSession) CapturePaneContent() (string, error) { return "", nil }
func (deadExecutionSession) CapturePaneContentWithOptions(string, string) (string, error) {
	return "", nil
}
func (deadExecutionSession) HasUpdated() (bool, bool) { return false, false }
func (deadExecutionSession) HasUpdatedWithContent() (bool, bool, string, bool) {
	return false, false, "", false
}
func (deadExecutionSession) GetPanePID() (int, error)                     { return 0, fmt.Errorf("no pane") }
func (deadExecutionSession) Attach() (chan struct{}, error)               { return nil, nil }
func (deadExecutionSession) DetachSafely() error                          { return nil }
func (deadExecutionSession) SetDetachedSize(int, int) error               { return nil }
func (deadExecutionSession) GetSanitizedName() string                     { return "" }
func (deadExecutionSession) SetAgentType(string)                          {}
func (deadExecutionSession) SetInitialPrompt(string)                      {}
func (deadExecutionSession) SetNoFlicker(bool)                            {}
func (deadExecutionSession) SetTaskEnv(int, int, int)                     {}
func (deadExecutionSession) SetProject(string)                            {}
func (deadExecutionSession) SetSessionTitle(string)                       {}
func (deadExecutionSession) SetTitleFunc(func(string, time.Time, string)) {}
func (deadExecutionSession) SetSDKSpeedTier(string)                       {}

// SendKeys sends raw key sequences to the pane.
// Returns an error if the instance is not started or is paused.
func (i *Instance) SendKeys(keys string) error {
	if !i.started || i.Status == Paused {
		return fmt.Errorf("cannot send keys to instance that has not been started or is paused")
	}
	return i.executionSession.SendKeys(keys)
}

// InstanceMetadata holds the results of a single per-tick poll for one instance.
// All fields are value types — safe to pass between goroutines without synchronization.
type InstanceMetadata struct {
	// Content is the raw capture output.
	Content         string
	ContentCaptured bool
	Updated         bool
	HasPrompt       bool
	CPUPercent      float64
	MemMB           float64
	// ResourceUsageValid is true when CPU/memory data was successfully collected.
	ResourceUsageValid bool
	// TmuxAlive reflects the result of session liveness check (used by the reviewer completion check).
	TmuxAlive        bool
	PermissionPrompt *PermissionPrompt
}

// CollectMetadata gathers all per-tick data for this instance via subprocess calls.
// Safe to call from a goroutine — does not mutate the instance's cached preview fields.
func (i *Instance) CollectMetadata() InstanceMetadata {
	var m InstanceMetadata

	if !i.started || i.Status == Paused {
		return m
	}

	// Single capture call shared by hash check, activity parsing, and preview.
	m.Updated, m.HasPrompt, m.Content, m.ContentCaptured = i.executionSession.HasUpdatedWithContent()

	// Permission prompt detection. SDK transports (claude/codex SDK) carry
	// structured permission state on the session itself — query that first so
	// the TUI overlay can fire on SDK sessions, whose renderer output lacks
	// the tmux-era "enter to submit" footer and numbered menu that
	// ParsePermissionPrompt relies on. When no direct state is available we
	// fall back to text-scraping the captured pane content, which is the
	// tmux-session path.
	if provider, ok := i.executionSession.(pendingPermissionProvider); ok {
		if desc, pattern, pending := provider.PendingPermission(); pending {
			m.PermissionPrompt = &PermissionPrompt{Description: desc, Pattern: pattern}
		}
	}
	if m.PermissionPrompt == nil && m.ContentCaptured && m.Content != "" {
		m.PermissionPrompt = ParsePermissionPrompt(m.Content, i.Program)
	}

	// Resource usage via pgrep + ps.
	m.CPUPercent, m.MemMB, m.ResourceUsageValid = i.collectResourceUsage()

	// Session liveness check for the reviewer completion logic.
	m.TmuxAlive = i.TmuxAlive()

	return m
}

// collectResourceUsage samples CPU and RSS memory for the agent process via pgrep and ps.
// Returns (cpu%, memMB, ok). Safe to call from a goroutine.
func (i *Instance) collectResourceUsage() (float64, float64, bool) {
	if !i.started || i.executionSession == nil {
		return 0, 0, false
	}

	pid, err := i.executionSession.GetPanePID()
	if err != nil {
		return 0, 0, false
	}

	// Prefer the first child process of the pane's shell so we measure the agent binary, not the shell.
	targetPid := strconv.Itoa(pid)
	childOut, err := exec.Command("pgrep", "-P", strconv.Itoa(pid)).Output()
	if err == nil {
		if children := strings.Fields(strings.TrimSpace(string(childOut))); len(children) > 0 {
			targetPid = children[0]
		}
	}

	psOut, err := exec.Command("ps", "-o", "%cpu=,rss=", "-p", targetPid).Output()
	if err != nil {
		return 0, 0, false
	}

	fields := strings.Fields(strings.TrimSpace(string(psOut)))
	if len(fields) < 2 {
		return 0, 0, false
	}

	cpu, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return 0, 0, false
	}
	rssKB, err := strconv.ParseFloat(fields[1], 64)
	if err != nil {
		return 0, 0, false
	}
	return cpu, rssKB / 1024, true
}

// UpdateResourceUsage refreshes the instance's CPU and memory fields.
func (i *Instance) UpdateResourceUsage() {
	if cpu, mem, ok := i.collectResourceUsage(); ok {
		i.CPUPercent, i.MemMB = cpu, mem
	}
}

// SendPermissionResponse forwards a permission dialog choice to the execution
// backend. The concrete backend selects any adapter-specific key sequence.
// No-op if the instance is not started or the execution session is nil.
func (i *Instance) SendPermissionResponse(choice tmux.PermissionChoice) {
	if !i.started || i.executionSession == nil {
		return
	}
	if err := i.executionSession.SendPermissionResponse(choice); err != nil {
		log.ErrorLog.Printf("error sending permission response: %v", err)
	}
}
