package pr

import (
	"context"
	"testing"

	"github.com/kastheco/kasmos/config/taskstore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEligibleIncludesBranchlessDoneTask(t *testing.T) {
	t.Parallel()
	assert.True(t, Eligible(taskstore.TaskEntry{Status: taskstore.StatusDone}))
	assert.False(t, Eligible(taskstore.TaskEntry{Status: taskstore.StatusReviewing}))
	assert.False(t, Eligible(taskstore.TaskEntry{Status: taskstore.StatusDone, PRURL: "https://example.test/pr/1"}))
}

func TestEnsurePersistsNonSuccessOutcomes(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		enabled bool
		outcome Outcome
		reason  string
	}{
		{name: "disabled", enabled: false, outcome: OutcomeSkipped, reason: "auto pr disabled by config"},
		{name: "missing branch", enabled: true, outcome: OutcomeBlocked, reason: "no branch recorded"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := taskstore.NewTestStore(t)
			require.NoError(t, store.Create("test", taskstore.TaskEntry{Filename: "plan", Status: taskstore.StatusDone}))
			res, err := Ensure(context.Background(), store, Request{RepoPath: t.TempDir(), Project: "test", PlanFile: "plan", Enabled: tt.enabled})
			require.NoError(t, err)
			assert.Equal(t, tt.outcome, res.Outcome)
			assert.Contains(t, res.Reason, tt.reason)
			entry, getErr := store.Get("test", "plan")
			require.NoError(t, getErr)
			assert.Equal(t, string(tt.outcome), entry.PRCreateState)
			assert.NotEmpty(t, entry.PRCreateError)
			assert.Equal(t, 1, entry.PRCreateAttempts)
		})
	}
}
