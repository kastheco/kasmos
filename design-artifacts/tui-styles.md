# kasmos TUI — Style & Color Specification

> Complete color palette, semantic mappings, component styles, and status indicator
> definitions. This file contains production-ready Go code for `styles.go`.

## Palette

### Core Colors

All hex values are the canonical Charm bubblegum palette.

```go
package tui

import "github.com/charmbracelet/lipgloss"

// ── Core Charm Bubblegum Palette ──

var (
    colorPurple    = lipgloss.Color("#7D56F4") // Primary accent: focus, selection, interactive
    colorHotPink   = lipgloss.Color("#F25D94") // Headers, dialog borders, emphasis
    colorGreen     = lipgloss.Color("#73F59F") // Success, done, positive
    colorLightBlue = lipgloss.Color("#82CFFF") // Info, file paths, session refs
    colorYellow    = lipgloss.Color("#EDFF82") // Warnings, suggestions, attention
    colorOrange    = lipgloss.Color("#FF9F43") // Errors, failed states, urgent
    colorCream     = lipgloss.Color("#FFFDF5") // Primary text on dark/colored backgrounds
    colorWhite     = lipgloss.Color("#FAFAFA") // Bright text
    colorDarkGray  = lipgloss.Color("#383838") // Unfocused borders, subtle backgrounds
    colorMidGray   = lipgloss.Color("#5C5C5C") // Faint text, disabled elements, separators
    colorLightGray = lipgloss.Color("#9B9B9B") // Secondary text, timestamps
)
```

### Adaptive Colors (light/dark terminal support)

```go
var (
    subtleColor    = lipgloss.AdaptiveColor{Light: "#D9DCCF", Dark: "#383838"}
    highlightColor = lipgloss.AdaptiveColor{Light: "#874BFD", Dark: "#7D56F4"}
    specialColor   = lipgloss.AdaptiveColor{Light: "#43BF6D", Dark: "#73F59F"}
)
```

### Semantic State Colors

```go
var (
    colorRunning = colorPurple   // ⣾ / ⟳ — active worker
    colorDone    = colorGreen    // ✓ — exited successfully
    colorFailed  = colorOrange   // ✗ — exited with non-zero code
    colorKilled  = colorHotPink  // ☠ — terminated by user
    colorPending = colorMidGray  // ○ — waiting to spawn
    colorWarning = colorYellow   // ⚠ — needs attention
)
```

### Semantic UI Colors

```go
var (
    colorFocusBorder   = colorPurple   // Focused panel border
    colorUnfocusBorder = colorDarkGray // Unfocused panel border
    colorDialogBorder  = colorHotPink  // Dialog overlay borders
    colorAlertBorder   = colorOrange   // Warning/quit dialogs (ThickBorder)
    colorHeader        = colorHotPink  // Section headers, titles
    colorHelp          = colorMidGray  // Help text, hints
    colorAccent        = colorLightBlue // Info badges, file paths, links
    colorTimestamp      = colorLightGray // Log timestamps
)
```

### Role Badge Colors

```go
// Role badge: colored background with contrasting text
var roleBadgeColors = map[string]struct{ bg, fg lipgloss.TerminalColor }{
    "planner":  {bg: lipgloss.Color("#2D6A4F"), fg: colorCream},
    "coder":    {bg: colorPurple, fg: colorCream},
    "reviewer": {bg: colorLightBlue, fg: lipgloss.Color("#0a0a18")},
    "release":  {bg: lipgloss.Color("#8B5CF6"), fg: colorCream},
}
```

---

## Style Definitions

### Panel Styles

```go
// Panel border styles — the primary visual organizer
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

// Helper to get panel style based on focus state
func panelStyle(focused bool) lipgloss.Style {
    if focused {
        return focusedPanelStyle
    }
    return unfocusedPanelStyle
}
```

### Header Styles

```go
var (
    // App title: gradient rendered (see gradient section below)
    titleBaseStyle = lipgloss.NewStyle().Bold(true)

    // "agent orchestrator" subtitle
    dimSubtitleStyle = lipgloss.NewStyle().
        Foreground(colorMidGray)

    // Version number (right-aligned)
    versionStyle = lipgloss.NewStyle().
        Foreground(colorLightGray)

    // Task source subtitle: "spec-kitty: kitty-specs/015/plan.md"
    sourceSubtitleStyle = lipgloss.NewStyle().
        Foreground(colorLightGray).
        MarginLeft(2)
)
```

### Title Gradient

The app title "kasmos" uses a character-by-character gradient from hot pink to purple.

```go
import "github.com/muesli/gamut"

func renderGradientTitle(text string) string {
    colors := gamut.Blends(
        colorToColor(colorHotPink),  // start: hot pink
        colorToColor(colorPurple),   // end: purple
        len(text),
    )

    var out strings.Builder
    for i, ch := range text {
        hex := gamut.ToHex(colors[i])
        out.WriteString(
            titleBaseStyle.Foreground(lipgloss.Color(hex)).Render(string(ch)),
        )
    }
    return out.String()
}

// Render: " kasmos  agent orchestrator"
// The leading space + gradient "kasmos" + 2 spaces + dim "agent orchestrator"
```

### Table Styles

```go
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
```

### Status Bar Style

```go
var statusBarStyle = lipgloss.NewStyle().
    Foreground(colorCream).
    Background(colorPurple).
    Padding(0, 1).
    Bold(false)
```

### Help Bar Styles

```go
import "github.com/charmbracelet/bubbles/help"

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
```

### Dialog Styles

```go
var (
    // Standard dialog (spawn, continue)
    dialogStyle = lipgloss.NewStyle().
        Border(lipgloss.RoundedBorder()).
        BorderForeground(colorDialogBorder).
        Padding(1, 2)

    // Alert dialog (quit confirmation)
    alertDialogStyle = lipgloss.NewStyle().
        Border(lipgloss.ThickBorder()).
        BorderForeground(colorAlertBorder).
        Padding(1, 2)

    // Dialog section header
    dialogHeaderStyle = lipgloss.NewStyle().
        Foreground(colorHotPink).
        Bold(true)

    // Active button: purple bg
    activeButtonStyle = lipgloss.NewStyle().
        Foreground(colorCream).
        Background(colorPurple).
        Padding(0, 2).
        Bold(true)

    // Inactive button: darkGray bg
    inactiveButtonStyle = lipgloss.NewStyle().
        Foreground(colorLightGray).
        Background(colorDarkGray).
        Padding(0, 2)

    // Alert button: orange bg
    alertButtonStyle = lipgloss.NewStyle().
        Foreground(lipgloss.Color("#0a0a18")).
        Background(colorOrange).
        Padding(0, 2).
        Bold(true)
)
```

### Text Input / Textarea Styles

```go
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
    ta.FocusedStyle.CursorLine = lipgloss.NewStyle().
        Background(colorDarkGray)
    ta.FocusedStyle.Base = lipgloss.NewStyle().
        Border(lipgloss.RoundedBorder()).
        BorderForeground(colorPurple)
    ta.BlurredStyle.Base = lipgloss.NewStyle().
        Border(lipgloss.RoundedBorder()).
        BorderForeground(colorDarkGray)
    return ta
}
```

### Spinner Style

```go
func styledSpinner() spinner.Model {
    s := spinner.New()
    s.Spinner = spinner.Dot  // ⣾⣽⣻⢿⡿⣟⣯⣷ — the Charm classic
    s.Style = lipgloss.NewStyle().Foreground(colorPurple)
    return s
}
```

---

## Status Indicators

### Worker State Indicators (for table cells)

```go
type WorkerState int

const (
    StatePending WorkerState = iota
    StateSpawning
    StateRunning
    StateExited     // exit code 0
    StateFailed     // exit code != 0
    StateKilled
)

// Static indicator (used for non-running states in table rows)
func statusIndicator(state WorkerState, exitCode int) string {
    switch state {
    case StateRunning:
        // NOTE: for running workers, use spinner.View() + " running" instead
        return lipgloss.NewStyle().Foreground(colorRunning).Render("⟳ running")
    case StateExited:
        return lipgloss.NewStyle().Foreground(colorDone).Render("✓ done")
    case StateFailed:
        return lipgloss.NewStyle().Foreground(colorFailed).Render(
            fmt.Sprintf("✗ failed(%d)", exitCode))
    case StateKilled:
        return lipgloss.NewStyle().Foreground(colorKilled).Render("☠ killed")
    case StatePending:
        return lipgloss.NewStyle().Foreground(colorPending).Render("○ pending")
    case StateSpawning:
        return lipgloss.NewStyle().Foreground(colorPurple).Render("◌ spawning")
    default:
        return lipgloss.NewStyle().Foreground(colorMidGray).Render("? unknown")
    }
}

// For running workers in table: inline spinner
func (m Model) runningStatusCell() string {
    return m.spinner.View() + " running"
}
```

### Task State Indicators (for task list items)

```go
type TaskState int

const (
    TaskUnassigned TaskState = iota
    TaskBlocked              // dependency not met
    TaskInProgress           // worker spawned for this task
    TaskDone                 // worker completed successfully
    TaskFailed               // worker failed
)

func taskStatusBadge(state TaskState, blockingDep string) string {
    switch state {
    case TaskDone:
        return lipgloss.NewStyle().Foreground(colorDone).Render("✓ done")
    case TaskInProgress:
        return lipgloss.NewStyle().Foreground(colorRunning).Render("⟳ in-progress")
    case TaskBlocked:
        return lipgloss.NewStyle().Foreground(colorOrange).Render(
            fmt.Sprintf("⊘ blocked (%s)", blockingDep))
    case TaskFailed:
        return lipgloss.NewStyle().Foreground(colorFailed).Render("✗ failed")
    default: // TaskUnassigned
        return lipgloss.NewStyle().Foreground(colorPending).Render("○ unassigned")
    }
}
```

### Role Badges (for table cells and dialog labels)

```go
func roleBadge(role string) string {
    colors, ok := roleBadgeColors[role]
    if !ok {
        colors = struct{ bg, fg lipgloss.TerminalColor }{
            bg: colorDarkGray, fg: colorCream,
        }
    }
    return lipgloss.NewStyle().
        Foreground(colors.fg).
        Background(colors.bg).
        Padding(0, 1).
        Render(role)
}
```

### Output Viewport Content Styling

```go
// Styled content lines in the output viewport
var (
    timestampStyle = lipgloss.NewStyle().Foreground(colorTimestamp)
    filePathStyle  = lipgloss.NewStyle().Foreground(colorLightBlue)
    successStyle   = lipgloss.NewStyle().Foreground(colorGreen)
    failStyle      = lipgloss.NewStyle().Foreground(colorOrange)
    warningStyle   = lipgloss.NewStyle().Foreground(colorYellow)
    agentTagStyle  = lipgloss.NewStyle().Foreground(colorPurple)
)

// Format an output line for the viewport
// Input:  "[14:32:01] [coder] Creating internal/auth/middleware.go"
// Output: styled version with colored timestamp, agent tag, file path
func formatOutputLine(line string) string {
    // Implementation: regex or string matching to identify and style:
    // - Timestamps [HH:MM:SS] → timestampStyle
    // - Agent tags [coder] [reviewer] → agentTagStyle
    // - File paths (anything with / that looks like a path) → filePathStyle
    // - "PASS" → successStyle
    // - "FAIL" → failStyle
    // - "Suggestion:" → warningStyle
    // - "✓" → successStyle
    // - "✗" → failStyle
    // Pass through everything else unstyled (terminal default foreground)
    return line // TODO: implement
}
```

### Analysis View Styles

```go
var (
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
)
```

---

## Panel Title Rendering

Panel titles are embedded in the top border line. This is not a built-in bubbles feature — it's a lipgloss rendering technique.

```go
// Render a panel with a title in the top border
func renderTitledPanel(title string, content string, width int, height int, focused bool) string {
    style := panelStyle(focused)

    // Build the top border manually with embedded title
    borderStyle := lipgloss.RoundedBorder()
    borderColor := colorFocusBorder
    if !focused {
        borderColor = colorUnfocusBorder
    }

    bc := lipgloss.NewStyle().Foreground(borderColor)

    // "╭─ Title ─────────╮"
    titleRendered := bc.Render(fmt.Sprintf("%s%s %s %s%s",
        string(borderStyle.TopLeft),
        string(borderStyle.Top),
        title,
        strings.Repeat(string(borderStyle.Top), max(0, width-lipgloss.Width(title)-5)),
        string(borderStyle.TopRight),
    ))

    // Content with side borders
    // ... (pad content lines, add │ on each side)

    // Bottom border
    bottomRendered := bc.Render(fmt.Sprintf("%s%s%s",
        string(borderStyle.BottomLeft),
        strings.Repeat(string(borderStyle.Bottom), width-2),
        string(borderStyle.BottomRight),
    ))

    return lipgloss.JoinVertical(lipgloss.Left, titleRendered, contentBody, bottomRendered)
}
```

**Alternative (simpler):** Just render the title as the first content line inside the border and use standard `style.Render()`:

```go
// Simpler approach: title as first line inside the panel
panelContent := lipgloss.JoinVertical(lipgloss.Left,
    bc.Render("Workers"),  // styled title line
    "",                     // blank separator
    m.table.View(),         // actual content
)
style.Width(w).Height(h).Render(panelContent)
```

---

## Overlay Backdrop

All overlays use the same backdrop pattern:

```go
func (m Model) renderWithBackdrop(dialog string) string {
    return lipgloss.Place(m.width, m.height,
        lipgloss.Center, lipgloss.Center,
        dialog,
        lipgloss.WithWhitespaceChars("░"),
        lipgloss.WithWhitespaceForeground(colorDarkGray),
    )
}
```

The `░` character in darkGray creates the signature Charm textured backdrop that makes dialogs pop visually.

---

## huh Form Theme

Use huh's built-in Charm theme for all form dialogs:

```go
form := huh.NewForm(groups...).WithTheme(huh.ThemeCharm())
```

This automatically applies purple accents, rounded borders, and the bubblegum aesthetic to all form fields (Select, Text, Input, Confirm). No custom theme needed.

---

## Color Behavior Rules

1. **Body text uses terminal default foreground.** Never force `Foreground(colorCream)` on general content. Only color text that carries semantic meaning.
2. **Purple is the interactive spine.** Focus borders, selected rows, active buttons, cursor, key labels in help. When you see purple, you know "this is where the action is."
3. **Hot pink is for headers and emphasis.** App title gradient endpoint, dialog borders, section headers in help and analysis. Less frequent than purple but higher visual weight.
4. **Green = positive, Orange = negative, Yellow = attention.** No exceptions. Don't use green for non-success states or orange for non-error states.
5. **Gray tones for everything secondary.** Timestamps → lightGray. Disabled/inactive → midGray. Borders → darkGray. These fade into the background.
6. **LightBlue is for references.** File paths, session IDs, commit hashes, links. Information that the user might want to act on.
7. **One spinner style everywhere.** `spinner.Dot` in purple. Don't mix spinner styles.
8. **Bold sparingly.** Headers, key labels, status labels. Not body text, not descriptions.
9. **Faint for hints only.** `Faint(true)` on help text, placeholder text, and "press X to do Y" hints. Not on content the user needs to read.
