package cmd

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kastheco/kasmos/config/taskstore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExecuteTaskUpdateContent(t *testing.T) {
	t.Run("ingests stdin and updates parsed metadata", func(t *testing.T) {
		store := taskstore.NewTestSQLiteStore(t)
		project := "test-project"
		err := store.Create(project, taskstore.TaskEntry{
			Filename:  "my-plan",
			Status:    taskstore.StatusReady,
			Branch:    "plan/my-plan",
			CreatedAt: time.Now(),
		})
		require.NoError(t, err)

		content := "# Updated Plan\n\n**Goal:** new goal\n\n## Wave 1\n\n### Task 1: foo\n\nDo it.\n"
		err = executeTaskUpdateContent(project, "my-plan", strings.NewReader(content), store)
		require.NoError(t, err)

		got, err := store.GetContent(project, "my-plan")
		require.NoError(t, err)
		assert.Equal(t, content, got)

		entry, err := store.Get(project, "my-plan")
		require.NoError(t, err)
		assert.Equal(t, "new goal", entry.Goal)

		subtasks, err := store.GetSubtasks(project, "my-plan")
		require.NoError(t, err)
		require.Len(t, subtasks, 1)
		assert.Equal(t, 1, subtasks[0].TaskNumber)
		assert.Equal(t, "foo", subtasks[0].Title)
		assert.Equal(t, taskstore.SubtaskStatusPending, subtasks[0].Status)
	})

	t.Run("accepts md suffix as compatibility input", func(t *testing.T) {
		store := taskstore.NewTestSQLiteStore(t)
		project := "test-project"
		require.NoError(t, store.Create(project, taskstore.TaskEntry{
			Filename:  "my-plan",
			Status:    taskstore.StatusReady,
			CreatedAt: time.Now(),
		}))

		content := "# Updated Plan\n\n## Wave 1\n\n### Task 1: foo\n"
		err := executeTaskUpdateContent(project, "my-plan.md", strings.NewReader(content), store)
		require.NoError(t, err)

		got, err := store.GetContent(project, "my-plan")
		require.NoError(t, err)
		assert.Equal(t, content, got)
	})

	t.Run("prefers exact stored md filename when present", func(t *testing.T) {
		store := taskstore.NewTestSQLiteStore(t)
		project := "test-project"
		require.NoError(t, store.Create(project, taskstore.TaskEntry{
			Filename:  "my-plan.md",
			Status:    taskstore.StatusReady,
			CreatedAt: time.Now(),
		}))

		content := "# Updated Plan\n\n## Wave 1\n\n### Task 1: foo\n"
		err := executeTaskUpdateContent(project, "my-plan.md", strings.NewReader(content), store)
		require.NoError(t, err)

		got, err := store.GetContent(project, "my-plan.md")
		require.NoError(t, err)
		assert.Equal(t, content, got)
	})

	t.Run("stores draft content without wave warning and clears stale subtasks", func(t *testing.T) {
		store := taskstore.NewTestSQLiteStore(t)
		project := "test-project"
		require.NoError(t, store.Create(project, taskstore.TaskEntry{
			Filename:  "my-plan",
			Status:    taskstore.StatusReady,
			CreatedAt: time.Now(),
		}))
		require.NoError(t, store.SetSubtasks(project, "my-plan", []taskstore.SubtaskEntry{{
			TaskNumber: 1,
			Title:      "stale",
			Status:     taskstore.SubtaskStatusDone,
		}}))

		// Valid markdown but no Wave sections — typical during early drafting.
		draftContent := "# My Plan\n\n**Goal:** in progress\n"
		err := executeTaskUpdateContent(project, "my-plan", strings.NewReader(draftContent), store)
		require.NoError(t, err)

		got, err := store.GetContent(project, "my-plan")
		require.NoError(t, err)
		assert.Equal(t, draftContent, got, "content must still be persisted")

		entry, err := store.Get(project, "my-plan")
		require.NoError(t, err)
		assert.Equal(t, "in progress", entry.Goal)

		subtasks, err := store.GetSubtasks(project, "my-plan")
		require.NoError(t, err)
		assert.Empty(t, subtasks)
	})

	t.Run("rejects empty stdin", func(t *testing.T) {
		store := taskstore.NewTestSQLiteStore(t)
		project := "test-project"
		require.NoError(t, store.Create(project, taskstore.TaskEntry{
			Filename:  "my-plan",
			Status:    taskstore.StatusReady,
			CreatedAt: time.Now(),
		}))

		err := executeTaskUpdateContent(project, "my-plan", strings.NewReader(" \n"), store)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no content provided")
	})

	t.Run("rejects tty stdin", func(t *testing.T) {
		stdinFile, err := os.Open("/dev/null")
		require.NoError(t, err)
		t.Cleanup(func() { _ = stdinFile.Close() })

		err = validateUpdateContentStdin(stdinFile)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "stdin is a tty")
	})

	t.Run("per-wave task numbers are renumbered globally", func(t *testing.T) {
		store := taskstore.NewTestSQLiteStore(t)
		project := "test-project"
		err := store.Create(project, taskstore.TaskEntry{
			Filename:  "my-plan",
			Status:    taskstore.StatusReady,
			CreatedAt: time.Now(),
		})
		require.NoError(t, err)

		// Both waves use Task 1 / Task 2 — the parser must renumber them 1..4.
		content := strings.Join([]string{
			"# Per-Wave Plan",
			"",
			"**Goal:** global renumber",
			"",
			"## Wave 1",
			"",
			"### Task 1: alpha",
			"",
			"Do alpha.",
			"",
			"### Task 2: beta",
			"",
			"Do beta.",
			"",
			"## Wave 2",
			"",
			"### Task 1: gamma",
			"",
			"Do gamma.",
			"",
			"### Task 2: delta",
			"",
			"Do delta.",
			"",
		}, "\n")

		err = executeTaskUpdateContent(project, "my-plan", strings.NewReader(content), store)
		require.NoError(t, err)

		subtasks, err := store.GetSubtasks(project, "my-plan")
		require.NoError(t, err)
		require.Len(t, subtasks, 4)
		assert.Equal(t, 1, subtasks[0].TaskNumber)
		assert.Equal(t, "alpha", subtasks[0].Title)
		assert.Equal(t, 2, subtasks[1].TaskNumber)
		assert.Equal(t, "beta", subtasks[1].Title)
		assert.Equal(t, 3, subtasks[2].TaskNumber)
		assert.Equal(t, "gamma", subtasks[2].Title)
		assert.Equal(t, 4, subtasks[3].TaskNumber)
		assert.Equal(t, "delta", subtasks[3].Title)
		for _, st := range subtasks {
			assert.Equal(t, taskstore.SubtaskStatusPending, st.Status)
		}
	})

	t.Run("uses file reader when provided", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "plan.md")
		require.NoError(t, os.WriteFile(path, []byte("# plan\n"), 0o644))

		reader, err := openUpdateContentReader(os.Stdin, path)
		require.NoError(t, err)
		t.Cleanup(func() { _ = reader.Close() })

		body, err := io.ReadAll(reader)
		require.NoError(t, err)
		assert.Equal(t, "# plan\n", string(body))
	})
}
