// Package sdk – SDK-backed execution session.
//
// Session wraps a Transport (claude or codex) and exposes the same
// ExecutionSession surface that tmux/headless sessions expose.  The session
// receives structured events from the Transport, feeds them into a Renderer,
// and services I/O via SendKeys/TapEnter/SendPermissionResponse.
//
// Interactive operations (Attach, DetachSafely, SetDetachedSize) return
// ErrInteractiveOnly because SDK sessions are fully in-process and cannot be
// attached to a terminal.
package sdk

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/kastheco/kasmos/session/common"
	"github.com/kastheco/kasmos/session/tmux"
)

// ErrInteractiveOnly is returned when the caller requests a terminal
// operation that is not supported by the in-process SDK backend.
var ErrInteractiveOnly = errors.New("interactive operation requires tmux execution")

// ExecutionSessionIface is a local alias for the ExecutionSession interface
// defined in the parent session package.  It is used only in this file for a
// compile-time interface assertion in tests (to avoid an import cycle the
// parent package imports session/sdk, not the other way around).
//
// The field list must match session.ExecutionSession exactly.
type ExecutionSessionIface interface {
	Start(workDir string) error
	Restore() error
	Close() error
	DoesSessionExist() bool

	SendKeys(keys string) error
	TapEnter() error
	SendPermissionResponse(choice tmux.PermissionChoice) error
	CapturePaneContent() (string, error)
	CapturePaneContentWithOptions(start, end string) (string, error)
	HasUpdated() (bool, bool)
	HasUpdatedWithContent() (bool, bool, string, bool)
	GetPanePID() (int, error)

	Attach() (chan struct{}, error)
	DetachSafely() error
	SetDetachedSize(width, height int) error

	GetSanitizedName() string

	SetAgentType(agentType string)
	SetInitialPrompt(prompt string)
	SetNoFlicker(enabled bool)
	SetTaskEnv(taskNumber, waveNumber, peerCount int)
	SetProject(project string)
	SetSessionTitle(title string)
	SetTitleFunc(fn func(workDir string, beforeStart time.Time, title string))
	SetSDKSpeedTier(tier string)
}

// Session is the SDK execution backend.  It satisfies the parent package's
// ExecutionSession interface without importing the parent package.
type Session struct {
	// builder fields — all set before Start
	name            string
	sanitizedName   string
	program         string
	skipPermissions bool
	agentType       string
	initialPrompt   string
	noFlicker       bool
	speedTier       string
	project         string
	sessionTitle    string
	titleFunc       func(workDir string, beforeStart time.Time, title string)
	taskNumber      int
	waveNumber      int
	peerCount       int

	// runtime — guarded by mu after Start
	mu          sync.Mutex
	transport   Transport
	renderer    *Renderer
	ctx         context.Context
	cancel      context.CancelFunc
	alive       bool   // true once Start succeeds; false after process exits
	promptBuf   string // text buffered via SendKeys
	hasPrompt   bool   // true when agent signals it is ready for input
	lastContent string // previous content snapshot for HasUpdated
}

type localImagePromptTransport interface {
	SendPromptWithLocalImages(ctx context.Context, prompt string, imagePaths []string) error
}

// New constructs an unstarted SDK Session.
func New(name, program string, skipPermissions bool) *Session {
	return &Session{
		name:            name,
		sanitizedName:   common.SanitizeSessionName(name),
		program:         program,
		skipPermissions: skipPermissions,
		renderer:        NewRenderer(),
	}
}

// --- Configuration (builder-style, must be called before Start) --------

func (s *Session) SetAgentType(agentType string)  { s.agentType = agentType }
func (s *Session) SetInitialPrompt(prompt string) { s.initialPrompt = prompt }
func (s *Session) SetNoFlicker(enabled bool)      { s.noFlicker = enabled }
func (s *Session) SetSDKSpeedTier(tier string)    { s.speedTier = tier }
func (s *Session) SetProject(project string)      { s.project = project }
func (s *Session) SetSessionTitle(title string)   { s.sessionTitle = title }
func (s *Session) SetTitleFunc(fn func(workDir string, beforeStart time.Time, title string)) {
	s.titleFunc = fn
}

func (s *Session) SetTaskEnv(taskNumber, waveNumber, peerCount int) {
	s.taskNumber = taskNumber
	s.waveNumber = waveNumber
	s.peerCount = peerCount
}

// GetSanitizedName returns the sanitised session name used for log file naming.
func (s *Session) GetSanitizedName() string { return s.sanitizedName }

// --- Lifecycle ---------------------------------------------------------

// Start selects a transport from the registry, launches the agent subprocess,
// and starts an internal event-consumer goroutine.
func (s *Session) Start(workDir string) error {
	tr, ok := NewTransport(s.program)
	if !ok {
		return fmt.Errorf("no SDK transport for program %q — use tmux mode or a supported program (claude, codex)", s.program)
	}

	cfg := LaunchConfig{
		Name:            s.name,
		Program:         s.program,
		WorkDir:         workDir,
		SkipPermissions: s.skipPermissions,
		AgentType:       s.agentType,
		InitialPrompt:   s.initialPrompt,
		Project:         s.project,
		TaskNumber:      s.taskNumber,
		WaveNumber:      s.waveNumber,
		PeerCount:       s.peerCount,
		NoFlicker:       s.noFlicker,
		SpeedTier:       s.speedTier,
	}

	ctx, cancel := context.WithCancel(context.Background())
	if err := tr.Start(ctx, cfg); err != nil {
		cancel()
		return fmt.Errorf("start SDK transport: %w", err)
	}

	s.mu.Lock()
	s.transport = tr
	s.ctx = ctx
	s.cancel = cancel
	s.alive = true
	s.mu.Unlock()

	go s.consumeEvents()
	return nil
}

// Restore is a no-op for SDK sessions.  A fresh Session object created after
// an app restart will fail DoesSessionExist() — the instance layer handles
// marking such rows as exited/ready.
func (s *Session) Restore() error { return nil }

// Close shuts down the transport and marks the session dead.  alive flips to
// false immediately so DoesSessionExist reflects the caller's intent without
// waiting for the event-consumer goroutine to observe the events channel
// closing.  The transport reference is retained so consumeEvents can finish
// draining the events channel without a nil-pointer dereference.
func (s *Session) Close() error {
	s.mu.Lock()
	tr := s.transport
	cancel := s.cancel
	s.alive = false
	s.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	if tr != nil {
		return tr.Close()
	}
	return nil
}

// DoesSessionExist reports whether the agent subprocess is currently alive.
func (s *Session) DoesSessionExist() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.alive
}

// --- I/O ---------------------------------------------------------------

// SendKeys buffers plain text for later submission via TapEnter.
// The special sequence "\x03" (ctrl-C) is treated as an interrupt signal and
// forwarded directly to the transport instead of being buffered.
func (s *Session) SendKeys(keys string) error {
	if keys == "\x03" {
		s.mu.Lock()
		tr := s.transport
		pctx := s.ctx
		s.mu.Unlock()
		if tr == nil {
			return nil
		}
		ctx, cancel := context.WithTimeout(pctx, 10*time.Second)
		defer cancel()
		return tr.Interrupt(ctx)
	}
	s.mu.Lock()
	s.promptBuf += keys
	s.mu.Unlock()
	return nil
}

// TapEnter submits the buffered prompt to the transport via SendPrompt.
// A 10-second timeout prevents wedging if the subprocess has exited.
func (s *Session) TapEnter() error {
	s.mu.Lock()
	prompt := s.promptBuf
	s.promptBuf = ""
	s.hasPrompt = false
	tr := s.transport
	pctx := s.ctx
	s.mu.Unlock()

	if tr == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(pctx, 10*time.Second)
	defer cancel()
	if err := tr.SendPrompt(ctx, prompt); err != nil {
		return err
	}
	s.renderer.AddEvent(Event{
		Kind:      EventUserPrompt,
		Text:      prompt,
		Timestamp: time.Now(),
	})
	return nil
}

// SendPromptWithLocalImages submits a prompt with one or more local image attachments.
// The prompt text may be empty when the turn consists only of image input.
func (s *Session) SendPromptWithLocalImages(prompt string, imagePaths []string) error {
	filtered := make([]string, 0, len(imagePaths))
	for _, imagePath := range imagePaths {
		if trimmed := strings.TrimSpace(imagePath); trimmed != "" {
			filtered = append(filtered, trimmed)
		}
	}
	if strings.TrimSpace(prompt) == "" && len(filtered) == 0 {
		return nil
	}

	s.mu.Lock()
	s.promptBuf = ""
	s.hasPrompt = false
	tr := s.transport
	pctx := s.ctx
	s.mu.Unlock()

	if tr == nil {
		return nil
	}
	sender, ok := tr.(localImagePromptTransport)
	if !ok {
		return fmt.Errorf("sdk transport does not support local image prompts")
	}

	ctx, cancel := context.WithTimeout(pctx, 10*time.Second)
	defer cancel()
	if err := sender.SendPromptWithLocalImages(ctx, prompt, filtered); err != nil {
		return err
	}
	s.renderer.AddEvent(Event{
		Kind:      EventUserPrompt,
		Text:      formatUserPromptText(prompt, len(filtered)),
		Timestamp: time.Now(),
	})
	return nil
}

func formatUserPromptText(prompt string, imageCount int) string {
	trimmed := strings.TrimSpace(prompt)
	switch {
	case imageCount <= 0:
		return prompt
	case trimmed == "":
		if imageCount == 1 {
			return "[image]"
		}
		return fmt.Sprintf("[%d images]", imageCount)
	case imageCount == 1:
		return prompt + " [image]"
	default:
		return fmt.Sprintf("%s [%d images]", prompt, imageCount)
	}
}

// SendPermissionResponse forwards a permission dialog choice to the transport.
// Returns ErrInteractiveOnly before Start is called (no transport available).
// A 10-second timeout prevents wedging if the subprocess has exited.
func (s *Session) SendPermissionResponse(choice tmux.PermissionChoice) error {
	s.mu.Lock()
	tr := s.transport
	pctx := s.ctx
	s.mu.Unlock()
	if tr == nil {
		return ErrInteractiveOnly
	}
	ctx, cancel := context.WithTimeout(pctx, 10*time.Second)
	defer cancel()
	return tr.RespondPermission(ctx, choice)
}

// PendingPermission reports whether the underlying transport has an
// unanswered permission request, and its description + pattern. The TUI
// uses this to fire a permission overlay on SDK sessions, since SDK output
// doesn't contain the structural cues (menu + "enter to submit" footer)
// that the text-scraping permission parser relies on for tmux sessions.
func (s *Session) PendingPermission() (description, pattern string, ok bool) {
	s.mu.Lock()
	tr := s.transport
	s.mu.Unlock()
	if tr == nil {
		return "", "", false
	}
	return tr.PendingPermission()
}

// CapturePaneContent returns the current accumulated output as a string.
func (s *Session) CapturePaneContent() (string, error) {
	return s.renderer.Capture(), nil
}

// CapturePresentation returns a deep copy of the structured turn model built
// from the events received so far.  Returns nil when no turns have been created.
// The returned slice and all nested rows are safe for callers to mutate.
func (s *Session) CapturePresentation() []*PresentationTurn {
	return s.renderer.CapturePresentation()
}

// CapturePaneContentWithOptions returns a line-range slice of the accumulated
// output.  start/end follow tmux -S/-E semantics ("-" = beginning/end of
// history, integers are 0-based line offsets, negative values count from the
// end).
func (s *Session) CapturePaneContentWithOptions(start, end string) (string, error) {
	return s.renderer.CaptureRange(start, end), nil
}

// HasUpdated reports whether new content has been produced since the last call.
// Also returns whether the agent is currently showing an input prompt.
func (s *Session) HasUpdated() (updated bool, hasPrompt bool) {
	updated, hasPrompt, _, _ = s.HasUpdatedWithContent()
	return
}

// HasUpdatedWithContent is like HasUpdated but also returns the current content
// and a boolean indicating whether content was captured (always true for SDK).
func (s *Session) HasUpdatedWithContent() (updated bool, hasPrompt bool, content string, captured bool) {
	content = s.renderer.Capture()

	s.mu.Lock()
	hasPrompt = s.hasPrompt
	updated = content != s.lastContent
	s.lastContent = content
	s.mu.Unlock()

	return updated, hasPrompt, content, true
}

// GetPanePID returns the OS PID of the agent subprocess.
// Returns 0 before Start is called.
func (s *Session) GetPanePID() (int, error) {
	s.mu.Lock()
	tr := s.transport
	s.mu.Unlock()
	if tr == nil {
		return 0, nil
	}
	return tr.PID(), nil
}

// --- Interactive operations (not supported) ----------------------------

// Attach returns ErrInteractiveOnly — SDK sessions are in-process and cannot
// be attached to a terminal.
func (s *Session) Attach() (chan struct{}, error) { return nil, ErrInteractiveOnly }

// DetachSafely returns ErrInteractiveOnly.
func (s *Session) DetachSafely() error { return ErrInteractiveOnly }

// SetDetachedSize returns ErrInteractiveOnly.
func (s *Session) SetDetachedSize(_, _ int) error { return ErrInteractiveOnly }

// --- internal ----------------------------------------------------------

// consumeEvents reads events from the transport and feeds them to the
// renderer.  It runs in its own goroutine until the events channel is closed.
func (s *Session) consumeEvents() {
	for e := range s.transport.Events() {
		s.renderer.AddEvent(e)
		if e.HasPrompt {
			s.mu.Lock()
			s.hasPrompt = true
			s.mu.Unlock()
		}
		if e.Kind == EventTurnStarted {
			s.mu.Lock()
			s.hasPrompt = false
			s.mu.Unlock()
		}
	}
	// Events channel closed — the subprocess has exited.
	s.mu.Lock()
	s.alive = false
	s.mu.Unlock()
}
