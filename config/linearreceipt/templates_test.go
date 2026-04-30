package linearreceipt

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestFormatPROpened(t *testing.T) {
	body := FormatPROpened(PRInput{
		Project:    "kasmos",
		Filename:   "task-file",
		Branch:     "branch-name",
		Identifier: "KAS-123",
		PRURL:      "https://github.com/kastheco/kasmos/pull/123",
		When:       time.Date(2026, 4, 30, 1, 2, 3, 0, time.UTC),
	})

	assert.Equal(t, `kasmos pr receipt

task: task-file
project: kasmos
branch: branch-name
identifier: KAS-123
pr: https://github.com/kastheco/kasmos/pull/123
time: 2026-04-30T01:02:03Z`, body)
}

func TestFormatMerged(t *testing.T) {
	body := FormatMerged(MergeInput{
		Project:    "kasmos",
		Filename:   "task-file",
		Branch:     "branch-name",
		Identifier: "KAS-123",
		MergeRef:   "main@abc123",
		When:       time.Date(2026, 4, 30, 1, 2, 3, 0, time.UTC),
	})

	assert.Equal(t, `kasmos merge receipt

task: task-file
project: kasmos
branch: branch-name
identifier: KAS-123
merge: main@abc123
time: 2026-04-30T01:02:03Z`, body)
}

func TestFormatCancelled(t *testing.T) {
	body := FormatCancelled(CancelInput{
		Project:  "kasmos",
		Filename: "task-file",
		Reason:   "operator cancelled from Linear.",
		When:     time.Date(2026, 4, 30, 1, 2, 3, 0, time.UTC),
	})

	assert.Equal(t, `kasmos cancellation receipt

task: task-file
project: kasmos
identifier: task-file
reason: operator cancelled from Linear.
time: 2026-04-30T01:02:03Z`, body)
}

func TestUTCFormatting(t *testing.T) {
	when := time.Date(2026, 4, 29, 20, 2, 3, 0, time.FixedZone("local", -5*60*60))

	body := FormatPROpened(PRInput{
		Filename: "task-file",
		When:     when,
	})

	assert.Contains(t, body, "time: 2026-04-30T01:02:03Z")
	assert.NotContains(t, body, "<no value>")
}
