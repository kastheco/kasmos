package app

import (
	"testing"

	"charm.land/bubbles/v2/spinner"
	"github.com/kastheco/kasmos/config/taskstate"
	"github.com/kastheco/kasmos/session"
	gitpkg "github.com/kastheco/kasmos/session/git"
	"github.com/kastheco/kasmos/ui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newPRSourceHome(t *testing.T, entry taskstate.TaskEntry) *home {
	t.Helper()
	repo := t.TempDir()
	s := spinner.New()
	return &home{
		activeRepoPath: repo,
		nav:            ui.NewNavigationPanel(&s),
		taskState: &taskstate.TaskState{Plans: map[string]taskstate.TaskEntry{
			"completed-task": entry,
		}},
	}
}

func newPRSourceInstance(t *testing.T, title, taskFile, repo, branch string, started bool) *session.Instance {
	t.Helper()
	inst, err := session.NewInstance(session.InstanceOptions{Title: title, Program: "opencode", TaskFile: taskFile})
	require.NoError(t, err)
	if branch != "" {
		inst.BindSharedTaskWorktree(repo, branch)
	}
	if started {
		inst.MarkStartedForTest()
	}
	return inst
}

func TestTaskPRSourceUsesTaskMetadataRegardlessOfInstanceLiveness(t *testing.T) {
	tests := []struct {
		name      string
		instances func(*testing.T, *home) []*session.Instance
	}{
		{name: "no instances"},
		{name: "never-started daemon placeholder", instances: func(t *testing.T, h *home) []*session.Instance {
			return []*session.Instance{newPRSourceInstance(t, "placeholder", "completed-task", h.activeRepoPath, "task/completed", false)}
		}},
		{name: "exited instance", instances: func(t *testing.T, h *home) []*session.Instance {
			inst := newPRSourceInstance(t, "exited", "completed-task", h.activeRepoPath, "task/completed", true)
			inst.Exited = true
			return []*session.Instance{inst}
		}},
		{name: "live instance", instances: func(t *testing.T, h *home) []*session.Instance {
			return []*session.Instance{newPRSourceInstance(t, "live", "completed-task", h.activeRepoPath, "task/completed", true)}
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newPRSourceHome(t, taskstate.TaskEntry{Branch: "task/completed", Description: "Completed task"})
			if tt.instances != nil {
				for _, inst := range tt.instances(t, h) {
					h.nav.AddInstance(inst)
				}
			}

			source, err := h.taskPRSource("completed-task")
			require.NoError(t, err)
			assert.Equal(t, "task/completed", source.branch)
			assert.Equal(t, gitpkg.TaskWorktreePath(h.activeRepoPath, "task/completed"), source.worktree.GetWorktreePath())
			assert.True(t, source.needsSetup)
			assert.NotContains(t, errString(err), "has not been started")
		})
	}
}

func TestTaskPRSourceFailsClosedWithoutBranch(t *testing.T) {
	h := newPRSourceHome(t, taskstate.TaskEntry{})
	_, err := h.taskPRSource("completed-task")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "completed-task")
	assert.Contains(t, err.Error(), "has no branch")
}

func TestInstancePRSourceFallsBackToTask(t *testing.T) {
	h := newPRSourceHome(t, taskstate.TaskEntry{Branch: "task/completed"})
	inst := newPRSourceInstance(t, "task session", "completed-task", h.activeRepoPath, "", false)

	source, err := h.instancePRSource(inst)
	require.NoError(t, err)
	assert.Equal(t, "task/completed", source.branch)
	assert.True(t, source.needsSetup)

	solo := newPRSourceInstance(t, "solo session", "", h.activeRepoPath, "", false)
	_, err = h.instancePRSource(solo)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "solo session")
}

func TestTaskPRSourceDeadSiblingDoesNotShadowLiveOne(t *testing.T) {
	h := newPRSourceHome(t, taskstate.TaskEntry{})
	dead := newPRSourceInstance(t, "dead", "completed-task", h.activeRepoPath, "task/live", false)
	live := newPRSourceInstance(t, "live", "completed-task", h.activeRepoPath, "task/live", true)
	h.nav.AddInstance(dead)
	h.nav.AddInstance(live)

	source, err := h.taskPRSource("completed-task")
	require.NoError(t, err)
	assert.Same(t, mustPRSourceWorktree(t, live), source.worktree)
}

func TestTaskPRSourceIsPure(t *testing.T) {
	h := newPRSourceHome(t, taskstate.TaskEntry{Branch: "task/completed"})

	source, err := h.taskPRSource("completed-task")
	require.NoError(t, err)
	assert.Equal(t, gitpkg.TaskWorktreePath(h.activeRepoPath, "task/completed"), source.worktree.GetWorktreePath())
}

func mustPRSourceWorktree(t *testing.T, inst *session.Instance) *gitpkg.GitWorktree {
	t.Helper()
	wt, err := inst.GetGitWorktree()
	require.NoError(t, err)
	return wt
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
