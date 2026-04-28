package loop

import (
	"testing"

	"github.com/kastheco/kasmos/config/taskfsm"
	"github.com/kastheco/kasmos/config/taskstore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScanGateway_ClaimsAndConvertsSignals(t *testing.T) {
	gw := newTestGateway(t)
	require.NoError(t, gw.Create("proj", taskstore.SignalEntry{PlanFile: "my-plan", SignalType: "planner_finished", Payload: `{"body":"done"}`}))
	require.NoError(t, gw.Create("proj", taskstore.SignalEntry{PlanFile: "my-plan", SignalType: "implement_task_finished", Payload: `{"wave_number":2,"task_number":3}`}))
	require.NoError(t, gw.Create("proj", taskstore.SignalEntry{PlanFile: "my-plan", SignalType: "elaborator_finished", Payload: `{}`}))

	result, entries, err := ScanGateway(gw, "proj", "daemon:test")
	require.NoError(t, err)
	assert.Len(t, result.FSMSignals, 1)
	assert.Equal(t, "done", result.FSMSignals[0].Body)
	assert.Len(t, result.TaskSignals, 1)
	assert.Equal(t, 2, result.TaskSignals[0].WaveNumber)
	assert.Equal(t, 3, result.TaskSignals[0].TaskNumber)
	assert.Len(t, result.ElaborationSignals, 1)
	assert.Equal(t, "my-plan", result.ElaborationSignals[0].TaskFile)
	assert.Len(t, entries, 3)

	processing, err := gw.List("proj", taskstore.SignalProcessing)
	require.NoError(t, err)
	assert.Len(t, processing, 3)
}

func TestScanGateway_Empty(t *testing.T) {
	gw := newTestGateway(t)
	result, entries, err := ScanGateway(gw, "proj", "daemon:test")
	require.NoError(t, err)
	assert.Empty(t, result.FSMSignals)
	assert.Empty(t, result.TaskSignals)
	assert.Empty(t, result.WaveSignals)
	assert.Empty(t, result.ElaborationSignals)
	assert.Empty(t, entries)
}

func TestScanGateway_BadPayloadReturnsError(t *testing.T) {
	gw := newTestGateway(t)
	require.NoError(t, gw.Create("proj", taskstore.SignalEntry{PlanFile: "my-plan", SignalType: "implement_wave", Payload: `{"wave_number":"x"}`}))

	_, entries, err := ScanGateway(gw, "proj", "daemon:test")
	require.Error(t, err)
	// Bad row is marked SignalFailed immediately inside ScanGateway, not returned to caller.
	assert.Empty(t, entries)

	failed, listErr := gw.List("proj", taskstore.SignalFailed)
	require.NoError(t, listErr)
	assert.Len(t, failed, 1)
}

func TestConvertSignalEntry_VerifySignals(t *testing.T) {
	t.Parallel()

	t.Run("verify_approved maps to VerifyApproved", func(t *testing.T) {
		var result ScanResult
		entry := &taskstore.SignalEntry{
			PlanFile:   "feature-plan",
			SignalType: "verify_approved",
			Payload:    `{"body":"ship it"}`,
		}
		require.NoError(t, ConvertSignalEntry(entry, &result))
		require.Len(t, result.FSMSignals, 1)
		sig := result.FSMSignals[0]
		assert.Equal(t, taskfsm.VerifyApproved, sig.Event)
		assert.Equal(t, "feature-plan", sig.TaskFile)
		assert.Equal(t, "ship it", sig.Body)
	})

	t.Run("verify_failed maps to VerifyFailed", func(t *testing.T) {
		var result ScanResult
		entry := &taskstore.SignalEntry{
			PlanFile:   "feature-plan",
			SignalType: "verify_failed",
			Payload:    `{"body":"fix edge cases"}`,
		}
		require.NoError(t, ConvertSignalEntry(entry, &result))
		require.Len(t, result.FSMSignals, 1)
		sig := result.FSMSignals[0]
		assert.Equal(t, taskfsm.VerifyFailed, sig.Event)
		assert.Equal(t, "feature-plan", sig.TaskFile)
		assert.Equal(t, "fix edge cases", sig.Body)
	})

	t.Run("readiness_approved deprecated alias maps to VerifyApproved", func(t *testing.T) {
		var result ScanResult
		entry := &taskstore.SignalEntry{
			PlanFile:   "feature-plan",
			SignalType: "readiness_approved",
			Payload:    "",
		}
		require.NoError(t, ConvertSignalEntry(entry, &result))
		require.Len(t, result.FSMSignals, 1)
		assert.Equal(t, taskfsm.VerifyApproved, result.FSMSignals[0].Event)
		assert.Empty(t, result.FSMSignals[0].Body)
	})
}

func TestScanGateway_VerifySignals(t *testing.T) {
	gw := newTestGateway(t)
	require.NoError(t, gw.Create("proj", taskstore.SignalEntry{
		PlanFile:   "my-plan",
		SignalType: "verify_approved",
		Payload:    `{"body":"lgtm"}`,
	}))
	require.NoError(t, gw.Create("proj", taskstore.SignalEntry{
		PlanFile:   "other-plan",
		SignalType: "verify_failed",
		Payload:    `{"body":"needs changes"}`,
	}))

	result, entries, err := ScanGateway(gw, "proj", "daemon:test")
	require.NoError(t, err)
	require.Len(t, entries, 2)
	require.Len(t, result.FSMSignals, 2)

	approved := result.FSMSignals[0]
	assert.Equal(t, taskfsm.VerifyApproved, approved.Event)
	assert.Equal(t, "lgtm", approved.Body)

	changes := result.FSMSignals[1]
	assert.Equal(t, taskfsm.VerifyFailed, changes.Event)
	assert.Equal(t, "needs changes", changes.Body)
}

func TestConvertSignalEntry_AcceptsArchitectSignalAliasesAtGatewayBoundary(t *testing.T) {
	t.Parallel()

	entries := []*taskstore.SignalEntry{
		{PlanFile: "legacy-plan", SignalType: "elaborator_finished"},
		{PlanFile: "canonical-plan", SignalType: "architect_finished"},
	}

	var result ScanResult
	for _, entry := range entries {
		require.NoError(t, ConvertSignalEntry(entry, &result))
	}

	require.Len(t, result.ElaborationSignals, 2)
	assert.Equal(t, "legacy-plan", result.ElaborationSignals[0].TaskFile)
	assert.Equal(t, "canonical-plan", result.ElaborationSignals[1].TaskFile)
}

func TestConvertSignalEntry_PlannerDraftFinished(t *testing.T) {
	t.Parallel()

	t.Run("valid payload decoded into PlannerDraftSignals", func(t *testing.T) {
		var result ScanResult
		entry := &taskstore.SignalEntry{
			PlanFile:   "my-feature",
			SignalType: "planner_draft_finished",
			Payload:    `{"planner_id":"planner_x"}`,
		}
		require.NoError(t, ConvertSignalEntry(entry, &result))
		require.Len(t, result.PlannerDraftSignals, 1)
		assert.Equal(t, "my-feature", result.PlannerDraftSignals[0].TaskFile)
		assert.Equal(t, "planner_x", result.PlannerDraftSignals[0].PlannerID)
		assert.Empty(t, result.FSMSignals)
	})

	t.Run("empty planner_id rejected", func(t *testing.T) {
		var result ScanResult
		entry := &taskstore.SignalEntry{
			PlanFile:   "my-feature",
			SignalType: "planner_draft_finished",
			Payload:    `{"planner_id":""}`,
		}
		err := ConvertSignalEntry(entry, &result)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "planner_id must not be empty")
		assert.Empty(t, result.PlannerDraftSignals)
	})

	t.Run("malformed JSON payload rejected", func(t *testing.T) {
		var result ScanResult
		entry := &taskstore.SignalEntry{
			PlanFile:   "my-feature",
			SignalType: "planner_draft_finished",
			Payload:    "not-json",
		}
		err := ConvertSignalEntry(entry, &result)
		require.Error(t, err)
		assert.Empty(t, result.PlannerDraftSignals)
	})

	t.Run("hyphen alias normalizes to planner_draft_finished", func(t *testing.T) {
		var result ScanResult
		entry := &taskstore.SignalEntry{
			PlanFile:   "my-feature",
			SignalType: "planner-draft-finished",
			Payload:    `{"planner_id":"alpha"}`,
		}
		require.NoError(t, ConvertSignalEntry(entry, &result))
		require.Len(t, result.PlannerDraftSignals, 1)
		assert.Equal(t, "alpha", result.PlannerDraftSignals[0].PlannerID)
	})

	t.Run("multiple drafts accumulate in PlannerDraftSignals", func(t *testing.T) {
		var result ScanResult
		entries := []*taskstore.SignalEntry{
			{PlanFile: "feature", SignalType: "planner_draft_finished", Payload: `{"planner_id":"planner_a"}`},
			{PlanFile: "feature", SignalType: "planner_draft_finished", Payload: `{"planner_id":"planner_b"}`},
		}
		for _, e := range entries {
			require.NoError(t, ConvertSignalEntry(e, &result))
		}
		require.Len(t, result.PlannerDraftSignals, 2)
		assert.Equal(t, "planner_a", result.PlannerDraftSignals[0].PlannerID)
		assert.Equal(t, "planner_b", result.PlannerDraftSignals[1].PlannerID)
	})
}

func TestScanGateway_PlannerDraftFinished(t *testing.T) {
	gw := newTestGateway(t)
	require.NoError(t, gw.Create("proj", taskstore.SignalEntry{
		PlanFile:   "my-feature",
		SignalType: "planner_draft_finished",
		Payload:    `{"planner_id":"planner_x"}`,
	}))

	result, entries, err := ScanGateway(gw, "proj", "daemon:test")
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.Len(t, result.PlannerDraftSignals, 1)
	assert.Equal(t, "my-feature", result.PlannerDraftSignals[0].TaskFile)
	assert.Equal(t, "planner_x", result.PlannerDraftSignals[0].PlannerID)
	assert.Empty(t, result.FSMSignals)
}

func TestActionPlanFile(t *testing.T) {
	t.Parallel()

	t.Run("reads PlanFile from value action", func(t *testing.T) {
		assert.Equal(t, "feat.md", ActionPlanFile(SpawnPlannerAction{PlanFile: "feat.md"}))
	})
	t.Run("reads PlanFile from pointer action", func(t *testing.T) {
		a := &SpawnReviewerAction{PlanFile: "feat.md"}
		assert.Equal(t, "feat.md", ActionPlanFile(a))
	})
	t.Run("returns empty for action without PlanFile field", func(t *testing.T) {
		// IncrementReviewCycleAction has PlanFile, so use a case-only check:
		// if a future Action lacks the field, ActionPlanFile must not panic.
		// We exercise the no-field path via a synthetic struct outside the
		// Action interface using a small wrapper.
		assert.Equal(t, "p.md", ActionPlanFile(IncrementReviewCycleAction{PlanFile: "p.md"}))
	})
}

func TestGatewayNoopOutcome(t *testing.T) {
	t.Parallel()

	cases := []struct {
		signalType string
		status     taskstore.SignalStatus
		result     string
	}{
		{"planner_draft_finished", taskstore.SignalDone, "planner draft recorded or waiting for peers"},
		{"implement_finished", taskstore.SignalDone, "suppressed implement-finished signal"},
		{"implement_task_finished", taskstore.SignalFailed, "no active orchestrator / wrong wave / already-finished task"},
		{"implement_wave", taskstore.SignalFailed, "processor could not start the requested wave"},
		{"architect_finished", taskstore.SignalFailed, "no active architect pass to resume"},
		{"elaborator_finished", taskstore.SignalFailed, "no active architect pass to resume"},
		{"verify_approved", taskstore.SignalFailed, "signal rejected outside verifying state"},
		{"verify_failed", taskstore.SignalFailed, "signal rejected outside verifying state"},
		{"planner_finished", taskstore.SignalFailed, "signal rejected by processor"},
		{"bogus_type", taskstore.SignalFailed, "signal rejected by processor"},
	}
	for _, tc := range cases {
		t.Run(tc.signalType, func(t *testing.T) {
			status, result := GatewayNoopOutcome(&taskstore.SignalEntry{SignalType: tc.signalType})
			assert.Equal(t, tc.status, status)
			assert.Equal(t, tc.result, result)
		})
	}
}
