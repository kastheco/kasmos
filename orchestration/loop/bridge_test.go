package loop

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kastheco/kasmos/config/taskstore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBridgeFilesystemSignals(t *testing.T) {
	dir := t.TempDir()
	signalsDir := filepath.Join(dir, ".kasmos", "signals")
	require.NoError(t, os.MkdirAll(signalsDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(signalsDir, "planner-finished-test-plan"), []byte("plan body"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(signalsDir, "implement-task-finished-w1-t2-test-plan"), nil, 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(signalsDir, "elaborator-finished-architect-plan"), nil, 0o644))

	gw := newTestGateway(t)
	n, err := BridgeFilesystemSignals(gw, "proj", dir, nil)
	require.NoError(t, err)
	assert.Equal(t, 3, n)

	entries, err := os.ReadDir(signalsDir)
	require.NoError(t, err)
	assert.Empty(t, entries)

	pending, err := gw.List("proj", taskstore.SignalPending)
	require.NoError(t, err)
	require.Len(t, pending, 3)

	var architectSignal *taskstore.SignalEntry
	for i := range pending {
		if pending[i].SignalType == "elaborator_finished" {
			architectSignal = &pending[i]
			break
		}
	}
	require.NotNil(t, architectSignal)
	assert.Equal(t, "architect-plan", architectSignal.PlanFile)
	assert.Equal(t, "", architectSignal.Payload)
}

func TestBridgeFilesystemSignals_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	gw := newTestGateway(t)
	n, err := BridgeFilesystemSignals(gw, "proj", dir, nil)
	require.NoError(t, err)
	assert.Equal(t, 0, n)
}
