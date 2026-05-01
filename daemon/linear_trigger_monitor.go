package daemon

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/kastheco/kasmos/config/lineartrigger"
	"github.com/kastheco/kasmos/daemon/api"
)

const minLinearTriggerMonitorInterval = 15 * time.Second

// LinearTriggerMonitor polls configured Linear trigger surfaces for registered repos.
type LinearTriggerMonitor struct {
	cfg         LinearTriggerMonitorConfig
	repos       *RepoManager
	broadcaster *api.EventBroadcaster
	logger      *slog.Logger
	pollers     sync.Map // project -> *lineartrigger.Poller
	credsLogged atomic.Bool
}

// NewLinearTriggerMonitor creates a Linear trigger monitor. It is inactive until Run is called.
func NewLinearTriggerMonitor(cfg LinearTriggerMonitorConfig, repos *RepoManager, broadcaster *api.EventBroadcaster, logger *slog.Logger) *LinearTriggerMonitor {
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = defaultDaemonConfig().LinearTriggerMonitor.PollInterval
	}
	if cfg.PollInterval < minLinearTriggerMonitorInterval {
		cfg.PollInterval = minLinearTriggerMonitorInterval
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &LinearTriggerMonitor{
		cfg:         cfg,
		repos:       repos,
		broadcaster: broadcaster,
		logger:      logger,
	}
}

// Run starts the Linear trigger monitor loop. It blocks until ctx is cancelled.
func (m *LinearTriggerMonitor) Run(ctx context.Context) error {
	interval := m.pollInterval()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	m.pollOnce(ctx)

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			m.pollOnce(ctx)
		}
	}
}

func (m *LinearTriggerMonitor) pollInterval() time.Duration {
	interval := m.cfg.PollInterval
	if interval <= 0 {
		interval = defaultDaemonConfig().LinearTriggerMonitor.PollInterval
	}
	for _, repo := range m.repos.List() {
		if !repo.LinearTriggerConfig.Enabled {
			continue
		}
		repoInterval := repo.LinearTriggerConfig.PollInterval
		if repoInterval <= 0 {
			repoInterval = m.cfg.PollInterval
		}
		if repoInterval > 0 && (interval <= 0 || repoInterval < interval) {
			interval = repoInterval
		}
	}
	if interval < minLinearTriggerMonitorInterval {
		return minLinearTriggerMonitorInterval
	}
	return interval
}

func (m *LinearTriggerMonitor) pollOnce(ctx context.Context) {
	for _, repo := range m.repos.List() {
		if ctx.Err() != nil {
			return
		}
		if !repo.LinearTriggerConfig.Enabled || repo.Store == nil {
			continue
		}
		m.pollRepo(ctx, repo)
	}
}

func (m *LinearTriggerMonitor) pollRepo(ctx context.Context, repo RepoEntry) {
	poller := repo.LinearTriggerPoller
	if cached, ok := m.pollers.Load(repo.Project); ok {
		poller, _ = cached.(*lineartrigger.Poller)
	} else if poller != nil {
		m.pollers.Store(repo.Project, poller)
	}
	if poller == nil {
		if m.credsLogged.CompareAndSwap(false, true) {
			m.logger.Warn("linear trigger monitor disabled until Linear API credentials are configured")
		}
		return
	}
	stats := poller.PollOnce(ctx)
	if stats.Err != nil && !stats.Aborted {
		m.logger.Warn("linear trigger poll failed", "project", repo.Project, "err", stats.Err)
	}
}
