package daemon

import (
	"context"
	"errors"
	"fmt"
	"strings"

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
		if entry.Status != taskstore.StatusDone || entry.Branch == "" || entry.VerifiedSHA == "" {
			continue
		}
		head, err := gitpkg.BranchHeadSHA(e.Path, entry.Branch)
		if errors.Is(err, gitpkg.ErrBranchNotFound) {
			continue
		}
		if err != nil {
			d.logger.Warn("resolve branch head for verification drift failed", "plan", entry.Filename, "err", err)
			continue
		}
		if strings.EqualFold(head, entry.VerifiedSHA) {
			continue
		}
		actions = append(actions,
			loop.StaleVerificationAction{PlanFile: entry.Filename, ReviewedSHA: entry.VerifiedSHA, CurrentSHA: head, Reason: staleReason(entry.VerifiedSHA, head)},
			loop.PausePlanAgentAction{PlanFile: entry.Filename, AgentType: session.AgentTypeMaster},
			loop.SpawnMasterAction{PlanFile: entry.Filename},
		)
	}
	return actions
}
