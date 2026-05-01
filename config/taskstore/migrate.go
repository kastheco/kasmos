package taskstore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
)

// MigrateRepoLocalToGlobal copies tasks, content, subtasks, topics, PR reviews,
// Linear triggers, comment cursors, and signals from a repo-local taskstore.db into the global store. If the
// repo-local DB does not exist, it returns (0, nil) — a no-op. The migration
// is idempotent: duplicate tasks, topics, and PR reviews are silently skipped;
// subtasks are overwritten (SetSubtasks replaces all); signals are deduplicated
// by (project, plan_file, signal_type, created_at).
//
// Returns the number of tasks successfully migrated (newly created).
func MigrateRepoLocalToGlobal(globalStore Store, project, repoKasmosDir string) (int, error) {
	localDBPath := filepath.Join(repoKasmosDir, "taskstore.db")

	if _, err := os.Stat(localDBPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return 0, nil
		}
		return 0, fmt.Errorf("stat repo-local taskstore: %w", err)
	}

	// Open the repo-local DB. NewSQLiteStore runs schema migrations as a
	// side-effect but they are idempotent no-ops on an already-current DB.
	localStore, err := NewSQLiteStore(localDBPath)
	if err != nil {
		return 0, fmt.Errorf("open repo-local taskstore: %w", err)
	}

	sourceProjects, err := listMigratedProjects(localStore, project)
	if err != nil {
		localStore.Close()
		return 0, fmt.Errorf("list repo-local projects: %w", err)
	}

	migrated := 0
	for _, sourceProject := range sourceProjects {
		targetProject := normalizeMigratedProjectName(sourceProject)

		tasks, err := localStore.List(sourceProject)
		if err != nil {
			localStore.Close()
			return 0, fmt.Errorf("list repo-local tasks for project %s: %w", sourceProject, err)
		}

		for _, task := range tasks {
			// INSERT OR IGNORE semantics: skip tasks that already exist.
			if err := globalStore.Create(targetProject, task); err != nil {
				if strings.Contains(err.Error(), "plan already exists") {
					// Already in global store — fall through to subtask migration.
				} else {
					localStore.Close()
					return migrated, fmt.Errorf("migrate task %s (%s→%s): %w", task.Filename, sourceProject, targetProject, err)
				}
			} else {
				migrated++
			}

			// Migrate subtasks (SetSubtasks replaces all — idempotent).
			if subtasks, subErr := localStore.GetSubtasks(sourceProject, task.Filename); subErr == nil && len(subtasks) > 0 {
				if err := globalStore.SetSubtasks(targetProject, task.Filename, subtasks); err != nil {
					localStore.Close()
					return migrated, fmt.Errorf("migrate subtasks for task %s (%s→%s): %w", task.Filename, sourceProject, targetProject, err)
				}
			}
		}

		// Migrate topics.
		if topics, err := localStore.ListTopics(sourceProject); err == nil {
			for _, topic := range topics {
				if err := globalStore.CreateTopic(targetProject, topic); err != nil {
					if !strings.Contains(err.Error(), "topic already exists") {
						localStore.Close()
						return migrated, fmt.Errorf("migrate topic %s (%s→%s): %w", topic.Name, sourceProject, targetProject, err)
					}
				}
			}
		}
	}

	// Close local store before ATTACH to avoid WAL lock contention.
	localStore.Close()

	// Bulk-copy PR reviews and signals via ATTACH when global store is SQLite.
	if globalSQLite, ok := globalStore.(*SQLiteStore); ok {
		for _, sourceProject := range sourceProjects {
			targetProject := normalizeMigratedProjectName(sourceProject)
			if err := migrateRepoLocalPRReviews(globalSQLite.db, localDBPath, sourceProject, targetProject); err != nil {
				return migrated, err
			}
			if err := migrateRepoLocalLinearTriggers(globalSQLite.db, localDBPath, sourceProject, targetProject); err != nil {
				return migrated, err
			}
			if err := migrateRepoLocalLinearWebhookDeliveries(globalSQLite.db, localDBPath, sourceProject, targetProject); err != nil {
				return migrated, err
			}
			if err := migrateRepoLocalLinearCommentCursors(globalSQLite.db, localDBPath, sourceProject, targetProject); err != nil {
				return migrated, err
			}
			if err := migrateRepoLocalSignals(globalSQLite.db, localDBPath, sourceProject, targetProject); err != nil {
				return migrated, err
			}
		}
	}

	return migrated, nil
}

func listMigratedProjects(localStore *SQLiteStore, fallbackProject string) ([]string, error) {
	seen := make(map[string]struct{})
	projects := make([]string, 0)

	collect := func(table string) error {
		var exists int
		if err := localStore.db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type='table' AND name = ?`, table).Scan(&exists); err != nil {
			return err
		}
		if exists == 0 {
			return nil
		}
		rows, err := localStore.db.Query(fmt.Sprintf(`SELECT DISTINCT project FROM %s WHERE trim(project) <> '' ORDER BY project`, table))
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var project string
			if err := rows.Scan(&project); err != nil {
				return err
			}
			project = strings.TrimSpace(project)
			if project == "" {
				continue
			}
			if _, ok := seen[project]; ok {
				continue
			}
			seen[project] = struct{}{}
			projects = append(projects, project)
		}
		return rows.Err()
	}

	for _, table := range []string{"tasks", "topics", "pr_reviews", "linear_triggers", "linear_webhook_deliveries", "linear_comment_cursor", "signals"} {
		if err := collect(table); err != nil {
			return nil, err
		}
	}

	if len(projects) == 0 {
		fallbackProject = strings.TrimSpace(fallbackProject)
		if fallbackProject != "" {
			projects = append(projects, fallbackProject)
		}
	}
	sort.Strings(projects)
	return projects, nil
}

func normalizeMigratedProjectName(project string) string {
	project = strings.TrimSpace(project)
	switch project {
	case "kas":
		return "kasmos"
	default:
		return project
	}
}

// migrateRepoLocalPRReviews copies PR review records from a repo-local DB into
// the global DB via ATTACH. Duplicate reviews (same project + plan_filename +
// review_id) are silently skipped via INSERT OR IGNORE.
func migrateRepoLocalPRReviews(globalDB *sql.DB, localDBPath, sourceProject, targetProject string) error {
	ctx := context.Background()
	const alias = "local_pr"

	conn, err := globalDB.Conn(ctx)
	if err != nil {
		return fmt.Errorf("acquire conn for pr_reviews migration: %w", err)
	}
	defer conn.Close()

	if _, err := conn.ExecContext(ctx, fmt.Sprintf("ATTACH DATABASE %q AS %s", localDBPath, alias)); err != nil {
		return fmt.Errorf("attach local DB for pr_reviews: %w", err)
	}
	defer conn.ExecContext(ctx, fmt.Sprintf("DETACH DATABASE %s", alias)) //nolint:errcheck

	var hasTable int
	if err := conn.QueryRowContext(ctx,
		fmt.Sprintf("SELECT count(*) FROM %s.sqlite_master WHERE type='table' AND name='pr_reviews'", alias),
	).Scan(&hasTable); err != nil {
		return fmt.Errorf("check pr_reviews table in local DB: %w", err)
	}
	if hasTable == 0 {
		return nil
	}

	// pr_reviews has UNIQUE(project, plan_filename, review_id), so OR IGNORE
	// provides idempotency.
	if _, err := conn.ExecContext(ctx, fmt.Sprintf(`
		INSERT OR IGNORE INTO pr_reviews
			(project, plan_filename, review_id, review_state, review_body,
			 reviewer_login, reaction_posted, fixer_dispatched, created_at)
		SELECT ?, plan_filename, review_id, review_state, review_body,
		       reviewer_login, reaction_posted, fixer_dispatched, created_at
		FROM %s.pr_reviews
		WHERE project = ?
	`, alias), targetProject, sourceProject); err != nil {
		return fmt.Errorf("copy pr_reviews: %w", err)
	}
	return nil
}

func migrateRepoLocalLinearTriggers(globalDB *sql.DB, localDBPath, sourceProject, targetProject string) error {
	ctx := context.Background()
	const alias = "local_linear_triggers"

	conn, err := globalDB.Conn(ctx)
	if err != nil {
		return fmt.Errorf("acquire conn for linear_triggers migration: %w", err)
	}
	defer conn.Close()

	if _, err := conn.ExecContext(ctx, fmt.Sprintf("ATTACH DATABASE %q AS %s", localDBPath, alias)); err != nil {
		return fmt.Errorf("attach local DB for linear_triggers: %w", err)
	}
	defer conn.ExecContext(ctx, fmt.Sprintf("DETACH DATABASE %s", alias)) //nolint:errcheck

	var hasTable int
	if err := conn.QueryRowContext(ctx,
		fmt.Sprintf("SELECT count(*) FROM %s.sqlite_master WHERE type='table' AND name='linear_triggers'", alias),
	).Scan(&hasTable); err != nil {
		return fmt.Errorf("check linear_triggers table in local DB: %w", err)
	}
	if hasTable == 0 {
		return nil
	}

	if _, err := conn.ExecContext(ctx, fmt.Sprintf(`
		INSERT OR IGNORE INTO linear_triggers
			(project, linear_issue_id, linear_identifier, command_kind, source_kind,
			 source_id, actor_id, actor_email, task_arg, detected_at, processed,
			 processed_at, outcome, rejection_reason, target_filename, ack_state)
		SELECT ?, linear_issue_id, linear_identifier, command_kind, source_kind,
		       source_id, actor_id, actor_email, task_arg, detected_at, processed,
		       processed_at, outcome, rejection_reason, target_filename, ack_state
		FROM %s.linear_triggers
		WHERE project = ?
	`, alias), targetProject, sourceProject); err != nil {
		return fmt.Errorf("copy linear_triggers: %w", err)
	}
	return nil
}

func migrateRepoLocalLinearWebhookDeliveries(globalDB *sql.DB, localDBPath, sourceProject, targetProject string) error {
	ctx := context.Background()
	const alias = "local_linear_webhook_deliveries"

	conn, err := globalDB.Conn(ctx)
	if err != nil {
		return fmt.Errorf("acquire conn for linear_webhook_deliveries migration: %w", err)
	}
	defer conn.Close()

	if _, err := conn.ExecContext(ctx, fmt.Sprintf("ATTACH DATABASE %q AS %s", localDBPath, alias)); err != nil {
		return fmt.Errorf("attach local DB for linear_webhook_deliveries: %w", err)
	}
	defer conn.ExecContext(ctx, fmt.Sprintf("DETACH DATABASE %s", alias)) //nolint:errcheck

	var hasTable int
	if err := conn.QueryRowContext(ctx,
		fmt.Sprintf("SELECT count(*) FROM %s.sqlite_master WHERE type='table' AND name='linear_webhook_deliveries'", alias),
	).Scan(&hasTable); err != nil {
		return fmt.Errorf("check linear_webhook_deliveries table in local DB: %w", err)
	}
	if hasTable == 0 {
		return nil
	}

	if _, err := conn.ExecContext(ctx, fmt.Sprintf(`
		INSERT OR IGNORE INTO linear_webhook_deliveries
			(project, delivery_id, linear_event, action, linear_issue_id, source_kind,
			 source_id, status, reason, received_at, processed_at)
		SELECT ?, delivery_id, linear_event, action, linear_issue_id, source_kind,
		       source_id, status, reason, received_at, processed_at
		FROM %s.linear_webhook_deliveries
		WHERE project = ?
	`, alias), targetProject, sourceProject); err != nil {
		return fmt.Errorf("copy linear_webhook_deliveries: %w", err)
	}
	return nil
}

func migrateRepoLocalLinearCommentCursors(globalDB *sql.DB, localDBPath, sourceProject, targetProject string) error {
	ctx := context.Background()
	const alias = "local_linear_cursor"

	conn, err := globalDB.Conn(ctx)
	if err != nil {
		return fmt.Errorf("acquire conn for linear_comment_cursor migration: %w", err)
	}
	defer conn.Close()

	if _, err := conn.ExecContext(ctx, fmt.Sprintf("ATTACH DATABASE %q AS %s", localDBPath, alias)); err != nil {
		return fmt.Errorf("attach local DB for linear_comment_cursor: %w", err)
	}
	defer conn.ExecContext(ctx, fmt.Sprintf("DETACH DATABASE %s", alias)) //nolint:errcheck

	var hasTable int
	if err := conn.QueryRowContext(ctx,
		fmt.Sprintf("SELECT count(*) FROM %s.sqlite_master WHERE type='table' AND name='linear_comment_cursor'", alias),
	).Scan(&hasTable); err != nil {
		return fmt.Errorf("check linear_comment_cursor table in local DB: %w", err)
	}
	if hasTable == 0 {
		return nil
	}

	if _, err := conn.ExecContext(ctx, fmt.Sprintf(`
		INSERT INTO linear_comment_cursor (project, linear_issue_id, last_seen_at)
		SELECT ?, linear_issue_id, last_seen_at
		FROM %s.linear_comment_cursor
		WHERE project = ?
		ON CONFLICT (project, linear_issue_id) DO UPDATE SET
			last_seen_at = excluded.last_seen_at
		WHERE excluded.last_seen_at > linear_comment_cursor.last_seen_at
	`, alias), targetProject, sourceProject); err != nil {
		return fmt.Errorf("copy linear_comment_cursor: %w", err)
	}
	return nil
}

// migrateRepoLocalSignals copies signals from a repo-local DB into the global
// DB via ATTACH. Signals are deduplicated by (project, plan_file, signal_type,
// created_at) since the signals table has no natural unique constraint.
// BEGIN IMMEDIATE is used to serialise concurrent migrations so the NOT EXISTS
// check and INSERT are atomic with respect to other writers.
func migrateRepoLocalSignals(globalDB *sql.DB, localDBPath, sourceProject, targetProject string) error {
	ctx := context.Background()
	const alias = "local_sig"

	conn, err := globalDB.Conn(ctx)
	if err != nil {
		return fmt.Errorf("acquire conn for signals migration: %w", err)
	}
	defer conn.Close()

	if _, err := conn.ExecContext(ctx, fmt.Sprintf("ATTACH DATABASE %q AS %s", localDBPath, alias)); err != nil {
		return fmt.Errorf("attach local DB for signals: %w", err)
	}
	defer conn.ExecContext(ctx, fmt.Sprintf("DETACH DATABASE %s", alias)) //nolint:errcheck

	var hasTable int
	if err := conn.QueryRowContext(ctx,
		fmt.Sprintf("SELECT count(*) FROM %s.sqlite_master WHERE type='table' AND name='signals'", alias),
	).Scan(&hasTable); err != nil {
		return fmt.Errorf("check signals table in local DB: %w", err)
	}
	if hasTable == 0 {
		return nil
	}

	// Ensure the global signals table exists (SQLiteStore does not create it;
	// only SQLiteSignalGateway does). This is a no-op when the table already exists.
	if _, err := conn.ExecContext(ctx, signalsSchema); err != nil {
		return fmt.Errorf("ensure signals schema: %w", err)
	}

	// BEGIN IMMEDIATE acquires a write lock immediately, serialising concurrent
	// migrations so the NOT EXISTS deduplication check is concurrency-safe.
	if _, err := conn.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		return fmt.Errorf("begin immediate for signals migration: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			conn.ExecContext(ctx, "ROLLBACK") //nolint:errcheck
		}
	}()

	if _, err := conn.ExecContext(ctx, fmt.Sprintf(`
		INSERT INTO signals
			(project, plan_file, signal_type, payload, status,
			 created_at, claimed_by, claimed_at, processed_at, result)
		SELECT ?, s.plan_file, s.signal_type, s.payload, s.status,
		       s.created_at, s.claimed_by, s.claimed_at, s.processed_at, s.result
		FROM %s.signals s
		WHERE s.project = ?
		AND NOT EXISTS (
			SELECT 1 FROM signals g
			WHERE g.project = ?
			  AND g.plan_file = s.plan_file
			  AND g.signal_type = s.signal_type
			  AND g.created_at = s.created_at
		)
	`, alias), targetProject, sourceProject, targetProject); err != nil {
		return fmt.Errorf("copy signals: %w", err)
	}

	if _, err := conn.ExecContext(ctx, "COMMIT"); err != nil {
		return fmt.Errorf("commit signals migration: %w", err)
	}
	committed = true
	return nil
}

// daemonTOMLRepos holds the minimal subset of daemon.toml needed to enumerate
// registered repos without importing the daemon package.
type daemonTOMLRepos struct {
	Repos []string `toml:"repos"`
}

// MigrateAllKnownRepos migrates repo-local taskstore data into the global
// store for every repo listed in daemon.toml, plus the current repo. Errors
// from individual repos are returned immediately.
func MigrateAllKnownRepos(globalStore Store) error {
	seen := make(map[string]struct{})

	// Scan daemon.toml for registered repos.
	if home, err := os.UserHomeDir(); err == nil {
		tomlPath := filepath.Join(home, ".config", "kasmos", "daemon.toml")
		if data, err := os.ReadFile(tomlPath); err == nil {
			var cfg daemonTOMLRepos
			if _, decErr := toml.Decode(string(data), &cfg); decErr == nil {
				for _, repoPath := range cfg.Repos {
					repoPath = filepath.Clean(repoPath)
					if _, dup := seen[repoPath]; dup {
						continue
					}
					seen[repoPath] = struct{}{}

					project := filepath.Base(repoPath)
					kasmosDir := filepath.Join(repoPath, ".kasmos")
					if _, err := MigrateRepoLocalToGlobal(globalStore, project, kasmosDir); err != nil {
						return fmt.Errorf("migrate repo %s: %w", repoPath, err)
					}
				}
			}
		}
	}

	// Also try the current repo.
	if kasmosDir, err := taskStoreConfigDir(); err == nil {
		repoRoot := filepath.Clean(filepath.Dir(kasmosDir))
		if _, dup := seen[repoRoot]; !dup {
			project := filepath.Base(repoRoot)
			if _, err := MigrateRepoLocalToGlobal(globalStore, project, kasmosDir); err != nil {
				return fmt.Errorf("migrate current repo %s: %w", repoRoot, err)
			}
		}
	}

	return nil
}
