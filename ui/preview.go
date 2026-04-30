package ui

import (
	"fmt"
	"strings"
	"time"
	"unicode"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/kastheco/kasmos/log"
	"github.com/kastheco/kasmos/session"
	"github.com/kastheco/kasmos/session/sdk"
)

var (
	previewPaneStyle    lipgloss.Style
	scrollbarTrackStyle lipgloss.Style
	scrollbarThumbStyle lipgloss.Style
)

func rebuildPreviewStyles() {
	previewPaneStyle = lipgloss.NewStyle().Foreground(ColorText)
	scrollbarTrackStyle = lipgloss.NewStyle().Foreground(ColorOverlay)
	scrollbarThumbStyle = lipgloss.NewStyle().Foreground(ColorIris)
}

func init() {
	rebuildPreviewStyles()
}

// previewState holds the current display state of the preview pane.
type previewState struct {
	// fallback indicates the pane is in fallback (no active session) mode.
	fallback bool
	// fallbackMsg is shown below the banner in banner-fallback mode.
	fallbackMsg string
	// text holds the raw content to display in normal or fallback-content modes.
	text string
}

// sdkPresentationView holds the three layout regions for a structured SDK pane.
// body is the turn timeline, sticky is the pinned activity strip (empty when no
// running turn), and footer is the composer footer rows.
type sdkPresentationView struct {
	body   string
	sticky string
	footer []string
}

// PreviewPane renders the agent session preview area.
type PreviewPane struct {
	width  int
	height int

	previewState previewState
	isScrolling  bool
	viewport     viewport.Model
	// lastInstanceKey tracks the most recently rendered instance so inherited
	// scroll mode can be cleared when the user switches to a different session.
	lastInstanceKey string
	// lastSDKComposerOwner tracks which logical SDK session owns the current
	// draft so daemon placeholder replacement does not wipe in-progress input.
	lastSDKComposerOwner string

	// bannerFrame is the current animation tick index for the idle banner.
	bannerFrame int
	// animateBanner enables the idle banner ticker when true.
	animateBanner bool
	// isDocument is true while showing a rendered plan document.
	// UpdateContent is a no-op when this flag is set.
	isDocument bool
	// isRawTerminal is true when content was pushed via SetRawContent.
	// The VT emulator already fills exactly p.height rows, so String() must
	// not subtract 1 for an ellipsis row or the last line gets dropped.
	isRawTerminal bool
	// springAnim drives the banner load-in animation on first render.
	springAnim *SpringAnim
	// sdkFocusMode is true when the TUI is focusing an SDK session directly.
	sdkFocusMode bool
	// sdkComposerText is the inline input buffer shown in the SDK footer while focused.
	sdkComposerText string
	// sdkComposerCursor is a rune index into sdkComposerText indicating where
	// the next keystroke will be inserted. Invariant: 0 <= cursor <= len([]rune(sdkComposerText)).
	sdkComposerCursor int
	// sdkComposerImages are local image attachments queued for the next SDK prompt.
	sdkComposerImages []string
	// sdkComposerShellMode is true when the SDK composer is in shell-command mode.
	// When true, input is prefixed with "!" and submitted text is executed as a shell command.
	sdkComposerShellMode bool

	// sdkView holds the structured layout regions for SDK sessions. Non-nil
	// only while displaying SDK structured content.
	sdkView *sdkPresentationView
	// sdkScrollStrip is the pinned one-line strip shown below the viewport in
	// SDK scroll mode. Set by enterScrollMode, cleared by ResetToNormalMode.
	sdkScrollStrip string
}

// NewPreviewPane constructs a PreviewPane with initial fallback state.
func NewPreviewPane() *PreviewPane {
	return &PreviewPane{
		viewport:   viewport.New(),
		springAnim: NewSpringAnim(6.0, 15),
		previewState: previewState{
			fallback:    true,
			fallbackMsg: "create [n]ew plan or select existing",
		},
	}
}

// TickSpring advances the spring load-in animation by one frame.
func (p *PreviewPane) TickSpring() {
	if p.springAnim != nil {
		p.springAnim.Tick()
	}
}

// SetRawContent sets preview content from a pre-rendered string (VT emulator path).
// Clears scroll, document, and fallback flags and marks the pane as raw-terminal.
func (p *PreviewPane) SetRawContent(content string) {
	log.InfoLog.Printf("preview_state: SetRawContent len=%d scrolling=%v key=%q", len(content), p.isScrolling, p.lastInstanceKey)
	// Keep live terminal frames buffered while the user is inspecting scrollback.
	// Active sessions can repaint frequently; clearing scroll mode here would snap
	// the viewport back to the bottom on the next render tick.
	if p.isScrolling {
		p.previewState = previewState{text: content}
		p.isRawTerminal = true
		return
	}

	p.previewState = previewState{text: content}
	p.isScrolling = false
	p.isDocument = false
	p.isRawTerminal = true
}

// SetSize stores the pane dimensions and configures the viewport.
// The viewport width is width-1 to reserve one column for the scrollbar.
func (p *PreviewPane) SetSize(width, maxHeight int) {
	p.width = width
	p.height = maxHeight
	p.viewport.SetWidth(max(0, width-1))
	p.viewport.SetHeight(maxHeight)
}

// setFallbackState puts the pane into banner+message fallback mode.
func (p *PreviewPane) setFallbackState(message string) {
	p.previewState = previewState{
		fallback:    true,
		fallbackMsg: message,
	}
	p.isRawTerminal = false
}

// SetDocumentContent loads scrollable document content into the viewport.
// The pane remains in document mode (UpdateContent is a no-op) until
// ClearDocumentMode is called.
func (p *PreviewPane) SetDocumentContent(content string) {
	p.previewState = previewState{fallback: false}
	p.isScrolling = false
	p.isDocument = true
	p.isRawTerminal = false
	p.viewport.SetContent(content)
	p.viewport.GotoTop()
}

// IsDocumentMode reports whether the pane is displaying a static document.
func (p *PreviewPane) IsDocumentMode() bool {
	return p.isDocument
}

// ClearDocumentMode exits document mode so UpdateContent resumes normal preview.
func (p *PreviewPane) ClearDocumentMode() {
	p.isDocument = false
}

// ViewportUpdate forwards a tea.Msg to the viewport when in document or scroll
// mode, enabling native viewport key handling (PgUp/PgDn, arrows, mouse wheel).
// Returns nil when the pane is not in a scrollable mode.
func (p *PreviewPane) ViewportUpdate(msg tea.Msg) tea.Cmd {
	if !p.isDocument && !p.isScrolling {
		return nil
	}
	var cmd tea.Cmd
	p.viewport, cmd = p.viewport.Update(msg)
	return cmd
}

func (p *PreviewPane) shouldAutoExitScrollMode() bool {
	return p.isScrolling && p.viewport.AtBottom()
}

func (p *PreviewPane) autoExitScrollMode(instance *session.Instance) error {
	if !p.shouldAutoExitScrollMode() {
		return nil
	}
	return p.ResetToNormalMode(instance)
}

// ViewportHandlesKey reports whether the viewport keymap matches the given key
// when the pane is in document or scroll mode.
func (p *PreviewPane) ViewportHandlesKey(msg tea.KeyMsg) bool {
	if !p.isDocument && !p.isScrolling {
		return false
	}
	km := p.viewport.KeyMap
	return key.Matches(msg,
		km.Up, km.Down, km.Left, km.Right,
		km.PageUp, km.PageDown,
		km.HalfPageUp, km.HalfPageDown,
	)
}

// setFallbackContent sets fallback mode with arbitrary centered content (no banner).
func (p *PreviewPane) setFallbackContent(content string) {
	p.previewState = previewState{
		fallback: true,
		text:     content,
	}
	p.isRawTerminal = false
}

// SetAnimateBanner enables or disables the idle banner animation ticker.
func (p *PreviewPane) SetAnimateBanner(enabled bool) {
	p.animateBanner = enabled
}

// TickBanner advances the banner animation frame. Call from the app tick loop.
func (p *PreviewPane) TickBanner() {
	if p.animateBanner && p.previewState.fallback {
		p.bannerFrame++
	}
}

// UpdateContent refreshes the pane based on the instance state. It is a no-op
// when in document mode. In normal (non-scroll) mode live content arrives via
// SetRawContent from the VT emulator; this method only handles nil/Loading/
// Paused/Exited special cases plus initial scroll-mode capture.
func (p *PreviewPane) UpdateContent(instance *session.Instance) error {
	if p.isDocument {
		return nil
	}
	instanceKey := ""
	if instance != nil {
		instanceKey = instance.IdentityKey()
	}
	composerOwnerKey := sdkComposerOwnerKey(instance)
	if instanceKey != p.lastInstanceKey {
		log.InfoLog.Printf("preview_state: UpdateContent CLEARING previewState (key %q -> %q, prev_text_len=%d)", p.lastInstanceKey, instanceKey, len(p.previewState.text))
		p.isScrolling = false
		p.viewport.SetContent("")
		p.viewport.SetHeight(p.height)
		p.viewport.GotoTop()
		p.sdkView = nil
		p.sdkScrollStrip = ""
		p.previewState = previewState{}
		p.isRawTerminal = false
		p.lastInstanceKey = instanceKey
	}
	if composerOwnerKey != p.lastSDKComposerOwner {
		p.sdkComposerText = ""
		p.sdkComposerCursor = 0
		p.sdkComposerImages = nil
		p.sdkComposerShellMode = false
		p.lastSDKComposerOwner = composerOwnerKey
	}

	switch {
	case instance == nil:
		p.setFallbackState("create [n]ew plan or select existing")
		return nil

	case instance.Status == session.Loading:
		stage := instance.LoadingStage
		total := instance.LoadingTotal
		if total == 0 {
			total = 7
		}
		barWidth := 20
		filled := (stage * barWidth) / total
		if filled > barWidth {
			filled = barWidth
		}
		bar := GradientBar(barWidth, filled, GradientStart, GradientEnd)

		stepText := instance.LoadingMessage
		if stepText == "" {
			stepText = "Starting..."
		}
		pct := 0
		if total > 0 {
			pct = (stage * 100) / total
		}

		p.setFallbackContent(lipgloss.JoinVertical(lipgloss.Center,
			"",
			lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(GradientStart)).Render("Starting instance"),
			"",
			bar,
			"",
			lipgloss.NewStyle().Foreground(ColorMuted).Render(stepText),
			lipgloss.NewStyle().Foreground(ColorMuted).Render(fmt.Sprintf("%d%%", pct)),
		))
		return nil

	case instance.Status == session.Paused:
		if p.setCachedSDKPresentation(instance) {
			return nil
		}
		if p.setCachedTerminalContent(instance) {
			return nil
		}
		p.setFallbackContent(lipgloss.JoinVertical(lipgloss.Center,
			"Session is paused. Press 'r' to resume.",
			"",
			lipgloss.NewStyle().Foreground(ColorGold).Render(fmt.Sprintf(
				"The instance can be checked out at '%s' (copied to your clipboard)",
				instance.Branch,
			)),
		))
		return nil

	case instance.Exited:
		if p.setCachedSDKPresentation(instance) {
			return nil
		}
		if p.setCachedTerminalContent(instance) {
			return nil
		}
		p.setFallbackContent(lipgloss.JoinVertical(lipgloss.Center,
			lipgloss.NewStyle().Foreground(ColorMuted).Render("session exited"),
			"",
			lipgloss.NewStyle().Foreground(ColorMuted).Render("press delete to remove"),
		))
		return nil
	}

	if session.NormalizeExecutionMode(instance.ExecutionMode) == session.ExecutionModeTmux && instance.Started() && !instance.TmuxAlive() {
		if p.setCachedTerminalContent(instance) {
			return nil
		}
		p.setFallbackContent(lipgloss.JoinVertical(lipgloss.Center,
			lipgloss.NewStyle().Foreground(ColorMuted).Render("session stopped"),
			"",
			lipgloss.NewStyle().Foreground(ColorMuted).Render("press r to resume"),
		))
		return nil
	}

	// If in scroll mode but haven't loaded content yet, capture full history now.
	if p.isScrolling && p.viewport.Height() > 0 && len(p.viewport.View()) == 0 {
		content, err := p.scrollbackContent(instance)
		if err != nil {
			return err
		}
		footer := lipgloss.NewStyle().Foreground(ColorMuted).Render("ESC to exit scroll mode")
		p.viewport.SetContent(lipgloss.JoinVertical(lipgloss.Left, content, footer))
		return nil
	}

	// SDK sessions have no PTY. Prefer the structured turn-grouped presentation
	// model (variant-c hierarchy; see docs/agent-sdk-pane-mockups.md). Fall back
	// to flat cached text, then to a placeholder when no output has arrived yet.
	if session.NormalizeExecutionMode(instance.ExecutionMode) == session.ExecutionModeSDK {
		if turns := instance.CapturePresentation(); len(turns) > 0 {
			view := buildSDKPresentationView(turns, p.width, p.sdkComposerText, p.sdkComposerCursor, p.sdkComposerImages, p.sdkFocusMode, instance.Program, instance.SDKSpeedTier, time.Now(), p.sdkComposerShellMode)
			p.sdkView = &view
			p.previewState = previewState{text: joinSDKView(&view, p.width)}
			p.isRawTerminal = false
			return nil
		}
		p.sdkView = nil
		if instance.CachedContentSet && instance.CachedContent != "" {
			p.previewState = previewState{text: instance.CachedContent}
			p.isRawTerminal = false
			return nil
		}
		// No output yet — drop any inherited scroll state so the viewport stops
		// consuming scroll keys against stale content from a previously selected
		// tmux instance, and render the empty SDK composer/footer instead of the
		// generic banner so ad-hoc SDK sessions can enter focus mode immediately.
		p.isScrolling = false
		p.viewport.SetContent("")
		p.previewState = previewState{text: renderSDKPresentationWithComposer(nil, p.width, p.sdkComposerText, p.sdkComposerCursor, p.sdkComposerImages, p.sdkFocusMode, instance.Program, instance.SDKSpeedTier, p.sdkComposerShellMode)}
		p.isRawTerminal = false
		return nil
	}

	// Normal mode: live content arrives via SetRawContent from the VT emulator.
	return nil
}

func (p *PreviewPane) setCachedTerminalContent(instance *session.Instance) bool {
	if instance == nil || !instance.CachedContentSet || instance.CachedContent == "" {
		return false
	}
	p.previewState = previewState{text: instance.CachedContent}
	p.sdkView = nil
	p.isScrolling = false
	p.isDocument = false
	p.isRawTerminal = true
	return true
}

func (p *PreviewPane) setCachedSDKPresentation(instance *session.Instance) bool {
	if instance == nil || session.NormalizeExecutionMode(instance.ExecutionMode) != session.ExecutionModeSDK {
		return false
	}
	turns := instance.CapturePresentation()
	if len(turns) == 0 {
		return false
	}
	view := buildSDKPresentationView(turns, p.width, p.sdkComposerText, p.sdkComposerCursor, p.sdkComposerImages, p.sdkFocusMode, instance.Program, instance.SDKSpeedTier, time.Now(), p.sdkComposerShellMode)
	p.sdkView = &view
	p.previewState = previewState{text: joinSDKView(&view, p.width)}
	p.isScrolling = false
	p.isDocument = false
	p.isRawTerminal = false
	return true
}

// renderScrollbar builds a vertical scrollbar string for the given height.
// Returns an empty string when all content fits on screen (no scrolling needed).
func (p *PreviewPane) renderScrollbar(height int) string {
	if height <= 0 {
		return ""
	}
	// Hide scrollbar when everything fits on one screen.
	if p.viewport.AtBottom() && p.viewport.YOffset() == 0 {
		return ""
	}

	pct := p.viewport.ScrollPercent()
	thumbSize := max(1, height/5)
	trackLen := height - thumbSize
	thumbPos := int(pct * float64(trackLen))

	var sb strings.Builder
	for i := range height {
		if i >= thumbPos && i < thumbPos+thumbSize {
			sb.WriteString(scrollbarThumbStyle.Render("▐"))
		} else {
			sb.WriteString(scrollbarTrackStyle.Render("│"))
		}
		if i < height-1 {
			sb.WriteByte('\n')
		}
	}
	return sb.String()
}

// String renders the preview pane to a string.
func (p *PreviewPane) String() string {
	if p.width == 0 || p.height == 0 {
		return strings.Repeat("\n", p.height)
	}

	if p.previewState.fallback {
		fallbackText := p.buildFallbackText()
		return lipgloss.Place(
			p.width, p.height,
			lipgloss.Center, lipgloss.Center,
			previewPaneStyle.Render(fallbackText),
		)
	}

	// Document or scroll mode: render via viewport + optional scrollbar.
	if p.isDocument || p.isScrolling {
		viewContent := p.viewport.View()
		scrollbar := p.renderScrollbar(p.viewport.Height())
		var viewBlock string
		if scrollbar == "" {
			viewBlock = viewContent
		} else {
			viewBlock = lipgloss.JoinHorizontal(lipgloss.Top, viewContent, scrollbar)
		}
		// In SDK scroll mode, append the pinned strip below the viewport.
		if p.isScrolling && p.sdkScrollStrip != "" {
			return lipgloss.JoinVertical(lipgloss.Left, viewBlock, p.sdkScrollStrip)
		}
		return viewBlock
	}

	// SDK structured mode: lay out body / sticky / footer explicitly so the
	// sticky strip is always visible above the composer footer.
	if p.sdkView != nil {
		return p.renderSDKStructuredView()
	}

	// Normal mode: wrap raw lines to pane width, then tail-slice the wrapped
	// rows so (a) wide content stays fully readable across multiple visible
	// rows instead of being clipped off the right edge, and (b) the pane
	// follows newest output instead of showing startup text forever.
	availableHeight := p.height
	if !p.isRawTerminal {
		availableHeight-- // reserve one row for the overflow indicator
	}

	rows := wrapPreviewRows(p.previewState.text, p.width)
	overflowed := false
	if availableHeight > 0 {
		if len(rows) > availableHeight {
			rows = rows[len(rows)-availableHeight:]
			overflowed = true
		} else {
			padding := availableHeight - len(rows)
			rows = append(rows, make([]string, padding)...)
		}
	}

	if overflowed && !p.isRawTerminal && availableHeight > 0 {
		// Replace the topmost visible row with an ellipsis marker so the
		// user knows there is buffered scrollback above the pane.
		rows[0] = "..."
	}

	return previewPaneStyle.Width(p.width).Render(strings.Join(rows, "\n"))
}

// narrowPaneThreshold is the column count below which the SDK pane switches to
// a compact layout: minimal turn headers, a single-rule response divider, and
// single-newline separators between turns.
const narrowPaneThreshold = 40

// renderSDKPresentation converts a slice of PresentationTurns into a single
// ANSI-styled string ready to store in previewState.text. The timeline uses
// the variant-c turn-block hierarchy described in docs/agent-sdk-pane-mockups.md:
// tool/setup noise is visually secondary (muted/subtle) and assistant prose is
// primary (text colour). A quiet composer footer is appended after the timeline.
//
// When width is under narrowPaneThreshold, turns are separated by a single
// newline instead of a blank line.
func renderSDKPresentation(turns []*sdk.PresentationTurn, width int) string {
	return renderSDKPresentationWithComposer(turns, width, "", 0, nil, false, "", "", false)
}

func renderSDKPresentationWithComposer(turns []*sdk.PresentationTurn, width int, composer string, cursor int, images []string, focused bool, program string, speedTier string, shellMode bool) string {
	view := buildSDKPresentationView(turns, width, composer, cursor, images, focused, program, speedTier, time.Now(), shellMode)
	return joinSDKView(&view, width)
}

// joinSDKView assembles the three regions of an sdkPresentationView into a
// single string for storage in previewState.text. The sticky strip (if any)
// is placed on its own line between the body and the footer separator.
func joinSDKView(view *sdkPresentationView, width int) string {
	if view == nil {
		return ""
	}
	narrow := width > 0 && width < narrowPaneThreshold
	sep := "\n\n"
	if narrow {
		sep = "\n"
	}

	var sb strings.Builder
	sb.WriteString(view.body)

	if view.sticky != "" {
		if sb.Len() > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString(view.sticky)
	}

	if sb.Len() > 0 {
		sb.WriteString(sep)
	}
	sb.WriteString(strings.Join(view.footer, "\n"))
	return sb.String()
}

// buildSDKPresentationView builds the three layout regions for an SDK pane:
//   - body: the full turn timeline (all turns rendered, no footer).
//   - sticky: the pinned activity strip for the last running turn, or "" if no
//     running turn or the turn has no derived activity.
//   - footer: the composer footer rows.
//
// Callers that need only the joined string should use joinSDKView; callers that
// need to lay out the regions independently (e.g. String() for tail-slicing)
// use the struct directly.
func buildSDKPresentationView(turns []*sdk.PresentationTurn, width int, composer string, cursor int, images []string, focused bool, program, speedTier string, now time.Time, shellMode bool) sdkPresentationView {
	if width <= 0 {
		width = 80
	}
	narrow := width < narrowPaneThreshold
	sep := "\n\n"
	if narrow {
		sep = "\n"
	}

	var parts []string
	for _, turn := range turns {
		if turn == nil {
			continue
		}
		rows := renderSDKTurn(turn, width)
		if len(rows) > 0 {
			parts = append(parts, strings.Join(rows, "\n"))
		}
	}

	var bodyBuilder strings.Builder
	for i, part := range parts {
		if i > 0 {
			bodyBuilder.WriteString(sep)
		}
		bodyBuilder.WriteString(part)
	}

	// Derive sticky strip from the last running turn with activity.
	sticky := ""
	for i := len(turns) - 1; i >= 0; i-- {
		turn := turns[i]
		if turn == nil {
			continue
		}
		if turn.Running() && turn.Activity != nil {
			label := sdk.FormatActivityLabel(turn.Activity, now, narrow)
			activityStyle := lipgloss.NewStyle().Foreground(ColorMuted)
			sticky = activityStyle.Render(label)
			break
		}
	}

	footer := renderComposerFooter(width, composer, cursor, images, focused, program, speedTier, shellMode)

	return sdkPresentationView{
		body:   bodyBuilder.String(),
		sticky: sticky,
		footer: footer,
	}
}

// renderSDKStructuredView lays out the SDK pane using the current p.sdkView:
// the body is tail-sliced to fit above the sticky strip and footer, the sticky
// strip is placed immediately above the footer, and the footer closes the pane.
// An ellipsis replaces the top body row when content was truncated.
func (p *PreviewPane) renderSDKStructuredView() string {
	view := p.sdkView

	stickyRows := 0
	if view.sticky != "" {
		stickyRows = 1
	}
	// Reserve one row for the overflow indicator (matching normal-mode behaviour).
	availableBody := p.height - len(view.footer) - stickyRows - 1
	if availableBody < 0 {
		availableBody = 0
	}

	bodyRows := wrapPreviewRows(view.body, p.width)
	overflowed := false
	if len(bodyRows) > availableBody {
		bodyRows = bodyRows[len(bodyRows)-availableBody:]
		overflowed = true
	} else {
		padding := availableBody - len(bodyRows)
		bodyRows = append(bodyRows, make([]string, padding)...)
	}
	if overflowed && len(bodyRows) > 0 {
		bodyRows[0] = "..."
	}

	allRows := make([]string, 0, len(bodyRows)+stickyRows+len(view.footer))
	allRows = append(allRows, bodyRows...)
	if view.sticky != "" {
		allRows = append(allRows, view.sticky)
	}
	allRows = append(allRows, view.footer...)

	return previewPaneStyle.Width(p.width).Render(strings.Join(allRows, "\n"))
}

// renderSDKTurn renders one turn block as a slice of styled lines following
// variant-c colour assignments (see docs/agent-sdk-pane-mockups.md):
//   - header and tool rows: ColorSubtle (secondary)
//   - ok-result rows: ColorFoam
//   - error-result rows: ColorLove
//   - permission rows: ColorRose (salmon)
//   - warning, system, and RowStatus rows: ColorGold (warning amber)
//   - preview rows: ColorSubtle
//   - prose rows: ColorText (primary)
//   - user rows: ColorFoam
//   - thinking rows: ColorMuted
//   - RowResponse sentinel: emits a muted divider rule
//
// In narrow mode (width < narrowPaneThreshold):
//   - the header is reduced to just "turn N" (no elapsed, tool count, running label)
//   - the response divider collapses to a single muted rule row
func renderSDKTurn(turn *sdk.PresentationTurn, width int) []string {
	if turn == nil {
		return nil
	}
	narrow := width > 0 && width < narrowPaneThreshold
	var rows []string

	headerStyle := lipgloss.NewStyle().Foreground(ColorSubtle)
	if narrow {
		rows = append(rows, headerStyle.Render(fmt.Sprintf("turn %d", turn.Number)))
	} else {
		rows = append(rows, headerStyle.Render(turn.HeaderText(time.Now())))
	}

	toolStyle := lipgloss.NewStyle().Foreground(ColorPine)
	toolArgStyle := lipgloss.NewStyle().Foreground(ColorGold)
	userPrefixStyle := lipgloss.NewStyle().Foreground(ColorRose)
	userTextStyle := lipgloss.NewStyle().Foreground(ColorFoam)
	resultOKStyle := lipgloss.NewStyle().Foreground(ColorFoam)
	resultErrStyle := lipgloss.NewStyle().Foreground(ColorLove)
	systemStyle := lipgloss.NewStyle().Foreground(ColorGold)
	warningStyle := lipgloss.NewStyle().Foreground(ColorGold)
	permStyle := lipgloss.NewStyle().Foreground(ColorRose)
	proseStyle := lipgloss.NewStyle().Foreground(ColorText)
	timestampStyle := lipgloss.NewStyle().Foreground(ColorSubtle)
	statusStyle := lipgloss.NewStyle().Foreground(ColorGold)
	thinkingStyle := lipgloss.NewStyle().Foreground(ColorMuted)
	narrowRuleStyle := lipgloss.NewStyle().Foreground(ColorMuted)
	gutterStyle := lipgloss.NewStyle().Foreground(ColorSubtle)
	diffAddedStyle := lipgloss.NewStyle().Foreground(ColorFoam)
	diffRemovedStyle := lipgloss.NewStyle().Foreground(ColorLove)
	diffContextStyle := lipgloss.NewStyle().Foreground(ColorSubtle)
	previewStyle := lipgloss.NewStyle().Foreground(ColorSubtle)
	codeGutterStyle := lipgloss.NewStyle().Foreground(ColorMuted)
	codeStyle := lipgloss.NewStyle().Foreground(ColorFoam)
	mdStyles := sdk.MarkdownLineStyles{
		Base:         proseStyle,
		Bold:         proseStyle.Bold(true),
		Italic:       proseStyle.Italic(true),
		Code:         codeStyle,
		Heading:      lipgloss.NewStyle().Foreground(ColorGold).Bold(true),
		BulletPrefix: lipgloss.NewStyle().Foreground(ColorRose),
		NumberPrefix: lipgloss.NewStyle().Foreground(ColorFoam),
		QuotePrefix:  lipgloss.NewStyle().Foreground(ColorMuted),
	}

	prevKind := sdk.PresentationRowKind("")
	for i, row := range turn.Rows {
		switch row.Kind {
		case sdk.RowUser:
			rows = append(rows, sdk.RenderPromptLineWithTimestamp(">", row.Text, row.Timestamp, width, userPrefixStyle, userTextStyle, timestampStyle))
		case sdk.RowTool:
			head, args := sdk.SplitToolCallText(row.Text, row.ToolName)
			line := sdk.RenderToolCallLineWithStatus(
				head,
				args,
				sdk.ToolCallSuccessMarker(turn.Rows, i),
				width-lipgloss.Width(sdk.ToolCallIndent),
				toolStyle,
				toolArgStyle,
				toolStyle,
			)
			if line != "" {
				rows = append(rows, sdk.ToolCallIndent+line)
			}
		case sdk.RowToolDiff:
			if row.ToolDiff != nil {
				diffRows := sdk.BuildToolDiffBlock(row.ToolDiff, width)
				for i, dl := range row.ToolDiff.Lines {
					if i >= len(diffRows) {
						break
					}
					switch dl.Kind {
					case sdk.DiffLineAdded:
						rows = append(rows, sdk.ToolChildIndent+sdk.RenderStructuredChildLine(diffRows[i], gutterStyle, diffAddedStyle))
					case sdk.DiffLineRemoved:
						rows = append(rows, sdk.ToolChildIndent+sdk.RenderStructuredChildLine(diffRows[i], gutterStyle, diffRemovedStyle))
					default:
						rows = append(rows, sdk.ToolChildIndent+sdk.RenderStructuredChildLine(diffRows[i], gutterStyle, diffContextStyle))
					}
				}
				// Truncation indicator row, if present.
				if len(diffRows) > len(row.ToolDiff.Lines) {
					rows = append(rows, sdk.ToolChildIndent+sdk.RenderStructuredChildLine(diffRows[len(row.ToolDiff.Lines)], gutterStyle, diffContextStyle))
				}
			}
		case sdk.RowToolPreview:
			if row.ToolPreview != nil {
				for _, pr := range sdk.BuildToolPreviewBlock(row.ToolPreview, width) {
					rows = append(rows, sdk.ToolChildIndent+sdk.RenderStructuredChildLine(pr, gutterStyle, previewStyle))
				}
			}
		case sdk.RowResult:
			if sdk.SuppressInlineSuccessResult(turn.Rows, i) {
				break
			}
			if row.IsError {
				rows = append(rows, sdk.ToolChildIndent+resultErrStyle.Render(row.Text))
			} else {
				rows = append(rows, sdk.ToolChildIndent+resultOKStyle.Render(row.Text))
			}
		case sdk.RowWarning:
			rows = append(rows, warningStyle.Render(row.Text))
		case sdk.RowSystem:
			if prevKind == sdk.RowSystem {
				rows = append(rows, systemStyle.Render(row.Text))
			} else {
				rows = append(rows, sdk.RenderTextLineWithTimestamp(row.Text, row.Timestamp, width, systemStyle, timestampStyle))
			}
		case sdk.RowPermission:
			rows = append(rows, permStyle.Render(row.Text))
		case sdk.RowResponse:
			if narrow {
				rule := strings.Repeat("─", max(0, width))
				rows = append(rows, narrowRuleStyle.Render(rule))
			} else {
				rows = append(rows, renderResponseDivider(width))
			}
		case sdk.RowProse:
			line := sdk.RenderMarkdownProseLine(row.Text, mdStyles)
			if sdkResponseTextKind(prevKind) {
				rows = append(rows, line)
			} else {
				rows = append(rows, sdk.RenderStyledLineWithTimestamp(line, row.Timestamp, width, timestampStyle))
			}
		case sdk.RowCodeBlock:
			line := sdk.ToolCallIndent + sdk.RenderStructuredChildLine("│ "+row.Text, codeGutterStyle, codeStyle)
			if sdkResponseTextKind(prevKind) {
				rows = append(rows, line)
			} else {
				rows = append(rows, sdk.RenderStyledLineWithTimestamp(line, row.Timestamp, width, timestampStyle))
			}
		case sdk.RowStatus:
			rows = append(rows, statusStyle.Render(row.Text))
		case sdk.RowThinking:
			rows = append(rows, thinkingStyle.Render(row.Text))
		}
		prevKind = row.Kind
	}
	return rows
}

func sdkResponseTextKind(kind sdk.PresentationRowKind) bool {
	return kind == sdk.RowProse || kind == sdk.RowCodeBlock
}

func sdkComposerOwnerKey(instance *session.Instance) string {
	if instance == nil {
		return ""
	}
	if session.NormalizeExecutionMode(instance.ExecutionMode) == session.ExecutionModeSDK && !instance.Started() {
		return fmt.Sprintf(
			"sdk-placeholder|%s|%s|%s",
			strings.TrimSpace(instance.Title),
			strings.TrimSpace(instance.Program),
			strings.TrimSpace(instance.Path),
		)
	}
	return instance.IdentityKey()
}

// renderResponseDivider returns a muted horizontal rule separating tool/setup
// noise from assistant prose.
// See variant-c in docs/agent-sdk-pane-mockups.md.
func renderResponseDivider(width int) string {
	ruleStyle := lipgloss.NewStyle().Foreground(ColorMuted)
	if width <= 0 {
		return ""
	}
	return ruleStyle.Render(strings.Repeat("─", width))
}

// renderComposerFooter returns a quiet display-only footer appended below the
// turn timeline. The send overlay is not plumbed into the pane in this plan.
func renderComposerFooter(width int, composer string, cursor int, images []string, focused bool, program string, speedTier string, shellMode bool) []string {
	ruleStyle := lipgloss.NewStyle().Foreground(ColorMuted)
	promptPrefixStyle := lipgloss.NewStyle().Foreground(ColorRose)
	placeholderStyle := lipgloss.NewStyle().Foreground(ColorSubtle)
	composerStyle := lipgloss.NewStyle().Foreground(ColorText)
	hintStyle := lipgloss.NewStyle().Foreground(ColorSubtle)
	metaStyle := lipgloss.NewStyle().Foreground(ColorIris)
	attachmentStyle := lipgloss.NewStyle().Foreground(ColorGold)
	enterStyle := lipgloss.NewStyle().Foreground(ColorFoam)
	newlineStyle := lipgloss.NewStyle().Foreground(ColorGold)
	escapeStyle := lipgloss.NewStyle().Foreground(ColorRose)

	if shellMode {
		composerStyle = composerStyle.Foreground(ColorRose)
	}

	rule := ""
	if width > 0 {
		rule = ruleStyle.Render(strings.Repeat("─", width))
	}
	var prompt string
	if shellMode {
		if composer != "" || focused {
			cursorStyle := composerStyle.Reverse(true)
			body := renderComposerPromptBody(composer, cursor, focused, composerStyle, cursorStyle)
			prompt = promptPrefixStyle.Render("!") + " " + body
		} else {
			prompt = sdk.RenderPromptLine("!", "run a shell command …", promptPrefixStyle, placeholderStyle)
		}
	} else {
		if composer != "" || focused {
			cursorStyle := composerStyle.Reverse(true)
			body := renderComposerPromptBody(composer, cursor, focused, composerStyle, cursorStyle)
			prompt = promptPrefixStyle.Render(">") + " " + body
		} else {
			prompt = sdk.RenderPromptLine(">", "send a message to the agent …", promptPrefixStyle, placeholderStyle)
		}
	}
	sendLabel := "send"
	if shellMode {
		sendLabel = "run"
	}
	hints := enterStyle.Render("enter") +
		hintStyle.Render(" "+sendLabel+"   ") +
		newlineStyle.Render("shift+enter") +
		hintStyle.Render(" newline   ") +
		escapeStyle.Render("esc") +
		hintStyle.Render(" stop")
	rows := []string{rule, prompt, ""}
	if attachmentLabel := sdkFooterAttachmentLabel(len(images)); attachmentLabel != "" {
		rows = append(rows, attachmentStyle.Render(attachmentLabel))
	}
	statusLabel := sdkFooterModelAndEffort(program)
	if tierLabel := sdkFooterSpeedTier(speedTier); tierLabel != "" {
		if statusLabel != "" {
			statusLabel += " " + tierLabel
		} else {
			statusLabel = tierLabel
		}
	}
	if statusLabel != "" {
		rows = append(rows, metaStyle.Render(statusLabel)+hintStyle.Render(" • ")+hints)
	} else {
		rows = append(rows, hints)
	}
	return rows
}

// clampComposerCursor returns cursor clamped to [0, length].
func clampComposerCursor(cursor, length int) int {
	if cursor < 0 {
		return 0
	}
	if cursor > length {
		return length
	}
	return cursor
}

// renderComposerPromptBody renders the composer text with an ANSI block cursor
// at the given rune index. When not focused the text is rendered unstyled.
// When focused and composer is empty or cursor is at end, a "█" block is appended.
// When cursor rests on a newline, a "█" is shown and the newline is preserved.
// Otherwise the rune at cursor is highlighted with a reversed style.
func renderComposerPromptBody(composer string, cursor int, focused bool, composerStyle, cursorStyle lipgloss.Style) string {
	if !focused {
		return composerStyle.Render(composer)
	}
	runes := []rune(composer)
	cursor = clampComposerCursor(cursor, len(runes))
	if len(runes) == 0 || cursor == len(runes) {
		return composerStyle.Render(string(runes)) + cursorStyle.Render("█")
	}
	left := string(runes[:cursor])
	if runes[cursor] == '\n' {
		return composerStyle.Render(left) + cursorStyle.Render("█") + "\n" + composerStyle.Render(string(runes[cursor+1:]))
	}
	return composerStyle.Render(left) + cursorStyle.Render(string(runes[cursor])) + composerStyle.Render(string(runes[cursor+1:]))
}

func sdkFooterAttachmentLabel(count int) string {
	switch {
	case count <= 0:
		return ""
	case count == 1:
		return "1 image attached"
	default:
		return fmt.Sprintf("%d images attached", count)
	}
}

func sdkFooterSpeedTier(speedTier string) string {
	switch strings.TrimSpace(strings.ToLower(speedTier)) {
	case "fast":
		return "fast (2x)"
	default:
		return ""
	}
}

func sdkFooterModelAndEffort(program string) string {
	model, effort := parseSDKProgramModelAndEffort(program)
	switch {
	case model != "" && effort != "":
		return model + " " + effort
	case model != "":
		return model
	case effort != "":
		return effort
	default:
		return ""
	}
}

func parseSDKProgramModelAndEffort(program string) (string, string) {
	tokens := strings.Fields(strings.TrimSpace(program))
	if len(tokens) == 0 {
		return "", ""
	}

	var model string
	var effort string
	for i := 0; i < len(tokens); i++ {
		tok := tokens[i]
		switch {
		case tok == "--model" || tok == "-m":
			if i+1 < len(tokens) {
				model = tokens[i+1]
				i++
			}
		case strings.HasPrefix(tok, "--model="):
			model = strings.TrimPrefix(tok, "--model=")
		case tok == "--effort":
			if i+1 < len(tokens) {
				effort = tokens[i+1]
				i++
			}
		case strings.HasPrefix(tok, "--effort="):
			effort = strings.TrimPrefix(tok, "--effort=")
		case tok == "-c":
			if i+1 < len(tokens) {
				effort = parseSDKConfigToken(tokens[i+1], effort)
				i++
			}
		case strings.HasPrefix(tok, "-c="):
			effort = parseSDKConfigToken(strings.TrimPrefix(tok, "-c="), effort)
		}
	}

	if slash := strings.LastIndex(model, "/"); slash >= 0 {
		model = model[slash+1:]
	}
	return model, effort
}

func parseSDKConfigToken(token, currentEffort string) string {
	if strings.HasPrefix(token, "reasoning.effort=") {
		return strings.TrimPrefix(token, "reasoning.effort=")
	}
	return currentEffort
}

func (p *PreviewPane) SetSDKFocusMode(enabled bool) {
	p.sdkFocusMode = enabled
}

// composerRunes returns the composer text as a rune slice.
func (p *PreviewPane) composerRunes() []rune {
	return []rune(p.sdkComposerText)
}

// setComposerRunes replaces the composer text and clamps the cursor to the new length.
func (p *PreviewPane) setComposerRunes(runes []rune) {
	p.sdkComposerText = string(runes)
	p.sdkComposerCursor = clampComposerCursor(p.sdkComposerCursor, len(runes))
}

// insertSDKComposerText inserts text at the current cursor position and
// advances the cursor by the number of inserted runes.
func (p *PreviewPane) insertSDKComposerText(text string) {
	if text == "" {
		return
	}
	runes := p.composerRunes()
	cursor := clampComposerCursor(p.sdkComposerCursor, len(runes))
	ins := []rune(text)
	newRunes := make([]rune, 0, len(runes)+len(ins))
	newRunes = append(newRunes, runes[:cursor]...)
	newRunes = append(newRunes, ins...)
	newRunes = append(newRunes, runes[cursor:]...)
	p.sdkComposerText = string(newRunes)
	p.sdkComposerCursor = cursor + len(ins)
}

// deleteSDKComposerRange removes runes in [start, end) and clamps the cursor.
func (p *PreviewPane) deleteSDKComposerRange(start, end int) {
	runes := p.composerRunes()
	n := len(runes)
	if start < 0 {
		start = 0
	}
	if end > n {
		end = n
	}
	if start >= end {
		return
	}
	newRunes := make([]rune, 0, n-(end-start))
	newRunes = append(newRunes, runes[:start]...)
	newRunes = append(newRunes, runes[end:]...)
	p.setComposerRunes(newRunes)
}

// isComposerWordRune reports whether r is a word rune for composer word-movement.
// Unicode letters, digits, and underscores are word runes; emoji and punctuation are separators.
func isComposerWordRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_'
}

// wordBoundaryLeft returns the rune index of the left word boundary from cursor.
// It skips separators leftward, then skips word runes leftward.
func wordBoundaryLeft(runes []rune, cursor int) int {
	i := cursor
	for i > 0 && !isComposerWordRune(runes[i-1]) {
		i--
	}
	for i > 0 && isComposerWordRune(runes[i-1]) {
		i--
	}
	return i
}

// wordBoundaryRight returns the rune index of the right word boundary from cursor.
// It skips separators rightward, then skips word runes rightward.
func wordBoundaryRight(runes []rune, cursor int) int {
	n := len(runes)
	i := cursor
	for i < n && !isComposerWordRune(runes[i]) {
		i++
	}
	for i < n && isComposerWordRune(runes[i]) {
		i++
	}
	return i
}

// lineColumn returns the rune index of the start of the line containing cursor
// (the index after the previous newline, or 0) and the column offset within that line.
func lineColumn(runes []rune, cursor int) (lineStart, column int) {
	lineStart = 0
	for i := cursor - 1; i >= 0; i-- {
		if runes[i] == '\n' {
			lineStart = i + 1
			break
		}
	}
	return lineStart, cursor - lineStart
}

// lineEnd returns the rune index of the next newline at or after start,
// or len(runes) if none exists.
func lineEnd(runes []rune, start int) int {
	for i := start; i < len(runes); i++ {
		if runes[i] == '\n' {
			return i
		}
	}
	return len(runes)
}

// AppendSDKComposerText inserts text at the cursor position and advances the cursor.
func (p *PreviewPane) AppendSDKComposerText(text string) {
	if text == "" {
		return
	}
	p.insertSDKComposerText(text)
}

// InsertSDKComposerNewline inserts a newline at the cursor position.
func (p *PreviewPane) InsertSDKComposerNewline() {
	p.insertSDKComposerText("\n")
}

// AppendSDKComposerImage appends a local image path to the attachment list.
// It does not alter the cursor position.
func (p *PreviewPane) AppendSDKComposerImage(path string) {
	if strings.TrimSpace(path) == "" {
		return
	}
	p.sdkComposerImages = append(p.sdkComposerImages, path)
}

// DeleteSDKComposerBackward deletes the rune immediately before the cursor.
// It is a no-op when the cursor is at position 0.
func (p *PreviewPane) DeleteSDKComposerBackward() {
	cursor := p.sdkComposerCursor
	if cursor == 0 {
		return
	}
	p.deleteSDKComposerRange(cursor-1, cursor)
	p.sdkComposerCursor = cursor - 1
}

// DeleteSDKComposerForward deletes the rune at the cursor position.
// It is a no-op when the cursor is at the end of the text.
func (p *PreviewPane) DeleteSDKComposerForward() {
	runes := p.composerRunes()
	cursor := p.sdkComposerCursor
	if cursor >= len(runes) {
		return
	}
	p.deleteSDKComposerRange(cursor, cursor+1)
	// cursor stays at the same position after a forward delete
}

// MoveSDKComposerCursorLeft moves the cursor one rune to the left.
func (p *PreviewPane) MoveSDKComposerCursorLeft() {
	if p.sdkComposerCursor > 0 {
		p.sdkComposerCursor--
	}
}

// MoveSDKComposerCursorRight moves the cursor one rune to the right.
func (p *PreviewPane) MoveSDKComposerCursorRight() {
	runes := p.composerRunes()
	if p.sdkComposerCursor < len(runes) {
		p.sdkComposerCursor++
	}
}

// MoveSDKComposerCursorWordLeft moves the cursor to the left word boundary.
func (p *PreviewPane) MoveSDKComposerCursorWordLeft() {
	runes := p.composerRunes()
	p.sdkComposerCursor = wordBoundaryLeft(runes, p.sdkComposerCursor)
}

// MoveSDKComposerCursorWordRight moves the cursor to the right word boundary.
func (p *PreviewPane) MoveSDKComposerCursorWordRight() {
	runes := p.composerRunes()
	p.sdkComposerCursor = wordBoundaryRight(runes, p.sdkComposerCursor)
}

// MoveSDKComposerCursorLineStart moves the cursor to the start of the current line.
func (p *PreviewPane) MoveSDKComposerCursorLineStart() {
	runes := p.composerRunes()
	lineStart, _ := lineColumn(runes, p.sdkComposerCursor)
	p.sdkComposerCursor = lineStart
}

// MoveSDKComposerCursorLineEnd moves the cursor to the end of the current line.
func (p *PreviewPane) MoveSDKComposerCursorLineEnd() {
	runes := p.composerRunes()
	lineStart, _ := lineColumn(runes, p.sdkComposerCursor)
	p.sdkComposerCursor = lineEnd(runes, lineStart)
}

// MoveSDKComposerCursorUp moves the cursor to the same column on the previous line,
// clamping to the line length. It is a no-op when already on the first line.
func (p *PreviewPane) MoveSDKComposerCursorUp() {
	runes := p.composerRunes()
	lineStart, col := lineColumn(runes, p.sdkComposerCursor)
	if lineStart == 0 {
		return
	}
	prevLineEnd := lineStart - 1 // index of the '\n' character
	prevLineStart, _ := lineColumn(runes, prevLineEnd)
	prevLineLen := prevLineEnd - prevLineStart
	targetCol := col
	if targetCol > prevLineLen {
		targetCol = prevLineLen
	}
	p.sdkComposerCursor = prevLineStart + targetCol
}

// MoveSDKComposerCursorDown moves the cursor to the same column on the next line,
// clamping to the line length. It is a no-op when already on the last line.
func (p *PreviewPane) MoveSDKComposerCursorDown() {
	runes := p.composerRunes()
	lineStart, col := lineColumn(runes, p.sdkComposerCursor)
	end := lineEnd(runes, lineStart)
	if end == len(runes) {
		return
	}
	nextLineStart := end + 1
	nextLineEnd := lineEnd(runes, nextLineStart)
	nextLineLen := nextLineEnd - nextLineStart
	targetCol := col
	if targetCol > nextLineLen {
		targetCol = nextLineLen
	}
	p.sdkComposerCursor = nextLineStart + targetCol
}

// DeleteSDKComposerWordBackward deletes from the left word boundary to the cursor.
func (p *PreviewPane) DeleteSDKComposerWordBackward() {
	runes := p.composerRunes()
	cursor := p.sdkComposerCursor
	boundary := wordBoundaryLeft(runes, cursor)
	if boundary == cursor {
		return
	}
	p.deleteSDKComposerRange(boundary, cursor)
	p.sdkComposerCursor = boundary
}

// DeleteSDKComposerWordForward deletes from the cursor to the right word boundary.
func (p *PreviewPane) DeleteSDKComposerWordForward() {
	runes := p.composerRunes()
	cursor := p.sdkComposerCursor
	boundary := wordBoundaryRight(runes, cursor)
	if boundary == cursor {
		return
	}
	p.deleteSDKComposerRange(cursor, boundary)
	// cursor stays at same position (already set by setComposerRunes clamp)
}

// SDKComposerCursor returns the current cursor position as a rune index.
func (p *PreviewPane) SDKComposerCursor() int { return p.sdkComposerCursor }

func (p *PreviewPane) SDKComposerText() string { return p.sdkComposerText }

func (p *PreviewPane) SDKComposerImages() []string {
	if len(p.sdkComposerImages) == 0 {
		return nil
	}
	out := make([]string, len(p.sdkComposerImages))
	copy(out, p.sdkComposerImages)
	return out
}

// ClearSDKComposerText clears the composer text, images, cursor, and shell mode.
func (p *PreviewPane) ClearSDKComposerText() {
	p.sdkComposerText = ""
	p.sdkComposerImages = nil
	p.sdkComposerCursor = 0
	p.sdkComposerShellMode = false
}

// SDKComposerShellMode reports whether the SDK composer is in shell-command mode.
func (p *PreviewPane) SDKComposerShellMode() bool { return p.sdkComposerShellMode }

// SetSDKComposerShellMode enables or disables shell-command mode on the SDK composer.
func (p *PreviewPane) SetSDKComposerShellMode(on bool) { p.sdkComposerShellMode = on }

// ClearSDKComposerShellMode exits shell-command mode on the SDK composer.
func (p *PreviewPane) ClearSDKComposerShellMode() { p.sdkComposerShellMode = false }

// wrapPreviewRows splits text into logical lines, then hard-wraps each line
// to width using ANSI-aware wrapping. Empty input and non-positive widths
// are handled by returning the raw split. The returned slice represents
// visible terminal rows — each entry is guaranteed to render in a single
// row at the given width, so downstream tail-slicing by count matches what
// the user actually sees.
func wrapPreviewRows(text string, width int) []string {
	lines := strings.Split(text, "\n")
	if width <= 0 {
		return lines
	}
	rows := make([]string, 0, len(lines))
	for _, line := range lines {
		if ansi.StringWidth(line) <= width {
			rows = append(rows, line)
			continue
		}
		wrapped := ansi.Hardwrap(line, width, false)
		rows = append(rows, strings.Split(wrapped, "\n")...)
	}
	return rows
}

// buildFallbackText constructs the text for fallback (no active session) rendering.
func (p *PreviewPane) buildFallbackText() string {
	// Content fallback (loading spinner, paused state, exited, etc.)
	if p.previewState.fallbackMsg == "" {
		if p.previewState.text != "" {
			return p.previewState.text
		}
		// Banner only — no CTA message.
		bannerLines := BannerLines(p.bannerFrame)
		if p.springAnim != nil && !p.springAnim.Settled() {
			visibleRows := p.springAnim.VisibleRows()
			if visibleRows <= 0 {
				return ""
			}
			total := len(bannerLines)
			start := (total - visibleRows) / 2
			end := start + visibleRows
			if start < 0 {
				start = 0
			}
			if end > total {
				end = total
			}
			return strings.Join(bannerLines[start:end], "\n")
		}
		return FallBackText(p.bannerFrame)
	}

	// Banner + CTA message fallback.
	bannerLines := BannerLines(p.bannerFrame)
	animating := p.springAnim != nil && !p.springAnim.Settled()
	visibleRows := len(bannerLines)
	if animating {
		visibleRows = p.springAnim.VisibleRows()
	}

	if visibleRows <= 0 {
		// Reserve stable space while spring starts up.
		bannerWidth := lipgloss.Width(bannerLines[0])
		blankBanner := strings.Repeat(strings.Repeat(" ", bannerWidth)+"\n", len(bannerLines)-1)
		blankBanner += strings.Repeat(" ", bannerWidth)
		blankCTA := strings.Repeat(" ", lipgloss.Width(p.previewState.fallbackMsg))
		return lipgloss.JoinVertical(lipgloss.Left, blankBanner, "", blankCTA)
	}

	// Build banner block with hidden rows as blanks to keep height constant.
	totalRows := len(bannerLines)
	startRow := (totalRows - visibleRows) / 2
	endRow := startRow + visibleRows
	if startRow < 0 {
		startRow = 0
	}
	if endRow > totalRows {
		endRow = totalRows
	}

	bannerWidth := lipgloss.Width(bannerLines[0])
	blankLine := strings.Repeat(" ", bannerWidth)
	bannerParts := make([]string, 0, totalRows)
	for range startRow {
		bannerParts = append(bannerParts, blankLine)
	}
	bannerParts = append(bannerParts, bannerLines[startRow:endRow]...)
	for range totalRows - endRow {
		bannerParts = append(bannerParts, blankLine)
	}
	banner := strings.Join(bannerParts, "\n")

	// CTA: horizontal character-by-character reveal after spring delay.
	ctaMsg := p.previewState.fallbackMsg
	ctaRunes := []rune(ctaMsg)
	ctaFullWidth := lipgloss.Width(ctaMsg)
	ctaPad := (bannerWidth - ctaFullWidth) / 2
	if ctaPad < 0 {
		ctaPad = 0
	}

	if p.springAnim != nil {
		p.springAnim.SetCTALen(len(ctaRunes))
	}

	visChars := len(ctaRunes) // default: fully visible when settled
	if p.springAnim != nil && !p.springAnim.Settled() {
		visChars = p.springAnim.CTAVisibleChars()
	}

	switch {
	case visChars <= 0:
		blankCTA := strings.Repeat(" ", ctaFullWidth)
		centeredCTA := strings.Repeat(" ", ctaPad) + blankCTA
		return lipgloss.JoinVertical(lipgloss.Left, banner, "", centeredCTA)
	case visChars >= len(ctaRunes):
		centeredCTA := strings.Repeat(" ", ctaPad) + ctaMsg
		return lipgloss.JoinVertical(lipgloss.Left, banner, "", centeredCTA)
	default:
		shown := string(ctaRunes[:visChars])
		remaining := ctaFullWidth - lipgloss.Width(shown)
		partialCTA := strings.Repeat(" ", ctaPad) + shown + strings.Repeat(" ", remaining)
		return lipgloss.JoinVertical(lipgloss.Left, banner, "", partialCTA)
	}
}

func (p *PreviewPane) scrollbackContent(instance *session.Instance) (string, error) {
	if instance == nil {
		return "", nil
	}
	if session.NormalizeExecutionMode(instance.ExecutionMode) == session.ExecutionModeSDK {
		// For SDK scroll mode, load only the body region into the viewport so
		// the composer footer is not scrolled away and the sticky strip can be
		// pinned outside the viewport.
		if p.sdkView != nil {
			return p.sdkView.body, nil
		}
		if turns := instance.CapturePresentation(); len(turns) > 0 {
			width := p.viewport.Width()
			if width <= 0 {
				width = p.width
			}
			view := buildSDKPresentationView(turns, width, "", 0, nil, false, instance.Program, instance.SDKSpeedTier, time.Now(), false)
			p.sdkView = &view
			return view.body, nil
		}
		if instance.CachedContentSet && instance.CachedContent != "" {
			return instance.CachedContent, nil
		}
	}
	return instance.PreviewFullHistory()
}

// enterScrollMode captures the full preview history and sets up the viewport
// for scroll mode. Shared by all scroll entry points.
//
// For SDK sessions with an active sticky strip, the viewport height is reduced
// by one row so the pinned strip can be rendered below the viewport in String().
func (p *PreviewPane) enterScrollMode(instance *session.Instance) error {
	content, err := p.scrollbackContent(instance)
	if err != nil {
		return err
	}

	// For SDK sessions with a running activity, pin the activity + escape hint
	// as a bottom strip outside the viewport. The viewport height is reduced by
	// one row to make room for it.
	p.sdkScrollStrip = ""
	viewportHeight := p.height
	if p.sdkView != nil && p.sdkView.sticky != "" {
		escHint := lipgloss.NewStyle().Foreground(ColorMuted).Render("esc exit scroll mode")
		p.sdkScrollStrip = p.sdkView.sticky + " · " + escHint
		viewportHeight = p.height - 1
		if viewportHeight < 1 {
			viewportHeight = 1
		}
	}
	p.viewport.SetHeight(viewportHeight)

	footer := lipgloss.NewStyle().Foreground(ColorMuted).Render("esc exit scroll mode")
	if p.sdkScrollStrip != "" {
		p.viewport.SetContent(content)
	} else {
		p.viewport.SetContent(lipgloss.JoinVertical(lipgloss.Left, content, footer))
	}
	p.viewport.GotoBottom()
	p.isScrolling = true
	return nil
}

// ScrollUp scrolls the preview up one line. Enters scroll mode on first call.
func (p *PreviewPane) ScrollUp(instance *session.Instance) error {
	if p.isDocument {
		p.viewport.ScrollUp(1)
		return nil
	}
	if instance == nil || instance.Status == session.Paused {
		return nil
	}
	if !p.isScrolling {
		if err := p.enterScrollMode(instance); err != nil {
			return err
		}
		return nil
	}
	p.viewport.ScrollUp(1)
	return nil
}

// ScrollDown scrolls the preview down one line. Enters scroll mode on first call.
func (p *PreviewPane) ScrollDown(instance *session.Instance) error {
	if p.isDocument {
		p.viewport.ScrollDown(1)
		return nil
	}
	if instance == nil || instance.Status == session.Paused {
		return nil
	}
	if !p.isScrolling {
		if err := p.enterScrollMode(instance); err != nil {
			return err
		}
		return p.autoExitScrollMode(instance)
	}
	p.viewport.ScrollDown(1)
	return p.autoExitScrollMode(instance)
}

// HalfPageUp scrolls up half a viewport height. Enters scroll mode on first call.
func (p *PreviewPane) HalfPageUp(instance *session.Instance) error {
	if p.isDocument {
		p.viewport.HalfPageUp()
		return nil
	}
	if instance == nil || instance.Status == session.Paused {
		return nil
	}
	if !p.isScrolling {
		if err := p.enterScrollMode(instance); err != nil {
			return err
		}
	}
	p.viewport.HalfPageUp()
	return nil
}

// HalfPageDown scrolls down half a viewport height. Enters scroll mode on first call.
func (p *PreviewPane) HalfPageDown(instance *session.Instance) error {
	if p.isDocument {
		p.viewport.HalfPageDown()
		return nil
	}
	if instance == nil || instance.Status == session.Paused {
		return nil
	}
	if !p.isScrolling {
		if err := p.enterScrollMode(instance); err != nil {
			return err
		}
	}
	p.viewport.HalfPageDown()
	return p.autoExitScrollMode(instance)
}

// ResetToNormalMode exits scroll mode and returns to live preview.
func (p *PreviewPane) ResetToNormalMode(instance *session.Instance) error {
	if instance == nil || instance.Status == session.Paused {
		return nil
	}
	if !p.isScrolling {
		return nil
	}
	p.isScrolling = false
	p.sdkScrollStrip = ""
	p.viewport.SetHeight(p.height)
	p.viewport.SetContent("")
	p.viewport.GotoTop()

	// Fetch fresh preview content immediately rather than waiting for next tick.
	content, err := instance.Preview()
	if err != nil {
		return err
	}
	p.previewState.text = content
	return nil
}
