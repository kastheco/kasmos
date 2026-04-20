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

	// Append a single activity row immediately above the composer footer when
	// the last turn is still running and has derived activity information.
	var activityLine string
	if len(turns) > 0 {
		last := turns[len(turns)-1]
		if last.Running() && last.Activity != nil {
			activityStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(presentationColorMuted))
			activityLine = activityStyle.Render(FormatActivityLabel(last.Activity, time.Now(), narrow))
		}
	}

	if sb.Len() > 0 {
		sb.WriteString(sep)
	}
	if activityLine != "" {
		sb.WriteString(activityLine)
		sb.WriteString("\n")
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
	userStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9ccfd8"))
	resultOKStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(presentationColorMuted))
	resultErrStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(presentationColorLove))
	systemStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(presentationColorMuted))
	permStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(presentationColorRose))
	proseStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(presentationColorText))
	statusStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(presentationColorGold))
	thinkingStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(presentationColorMuted))
	narrowRuleStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(presentationColorMuted))
	gutterStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(presentationColorMuted))

	for _, row := range turn.Rows {
		switch row.Kind {
		case RowUser:
			rows = append(rows, userStyle.Render("you: "+row.Text))
		case RowTool:
			rows = append(rows, toolStyle.Render(row.Text))
		case RowToolDiff:
			if row.ToolDiff != nil {
				diffRows := BuildToolDiffBlock(row.ToolDiff, width)
				for i, dl := range row.ToolDiff.Lines {
					if i >= len(diffRows) {
						break
					}
					switch dl.Kind {
					case DiffLineAdded:
						rows = append(rows, lipgloss.NewStyle().Foreground(lipgloss.Color(presentationColorRose)).Render(diffRows[i]))
					case DiffLineRemoved:
						rows = append(rows, resultErrStyle.Render(diffRows[i]))
					default:
						rows = append(rows, gutterStyle.Render(diffRows[i]))
					}
				}
				// Truncation indicator row, if present.
				if len(diffRows) > len(row.ToolDiff.Lines) {
					rows = append(rows, gutterStyle.Render(diffRows[len(row.ToolDiff.Lines)]))
				}
			}
		case RowToolPreview:
			if row.ToolPreview != nil {
				for _, pr := range BuildToolPreviewBlock(row.ToolPreview, width) {
					rows = append(rows, gutterStyle.Render(pr))
				}
			}
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
	ruleStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(presentationColorMuted))
	if width <= 0 {
		return ""
	}
	return ruleStyle.Render(strings.Repeat("─", width))
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
