package cmd

import (
	"context"

	"github.com/kastheco/kasmos/config/auditlog"
	"github.com/kastheco/kasmos/config/linearlink"
	"github.com/kastheco/kasmos/config/taskstore"
	"github.com/kastheco/kasmos/internal/linear"
)

var newTaskLinearFetcher = func(cfg linear.Config) linearlink.IssueFetcher {
	return linear.NewClientFromConfig(cfg)
}

func openTaskAuditLogger() (auditlog.Logger, func()) {
	logger, err := auditlog.NewSQLiteLogger(taskstore.ResolvedDBPath())
	if err != nil {
		return auditlog.NopLogger(), func() {}
	}
	return logger, func() {
		_ = logger.Close()
	}
}

func executeTaskLinkLinear(ctx context.Context, project string, in linearlink.LinkInput, store taskstore.Store) (linearlink.LinkResult, error) {
	logger, closeLogger := openTaskAuditLogger()
	defer closeLogger()

	cfg, err := linear.ConfigFromEnv()
	if err != nil {
		return linearlink.LinkResult{}, err
	}
	return executeTaskLinkLinearWithLoggerFetcher(ctx, project, in, store, logger, newTaskLinearFetcher(cfg))
}

func executeTaskUnlinkLinear(ctx context.Context, project, filename, reason string, store taskstore.Store) (linearlink.LinkResult, error) {
	logger, closeLogger := openTaskAuditLogger()
	defer closeLogger()

	return executeTaskUnlinkLinearWithLoggerFetcher(ctx, project, filename, reason, store, logger, nil)
}

func executeTaskLinkLinearWithLoggerFetcher(
	ctx context.Context,
	project string,
	in linearlink.LinkInput,
	store taskstore.Store,
	logger auditlog.Logger,
	fetcher linearlink.IssueFetcher,
) (linearlink.LinkResult, error) {
	return linearlink.New(store, fetcher, logger, project).Link(ctx, in)
}

func executeTaskUnlinkLinearWithLoggerFetcher(
	ctx context.Context,
	project string,
	filename string,
	reason string,
	store taskstore.Store,
	logger auditlog.Logger,
	fetcher linearlink.IssueFetcher,
) (linearlink.LinkResult, error) {
	return linearlink.New(store, fetcher, logger, project).Unlink(ctx, filename, reason)
}
