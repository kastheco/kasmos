package taskfsm

import (
	"testing"

	"github.com/kastheco/kasmos/config/taskstore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCanonicalGatewaySignalType(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{name: "hyphenated planner", raw: "planner-finished", want: "planner_finished"},
		{name: "review changes alias", raw: "review_changes", want: "review_changes_requested"},
		{name: "architect canonical", raw: "architect_finished", want: "elaborator_finished"},
		{name: "architect wire alias", raw: "elaborator-finished", want: "elaborator_finished"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CanonicalGatewaySignalType(tt.raw)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
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
