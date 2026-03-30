package taskfsm

import (
	"testing"

	"github.com/kastheco/kasmos/config/taskstore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCanonicalGatewaySignalType_CoversCanonicalAndAliases(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{name: "planner canonical underscore", raw: "planner_finished", want: "planner_finished"},
		{name: "planner hyphen alias", raw: "planner-finished", want: "planner_finished"},
		{name: "implement canonical underscore", raw: "implement_finished", want: "implement_finished"},
		{name: "implement hyphen alias", raw: "implement-finished", want: "implement_finished"},
		{name: "review approved canonical underscore", raw: "review_approved", want: "review_approved"},
		{name: "review approved hyphen alias", raw: "review-approved", want: "review_approved"},
		{name: "review changes requested canonical underscore", raw: "review_changes_requested", want: "review_changes_requested"},
		{name: "review changes requested hyphen alias", raw: "review-changes-requested", want: "review_changes_requested"},
		{name: "review changes underscore alias", raw: "review_changes", want: "review_changes_requested"},
		{name: "review changes hyphen alias", raw: "review-changes", want: "review_changes_requested"},
		{name: "implement task finished canonical underscore", raw: "implement_task_finished", want: "implement_task_finished"},
		{name: "implement task finished hyphen alias", raw: "implement-task-finished", want: "implement_task_finished"},
		{name: "implement wave canonical underscore", raw: "implement_wave", want: "implement_wave"},
		{name: "implement wave hyphen alias", raw: "implement-wave", want: "implement_wave"},
		{name: "elaborator canonical wire name", raw: "elaborator_finished", want: "elaborator_finished"},
		{name: "elaborator hyphen wire alias", raw: "elaborator-finished", want: "elaborator_finished"},
		{name: "architect internal alias", raw: "architect_finished", want: "elaborator_finished"},
		{name: "architect hyphen alias", raw: "architect-finished", want: "elaborator_finished"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CanonicalGatewaySignalType(tt.raw)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}

	_, err := CanonicalGatewaySignalType("unknown_signal")
	assert.Error(t, err)
}

func TestGatewaySignalTypeForEvent(t *testing.T) {
	tests := []struct {
		name  string
		event Event
		want  string
	}{
		{name: "planner finished", event: PlannerFinished, want: "planner_finished"},
		{name: "review changes requested", event: ReviewChangesRequested, want: "review_changes_requested"},
		{name: "architect finished", event: ArchitectFinished, want: "elaborator_finished"},
		{name: "legacy elaborator finished", event: Event("elaborator_finished"), want: "elaborator_finished"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GatewaySignalTypeForEvent(tt.event)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}

	_, err := GatewaySignalTypeForEvent(RequestReview)
	assert.Error(t, err)
}

func TestNormalizeGatewaySignalPayload_AcceptsAliases(t *testing.T) {
	payload, err := NormalizeGatewaySignalPayload("review-changes", "needs fixes")
	require.NoError(t, err)
	assert.JSONEq(t, `{"body":"needs fixes"}`, payload)

	payload, err = NormalizeGatewaySignalPayload("architect_finished", "")
	require.NoError(t, err)
	assert.Equal(t, "", payload)
}

func TestEmitGatewaySignal_CanonicalizesStoredSignalType(t *testing.T) {
	gw, err := taskstore.NewSQLiteSignalGateway(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = gw.Close() })

	err = EmitGatewaySignal(gw, "proj", "architect-finished", "feature", "")
	require.NoError(t, err)

	signals, err := gw.List("proj", taskstore.SignalPending)
	require.NoError(t, err)
	require.Len(t, signals, 1)
	assert.Equal(t, "elaborator_finished", signals[0].SignalType)
}
