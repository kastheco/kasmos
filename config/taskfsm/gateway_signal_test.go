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
		{name: "plan_start canonical underscore", raw: "plan_start", want: "plan_start"},
		{name: "plan_start hyphen alias", raw: "plan-start", want: "plan_start"},
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
		{name: "verify approved canonical", raw: "verify_approved", want: "verify_approved"},
		{name: "verify approved hyphen alias", raw: "verify-approved", want: "verify_approved"},
		{name: "verify failed canonical", raw: "verify_failed", want: "verify_failed"},
		{name: "verify failed hyphen alias", raw: "verify-failed", want: "verify_failed"},
		// deprecated readiness/master aliases canonicalize to verify_*
		{name: "readiness approved alias", raw: "readiness_approved", want: "verify_approved"},
		{name: "readiness approved hyphen alias", raw: "readiness-approved", want: "verify_approved"},
		{name: "master approved alias", raw: "master_approved", want: "verify_approved"},
		{name: "master approved hyphen alias", raw: "master-approved", want: "verify_approved"},
		{name: "readiness changes requested alias", raw: "readiness_changes_requested", want: "verify_failed"},
		{name: "readiness changes requested hyphen alias", raw: "readiness-changes-requested", want: "verify_failed"},
		{name: "readiness changes short alias", raw: "readiness_changes", want: "verify_failed"},
		{name: "readiness changes short hyphen alias", raw: "readiness-changes", want: "verify_failed"},
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
		{name: "plan start", event: PlanStart, want: "plan_start"},
		{name: "planner finished", event: PlannerFinished, want: "planner_finished"},
		{name: "review changes requested", event: ReviewChangesRequested, want: "review_changes_requested"},
		{name: "verify approved", event: VerifyApproved, want: "verify_approved"},
		{name: "verify failed", event: VerifyFailed, want: "verify_failed"},
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

func TestNormalizeGatewaySignalPayload_VerifySignals(t *testing.T) {
	tests := []struct {
		name        string
		signalType  string
		payload     string
		wantPayload string
	}{
		{
			name:        "verify_approved empty payload",
			signalType:  "verify_approved",
			payload:     "",
			wantPayload: "",
		},
		{
			name:        "verify_approved plain text wrapped in body",
			signalType:  "verify_approved",
			payload:     "lgtm",
			wantPayload: `{"body":"lgtm"}`,
		},
		{
			name:        "verify_approved json passthrough",
			signalType:  "verify_approved",
			payload:     `{"body":"all good"}`,
			wantPayload: `{"body":"all good"}`,
		},
		{
			name:        "verify_failed plain text wrapped",
			signalType:  "verify_failed",
			payload:     "address security findings",
			wantPayload: `{"body":"address security findings"}`,
		},
		// deprecated alias still normalizes payload correctly
		{
			name:        "master_approved alias normalizes payload",
			signalType:  "master_approved",
			payload:     "ship it",
			wantPayload: `{"body":"ship it"}`,
		},
		{
			name:        "readiness_changes alias normalizes payload",
			signalType:  "readiness-changes",
			payload:     "fix the edge cases",
			wantPayload: `{"body":"fix the edge cases"}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NormalizeGatewaySignalPayload(tt.signalType, tt.payload)
			require.NoError(t, err)
			if tt.wantPayload == "" {
				assert.Equal(t, "", got)
			} else {
				assert.JSONEq(t, tt.wantPayload, got)
			}
		})
	}
}

func TestEmitGatewaySignal_VerifySignalsCanonicalize(t *testing.T) {
	gw, err := taskstore.NewSQLiteSignalGateway(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = gw.Close() })

	// canonical new types
	require.NoError(t, EmitGatewaySignal(gw, "proj", "verify_approved", "feature", "lgtm"))
	require.NoError(t, EmitGatewaySignal(gw, "proj", "verify_failed", "feature", "needs work"))

	signals, err := gw.List("proj", taskstore.SignalPending)
	require.NoError(t, err)
	require.Len(t, signals, 2)

	assert.Equal(t, "verify_approved", signals[0].SignalType)
	assert.JSONEq(t, `{"body":"lgtm"}`, signals[0].Payload)

	assert.Equal(t, "verify_failed", signals[1].SignalType)
	assert.JSONEq(t, `{"body":"needs work"}`, signals[1].Payload)
}

func TestEmitGatewaySignal_DeprecatedAliasesCanonicalize(t *testing.T) {
	gw, err := taskstore.NewSQLiteSignalGateway(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = gw.Close() })

	require.NoError(t, EmitGatewaySignal(gw, "proj", "master-approved", "feature", "lgtm"))
	require.NoError(t, EmitGatewaySignal(gw, "proj", "readiness-changes", "feature", "needs work"))

	signals, err := gw.List("proj", taskstore.SignalPending)
	require.NoError(t, err)
	require.Len(t, signals, 2)

	assert.Equal(t, "verify_approved", signals[0].SignalType)
	assert.JSONEq(t, `{"body":"lgtm"}`, signals[0].Payload)

	assert.Equal(t, "verify_failed", signals[1].SignalType)
	assert.JSONEq(t, `{"body":"needs work"}`, signals[1].Payload)
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

func TestCanonicalGatewaySignalType_PlannerDraftFinished(t *testing.T) {
	got, err := CanonicalGatewaySignalType("planner_draft_finished")
	require.NoError(t, err)
	assert.Equal(t, "planner_draft_finished", got)

	got, err = CanonicalGatewaySignalType("planner-draft-finished")
	require.NoError(t, err)
	assert.Equal(t, "planner_draft_finished", got)
}

func TestGatewaySignalTypeForEvent_DoesNotMapPlannerDraftFinished(t *testing.T) {
	// planner_draft_finished is intentionally not an FSM event and must not be
	// returned by GatewaySignalTypeForEvent.
	for _, event := range []Event{PlanStart, PlannerFinished, ImplementFinished,
		ReviewApproved, ReviewChangesRequested, VerifyApproved, VerifyFailed,
		ArchitectFinished} {
		got, err := GatewaySignalTypeForEvent(event)
		require.NoError(t, err)
		assert.NotEqual(t, "planner_draft_finished", got)
	}
}

func TestNormalizeGatewaySignalPayload_PlannerDraftFinished(t *testing.T) {
	tests := []struct {
		name      string
		payload   string
		wantErr   bool
		errSubstr string
		wantOut   string
	}{
		{
			name:      "empty payload rejected",
			payload:   "",
			wantErr:   true,
			errSubstr: "requires JSON with a non-empty planner_id",
		},
		{
			name:      "malformed JSON rejected",
			payload:   "not-json",
			wantErr:   true,
			errSubstr: "must be valid JSON",
		},
		{
			name:      "array payload rejected",
			payload:   `["planner_x"]`,
			wantErr:   true,
			errSubstr: "must be valid JSON",
		},
		{
			name:      "non-string planner_id rejected",
			payload:   `{"planner_id":42}`,
			wantErr:   true,
			errSubstr: "planner_id must be a string",
		},
		{
			name:      "empty string planner_id rejected",
			payload:   `{"planner_id":""}`,
			wantErr:   true,
			errSubstr: "planner_id must not be empty",
		},
		{
			name:      "whitespace planner_id rejected",
			payload:   `{"planner_id":"   "}`,
			wantErr:   true,
			errSubstr: "planner_id must not be empty",
		},
		{
			name:    "planner_id is normalized",
			payload: `{"planner_id":" planner_x "}`,
			wantOut: `{"planner_id":"planner_x"}`,
		},
		{
			name:    "valid payload accepted",
			payload: `{"planner_id":"planner_x"}`,
			wantOut: `{"planner_id":"planner_x"}`,
		},
		{
			name:    "valid payload with extra fields accepted",
			payload: `{"planner_id":"alpha","extra":"ignored"}`,
			wantOut: `{"planner_id":"alpha","extra":"ignored"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NormalizeGatewaySignalPayload("planner_draft_finished", tt.payload)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errSubstr)
				return
			}
			require.NoError(t, err)
			assert.JSONEq(t, tt.wantOut, got)
		})
	}
}

func TestNormalizeGatewaySignalPayload_PlannerDraftFinished_HyphenAlias(t *testing.T) {
	got, err := NormalizeGatewaySignalPayload("planner-draft-finished", `{"planner_id":"planner_x"}`)
	require.NoError(t, err)
	assert.JSONEq(t, `{"planner_id":"planner_x"}`, got)
}

func TestEmitGatewaySignal_PlannerDraftFinished(t *testing.T) {
	gw, err := taskstore.NewSQLiteSignalGateway(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = gw.Close() })

	err = EmitGatewaySignal(gw, "proj", "planner_draft_finished", "my-feature", `{"planner_id":"planner_x"}`)
	require.NoError(t, err)

	signals, err := gw.List("proj", taskstore.SignalPending)
	require.NoError(t, err)
	require.Len(t, signals, 1)
	assert.Equal(t, "planner_draft_finished", signals[0].SignalType)
	assert.JSONEq(t, `{"planner_id":"planner_x"}`, signals[0].Payload)
	assert.Equal(t, "my-feature", signals[0].PlanFile)
}

func TestEmitGatewaySignal_PlannerDraftFinished_RejectsEmptyPayload(t *testing.T) {
	gw, err := taskstore.NewSQLiteSignalGateway(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = gw.Close() })

	err = EmitGatewaySignal(gw, "proj", "planner_draft_finished", "my-feature", "")
	assert.Error(t, err)
}
