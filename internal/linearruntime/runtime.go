package linearruntime

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/kastheco/kasmos/config"
	"github.com/kastheco/kasmos/config/auditlog"
	"github.com/kastheco/kasmos/config/linearlink"
	"github.com/kastheco/kasmos/config/lineartrigger"
	"github.com/kastheco/kasmos/config/taskstore"
	"github.com/kastheco/kasmos/internal/linear"
)

// Resolved bundles the dependencies needed to run a Linear poller or webhook
// ingestor for one repo/project pair.
type Resolved struct {
	Project      string
	RepoPath     string
	TriggerCfg   lineartrigger.Config
	LinearCfg    linear.Config
	Client       *linear.Client
	Linker       *linearlink.Linker
	Poller       *lineartrigger.Poller
	Ingestor     *lineartrigger.WebhookIngestor
	Audit        auditlog.Logger
	SecretLookup func(string) (string, bool)
}

// Options controls runtime dependency construction.
type Options struct {
	Store   taskstore.Store
	Gateway taskstore.SignalGateway
	Audit   auditlog.Logger
	Now     func() time.Time
	Logger  *slog.Logger
}

// Resolve loads .env + process env, parses [linear.triggers], and constructs
// every dependency needed for Linear trigger polling or webhook ingestion.
func Resolve(ctx context.Context, repoPath, project string, opts Options) (*Resolved, error) {
	_ = ctx
	projTomlPath := filepath.Join(repoPath, ".kasmos", config.TOMLConfigFileName)
	if _, err := os.Stat(projTomlPath); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	result, err := config.LoadTOMLConfigFrom(projTomlPath)
	if err != nil {
		return nil, err
	}
	if result == nil || !result.LinearTriggers.Enabled {
		return nil, nil
	}

	lookup, err := EnvLookup(repoPath)
	if err != nil {
		return nil, err
	}
	linearCfg, err := linear.ConfigFromLookup(lookup)
	if err != nil {
		if errors.Is(err, linear.ErrNotConfigured) {
			return nil, nil
		}
		return nil, err
	}

	client := linear.NewClientFromConfig(linearCfg)
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default().With("monitor", "linear_trigger", "project", project)
	}
	auditLogger := opts.Audit
	if auditLogger == nil {
		auditLogger = auditlog.NopLogger()
	}
	now := opts.Now
	if now == nil {
		now = time.Now
	}

	linker := linearlink.New(opts.Store, client, auditLogger, project)
	service := lineartrigger.NewService(
		project,
		result.LinearTriggers,
		opts.Store,
		lineartrigger.NewRouter(result.LinearTriggers),
		lineartrigger.NewAuthoriser(result.LinearTriggers),
		lineartrigger.NewValidator(result.LinearTriggers, opts.Store, project),
	)
	poller := lineartrigger.NewPoller(lineartrigger.PollerDeps{
		Project: project,
		Config:  result.LinearTriggers,
		Store:   opts.Store,
		Linker:  linker,
		Linear:  client,
		Gateway: opts.Gateway,
		Audit:   auditLogger,
		Service: service,
		Now:     now,
		Logger:  logger,
	})

	resolved := &Resolved{
		Project:      project,
		RepoPath:     repoPath,
		TriggerCfg:   result.LinearTriggers,
		LinearCfg:    linearCfg,
		Client:       client,
		Linker:       linker,
		Poller:       poller,
		Audit:        auditLogger,
		SecretLookup: lookup,
	}
	if result.LinearTriggers.Webhook.Enabled {
		resolved.Ingestor = &lineartrigger.WebhookIngestor{
			Project: project,
			Config:  result.LinearTriggers,
			Store:   opts.Store,
			Linear:  client,
			Audit:   auditLogger,
			Now:     now,
			Logger:  logger,
		}
	}
	return resolved, nil
}

// LinearConfigForRepo resolves Linear API configuration with process env taking
// precedence over <repoPath>/.env.
func LinearConfigForRepo(repoPath string) (linear.Config, error) {
	lookup, err := EnvLookup(repoPath)
	if err != nil {
		return linear.Config{}, err
	}
	return linear.ConfigFromLookup(lookup)
}

// EnvLookup returns the repo-scoped lookup used for Linear API keys and webhook
// secrets. Process environment wins over project .env values.
func EnvLookup(repoPath string) (func(string) (string, bool), error) {
	values, err := config.ReadDotEnv(filepath.Join(repoPath, ".env"))
	if err != nil {
		if os.IsNotExist(err) {
			values = config.DotEnvValues{}
		} else {
			return nil, err
		}
	}
	return func(key string) (string, bool) {
		if value, ok := os.LookupEnv(key); ok && strings.TrimSpace(value) != "" {
			return value, true
		}
		value, ok := values[key]
		return value, ok
	}, nil
}
