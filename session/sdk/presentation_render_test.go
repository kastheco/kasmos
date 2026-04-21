package sdk

import (
	"strings"
	"testing"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/stretchr/testify/require"
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

func TestRenderPresentation_UserPrefix(t *testing.T) {
	now := time.Now()
	turn := &PresentationTurn{
		ID:          "t1",
		Number:      1,
		StartedAt:   now,
		CompletedAt: now,
		Rows: []PresentationRow{
			{Kind: RowUser, Text: "show logs", Timestamp: now},
		},
	}

	result := RenderPresentation([]*PresentationTurn{turn}, 80)
	plain := stripANSI(result)

	require.Contains(t, plain, "> show logs")
	require.NotContains(t, plain, "you: show logs")
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
			lipgloss.NewStyle().Foreground(lipgloss.Color(presentationColorGold)).Render("package main"),
		"preview rows must be indented with a subtle gutter and gold content")
	require.Contains(t, plain, ToolChildIndent+"│ func main() {}", "preview line must be indented beneath the tool row")
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
