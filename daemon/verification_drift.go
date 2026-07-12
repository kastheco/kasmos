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
				head = remoteHead
				reason = staleReason(entry.VerifiedSHA, remoteHead)
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
		actions = append(actions,
			loop.StaleVerificationAction{PlanFile: entry.Filename, ReviewedSHA: entry.VerifiedSHA, CurrentSHA: head, Reason: reason},
			loop.PausePlanAgentAction{PlanFile: entry.Filename, AgentType: session.AgentTypeMaster},
			loop.SpawnMasterAction{PlanFile: entry.Filename},
		)
	}
	return actions
}
