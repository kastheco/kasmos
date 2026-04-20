package session

import (
	"testing"

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
	assert.Contains(t, preview, "> send a message to the agent")
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
