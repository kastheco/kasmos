package session

import (
	"context"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/kastheco/kasmos/internal/theme"
	"github.com/kastheco/kasmos/session/sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockPresentationSession implements ExecutionSession + presentationProvider
// for testing the CapturePresentation path on Instance.
type mockPresentationSession struct {
	deadExecutionSession
	turns []*sdk.PresentationTurn
}

func (m *mockPresentationSession) CapturePresentation() []*sdk.PresentationTurn {
	return m.turns
}

type mockLocalImagePromptSession struct {
	deadExecutionSession
	lastPrompt string
	lastImages []string
}

func (m *mockLocalImagePromptSession) SendPromptWithLocalImages(prompt string, imagePaths []string) error {
	m.lastPrompt = prompt
	m.lastImages = append([]string(nil), imagePaths...)
	return nil
}

func TestInstance_CapturePresentation_WithSDKSession(t *testing.T) {
	inst := &Instance{started: true}
	turns := []*sdk.PresentationTurn{
		{ID: "t1", Number: 1},
	}
	inst.SetExecutionSessionForTest(&mockPresentationSession{turns: turns})

	result := inst.CapturePresentation()
	require.Len(t, result, 1)
	assert.Equal(t, "t1", result[0].ID)
}

func TestInstance_CapturePresentation_WithNonSDKSession(t *testing.T) {
	inst := &Instance{started: true}
	inst.SetExecutionSessionForTest(deadExecutionSession{})

	result := inst.CapturePresentation()
	assert.Nil(t, result)
}

func TestInstance_CapturePresentation_NotStarted(t *testing.T) {
	inst := &Instance{}
	result := inst.CapturePresentation()
	assert.Nil(t, result)
}

func TestInstance_CapturePresentation_UsesCachedPresentationForPlaceholder(t *testing.T) {
	inst := &Instance{}
	inst.SetCachedPresentation([]*sdk.PresentationTurn{
		{
			ID:     "t1",
			Number: 1,
			Rows: []sdk.PresentationRow{
				{Kind: sdk.RowResponse},
				{Kind: sdk.RowProse, Text: "cached daemon output"},
			},
		},
	})

	result := inst.CapturePresentation()
	require.Len(t, result, 1)
	assert.Equal(t, "cached daemon output", result[0].Rows[1].Text)

	result[0].Rows[1].Text = "mutated"
	fresh := inst.CapturePresentation()
	require.Len(t, fresh, 1)
	assert.Equal(t, "cached daemon output", fresh[0].Rows[1].Text)
}

func TestInstance_CapturePresentation_NilSession(t *testing.T) {
	inst := &Instance{started: true}
	// executionSession is nil
	result := inst.CapturePresentation()
	assert.Nil(t, result)
}

func TestInstance_Preview_WithSDKPresentation(t *testing.T) {
	inst := &Instance{started: true, ExecutionMode: ExecutionModeSDK}
	turns := []*sdk.PresentationTurn{
		{
			ID:     "t1",
			Number: 1,
			Rows: []sdk.PresentationRow{
				{Kind: sdk.RowTool, Text: "• read_file main.go"},
				{Kind: sdk.RowResult, Text: "→ 42 lines"},
				{Kind: sdk.RowResponse},
				{Kind: sdk.RowProse, Text: "assistant text"},
			},
		},
	}
	inst.SetExecutionSessionForTest(&mockPresentationSession{turns: turns})

	preview, err := inst.Preview()
	require.NoError(t, err)
	assert.NotContains(t, preview, "response")
	assert.Contains(t, preview, "assistant text")
	assert.Contains(t, ansi.Strip(preview), "> send a message to the agent")
}

func TestInstance_Preview_WithSDKPresentationUsesCurrentTheme(t *testing.T) {
	t.Cleanup(func() {
		theme.SetCurrent(theme.DefaultPalette())
	})
	custom := theme.DefaultPalette()
	custom.Text = "#123456"
	theme.SetCurrent(custom)

	inst := &Instance{started: true, ExecutionMode: ExecutionModeSDK}
	turns := []*sdk.PresentationTurn{
		{
			ID:     "t1",
			Number: 1,
			Rows: []sdk.PresentationRow{
				{Kind: sdk.RowResponse},
				{Kind: sdk.RowProse, Text: "themed assistant text"},
			},
		},
	}
	inst.SetExecutionSessionForTest(&mockPresentationSession{turns: turns})

	preview, err := inst.Preview()
	require.NoError(t, err)
	assert.Contains(t, preview, lipgloss.NewStyle().Foreground(lipgloss.Color(string(custom.Text))).Render("themed assistant text"))
}

func TestInstance_PreviewWithPalette_UsesSuppliedPaletteNotGlobal(t *testing.T) {
	t.Cleanup(func() {
		theme.SetCurrent(theme.DefaultPalette())
	})
	// Set a process-global palette that should NOT be used; the explicit
	// palette argument must win so daemons serving multiple repos can render
	// each repo's previews with its own colors.
	globalCustom := theme.DefaultPalette()
	globalCustom.Text = "#aabbcc"
	theme.SetCurrent(globalCustom)

	repoPalette := theme.DefaultPalette()
	repoPalette.Text = "#112233"

	inst := &Instance{started: true, ExecutionMode: ExecutionModeSDK}
	turns := []*sdk.PresentationTurn{
		{
			ID:     "t1",
			Number: 1,
			Rows: []sdk.PresentationRow{
				{Kind: sdk.RowResponse},
				{Kind: sdk.RowProse, Text: "per-repo themed text"},
			},
		},
	}
	inst.SetExecutionSessionForTest(&mockPresentationSession{turns: turns})

	preview, err := inst.PreviewWithPalette(repoPalette)
	require.NoError(t, err)
	assert.Contains(t, preview, lipgloss.NewStyle().Foreground(lipgloss.Color(string(repoPalette.Text))).Render("per-repo themed text"))
	assert.NotContains(t, preview, lipgloss.NewStyle().Foreground(lipgloss.Color(string(globalCustom.Text))).Render("per-repo themed text"))
}

func TestInstance_PreviewRangeWithPalette_UsesSuppliedPaletteForSDKPresentation(t *testing.T) {
	t.Cleanup(func() {
		theme.SetCurrent(theme.DefaultPalette())
	})
	globalCustom := theme.DefaultPalette()
	globalCustom.Text = "#aabbcc"
	theme.SetCurrent(globalCustom)

	repoPalette := theme.DefaultPalette()
	repoPalette.Text = "#112233"

	inst := &Instance{started: true, ExecutionMode: ExecutionModeSDK}
	turns := []*sdk.PresentationTurn{
		{
			ID:     "t1",
			Number: 1,
			Rows: []sdk.PresentationRow{
				{Kind: sdk.RowResponse},
				{Kind: sdk.RowProse, Text: "ranged themed text"},
			},
		},
	}
	inst.SetExecutionSessionForTest(&mockPresentationSession{turns: turns})

	preview, err := inst.PreviewRangeWithPalette("0", "999", repoPalette)
	require.NoError(t, err)
	assert.Contains(t, preview, lipgloss.NewStyle().Foreground(lipgloss.Color(string(repoPalette.Text))).Render("ranged themed text"))
	assert.NotContains(t, preview, lipgloss.NewStyle().Foreground(lipgloss.Color(string(globalCustom.Text))).Render("ranged themed text"))
}

func TestInstance_SendPromptWithLocalImages_DelegatesToExecutionSession(t *testing.T) {
	inst := &Instance{started: true, Program: "codex"}
	mock := &mockLocalImagePromptSession{}
	inst.SetExecutionSessionForTest(mock)

	err := inst.SendPromptWithLocalImages("describe this", []string{"/tmp/screenshot.png"})
	require.NoError(t, err)
	assert.Equal(t, "describe this", mock.lastPrompt)
	assert.Equal(t, []string{"/tmp/screenshot.png"}, mock.lastImages)
}

// TestInstance_SetCachedPresentation_DeepCopy_ToolDiff verifies that
// SetCachedPresentation stores a deep copy so subsequent mutations to the
// source slice do not affect the cached data.
func TestInstance_SetCachedPresentation_DeepCopy_ToolDiff(t *testing.T) {
	oldN, newN := 1, 1
	turns := []*sdk.PresentationTurn{
		{
			ID:     "t1",
			Number: 1,
			Rows: []sdk.PresentationRow{
				{
					Kind:     sdk.RowToolDiff,
					ToolName: "Edit",
					ToolDiff: &sdk.ToolDiffPayload{
						Path: "original.go",
						Lines: []sdk.ToolDiffLine{
							{Kind: sdk.DiffLineAdded, NewNumber: &newN, NewText: "added"},
							{Kind: sdk.DiffLineRemoved, OldNumber: &oldN, OldText: "removed"},
						},
					},
				},
			},
		},
	}

	inst := &Instance{}
	inst.SetCachedPresentation(turns)

	// Mutate the original after storing.
	turns[0].Rows[0].ToolDiff.Path = "mutated.go"
	turns[0].Rows[0].ToolDiff.Lines[0].NewText = "mutated"
	*turns[0].Rows[0].ToolDiff.Lines[0].NewNumber = 99

	result := inst.CapturePresentation()
	require.Len(t, result, 1)
	require.NotNil(t, result[0].Rows[0].ToolDiff)
	assert.Equal(t, "original.go", result[0].Rows[0].ToolDiff.Path)
	assert.Equal(t, "added", result[0].Rows[0].ToolDiff.Lines[0].NewText)
	require.NotNil(t, result[0].Rows[0].ToolDiff.Lines[0].NewNumber)
	assert.Equal(t, 1, *result[0].Rows[0].ToolDiff.Lines[0].NewNumber)
}

// TestInstance_SetCachedPresentation_DeepCopy_ToolPreview verifies that
// SetCachedPresentation deep copies ToolPreview payloads.
func TestInstance_SetCachedPresentation_DeepCopy_ToolPreview(t *testing.T) {
	turns := []*sdk.PresentationTurn{
		{
			ID:     "t1",
			Number: 1,
			Rows: []sdk.PresentationRow{
				{
					Kind:     sdk.RowToolPreview,
					ToolName: "read_file",
					ToolPreview: &sdk.ToolPreviewPayload{
						Lines: []string{"original line"},
					},
				},
			},
		},
	}

	inst := &Instance{}
	inst.SetCachedPresentation(turns)

	// Mutate the original.
	turns[0].Rows[0].ToolPreview.Lines[0] = "mutated"

	result := inst.CapturePresentation()
	require.Len(t, result, 1)
	require.NotNil(t, result[0].Rows[0].ToolPreview)
	assert.Equal(t, "original line", result[0].Rows[0].ToolPreview.Lines[0])
}

// mockShellRunner is an ExecutionSession that satisfies shellCommandRunner.
type mockShellRunner struct {
	deadExecutionSession
	runShellErr error
	lastCommand string
}

func (m *mockShellRunner) RunShellCommand(_ context.Context, command string) error {
	m.lastCommand = command
	return m.runShellErr
}

func TestInstance_RunShellCommand_NotStarted_ReturnsError(t *testing.T) {
	inst := &Instance{}
	err := inst.RunShellCommand(context.Background(), "echo")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not started")
}

func TestInstance_RunShellCommand_DeadSession_UnsupportedBackend(t *testing.T) {
	inst := &Instance{}
	inst.MarkStartedDeadForTest()
	// deadExecutionSession does not implement shellCommandRunner.
	err := inst.RunShellCommand(context.Background(), "echo")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "shell execution is not supported")
}

func TestInstance_RunShellCommand_TmuxSession_UnsupportedBackend(t *testing.T) {
	inst := &Instance{ExecutionMode: ExecutionModeTmux}
	inst.started = true
	// tmuxExecutionSession wraps a nil TmuxSession — won't implement shellCommandRunner.
	inst.SetExecutionSessionForTest(&tmuxExecutionSession{})
	err := inst.RunShellCommand(context.Background(), "echo")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "shell execution is not supported")
	assert.Contains(t, err.Error(), "tmux")
}

func TestInstance_RunShellCommand_SDKSession_Delegates(t *testing.T) {
	inst := &Instance{ExecutionMode: ExecutionModeSDK}
	inst.started = true
	mock := &mockShellRunner{}
	inst.SetExecutionSessionForTest(mock)

	err := inst.RunShellCommand(context.Background(), "echo hello")
	require.NoError(t, err)
	assert.Equal(t, "echo hello", mock.lastCommand)
}

// mockRendererStatsSession is an ExecutionSession that also implements
// rendererStatsProvider to let CollectMetadata tests verify the stats path.
type mockRendererStatsSession struct {
	deadExecutionSession
	stats sdk.RendererStats
}

func (m *mockRendererStatsSession) RendererStats() sdk.RendererStats {
	return m.stats
}

// TestCollectMetadata_CopiesRendererStats verifies that CollectMetadata copies
// RendererStats from the session into InstanceMetadata when the session implements
// rendererStatsProvider.
func TestCollectMetadata_CopiesRendererStats(t *testing.T) {
	inst := &Instance{started: true, Status: Running}
	mock := &mockRendererStatsSession{
		stats: sdk.RendererStats{
			Bytes:    1024,
			Lines:    42,
			Turns:    5,
			MaxBytes: 4 << 20,
			MaxTurns: 2000,
		},
	}
	inst.SetExecutionSessionForTest(mock)

	m := inst.CollectMetadata()
	assert.Equal(t, int64(1024), m.RendererStats.Bytes)
	assert.Equal(t, int64(42), m.RendererStats.Lines)
	assert.Equal(t, int64(5), m.RendererStats.Turns)
	assert.Equal(t, int64(4<<20), m.RendererStats.MaxBytes)
	assert.Equal(t, int64(2000), m.RendererStats.MaxTurns)
}

// TestCollectMetadata_TmuxSession_NoRendererStats verifies that CollectMetadata
// returns a zero RendererStats for tmux-backed sessions that do not implement
// rendererStatsProvider.
func TestCollectMetadata_TmuxSession_NoRendererStats(t *testing.T) {
	inst := &Instance{started: true, Status: Running}
	inst.SetExecutionSessionForTest(deadExecutionSession{})

	m := inst.CollectMetadata()
	assert.Equal(t, sdk.RendererStats{}, m.RendererStats)
}

// TestInstance_SetCachedRendererStats_IsValueCopy verifies that
// SetCachedRendererStats stores a value-copy that cannot be aliased by the
// caller retaining the original struct.
func TestInstance_SetCachedRendererStats_IsValueCopy(t *testing.T) {
	inst := &Instance{}
	original := sdk.RendererStats{Bytes: 100, Lines: 10, Turns: 3}
	inst.SetCachedRendererStats(original)

	// Mutating the local copy must not affect the stored value.
	original.Bytes = 999
	assert.Equal(t, int64(100), inst.RendererStats.Bytes, "cached stats must not alias original")
	assert.Equal(t, int64(10), inst.RendererStats.Lines)
	assert.Equal(t, int64(3), inst.RendererStats.Turns)
}

// TestInstance_SetCachedPresentation_DeepCopy_Activity verifies that Activity
// pointer fields are deep-copied.
func TestInstance_SetCachedPresentation_DeepCopy_Activity(t *testing.T) {
	ts := sdk.PresentationTurn{}.StartedAt // zero time
	_ = ts

	turns := []*sdk.PresentationTurn{
		{
			ID:     "t1",
			Number: 1,
			Activity: &sdk.TurnActivity{
				Kind:  "tool",
				Label: "original label",
			},
		},
	}

	inst := &Instance{}
	inst.SetCachedPresentation(turns)

	// Mutate original activity.
	turns[0].Activity.Label = "mutated"

	result := inst.CapturePresentation()
	require.Len(t, result, 1)
	require.NotNil(t, result[0].Activity)
	assert.Equal(t, "original label", result[0].Activity.Label)
}
