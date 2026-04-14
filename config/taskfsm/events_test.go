package taskfsm

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEventByName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		input     string
		wantEvent Event
		wantOK    bool
	}{
		// Primary tokens
		{name: "plan_start", input: "plan_start", wantEvent: PlanStart, wantOK: true},
		{name: "planner_finished", input: "planner_finished", wantEvent: PlannerFinished, wantOK: true},
		{name: "implement_start", input: "implement_start", wantEvent: ImplementStart, wantOK: true},
		{name: "implement_finished", input: "implement_finished", wantEvent: ImplementFinished, wantOK: true},
		{name: "review_approved", input: "review_approved", wantEvent: ReviewApproved, wantOK: true},
		{name: "review_changes primary token", input: "review_changes", wantEvent: ReviewChangesRequested, wantOK: true},
		{name: "verify_approved", input: "verify_approved", wantEvent: VerifyApproved, wantOK: true},
		{name: "verify_failed", input: "verify_failed", wantEvent: VerifyFailed, wantOK: true},
		{name: "request_review", input: "request_review", wantEvent: RequestReview, wantOK: true},
		{name: "start_over", input: "start_over", wantEvent: StartOver, wantOK: true},
		{name: "reimplement", input: "reimplement", wantEvent: Reimplement, wantOK: true},
		{name: "cancel", input: "cancel", wantEvent: Cancel, wantOK: true},
		{name: "reopen", input: "reopen", wantEvent: Reopen, wantOK: true},

		// Canonical alias for ReviewChangesRequested
		{name: "review_changes_requested alias", input: "review_changes_requested", wantEvent: ReviewChangesRequested, wantOK: true},

		// Hyphen normalization
		{name: "review-changes hyphen", input: "review-changes", wantEvent: ReviewChangesRequested, wantOK: true},
		{name: "plan-start hyphen", input: "plan-start", wantEvent: PlanStart, wantOK: true},

		// Whitespace trimming
		{name: "trimmed whitespace", input: "  plan_start  ", wantEvent: PlanStart, wantOK: true},
		{name: "trimmed hyphen-form", input: "  review-changes  ", wantEvent: ReviewChangesRequested, wantOK: true},

		// Unknown events
		{name: "empty string", input: "", wantOK: false},
		{name: "unknown event", input: "fly_away", wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ev, ok := EventByName(tt.input)
			assert.Equal(t, tt.wantOK, ok)
			assert.Equal(t, tt.wantEvent, ev)
		})
	}
}

func TestEventNames(t *testing.T) {
	t.Parallel()

	names := EventNames()

	// Must be sorted
	sorted := make([]string, len(names))
	copy(sorted, names)
	sort.Strings(sorted)
	assert.Equal(t, sorted, names, "EventNames() must return sorted names")

	// Must include primary tokens
	require.Contains(t, names, "plan_start")
	require.Contains(t, names, "review_changes")
	require.Contains(t, names, "cancel")

	// Must NOT include the alias
	assert.NotContains(t, names, "review_changes_requested",
		"alias must not appear in EventNames()")

	// Count must match the primary map
	assert.Len(t, names, len(primaryEventNames))
}
