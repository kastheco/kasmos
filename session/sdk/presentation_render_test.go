package sdk

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stripANSI removes ANSI escape sequences from s, returning plain text.
func stripANSI(s string) string {
	return ansi.Strip(s)
}

// makePresentationTurn returns a minimal PresentationTurn with the given rows.
func makePresentationTurn(rows ...PresentationRow) *PresentationTurn {
	return &PresentationTurn{
		Number: 1,
		Rows:   rows,
	}
}

// renderTurnLines calls renderPresentationTurn and returns joined output.
func renderTurnLines(turn *PresentationTurn) []string {
	return renderPresentationTurn(turn, 80)
}

// plainLines strips ANSI from each line.
func plainLines(lines []string) []string {
	out := make([]string, len(lines))
	for i, l := range lines {
		out[i] = stripANSI(l)
	}
	return out
}

// TestRenderPresentation_UserPrefix verifies the user-row prefix is "> " not "you: ".
func TestRenderPresentation_UserPrefix(t *testing.T) {
	turn := makePresentationTurn(PresentationRow{Kind: RowUser, Text: "show logs"})
	lines := plainLines(renderTurnLines(turn))
	result := strings.Join(lines, "\n")
	assert.Contains(t, result, "> show logs")
	assert.NotContains(t, result, "you: show logs")
}

// TestRenderPresentation_ToolHighlight verifies that a tool row is rendered as
// ToolCallIndent + subtle("• Edit") + " " + text("main.go") when ToolName is set.
func TestRenderPresentation_ToolHighlight(t *testing.T) {
	toolStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(presentationColorSubtle))
	toolArgStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(presentationColorText))
	expected := ToolCallIndent + toolStyle.Render("• Edit") + " " + toolArgStyle.Render("main.go")

	turn := makePresentationTurn(PresentationRow{
		Kind:     RowTool,
		Text:     "• Edit main.go",
		ToolName: "Edit",
	})
	lines := renderTurnLines(turn)
	result := strings.Join(lines, "\n")
	assert.Contains(t, result, expected)
}

// TestRenderPresentation_ToolIndent verifies that all tool rows are indented with ToolCallIndent.
func TestRenderPresentation_ToolIndent(t *testing.T) {
	turn := makePresentationTurn(
		PresentationRow{Kind: RowTool, Text: "• Read src/main.go", ToolName: "Read"},
		PresentationRow{Kind: RowTool, Text: "• Grep pattern", ToolName: "Grep"},
	)
	lines := renderTurnLines(turn)
	var toolLines []string
	for _, l := range lines {
		plain := stripANSI(l)
		if strings.Contains(plain, "• Read") || strings.Contains(plain, "• Grep") {
			toolLines = append(toolLines, l)
		}
	}
	require.Len(t, toolLines, 2, "two tool rows must appear")
	for _, l := range toolLines {
		assert.True(t, strings.HasPrefix(l, ToolCallIndent),
			"tool row %q must start with ToolCallIndent %q", l, ToolCallIndent)
	}
}

// TestRenderPresentation_ResultIndent verifies that ok and error result rows are
// indented with ToolChildIndent.
func TestRenderPresentation_ResultIndent(t *testing.T) {
	turn := makePresentationTurn(
		PresentationRow{Kind: RowResult, Text: "→ 5 lines", IsError: false},
		PresentationRow{Kind: RowResult, Text: "✗ permission denied", IsError: true},
	)
	lines := renderTurnLines(turn)
	var resultLines []string
	for _, l := range lines {
		plain := stripANSI(l)
		if strings.Contains(plain, "→ 5 lines") || strings.Contains(plain, "✗ permission denied") {
			resultLines = append(resultLines, l)
		}
	}
	require.Len(t, resultLines, 2, "two result rows must appear")
	for _, l := range resultLines {
		assert.True(t, strings.HasPrefix(l, ToolChildIndent),
			"result row %q must start with ToolChildIndent %q", l, ToolChildIndent)
	}
}

// TestRenderPresentation_PreviewRows verifies that preview content rows are
// indented with ToolChildIndent and carry the │ gutter.
func TestRenderPresentation_PreviewRows(t *testing.T) {
	turn := makePresentationTurn(PresentationRow{
		Kind: RowToolPreview,
		ToolPreview: &ToolPreviewPayload{
			Rows: []string{"package main", "func main() {}"},
		},
	})
	lines := renderTurnLines(turn)
	var previewLines []string
	for _, l := range lines {
		plain := stripANSI(l)
		if strings.Contains(plain, "package main") || strings.Contains(plain, "func main") {
			previewLines = append(previewLines, l)
		}
	}
	require.Len(t, previewLines, 2, "two preview rows must appear")
	for _, l := range previewLines {
		plain := stripANSI(l)
		assert.True(t, strings.HasPrefix(plain, ToolChildIndent+"│ "),
			"preview row %q must start with ToolChildIndent+gutter %q", plain, ToolChildIndent+"│ ")
	}
}

// TestRenderPresentation_PreviewRows_Truncation verifies that the truncation
// sentinel row is indented with ToolChildIndent and carries the │ gutter.
func TestRenderPresentation_PreviewRows_Truncation(t *testing.T) {
	turn := makePresentationTurn(PresentationRow{
		Kind: RowToolPreview,
		ToolPreview: &ToolPreviewPayload{
			Rows:      []string{"line one"},
			Truncated: true,
		},
	})
	lines := renderTurnLines(turn)
	var truncLine string
	for _, l := range lines {
		if strings.Contains(stripANSI(l), "truncated") {
			truncLine = l
			break
		}
	}
	require.NotEmpty(t, truncLine, "truncation row must be present in preview output")
	plain := stripANSI(truncLine)
	assert.True(t, strings.HasPrefix(plain, ToolChildIndent+"│ "),
		"preview truncation row %q must start with ToolChildIndent+gutter", plain)
}

// TestRenderPresentation_DiffRows verifies that added, removed, and context
// diff lines are each indented with ToolChildIndent and carry the │ gutter.
func TestRenderPresentation_DiffRows(t *testing.T) {
	turn := makePresentationTurn(PresentationRow{
		Kind: RowToolDiff,
		ToolDiff: &ToolDiffPayload{
			Lines: []DiffLine{
				{Kind: DiffLineAdded, Text: "+ new line"},
				{Kind: DiffLineRemoved, Text: "- old line"},
				{Kind: DiffLineContext, Text: "  unchanged"},
			},
		},
	})
	lines := renderTurnLines(turn)
	var diffLines []string
	for _, l := range lines {
		plain := stripANSI(l)
		if strings.Contains(plain, "+ new line") ||
			strings.Contains(plain, "- old line") ||
			strings.Contains(plain, "  unchanged") {
			diffLines = append(diffLines, l)
		}
	}
	require.Len(t, diffLines, 3, "three diff lines must appear")
	for _, l := range diffLines {
		plain := stripANSI(l)
		assert.True(t, strings.HasPrefix(plain, ToolChildIndent+"│ "),
			"diff row %q must start with ToolChildIndent+gutter %q", plain, ToolChildIndent+"│ ")
	}
}

// TestRenderPresentation_DiffRows_Truncation verifies that the truncation
// sentinel at diffRows[len(row.ToolDiff.Lines)] is indented with ToolChildIndent
// and carries the │ gutter.
func TestRenderPresentation_DiffRows_Truncation(t *testing.T) {
	turn := makePresentationTurn(PresentationRow{
		Kind: RowToolDiff,
		ToolDiff: &ToolDiffPayload{
			Lines:     []DiffLine{{Kind: DiffLineContext, Text: "  ctx line"}},
			Truncated: true,
		},
	})
	lines := renderTurnLines(turn)
	var truncLine string
	for _, l := range lines {
		if strings.Contains(stripANSI(l), "truncated") {
			truncLine = l
			break
		}
	}
	require.NotEmpty(t, truncLine, "truncation row must be present in diff output")
	plain := stripANSI(truncLine)
	assert.True(t, strings.HasPrefix(plain, ToolChildIndent+"│ "),
		"diff truncation row %q must start with ToolChildIndent+gutter", plain)
}

// TestRenderPresentation_NilDiffPayload verifies that a RowToolDiff row with a
// nil ToolDiff payload does not panic and produces no extra output.
func TestRenderPresentation_NilDiffPayload(t *testing.T) {
	turn := makePresentationTurn(PresentationRow{Kind: RowToolDiff, ToolDiff: nil})
	// Must not panic.
	lines := plainLines(renderTurnLines(turn))
	// Only the header row should be present; no diff content.
	for _, l := range lines {
		assert.False(t, strings.HasPrefix(l, ToolChildIndent),
			"nil diff must not produce indented rows; got %q", l)
	}
}

// TestRenderPresentation_NilPreviewPayload verifies that a RowToolPreview row
// with a nil ToolPreview payload does not panic and produces no extra output.
func TestRenderPresentation_NilPreviewPayload(t *testing.T) {
	turn := makePresentationTurn(PresentationRow{Kind: RowToolPreview, ToolPreview: nil})
	// Must not panic.
	lines := plainLines(renderTurnLines(turn))
	// Only the header row should be present; no preview content.
	for _, l := range lines {
		assert.False(t, strings.HasPrefix(l, ToolChildIndent),
			"nil preview must not produce indented rows; got %q", l)
	}
}
