package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kastheco/kasmos/config/taskfsm"
	"github.com/kastheco/kasmos/config/taskstate"
	"github.com/kastheco/kasmos/config/taskstore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExecuteSignalProcess_ConsumesNonFSMSignalsWithoutTransitioningState(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		filename   string
		signalFile string
	}{
		{name: "wave signal", filename: "wave-plan", signalFile: "implement-wave-2-wave-plan"},
		{name: "elaboration signal", filename: "architect-plan", signalFile: "elaborator-finished-architect-plan"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := taskstore.NewTestSQLiteStore(t)
			root := t.TempDir()
			signalsDir := filepath.Join(root, ".kasmos", "signals")
			require.NoError(t, taskfsm.EnsureSignalDirs(signalsDir))

			const project = "proj"
			require.NoError(t, store.Create(project, taskstore.TaskEntry{
				Filename: tt.filename,
				Status:   taskstore.StatusImplementing,
			}))
			require.NoError(t, os.WriteFile(filepath.Join(signalsDir, tt.signalFile), nil, 0o644))

			processed, err := executeSignalProcess(signalProcessOptions{
				repoRoot:   root,
				project:    project,
				signalsDir: signalsDir,
				store:      store,
			})
			require.NoError(t, err)
			assert.Equal(t, 0, processed, "non-FSM signals should be consumed but not counted as FSM transitions")

			_, err = os.Stat(filepath.Join(signalsDir, tt.signalFile))
			assert.True(t, os.IsNotExist(err), "signal file should be consumed")

			ps, err := taskstate.Load(store, project, "")
			require.NoError(t, err)
			entry, ok := ps.Entry(tt.filename)
			require.True(t, ok)
			assert.Equal(t, taskstate.StatusImplementing, entry.Status)
		})
	}
}
