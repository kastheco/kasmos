package pr

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"time"

	"github.com/kastheco/kasmos/config/taskstate"
	"github.com/kastheco/kasmos/config/taskstore"
	gitpkg "github.com/kastheco/kasmos/session/git"
)

type Outcome string

const (
	OutcomeCreated Outcome = "created"
	OutcomeAdopted Outcome = "adopted"
	OutcomeSkipped Outcome = "skipped"
	OutcomeFailed  Outcome = "failed"
	OutcomeBlocked Outcome = "blocked"
)

type Request struct {
	RepoPath, Project, PlanFile, ReviewBody, Title string
	Enabled, Manual                                bool
}

type Result struct {
	Outcome Outcome
	URL     string
	Number  int
	Reason  string
}

func Eligible(entry taskstore.TaskEntry) bool {
	return entry.Status == taskstore.StatusDone
}

// IsGHUnavailable reports terminal local gh configuration failures.
func IsGHUnavailable(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	for _, marker := range []string{"executable file not found", "not found in $path", "exit status 127", "not logged in", "not authenticated", "authentication token", "github_token"} {
		if strings.Contains(s, marker) {
			return true
		}
	}
	return errors.Is(err, exec.ErrNotFound)
}

func classify(err error) Outcome {
	if errors.Is(err, gitpkg.ErrWorktreeDirty) || IsGHUnavailable(err) {
		return OutcomeBlocked
	}
	return OutcomeFailed
}

func Ensure(ctx context.Context, store taskstore.Store, req Request) (res Result, err error) {
	_ = ctx
	entry := taskstore.TaskEntry{}
	persist := func(outcome Outcome, reason string) (Result, error) {
		res = Result{Outcome: outcome, Reason: reason}
		attempts := entry.PRCreateAttempts + 1
		if store != nil {
			if saveErr := store.SetPRCreateOutcome(req.Project, req.PlanFile, taskstore.PRCreateOutcome{State: string(outcome), Error: reason, Attempts: attempts, AttemptedAt: time.Now()}); saveErr != nil {
				return res, fmt.Errorf("persist pr creation outcome: %w", saveErr)
			}
		}
		return res, nil
	}
	if store == nil {
		return Result{Outcome: OutcomeFailed, Reason: "task store unavailable"}, fmt.Errorf("task store unavailable")
	}
	if !req.Enabled && !req.Manual {
		return persist(OutcomeSkipped, "auto pr disabled by config")
	}
	entry, err = store.Get(req.Project, req.PlanFile)
	if err != nil {
		return persist(OutcomeFailed, fmt.Sprintf("load task entry: %v", err))
	}
	if entry.PRURL != "" {
		return persist(OutcomeSkipped, "pr already recorded")
	}

	branch := entry.Branch
	if branch == "" {
		derived := gitpkg.TaskBranchFromFile(req.PlanFile)
		local, remote := gitpkg.TaskBranchExists(req.RepoPath, derived)
		if !local && !remote {
			return persist(OutcomeBlocked, "no branch recorded for task and no derived branch exists")
		}
		branch = derived
	}
	handle := gitpkg.NewSharedTaskWorktree(req.RepoPath, branch)
	state, queryErr := handle.QueryPRState()
	if queryErr != nil {
		return persist(classify(queryErr), queryErr.Error())
	}
	if state.URL != "" {
		return adopt(store, req, state)
	}

	wt, setupErr := gitpkg.EnsureTaskWorktree(req.RepoPath, branch)
	if setupErr != nil {
		return persist(classify(setupErr), setupErr.Error())
	}
	subtasks, _ := store.GetSubtasks(req.Project, req.PlanFile)
	base := wt.GetBaseCommitSHA()
	changes, commits, stats := "", "", ""
	if base != "" {
		changes, _ = gitOutput(wt.GetWorktreePath(), "diff", "--name-only", base)
		commits, _ = gitOutput(wt.GetWorktreePath(), "log", "--oneline", base+"..HEAD")
		stats, _ = gitOutput(wt.GetWorktreePath(), "diff", "--stat", base)
	}
	meta := gitpkg.AssemblePRMetadata(entry, subtasks, req.ReviewBody, entry.ReviewCycle, changes, commits, stats)
	title := req.Title
	if title == "" {
		title = gitpkg.BuildPRTitle(entry.Description, taskstate.DisplayName(req.PlanFile))
	}
	body := gitpkg.BuildPRBody(meta)
	createErr := wt.CreatePR(title, body)
	if createErr != nil && !errors.Is(createErr, gitpkg.ErrPRAlreadyExists) {
		return persist(classify(createErr), createErr.Error())
	}
	state, queryErr = wt.QueryPRState()
	if queryErr != nil {
		return persist(classify(queryErr), queryErr.Error())
	}
	if state.URL == "" {
		return persist(OutcomeFailed, "empty pr url after creation")
	}
	if saveErr := store.SetPRURL(req.Project, req.PlanFile, state.URL); saveErr != nil {
		return persist(OutcomeFailed, saveErr.Error())
	}
	if clearErr := store.ClearPRCreateOutcome(req.Project, req.PlanFile); clearErr != nil {
		return Result{}, clearErr
	}
	if state.Number > 0 {
		if reviewErr := wt.PostGitHubReview(state.Number, body, true); reviewErr != nil {
			log.Printf("post approving review: %v", reviewErr)
		}
	}
	return Result{Outcome: OutcomeCreated, URL: state.URL, Number: state.Number}, nil
}

func adopt(store taskstore.Store, req Request, state gitpkg.PRState) (Result, error) {
	if err := store.SetPRURL(req.Project, req.PlanFile, state.URL); err != nil {
		return Result{}, err
	}
	if err := store.ClearPRCreateOutcome(req.Project, req.PlanFile); err != nil {
		return Result{}, err
	}
	return Result{Outcome: OutcomeAdopted, URL: state.URL, Number: state.Number, Reason: "existing pr adopted"}, nil
}

func gitOutput(dir string, args ...string) (string, error) {
	out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput()
	return strings.TrimSpace(string(out)), err
}
