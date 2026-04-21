package ui

import (
	"image/color"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/kastheco/kasmos/log"
	"github.com/kastheco/kasmos/session"
	zone "github.com/lrstanley/bubblezone/v2"
)

var (
	windowBorder = lipgloss.RoundedBorder()

	// windowStyle draws all four sides of the content area border.
	windowStyle = lipgloss.NewStyle().
			BorderForeground(ColorIris).
			Border(windowBorder, true, true, true, true)
)

// TabbedWindow composes two content panes (info, preview) behind a window border.
// It manages focus state and delegates rendering and scroll operations to the
// appropriate child pane.
type TabbedWindow struct {
	height int // total allocated height (post AdjustPreviewWidth)
	width  int // total allocated width (post AdjustPreviewWidth)

	preview  *PreviewPane
	info     *InfoPane
	instance *session.Instance // last known selected instance

	focused   bool // true when this panel owns keyboard focus
	focusMode bool // true when user is typing directly into the agent pane

	// showInfo controls whether the compact info summary is visible above the content window.
	showInfo bool
}

// NewTabbedWindow creates a TabbedWindow wiring the two child panes together.
// The compact info header is shown by default.
func NewTabbedWindow(preview *PreviewPane, info *InfoPane) *TabbedWindow {
	return &TabbedWindow{
		preview:  preview,
		info:     info,
		showInfo: true,
	}
}

// ── Focus helpers ─────────────────────────────────────────────────────────────

// SetFocusMode enables or disables insert / focus mode. When enabled the
// embedded terminal owns the pane and most updates become no-ops.
func (w *TabbedWindow) SetFocusMode(enabled bool) {
	w.focusMode = enabled
	w.preview.SetSDKFocusMode(enabled)
}

// IsFocusMode reports whether the window is in focus/insert mode.
func (w *TabbedWindow) IsFocusMode() bool { return w.focusMode }

// SetFocused marks whether this panel currently holds keyboard focus.
func (w *TabbedWindow) SetFocused(focused bool) { w.focused = focused }

// SetInstance stores the currently selected session instance.
func (w *TabbedWindow) SetInstance(instance *session.Instance) { w.instance = instance }

// ── Layout helpers ────────────────────────────────────────────────────────────

// AdjustPreviewWidth returns the usable width after subtracting the margin
// needed for borders (width - 2).
func AdjustPreviewWidth(width int) int { return width - 2 }

// compactInfo renders the compact info header and returns its content and
// rendered height. Returns ("", 0) when width is non-positive, showInfo is
// false, or there is no content to show.
func (w *TabbedWindow) compactInfo(width int) (string, int) {
	if width <= 0 || !w.showInfo {
		return "", 0
	}
	compact := w.info.RenderCompact(width)
	if compact == "" {
		return "", 0
	}
	return compact, lipgloss.Height(compact)
}

// SetSize allocates space to the window and propagates the resulting content
// dimensions to the preview and info child panes.
func (w *TabbedWindow) SetSize(width, height int) {
	w.width = AdjustPreviewWidth(width)
	w.height = height

	// Height consumed by the compact info header (may be 0).
	_, compactH := w.compactInfo(w.width)

	// Remaining content dimensions after compact header and window border.
	contentH := height - compactH - windowStyle.GetVerticalFrameSize()
	if contentH < 0 {
		contentH = 0
	}
	contentW := w.width - windowStyle.GetHorizontalFrameSize()
	if contentW < 0 {
		contentW = 0
	}

	w.preview.SetSize(contentW, contentH)
	w.info.SetSize(contentW, contentH)
}

// GetPreviewSize returns the dimensions currently allocated to the preview pane.
func (w *TabbedWindow) GetPreviewSize() (int, int) {
	return w.preview.width, w.preview.height
}

// SetShowInfo enables or disables the compact info summary above the content window.
func (w *TabbedWindow) SetShowInfo(show bool) { w.showInfo = show }

// IsShowingInfo reports whether the compact info summary is currently visible.
func (w *TabbedWindow) IsShowingInfo() bool { return w.showInfo }

// ── Preview pane delegation ───────────────────────────────────────────────────

// UpdatePreview refreshes the preview pane content from the given instance.
// During focus mode, updates are skipped for tmux-backed instances because the
// embedded PTY terminal owns the pane. SDK instances always receive updates
// since cached content is their only rendering path.
func (w *TabbedWindow) UpdatePreview(instance *session.Instance) error {
	if w.focusMode && (instance == nil || session.NormalizeExecutionMode(instance.ExecutionMode) == session.ExecutionModeTmux) {
		return nil
	}
	return w.preview.UpdateContent(instance)
}

// SetPreviewContent sets preview content directly from a pre-rendered string.
// Used by the embedded terminal in focus mode to bypass tmux capture-pane.
func (w *TabbedWindow) SetPreviewContent(content string) {
	w.preview.SetRawContent(content)
}

func (w *TabbedWindow) AppendSDKComposerText(text string) {
	w.preview.AppendSDKComposerText(text)
}

func (w *TabbedWindow) AppendSDKComposerImage(path string) {
	w.preview.AppendSDKComposerImage(path)
}

func (w *TabbedWindow) InsertSDKComposerNewline() {
	w.preview.InsertSDKComposerNewline()
}

func (w *TabbedWindow) DeleteSDKComposerBackward() {
	w.preview.DeleteSDKComposerBackward()
}

func (w *TabbedWindow) SDKComposerText() string {
	return w.preview.SDKComposerText()
}

func (w *TabbedWindow) SDKComposerImages() []string {
	return w.preview.SDKComposerImages()
}

func (w *TabbedWindow) ClearSDKComposerText() {
	w.preview.ClearSDKComposerText()
}

// SetConnectingState shows the animated banner with a "connecting…" message.
func (w *TabbedWindow) SetConnectingState() {
	w.preview.setFallbackState("connecting…")
}

// SetDocumentContent puts the preview pane into document mode, showing the
// supplied content (e.g. plan markdown) with scroll support.
func (w *TabbedWindow) SetDocumentContent(content string) {
	w.preview.SetDocumentContent(content)
}

// ClearDocumentMode exits document mode so UpdatePreview resumes normal behaviour.
func (w *TabbedWindow) ClearDocumentMode() { w.preview.ClearDocumentMode() }

// IsDocumentMode reports whether the preview pane is showing a static document.
func (w *TabbedWindow) IsDocumentMode() bool { return w.preview.IsDocumentMode() }

// ViewportUpdate forwards a tea.Msg to the preview viewport for native key
// handling (PgUp/PgDn, Home/End, etc.).
func (w *TabbedWindow) ViewportUpdate(msg tea.Msg) tea.Cmd {
	cmd := w.preview.ViewportUpdate(msg)
	if err := w.preview.autoExitScrollMode(w.instance); err != nil {
		log.InfoLog.Printf("tabbed window failed to auto-exit preview scroll mode: %v", err)
	}
	return cmd
}

// ViewportHandlesKey reports whether the preview viewport keymap handles msg.
func (w *TabbedWindow) ViewportHandlesKey(msg tea.KeyMsg) bool {
	return w.preview.ViewportHandlesKey(msg)
}

// ResetPreviewToNormalMode resets the preview pane to normal (live) mode.
func (w *TabbedWindow) ResetPreviewToNormalMode(instance *session.Instance) error {
	return w.preview.ResetToNormalMode(instance)
}

// IsPreviewInScrollMode reports whether the preview pane is in scroll mode.
func (w *TabbedWindow) IsPreviewInScrollMode() bool { return w.preview.isScrolling }

// ── Info pane delegation ──────────────────────────────────────────────────────

// SetInfoData updates the metadata shown in the info pane.
func (w *TabbedWindow) SetInfoData(data InfoData) { w.info.SetData(data) }

// GetInfoData returns the current InfoData held by the info pane.
func (w *TabbedWindow) GetInfoData() InfoData { return w.info.data }

// ── Scroll / pagination ───────────────────────────────────────────────────────

// ScrollUp scrolls the preview pane upward.
func (w *TabbedWindow) ScrollUp() {
	if err := w.preview.ScrollUp(w.instance); err != nil {
		log.InfoLog.Printf("tabbed window failed to scroll up: %v", err)
	}
}

// ScrollDown scrolls the preview pane downward.
func (w *TabbedWindow) ScrollDown() {
	if err := w.preview.ScrollDown(w.instance); err != nil {
		log.InfoLog.Printf("tabbed window failed to scroll down: %v", err)
	}
}

// HalfPageUp scrolls the preview pane up by half a page.
func (w *TabbedWindow) HalfPageUp() {
	if err := w.preview.HalfPageUp(w.instance); err != nil {
		log.InfoLog.Printf("tabbed window failed to half page up: %v", err)
	}
}

// HalfPageDown scrolls the preview pane down by half a page.
func (w *TabbedWindow) HalfPageDown() {
	if err := w.preview.HalfPageDown(w.instance); err != nil {
		log.InfoLog.Printf("tabbed window failed to half page down: %v", err)
	}
}

// ContentScrollUp scrolls the preview pane upward. Intended for mouse-wheel events.
func (w *TabbedWindow) ContentScrollUp() {
	if err := w.preview.ScrollUp(w.instance); err != nil {
		log.InfoLog.Printf("tabbed window failed to content scroll up: %v", err)
	}
}

// ContentScrollDown scrolls the preview pane downward. Intended for mouse-wheel events.
func (w *TabbedWindow) ContentScrollDown() {
	if err := w.preview.ScrollDown(w.instance); err != nil {
		log.InfoLog.Printf("tabbed window failed to content scroll down: %v", err)
	}
}

// ── Banner animation ──────────────────────────────────────────────────────────

// TickBanner advances the preview pane's banner animation by one frame.
func (w *TabbedWindow) TickBanner() { w.preview.TickBanner() }

// TickSpring advances the spring load-in animation on the preview pane.
func (w *TabbedWindow) TickSpring() { w.preview.TickSpring() }

// SetAnimateBanner enables or disables the idle banner animation.
func (w *TabbedWindow) SetAnimateBanner(enabled bool) { w.preview.SetAnimateBanner(enabled) }

// ── Rendering ─────────────────────────────────────────────────────────────────

// String renders the compact info header and content window as a single string.
// Returns an empty string when no size has been allocated.
func (w *TabbedWindow) String() string {
	if w.width <= 0 || w.height <= 0 {
		return ""
	}

	// Choose border accent based on focus state.
	var borderColor color.Color
	switch {
	case w.focusMode:
		borderColor = ColorFoam
	case w.focused:
		borderColor = ColorIris
	default:
		borderColor = ColorOverlay
	}

	// ── Compact info header ───────────────────────────────────────────────────

	compact, compactH := w.compactInfo(w.width)

	// ── Content window ────────────────────────────────────────────────────────

	content := w.preview.String()

	ws := windowStyle.BorderForeground(borderColor)
	innerW := w.width - ws.GetHorizontalFrameSize()
	innerH := w.height - ws.GetVerticalFrameSize() - compactH
	if innerW < 0 {
		innerW = 0
	}
	if innerH < 0 {
		innerH = 0
	}

	window := ws.Render(lipgloss.Place(innerW, innerH, lipgloss.Left, lipgloss.Top, content))
	// Wrap the preview content in a zone so mouse clicks are detected.
	window = zone.Mark(ZoneAgentPane, window)

	// ── Assemble ──────────────────────────────────────────────────────────────

	parts := make([]string, 0, 2)
	if compact != "" {
		parts = append(parts, compact)
	}
	parts = append(parts, window)

	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}
