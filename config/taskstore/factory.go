package taskstore

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kastheco/kasmos/config"
)

// NewStoreFromConfig creates a Store from a remote task store URL and project name.
// If storeURL is empty, it returns (nil, nil) so the caller can choose the
// local embedded/SQLite task store path instead.
// The returned store uses lazy connection: the URL is validated syntactically
// but no network connection is made until the first operation (or Ping).
func NewStoreFromConfig(storeURL, project string) (Store, error) {
	if strings.TrimSpace(storeURL) == "" {
		return nil, nil // no remote store configured
	}
	return NewHTTPStoreWithOptions(HTTPStoreOptions{BaseURL: storeURL, Project: project}), nil
}

// OpenAuthoritativeStore opens the authoritative task store for the current
// repo/project. When a remote task store is configured, it must be reachable —
// callers do not silently fall back to a second local writer. When no remote
// authority is configured, repo-local SQLite is the only valid authority.
func OpenAuthoritativeStore(project string) (Store, error) {
	cfg := config.LoadConfig()
	if strings.TrimSpace(cfg.DatabaseURL) == "" {
		store, err := OpenBackingSQLiteStore()
		if err != nil {
			return nil, fmt.Errorf("open authoritative task store for project %s: %w", project, err)
		}
		return store, nil
	}

	store, err := NewStoreFromConfig(cfg.DatabaseURL, project)
	if err != nil {
		return nil, fmt.Errorf("open authoritative task store for project %s: %w", project, err)
	}
	if err := store.Ping(); err != nil {
		_ = store.Close()
		return nil, fmt.Errorf("open authoritative task store for project %s: %w", project, err)
	}
	return store, nil
}

// OpenAuthoritativeSignalGateway opens the authoritative signal gateway for the
// current repo/project. Remote task-store authorities must be reachable; when a
// remote authority is configured but does not expose signals, this fails fast
// instead of silently writing to repo-local SQLite. When no remote authority is
// configured, repo-local SQLite is the only valid authority.
func OpenAuthoritativeSignalGateway(project string) (SignalGateway, error) {
	cfg := config.LoadConfig()
	if strings.TrimSpace(cfg.DatabaseURL) == "" {
		gateway, err := OpenBackingSQLiteSignalGateway()
		if err != nil {
			return nil, fmt.Errorf("open authoritative signal gateway for project %s: %w", project, err)
		}
		return gateway, nil
	}

	store, err := NewStoreFromConfig(cfg.DatabaseURL, project)
	if err != nil {
		return nil, fmt.Errorf("open authoritative signal gateway for project %s: %w", project, err)
	}
	if err := store.Ping(); err != nil {
		_ = store.Close()
		return nil, fmt.Errorf("open authoritative signal gateway for project %s: %w", project, err)
	}
	_ = store.Close()

	return nil, fmt.Errorf("open authoritative signal gateway for project %s: remote task store %q does not expose signal gateway access", project, strings.TrimSpace(cfg.DatabaseURL))
}

// OpenBackingSQLiteStore opens the repo-root-backed SQLite store used by the
// daemon, embedded server bootstrap, and authoritative local fallback.
func OpenBackingSQLiteStore() (Store, error) {
	dbPath := ResolvedDBPath()
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, fmt.Errorf("create kasmos config dir: %w", err)
	}
	return NewSQLiteStore(dbPath)
}

// OpenBackingSQLiteSignalGateway opens the repo-root-backed SQLite signal
// gateway used by local authoritative signal writers.
func OpenBackingSQLiteSignalGateway() (SignalGateway, error) {
	dbPath := ResolvedDBPath()
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, fmt.Errorf("create kasmos config dir: %w", err)
	}
	return NewSQLiteSignalGateway(dbPath)
}

// ResolvedDBPath returns the filesystem path that the factory would use for a
// local SQLite taskstore. It delegates to config.GetConfigDir() to resolve the
// project-local config directory (<repo-root>/.kasmos/ when in a git repo) and
// appends "taskstore.db".
// This path is shared with the auditlog SQLiteLogger so both can coexist in
// the same database file (each using a separate table).
func ResolvedDBPath() string {
	dir, err := config.GetConfigDir()
	if err != nil {
		return filepath.Join(".", ".kasmos", "taskstore.db")
	}
	return filepath.Join(dir, "taskstore.db")
}
