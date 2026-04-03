package taskstore

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/BurntSushi/toml"

	"github.com/kastheco/kasmos/config"
)

// jsonTaskEntry is the on-disk format for a single plan in plan-state.json.
type jsonTaskEntry struct {
	Status      string `json:"status"`
	Description string `json:"description,omitempty"`
	Branch      string `json:"branch,omitempty"`
	Topic       string `json:"topic,omitempty"`
	CreatedAt   string `json:"created_at,omitempty"`
	Implemented string `json:"implemented,omitempty"`
}

// jsonTopicEntry is the on-disk format for a single topic in plan-state.json.
type jsonTopicEntry struct {
	CreatedAt string `json:"created_at"`
}

// jsonTaskState is the top-level structure of plan-state.json.
type jsonTaskState struct {
	Plans  map[string]jsonTaskEntry  `json:"plans"`
	Topics map[string]jsonTopicEntry `json:"topics"`
}

// MigrateFromJSON reads plan-state.json from plansDir and imports all plans
// and topics into the store under the given project. If plan-state.json does
// not exist, it returns (0, nil) — a no-op. The migration is idempotent:
// plans and topics that already exist in the store are silently skipped.
// For each plan entry that has a corresponding plan content file in plansDir, the
// content is also imported via SetContent.
//
// Returns the number of plans successfully migrated (newly created).
func MigrateFromJSON(store Store, project, plansDir string) (int, error) {
	stateFile := filepath.Join(plansDir, "plan-state.json")

	data, err := os.ReadFile(stateFile)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return 0, nil
		}
		return 0, fmt.Errorf("read plan-state.json: %w", err)
	}

	var state jsonTaskState
	if err := json.Unmarshal(data, &state); err != nil {
		return 0, fmt.Errorf("parse plan-state.json: %w", err)
	}

	migrated := 0

	// Migrate plans.
	filenames := make([]string, 0, len(state.Plans))
	for filename := range state.Plans {
		filenames = append(filenames, filename)
	}
	sort.Slice(filenames, func(i, j int) bool {
		left := normalizeMigratedFilename(filenames[i])
		right := normalizeMigratedFilename(filenames[j])
		if left != right {
			return left < right
		}
		leftCanonical := filenames[i] == left
		rightCanonical := filenames[j] == right
		if leftCanonical != rightCanonical {
			return leftCanonical
		}
		return filenames[i] < filenames[j]
	})

	for _, filename := range filenames {
		jp := state.Plans[filename]
		storedFilename := normalizeMigratedFilename(filename)
		entry := TaskEntry{
			Filename:    storedFilename,
			Status:      normalizeMigratedStatus(jp.Status),
			Description: jp.Description,
			Branch:      jp.Branch,
			Topic:       jp.Topic,
			Implemented: jp.Implemented,
		}
		if jp.CreatedAt != "" {
			entry.CreatedAt = parseTime(jp.CreatedAt)
		}

		if err := store.Create(project, entry); err != nil {
			// If the normalized bare slug already exists, keep the colliding legacy
			// .md filename verbatim rather than clobbering the bare task. This mirrors
			// the collision-safe behavior of the durable SQLite migration.
			if strings.Contains(err.Error(), "plan already exists") {
				if storedFilename != filename {
					entry.Filename = filename
					if fallbackErr := store.Create(project, entry); fallbackErr != nil {
						if strings.Contains(fallbackErr.Error(), "plan already exists") {
							continue
						}
						return migrated, fmt.Errorf("migrate plan %s: %w", filename, fallbackErr)
					}
					storedFilename = filename
				} else {
					continue
				}
			} else {
				return migrated, fmt.Errorf("migrate plan %s: %w", filename, err)
			}
		}
		migrated++

		// Import legacy content from the original source filename on disk, then store
		// it under whichever filename was persisted above.
		mdPath := filepath.Join(plansDir, filename)
		content, err := os.ReadFile(mdPath)
		if err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				return migrated, fmt.Errorf("read plan content %s: %w", filename, err)
			}
			// No legacy plan content file — that's fine, content stays empty.
		} else {
			if err := store.SetContent(project, storedFilename, string(content)); err != nil {
				return migrated, fmt.Errorf("set content for %s: %w", storedFilename, err)
			}
		}
	}

	// Migrate topics.
	for name, jt := range state.Topics {
		var createdAt time.Time
		if jt.CreatedAt != "" {
			createdAt = parseTime(jt.CreatedAt)
		}
		entry := TopicEntry{
			Name:      name,
			CreatedAt: createdAt,
		}
		if err := store.CreateTopic(project, entry); err != nil {
			// Skip if already exists (idempotent).
			if strings.Contains(err.Error(), "topic already exists") {
				continue
			}
			return migrated, fmt.Errorf("migrate topic %s: %w", name, err)
		}
	}

	return migrated, nil
}

// migrateFromPlanstoreDB copies data from a legacy planstore.db file into the
// current taskstore.db. This handles the rename-plan-to-task transition where
// the DB filename changed but existing users still have their data in the old
// file.
//
// The migration is idempotent: it only runs when planstore.db exists in the
// same directory as dbPath AND the tasks table in the current DB is empty.
// It copies tasks (from the plans table), topics, and audit_events.
func migrateFromPlanstoreDB(db *sql.DB, dbPath string) error {
	dir := filepath.Dir(dbPath)
	oldDBPath := filepath.Join(dir, "planstore.db")

	// Check if old DB exists.
	if _, err := os.Stat(oldDBPath); err != nil {
		return nil // no old DB — nothing to migrate
	}

	// Check if the new tasks table already has data (skip if so).
	var taskCount int
	if err := db.QueryRow("SELECT count(*) FROM tasks").Scan(&taskCount); err != nil {
		return nil // table might not exist yet — schema hasn't run; caller handles
	}
	if taskCount > 0 {
		return nil // already has data — don't overwrite
	}

	// Attach the old database and copy data.
	attachSQL := fmt.Sprintf("ATTACH DATABASE %q AS old", oldDBPath)
	if _, err := db.Exec(attachSQL); err != nil {
		return fmt.Errorf("attach planstore.db: %w", err)
	}
	defer db.Exec("DETACH DATABASE old") //nolint:errcheck

	// Copy plans → tasks (the old table is named "plans"). Status aliases are
	// normalized here because this is an explicit legacy import boundary. Filename
	// normalization stays in migrateStripMdSuffix so SQLite can keep its OR IGNORE
	// collision handling when both bare and .md-suffixed rows exist.
	if tableExistsInAttached(db, "old", "plans") {
		if _, err := db.Exec(`
			INSERT OR IGNORE INTO tasks (project, filename, status, description, branch, topic, created_at, implemented)
			SELECT project, filename,
				CASE status
					WHEN 'in_progress' THEN 'implementing'
					WHEN 'completed' THEN 'done'
					ELSE status
				END,
				description, branch, topic, created_at, implemented
			FROM old.plans
		`); err != nil {
			return fmt.Errorf("copy plans to tasks: %w", err)
		}

		// Copy content column if it exists in the old table.
		if columnExistsInAttached(db, "old", "plans", "content") {
			if _, err := db.Exec(`
				UPDATE tasks SET content = (
					SELECT old.plans.content FROM old.plans
					WHERE old.plans.project = tasks.project AND old.plans.filename = tasks.filename
				)
				WHERE EXISTS (
					SELECT 1 FROM old.plans
					WHERE old.plans.project = tasks.project AND old.plans.filename = tasks.filename
				)
			`); err != nil {
				// Non-fatal — content is optional.
				_ = err
			}
		}

		// Copy clickup_task_id if it exists.
		if columnExistsInAttached(db, "old", "plans", "clickup_task_id") {
			if _, err := db.Exec(`
				UPDATE tasks SET clickup_task_id = (
					SELECT old.plans.clickup_task_id FROM old.plans
					WHERE old.plans.project = tasks.project AND old.plans.filename = tasks.filename
				)
				WHERE EXISTS (
					SELECT 1 FROM old.plans
					WHERE old.plans.project = tasks.project AND old.plans.filename = tasks.filename
				)
			`); err != nil {
				_ = err
			}
		}

		// Copy review_cycle if it exists.
		if columnExistsInAttached(db, "old", "plans", "review_cycle") {
			if _, err := db.Exec(`
				UPDATE tasks SET review_cycle = (
					SELECT old.plans.review_cycle FROM old.plans
					WHERE old.plans.project = tasks.project AND old.plans.filename = tasks.filename
				)
				WHERE EXISTS (
					SELECT 1 FROM old.plans
					WHERE old.plans.project = tasks.project AND old.plans.filename = tasks.filename
				)
			`); err != nil {
				_ = err
			}
		}

		// Copy pr_url if it exists.
		if columnExistsInAttached(db, "old", "plans", "pr_url") {
			if _, err := db.Exec(`
				UPDATE tasks SET pr_url = (
					SELECT old.plans.pr_url FROM old.plans
					WHERE old.plans.project = tasks.project AND old.plans.filename = tasks.filename
				)
				WHERE EXISTS (
					SELECT 1 FROM old.plans
					WHERE old.plans.project = tasks.project AND old.plans.filename = tasks.filename
				)
			`); err != nil {
				_ = err
			}
		}

		// Copy pr_review_decision if it exists.
		if columnExistsInAttached(db, "old", "plans", "pr_review_decision") {
			if _, err := db.Exec(`
				UPDATE tasks SET pr_review_decision = (
					SELECT old.plans.pr_review_decision FROM old.plans
					WHERE old.plans.project = tasks.project AND old.plans.filename = tasks.filename
				)
				WHERE EXISTS (
					SELECT 1 FROM old.plans
					WHERE old.plans.project = tasks.project AND old.plans.filename = tasks.filename
				)
			`); err != nil {
				_ = err
			}
		}

		// Copy pr_check_status if it exists.
		if columnExistsInAttached(db, "old", "plans", "pr_check_status") {
			if _, err := db.Exec(`
				UPDATE tasks SET pr_check_status = (
					SELECT old.plans.pr_check_status FROM old.plans
					WHERE old.plans.project = tasks.project AND old.plans.filename = tasks.filename
				)
				WHERE EXISTS (
					SELECT 1 FROM old.plans
					WHERE old.plans.project = tasks.project AND old.plans.filename = tasks.filename
				)
			`); err != nil {
				_ = err
			}
		}
	}

	// Copy topics.
	if tableExistsInAttached(db, "old", "topics") {
		if _, err := db.Exec(`
			INSERT OR IGNORE INTO topics (project, name, created_at)
			SELECT project, name, created_at
			FROM old.topics
		`); err != nil {
			return fmt.Errorf("copy topics: %w", err)
		}
	}

	// Copy audit_events (the auditlog package shares the same DB file).
	if tableExistsInAttached(db, "old", "audit_events") {
		// Ensure audit_events table exists in the new DB.
		if _, err := db.Exec(`
			CREATE TABLE IF NOT EXISTS audit_events (
				id             INTEGER PRIMARY KEY,
				kind           TEXT    NOT NULL,
				timestamp      TEXT    NOT NULL,
				project        TEXT    NOT NULL DEFAULT '',
				plan_file      TEXT    NOT NULL DEFAULT '',
				instance_title TEXT    NOT NULL DEFAULT '',
				agent_type     TEXT    NOT NULL DEFAULT '',
				wave_number    INTEGER NOT NULL DEFAULT 0,
				task_number    INTEGER NOT NULL DEFAULT 0,
				message        TEXT    NOT NULL DEFAULT '',
				detail         TEXT    NOT NULL DEFAULT '',
				level          TEXT    NOT NULL DEFAULT 'info'
			)
		`); err != nil {
			return fmt.Errorf("create audit_events table: %w", err)
		}

		if _, err := db.Exec(`
			INSERT INTO audit_events (kind, timestamp, project, plan_file, instance_title, agent_type, wave_number, task_number, message, detail, level)
			SELECT kind, timestamp, project, plan_file, instance_title, agent_type, wave_number, task_number, message, detail, level
			FROM old.audit_events
		`); err != nil {
			return fmt.Errorf("copy audit_events: %w", err)
		}
	}

	return nil
}

// MigrateRepoLocalToGlobal copies tasks, content, subtasks, topics, PR reviews,
// and signals from a repo-local taskstore.db into the global store. If the
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

	tasks, err := localStore.List(project)
	if err != nil {
		localStore.Close()
		return 0, fmt.Errorf("list repo-local tasks: %w", err)
	}

	migrated := 0
	for _, task := range tasks {
		// INSERT OR IGNORE semantics: skip tasks that already exist.
		if err := globalStore.Create(project, task); err != nil {
			if strings.Contains(err.Error(), "plan already exists") {
				// Already in global store — fall through to subtask migration.
			} else {
				localStore.Close()
				return migrated, fmt.Errorf("migrate task %s: %w", task.Filename, err)
			}
		} else {
			migrated++
		}

		// Migrate subtasks (SetSubtasks replaces all — idempotent).
		if subtasks, subErr := localStore.GetSubtasks(project, task.Filename); subErr == nil && len(subtasks) > 0 {
			_ = globalStore.SetSubtasks(project, task.Filename, subtasks)
		}
	}

	// Migrate topics.
	if topics, err := localStore.ListTopics(project); err == nil {
		for _, topic := range topics {
			if err := globalStore.CreateTopic(project, topic); err != nil {
				if !strings.Contains(err.Error(), "topic already exists") {
					localStore.Close()
					return migrated, fmt.Errorf("migrate topic %s: %w", topic.Name, err)
				}
			}
		}
	}

	// Close local store before ATTACH to avoid WAL lock contention.
	localStore.Close()

	// Bulk-copy PR reviews and signals via ATTACH when global store is SQLite.
	if globalSQLite, ok := globalStore.(*SQLiteStore); ok {
		migrateRepoLocalPRReviews(globalSQLite.db, localDBPath, project)
		migrateRepoLocalSignals(globalSQLite.db, localDBPath, project)
	}

	return migrated, nil
}

// migrateRepoLocalPRReviews copies PR review records from a repo-local DB into
// the global DB via ATTACH. Duplicate reviews (same project + plan_filename +
// review_id) are silently skipped via INSERT OR IGNORE.
func migrateRepoLocalPRReviews(globalDB *sql.DB, localDBPath, project string) {
	const alias = "local_pr"
	if _, err := globalDB.Exec(fmt.Sprintf("ATTACH DATABASE %q AS %s", localDBPath, alias)); err != nil {
		return
	}
	defer globalDB.Exec(fmt.Sprintf("DETACH DATABASE %s", alias)) //nolint:errcheck

	if !tableExistsInAttached(globalDB, alias, "pr_reviews") {
		return
	}

	// pr_reviews has UNIQUE(project, plan_filename, review_id), so OR IGNORE
	// provides idempotency.
	_, _ = globalDB.Exec(fmt.Sprintf(`
		INSERT OR IGNORE INTO pr_reviews
			(project, plan_filename, review_id, review_state, review_body,
			 reviewer_login, reaction_posted, fixer_dispatched, created_at)
		SELECT project, plan_filename, review_id, review_state, review_body,
		       reviewer_login, reaction_posted, fixer_dispatched, created_at
		FROM %s.pr_reviews
		WHERE project = ?
	`, alias), project)
}

// migrateRepoLocalSignals copies signals from a repo-local DB into the global
// DB via ATTACH. Signals are deduplicated by (project, plan_file, signal_type,
// created_at) since the signals table has no natural unique constraint.
func migrateRepoLocalSignals(globalDB *sql.DB, localDBPath, project string) {
	const alias = "local_sig"
	if _, err := globalDB.Exec(fmt.Sprintf("ATTACH DATABASE %q AS %s", localDBPath, alias)); err != nil {
		return
	}
	defer globalDB.Exec(fmt.Sprintf("DETACH DATABASE %s", alias)) //nolint:errcheck

	if !tableExistsInAttached(globalDB, alias, "signals") {
		return
	}

	// Ensure the global signals table exists (SQLiteStore does not create it;
	// only SQLiteSignalGateway does).
	_, _ = globalDB.Exec(signalsSchema)

	_, _ = globalDB.Exec(fmt.Sprintf(`
		INSERT INTO signals
			(project, plan_file, signal_type, payload, status,
			 created_at, claimed_by, claimed_at, processed_at, result)
		SELECT s.project, s.plan_file, s.signal_type, s.payload, s.status,
		       s.created_at, s.claimed_by, s.claimed_at, s.processed_at, s.result
		FROM %s.signals s
		WHERE s.project = ?
		AND NOT EXISTS (
			SELECT 1 FROM signals g
			WHERE g.project = s.project
			  AND g.plan_file = s.plan_file
			  AND g.signal_type = s.signal_type
			  AND g.created_at = s.created_at
		)
	`, alias), project)
}

// daemonTOMLRepos holds the minimal subset of daemon.toml needed to enumerate
// registered repos without importing the daemon package.
type daemonTOMLRepos struct {
	Repos []string `toml:"repos"`
}

// MigrateAllKnownRepos migrates repo-local taskstore data into the global
// store for every repo listed in daemon.toml, plus the current repo (via
// config.GetConfigDir). Errors from individual repos are returned immediately.
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
	if kasmosDir, err := config.GetConfigDir(); err == nil {
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

func normalizeMigratedFilename(filename string) string {
	return strings.TrimSuffix(filename, ".md")
}

func normalizeMigratedStatus(status string) Status {
	switch status {
	case "in_progress":
		return StatusImplementing
	case "completed":
		return StatusDone
	default:
		return Status(status)
	}
}

// tableExistsInAttached checks if a table exists in an attached database.
func tableExistsInAttached(db *sql.DB, schema, tableName string) bool {
	var count int
	err := db.QueryRow(
		fmt.Sprintf("SELECT count(*) FROM %s.sqlite_master WHERE type='table' AND name=?", schema),
		tableName,
	).Scan(&count)
	return err == nil && count > 0
}

// columnExistsInAttached checks if a column exists in a table in an attached database.
func columnExistsInAttached(db *sql.DB, schema, tableName, columnName string) bool {
	rows, err := db.Query(fmt.Sprintf("PRAGMA %s.table_info(%s)", schema, tableName))
	if err != nil {
		return false
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name, colType string
		var notNull int
		var dfltValue sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &colType, &notNull, &dfltValue, &pk); err != nil {
			return false
		}
		if name == columnName {
			return true
		}
	}
	return false
}
