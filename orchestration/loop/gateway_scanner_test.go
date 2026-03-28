package loop

import (
	"testing"

	"github.com/kastheco/kasmos/config/taskstore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScanGateway_ClaimsAndConvertsSignals(t *testing.T) {
	gw := newTestGateway(t)
	require.NoError(t, gw.Create("proj", taskstore.SignalEntry{PlanFile: "my-plan", SignalType: "planner_finished", Payload: `{"body":"done"}`}))
	require.NoError(t, gw.Create("proj", taskstore.SignalEntry{PlanFile: "my-plan", SignalType: "implement_task_finished", Payload: `{"wave_number":2,"task_number":3}`}))
	require.NoError(t, gw.Create("proj", taskstore.SignalEntry{PlanFile: "my-plan", SignalType: "elaborator_finished", Payload: `{}`}))

	result, ids, err := ScanGateway(gw, "proj", "daemon:test")
	require.NoError(t, err)
	assert.Len(t, result.FSMSignals, 1)
	assert.Equal(t, "done", result.FSMSignals[0].Body)
	assert.Len(t, result.TaskSignals, 1)
	assert.Equal(t, 2, result.TaskSignals[0].WaveNumber)
	assert.Equal(t, 3, result.TaskSignals[0].TaskNumber)
	assert.Len(t, result.ElaborationSignals, 1)
	assert.Equal(t, "my-plan", result.ElaborationSignals[0].TaskFile)
	assert.Len(t, ids, 3)

	processing, err := gw.List("proj", taskstore.SignalProcessing)
	require.NoError(t, err)
	assert.Len(t, processing, 3)
}

func TestScanGateway_Empty(t *testing.T) {
	gw := newTestGateway(t)
	result, ids, err := ScanGateway(gw, "proj", "daemon:test")
	require.NoError(t, err)
	assert.Empty(t, result.FSMSignals)
	assert.Empty(t, result.TaskSignals)
	assert.Empty(t, result.WaveSignals)
	assert.Empty(t, result.ElaborationSignals)
	assert.Empty(t, ids)
}

func TestScanGateway_BadPayloadReturnsError(t *testing.T) {
	gw := newTestGateway(t)
	require.NoError(t, gw.Create("proj", taskstore.SignalEntry{PlanFile: "my-plan", SignalType: "implement_wave", Payload: `{"wave_number":"x"}`}))

	_, ids, err := ScanGateway(gw, "proj", "daemon:test")
	require.Error(t, err)
	// Bad row is marked SignalFailed immediately inside ScanGateway, not returned in ids.
	assert.Empty(t, ids)

	failed, listErr := gw.List("proj", taskstore.SignalFailed)
	require.NoError(t, listErr)
	assert.Len(t, failed, 1)
}

func TestConvertSignalEntry_AcceptsArchitectSignalAliasesAtGatewayBoundary(t *testing.T) {
	t.Parallel()

	entries := []*taskstore.SignalEntry{
		{PlanFile: "legacy-plan", SignalType: "elaborator_finished"},
		{PlanFile: "canonical-plan", SignalType: "architect_finished"},
	}

	var result ScanResult
	for _, entry := range entries {
		require.NoError(t, ConvertSignalEntry(entry, &result))
	}

	require.Len(t, result.ElaborationSignals, 2)
	assert.Equal(t, "legacy-plan", result.ElaborationSignals[0].TaskFile)
	assert.Equal(t, "canonical-plan", result.ElaborationSignals[1].TaskFile)
}
