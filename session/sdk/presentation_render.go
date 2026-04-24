package sdk

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
)

const (
	// Match ui/theme.go so daemon-served SDK previews look the same as local ones.
	presentationColorMuted  = "#6e6a86"
	presentationColorSubtle = "#908caa"
	presentationColorText   = "#e0def4"
	presentationColorFoam   = "#9ccfd8"
	presentationColorLove   = "#eb6f92"
	presentationColorGold   = "#f6c177"
	presentationColorRose   = "#ea9a97"
	presentationColorPine   = "#3e8fb0"

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
	// the most recent non-nil turn is still running and has derived activity
	// information.
	var activityLine string
	for i := len(turns) - 1; i >= 0; i-- {
		turn := turns[i]
		if turn == nil {
			continue
		}
		if turn.Running() && turn.Activity != nil {
			activityStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(presentationColorMuted))
			activityLine = activityStyle.Render(FormatActivityLabel(turn.Activity, time.Now(), narrow))
			break
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
	if turn == nil {
		return nil
	}
	narrow := width < presentationNarrowPaneThreshold
	var rows []string

	headerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(presentationColorSubtle))
	if narrow {
		rows = append(rows, headerStyle.Render(fmt.Sprintf("turn %d", turn.Number)))
	} else {
		rows = append(rows, headerStyle.Render(turn.HeaderText(time.Now())))
	}

	toolStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(presentationColorPine))
	toolArgStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(presentationColorGold))
	userPrefixStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(presentationColorRose))
	userTextStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(presentationColorFoam))
	resultOKStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(presentationColorFoam))
	resultErrStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(presentationColorLove))
	systemStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(presentationColorGold))
	warningStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(presentationColorGold))
	permStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(presentationColorRose))
	proseStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(presentationColorText))
	timestampStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(presentationColorSubtle))
	statusStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(presentationColorGold))
	thinkingStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(presentationColorMuted))
	narrowRuleStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(presentationColorMuted))
	gutterStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(presentationColorSubtle))
	addedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(presentationColorFoam))
	removedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(presentationColorLove))
	diffContextStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(presentationColorSubtle))
	previewStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(presentationColorSubtle))
	codeGutterStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(presentationColorMuted))
	codeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(presentationColorFoam))
	mdStyles := MarkdownLineStyles{
		Base:         proseStyle,
		Bold:         proseStyle.Bold(true),
		Italic:       proseStyle.Italic(true),
		Code:         lipgloss.NewStyle().Foreground(lipgloss.Color(presentationColorFoam)),
		Heading:      lipgloss.NewStyle().Foreground(lipgloss.Color(presentationColorGold)).Bold(true),
		BulletPrefix: lipgloss.NewStyle().Foreground(lipgloss.Color(presentationColorRose)),
		NumberPrefix: lipgloss.NewStyle().Foreground(lipgloss.Color(presentationColorFoam)),
		QuotePrefix:  lipgloss.NewStyle().Foreground(lipgloss.Color(presentationColorMuted)),
	}

	prevKind := PresentationRowKind("")
	for i, row := range turn.Rows {
		switch row.Kind {
		case RowUser:
			rows = append(rows, RenderPromptLineWithTimestamp(">", row.Text, row.Timestamp, width, userPrefixStyle, userTextStyle, timestampStyle))
		case RowTool:
			head, args := SplitToolCallText(row.Text, row.ToolName)
			line := RenderToolCallLineWithStatus(
				head,
				args,
				ToolCallSuccessMarker(turn.Rows, i),
				width-lipgloss.Width(ToolCallIndent),
				toolStyle,
				toolArgStyle,
				toolStyle,
			)
			if line != "" {
				rows = append(rows, ToolCallIndent+line)
			}
		case RowToolDiff:
			if row.ToolDiff != nil {
				diffRows := BuildToolDiffBlock(row.ToolDiff, width)
				for i, dl := range row.ToolDiff.Lines {
					if i >= len(diffRows) {
						break
					}
					switch dl.Kind {
					case DiffLineAdded:
						rows = append(rows, ToolChildIndent+RenderStructuredChildLine(diffRows[i], gutterStyle, addedStyle))
					case DiffLineRemoved:
						rows = append(rows, ToolChildIndent+RenderStructuredChildLine(diffRows[i], gutterStyle, removedStyle))
					default:
						rows = append(rows, ToolChildIndent+RenderStructuredChildLine(diffRows[i], gutterStyle, diffContextStyle))
					}
				}
				// Truncation indicator row, if present.
				if len(diffRows) > len(row.ToolDiff.Lines) {
					rows = append(rows, ToolChildIndent+RenderStructuredChildLine(diffRows[len(row.ToolDiff.Lines)], gutterStyle, diffContextStyle))
				}
			}
		case RowToolPreview:
			if row.ToolPreview != nil {
				for _, pr := range BuildToolPreviewBlock(row.ToolPreview, width) {
					rows = append(rows, ToolChildIndent+RenderStructuredChildLine(pr, gutterStyle, previewStyle))
				}
			}
		case RowResult:
			if SuppressInlineSuccessResult(turn.Rows, i) {
				break
			}
			if row.ExitCode != nil {
				// Structured commandExecution result: render coloured glyph + output.
				var markerStyle, outputStyle lipgloss.Style
				var glyph, codeSegment string
				output := normalizeCommandResultOutput(row.Output)
				if *row.ExitCode == 0 {
					outputStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(presentationColorFoam))
				} else {
					markerStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(presentationColorLove))
					outputStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(presentationColorLove))
					glyph = "✗"
					codeSegment = " " + strconv.Itoa(*row.ExitCode)
				}
				if *row.ExitCode == 0 {
					if output != "" {
						rows = append(rows, ToolChildIndent+outputStyle.Render(output))
					}
				} else {
					rendered := ToolChildIndent + markerStyle.Render(glyph+codeSegment)
					if output != "" {
						rendered += " " + outputStyle.Render(output)
					}
					rows = append(rows, rendered)
				}
			} else if row.IsError {
				rows = append(rows, ToolChildIndent+resultErrStyle.Render(row.Text))
			} else {
				rows = append(rows, ToolChildIndent+resultOKStyle.Render(row.Text))
			}
		case RowWarning:
			rows = append(rows, warningStyle.Render(row.Text))
		case RowSystem:
			if prevKind == RowSystem {
				rows = append(rows, systemStyle.Render(row.Text))
			} else {
				rows = append(rows, RenderTextLineWithTimestamp(row.Text, row.Timestamp, width, systemStyle, timestampStyle))
			}
		case RowPermission:
			rows = append(rows, permStyle.Render(row.Text))
		case RowResponse:
			if narrow {
				rows = append(rows, narrowRuleStyle.Render(strings.Repeat("─", width)))
			} else {
				rows = append(rows, renderPresentationResponseDivider(width))
			}
		case RowProse:
			base := RenderMarkdownProseLine(row.Text, mdStyles)
			if presentationResponseTextKind(prevKind) {
				rows = append(rows, base)
			} else {
				rows = append(rows, RenderStyledLineWithTimestamp(base, row.Timestamp, width, timestampStyle))
			}
		case RowCodeBlock:
			base := ToolCallIndent + codeGutterStyle.Render("│ ") + codeStyle.Render(row.Text)
			if presentationResponseTextKind(prevKind) {
				rows = append(rows, base)
			} else {
				rows = append(rows, RenderStyledLineWithTimestamp(base, row.Timestamp, width, timestampStyle))
			}
		case RowStatus:
			rows = append(rows, statusStyle.Render(row.Text))
		case RowThinking:
			rows = append(rows, thinkingStyle.Render(row.Text))
		}
		prevKind = row.Kind
	}
	return rows
}

func presentationResponseTextKind(kind PresentationRowKind) bool {
	return kind == RowProse || kind == RowCodeBlock
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
	promptPrefixStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(presentationColorRose))
	textStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(presentationColorSubtle))
	hintStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(presentationColorSubtle))
	enterStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(presentationColorFoam))
	newlineStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(presentationColorGold))
	escapeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(presentationColorRose))

	rule := ""
	if width > 0 {
		rule = ruleStyle.Render(strings.Repeat("─", width))
	}
	prompt := RenderPromptLine(">", "send a message to the agent …", promptPrefixStyle, textStyle)
	hints := enterStyle.Render("enter") +
		hintStyle.Render(" send   ") +
		newlineStyle.Render("shift+enter") +
		hintStyle.Render(" newline   ") +
		escapeStyle.Render("esc") +
		hintStyle.Render(" stop")
	return []string{rule, prompt, hints}
}
