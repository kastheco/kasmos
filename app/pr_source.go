package app

import (
	"fmt"

	"github.com/kastheco/kasmos/config/taskstate"
	"github.com/kastheco/kasmos/session"
	gitpkg "github.com/kastheco/kasmos/session/git"
)

type prSource struct {
	worktree   *gitpkg.GitWorktree
	planFile   string
	branch     string
	title      string
	needsSetup bool
}

func (m *home) taskPRSource(planFile string) (prSource, error) {
	if planFile == "" {
		return prSource{}, fmt.Errorf("no task selected")
	}

	entry, ok := m.refreshTaskEntry(planFile)
	if !ok {
		return prSource{}, fmt.Errorf("task '%s' not found", taskstate.DisplayName(planFile))
	}

	title := gitpkg.BuildPRTitle(entry.Description, taskstate.DisplayName(planFile))
	if entry.Branch != "" {
		return prSource{
			worktree:   gitpkg.NewSharedTaskWorktree(m.activeRepoPath, entry.Branch),
			planFile:   planFile,
			branch:     entry.Branch,
			title:      title,
			needsSetup: true,
		}, nil
	}

	for _, inst := range m.nav.GetInstances() {
		if inst.TaskFile != planFile {
			continue
		}
		wt, err := inst.GetGitWorktree()
		if err != nil || wt.GetBranchName() == "" {
			continue
		}
		return prSource{
			worktree: wt,
			planFile: planFile,
			branch:   wt.GetBranchName(),
			title:    title,
		}, nil
	}

	return prSource{}, fmt.Errorf("task '%s' has no branch — nothing to open a pr from", taskstate.DisplayName(planFile))
}

func (m *home) instancePRSource(inst *session.Instance) (prSource, error) {
	if inst == nil {
		return prSource{}, fmt.Errorf("no session selected")
	}

	wt, err := inst.GetGitWorktree()
	if err == nil && wt.GetBranchName() != "" {
		return prSource{
			worktree: wt,
			planFile: inst.TaskFile,
			branch:   wt.GetBranchName(),
			title:    inst.Title,
		}, nil
	}
	if inst.TaskFile != "" {
		return m.taskPRSource(inst.TaskFile)
	}

	return prSource{}, fmt.Errorf("session '%s' has no branch — nothing to open a pr from", inst.Title)
}
