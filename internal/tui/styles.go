package tui

import (
	"fmt"
	"image/color"
	"strings"

	"github.com/charmbracelet/bubbles/v2/help"
	"github.com/charmbracelet/bubbles/v2/spinner"
	"github.com/charmbracelet/bubbles/v2/table"
	"github.com/charmbracelet/bubbles/v2/textarea"
	"github.com/charmbracelet/bubbles/v2/textinput"
	"github.com/charmbracelet/lipgloss/v2"
	"github.com/lucasb-eyer/go-colorful"
	"github.com/muesli/gamut"

	"github.com/user/kasmos/internal/task"
	"github.com/user/kasmos/internal/worker"
)

var (
	colorPurple    = lipgloss.Color("#7D56F4")
	colorHotPink   = lipgloss.Color("#F25D94")
	colorGreen     = lipgloss.Color("#73F59F")
	colorLightBlue = lipgloss.Color("#82CFFF")
	colorYellow    = lipgloss.Color("#EDFF82")
	colorOrange    = lipgloss.Color("#FF9F43")
	colorCream     = lipgloss.Color("#FFFDF5")
	colorWhite     = lipgloss.Color("#FAFAFA")
	colorDarkGray  = lipgloss.Color("#383838")
	colorMidGray   = lipgloss.Color("#5C5C5C")
	colorLightGray = lipgloss.Color("#9B9B9B")
	colorBlocked   = lipgloss.Color("#555555")
)

var (
	subtleColor    color.Color = lipgloss.Color("#383838")
	highlightColor color.Color = lipgloss.Color("#7D56F4")
	specialColor   color.Color = lipgloss.Color("#73F59F")
)

var (
	colorRunning = colorPurple
	colorDone    = colorGreen
	colorFailed  = colorOrange
	colorKilled  = colorHotPink
	colorPending = colorMidGray
	colorWarning = colorYellow
)

var (
	colorFocusBorder   = colorPurple
	colorUnfocusBorder = colorDarkGray
	colorDialogBorder  = colorHotPink
	colorAlertBorder   = colorOrange
	colorHeader        = colorHotPink
	colorHelp          = colorMidGray
	colorAccent        = colorLightBlue
	colorTimestamp     = colorLightGray
)

var roleBadgeColors = map[string]struct{ bg, fg color.Color }{
	"planner":  {bg: lipgloss.Color("#2D6A4F"), fg: colorCream},
	"coder":    {bg: colorPurple, fg: colorCream},
	"reviewer": {bg: colorLightBlue, fg: lipgloss.Color("#0a0a18")},
	"release":  {bg: lipgloss.Color("#8B5CF6"), fg: colorCream},
}

var (
	focusedPanelStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(colorFocusBorder).
				Padding(0, 1)

	unfocusedPanelStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(colorUnfocusBorder).
				Padding(0, 1)
)

func panelStyle(focused bool, contentHeight int) lipgloss.Style {
	if focused {
		return focusedPanelStyle.Height(contentHeight)
	}
	return unfocusedPanelStyle.Height(contentHeight)
}

var (
	titleBaseStyle = lipgloss.NewStyle().Bold(true)

	dimSubtitleStyle = lipgloss.NewStyle().
				Foreground(colorMidGray)

	versionStyle = lipgloss.NewStyle().
			Foreground(colorLightGray)

	sourceSubtitleStyle = lipgloss.NewStyle().
				Foreground(colorLightGray).
				MarginLeft(2)
)

func renderGradientTitle(text string) string {
	if text == "" {
		return ""
	}

	start, ok := colorful.MakeColor(colorHotPink)
	if !ok {
		return titleBaseStyle.Render(text)
	}

	end, ok := colorful.MakeColor(colorPurple)
	if !ok {
		return titleBaseStyle.Render(text)
	}

	colors := gamut.Blends(start, end, len([]rune(text)))

	var out strings.Builder
	for i, ch := range text {
		hex := gamut.ToHex(colors[i])
		out.WriteString(titleBaseStyle.Foreground(lipgloss.Color(hex)).Render(string(ch)))
	}
	return out.String()
}

func workerTableStyles() table.Styles {
	s := table.DefaultStyles()

	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(colorPurple).
		BorderBottom(true).
		Bold(true).
		Foreground(colorHotPink)

	s.Selected = s.Selected.
		Foreground(colorCream).
		Background(colorPurple).
		Bold(false)

	s.Cell = s.Cell.
		Padding(0, 1)

	return s
}

var statusBarStyle = lipgloss.NewStyle().
	Foreground(colorCream).
	Padding(0, 1).
	Bold(false)

var modeIndicatorStyle = lipgloss.NewStyle().
	Foreground(colorCream).
	Background(colorPurple)

func styledHelp() help.Model {
	h := help.New()
	h.ShowAll = false

	h.Styles.ShortKey = lipgloss.NewStyle().
		Foreground(colorPurple).
		Bold(true)

	h.Styles.ShortDesc = lipgloss.NewStyle().
		Foreground(colorMidGray)

	h.Styles.ShortSeparator = lipgloss.NewStyle().
		Foreground(colorDarkGray)

	h.Styles.FullKey = lipgloss.NewStyle().
		Foreground(colorPurple).
		Bold(true)

	h.Styles.FullDesc = lipgloss.NewStyle().
		Foreground(colorLightGray)

	h.Styles.FullSeparator = lipgloss.NewStyle().
		Foreground(colorDarkGray)

	return h
}

var (
	dialogStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorDialogBorder).
			Padding(1, 2)

	alertDialogStyle = lipgloss.NewStyle().
				Border(lipgloss.ThickBorder()).
				BorderForeground(colorAlertBorder).
				Padding(1, 2)

	dialogHeaderStyle = lipgloss.NewStyle().
				Foreground(colorHotPink).
				Bold(true)

	activeButtonStyle = lipgloss.NewStyle().
				Foreground(colorCream).
				Background(colorPurple).
				Padding(0, 2).
				Bold(true)

	inactiveButtonStyle = lipgloss.NewStyle().
				Foreground(colorLightGray).
				Background(colorDarkGray).
				Padding(0, 2)

	alertButtonStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#0a0a18")).
				Background(colorOrange).
				Padding(0, 2).
				Bold(true)
)

func styledSpinner() spinner.Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(colorPurple)
	return s
}

func styledTextInput() textinput.Model {
	ti := textinput.New()
	ti.PromptStyle = lipgloss.NewStyle().Foreground(colorPurple)
	ti.TextStyle = lipgloss.NewStyle().Foreground(colorCream)
	ti.Cursor.Style = lipgloss.NewStyle().Foreground(colorHotPink)
	ti.PlaceholderStyle = lipgloss.NewStyle().Foreground(colorMidGray)
	return ti
}

func styledTextArea() textarea.Model {
	ta := textarea.New()
	ta.CharLimit = 0 // bubbles v2 alpha.2 defaults to 400; we need unlimited for WP prompts
	ta.MaxHeight = 0
	ta.Styles.Focused.CursorLine = lipgloss.NewStyle().
		Background(colorDarkGray)
	ta.Styles.Focused.Base = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorPurple)
	ta.Styles.Blurred.Base = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorDarkGray)
	return ta
}

func plainStatus(state worker.WorkerState, exitCode int) string {
	switch state {
	case worker.StateRunning:
		return "⟳ running"
	case worker.StateExited:
		return "✓ done"
	case worker.StateFailed:
		return fmt.Sprintf("✗ fail(%d)", exitCode)
	case worker.StateKilled:
		return "☠ killed"
	case worker.StatePending:
		return "○ pending"
	case worker.StateSpawning:
		return "◌ spawning"
	default:
		return "? unknown"
	}
}

func statusIndicator(state worker.WorkerState, exitCode int) string {
	switch state {
	case worker.StateRunning:
		return lipgloss.NewStyle().Foreground(colorRunning).Render("⟳ running")
	case worker.StateExited:
		return lipgloss.NewStyle().Foreground(colorDone).Render("✓ done")
	case worker.StateFailed:
		return lipgloss.NewStyle().Foreground(colorFailed).Render(fmt.Sprintf("✗ failed(%d)", exitCode))
	case worker.StateKilled:
		return lipgloss.NewStyle().Foreground(colorKilled).Render("☠ killed")
	case worker.StatePending:
		return lipgloss.NewStyle().Foreground(colorPending).Render("○ pending")
	case worker.StateSpawning:
		return lipgloss.NewStyle().Foreground(colorPurple).Render("◌ spawning")
	default:
		return lipgloss.NewStyle().Foreground(colorMidGray).Render("? unknown")
	}
}

func taskStatusBadge(state task.TaskState, blockingDep string) string {
	switch state {
	case task.TaskDone:
		return lipgloss.NewStyle().Foreground(colorDone).Render("✓ done")
	case task.TaskForReview:
		return lipgloss.NewStyle().Foreground(colorLightBlue).Render("◆ for review")
	case task.TaskInProgress:
		return lipgloss.NewStyle().Foreground(colorRunning).Render("⟳ in-progress")
	case task.TaskBlocked:
		return lipgloss.NewStyle().Foreground(colorOrange).Render(fmt.Sprintf("⊘ blocked (%s)", blockingDep))
	case task.TaskFailed:
		return lipgloss.NewStyle().Foreground(colorFailed).Render("✗ failed")
	default:
		return lipgloss.NewStyle().Foreground(colorPending).Render("○ unassigned")
	}
}

func roleBadge(role string) string {
	colors, ok := roleBadgeColors[role]
	if !ok {
		colors = struct{ bg, fg color.Color }{
			bg: colorDarkGray,
			fg: colorCream,
		}
	}

	return lipgloss.NewStyle().
		Foreground(colors.fg).
		Background(colors.bg).
		Padding(0, 1).
		Render(role)
}

func historyTypeBadge(entryType string) string {
	style := lipgloss.NewStyle().Foreground(colorCream).Padding(0, 1)
	switch entryType {
	case "spec-kitty":
		style = style.Background(colorHotPink)
	case "gsd":
		style = style.Background(colorLightBlue).Foreground(lipgloss.Color("#0a0a18"))
	case "yolo":
		style = style.Background(colorOrange).Foreground(lipgloss.Color("#0a0a18"))
	default:
		style = style.Background(colorDarkGray)
	}
	return style.Render(entryType)
}

func historyStatusBadge(status string) string {
	base := lipgloss.NewStyle().Padding(0, 1)
	switch status {
	case "complete":
		return base.Foreground(colorCream).Background(colorGreen).Render(status)
	case "in-progress":
		return base.Foreground(colorCream).Background(colorPurple).Render(status)
	case "planned":
		return base.Foreground(colorCream).Background(colorMidGray).Render(status)
	case "partial":
		return base.Foreground(lipgloss.Color("#0a0a18")).Background(colorYellow).Render(status)
	default:
		return base.Foreground(colorCream).Background(colorDarkGray).Render(status)
	}
}

var (
	timestampStyle = lipgloss.NewStyle().Foreground(colorTimestamp)
	filePathStyle  = lipgloss.NewStyle().Foreground(colorLightBlue)
	successStyle   = lipgloss.NewStyle().Foreground(colorGreen)
	failStyle      = lipgloss.NewStyle().Foreground(colorOrange)
	warningStyle   = lipgloss.NewStyle().Foreground(colorYellow)
	agentTagStyle  = lipgloss.NewStyle().Foreground(colorPurple)

	analysisHeaderStyle = lipgloss.NewStyle().
				Foreground(colorHotPink).
				Bold(true)

	rootCauseLabelStyle = lipgloss.NewStyle().
				Foreground(colorOrange).
				Bold(true)

	suggestedFixLabelStyle = lipgloss.NewStyle().
				Foreground(colorGreen).
				Bold(true)

	analysisHintStyle = lipgloss.NewStyle().
				Foreground(colorMidGray).
				Faint(true)

	newDialogOptionStyle = lipgloss.NewStyle().
				Foreground(colorPurple).
				Bold(true)

	newDialogMutedStyle = lipgloss.NewStyle().
				Foreground(colorLightGray)

	newDialogHelpStyle = lipgloss.NewStyle().
				Foreground(colorMidGray)

	newDialogErrorStyle = lipgloss.NewStyle().
				Foreground(colorOrange).
				Bold(true)
)

func (m Model) renderWithBackdrop(dialog string) string {
	return lipgloss.Place(m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		dialog,
		lipgloss.WithWhitespaceChars("░"),
		lipgloss.WithWhitespaceStyle(lipgloss.NewStyle().Foreground(colorDarkGray)),
	)
}
