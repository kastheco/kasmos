package taskfsm

import (
	"testing"

	"github.com/kastheco/kasmos/config/taskstore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTaskStateMachine_LifecycleSignalMatrix(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		initial    taskstore.Status
		event      Event
		wantStatus taskstore.Status
	}{
		{name: "planner finished", initial: taskstore.StatusPlanning, event: PlannerFinished, wantStatus: taskstore.StatusReady},
		{name: "implement finished", initial: taskstore.StatusImplementing, event: ImplementFinished, wantStatus: taskstore.StatusReviewing},
		{name: "review approved", initial: taskstore.StatusReviewing, event: ReviewApproved, wantStatus: taskstore.StatusVerifying},
		{name: "review changes requested", initial: taskstore.StatusReviewing, event: ReviewChangesRequested, wantStatus: taskstore.StatusImplementing},
		{name: "verify approved", initial: taskstore.StatusVerifying, event: VerifyApproved, wantStatus: taskstore.StatusDone},
		{name: "verify failed", initial: taskstore.StatusVerifying, event: VerifyFailed, wantStatus: taskstore.StatusImplementing},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store, err := taskstore.NewSQLiteStore(":memory:")
			require.NoError(t, err)
			defer store.Close()

			const project = "proj"
			require.NoError(t, store.Create(project, taskstore.TaskEntry{Filename: "plan.md", Status: tt.initial}))

			fsm := New(store, project, "")
			require.NoError(t, fsm.Transition("plan.md", tt.event))

			entry, err := store.Get(project, "plan.md")
			require.NoError(t, err)
			assert.Equal(t, tt.wantStatus, entry.Status)
		})
	}
}
