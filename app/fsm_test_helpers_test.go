package app

import (
	"fmt"
	"testing"

	"github.com/kastheco/kasmos/config/planfsm"
	"github.com/kastheco/kasmos/config/planstate"
	"github.com/stretchr/testify/require"
)

// testFSM wraps PlanStateMachine with a convenience method for string-based events.
type testFSM struct {
	*planfsm.PlanStateMachine
}

func newFSMForTest(dir string) *testFSM {
	return &testFSM{planfsm.New(dir)}
}

// seedPlanStatus directly sets a plan's status in the PlanState for test setup,
// bypassing the FSM. Use this instead of ps.SetStatus in tests.
func seedPlanStatus(t *testing.T, ps *planstate.PlanState, planFile string, status planstate.Status) {
	t.Helper()
	entry := ps.Plans[planFile]
	entry.Status = status
	ps.Plans[planFile] = entry
	require.NoError(t, ps.Save())
}

// TransitionByName applies an event by its string name (for table-driven tests).
func (f *testFSM) TransitionByName(planFile, eventName string) error {
	eventMap := map[string]planfsm.Event{
		"plan_start":         planfsm.PlanStart,
		"planner_finished":   planfsm.PlannerFinished,
		"implement_start":    planfsm.ImplementStart,
		"implement_finished": planfsm.ImplementFinished,
		"review_approved":    planfsm.ReviewApproved,
		"review_changes":     planfsm.ReviewChangesRequested,
		"start_over":         planfsm.StartOver,
		"cancel":             planfsm.Cancel,
		"reopen":             planfsm.Reopen,
	}
	ev, ok := eventMap[eventName]
	if !ok {
		return fmt.Errorf("unknown event name: %q", eventName)
	}
	return f.Transition(planFile, ev)
}
