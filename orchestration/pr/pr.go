package pr

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/kastheco/kasmos/config/taskfsm"
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
	RepoPath, Project, PlanFile, ReviewBody, BodyOverride, Title string
	Enabled, Manual                                              bool
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
	blockStale := func(reason string) (Result, error) {
		if clearErr := store.ClearVerification(req.Project, req.PlanFile, reason); clearErr != nil {
			return persist(OutcomeFailed, fmt.Sprintf("clear stale verification: %v", clearErr))
		}
		current, getErr := store.Get(req.Project, req.PlanFile)
		if getErr != nil {
			return persist(OutcomeFailed, fmt.Sprintf("reload stale task: %v", getErr))
		}
		if current.Status == taskstore.StatusDone {
			if transitionErr := taskfsm.New(store, req.Project, "").Transition(req.PlanFile, taskfsm.VerificationStale); transitionErr != nil {
				return persist(OutcomeFailed, fmt.Sprintf("reopen stale verification: %v", transitionErr))
			}
		}
		return persist(OutcomeBlocked, "verification stale: "+reason)
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
	branch := entry.Branch
	if branch == "" {
		derived := gitpkg.TaskBranchFromFile(req.PlanFile)
		local, remote := gitpkg.TaskBranchExists(req.RepoPath, derived)
		if !local && !remote {
			return persist(OutcomeBlocked, "no branch recorded for task and no derived branch exists")
		}
		branch = derived
	}
	// Nothing below this point has anything to do once the branch has landed:
	// every check between here and CreatePRAtSHA guards against opening a pull
	// request for an unverified commit, and there is no pull request left to
	// open. Asking them anyway is actively harmful -- a self-fix commit that the
	// master pushed after verification makes them report staleness, which clears
	// the verification and sends a merged task back for another full
	// coder/reviewer/verifier round. That is what put mvp-07 at review cycle 8
	// and ga-07 at 10 with their work already in the base branch.
	//
	// A transport failure here must not be mistaken for "not merged", so the
	// error path falls through to the existing checks rather than assuming.
	if merged, mergedErr := gitpkg.BranchAlreadyMerged(req.RepoPath, branch); mergedErr == nil && merged {
		// An already-recorded pull request keeps its existing contract: callers
		// rely on getting the URL back, so report it exactly as the normal
		// already-recorded path would rather than shadowing it.
		if entry.PRURL != "" {
			result, persistErr := persist(OutcomeSkipped, "pr already recorded")
			result.URL = entry.PRURL
			return result, persistErr
		}
		return persist(OutcomeSkipped, "branch already merged into pr base")
	}
	expectedHead, _, staleReason, verifyErr := gitpkg.ValidateVerification(req.RepoPath, branch, entry.VerifiedSHA, entry.VerifiedBaseSHA)
	if verifyErr != nil {
		return persist(OutcomeBlocked, verifyErr.Error())
	}
	if staleReason != "" {
		return blockStale(staleReason)
	}
	remoteHead, remoteErr := gitpkg.RemoteBranchHeadSHA(req.RepoPath, branch)
	if remoteErr != nil && !errors.Is(remoteErr, gitpkg.ErrBranchNotFound) {
		return persist(OutcomeBlocked, remoteErr.Error())
	}
	if remoteErr == nil && !strings.EqualFold(remoteHead, expectedHead) {
		// Origin behind the verified commit is a push in flight, not a branch
		// that moved: the agent commits in its worktree and pushes a beat later,
		// so this races on every ordinary task. Only a remote that is genuinely
		// ahead of, or divergent from, what was verified is stale.
		awaiting, awaitErr := gitpkg.RemoteAwaitingPush(req.RepoPath, remoteHead, expectedHead)
		if awaitErr != nil {
			return persist(OutcomeBlocked, awaitErr.Error())
		}
		if !awaiting {
			reason := fmt.Sprintf("remote branch moved after verification: expected %s, current %s", gitpkg.ShortSHA(expectedHead), gitpkg.ShortSHA(remoteHead))
			return blockStale(reason)
		}
	}
	remoteBase, remoteBaseErr := gitpkg.RemoteDefaultBranchHeadSHA(req.RepoPath)
	if remoteBaseErr != nil {
		return persist(OutcomeBlocked, remoteBaseErr.Error())
	}
	if !strings.EqualFold(remoteBase, entry.VerifiedBaseSHA) {
		reason := fmt.Sprintf("remote default branch moved after verification: expected %s, current %s", gitpkg.ShortSHA(entry.VerifiedBaseSHA), gitpkg.ShortSHA(remoteBase))
		return blockStale(reason)
	}
	handle := gitpkg.NewSharedTaskWorktree(req.RepoPath, branch)
	state, queryErr := handle.QueryPRState()
	if queryErr != nil {
		return persist(classify(queryErr), queryErr.Error())
	}
	if state.URL != "" {
		if state.HeadSHA == "" || !strings.EqualFold(state.HeadSHA, expectedHead) {
			reason := fmt.Sprintf("pull request head moved after verification: expected %s, current %s", gitpkg.ShortSHA(expectedHead), gitpkg.ShortSHA(state.HeadSHA))
			return blockStale(reason)
		}
		if entry.PRURL != "" {
			result, persistErr := persist(OutcomeSkipped, "pr already recorded")
			result.URL = entry.PRURL
			return result, persistErr
		}
		return adopt(store, req, state, entry.PRCreateAttempts+1)
	}
	if entry.PRURL != "" {
		return persist(OutcomeBlocked, "recorded pull request could not be resolved for verified branch")
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
	body := strings.TrimSpace(req.BodyOverride)
	if body == "" {
		body = gitpkg.BuildPRBody(meta)
	}
	// Revalidate immediately before pushing and create from the reviewed object.
	latest, latestErr := store.Get(req.Project, req.PlanFile)
	if latestErr != nil {
		return persist(OutcomeFailed, latestErr.Error())
	}
	currentHead, _, staleReason, verifyErr := gitpkg.ValidateVerification(req.RepoPath, branch, latest.VerifiedSHA, latest.VerifiedBaseSHA)
	if verifyErr != nil {
		return persist(OutcomeBlocked, verifyErr.Error())
	}
	if staleReason != "" || !strings.EqualFold(currentHead, expectedHead) {
		if staleReason == "" {
			staleReason = "head changed while preparing pull request"
		}
		return blockStale(staleReason)
	}
	latestRemoteBase, remoteBaseErr := gitpkg.RemoteDefaultBranchHeadSHA(req.RepoPath)
	if remoteBaseErr != nil {
		return persist(OutcomeBlocked, remoteBaseErr.Error())
	}
	if !strings.EqualFold(latestRemoteBase, latest.VerifiedBaseSHA) {
		reason := fmt.Sprintf("remote default branch moved while preparing pull request: expected %s, current %s", gitpkg.ShortSHA(latest.VerifiedBaseSHA), gitpkg.ShortSHA(latestRemoteBase))
		return blockStale(reason)
	}
	createErr := wt.CreatePRAtSHA(title, body, expectedHead)
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
	created, persistErr := persist(OutcomeCreated, "")
	if persistErr != nil {
		return Result{}, persistErr
	}
	// No review is posted here. Approving your own pull request is rejected by
	// GitHub, and the body passed to PostGitHubReview was the PR description
	// verbatim, so the call could only ever fail or duplicate. Approval is a
	// human step; kasmos opens the PR and stops.
	created.URL = state.URL
	created.Number = state.Number
	return created, nil
}

func adopt(store taskstore.Store, req Request, state gitpkg.PRState, attempts int) (Result, error) {
	if err := store.SetPRURL(req.Project, req.PlanFile, state.URL); err != nil {
		return Result{}, err
	}
	if err := store.SetPRCreateOutcome(req.Project, req.PlanFile, taskstore.PRCreateOutcome{State: string(OutcomeAdopted), Attempts: attempts, AttemptedAt: time.Now()}); err != nil {
		return Result{}, err
	}
	return Result{Outcome: OutcomeAdopted, URL: state.URL, Number: state.Number, Reason: "existing pr adopted"}, nil
}

func gitOutput(dir string, args ...string) (string, error) {
	out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput()
	return strings.TrimSpace(string(out)), err
}
