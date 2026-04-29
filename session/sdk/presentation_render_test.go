package sdk

import (
	"strings"
	"testing"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/stretchr/testify/require"
)

var defaultPresentationPaletteForTest = DefaultPresentationPalette()

var (
	presentationColorMuted  = defaultPresentationPaletteForTest.Muted
	presentationColorSubtle = defaultPresentationPaletteForTest.Subtle
	presentationColorText   = defaultPresentationPaletteForTest.Text
	presentationColorFoam   = defaultPresentationPaletteForTest.Foam
	presentationColorLove   = defaultPresentationPaletteForTest.Love
	presentationColorGold   = defaultPresentationPaletteForTest.Gold
	presentationColorRose   = defaultPresentationPaletteForTest.Rose
	presentationColorPine   = defaultPresentationPaletteForTest.Pine
	presentationColorIris   = defaultPresentationPaletteForTest.Iris
)

// stripANSI is a naive ANSI escape sequence stripper for testing.
// It removes sequences of the form ESC[...m.
func stripANSI(s string) string {
	var out strings.Builder
	i := 0
	for i < len(s) {
		if s[i] == '\x1b' && i+1 < len(s) && s[i+1] == '[' {
			// Skip until 'm'.
			i += 2
			for i < len(s) && s[i] != 'm' {
				i++
			}
			i++ // consume 'm'
			continue
		}
		out.WriteByte(s[i])
		i++
	}
	return out.String()
}

// intPtr returns a pointer to the given int (helper for ToolDiffLine line numbers).
func intPtr(n int) *int { return &n }

func localRenderTime(hour, minute int) time.Time {
	return time.Date(2026, time.April, 22, hour, minute, 0, 0, time.Local)
}

func presentationMarkdownStylesForTest() MarkdownLineStyles {
	proseStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(presentationColorText))
	return MarkdownLineStyles{
		Base:         proseStyle,
		Bold:         proseStyle.Bold(true),
		Italic:       proseStyle.Italic(true),
		Code:         lipgloss.NewStyle().Foreground(lipgloss.Color(presentationColorFoam)),
		Heading:      lipgloss.NewStyle().Foreground(lipgloss.Color(presentationColorGold)).Bold(true),
		BulletPrefix: lipgloss.NewStyle().Foreground(lipgloss.Color(presentationColorRose)),
		NumberPrefix: lipgloss.NewStyle().Foreground(lipgloss.Color(presentationColorFoam)),
		QuotePrefix:  lipgloss.NewStyle().Foreground(lipgloss.Color(presentationColorMuted)),
	}
}

func TestRenderPresentationWithPalette_UsesCustomPalette(t *testing.T) {
	failCode := 9
	palette := PresentationPalette{
		Base:    "#101010",
		Overlay: "#202020",
		Muted:   "#111111",
		Subtle:  "#222222",
		Text:    "#333333",
		Love:    "#444444",
		Gold:    "#555555",
		Rose:    "#666666",
		Pine:    "#777777",
		Foam:    "#888888",
		Iris:    "#999999",
	}
	turn := &PresentationTurn{
		ID:     "t1",
		Number: 1,
		Rows: []PresentationRow{
			{Kind: RowResponse},
			{Kind: RowProse, Text: "assistant prose"},
			{Kind: RowToolPreview, ToolPreview: &ToolPreviewPayload{Lines: []string{"tool output"}}},
			{Kind: RowWarning, Text: "[warning: slow]"},
			{Kind: RowResult, IsError: true, Text: "plain error"},
			{Kind: RowResult, ExitCode: &failCode, Output: "exit error"},
			{Kind: RowStatus, Text: "working"},
		},
	}

	result := RenderPresentationWithPalette([]*PresentationTurn{turn}, 80, palette)

	require.Contains(t, result, lipgloss.NewStyle().Foreground(lipgloss.Color(palette.Muted)).Render(strings.Repeat("─", 80)))
	require.Contains(t, result, lipgloss.NewStyle().Foreground(lipgloss.Color(palette.Text)).Render("assistant prose"))
	require.Contains(t, result, lipgloss.NewStyle().Foreground(lipgloss.Color(palette.Subtle)).Render("tool output"))
	require.Contains(t, result, lipgloss.NewStyle().Foreground(lipgloss.Color(palette.Gold)).Render("[warning: slow]"))
	require.Contains(t, result, lipgloss.NewStyle().Foreground(lipgloss.Color(palette.Love)).Render("plain error"))
	require.Contains(t, result, lipgloss.NewStyle().Foreground(lipgloss.Color(palette.Love)).Render("✗ 9"))
	require.Contains(t, result, lipgloss.NewStyle().Foreground(lipgloss.Color(palette.Gold)).Render("working"))
}

func TestRenderPresentation_UserPrefix(t *testing.T) {
	now := localRenderTime(9, 41)
	turn := &PresentationTurn{
		ID:          "t1",
		Number:      1,
		StartedAt:   now,
		CompletedAt: now,
		Rows: []PresentationRow{
			{Kind: RowUser, Text: "show logs", Timestamp: now},
		},
	}

	result := RenderPresentation([]*PresentationTurn{turn}, 24)
	plain := stripANSI(result)
	expectedLine := "> show logs" + strings.Repeat(" ", 24-len("> show logs")-len("09:41")) + "09:41"

	require.Contains(t, plain, expectedLine)
	require.NotContains(t, plain, "you: show logs")
	require.Contains(t, result, lipgloss.NewStyle().Foreground(lipgloss.Color(presentationColorIris)).Render("show logs"))
	require.Contains(t, result, lipgloss.NewStyle().Foreground(lipgloss.Color(presentationColorSubtle)).Render("09:41"))
}

func TestRenderPresentation_ProseRowsShowRightAlignedTimestamp(t *testing.T) {
	now := localRenderTime(9, 42)
	turn := &PresentationTurn{
		ID:          "t1",
		Number:      1,
		StartedAt:   now,
		CompletedAt: now,
		Rows: []PresentationRow{
			{Kind: RowResponse, Timestamp: now},
			{Kind: RowProse, Text: "assistant text", Timestamp: now},
		},
	}

	result := RenderPresentation([]*PresentationTurn{turn}, 24)
	plain := stripANSI(result)
	expectedLine := "assistant text" + strings.Repeat(" ", 24-len("assistant text")-len("09:42")) + "09:42"

	require.Contains(t, plain, expectedLine)
	require.Contains(t, result, lipgloss.NewStyle().Foreground(lipgloss.Color(presentationColorSubtle)).Render("09:42"))
	require.Equal(t, 1, strings.Count(plain, "09:42"))
}

func TestRenderPresentation_ProseBlockOnlyTimestampsFirstRow(t *testing.T) {
	now := localRenderTime(9, 43)
	turn := &PresentationTurn{
		ID:          "t1",
		Number:      1,
		StartedAt:   now,
		CompletedAt: now,
		Rows: []PresentationRow{
			{Kind: RowResponse, Timestamp: now},
			{Kind: RowProse, Text: "first response line", Timestamp: now},
			{Kind: RowProse, Text: "second response line", Timestamp: now},
		},
	}

	result := RenderPresentation([]*PresentationTurn{turn}, 40)
	plain := stripANSI(result)
	expectedLine := "first response line" + strings.Repeat(" ", 40-len("first response line")-len("09:43")) + "09:43"

	require.Contains(t, plain, expectedLine)
	require.Contains(t, plain, "second response line")
	require.Equal(t, 1, strings.Count(plain, "09:43"))
}

func TestRenderPresentation_ProseTimestampResetsAfterToolResult(t *testing.T) {
	now := localRenderTime(9, 44)
	turn := &PresentationTurn{
		ID:          "t1",
		Number:      1,
		StartedAt:   now,
		CompletedAt: now,
		Rows: []PresentationRow{
			{Kind: RowProse, Text: "before tool", Timestamp: now},
			{Kind: RowTool, Text: "• Read main.go", ToolName: "Read", Timestamp: now},
			{Kind: RowResult, Text: "→ ok", Timestamp: now},
			{Kind: RowProse, Text: "after tool", Timestamp: now},
		},
	}

	result := RenderPresentation([]*PresentationTurn{turn}, 40)
	plain := stripANSI(result)

	require.Contains(t, plain, "before tool")
	require.Contains(t, plain, "after tool")
	require.Equal(t, 2, strings.Count(plain, "09:44"))
}

func TestRenderPresentation_ProseMarkdownInlineStyles(t *testing.T) {
	now := localRenderTime(9, 45)
	styles := presentationMarkdownStylesForTest()
	turn := &PresentationTurn{
		ID:          "t1",
		Number:      1,
		StartedAt:   now,
		CompletedAt: now,
		Rows: []PresentationRow{
			{Kind: RowProse, Text: "plain **bold** *italic* `code`", Timestamp: now},
		},
	}

	result := RenderPresentation([]*PresentationTurn{turn}, 80)

	require.Contains(t, result, styles.Base.Render("plain "))
	require.Contains(t, result, styles.Bold.Render("bold"))
	require.Contains(t, result, styles.Italic.Render("italic"))
	require.Contains(t, result, styles.Code.Render("code"))
	require.Contains(t, stripANSI(result), "plain bold italic code")
}

func TestRenderPresentation_ProseMarkdownLinePrefixes(t *testing.T) {
	now := localRenderTime(9, 46)
	styles := presentationMarkdownStylesForTest()
	tests := []struct {
		name       string
		text       string
		wantPlain  string
		notPlain   string
		wantStyled []string
	}{
		{
			name:      "heading",
			text:      "# Heading **body**",
			wantPlain: "Heading body",
			notPlain:  "# Heading",
			wantStyled: []string{
				styles.Heading.Render("Heading "),
				styles.Bold.Render("body"),
			},
		},
		{
			name:      "bullet",
			text:      "- item **body**",
			wantPlain: "• item body",
			notPlain:  "- item",
			wantStyled: []string{
				styles.BulletPrefix.Render("• "),
				styles.Base.Render("item "),
				styles.Bold.Render("body"),
			},
		},
		{
			name:      "numbered",
			text:      "7. item _body_",
			wantPlain: "7. item body",
			notPlain:  "7. item _body_",
			wantStyled: []string{
				styles.NumberPrefix.Render("7. "),
				styles.Base.Render("item "),
				styles.Italic.Render("body"),
			},
		},
		{
			name:      "blockquote",
			text:      "> quote `body`",
			wantPlain: "│ quote body",
			notPlain:  "> quote",
			wantStyled: []string{
				styles.QuotePrefix.Render("│ "),
				styles.Base.Render("quote "),
				styles.Code.Render("body"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			turn := &PresentationTurn{
				ID:          "t1",
				Number:      1,
				StartedAt:   now,
				CompletedAt: now,
				Rows: []PresentationRow{
					{Kind: RowProse, Text: tt.text, Timestamp: now},
				},
			}

			result := RenderPresentation([]*PresentationTurn{turn}, 80)
			plain := stripANSI(result)

			require.Contains(t, plain, tt.wantPlain)
			require.NotContains(t, plain, tt.notPlain)
			for _, segment := range tt.wantStyled {
				require.Contains(t, result, segment)
			}
		})
	}
}

func TestRenderPresentation_CodeBlockRowsUseCodeStyleAndNoInlineMarkdown(t *testing.T) {
	now := localRenderTime(9, 47)
	codeText := `fmt.Println("*x*")`
	codeGutterStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(presentationColorMuted))
	codeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(presentationColorFoam))
	turn := &PresentationTurn{
		ID:          "t1",
		Number:      1,
		StartedAt:   now,
		CompletedAt: now,
		Rows: []PresentationRow{
			{Kind: RowCodeBlock, Text: codeText, Timestamp: now},
		},
	}

	result := RenderPresentation([]*PresentationTurn{turn}, 80)
	plain := stripANSI(result)
	expected := ToolCallIndent + codeGutterStyle.Render("│ ") + codeStyle.Render(codeText)

	require.Contains(t, result, expected)
	require.Contains(t, plain, `  │ fmt.Println("*x*")`)
	require.Contains(t, plain, "*x*")
}

func TestRenderPresentation_StyledTimestampAlignmentUsesVisibleWidth(t *testing.T) {
	now := localRenderTime(9, 48)
	turn := &PresentationTurn{
		ID:          "t1",
		Number:      1,
		StartedAt:   now,
		CompletedAt: now,
		Rows: []PresentationRow{
			{Kind: RowProse, Text: "hello **bold**", Timestamp: now},
		},
	}

	result := RenderPresentation([]*PresentationTurn{turn}, 32)
	plain := stripANSI(result)
	var proseLine string
	for _, line := range strings.Split(plain, "\n") {
		if strings.Contains(line, "hello bold") {
			proseLine = line
			break
		}
	}

	require.NotEmpty(t, proseLine)
	require.Equal(t, 32-len("09:48"), strings.Index(proseLine, "09:48"))
	require.Contains(t, result, lipgloss.NewStyle().Foreground(lipgloss.Color(presentationColorSubtle)).Render("09:48"))
}

func TestRenderPresentation_ToolLineHighlightAndIndent(t *testing.T) {
	now := time.Now()
	turn := &PresentationTurn{
		ID:          "t1",
		Number:      1,
		StartedAt:   now,
		CompletedAt: now,
		Rows: []PresentationRow{
			{Kind: RowTool, Text: "• Edit main.go", ToolName: "Edit", Timestamp: now},
			{Kind: RowResult, Text: "→ ok", Timestamp: now},
		},
	}

	result := RenderPresentation([]*PresentationTurn{turn}, 80)

	expectedTool := ToolCallIndent +
		lipgloss.NewStyle().Foreground(lipgloss.Color(presentationColorPine)).Render("• Edit") +
		" " +
		lipgloss.NewStyle().Foreground(lipgloss.Color(presentationColorGold)).Render("main.go")
	require.Contains(t, result, expectedTool, "tool row must indent and highlight head vs args")
	require.Regexp(t, `(?m)^  • Edit main\.go\s+✓$`, stripANSI(result),
		"successful tool marker must be right-aligned on the tool row")

	expectedResult := ToolChildIndent +
		lipgloss.NewStyle().Foreground(lipgloss.Color(presentationColorFoam)).Render("→ ok")
	require.Contains(t, result, expectedResult, "tool result must render as an indented child row")
}

// TestRenderPresentation_DiffRows verifies that RowToolDiff rows produce gutter
// lines with +/- markers for added/removed lines and spaces for context lines.
func TestRenderPresentation_DiffRows(t *testing.T) {
	now := time.Now()
	turn := &PresentationTurn{
		ID:          "t1",
		Number:      1,
		StartedAt:   now,
		CompletedAt: now,
		Rows: []PresentationRow{
			{Kind: RowTool, Text: "• Edit main.go", Timestamp: now},
			{
				Kind: RowToolDiff,
				ToolDiff: &ToolDiffPayload{
					Path: "main.go",
					Lines: []ToolDiffLine{
						{Kind: DiffLineContext, OldNumber: intPtr(10), NewNumber: intPtr(10), OldText: "unchanged"},
						{Kind: DiffLineRemoved, OldNumber: intPtr(11), OldText: "old line"},
						{Kind: DiffLineAdded, NewNumber: intPtr(11), NewText: "new line"},
					},
				},
			},
		},
	}

	result := RenderPresentation([]*PresentationTurn{turn}, 80)
	plain := stripANSI(result)

	require.Contains(t, result,
		ToolChildIndent+
			lipgloss.NewStyle().Foreground(lipgloss.Color(presentationColorSubtle)).Render("│ ")+
			lipgloss.NewStyle().Foreground(lipgloss.Color(presentationColorLove)).Render("11 - old line"),
		"removed diff line must be indented with a subtle gutter and love content")
	require.Contains(t, result,
		ToolChildIndent+
			lipgloss.NewStyle().Foreground(lipgloss.Color(presentationColorSubtle)).Render("│ ")+
			lipgloss.NewStyle().Foreground(lipgloss.Color(presentationColorFoam)).Render("11 + new line"),
		"added diff line must be indented with a subtle gutter and foam content")
	require.Contains(t, plain, ToolChildIndent+"│ 10   unchanged", "context line must be indented beneath the tool row")
}

// TestRenderPresentation_DiffRows_Truncation verifies that the truncation
// indicator is appended when HiddenLineCount > 0.
func TestRenderPresentation_DiffRows_Truncation(t *testing.T) {
	now := time.Now()
	turn := &PresentationTurn{
		ID:          "t1",
		Number:      1,
		StartedAt:   now,
		CompletedAt: now,
		Rows: []PresentationRow{
			{
				Kind: RowToolDiff,
				ToolDiff: &ToolDiffPayload{
					Lines: []ToolDiffLine{
						{Kind: DiffLineAdded, NewNumber: intPtr(1), NewText: "first"},
					},
					Truncated:       true,
					HiddenLineCount: 42,
				},
			},
		},
	}

	result := RenderPresentation([]*PresentationTurn{turn}, 80)
	plain := stripANSI(result)

	require.Contains(t, plain, "+42 more lines", "truncation indicator must appear")
}

// TestRenderPresentation_PreviewRows verifies that RowToolPreview rows are
// rendered with a │ gutter prefix for each line.
func TestRenderPresentation_PreviewRows(t *testing.T) {
	now := time.Now()
	turn := &PresentationTurn{
		ID:          "t1",
		Number:      1,
		StartedAt:   now,
		CompletedAt: now,
		Rows: []PresentationRow{
			{Kind: RowTool, Text: "• Read main.go", Timestamp: now},
			{Kind: RowResult, Text: "→ ok", Timestamp: now},
			{
				Kind: RowToolPreview,
				ToolPreview: &ToolPreviewPayload{
					Lines: []string{"package main", "func main() {}"},
				},
			},
		},
	}

	result := RenderPresentation([]*PresentationTurn{turn}, 80)
	plain := stripANSI(result)

	require.Contains(t, result,
		ToolChildIndent+
			lipgloss.NewStyle().Foreground(lipgloss.Color(presentationColorSubtle)).Render("│ ")+
			lipgloss.NewStyle().Foreground(lipgloss.Color(presentationColorSubtle)).Render("package main"),
		"preview rows must be indented with a subtle gutter and subtle content")
	require.Contains(t, plain, ToolChildIndent+"│ func main() {}", "preview line must be indented beneath the tool row")
}

func TestRenderPresentation_PreviewRows_CappedAtFiveLines(t *testing.T) {
	now := time.Now()
	turn := &PresentationTurn{
		ID:          "t1",
		Number:      1,
		StartedAt:   now,
		CompletedAt: now,
		Rows: []PresentationRow{
			{
				Kind: RowToolPreview,
				ToolPreview: &ToolPreviewPayload{
					Lines: []string{"line 1", "line 2", "line 3", "line 4", "line 5", "line 6"},
				},
			},
		},
	}

	result := RenderPresentation([]*PresentationTurn{turn}, 80)
	plain := stripANSI(result)

	require.Contains(t, plain, "│ line 5", "the fifth preview line must remain visible")
	require.NotContains(t, plain, "│ line 6", "preview rendering must cap visible lines at five")
	require.Contains(t, plain, "│ … +1 more lines", "render-time truncation must account for hidden preview lines")
}

// TestRenderPresentation_PreviewRows_Truncation verifies that the preview
// truncation indicator is appended.
func TestRenderPresentation_PreviewRows_Truncation(t *testing.T) {
	now := time.Now()
	turn := &PresentationTurn{
		ID:          "t1",
		Number:      1,
		StartedAt:   now,
		CompletedAt: now,
		Rows: []PresentationRow{
			{
				Kind: RowToolPreview,
				ToolPreview: &ToolPreviewPayload{
					Lines:           []string{"line one"},
					Truncated:       true,
					HiddenLineCount: 7,
				},
			},
		},
	}

	result := RenderPresentation([]*PresentationTurn{turn}, 80)
	plain := stripANSI(result)

	require.Contains(t, plain, "│ … +7 more lines", "preview truncation indicator must appear")
}

// TestRenderPresentation_ActivityRow_RunningTurn verifies that a running turn
// with non-nil Activity emits one activity row above the composer footer.
func TestRenderPresentation_ActivityRow_RunningTurn(t *testing.T) {
	now := time.Now()
	startedAt := now.Add(-15 * time.Second)
	turn := &PresentationTurn{
		ID:        "t1",
		Number:    1,
		StartedAt: startedAt,
		// No CompletedAt → Running.
		Activity: &TurnActivity{
			Kind:      "tool",
			Label:     "editing renderer.go",
			StartedAt: startedAt,
		},
	}

	result := RenderPresentation([]*PresentationTurn{turn}, 80)
	plain := stripANSI(result)

	require.Contains(t, plain, "✺", "activity row must include spinner glyph")
	require.Contains(t, plain, "editing renderer.go", "activity row must include tool label")
}

func TestRenderPresentation_ActivityRow_ZeroStartedAt(t *testing.T) {
	turn := &PresentationTurn{
		ID:     "t1",
		Number: 1,
		Activity: &TurnActivity{
			Kind:  "tool",
			Label: "editing renderer.go",
		},
	}

	result := RenderPresentation([]*PresentationTurn{turn}, 80)
	plain := stripANSI(result)

	require.Contains(t, plain, "editing renderer.go")
	require.Contains(t, plain, "00:00", "zero started_at must render as zero elapsed time")
}

func TestRenderPresentation_ActivityRow_SkipsNilTrailingTurn(t *testing.T) {
	now := time.Now()
	startedAt := now.Add(-10 * time.Second)
	turn := &PresentationTurn{
		ID:        "t1",
		Number:    1,
		StartedAt: startedAt,
		Activity: &TurnActivity{
			Kind:      "tool",
			Label:     "editing renderer.go",
			StartedAt: startedAt,
		},
	}

	result := RenderPresentation([]*PresentationTurn{turn, nil}, 80)
	plain := stripANSI(result)

	require.Contains(t, plain, "editing renderer.go", "activity row must use the last non-nil running turn")
}

// TestRenderPresentation_ActivityRow_CompletedTurn verifies that a completed
// turn does not emit an activity row regardless of Activity being set.
func TestRenderPresentation_ActivityRow_CompletedTurn(t *testing.T) {
	now := time.Now()
	turn := &PresentationTurn{
		ID:          "t1",
		Number:      1,
		StartedAt:   now.Add(-5 * time.Second),
		CompletedAt: now, // completed — not running
		Activity: &TurnActivity{
			Kind:      "working",
			Label:     "stale",
			StartedAt: now.Add(-5 * time.Second),
		},
	}

	result := RenderPresentation([]*PresentationTurn{turn}, 80)
	plain := stripANSI(result)

	// The "✺" spinner must NOT appear for a completed turn.
	require.NotContains(t, plain, "✺", "completed turn must not emit activity row")
}

// TestRenderPresentation_ActivityRow_InterruptedTurn verifies that an
// interrupted turn does not emit an activity row.
func TestRenderPresentation_ActivityRow_InterruptedTurn(t *testing.T) {
	now := time.Now()
	turn := &PresentationTurn{
		ID:          "t1",
		Number:      1,
		StartedAt:   now.Add(-5 * time.Second),
		Interrupted: true,
		Activity: &TurnActivity{
			Kind:      "tool",
			Label:     "stale-tool",
			StartedAt: now.Add(-5 * time.Second),
		},
	}

	result := RenderPresentation([]*PresentationTurn{turn}, 80)
	plain := stripANSI(result)

	require.NotContains(t, plain, "✺", "interrupted turn must not emit activity row")
}

// TestRenderPresentation_NarrowMode_ActivityCollapsed verifies that in narrow
// mode (width < 40) the activity label is suppressed to just "✺ MM:SS".
func TestRenderPresentation_NarrowMode_ActivityCollapsed(t *testing.T) {
	now := time.Now()
	startedAt := now.Add(-90 * time.Second)
	turn := &PresentationTurn{
		ID:        "t1",
		Number:    1,
		StartedAt: startedAt,
		Activity: &TurnActivity{
			Kind:      "tool",
			Label:     "editing very long file name that should be hidden",
			StartedAt: startedAt,
		},
	}

	result := RenderPresentation([]*PresentationTurn{turn}, 30) // narrow
	plain := stripANSI(result)

	require.Contains(t, plain, "✺", "narrow activity must show spinner")
	require.NotContains(t, plain, "editing very long file name that should be hidden",
		"narrow activity must suppress label text")
	// Should show clock format.
	require.Contains(t, plain, "01:30", "narrow activity must show MM:SS clock")
}

// TestRenderPresentation_ComposerFooter verifies the composer footer appears.
func TestRenderPresentation_ComposerFooter(t *testing.T) {
	result := RenderPresentation(nil, 80)
	plain := stripANSI(result)
	require.Contains(t, plain, "> send a message to the agent")
	require.Contains(t, plain, "esc stop")
}

func TestRenderPresentation_WarningRowsUseGold(t *testing.T) {
	now := time.Now()
	turn := &PresentationTurn{
		ID:          "t1",
		Number:      1,
		StartedAt:   now,
		CompletedAt: now,
		Rows: []PresentationRow{
			{Kind: RowWarning, Text: "[warning: mcp server startup is slow]", Timestamp: now},
		},
	}

	result := RenderPresentation([]*PresentationTurn{turn}, 80)
	require.Contains(t, result,
		lipgloss.NewStyle().Foreground(lipgloss.Color(presentationColorGold)).Render("[warning: mcp server startup is slow]"))
}

func TestRenderPresentation_SystemRowsUseGold(t *testing.T) {
	now := time.Now()
	turn := &PresentationTurn{
		ID:          "t1",
		Number:      1,
		StartedAt:   now,
		CompletedAt: now,
		Rows: []PresentationRow{
			{Kind: RowSystem, Text: "[system: unknown message received]", Timestamp: now},
		},
	}

	result := RenderPresentation([]*PresentationTurn{turn}, 80)
	require.Contains(t, result,
		lipgloss.NewStyle().Foreground(lipgloss.Color(presentationColorGold)).Render("[system: unknown message received]"))
}

func TestRenderPresentation_SystemRowsWrapTimestamp(t *testing.T) {
	now := time.Now()
	turn := &PresentationTurn{
		ID:          "t1",
		Number:      1,
		StartedAt:   now,
		CompletedAt: now,
		Rows: []PresentationRow{
			{Kind: RowSystem, Text: "[system: transport failed]", Timestamp: now},
		},
	}

	result := RenderPresentation([]*PresentationTurn{turn}, 24)
	require.Contains(t, result,
		RenderTextLineWithTimestamp("[system: transport failed]", now, 24,
			lipgloss.NewStyle().Foreground(lipgloss.Color(presentationColorGold)),
			lipgloss.NewStyle().Foreground(lipgloss.Color(presentationColorSubtle))))
}

func TestRenderPresentation_SystemBlockOnlyTimestampsFirstRow(t *testing.T) {
	now := time.Now()
	turn := &PresentationTurn{
		ID:          "t1",
		Number:      1,
		StartedAt:   now,
		CompletedAt: now,
		Rows: []PresentationRow{
			{Kind: RowSystem, Text: "[system: first line]", Timestamp: now},
			{Kind: RowSystem, Text: "[system: second line]", Timestamp: now},
		},
	}

	result := RenderPresentation([]*PresentationTurn{turn}, 80)
	require.Contains(t, result,
		RenderTextLineWithTimestamp("[system: first line]", now, 80,
			lipgloss.NewStyle().Foreground(lipgloss.Color(presentationColorGold)),
			lipgloss.NewStyle().Foreground(lipgloss.Color(presentationColorSubtle))))
	require.Contains(t, result,
		lipgloss.NewStyle().Foreground(lipgloss.Color(presentationColorGold)).Render("[system: second line]"))
	require.Equal(t, 1, strings.Count(stripANSI(result), now.Local().Format("15:04")))
}

// TestRenderPresentation_NilDiffPayload verifies that a RowToolDiff row with a
// nil ToolDiff is silently skipped without panic.
func TestRenderPresentation_NilDiffPayload(t *testing.T) {
	now := time.Now()
	turn := &PresentationTurn{
		ID:          "t1",
		Number:      1,
		StartedAt:   now,
		CompletedAt: now,
		Rows: []PresentationRow{
			{Kind: RowToolDiff, ToolDiff: nil},
		},
	}
	require.NotPanics(t, func() {
		RenderPresentation([]*PresentationTurn{turn}, 80)
	})
}

// TestRenderPresentation_NilPreviewPayload verifies that a RowToolPreview row
// with a nil ToolPreview is silently skipped without panic.
func TestRenderPresentation_NilPreviewPayload(t *testing.T) {
	now := time.Now()
	turn := &PresentationTurn{
		ID:          "t1",
		Number:      1,
		StartedAt:   now,
		CompletedAt: now,
		Rows: []PresentationRow{
			{Kind: RowToolPreview, ToolPreview: nil},
		},
	}
	require.NotPanics(t, func() {
		RenderPresentation([]*PresentationTurn{turn}, 80)
	})
}

// TestRenderPresentation_CommandExecutionResult_Success verifies that a RowResult
// with ExitCode=0 renders output without duplicating the inline success marker.
func TestRenderPresentation_CommandExecutionResult_Success(t *testing.T) {
	now := time.Now()
	exitCode := 0
	turn := &PresentationTurn{
		ID:          "t1",
		Number:      1,
		StartedAt:   now,
		CompletedAt: now,
		Rows: []PresentationRow{
			{Kind: RowTool, Text: "• go test ./...", ToolName: "commandExecution"},
			{
				Kind:     RowResult,
				Text:     "tests passed",
				ExitCode: &exitCode,
				Output:   "tests passed",
				ToolName: "commandExecution",
			},
		},
	}

	result := RenderPresentation([]*PresentationTurn{turn}, 80)

	expectedMarker := ToolChildIndent +
		lipgloss.NewStyle().Foreground(lipgloss.Color(presentationColorFoam)).Render("tests passed")
	require.Contains(t, result, expectedMarker, "success exit code must render Foam output")
	require.NotRegexp(t, `(?m)^    ✓$`, stripANSI(result),
		"success marker must not be duplicated on a child row")
	require.NotContains(t, stripANSI(result), "exit_code", "exit_code= prefix must not appear in rendered output")
	require.NotContains(t, stripANSI(result), "output=", "output= prefix must not appear in rendered output")
}

// TestRenderPresentation_CommandExecutionResult_Failure verifies that a RowResult
// with non-zero ExitCode renders as a Love-styled ✗ N glyph + Love-styled output.
func TestRenderPresentation_CommandExecutionResult_Failure(t *testing.T) {
	now := time.Now()
	exitCode := 2
	turn := &PresentationTurn{
		ID:          "t1",
		Number:      1,
		StartedAt:   now,
		CompletedAt: now,
		Rows: []PresentationRow{
			{
				Kind:     RowResult,
				Text:     "✗ exit=2: build failed",
				ExitCode: &exitCode,
				Output:   "build failed",
				IsError:  true,
				ToolName: "commandExecution",
			},
		},
	}

	result := RenderPresentation([]*PresentationTurn{turn}, 80)

	expectedMarker := ToolChildIndent +
		lipgloss.NewStyle().Foreground(lipgloss.Color(presentationColorLove)).Render("✗ 2") +
		" " +
		lipgloss.NewStyle().Foreground(lipgloss.Color(presentationColorLove)).Render("build failed")
	require.Contains(t, result, expectedMarker, "failure exit code must render Love ✗ N + Love output")
}

// TestRenderPresentation_CommandExecutionResult_SuccessNoOutput verifies that a
// RowResult with ExitCode=0 and no output renders just the Pine ✓ glyph.
func TestRenderPresentation_CommandExecutionResult_SuccessNoOutput(t *testing.T) {
	now := time.Now()
	exitCode := 0
	turn := &PresentationTurn{
		ID:          "t1",
		Number:      1,
		StartedAt:   now,
		CompletedAt: now,
		Rows: []PresentationRow{
			{Kind: RowTool, Text: "• true", ToolName: "commandExecution"},
			{
				Kind:     RowResult,
				Text:     "✓",
				ExitCode: &exitCode,
				Output:   "",
				ToolName: "commandExecution",
			},
		},
	}

	result := RenderPresentation([]*PresentationTurn{turn}, 80)

	require.NotRegexp(t, `(?m)^    ✓$`, stripANSI(result),
		"success with no output must be represented by the inline tool marker only")
}

func TestRenderPresentation_CommandExecutionResult_NormalizesMultilineOutput(t *testing.T) {
	now := time.Now()
	exitCode := 0
	turn := &PresentationTurn{
		ID:          "t1",
		Number:      1,
		StartedAt:   now,
		CompletedAt: now,
		Rows: []PresentationRow{
			{Kind: RowTool, Text: "• go test ./...", ToolName: "commandExecution"},
			{
				Kind:     RowResult,
				Text:     "→ tests passed with warnings",
				ExitCode: &exitCode,
				Output:   "tests passed\n\nwith warnings",
				ToolName: "commandExecution",
			},
		},
	}

	result := RenderPresentation([]*PresentationTurn{turn}, 80)

	expectedMarker := ToolChildIndent +
		lipgloss.NewStyle().Foreground(lipgloss.Color(presentationColorFoam)).Render("tests passed with warnings")
	require.Contains(t, result, expectedMarker)
	require.NotContains(t, stripANSI(result), "\n\nwith warnings")
}
