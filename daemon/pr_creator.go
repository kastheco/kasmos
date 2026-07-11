package daemon

import (
	"context"
	"log/slog"
	"os/exec"
	"sync/atomic"
	"time"

	"github.com/kastheco/kasmos/config/taskstore"
	"github.com/kastheco/kasmos/orchestration/loop"
	prsvc "github.com/kastheco/kasmos/orchestration/pr"
)

// PRCreator retries transient PR creation failures with bounded exponential backoff.
type PRCreator struct {
	cfg                 PRCreatorConfig
	repos               *RepoManager
	logger              *slog.Logger
	dispatch            dispatchFunc
	now                 func() time.Time
	checkGH             func(context.Context) error
	ghUnavailableLogged atomic.Bool
}

func NewPRCreator(cfg PRCreatorConfig, repos *RepoManager, logger *slog.Logger, dispatch dispatchFunc) *PRCreator {
	defaults := defaultDaemonConfig().PRCreator
	if cfg.RetryInterval <= 0 {
		cfg.RetryInterval = defaults.RetryInterval
	}
	if cfg.MaxAttempts <= 0 {
		cfg.MaxAttempts = defaults.MaxAttempts
	}
	return &PRCreator{
		cfg: cfg, repos: repos, logger: logger, dispatch: dispatch,
		now: time.Now,
		checkGH: func(ctx context.Context) error {
			return exec.CommandContext(ctx, "gh", "auth", "status").Run()
		},
	}
}

// Run performs an immediate sweep, then repeats until the context is cancelled.
func (c *PRCreator) Run(ctx context.Context) error {
	ticker := time.NewTicker(c.cfg.RetryInterval)
	defer ticker.Stop()
	c.sweepOnce(ctx)
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			c.sweepOnce(ctx)
		}
	}
}

func (c *PRCreator) sweepOnce(ctx context.Context) {
	if err := c.checkGH(ctx); err != nil {
		if prsvc.IsGHUnavailable(err) && c.ghUnavailableLogged.CompareAndSwap(false, true) {
			c.logger.Warn("pr_creator: gh unavailable, skipping retry cycle", "err", err)
		}
		return
	}
	for _, repo := range c.repos.List() {
		if ctx.Err() != nil {
			return
		}
		if !repo.AutoCreatePR || repo.Store == nil {
			continue
		}
		entries, err := repo.Store.ListByStatus(repo.Project, taskstore.StatusDone)
		if err != nil {
			c.logger.Warn("pr_creator: list tasks failed", "repo", repo.Path, "err", err)
			continue
		}
		for _, entry := range entries {
			if entry.PRURL != "" || entry.PRCreateState != "failed" || entry.PRCreateAttempts >= c.cfg.MaxAttempts {
				continue
			}
			backoff := time.Duration(1<<entry.PRCreateAttempts) * time.Minute
			if entry.PRCreateAttemptedAt.Add(backoff).After(c.now()) {
				continue
			}
			if err := c.dispatch(ctx, repo, loop.CreatePRAction{PlanFile: entry.Filename}); err != nil {
				if prsvc.IsGHUnavailable(err) {
					return
				}
				c.logger.Warn("pr_creator: retry dispatch failed", "plan", entry.Filename, "err", err)
			}
		}
	}
}
