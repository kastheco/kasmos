package taskstore

import (
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
	_ "modernc.org/sqlite" // register sqlite driver
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
	databaseURL := loadConfiguredDatabaseURL()
	if strings.TrimSpace(databaseURL) == "" {
		store, err := OpenBackingSQLiteStore()
		if err != nil {
			return nil, fmt.Errorf("open authoritative task store for project %s: %w", project, err)
		}
		return store, nil
	}

	store, err := NewStoreFromConfig(databaseURL, project)
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
	databaseURL := loadConfiguredDatabaseURL()
	if strings.TrimSpace(databaseURL) == "" {
		gateway, err := OpenBackingSQLiteSignalGateway()
		if err != nil {
			return nil, fmt.Errorf("open authoritative signal gateway for project %s: %w", project, err)
		}
		return gateway, nil
	}

	store, err := NewStoreFromConfig(databaseURL, project)
	if err != nil {
		return nil, fmt.Errorf("open authoritative signal gateway for project %s: %w", project, err)
	}
	if err := store.Ping(); err != nil {
		_ = store.Close()
		return nil, fmt.Errorf("open authoritative signal gateway for project %s: %w", project, err)
	}
	_ = store.Close()

	return nil, fmt.Errorf("open authoritative signal gateway for project %s: remote task store %q does not expose signal gateway access", project, strings.TrimSpace(databaseURL))
}

type taskStoreTOMLConfig struct {
	DatabaseURL string `toml:"database_url"`
}

func loadConfiguredDatabaseURL() string {
	configDir, err := taskStoreConfigDir()
	if err != nil {
		return ""
	}
	data, err := os.ReadFile(filepath.Join(configDir, "config.toml"))
	if err != nil {
		return ""
	}
	var cfg taskStoreTOMLConfig
	if _, err := toml.Decode(string(data), &cfg); err != nil {
		return ""
	}
	return cfg.DatabaseURL
}

func taskStoreConfigDir() (string, error) {
	if cwd, err := os.Getwd(); err == nil {
		if repoRoot, err := taskStoreRepoRoot(cwd); err == nil {
			return filepath.Join(repoRoot, ".kasmos"), nil
		}
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "kasmos"), nil
}

func taskStoreRepoRoot(dir string) (string, error) {
	out, err := exec.Command("git", "-C", dir, "rev-parse", "--path-format=absolute", "--git-common-dir").Output()
	if err != nil {
		out, err = exec.Command("git", "-C", dir, "rev-parse", "--git-common-dir").Output()
		if err != nil {
			return "", fmt.Errorf("resolve repo root for %s: %w", dir, err)
		}
	}
	gitDir := strings.TrimSpace(string(out))
	if gitDir == "" {
		return "", fmt.Errorf("resolve repo root for %s: empty git-common-dir output", dir)
	}
	if !filepath.IsAbs(gitDir) {
		gitDir = filepath.Join(dir, gitDir)
	}
	return filepath.Dir(filepath.Clean(gitDir)), nil
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

// OpenSharedDB opens a single *sql.DB with WAL mode, busy_timeout, foreign
// keys, synchronous=normal, and txlock=immediate applied to every pooled
// connection via modernc.org/sqlite's DSN _pragma mechanism. Callers pass the
// returned handle to NewSQLiteStoreFromDB, NewSQLiteSignalGatewayFromDB, and
// auditlog.NewSQLiteLoggerFromDB so that all subsystems share one connection
// pool. The caller owns the returned *sql.DB and must close it after all
// subsystems are done.
//
// The previous implementation ran PRAGMAs via db.Exec after open, which only
// applied them to the FIRST connection Go happened to use — every subsequent
// connection in the pool defaulted to busy_timeout=0 and failed immediately
// on any write contention, producing SQLITE_BUSY errors under normal load.
// DSN pragmas run as part of the per-connection init callback, so every
// connection (including ones created after pool growth) picks them up.
func OpenSharedDB(dbPath string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", buildSQLiteDSN(dbPath))
	if err != nil {
		return nil, fmt.Errorf("open shared sqlite db: %w", err)
	}
	// sql.Open is lazy — Ping forces the driver to actually create the
	// connection (and the DB file) now, so callers get the same behaviour they
	// had with the old Exec-based PRAGMA setup. It also surfaces DSN typos.
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping shared sqlite db: %w", err)
	}
	return db, nil
}

// BuildSQLiteDSN returns a modernc.org/sqlite DSN with the standard kasmos
// PRAGMAs applied per-connection. Exported so sibling packages (auditlog,
// config/permission_store) use the exact same pragma set without duplicating
// the list. Internal callers can also use this directly when building their
// own *sql.DB pools.
func BuildSQLiteDSN(dbPath string) string {
	return buildSQLiteDSN(dbPath)
}

// buildSQLiteDSN returns a modernc.org/sqlite DSN with the standard kasmos
// PRAGMAs applied per-connection. Internal entry point for taskstore callers.
func buildSQLiteDSN(dbPath string) string {
	if dbPath == ":memory:" {
		// :memory: databases don't support WAL and are per-connection anyway.
		// Keep busy_timeout so tests that share a pool still behave sanely.
		return dbPath + "?_pragma=busy_timeout(30000)&_pragma=foreign_keys(on)"
	}
	// _txlock=immediate ensures write transactions grab the exclusive lock up
	// front, which pairs correctly with busy_timeout. Without it, a DEFERRED
	// tx can escalate from shared to exclusive mid-transaction and fail fast.
	return "file:" + dbPath + "?_pragma=journal_mode(wal)" +
		"&_pragma=busy_timeout(30000)" +
		"&_pragma=foreign_keys(on)" +
		"&_pragma=synchronous(normal)" +
		"&_txlock=immediate"
}

// OpenBackingSharedDB is a convenience wrapper that calls OpenSharedDB with
// the resolved global DB path (~/.config/kasmos/taskstore.db).
func OpenBackingSharedDB() (*sql.DB, error) {
	dbPath := ResolvedDBPath()
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, fmt.Errorf("create kasmos config dir: %w", err)
	}
	return OpenSharedDB(dbPath)
}

// GlobalDBPath returns the global taskstore path: ~/.config/kasmos/taskstore.db.
func GlobalDBPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".", ".kasmos", "taskstore.db")
	}
	return filepath.Join(home, ".config", "kasmos", "taskstore.db")
}

// ResolvedDBPath returns the filesystem path that the factory would use for a
// local SQLite taskstore.
// This path is shared with the auditlog SQLiteLogger so both can coexist in
// the same database file (each using a separate table).
func ResolvedDBPath() string {
	return GlobalDBPath()
}
