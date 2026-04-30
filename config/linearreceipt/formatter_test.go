package linearreceipt

import (
	"strings"
	"testing"
	"time"

	"github.com/kastheco/kasmos/config/taskfsm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFormatLifecycleGolden(t *testing.T) {
	when := time.Date(2026, 4, 30, 15, 4, 5, 123, time.FixedZone("local", -5*60*60))

	body := FormatLifecycle(LifecycleInput{
		Project:    "kasmos",
		Filename:   "linear-phase-3-kasmos-to-linear-updates",
		Branch:     "task-2-receipts",
		Identifier: "KAS-123",
		Event:      taskfsm.ReviewChangesRequested,
		From:       taskfsm.StatusReviewing,
		To:         taskfsm.StatusImplementing,
		PRURL:      "https://github.com/kastheco/kasmos/pull/123",
		ReviewBody: "keep operator text as-is for KAS-123 and URLs.",
		When:       when,
	})

	assert.Equal(t, `kasmos status receipt

task: linear-phase-3-kasmos-to-linear-updates
project: kasmos
branch: task-2-receipts
identifier: KAS-123
event: review_changes_requested
status: reviewing -> implementing
pr: https://github.com/kastheco/kasmos/pull/123

review notes:
keep operator text as-is for KAS-123 and URLs.
time: 2026-04-30T20:04:05Z`, body)
}

func TestFormatLifecycleFallbacksAndOptionalPR(t *testing.T) {
	body := FormatLifecycle(LifecycleInput{
		Project:  "kasmos",
		Filename: "task-file",
		Event:    taskfsm.PlanStart,
		From:     taskfsm.StatusReady,
		To:       taskfsm.StatusPlanning,
		When:     time.Date(2026, 4, 30, 1, 2, 3, 0, time.UTC),
	})

	assert.Contains(t, body, "branch: (no branch)")
	assert.Contains(t, body, "identifier: task-file")
	assert.NotContains(t, body, "\npr: ")
	assert.NotContains(t, body, "<no value>")
}

func TestFormatLifecycleReviewBodyTruncatesAtRuneBoundary(t *testing.T) {
	short := strings.Repeat("é", ReviewBodyLimit)
	shortBody := FormatLifecycle(LifecycleInput{
		Filename:   "task-file",
		ReviewBody: short,
		When:       time.Date(2026, 4, 30, 1, 2, 3, 0, time.UTC),
	})
	assert.Contains(t, shortBody, short)
	assert.NotContains(t, shortBody, truncatedMarker)

	long := strings.Repeat("é", ReviewBodyLimit+1)
	longBody := FormatLifecycle(LifecycleInput{
		Filename:   "task-file",
		ReviewBody: long,
		When:       time.Date(2026, 4, 30, 1, 2, 3, 0, time.UTC),
	})
	want := strings.Repeat("é", ReviewBodyLimit) + truncatedMarker
	assert.Contains(t, longBody, want)
	assert.NotContains(t, longBody, strings.Repeat("é", ReviewBodyLimit+1))
}

func TestFormatLifecycleKnownTransitionCombinationsRender(t *testing.T) {
	transitions := []struct {
		from  taskfsm.Status
		event taskfsm.Event
		to    taskfsm.Status
	}{
		{taskfsm.StatusReady, taskfsm.PlanStart, taskfsm.StatusPlanning},
		{taskfsm.StatusReady, taskfsm.ImplementStart, taskfsm.StatusImplementing},
		{taskfsm.StatusReady, taskfsm.MarkDone, taskfsm.StatusDone},
		{taskfsm.StatusReady, taskfsm.Cancel, taskfsm.StatusCancelled},
		{taskfsm.StatusPlanning, taskfsm.PlanStart, taskfsm.StatusPlanning},
		{taskfsm.StatusPlanning, taskfsm.PlannerFinished, taskfsm.StatusReady},
		{taskfsm.StatusPlanning, taskfsm.Cancel, taskfsm.StatusCancelled},
		{taskfsm.StatusImplementing, taskfsm.ImplementFinished, taskfsm.StatusReviewing},
		{taskfsm.StatusImplementing, taskfsm.Cancel, taskfsm.StatusCancelled},
		{taskfsm.StatusReviewing, taskfsm.ReviewApproved, taskfsm.StatusVerifying},
		{taskfsm.StatusReviewing, taskfsm.ReviewChangesRequested, taskfsm.StatusImplementing},
		{taskfsm.StatusReviewing, taskfsm.Cancel, taskfsm.StatusCancelled},
		{taskfsm.StatusVerifying, taskfsm.VerifyApproved, taskfsm.StatusDone},
		{taskfsm.StatusVerifying, taskfsm.VerifyFailed, taskfsm.StatusImplementing},
		{taskfsm.StatusVerifying, taskfsm.Cancel, taskfsm.StatusCancelled},
		{taskfsm.StatusDone, taskfsm.StartOver, taskfsm.StatusPlanning},
		{taskfsm.StatusDone, taskfsm.Reimplement, taskfsm.StatusImplementing},
		{taskfsm.StatusDone, taskfsm.RequestReview, taskfsm.StatusReviewing},
		{taskfsm.StatusDone, taskfsm.Cancel, taskfsm.StatusCancelled},
		{taskfsm.StatusCancelled, taskfsm.Reopen, taskfsm.StatusPlanning},
	}

	for _, tt := range transitions {
		t.Run(string(tt.from)+"_"+string(tt.event), func(t *testing.T) {
			require.Equal(t, tt.to, mustApplyTransition(t, tt.from, tt.event))
			body := FormatLifecycle(LifecycleInput{
				Project:  "kasmos",
				Filename: "task-file",
				Event:    tt.event,
				From:     tt.from,
				To:       tt.to,
				When:     time.Date(2026, 4, 30, 1, 2, 3, 0, time.UTC),
			})
			assert.Contains(t, body, "event: "+string(tt.event))
			assert.Contains(t, body, "status: "+string(tt.from)+" -> "+string(tt.to))
		})
	}
}

func mustApplyTransition(t *testing.T, from taskfsm.Status, event taskfsm.Event) taskfsm.Status {
	t.Helper()
	to, err := taskfsm.ApplyTransition(from, event)
	require.NoError(t, err)
	return to
}
