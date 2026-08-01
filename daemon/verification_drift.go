package daemon

import (
	"context"
	"errors"
	"fmt"

	"github.com/kastheco/kasmos/config/taskstore"
	"github.com/kastheco/kasmos/orchestration/loop"
	"github.com/kastheco/kasmos/session"
	gitpkg "github.com/kastheco/kasmos/session/git"
)

func staleReason(verified, head string) string {
	short := func(sha string) string {
		if len(sha) > 7 {
			return sha[:7]
		}
		return sha
	}
	return fmt.Sprintf("head_changed_after_verification: verified %s, head is now %s", short(verified), short(head))
}

func (d *Daemon) checkVerificationDrift(_ context.Context, e RepoEntry) []loop.Action {
	entries, err := e.Store.List(e.Project)
	if err != nil {
		d.logger.Warn("list tasks for verification drift failed", "repo", e.Path, "err", err)
		return nil
	}
	var actions []loop.Action
	for _, entry := range entries {
		if entry.Status != taskstore.StatusDone || entry.VerifiedSHA == "" {
			continue
		}
		branch := entry.Branch
		if branch == "" {
			branch = gitpkg.TaskBranchFromFile(entry.Filename)
		}
		head, _, reason, err := gitpkg.ValidateVerification(e.Path, branch, entry.VerifiedSHA, entry.VerifiedBaseSHA)
		if errors.Is(err, gitpkg.ErrBranchNotFound) {
			continue
		}
		if err != nil {
			d.logger.Warn("resolve branch head for verification drift failed", "plan", entry.Filename, "err", err)
			continue
		}
		if reason == "" && entry.PRURL != "" {
			remoteHead, remoteErr := gitpkg.RemoteBranchHeadSHA(e.Path, branch)
			if remoteErr == nil && remoteHead != entry.VerifiedSHA {
				// Inequality alone does not mean the head moved. Origin sitting
				// behind the verified commit is a push that has not landed yet,
				// not a change to review, and the ancestry check is what tells
				// the two apart. An error here is not an answer, so it falls
				// through to the strict reading rather than assuming either way.
				awaiting, awaitErr := gitpkg.RemoteAwaitingPush(e.Path, remoteHead, entry.VerifiedSHA)
				if awaitErr != nil {
					d.logger.Warn("compare remote head against verified sha failed", "plan", entry.Filename, "err", awaitErr)
				}
				if !awaiting {
					head = remoteHead
					reason = staleReason(entry.VerifiedSHA, remoteHead)
				}
			} else if remoteErr != nil && !errors.Is(remoteErr, gitpkg.ErrBranchNotFound) {
				d.logger.Warn("resolve remote branch head for verification drift failed", "plan", entry.Filename, "err", remoteErr)
				continue
			}
		}
		if reason == "" && entry.PRURL != "" && entry.VerifiedBaseSHA != "" {
			remoteBase, remoteErr := gitpkg.RemoteDefaultBranchHeadSHA(e.Path)
			if remoteErr != nil {
				d.logger.Warn("resolve remote default branch for verification drift failed", "plan", entry.Filename, "err", remoteErr)
				continue
			}
			if remoteBase != entry.VerifiedBaseSHA {
				reason = fmt.Sprintf("default_branch_changed_after_verification: verified %s, head is now %s", gitpkg.ShortSHA(entry.VerifiedBaseSHA), gitpkg.ShortSHA(remoteBase))
			}
		}
		if reason == "" {
			continue
		}
		// Last question before reopening a finished task: is there anything left
		// to reopen it for? A branch whose every commit already sits in the base
		// branch cannot produce a pull request, so clearing its verification buys
		// nothing and costs a full coder/reviewer/verifier round against work that
		// has already landed. This is deliberately the *last* check rather than
		// the first -- it fetches, and this function runs on every poll tick, so
		// asking it up front would put a fetch per finished task per second on the
		// wire. Down here it only runs when drift is otherwise about to fire.
		//
		// A transport failure must not read as "not merged", so only a clean true
		// skips; anything else falls through to the strict behaviour.
		if merged, mergedErr := gitpkg.BranchAlreadyMerged(e.Path, branch); mergedErr == nil && merged {
			continue
		} else if mergedErr != nil {
			d.logger.Warn("merged check for verification drift failed", "plan", entry.Filename, "err", mergedErr)
		}
		actions = append(actions,
			loop.StaleVerificationAction{PlanFile: entry.Filename, ReviewedSHA: entry.VerifiedSHA, CurrentSHA: head, Reason: reason},
			loop.PausePlanAgentAction{PlanFile: entry.Filename, AgentType: session.AgentTypeMaster},
			loop.SpawnMasterAction{PlanFile: entry.Filename},
		)
	}
	return actions
}
