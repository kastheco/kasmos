package ui

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func stripANSI(s string) string {
	var b strings.Builder
	inEsc := false
	for _, r := range s {
		if r == '\x1b' {
			inEsc = true
		}
		if !inEsc {
			b.WriteRune(r)
		}
		if inEsc && r == 'm' {
			inEsc = false
		}
	}
	return b.String()
}

func TestStatusBar_Baseline(t *testing.T) {
	sb := NewStatusBar()
	sb.SetSize(80)
	sb.SetData(StatusBarData{
		Branch: "main",
	})

	result := sb.String()
	assert.Contains(t, result, "main")
	// Should be exactly 1 line (no newlines in output)
	assert.Equal(t, 0, strings.Count(result, "\n"))
}

func TestStatusBar_PlanContext(t *testing.T) {
	sb := NewStatusBar()
	sb.SetSize(120)
	sb.SetData(StatusBarData{
		Branch:     "plan/auth-refactor",
		PlanName:   "auth-refactor",
		PlanStatus: "implementing",
	})

	result := sb.String()
	plain := stripANSI(result)
	assert.Contains(t, plain, "kasmos")
	assert.Contains(t, result, "plan/auth-refactor")
	assert.Contains(t, result, "implementing")
}

func TestStatusBar_StatusLeftAlignedAfterLogo(t *testing.T) {
	sb := NewStatusBar()
	sb.SetSize(120)
	sb.SetData(StatusBarData{
		Branch:     "plan/auth-refactor",
		PlanName:   "auth-refactor",
		PlanStatus: "implementing",
		ProjectDir: "myproject",
	})
	plain := stripANSI(sb.String())

	appIdx := strings.Index(plain, "kasmos")
	statusIdx := strings.Index(plain, "implementing")
	projectIdx := strings.Index(plain, "myproject")
	branchIdx := strings.Index(plain, "plan/auth-refactor")

	require.NotEqual(t, -1, appIdx)
	require.NotEqual(t, -1, statusIdx)
	require.NotEqual(t, -1, projectIdx)
	require.NotEqual(t, -1, branchIdx)
	assert.Greater(t, statusIdx, appIdx, "status must appear after app logo")
	assert.Greater(t, projectIdx, statusIdx, "project dir should be centered away from the left status block")
	assert.Greater(t, branchIdx, projectIdx, "branch should render to the right of the centered project dir")
}

func TestStatusBar_VersionRenderedAfterLogo(t *testing.T) {
	sb := NewStatusBar()
	sb.SetSize(120)
	sb.SetData(StatusBarData{Version: "v1.4.2-abc1234", PlanStatus: "implementing"})
	plain := stripANSI(sb.String())

	appIdx := strings.Index(plain, "kasmos")
	versionIdx := strings.Index(plain, "v1.4.2-abc1234")
	statusIdx := strings.Index(plain, "implementing")

	require.NotEqual(t, -1, appIdx)
	require.NotEqual(t, -1, versionIdx)
	require.NotEqual(t, -1, statusIdx)
	assert.Greater(t, versionIdx, appIdx)
	assert.Greater(t, statusIdx, versionIdx)
}

func TestStatusBar_EmptyVersionSkipped(t *testing.T) {
	sb := NewStatusBar()
	sb.SetSize(80)
	sb.SetData(StatusBarData{})
	assert.NotContains(t, stripANSI(sb.String()), "v1.")
}

func TestStatusBar_ProjectDirCentered(t *testing.T) {
	sb := NewStatusBar()
	sb.SetSize(100)
	sb.SetData(StatusBarData{
		Branch:     "main",
		PlanStatus: "reviewing",
		ProjectDir: "myproject",
	})

	plain := stripANSI(sb.String())

	projectIdx := strings.Index(plain, "myproject")
	require.NotEqual(t, -1, projectIdx)
	projectCenter := projectIdx + len("myproject")/2
	assert.InDelta(t, 50, projectCenter, 6,
		"project dir should be centered in the status bar")
}

func TestStatusBar_WaveGlyphs(t *testing.T) {
	sb := NewStatusBar()
	sb.SetSize(120)
	sb.SetData(StatusBarData{
		Branch:     "plan/auth-refactor",
		PlanName:   "auth-refactor",
		PlanStatus: "implementing",
		WaveLabel:  "wave 2/4",
		TaskGlyphs: []TaskGlyph{
			TaskGlyphComplete,
			TaskGlyphComplete,
			TaskGlyphRunning,
			TaskGlyphFailed,
			TaskGlyphPending,
		},
	})

	result := sb.String()
	assert.Contains(t, result, "wave 2/4")
	// Glyphs should be present (check the raw glyph chars)
	assert.Contains(t, result, "✓")
	assert.Contains(t, result, "●")
	assert.Contains(t, result, "✕")
	assert.Contains(t, result, "○")
}

func TestStatusBar_WaveGlyphsAreSpaced(t *testing.T) {
	sb := NewStatusBar()
	sb.SetSize(120)
	sb.SetData(StatusBarData{
		Branch:    "plan/auth-refactor",
		WaveLabel: "wave 1/4",
		TaskGlyphs: []TaskGlyph{
			TaskGlyphRunning,
			TaskGlyphPending,
			TaskGlyphPending,
		},
	})

	plain := stripANSI(sb.String())
	assert.Contains(t, plain, "● ○ ○",
		"wave task glyphs should have spacing to avoid cramped status bar rendering")
}

func TestStatusBar_WaveGlyphsAppearBeforeWaveLabel(t *testing.T) {
	sb := NewStatusBar()
	sb.SetSize(120)
	sb.SetData(StatusBarData{
		Branch:    "plan/auth-refactor",
		WaveLabel: "wave 1/4",
		TaskGlyphs: []TaskGlyph{
			TaskGlyphRunning,
			TaskGlyphPending,
			TaskGlyphPending,
		},
	})

	plain := stripANSI(sb.String())
	glyphIdx := strings.Index(plain, "● ○ ○")
	labelIdx := strings.Index(plain, "wave 1/4")
	require.NotEqual(t, -1, glyphIdx)
	require.NotEqual(t, -1, labelIdx)
	assert.Less(t, glyphIdx, labelIdx,
		"wave glyphs should appear before the wave label in the status bar")
}

func TestStatusBar_WaveProgressLeftAlignedAfterLogo(t *testing.T) {
	sb := NewStatusBar()
	sb.SetSize(120)
	sb.SetData(StatusBarData{
		Branch:     "plan/auth-refactor",
		ProjectDir: "myproject",
		WaveLabel:  "wave 1/4",
		TaskGlyphs: []TaskGlyph{
			TaskGlyphRunning,
			TaskGlyphPending,
			TaskGlyphPending,
		},
	})

	plain := stripANSI(sb.String())
	appIdx := strings.Index(plain, "kasmos")
	waveIdx := strings.Index(plain, "wave 1/4")
	projectIdx := strings.Index(plain, "myproject")
	branchIdx := strings.Index(plain, "plan/auth-refactor")

	require.NotEqual(t, -1, appIdx)
	require.NotEqual(t, -1, waveIdx)
	require.NotEqual(t, -1, projectIdx)
	require.NotEqual(t, -1, branchIdx)
	assert.Greater(t, waveIdx, appIdx, "wave progress should appear after app logo")
	assert.Greater(t, projectIdx, waveIdx, "project dir should remain centered, not grouped with wave progress")
	assert.Greater(t, branchIdx, projectIdx, "branch should render to the right of the centered project dir")
}

func TestStatusBar_Truncation(t *testing.T) {
	sb := NewStatusBar()
	sb.SetSize(40) // narrow terminal
	sb.SetData(StatusBarData{
		Branch: "feature/extremely-long-branch-name-here",
	})

	result := sb.String()
	// Should not exceed width (lipgloss handles this, but verify no panic)
	require.NotEmpty(t, result)
}

func TestStatusBar_EmptyData(t *testing.T) {
	sb := NewStatusBar()
	sb.SetSize(80)
	sb.SetData(StatusBarData{})

	result := sb.String()
	// App name is gradient-rendered so individual chars are split by ANSI escapes;
	// verify each character is present in order.
	for _, c := range "kasmos" {
		assert.Contains(t, result, string(c))
	}
}

func TestStatusBar_TmuxSessionCountMovedToMenu(t *testing.T) {
	// tmux session count is now rendered in the bottom Menu bar, not the status bar.
	sb := NewStatusBar()
	sb.SetSize(100)
	sb.SetData(StatusBarData{
		Branch:           "main",
		TmuxSessionCount: 3,
	})

	plain := stripANSI(sb.String())
	assert.NotContains(t, plain, "tmux:")
}

func TestStatusBar_BranchRightAligned(t *testing.T) {
	sb := NewStatusBar()
	sb.SetSize(100)
	sb.SetData(StatusBarData{
		Branch:     "feature/center-right",
		ProjectDir: "myproject",
	})

	plain := stripANSI(sb.String())
	assert.Contains(t, plain, "myproject")
	assert.Contains(t, plain, "feature/center-right")

	projectIdx := strings.Index(plain, "myproject")
	branchIdx := strings.Index(plain, "feature/center-right")
	require.NotEqual(t, -1, projectIdx)
	require.NotEqual(t, -1, branchIdx)
	assert.Greater(t, branchIdx, projectIdx,
		"branch should appear to the right of the centered project dir")
}

func TestStatusBar_BranchDroppedWhenNarrow(t *testing.T) {
	sb := NewStatusBar()
	sb.SetSize(40) // very narrow
	sb.SetData(StatusBarData{
		Branch:     "feature/some-long-branch-name",
		ProjectDir: "myproject",
	})

	plain := stripANSI(sb.String())
	require.NotEmpty(t, plain)
	assert.Contains(t, plain, "myproject")
	assert.NotContains(t, plain, "feature/some-long-branch-name",
		"branch should drop before the centered project dir on narrow layouts")
}

func TestStatusBar_FocusModeNoLongerShowsPill(t *testing.T) {
	sb := NewStatusBar()
	sb.SetSize(100)
	sb.SetData(StatusBarData{
		Branch:    "main",
		FocusMode: true,
	})

	result := sb.String()
	assert.NotContains(t, result, "interactive",
		"interactive indicator moved to bottom menu bar")
}

func TestStatusBar_PRIndicator(t *testing.T) {
	sb := NewStatusBar()
	sb.SetSize(120)
	sb.SetData(StatusBarData{
		Branch:     "plan/test",
		ProjectDir: "myproject",
		PRState:    "approved",
		PRChecks:   "passing",
	})
	rendered := sb.String()
	assert.Contains(t, rendered, "✓")
}

func TestStatusBar_PRIndicator_ComposesWithRightAlignedBranch(t *testing.T) {
	sb := NewStatusBar()
	sb.SetSize(120)
	sb.SetData(StatusBarData{
		Branch:     "plan/test",
		ProjectDir: "myproject",
		PRState:    "approved",
		PRChecks:   "passing",
	})

	plain := stripANSI(sb.String())
	projectIdx := strings.Index(plain, "myproject")
	prIdx := strings.Index(plain, "✓ pr")
	branchIdx := strings.Index(plain, "plan/test")

	require.NotEqual(t, -1, projectIdx)
	require.NotEqual(t, -1, prIdx)
	require.NotEqual(t, -1, branchIdx)
	assert.Greater(t, prIdx, projectIdx, "pr indicator should render on the right side beside the branch")
	assert.Greater(t, branchIdx, projectIdx, "branch should remain on the right side of the centered project dir")
	assert.Less(t, prIdx, branchIdx, "pr indicator should appear before the branch text")
}

func TestStatusBar_PRIndicator_ChangesRequested(t *testing.T) {
	sb := NewStatusBar()
	sb.SetSize(120)
	sb.SetData(StatusBarData{
		Branch:     "plan/test",
		ProjectDir: "myproject",
		PRState:    "changes_requested",
		PRChecks:   "passing",
	})
	rendered := sb.String()
	assert.Contains(t, rendered, "●")
}

func TestStatusBar_PRIndicator_Empty(t *testing.T) {
	sb := NewStatusBar()
	sb.SetSize(120)
	sb.SetData(StatusBarData{
		Branch:     "main",
		ProjectDir: "myproject",
	})
	plain := stripANSI(sb.String())
	// None of the PR indicator glyphs should appear when PRState is empty.
	assert.NotContains(t, plain, "✓ pr")
	assert.NotContains(t, plain, "● pr")
	assert.NotContains(t, plain, "✕ pr")
	assert.NotContains(t, plain, "○ pr")
}

func TestStatusBar_PRIndicator_NarrowDrops(t *testing.T) {
	sb := NewStatusBar()
	sb.SetSize(30) // very narrow — PR indicator should drop cleanly
	sb.SetData(StatusBarData{
		Branch:     "plan/test",
		ProjectDir: "myproject",
		PRState:    "approved",
		PRChecks:   "passing",
	})
	result := sb.String()
	// Should not panic and should produce output
	require.NotEmpty(t, result)
	// The output should still contain the app name
	assert.Contains(t, result, "k") // gradient-rendered "kasmos"
}
