package lineartrigger

import (
	"testing"

	"github.com/kastheco/kasmos/config/taskstore"
	"github.com/kastheco/kasmos/internal/linear"
	"github.com/stretchr/testify/assert"
)

type fakeLinkedTaskFinder struct {
	filename string
	err      error
	calls    int
	statuses []taskstore.Status
}

func (f *fakeLinkedTaskFinder) FindLinkedTask(_ string, _ string, statuses ...taskstore.Status) (string, error) {
	f.calls++
	f.statuses = append([]taskstore.Status(nil), statuses...)
	return f.filename, f.err
}

func TestValidatorValidate(t *testing.T) {
	issue := linear.Issue{ID: "issue-1", Labels: []linear.Label{{ID: "start-label"}}}
	validContent := "**Goal:** ship it\n\n## Wave 1\n\n### Task 1: do it\n"

	t.Run("link requires existing task", func(t *testing.T) {
		result := NewValidator(Config{}, nil, "").Validate(VerbLink, taskstore.TaskEntry{}, issue)

		assert.False(t, result.OK)
		assert.Equal(t, "unlinked_target", result.Reason)
	})

	t.Run("link rejects task linked to another issue", func(t *testing.T) {
		entry := taskstore.TaskEntry{Filename: "task", LinearIssueID: "issue-2"}

		result := NewValidator(Config{}, nil, "").Validate(VerbLink, entry, issue)

		assert.False(t, result.OK)
		assert.Equal(t, "already_linked", result.Reason)
	})

	t.Run("create detects duplicate linked active task", func(t *testing.T) {
		store := &fakeLinkedTaskFinder{filename: "existing"}

		result := NewValidator(Config{}, store, "kasmos").Validate(VerbCreate, taskstore.TaskEntry{}, issue)

		assert.False(t, result.OK)
		assert.Equal(t, "duplicate_link", result.Reason)
		assert.Equal(t, 1, store.calls)
		assert.Equal(t, []taskstore.Status{
			taskstore.StatusReady,
			taskstore.StatusPlanning,
			taskstore.StatusImplementing,
			taskstore.StatusReviewing,
			taskstore.StatusVerifying,
		}, store.statuses)
	})

	t.Run("plan rejects implementing task", func(t *testing.T) {
		entry := taskstore.TaskEntry{
			Filename:      "task",
			Status:        taskstore.StatusImplementing,
			LinearIssueID: "issue-1",
			Content:       validContent,
		}

		result := NewValidator(Config{}, nil, "").Validate(VerbPlan, entry, issue)

		assert.False(t, result.OK)
		assert.Equal(t, "invalid_transition", result.Reason)
		assert.Equal(t, taskstore.StatusImplementing, result.CurrentStatus)
	})

	t.Run("plan requires content", func(t *testing.T) {
		entry := taskstore.TaskEntry{
			Filename:      "task",
			Status:        taskstore.StatusReady,
			LinearIssueID: "issue-1",
		}

		result := NewValidator(Config{}, nil, "").Validate(VerbPlan, entry, issue)

		assert.False(t, result.OK)
		assert.Equal(t, "missing_plan_content", result.Reason)
	})

	t.Run("start rejects unparseable content without wave header", func(t *testing.T) {
		entry := taskstore.TaskEntry{
			Filename:      "task",
			Status:        taskstore.StatusReady,
			LinearIssueID: "issue-1",
			Content:       "**Goal:** not enough",
		}

		result := NewValidator(Config{}, nil, "").Validate(VerbStart, entry, issue)

		assert.False(t, result.OK)
		assert.Equal(t, "unparseable_plan", result.Reason)
	})

	t.Run("start requires start label when guard is enabled", func(t *testing.T) {
		entry := taskstore.TaskEntry{
			Filename:      "task",
			Status:        taskstore.StatusReady,
			LinearIssueID: "issue-1",
			Content:       validContent,
		}
		cfg := Config{StartGuard: StartGuard{RequireStartLabel: true}, Labels: LabelMap{Start: "start-label"}}

		result := NewValidator(cfg, nil, "").Validate(VerbStart, entry, linear.Issue{ID: "issue-1"})

		assert.False(t, result.OK)
		assert.Equal(t, "missing_start_label", result.Reason)
	})

	t.Run("start accepts ready linked parseable task with start label", func(t *testing.T) {
		entry := taskstore.TaskEntry{
			Filename:      "task",
			Status:        taskstore.StatusReady,
			LinearIssueID: "issue-1",
			Content:       validContent,
		}
		cfg := Config{StartGuard: StartGuard{RequireStartLabel: true}, Labels: LabelMap{Start: "start-label"}}

		result := NewValidator(cfg, nil, "").Validate(VerbStart, entry, issue)

		assert.True(t, result.OK)
		assert.Empty(t, result.Reason)
	})
}
