package daemon

import (
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	"github.com/kastheco/kasmos/config"
	"github.com/kastheco/kasmos/config/taskfsm"
	"github.com/kastheco/kasmos/config/taskstore"
	"github.com/kastheco/kasmos/orchestration/loop"
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
	// SignalGateway is the global DB-backed signal gateway shared by all registered repos.
	// It may be nil when the gateway has not yet been opened or is unavailable.
	SignalGateway taskstore.SignalGateway
	// SignalsDir is the path to the signals directory (<repo>/.kasmos/signals/).
	SignalsDir string
	// Processor is the signal processor for this repo. It persists across ticks
	// so that wave orchestrator state is maintained between poll cycles.
	Processor *loop.Processor
	// ReadinessSelfFixMaxLines is the effective per-repo self-fix line ceiling.
	ReadinessSelfFixMaxLines int
	// ReadinessMaxVerifyCycles is the effective per-repo verify-round cap.
	ReadinessMaxVerifyCycles int
	// ParallelPlannerArchitect is the effective per-repo parallel baseline flag.
	// Defaults to true (opt-out: set parallel_planner_architect = false in .kasmos/config.toml to disable).
	ParallelPlannerArchitect bool
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

	// Load per-repo TOML overrides once and derive effective config values.
	autoAdvance, autoReadinessReview, selfFixMaxLines, maxVerifyCycles, parallelPlannerArchitect := m.resolveRepoConfig(path)

	// Create a per-repo processor that persists across poll ticks so that wave
	// orchestrator state is maintained between cycles.
	proc := loop.NewProcessor(loop.ProcessorConfig{
		AutoAdvance:              autoAdvance,
		AutoReviewFix:            m.autoReviewFix,
		AutoReadinessReview:      autoReadinessReview,
		Store:                    m.globalStore,
		Project:                  project,
		MaxReviewFixCycles:       m.maxReviewFixCycles,
		ReadinessSelfFixMaxLines: selfFixMaxLines,
		ReadinessMaxVerifyCycles: maxVerifyCycles,
		ParallelPlannerArchitect: parallelPlannerArchitect,
		Hooks:                    hooks,
	})

	m.repos = append(m.repos, RepoEntry{
		Path:                     path,
		Project:                  project,
		Store:                    m.globalStore,
		SignalGateway:            m.globalGateway,
		SignalsDir:               signalsDir,
		Processor:                proc,
		ReadinessSelfFixMaxLines: selfFixMaxLines,
		ReadinessMaxVerifyCycles: maxVerifyCycles,
		ParallelPlannerArchitect: parallelPlannerArchitect,
	})
	return nil
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
// values for autoAdvance, autoReadinessReview, readinessSelfFixMaxLines,
// readinessMaxVerifyCycles, and parallelPlannerArchitect for the given repo path.
// parallelPlannerArchitect defaults to true (opt-out); a project TOML key overrides
// only when explicitly present. Falls back to daemon-level defaults for the other fields.
func (m *RepoManager) resolveRepoConfig(path string) (autoAdvance bool, autoReadinessReview bool, selfFixMaxLines int, maxVerifyCycles int, parallelPlannerArchitect bool) {
	autoAdvance = m.autoAdvance
	autoReadinessReview = m.autoReadinessReview
	selfFixMaxLines = m.readinessSelfFixMaxLines
	maxVerifyCycles = m.readinessMaxVerifyCycles
	parallelPlannerArchitect = true // default-on opt-out

	projTomlPath := filepath.Join(path, ".kasmos", config.TOMLConfigFileName)
	if _, err := os.Stat(projTomlPath); err != nil {
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
	if result.ParallelPlannerArchitect != nil {
		parallelPlannerArchitect = *result.ParallelPlannerArchitect
	}
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
	return
}
