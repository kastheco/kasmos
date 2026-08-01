package loop

import (
	"testing"

	"github.com/kastheco/kasmos/config/taskfsm"
	"github.com/kastheco/kasmos/config/taskstore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProcessor_ProcessDecisionSignals_EmitsBlock(t *testing.T) {
	store := taskstore.NewTestStore(t)
	require.NoError(t, store.Create("test", taskstore.TaskEntry{
		Filename: "my-plan.md",
		Status:   taskstore.StatusReviewing,
		Branch:   "plan/my-plan",
	}))

	p := NewProcessor(ProcessorConfig{Store: store, Project: "test", AutoReviewFix: true})
	actions := p.ProcessDecisionSignals([]taskfsm.DecisionSignal{
		{TaskFile: "my-plan.md", Reason: "contracts or redaction?", Source: "agent"},
	})

	require.Len(t, actions, 1)
	block, ok := actions[0].(BlockTaskAction)
	require.True(t, ok, "expected BlockTaskAction, got %T", actions[0])
	assert.Equal(t, "my-plan.md", block.PlanFile)
	assert.Equal(t, "contracts or redaction?", block.Reason)
	assert.Equal(t, "agent", block.Source)
}

func TestProcessor_ProcessDecisionSignals_RejectsUnusableSignals(t *testing.T) {
	store := taskstore.NewTestStore(t)
	require.NoError(t, store.Create("test", taskstore.TaskEntry{
		Filename: "my-plan.md",
		Status:   taskstore.StatusReviewing,
	}))

	p := NewProcessor(ProcessorConfig{Store: store, Project: "test"})

	// A reason-less block would surface as "blocked" with nothing to decide,
	// which is the silent stall this signal exists to replace.
	assert.Empty(t, p.ProcessDecisionSignals([]taskfsm.DecisionSignal{
		{TaskFile: "my-plan.md", Reason: "   "},
	}))
	assert.Empty(t, p.ProcessDecisionSignals([]taskfsm.DecisionSignal{
		{TaskFile: "no-such-plan.md", Reason: "who decides?"},
	}))
}

// The point of the whole path: a task waiting on a human must stop consuming
// agent time. Before the block existed, reviewer feedback on a stuck task kept
// spawning fixers to guess at an answer only the operator had.
func TestProcessor_BlockedTask_SpawnsNoFixer(t *testing.T) {
	store := taskstore.NewTestStore(t)
	require.NoError(t, store.Create("test", taskstore.TaskEntry{
		Filename: "my-plan.md",
		Status:   taskstore.StatusReviewing,
		Branch:   "plan/my-plan",
	}))
	require.NoError(t, store.SetBlocked("test", "my-plan.md", "founder must pick (a) or (b)", "agent"))

	p := NewProcessor(ProcessorConfig{Store: store, Project: "test", AutoReviewFix: true})
	actions := p.ProcessFSMSignals([]taskfsm.Signal{
		{Event: taskfsm.ReviewChangesRequested, TaskFile: "my-plan.md", Body: "still broken"},
	})

	assert.Empty(t, actions, "a blocked task must produce no actions at all")

	// The block must also survive the dropped signal: nothing here counts as the
	// human answering, so the task stays stopped.
	entry, err := store.Get("test", "my-plan.md")
	require.NoError(t, err)
	assert.Equal(t, taskstore.StatusReviewing, entry.Status)
	assert.Equal(t, "founder must pick (a) or (b)", entry.BlockedReason)
}

func TestProcessor_UnblockedTask_StillSpawnsFixer(t *testing.T) {
	store := taskstore.NewTestStore(t)
	require.NoError(t, store.Create("test", taskstore.TaskEntry{
		Filename: "my-plan.md",
		Status:   taskstore.StatusReviewing,
		Branch:   "plan/my-plan",
	}))

	p := NewProcessor(ProcessorConfig{Store: store, Project: "test", AutoReviewFix: true})
	actions := p.ProcessFSMSignals([]taskfsm.Signal{
		{Event: taskfsm.ReviewChangesRequested, TaskFile: "my-plan.md", Body: "still broken"},
	})

	var spawned bool
	for _, a := range actions {
		if _, ok := a.(SpawnFixerAction); ok {
			spawned = true
		}
	}
	assert.True(t, spawned, "an unblocked task must still auto-fix")
}
