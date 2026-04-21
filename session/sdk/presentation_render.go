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

// BuildToolDiffBlock converts a ToolDiffPayload into a flat slice of plain-text
// lines ready for colour-styling. Entry i corresponds to payload.Lines[i].Text.
// When payload.Truncated is true an extra truncation sentinel "… (truncated)" is
// appended at index len(payload.Lines). Returns nil when payload is nil.
func BuildToolDiffBlock(payload *ToolDiffPayload) []string {
	if payload == nil {
		return nil
	}
	rows := make([]string, len(payload.Lines))
	for i, l := range payload.Lines {
		rows[i] = "│ " + l.Text
	}
	if payload.Truncated {
		rows = append(rows, "│ … (truncated)")
	}
	return rows
}

// BuildToolPreviewBlock converts a ToolPreviewPayload into a flat slice of
// plain-text lines. When payload.Truncated is true an extra truncation sentinel
// "… (truncated)" is appended after the last row. Returns nil when payload is nil.
func BuildToolPreviewBlock(payload *ToolPreviewPayload) []string {
	if payload == nil {
		return nil
	}
	rows := make([]string, len(payload.Rows))
	for i, r := range payload.Rows {
		rows[i] = "│ " + r
	}
	if payload.Truncated {
		rows = append(rows, "│ … (truncated)")
	}
	return rows
}

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
	toolArgStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(presentationColorText))
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
	addedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(presentationColorRose))
	removedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(presentationColorLove))
	diffContextStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(presentationColorMuted))

	for _, row := range turn.Rows {
		switch row.Kind {
		case RowUser:
			rows = append(rows, userStyle.Render("> "+row.Text))
		case RowTool:
			head, args := SplitToolCallText(row.Text, row.ToolName)
			line := RenderToolCallLine(head, args, toolStyle, toolArgStyle)
			if line == "" {
				rows = append(rows, line)
			} else {
				rows = append(rows, ToolCallIndent+line)
			}
		case RowResult:
			var styled string
			if row.IsError {
				styled = resultErrStyle.Render(row.Text)
			} else {
				styled = resultOKStyle.Render(row.Text)
			}
			if styled != "" {
				rows = append(rows, ToolChildIndent+styled)
			} else {
				rows = append(rows, styled)
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
		case RowToolPreview:
			if row.ToolPreview == nil {
				break
			}
			previewRows := BuildToolPreviewBlock(row.ToolPreview)
			for _, pr := range previewRows {
				rows = append(rows, ToolChildIndent+gutterStyle.Render(pr))
			}
		case RowToolDiff:
			if row.ToolDiff == nil {
				break
			}
			diffRows := BuildToolDiffBlock(row.ToolDiff)
			for i, dr := range diffRows {
				var styled string
				if row.ToolDiff.Truncated && i == len(row.ToolDiff.Lines) {
					styled = diffContextStyle.Render(dr)
				} else {
					switch row.ToolDiff.Lines[i].Kind {
					case DiffLineAdded:
						styled = addedStyle.Render(dr)
					case DiffLineRemoved:
						styled = removedStyle.Render(dr)
					default:
						styled = diffContextStyle.Render(dr)
					}
				}
				rows = append(rows, ToolChildIndent+styled)
			}
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
