package taskfsm

import (
	"testing"

	"github.com/kastheco/kasmos/config/taskstore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Transition is the single unblock path. Nothing else can move a blocked task —
// blocking suppresses every agent spawn — so a transition arriving at all means
// a human (or the supervisor acting for one) made the call.
func TestTransition_ClearsDecisionBlock(t *testing.T) {
	store := taskstore.NewTestStore(t)
	require.NoError(t, store.Create("test", taskstore.TaskEntry{
		Filename: "my-plan.md",
		Status:   taskstore.StatusReviewing,
	}))
	require.NoError(t, store.SetBlocked("test", "my-plan.md", "pick (a) or (b)", "agent"))

	m := New(store, "test", "")
	require.NoError(t, m.Transition("my-plan.md", ReviewChangesRequested))

	entry, err := store.Get("test", "my-plan.md")
	require.NoError(t, err)
	assert.Equal(t, taskstore.StatusImplementing, entry.Status)
	assert.Empty(t, entry.BlockedReason)
	assert.Empty(t, entry.BlockedSource)
}

// A rejected transition must not clear the block: the task is still waiting.
func TestTransition_InvalidEventKeepsDecisionBlock(t *testing.T) {
	store := taskstore.NewTestStore(t)
	require.NoError(t, store.Create("test", taskstore.TaskEntry{
		Filename: "my-plan.md",
		Status:   taskstore.StatusReviewing,
	}))
	require.NoError(t, store.SetBlocked("test", "my-plan.md", "pick (a) or (b)", "agent"))

	m := New(store, "test", "")
	require.Error(t, m.Transition("my-plan.md", PlannerFinished))

	entry, err := store.Get("test", "my-plan.md")
	require.NoError(t, err)
	assert.Equal(t, "pick (a) or (b)", entry.BlockedReason)
}

func TestNormalizeGatewaySignalPayload_NeedsDecision(t *testing.T) {
	for _, tc := range []struct {
		name    string
		payload string
		want    string
	}{
		{"json reason", `{"reason":"pick (a) or (b)"}`, `{"reason":"pick (a) or (b)"}`},
		{"body alias", `{"body":"pick (a) or (b)"}`, `{"reason":"pick (a) or (b)"}`},
		{"bare text", `pick (a) or (b)`, `{"reason":"pick (a) or (b)"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := NormalizeGatewaySignalPayload(NeedsDecisionSignal, tc.payload)
			require.NoError(t, err)
			assert.JSONEq(t, tc.want, got)
		})
	}

	// A blocked task with no stated question is worse than useless: it stops the
	// work and tells the operator nothing about how to restart it.
	for _, payload := range []string{"", `{}`, `{"reason":"  "}`} {
		_, err := NormalizeGatewaySignalPayload(NeedsDecisionSignal, payload)
		assert.Error(t, err, "payload %q must be rejected", payload)
	}

	// A truncated or otherwise broken envelope must not slide into the bare-text
	// branch: that stores the whole malformed envelope as the reason, so the
	// operator is shown nested JSON instead of the question. Observed for real
	// when a single closing brace was lost in transit.
	_, err := NormalizeGatewaySignalPayload(NeedsDecisionSignal, `{"reason":"pick (a) or (b)"`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "looks like JSON but does not parse")
}

func TestCanonicalGatewaySignalType_NeedsDecisionAliases(t *testing.T) {
	for _, raw := range []string{"needs_decision", "needs-decision", "needs_input", "blocked", "needs_human"} {
		got, err := CanonicalGatewaySignalType(raw)
		require.NoError(t, err, "alias %q", raw)
		assert.Equal(t, NeedsDecisionSignal, got)
	}
}
