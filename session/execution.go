package session

import (
	"errors"
	"strings"
	"time"

	"github.com/kastheco/kasmos/session/sdk"
	"github.com/kastheco/kasmos/session/tmux"
)

// ExecutionMode determines how an instance's agent process is hosted.
type ExecutionMode string

const (
	// ExecutionModeTmux uses tmux as the process host (default for ad-hoc sessions).
	ExecutionModeTmux ExecutionMode = "tmux"
	// ExecutionModeSDK drives the agent via its app-server JSON-RPC protocol.
	// This is the primary mode for managed profiles.
	ExecutionModeSDK ExecutionMode = "sdk"
)

// ErrInteractiveOnly is returned by SDK/headless sessions when an interactive
// operation (e.g. Attach, SendKeys) is requested.
var ErrInteractiveOnly = errors.New("interactive operation requires tmux execution")

// ExecutionSession abstracts the process host (tmux or headless) behind a common interface.
// All methods are equivalent to those on *tmux.TmuxSession; headless implementations
// return ErrInteractiveOnly for operations that require a live terminal.
type ExecutionSession interface {
	// Lifecycle
	Start(workDir string) error
	Restore() error
	Close() error
	DoesSessionExist() bool

	// I/O
	SendKeys(keys string) error
	TapEnter() error
	// SendPermissionResponse forwards a permission choice to the execution
	// backend. Tmux delegates to its adapter-aware session; headless returns
	// ErrInteractiveOnly.
	SendPermissionResponse(choice tmux.PermissionChoice) error
	CapturePaneContent() (string, error)
	CapturePaneContentWithOptions(start, end string) (string, error)
	HasUpdated() (bool, bool)
	HasUpdatedWithContent() (bool, bool, string, bool)
	GetPanePID() (int, error)

	// Attach/Detach
	Attach() (chan struct{}, error)
	DetachSafely() error
	SetDetachedSize(width, height int) error

	// Accessors
	GetSanitizedName() string

	// Configuration (builder-style, called before Start)
	SetAgentType(agentType string)
	SetInitialPrompt(prompt string)
	SetNoFlicker(enabled bool)
	SetTaskEnv(taskNumber, waveNumber, peerCount int)
	SetProject(project string)
	SetSessionTitle(title string)
	SetTitleFunc(fn func(workDir string, beforeStart time.Time, title string))
	// SetSDKSpeedTier sets the session-scoped speed tier ("" or "fast").
	// Non-SDK backends (tmux) ignore this; SDK backends forward it to the transport.
	SetSDKSpeedTier(tier string)
}

// progressReporter is optionally implemented by session types that support
// a progress callback hook. The instance layer uses a type assertion to set
// it without requiring all ExecutionSession implementations to support it.
type progressReporter interface {
	SetProgressFunc(fn func(int, string))
}

// pendingPermissionProvider is optionally implemented by execution sessions
// that carry structured permission-request state (SDK transports). The
// instance layer type-asserts to this interface inside CollectMetadata so
// tmux-backed sessions — which keep using text-scraping — don't need a
// stub method. Returns ok=false when no permission is pending.
type pendingPermissionProvider interface {
	PendingPermission() (description, pattern string, ok bool)
}

// presentationProvider is optionally implemented by execution sessions that
// expose a structured turn-grouped presentation model (SDK transports). The
// instance layer type-asserts to this interface so tmux-backed sessions —
// which expose only flat text — don't need a stub method. Follows the same
// pattern as pendingPermissionProvider.
type presentationProvider interface {
	CapturePresentation() []*sdk.PresentationTurn
}

// NormalizeSDKSpeedTier canonicalises a speed-tier value.
// Only "fast" (case-insensitive, trimmed) is a recognised non-default tier.
// Any other value — including the empty string, whitespace-only strings, or
// unknown tier names — normalises to "" (the default tier).
func NormalizeSDKSpeedTier(value string) string {
	if strings.ToLower(strings.TrimSpace(value)) == "fast" {
		return "fast"
	}
	return ""
}

// NormalizeExecutionMode canonicalises mode for the session layer.
//
//   - "sdk" → ExecutionModeSDK
//   - "headless" (legacy alias) → ExecutionModeSDK
//   - "tmux" → ExecutionModeTmux
//   - "" or anything unknown → ExecutionModeTmux (conservative default for ad-hoc sessions)
func NormalizeExecutionMode(mode ExecutionMode) ExecutionMode {
	switch ExecutionMode(strings.TrimSpace(string(mode))) {
	case ExecutionModeSDK, "headless":
		return ExecutionModeSDK
	case ExecutionModeTmux:
		return ExecutionModeTmux
	default:
		return ExecutionModeTmux
	}
}

// ResolveExecutionMode determines the actual execution mode for the given
// requested mode and program.  When mode is SDK but the program is not
// supported by any SDK transport, the mode falls back to tmux so the UI and
// livepreview layers always reflect the real process host.
func ResolveExecutionMode(requested ExecutionMode, program string) ExecutionMode {
	normalised := NormalizeExecutionMode(requested)
	if normalised == ExecutionModeSDK && !sdk.SupportsProgram(program) {
		return ExecutionModeTmux
	}
	return normalised
}

// NewExecutionSession constructs the appropriate ExecutionSession for the given mode.
// Callers should pass an already-resolved mode (via ResolveExecutionMode) to
// avoid constructing an SDK session for an unsupported program.
func NewExecutionSession(mode ExecutionMode, name, program string, skipPermissions bool) ExecutionSession {
	switch NormalizeExecutionMode(mode) {
	case ExecutionModeSDK:
		return sdk.New(name, program, skipPermissions)
	default:
		return newTmuxExecutionSession(name, program, skipPermissions)
	}
}

// newExecutionSession is a test seam around NewExecutionSession.
var newExecutionSession = NewExecutionSession

// --- tmux adapter -------------------------------------------------------

// tmuxExecutionSession wraps *tmux.TmuxSession and implements ExecutionSession.
// It also satisfies progressReporter so the instance layer can set ProgressFunc
// without depending on the concrete *TmuxSession type.
type tmuxExecutionSession struct {
	s *tmux.TmuxSession
}

func newTmuxExecutionSession(name, program string, skipPermissions bool) *tmuxExecutionSession {
	return &tmuxExecutionSession{s: tmux.NewTmuxSession(name, program, skipPermissions)}
}

// Lifecycle
func (w *tmuxExecutionSession) Start(workDir string) error { return w.s.Start(workDir) }
func (w *tmuxExecutionSession) Restore() error             { return w.s.Restore() }
func (w *tmuxExecutionSession) Close() error               { return w.s.Close() }
func (w *tmuxExecutionSession) DoesSessionExist() bool     { return w.s.DoesSessionExist() }

// I/O
func (w *tmuxExecutionSession) SendKeys(keys string) error { return w.s.SendKeys(keys) }
func (w *tmuxExecutionSession) TapEnter() error            { return w.s.TapEnter() }
func (w *tmuxExecutionSession) SendPermissionResponse(choice tmux.PermissionChoice) error {
	return w.s.SendPermissionResponse(choice)
}
func (w *tmuxExecutionSession) CapturePaneContent() (string, error) {
	return w.s.CapturePaneContent()
}
func (w *tmuxExecutionSession) CapturePaneContentWithOptions(start, end string) (string, error) {
	return w.s.CapturePaneContentWithOptions(start, end)
}
func (w *tmuxExecutionSession) HasUpdated() (bool, bool) { return w.s.HasUpdated() }
func (w *tmuxExecutionSession) HasUpdatedWithContent() (bool, bool, string, bool) {
	return w.s.HasUpdatedWithContent()
}
func (w *tmuxExecutionSession) GetPanePID() (int, error) { return w.s.GetPanePID() }

// Attach/Detach
func (w *tmuxExecutionSession) Attach() (chan struct{}, error) { return w.s.Attach() }
func (w *tmuxExecutionSession) DetachSafely() error            { return w.s.DetachSafely() }
func (w *tmuxExecutionSession) SetDetachedSize(width, height int) error {
	return w.s.SetDetachedSize(width, height)
}

// Accessors
func (w *tmuxExecutionSession) GetSanitizedName() string { return w.s.GetSanitizedName() }

// Configuration
func (w *tmuxExecutionSession) SetAgentType(agentType string)  { w.s.SetAgentType(agentType) }
func (w *tmuxExecutionSession) SetInitialPrompt(prompt string) { w.s.SetInitialPrompt(prompt) }
func (w *tmuxExecutionSession) SetNoFlicker(enabled bool)      { w.s.SetNoFlicker(enabled) }
func (w *tmuxExecutionSession) SetTaskEnv(taskNumber, waveNumber, peerCount int) {
	w.s.SetTaskEnv(taskNumber, waveNumber, peerCount)
}
func (w *tmuxExecutionSession) SetProject(project string)    { w.s.SetProject(project) }
func (w *tmuxExecutionSession) SetSessionTitle(title string) { w.s.SetSessionTitle(title) }
func (w *tmuxExecutionSession) SetTitleFunc(fn func(workDir string, beforeStart time.Time, title string)) {
	w.s.SetTitleFunc(fn)
}

// SetProgressFunc implements progressReporter, allowing the instance layer to
// inject a progress hook without knowing the concrete TmuxSession type.
func (w *tmuxExecutionSession) SetProgressFunc(fn func(int, string)) {
	w.s.ProgressFunc = fn
}

// SetSDKSpeedTier is a no-op for tmux-backed sessions.
// Speed tiers apply only to SDK transports (e.g. Codex).
func (w *tmuxExecutionSession) SetSDKSpeedTier(_ string) {}
