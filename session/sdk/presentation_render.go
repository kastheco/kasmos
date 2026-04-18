package sdk

import (
	"fmt"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
)

const (
	// Match ui/theme.go so daemon-served SDK previews look the same as local ones.
	presentationColorMuted  = "#6e6a86"
	presentationColorSubtle = "#908caa"
	presentationColorText   = "#e0def4"
	presentationColorLove   = "#eb6f92"
	presentationColorGold   = "#f6c177"
	presentationColorRose   = "#ea9a97"

	presentationNarrowPaneThreshold = 40
	presentationDefaultWidth        = 80
)

// RenderPresentation converts a slice of PresentationTurns into an ANSI-styled
// terminal transcript that follows the variant-c hierarchy from
// docs/agent-sdk-pane-mockups.md.
func RenderPresentation(turns []*PresentationTurn, width int) string {
	if width <= 0 {
		width = presentationDefaultWidth
	}
	narrow := width < presentationNarrowPaneThreshold
	sep := "\n\n"
	if narrow {
		sep = "\n"
	}

	var parts []string
	for _, turn := range turns {
		rows := renderPresentationTurn(turn, width)
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

	footerRows := renderPresentationComposerFooter(width)
	if sb.Len() > 0 {
		sb.WriteString(sep)
	}
	sb.WriteString(strings.Join(footerRows, "\n"))
	return sb.String()
}

func renderPresentationTurn(turn *PresentationTurn, width int) []string {
	narrow := width < presentationNarrowPaneThreshold
	var rows []string

	headerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(presentationColorSubtle))
	if narrow {
		rows = append(rows, headerStyle.Render(fmt.Sprintf("turn %d", turn.Number)))
	} else {
		rows = append(rows, headerStyle.Render(turn.HeaderText(time.Now())))
	}

	toolStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(presentationColorSubtle))
	resultOKStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(presentationColorMuted))
	resultErrStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(presentationColorLove))
	systemStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(presentationColorMuted))
	permStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(presentationColorRose))
	proseStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(presentationColorText))
	statusStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(presentationColorGold))
	thinkingStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(presentationColorMuted))
	narrowRuleStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(presentationColorMuted))

	for _, row := range turn.Rows {
		switch row.Kind {
		case RowTool:
			rows = append(rows, toolStyle.Render(row.Text))
		case RowResult:
			if row.IsError {
				rows = append(rows, resultErrStyle.Render(row.Text))
			} else {
				rows = append(rows, resultOKStyle.Render(row.Text))
			}
		case RowSystem:
			rows = append(rows, systemStyle.Render(row.Text))
		case RowPermission:
			rows = append(rows, permStyle.Render(row.Text))
		case RowResponse:
			if narrow {
				rows = append(rows, narrowRuleStyle.Render(strings.Repeat("─", width)))
			} else {
				rows = append(rows, renderPresentationResponseDivider(width))
			}
		case RowProse:
			rows = append(rows, proseStyle.Render(row.Text))
		case RowStatus:
			rows = append(rows, statusStyle.Render(row.Text))
		case RowThinking:
			rows = append(rows, thinkingStyle.Render(row.Text))
		}
	}
	return rows
}

func renderPresentationResponseDivider(width int) string {
	const label = " response "
	labelLen := len([]rune(label))
	ruleStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(presentationColorMuted))
	labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(presentationColorSubtle))

	if width <= labelLen+4 {
		return labelStyle.Render(label)
	}

	remaining := width - labelLen
	left := remaining / 2
	right := remaining - left
	return ruleStyle.Render(strings.Repeat("─", left)) +
		labelStyle.Render(label) +
		ruleStyle.Render(strings.Repeat("─", right))
}

func renderPresentationComposerFooter(width int) []string {
	ruleStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(presentationColorMuted))
	textStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(presentationColorMuted))
	hintStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(presentationColorSubtle))

	rule := ""
	if width > 0 {
		rule = ruleStyle.Render(strings.Repeat("─", width))
	}
	prompt := textStyle.Render("> send a message to the agent …")
	hints := hintStyle.Render("enter send   shift+enter newline   esc unfocus")
	return []string{rule, prompt, hints}
}
