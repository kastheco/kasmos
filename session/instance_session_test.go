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

func TestInstance_CapturePresentation_NilSession(t *testing.T) {
	inst := &Instance{started: true}
	// executionSession is nil
	result := inst.CapturePresentation()
	assert.Nil(t, result)
}
