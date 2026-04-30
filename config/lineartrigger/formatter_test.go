package lineartrigger

import (
	"strings"
	"testing"

	"github.com/kastheco/kasmos/config/taskstore"
	"github.com/stretchr/testify/assert"
)

func TestFormatHelpGolden(t *testing.T) {
	body := FormatHelp(HelpInput{
		EnabledVerbs: []Verb{VerbStart, VerbHelp, VerbPlan},
		Labels: LabelMap{
			Create: "00000000-0000-0000-0000-000000000001",
			Plan:   "00000000-0000-0000-0000-000000000002",
			Start:  "",
			Ack:    "00000000-0000-0000-0000-000000000003",
		},
	})

	assert.Equal(t, `kasmos trigger help

verbs:
- /kasmos help
- /kasmos plan
- /kasmos start

labels:
- ack: 00000000-0000-0000-0000-000000000003
- create: 00000000-0000-0000-0000-000000000001
- plan: 00000000-0000-0000-0000-000000000002`, body)
}

func TestFormatHelpDisabled(t *testing.T) {
	body := FormatHelp(HelpInput{})

	assert.Equal(t, `kasmos trigger help

verbs: none
linear triggers are disabled.
labels: none`, body)
}

func TestFormatStatusGolden(t *testing.T) {
	body := FormatStatus(StatusInput{
		Filename:       "linear-phase-4-guarded-linear-to-kasmos-triggers",
		Branch:         "linear/kas-123",
		Status:         taskstore.StatusReviewing,
		ExecutionPhase: "reviewing",
		ActiveAgent:    "reviewer",
		ActiveWave:     2,
		PRURL:          "https://github.com/kastheco/kasmos/pull/123",
		ReviewBody:     "needs a focused formatter test.",
		Identifier:     "KAS-123",
	})

	assert.Equal(t, `kasmos trigger status

task: linear-phase-4-guarded-linear-to-kasmos-triggers
branch: linear/kas-123
identifier: kas-123
status: reviewing
execution phase: reviewing
active agent: reviewer
active wave: 2
pr: https://github.com/kastheco/kasmos/pull/123

review notes:
needs a focused formatter test.`, body)
}

func TestFormatSuccessGolden(t *testing.T) {
	body := FormatSuccess(SuccessInput{
		Verb:       VerbPlan,
		Filename:   "linear-phase-4-guarded-linear-to-kasmos-triggers",
		Identifier: "KAS-123",
		Branch:     "linear/kas-123",
	})

	assert.Equal(t, `kasmos trigger ack

verb: plan
task: linear-phase-4-guarded-linear-to-kasmos-triggers
identifier: kas-123
branch: linear/kas-123`, body)
}

func TestFormatRejectGolden(t *testing.T) {
	body := FormatReject(RejectInput{
		Verb:   VerbStart,
		Reason: "route_missing",
	})

	assert.Equal(t, `kasmos trigger rejected

verb: start
reason: route_missing
hint: no [linear.triggers.routes] entry matched this issue's team/project/labels`, body)
}

func TestFormatRejectUsesCallerHint(t *testing.T) {
	body := FormatReject(RejectInput{
		Verb:   VerbPlan,
		Reason: "task_not_ready",
		Hint:   "wait for the current agent to finish.",
	})

	assert.Contains(t, body, "hint: wait for the current agent to finish.")
	assert.NotContains(t, body, "ready before starting")
}

func TestFormatStatusReviewBodyTruncatesAtRuneBoundary(t *testing.T) {
	short := strings.Repeat("é", triggerReviewBodyLimit)
	shortBody := FormatStatus(StatusInput{Filename: "task-file", ReviewBody: short})
	assert.Contains(t, shortBody, short)
	assert.NotContains(t, shortBody, triggerTruncatedMarker)

	long := strings.Repeat("é", triggerReviewBodyLimit+1)
	longBody := FormatStatus(StatusInput{Filename: "task-file", ReviewBody: long})
	want := strings.Repeat("é", triggerReviewBodyLimit) + triggerTruncatedMarker
	assert.Contains(t, longBody, want)
	assert.NotContains(t, longBody, strings.Repeat("é", triggerReviewBodyLimit+1))
}

func TestFormattersRedactPathsAndSecrets(t *testing.T) {
	bodies := []string{
		FormatHelp(HelpInput{
			EnabledVerbs: []Verb{VerbHelp},
			Labels:       LabelMap{Plan: "/home/kas/private/label-secret"},
		}),
		FormatStatus(StatusInput{
			Filename:   "/home/kas/dev/kasmos/task",
			Branch:     "feature/api_token=super-secret-token",
			ReviewBody: "failed at /tmp/build/output with password=hunter2",
		}),
		FormatSuccess(SuccessInput{
			Verb:     VerbCreate,
			Filename: "/users/kas/task",
			Branch:   "token: abc123",
		}),
		FormatReject(RejectInput{
			Verb:   VerbStart,
			Reason: "/var/tmp/route_missing",
			Hint:   "set api_key=linear-secret",
		}),
	}

	for _, body := range bodies {
		assert.NotContains(t, body, "/home/")
		assert.NotContains(t, body, "/users/")
		assert.NotContains(t, body, "/tmp/")
		assert.NotContains(t, body, "/var/")
		assert.NotContains(t, body, "super-secret-token")
		assert.NotContains(t, body, "hunter2")
		assert.NotContains(t, body, "linear-secret")
	}
}
