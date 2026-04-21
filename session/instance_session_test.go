package session

import (
	"testing"

	"github.com/charmbracelet/x/ansi"
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
