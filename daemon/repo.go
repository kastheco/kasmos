package daemon

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/kastheco/kasmos/config"
	"github.com/kastheco/kasmos/config/auditlog"
	"github.com/kastheco/kasmos/config/linearreceipt"
	"github.com/kastheco/kasmos/config/lineartrigger"
	"github.com/kastheco/kasmos/config/taskfsm"
	"github.com/kastheco/kasmos/config/taskstore"
	"github.com/kastheco/kasmos/internal/linear"
	"github.com/kastheco/kasmos/internal/linearruntime"
	theme "github.com/kastheco/kasmos/internal/theme"
	"github.com/kastheco/kasmos/orchestration/loop"
	gitpkg "github.com/kastheco/kasmos/session/git"
)

// RepoEntry holds per-repo registration metadata.
type RepoEntry struct {
	// Path is the absolute path to the repository root.
	Path string
	// Project is the basename of the repository directory (e.g. "my-project").
	Project string
	// Store is the global task store shared by all registered repos.
	// It may be nil when the store has not yet been opened or is unavailable.
	// Per-repo data is namespaced by the project column inside the global DB.
	Store taskstore.Store
	// AutoCreatePR controls automatic pull request creation after verification approval.
	AutoCreatePR bool
	// SignalGateway is the global DB-backed signal gateway shared by all registered repos.
	// It may be nil when the gateway has not yet been opened or is unavailable.
	SignalGateway taskstore.SignalGateway
	// SignalsDir is the path to the signals directory (<repo>/.kasmos/signals/).
	SignalsDir string
	// Processor is the signal processor for this repo. It persists across ticks
	// so that wave orchestrator state is maintained between poll cycles.
	Processor *loop.Processor
	// Hooks is the repo-specific registry attached to daemon-owned FSM transitions.
	Hooks *taskfsm.HookRegistry
	// LinearReceiptHook posts non-FSM Linear receipts such as pull request creation.
	LinearReceiptHook *linearreceipt.Hook
	// LinearTriggerPoller polls guarded Linear trigger labels and slash commands.
	LinearTriggerPoller *lineartrigger.Poller
	// LinearTriggerConfig is the resolved per-repo trigger config.
	LinearTriggerConfig lineartrigger.Config
	// ReadinessSelfFixMaxLines is the effective per-repo self-fix line ceiling.
	ReadinessSelfFixMaxLines int
	// ReadinessMaxVerifyCycles is the effective per-repo verify-round cap.
	ReadinessMaxVerifyCycles int
	// MaxReviewFixCycles is the effective per-repo review-fix cycle cap.
	// Zero means unlimited.
	MaxReviewFixCycles int
	// MaxReviewFixCyclesResolved distinguishes an explicit/effective zero cap
	// from manually-constructed RepoEntry values that should use daemon defaults.
	MaxReviewFixCyclesResolved bool
	// PlannerProfiles is the ordered list of named planner profiles configured
	// for multi-planner draft mode. Empty means legacy single-planner mode.
	PlannerProfiles []string
	// PlannerDraftMode is true when PlannerProfiles should fan out into
	// planner draft cache writers instead of a single legacy planner.
	PlannerDraftMode bool
	// CacheDir is the per-repo cache directory used for planner draft artifacts.
	CacheDir string
	// SDK holds the resolved SDK transcript retention limits for this repo.
	// These are forwarded into SpawnOpts so every agent spawned for this repo
	// applies the same in-process transcript limits.
	SDK config.SDKConfig
	// Resources holds the resolved resource-control policy for this repo.
	// A zero value (Enabled=false) represents the normal/no-op profile.
	// Forwarded into SpawnOpts so every agent spawned for this repo inherits it.
	Resources config.ResolvedResourceControls
	// Theme holds the resolved repo palette for daemon-managed preview surfaces.
	// Fallbacks are non-fatal and keep the built-in palette.
	Theme theme.Result
}

func (e RepoEntry) newFSMWithHooks() *taskfsm.TaskStateMachine {
	fsm := taskfsm.New(e.Store, e.Project, "")
	fsm.SetHooks(e.Hooks)
	return fsm
}

// withSDKTranscriptRetention copies the repo's SDK transcript limits and
// resolved resource-control policy into opts. Call this on every SpawnOpts
// before handing it to any spawn function.
func withSDKTranscriptRetention(entry RepoEntry, opts loop.SpawnOpts) loop.SpawnOpts {
	opts.SDKTranscriptLimitsSet = true
	opts.SDKTranscriptMaxBytes = entry.SDK.TranscriptMaxBytes
	opts.SDKTranscriptMaxTurns = entry.SDK.TranscriptMaxTurns
	opts.ResourceControls = entry.Resources
	return opts
}

// resolvedResourceControlsForRepo returns the pre-loaded resource-control policy
// for entry. It reads from the already-resolved RepoEntry.Resources field and
// does not re-parse config.toml. A zero value (Enabled=false) means normal/no-op.
func resolvedResourceControlsForRepo(entry RepoEntry) config.ResolvedResourceControls {
	return entry.Resources
}

// RepoManager tracks registered repositories for the daemon.
// It is safe for concurrent use.
//
// All registered repos share a single global SQLite store at
// ~/.config/kasmos/taskstore.db; per-repo data is namespaced by the project
// column. The store is lazy-opened on the first Add() call and closed via
// Close() or when the last repo entry is removed.
type RepoManager struct {
	mu                       sync.RWMutex
	repos                    []RepoEntry
	autoAdvance              bool
	autoReviewFix            bool
	autoReadinessReview      bool
	autoCreatePR             bool
	maxReviewFixCycles       int
	readinessSelfFixMaxLines int
	readinessMaxVerifyCycles int
	// globalDB is the single shared *sql.DB, lazy-opened on the first Add().
	// Both globalStore and globalGateway are derived from it.
	globalDB *sql.DB
	// globalStore is the shared backing store, lazy-opened on the first Add().
	globalStore taskstore.Store
	// globalGateway is the shared signal gateway, lazy-opened on the first Add().
	globalGateway taskstore.SignalGateway
	// openDB is the factory function used to open the shared database.
	// It defaults to taskstore.OpenBackingSharedDB and may be overridden in tests.
	openDB func() (*sql.DB, error)
	// openStore and openGateway are legacy factory functions kept for test
	// compatibility. When openDB is set (the default), these are ignored.
	openStore   func() (taskstore.Store, error)
	openGateway func() (taskstore.SignalGateway, error)
}

// NewRepoManager returns an empty, ready-to-use RepoManager.
func NewRepoManager() *RepoManager {
	return &RepoManager{
		openDB: taskstore.OpenBackingSharedDB,
	}
}

// Add registers a repository by absolute path.
// It derives the project name from the directory basename and sets the signals dir.
// The global SQLite taskstore (~/.config/kasmos/taskstore.db) is opened lazily on
// the first Add() call and shared across all repos; per-repo data is namespaced by
// the project column. Any error opening the global store is non-fatal — the entry
// is added with a nil Store.
// On each Add a one-time migration from the repo's legacy local taskstore.db is
// attempted (a no-op when the local file does not exist or is already migrated).
// Returns an error if path is already registered.
func (m *RepoManager) Add(path string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	project := filepath.Base(path)

	for _, r := range m.repos {
		if r.Path == path {
			return fmt.Errorf("repo already registered: %s", path)
		}
		if r.Project == project {
			return fmt.Errorf("repo with basename %q already registered (path: %s); rename one of the directories or use distinct names", project, r.Path)
		}
	}

	kasmosDir := filepath.Join(path, ".kasmos")
	signalsDir := filepath.Join(kasmosDir, "signals")

	// Lazy-open the shared DB, store, and gateway on first Add().
	if m.globalDB == nil && m.globalStore == nil {
		if m.openDB != nil {
			// Preferred path: single shared *sql.DB for all subsystems.
			if db, err := m.openDB(); err == nil {
				m.globalDB = db
				if s, err := taskstore.NewSQLiteStoreFromDB(db); err == nil {
					m.globalStore = s
				} else {
					slog.Warn("daemon: failed to create store from shared db", "error", err)
				}
				if g, err := taskstore.NewSQLiteSignalGatewayFromDB(db); err == nil {
					m.globalGateway = g
				} else {
					slog.Warn("daemon: failed to create gateway from shared db", "error", err)
				}
			} else {
				slog.Warn("daemon: failed to open shared db", "error", err)
			}
		} else {
			// Legacy path: separate factory functions (used by tests).
			if m.openStore != nil {
				if s, err := m.openStore(); err == nil {
					m.globalStore = s
				} else {
					slog.Warn("daemon: failed to open global taskstore", "error", err)
				}
			}
			if m.openGateway != nil {
				if g, err := m.openGateway(); err == nil {
					m.globalGateway = g
				} else {
					slog.Warn("daemon: failed to open global signal gateway", "error", err)
				}
			}
		}
	}

	// Migrate any existing repo-local tasks into the global store (idempotent).
	if m.globalStore != nil {
		if _, err := taskstore.MigrateRepoLocalToGlobal(m.globalStore, project, kasmosDir); err != nil {
			slog.Warn("daemon: failed to migrate repo-local tasks to global store", "repo", path, "error", err)
		}
	}

	// Build a hook registry by reading the per-repo config.toml.
	// This is side-effect-free (no implicit writes) and anchored to the repo
	// being registered, so each repo gets its own hook configuration.
	var hooks *taskfsm.HookRegistry
	if hookCfgs, err := config.LoadHooksForRepo(path); err != nil {
		slog.Warn("daemon: failed to load hooks for repo, hooks disabled", "repo", path, "error", err)
	} else if len(hookCfgs) > 0 {
		cfgs := make([]taskfsm.HookConfig, len(hookCfgs))
		for i, h := range hookCfgs {
			cfgs[i] = taskfsm.HookConfig{
				Type:    h.Type,
				URL:     h.URL,
				Headers: h.Headers,
				Command: h.Command,
				Events:  h.Events,
			}
		}
		hooks = taskfsm.BuildHookRegistry(cfgs)
	}

	linearReceiptHook, linearReceiptConfig := m.loadLinearReceiptHook(path, project)
	if linearReceiptHook != nil {
		if hooks == nil {
			hooks = taskfsm.NewHookRegistry()
		}
		hooks.Add(linearReceiptHook, eventsFromConfig(linearReceiptConfig))
	}
	linearTriggerPoller, linearTriggerConfig := m.loadLinearTriggerPoller(path, project, m.globalStore, m.globalGateway)

	// Load per-repo TOML overrides once and derive effective config values.
	autoAdvance, autoReadinessReview, autoCreatePR, maxReviewFixCycles, selfFixMaxLines, maxVerifyCycles, plannerProfiles, plannerDraftMode, sdkCfg, resourceControls, err := m.resolveRepoConfig(path)
	if err != nil {
		return err
	}
	themeResult := resolveRepoTheme(context.Background(), path)

	// Create a per-repo processor that persists across poll ticks so that wave
	// orchestrator state is maintained between cycles.
	proc := loop.NewProcessor(loop.ProcessorConfig{
		AutoAdvance:              autoAdvance,
		AutoReviewFix:            m.autoReviewFix,
		AutoReadinessReview:      autoReadinessReview,
		Store:                    m.globalStore,
		Project:                  project,
		HeadSHA:                  func(branch string) (string, error) { return gitpkg.BranchHeadSHA(path, branch) },
		MergeBaseSHA:             func(branch string) (string, error) { return gitpkg.BranchMergeBaseSHA(path, branch) },
		MaxReviewFixCycles:       maxReviewFixCycles,
		ReadinessSelfFixMaxLines: selfFixMaxLines,
		ReadinessMaxVerifyCycles: maxVerifyCycles,
		PlannerProfiles:          plannerProfiles,
		PlannerDraftMode:         plannerDraftMode,
		CacheDir:                 filepath.Join(kasmosDir, "cache"),
		Hooks:                    hooks,
	})

	m.repos = append(m.repos, RepoEntry{
		Path:                       path,
		Project:                    project,
		Store:                      m.globalStore,
		AutoCreatePR:               autoCreatePR,
		SignalGateway:              m.globalGateway,
		SignalsDir:                 signalsDir,
		Processor:                  proc,
		Hooks:                      hooks,
		LinearReceiptHook:          linearReceiptHook,
		LinearTriggerPoller:        linearTriggerPoller,
		LinearTriggerConfig:        linearTriggerConfig,
		ReadinessSelfFixMaxLines:   selfFixMaxLines,
		ReadinessMaxVerifyCycles:   maxVerifyCycles,
		MaxReviewFixCycles:         maxReviewFixCycles,
		MaxReviewFixCyclesResolved: true,
		PlannerProfiles:            plannerProfiles,
		PlannerDraftMode:           plannerDraftMode,
		CacheDir:                   filepath.Join(kasmosDir, "cache"),
		SDK:                        sdkCfg,
		Resources:                  resourceControls,
		Theme:                      themeResult,
	})
	return nil
}

func (m *RepoManager) loadLinearReceiptHook(path, project string) (*linearreceipt.Hook, linearreceipt.Config) {
	projTomlPath := filepath.Join(path, ".kasmos", config.TOMLConfigFileName)
	if _, err := os.Stat(projTomlPath); err != nil {
		return nil, linearreceipt.Config{}
	}
	result, err := config.LoadTOMLConfigFrom(projTomlPath)
	if err != nil {
		slog.Warn("daemon: failed to load linear receipt config for repo, receipts disabled", "repo", path, "error", err)
		return nil, linearreceipt.Config{}
	}
	if result == nil || !result.LinearReceipts.Enabled {
		return nil, linearreceipt.Config{}
	}
	cfg, err := linearConfigForRepo(path)
	if err != nil && !errors.Is(err, linear.ErrNotConfigured) {
		slog.Warn("daemon: failed to create linear receipt client for repo, receipts disabled", "repo", path, "error", err)
		return nil, result.LinearReceipts
	}
	var client linearreceipt.ClientAdapter
	if err == nil {
		client = linear.NewClientFromConfig(cfg)
	}
	var auditLogger auditlog.Logger
	if m.globalDB != nil {
		if l, err := auditlog.NewSQLiteLoggerFromDB(m.globalDB); err == nil {
			auditLogger = l
		}
	}
	return linearreceipt.NewHook(result.LinearReceipts, m.globalStore, client, auditLogger, project), result.LinearReceipts
}

func (m *RepoManager) loadLinearTriggerPoller(path, project string, store taskstore.Store, gateway taskstore.SignalGateway) (*lineartrigger.Poller, lineartrigger.Config) {
	var auditLogger auditlog.Logger
	if m.globalDB != nil {
		if l, err := auditlog.NewSQLiteLoggerFromDB(m.globalDB); err == nil {
			auditLogger = l
		}
	}
	resolved, err := linearruntime.Resolve(context.Background(), path, project, linearruntime.Options{
		Store:   store,
		Gateway: gateway,
		Audit:   auditLogger,
		Now:     time.Now,
		Logger:  slog.Default().With("monitor", "linear_trigger", "project", project),
	})
	if err != nil {
		slog.Warn("daemon: failed to load linear trigger config for repo", "repo", path, "err", err)
		return nil, lineartrigger.Config{}
	}
	if resolved == nil {
		return nil, lineartrigger.Config{}
	}
	return resolved.Poller, resolved.TriggerCfg
}

func linearConfigForRepo(path string) (linear.Config, error) {
	return linearruntime.LinearConfigForRepo(path)
}

func eventsFromConfig(cfg linearreceipt.Config) []taskfsm.Event {
	events := make([]taskfsm.Event, 0, len(cfg.Events))
	for event := range cfg.Events {
		events = append(events, event)
	}
	return events
}

func resolveRepoTheme(ctx context.Context, path string) theme.Result {
	cfg := config.DefaultConfig()
	projTomlPath := filepath.Join(path, ".kasmos", config.TOMLConfigFileName)
	if _, err := os.Stat(projTomlPath); err != nil {
		return resolveTheme(ctx, cfg, filepath.Dir(projTomlPath))
	}
	if result, err := config.LoadTOMLConfigFrom(projTomlPath); err != nil {
		slog.Warn("daemon: failed to read project theme config, using default theme", "repo", path, "error", err)
	} else if result != nil {
		cfg.ThemeSource = result.ThemeSource
		cfg.SystemThemeProvider = result.SystemThemeProvider
		cfg.ThemePaletteFile = result.ThemePaletteFile
	}

	return resolveTheme(ctx, cfg, filepath.Dir(projTomlPath))
}

func resolveTheme(ctx context.Context, cfg *config.Config, paletteFileBaseDir string) theme.Result {
	return theme.Resolve(ctx, theme.Options{
		Source:             cfg.ThemeSource,
		Provider:           cfg.SystemThemeProvider,
		PaletteFile:        cfg.ThemePaletteFile,
		PaletteFileBaseDir: paletteFileBaseDir,
	}, theme.Dependencies{
		ReadFile: os.ReadFile,
		HomeDir:  os.UserHomeDir,
		RunCommand: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			return exec.CommandContext(ctx, name, args...).Output()
		},
	})
}

// Remove deregisters a repository by absolute path.
// The shared global store and gateway are closed only when the last repo is removed.
// Returns an error if path is not registered.
func (m *RepoManager) Remove(path string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i, r := range m.repos {
		if r.Path == path {
			m.repos = append(m.repos[:i], m.repos[i+1:]...)
			if len(m.repos) == 0 {
				m.closeGlobalLocked()
			}
			return nil
		}
	}
	return fmt.Errorf("repo not registered: %s", path)
}

// RemoveByProject deregisters a repository by its project name (the basename
// of the repo path). Returns an error if not found.
// The shared global store and gateway are closed only when the last repo is removed.
func (m *RepoManager) RemoveByProject(project string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i, r := range m.repos {
		if r.Project == project {
			m.repos = append(m.repos[:i], m.repos[i+1:]...)
			if len(m.repos) == 0 {
				m.closeGlobalLocked()
			}
			return nil
		}
	}
	return fmt.Errorf("repo not registered: %s", project)
}

// Close closes the shared global store and signal gateway.
// It is safe to call even when no repos are registered.
func (m *RepoManager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closeGlobalLocked()
}

// closeGlobalLocked closes and nils the global store, gateway, and shared DB.
// Caller must hold m.mu.
func (m *RepoManager) closeGlobalLocked() {
	// Close store and gateway first (no-ops when backed by shared DB).
	if m.globalStore != nil {
		_ = m.globalStore.Close()
		m.globalStore = nil
	}
	if m.globalGateway != nil {
		_ = m.globalGateway.Close()
		m.globalGateway = nil
	}
	// Close the shared DB last — this is the actual connection teardown.
	if m.globalDB != nil {
		_ = m.globalDB.Close()
		m.globalDB = nil
	}
}

// List returns a snapshot of all currently registered repositories.
// The returned slice is a copy — modifications do not affect internal state.
func (m *RepoManager) List() []RepoEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()

	out := make([]RepoEntry, len(m.repos))
	copy(out, m.repos)
	return out
}

// Get returns the RepoEntry for the given path, or an error if not registered.
func (m *RepoManager) Get(path string) (RepoEntry, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, r := range m.repos {
		if r.Path == path {
			return r, nil
		}
	}
	return RepoEntry{}, fmt.Errorf("repo not registered: %s", path)
}

// resolveRepoConfig reads per-repo TOML overrides and returns the effective
// values for autoAdvance, autoReadinessReview, maxReviewFixCycles,
// readinessSelfFixMaxLines, readinessMaxVerifyCycles, planner profile fan-out settings, the SDK transcript
// retention config, and the resolved resource-control policy for the given repo path.
// Nil or empty planner lists keep legacy single-planner mode. Falls back to
// daemon-level defaults for the other fields.
// SDK limits default to config.DefaultConfig().SDK values when absent from the TOML file.
// Resource controls default to the normal/no-op profile when [resources] is absent.
func (m *RepoManager) resolveRepoConfig(path string) (autoAdvance bool, autoReadinessReview bool, autoCreatePR bool, maxReviewFixCycles int, selfFixMaxLines int, maxVerifyCycles int, plannerProfiles []string, plannerDraftMode bool, sdkCfg config.SDKConfig, resourceControls config.ResolvedResourceControls, err error) {
	autoAdvance = m.autoAdvance
	autoReadinessReview = m.autoReadinessReview
	autoCreatePR = m.autoCreatePR
	maxReviewFixCycles = m.maxReviewFixCycles
	selfFixMaxLines = m.readinessSelfFixMaxLines
	maxVerifyCycles = m.readinessMaxVerifyCycles
	sdkCfg = config.DefaultConfig().SDK
	resourceControls = config.ResolvedResourceControls{Profile: "normal"}

	projTomlPath := filepath.Join(path, ".kasmos", config.TOMLConfigFileName)
	if _, statErr := os.Stat(projTomlPath); statErr != nil {
		// File absent or unreadable — use defaults.
		return
	}
	result, err := config.LoadTOMLConfigFrom(projTomlPath)
	if err != nil {
		slog.Warn("daemon: failed to read project config, using daemon defaults", "repo", path, "error", err)
		return
	}
	if result.AutoAdvance != nil {
		autoAdvance = *result.AutoAdvance
	}
	if result.AutoReadinessReview != nil {
		autoReadinessReview = *result.AutoReadinessReview
	}
	if result.AutoCreatePR != nil {
		autoCreatePR = *result.AutoCreatePR
	}
	if result.MaxReviewFixCycles != nil {
		if *result.MaxReviewFixCycles >= 0 {
			maxReviewFixCycles = *result.MaxReviewFixCycles
		} else {
			slog.Warn(
				"daemon: invalid project max_review_fix_cycles override, using daemon default",
				"repo", path,
				"config", projTomlPath,
				"value", *result.MaxReviewFixCycles,
				"default", maxReviewFixCycles,
			)
		}
	}
	cfg := &config.Config{
		Profiles: result.Profiles,
		Planners: result.Planners,
	}
	if validateErr := cfg.ValidatePlannerProfiles(); validateErr != nil {
		err = fmt.Errorf("daemon: invalid project planner config for %s: %w", path, validateErr)
		return
	}
	plannerProfiles = cfg.PlannerProfileNames()
	plannerDraftMode = len(plannerProfiles) > 0

	if result.ReadinessSelfFixMaxLines != nil {
		if *result.ReadinessSelfFixMaxLines > 0 {
			selfFixMaxLines = *result.ReadinessSelfFixMaxLines
		} else {
			slog.Warn(
				"daemon: invalid project readiness_self_fix_max_lines override, using daemon default",
				"repo", path,
				"config", projTomlPath,
				"value", *result.ReadinessSelfFixMaxLines,
				"default", selfFixMaxLines,
			)
		}
	}
	if result.ReadinessMaxVerifyCycles != nil {
		if *result.ReadinessMaxVerifyCycles > 0 {
			maxVerifyCycles = *result.ReadinessMaxVerifyCycles
		} else {
			slog.Warn(
				"daemon: invalid project readiness_max_verify_cycles override, using daemon default",
				"repo", path,
				"config", projTomlPath,
				"value", *result.ReadinessMaxVerifyCycles,
				"default", maxVerifyCycles,
			)
		}
	}
	// SDK transcript retention: nil pointers mean key absent (keep default from DefaultConfig).
	if result.SDK.TranscriptMaxBytes != nil {
		v := *result.SDK.TranscriptMaxBytes
		if v < 0 {
			slog.Warn("daemon: invalid project sdk.transcript_max_bytes, clamping to 0", "repo", path, "value", v)
			v = 0
		}
		sdkCfg.TranscriptMaxBytes = v
	}
	if result.SDK.TranscriptMaxTurns != nil {
		v := *result.SDK.TranscriptMaxTurns
		if v < 0 {
			slog.Warn("daemon: invalid project sdk.transcript_max_turns, clamping to 0", "repo", path, "value", v)
			v = 0
		}
		sdkCfg.TranscriptMaxTurns = v
	}
	// Resource controls: resolve once from the [resources] TOML block.
	if rc, resolveErr := result.Resources.Resolve(); resolveErr != nil {
		err = fmt.Errorf("daemon: invalid project [resources] config for %s: %w", path, resolveErr)
		return
	} else {
		resourceControls = rc
	}
	return
}
