package ui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/kastheco/kasmos/session"
	"github.com/kastheco/kasmos/session/sdk"
)

var (
	previewPaneStyle    = lipgloss.NewStyle().Foreground(ColorText)
	scrollbarTrackStyle = lipgloss.NewStyle().Foreground(ColorOverlay)
	scrollbarThumbStyle = lipgloss.NewStyle().Foreground(ColorIris)
)

// previewState holds the current display state of the preview pane.
type previewState struct {
	// fallback indicates the pane is in fallback (no active session) mode.
	fallback bool
	// fallbackMsg is shown below the banner in banner-fallback mode.
	fallbackMsg string
	// text holds the raw content to display in normal or fallback-content modes.
	text string
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
	// sdkComposerImages are local image attachments queued for the next SDK prompt.
	sdkComposerImages []string
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
		if instance.Title != "" {
			instanceKey = instance.Title
		} else {
			instanceKey = fmt.Sprintf("%p", instance)
		}
	}
	if instanceKey != p.lastInstanceKey {
		p.isScrolling = false
		p.viewport.SetContent("")
		p.viewport.GotoTop()
		p.sdkComposerText = ""
		p.sdkComposerImages = nil
		p.lastInstanceKey = instanceKey
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
		p.setFallbackContent(lipgloss.JoinVertical(lipgloss.Center,
			lipgloss.NewStyle().Foreground(ColorMuted).Render("session exited"),
			"",
			lipgloss.NewStyle().Foreground(ColorMuted).Render("press delete to remove"),
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
			p.previewState = previewState{text: renderSDKPresentationWithComposer(turns, p.width, p.sdkComposerText, p.sdkComposerImages, p.sdkFocusMode, instance.Program, instance.SDKSpeedTier)}
			p.isRawTerminal = false
			return nil
		}
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
		p.previewState = previewState{text: renderSDKPresentationWithComposer(nil, p.width, p.sdkComposerText, p.sdkComposerImages, p.sdkFocusMode, instance.Program, instance.SDKSpeedTier)}
		p.isRawTerminal = false
		return nil
	}

	// Normal mode: live content arrives via SetRawContent from the VT emulator.
	return nil
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
		if scrollbar == "" {
			return viewContent
		}
		return lipgloss.JoinHorizontal(lipgloss.Top, viewContent, scrollbar)
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
	return renderSDKPresentationWithComposer(turns, width, "", nil, false, "", "")
}

func renderSDKPresentationWithComposer(turns []*sdk.PresentationTurn, width int, composer string, images []string, focused bool, program string, speedTier string) string {
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
		rows := renderSDKTurn(turn, width)
		if len(rows) > 0 {
			parts = append(parts, strings.Join(rows, "\n"))
		}
	}

	var sb strings.Builder
	for i, part := range parts {
		if i > 0 {
			sb.WriteString(sep)
		}
		sb.WriteString(part)
	}

	footerRows := renderComposerFooter(width, composer, images, focused, program, speedTier)
	if sb.Len() > 0 {
		sb.WriteString(sep)
	}
	sb.WriteString(strings.Join(footerRows, "\n"))
	return sb.String()
}

// renderSDKTurn renders one turn block as a slice of styled lines following
// variant-c colour assignments (see docs/agent-sdk-pane-mockups.md):
//   - header and tool rows: ColorSubtle (secondary)
//   - ok-result, system, and thinking rows: ColorMuted (quieter than tools)
//   - error-result rows: ColorLove
//   - permission rows: ColorRose (salmon)
//   - prose rows: ColorText (primary)
//   - user rows: ColorFoam
//   - RowResponse sentinel: emits a muted divider rule
//   - RowStatus (interrupted) rows: ColorGold (warning amber)
//
// In narrow mode (width < narrowPaneThreshold):
//   - the header is reduced to just "turn N" (no elapsed, tool count, running label)
//   - the response divider collapses to a single muted rule row
func renderSDKTurn(turn *sdk.PresentationTurn, width int) []string {
	narrow := width > 0 && width < narrowPaneThreshold
	var rows []string

	headerStyle := lipgloss.NewStyle().Foreground(ColorSubtle)
	if narrow {
		rows = append(rows, headerStyle.Render(fmt.Sprintf("turn %d", turn.Number)))
	} else {
		rows = append(rows, headerStyle.Render(turn.HeaderText(time.Now())))
	}

	toolStyle := lipgloss.NewStyle().Foreground(ColorSubtle)
	toolArgStyle := lipgloss.NewStyle().Foreground(ColorText)
	userStyle := lipgloss.NewStyle().Foreground(ColorFoam)
	resultOKStyle := lipgloss.NewStyle().Foreground(ColorMuted)
	resultErrStyle := lipgloss.NewStyle().Foreground(ColorLove)
	systemStyle := lipgloss.NewStyle().Foreground(ColorMuted)
	permStyle := lipgloss.NewStyle().Foreground(ColorRose)
	proseStyle := lipgloss.NewStyle().Foreground(ColorText)
	statusStyle := lipgloss.NewStyle().Foreground(ColorGold)
	thinkingStyle := lipgloss.NewStyle().Foreground(ColorMuted)
	narrowRuleStyle := lipgloss.NewStyle().Foreground(ColorMuted)
	gutterStyle := lipgloss.NewStyle().Foreground(ColorMuted)
	addedStyle := lipgloss.NewStyle().Foreground(ColorRose)
	removedStyle := lipgloss.NewStyle().Foreground(ColorLove)
	diffContextStyle := lipgloss.NewStyle().Foreground(ColorMuted)

	for _, row := range turn.Rows {
		switch row.Kind {
		case sdk.RowUser:
			rows = append(rows, userStyle.Render("> "+row.Text))
		case sdk.RowTool:
			head, args := sdk.SplitToolCallText(row.Text, row.ToolName)
			line := sdk.RenderToolCallLine(head, args, toolStyle, toolArgStyle)
			if line == "" {
				rows = append(rows, line)
			} else {
				rows = append(rows, sdk.ToolCallIndent+line)
			}
		case sdk.RowResult:
			var styled string
			if row.IsError {
				styled = resultErrStyle.Render(row.Text)
			} else {
				styled = resultOKStyle.Render(row.Text)
			}
			if styled != "" {
				rows = append(rows, sdk.ToolChildIndent+styled)
			} else {
				rows = append(rows, styled)
			}
		case sdk.RowToolPreview:
			if row.ToolPreview == nil {
				break
			}
			previewRows := sdk.BuildToolPreviewBlock(row.ToolPreview)
			for _, pr := range previewRows {
				rows = append(rows, sdk.ToolChildIndent+gutterStyle.Render(pr))
			}
		case sdk.RowToolDiff:
			if row.ToolDiff == nil {
				break
			}
			diffRows := sdk.BuildToolDiffBlock(row.ToolDiff)
			for i, dr := range diffRows {
				var styled string
				if row.ToolDiff.Truncated && i == len(row.ToolDiff.Lines) {
					styled = diffContextStyle.Render(dr)
				} else {
					switch row.ToolDiff.Lines[i].Kind {
					case sdk.DiffLineAdded:
						styled = addedStyle.Render(dr)
					case sdk.DiffLineRemoved:
						styled = removedStyle.Render(dr)
					default:
						styled = diffContextStyle.Render(dr)
					}
				}
				rows = append(rows, sdk.ToolChildIndent+styled)
			}
		case sdk.RowSystem:
			rows = append(rows, systemStyle.Render(row.Text))
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
			rows = append(rows, proseStyle.Render(row.Text))
		case sdk.RowStatus:
			rows = append(rows, statusStyle.Render(row.Text))
		case sdk.RowThinking:
			rows = append(rows, thinkingStyle.Render(row.Text))
		}
	}
	return rows
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
func renderComposerFooter(width int, composer string, images []string, focused bool, program string, speedTier string) []string {
	ruleStyle := lipgloss.NewStyle().Foreground(ColorMuted)
	placeholderStyle := lipgloss.NewStyle().Foreground(ColorMuted)
	composerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#ffffff"))
	hintStyle := lipgloss.NewStyle().Foreground(ColorSubtle)

	rule := ""
	if width > 0 {
		rule = ruleStyle.Render(strings.Repeat("─", width))
	}
	promptText := "> send a message to the agent …"
	if composer != "" || focused {
		cursor := ""
		if focused {
			cursor = "█"
		}
		promptText = "> " + composer + cursor
	}
	prompt := placeholderStyle.Render(promptText)
	if composer != "" || focused {
		prompt = composerStyle.Render(promptText)
	}
	hints := "enter send   shift+enter newline   esc unfocus"
	rows := []string{rule, prompt, ""}
	if attachmentLabel := sdkFooterAttachmentLabel(len(images)); attachmentLabel != "" {
		rows = append(rows, hintStyle.Render(attachmentLabel))
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
		rows = append(rows, hintStyle.Render(statusLabel+" • "+hints))
	} else {
		rows = append(rows, hintStyle.Render(hints))
	}
	return rows
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
	if !enabled {
		p.sdkComposerText = ""
		p.sdkComposerImages = nil
	}
}

func (p *PreviewPane) AppendSDKComposerText(text string) {
	if text == "" {
		return
	}
	p.sdkComposerText += text
}

func (p *PreviewPane) InsertSDKComposerNewline() {
	p.sdkComposerText += "\n"
}

func (p *PreviewPane) AppendSDKComposerImage(path string) {
	if strings.TrimSpace(path) == "" {
		return
	}
	p.sdkComposerImages = append(p.sdkComposerImages, path)
}

func (p *PreviewPane) DeleteSDKComposerBackward() {
	runes := []rune(p.sdkComposerText)
	if len(runes) == 0 {
		return
	}
	p.sdkComposerText = string(runes[:len(runes)-1])
}

func (p *PreviewPane) SDKComposerText() string { return p.sdkComposerText }

func (p *PreviewPane) SDKComposerImages() []string {
	if len(p.sdkComposerImages) == 0 {
		return nil
	}
	out := make([]string, len(p.sdkComposerImages))
	copy(out, p.sdkComposerImages)
	return out
}

func (p *PreviewPane) ClearSDKComposerText() {
	p.sdkComposerText = ""
	p.sdkComposerImages = nil
}

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
		if turns := instance.CapturePresentation(); len(turns) > 0 {
			width := p.viewport.Width()
			if width <= 0 {
				width = p.width
			}
			return renderSDKPresentation(turns, width), nil
		}
		if instance.CachedContentSet && instance.CachedContent != "" {
			return instance.CachedContent, nil
		}
	}
	return instance.PreviewFullHistory()
}

// enterScrollMode captures the full preview history and sets up the viewport
// for scroll mode. Shared by all scroll entry points.
func (p *PreviewPane) enterScrollMode(instance *session.Instance) error {
	content, err := p.scrollbackContent(instance)
	if err != nil {
		return err
	}
	footer := lipgloss.NewStyle().Foreground(ColorMuted).Render("ESC to exit scroll mode")
	p.viewport.SetContent(lipgloss.JoinVertical(lipgloss.Left, content, footer))
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
		return nil
	}
	p.viewport.ScrollDown(1)
	return nil
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
	return nil
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
