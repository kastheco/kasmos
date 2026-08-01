package taskstore

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite" // register sqlite driver
)

const schema = `
CREATE TABLE IF NOT EXISTS tasks (
	id          INTEGER PRIMARY KEY,
	project     TEXT    NOT NULL,
	filename    TEXT    NOT NULL,
	status      TEXT    NOT NULL DEFAULT 'ready',
	description TEXT    NOT NULL DEFAULT '',
	branch      TEXT    NOT NULL DEFAULT '',
	topic       TEXT    NOT NULL DEFAULT '',
	created_at  TEXT    NOT NULL DEFAULT '',
	implemented TEXT    NOT NULL DEFAULT '',
	planning_at  TEXT    NOT NULL DEFAULT '',
	implementing_at TEXT NOT NULL DEFAULT '',
	reviewing_at TEXT    NOT NULL DEFAULT '',
	verifying_at TEXT    NOT NULL DEFAULT '',
	done_at     TEXT    NOT NULL DEFAULT '',
	execution_phase   TEXT    NOT NULL DEFAULT '',
	active_agent_type TEXT    NOT NULL DEFAULT '',
	active_wave       INTEGER NOT NULL DEFAULT 0,
	goal                TEXT    NOT NULL DEFAULT '',
	latest_review_feedback TEXT NOT NULL DEFAULT '',
	pr_url              TEXT    NOT NULL DEFAULT '',
	pr_review_decision  TEXT    NOT NULL DEFAULT '',
	pr_check_status     TEXT    NOT NULL DEFAULT '',
	pr_create_state TEXT NOT NULL DEFAULT '',
	pr_create_error TEXT NOT NULL DEFAULT '',
	pr_create_attempts INTEGER NOT NULL DEFAULT 0,
	pr_create_attempted_at TEXT NOT NULL DEFAULT '',
	verified_sha TEXT NOT NULL DEFAULT '',
	verified_base_sha TEXT NOT NULL DEFAULT '',
	verified_at TEXT NOT NULL DEFAULT '',
	verified_by TEXT NOT NULL DEFAULT '',
	stale_verification_reason TEXT NOT NULL DEFAULT '',
	blocked_reason TEXT NOT NULL DEFAULT '',
	blocked_source TEXT NOT NULL DEFAULT '',
	blocked_at TEXT NOT NULL DEFAULT '',
	UNIQUE(project, filename)
);

CREATE TABLE IF NOT EXISTS topics (
	id         INTEGER PRIMARY KEY,
	project    TEXT NOT NULL,
	name       TEXT NOT NULL,
	created_at TEXT NOT NULL DEFAULT '',
	UNIQUE(project, name)
);
`

// prReviewsTableMigration creates the pr_reviews table for tracking processed PR review comments.
const prReviewsTableMigration = `
	CREATE TABLE IF NOT EXISTS pr_reviews (
		id               INTEGER PRIMARY KEY,
		project          TEXT    NOT NULL,
		plan_filename    TEXT    NOT NULL,
		review_id        INTEGER NOT NULL,
		review_state     TEXT    NOT NULL DEFAULT '',
		review_body      TEXT    NOT NULL DEFAULT '',
		reviewer_login   TEXT    NOT NULL DEFAULT '',
		reaction_posted  INTEGER NOT NULL DEFAULT 0,
		fixer_dispatched INTEGER NOT NULL DEFAULT 0,
		created_at       TEXT    NOT NULL DEFAULT '',
		UNIQUE(project, plan_filename, review_id),
		FOREIGN KEY (project, plan_filename) REFERENCES tasks(project, filename) ON DELETE CASCADE
	)
`

const linearTriggersTableMigration = `
	CREATE TABLE IF NOT EXISTS linear_triggers (
		id                INTEGER PRIMARY KEY,
		project           TEXT    NOT NULL,
		linear_issue_id   TEXT    NOT NULL,
		linear_identifier TEXT    NOT NULL DEFAULT '',
		command_kind      TEXT    NOT NULL,
		source_kind       TEXT    NOT NULL,
		source_id         TEXT    NOT NULL,
		actor_id          TEXT    NOT NULL DEFAULT '',
		actor_email       TEXT    NOT NULL DEFAULT '',
		task_arg          TEXT    NOT NULL DEFAULT '',
		detected_at       TEXT    NOT NULL,
		processed         INTEGER NOT NULL DEFAULT 0,
		processed_at      TEXT    NOT NULL DEFAULT '',
		outcome           TEXT    NOT NULL DEFAULT '',
		rejection_reason  TEXT    NOT NULL DEFAULT '',
		target_filename   TEXT    NOT NULL DEFAULT '',
		ack_state         TEXT    NOT NULL DEFAULT '',
		UNIQUE (project, linear_issue_id, command_kind, source_id)
	)`

const linearTriggersUnprocessedIndex = `CREATE INDEX IF NOT EXISTS idx_linear_triggers_unprocessed ON linear_triggers(processed, detected_at)`

const linearWebhookDeliveriesTableMigration = `
	CREATE TABLE IF NOT EXISTS linear_webhook_deliveries (
		id              INTEGER PRIMARY KEY,
		project         TEXT NOT NULL,
		delivery_id     TEXT NOT NULL,
		linear_event    TEXT NOT NULL DEFAULT '',
		action          TEXT NOT NULL DEFAULT '',
		linear_issue_id TEXT NOT NULL DEFAULT '',
		source_kind     TEXT NOT NULL DEFAULT '',
		source_id       TEXT NOT NULL DEFAULT '',
		status          TEXT NOT NULL,
		reason          TEXT NOT NULL DEFAULT '',
		received_at     TEXT NOT NULL,
		processed_at    TEXT NOT NULL DEFAULT '',
		UNIQUE (project, delivery_id)
	)`

const linearWebhookDeliveriesReceivedIndex = `CREATE INDEX IF NOT EXISTS idx_linear_webhook_deliveries_received ON linear_webhook_deliveries(project, received_at DESC)`

const linearCommentCursorTableMigration = `
	CREATE TABLE IF NOT EXISTS linear_comment_cursor (
		project         TEXT NOT NULL,
		linear_issue_id TEXT NOT NULL,
		last_seen_at    TEXT NOT NULL,
		PRIMARY KEY (project, linear_issue_id)
	)`

// subtasksTableMigration creates the subtasks table for persisted plan subtasks.
const subtasksTableMigration = `
	CREATE TABLE IF NOT EXISTS subtasks (
		id INTEGER PRIMARY KEY,
		project TEXT NOT NULL,
		plan_filename TEXT NOT NULL,
		task_number INTEGER NOT NULL,
		title TEXT NOT NULL DEFAULT '',
		status TEXT NOT NULL DEFAULT 'pending',
		UNIQUE(project, plan_filename, task_number),
		FOREIGN KEY (project, plan_filename) REFERENCES tasks(project, filename) ON DELETE CASCADE
	)
`

// contentMigration adds the content column to existing databases that predate it.
const contentMigration = `ALTER TABLE tasks ADD COLUMN content TEXT NOT NULL DEFAULT ''`

// planningAtMigration adds planning_at to existing databases.
const planningAtMigration = `ALTER TABLE tasks ADD COLUMN planning_at TEXT NOT NULL DEFAULT ''`

// implementingAtMigration adds implementing_at to existing databases.
const implementingAtMigration = `ALTER TABLE tasks ADD COLUMN implementing_at TEXT NOT NULL DEFAULT ''`

// reviewingAtMigration adds reviewing_at to existing databases.
const reviewingAtMigration = `ALTER TABLE tasks ADD COLUMN reviewing_at TEXT NOT NULL DEFAULT ''`

// verifyingAtMigration adds verifying_at to existing databases.
const verifyingAtMigration = `ALTER TABLE tasks ADD COLUMN verifying_at TEXT NOT NULL DEFAULT ''`

// doneAtMigration adds done_at to existing databases.
const doneAtMigration = `ALTER TABLE tasks ADD COLUMN done_at TEXT NOT NULL DEFAULT ''`

// executionPhaseMigration adds execution_phase to existing databases.
const executionPhaseMigration = `ALTER TABLE tasks ADD COLUMN execution_phase TEXT NOT NULL DEFAULT ''`

// activeAgentTypeMigration adds active_agent_type to existing databases.
const activeAgentTypeMigration = `ALTER TABLE tasks ADD COLUMN active_agent_type TEXT NOT NULL DEFAULT ''`

// activeWaveMigration adds active_wave to existing databases.
const activeWaveMigration = `ALTER TABLE tasks ADD COLUMN active_wave INTEGER NOT NULL DEFAULT 0`

// goalMigration adds goal to existing databases.
const goalMigration = `ALTER TABLE tasks ADD COLUMN goal TEXT NOT NULL DEFAULT ''`

// clickupTaskIDMigration adds the clickup_task_id column to existing databases.
const clickupTaskIDMigration = `ALTER TABLE tasks ADD COLUMN clickup_task_id TEXT NOT NULL DEFAULT ''`

// linearIssueIDMigration adds the linear_issue_id column to existing databases.
const linearIssueIDMigration = `ALTER TABLE tasks ADD COLUMN linear_issue_id TEXT NOT NULL DEFAULT ''`

// linearIdentifierMigration adds the linear_identifier column to existing databases.
const linearIdentifierMigration = `ALTER TABLE tasks ADD COLUMN linear_identifier TEXT NOT NULL DEFAULT ''`

// linearURLMigration adds the linear_url column to existing databases.
const linearURLMigration = `ALTER TABLE tasks ADD COLUMN linear_url TEXT NOT NULL DEFAULT ''`

// linearTeamKeyMigration adds the linear_team_key column to existing databases.
const linearTeamKeyMigration = `ALTER TABLE tasks ADD COLUMN linear_team_key TEXT NOT NULL DEFAULT ''`

// linearProjectIDMigration adds the linear_project_id column to existing databases.
const linearProjectIDMigration = `ALTER TABLE tasks ADD COLUMN linear_project_id TEXT NOT NULL DEFAULT ''`

const linearIssueIDIndexMigration = `CREATE INDEX IF NOT EXISTS idx_tasks_linear_issue_id ON tasks(project, linear_issue_id)`

// reviewCycleMigration adds the review_cycle column to existing databases.
const reviewCycleMigration = `ALTER TABLE tasks ADD COLUMN review_cycle INTEGER NOT NULL DEFAULT 0`

// latestReviewFeedbackMigration adds the latest_review_feedback column to existing databases.
const latestReviewFeedbackMigration = `ALTER TABLE tasks ADD COLUMN latest_review_feedback TEXT NOT NULL DEFAULT ''`

// prURLMigration adds the pr_url column to existing databases.
const prURLMigration = `ALTER TABLE tasks ADD COLUMN pr_url TEXT NOT NULL DEFAULT ''`

// prReviewDecisionMigration adds the pr_review_decision column to existing databases.
const prReviewDecisionMigration = `ALTER TABLE tasks ADD COLUMN pr_review_decision TEXT NOT NULL DEFAULT ''`

// prCheckStatusMigration adds the pr_check_status column to existing databases.
const prCheckStatusMigration = `ALTER TABLE tasks ADD COLUMN pr_check_status TEXT NOT NULL DEFAULT ''`
const prCreateStateMigration = `ALTER TABLE tasks ADD COLUMN pr_create_state TEXT NOT NULL DEFAULT ''`
const prCreateErrorMigration = `ALTER TABLE tasks ADD COLUMN pr_create_error TEXT NOT NULL DEFAULT ''`
const prCreateAttemptsMigration = `ALTER TABLE tasks ADD COLUMN pr_create_attempts INTEGER NOT NULL DEFAULT 0`
const prCreateAttemptedAtMigration = `ALTER TABLE tasks ADD COLUMN pr_create_attempted_at TEXT NOT NULL DEFAULT ''`
const verifiedSHAMigration = `ALTER TABLE tasks ADD COLUMN verified_sha TEXT NOT NULL DEFAULT ''`
const verifiedBaseSHAMigration = `ALTER TABLE tasks ADD COLUMN verified_base_sha TEXT NOT NULL DEFAULT ''`
const verifiedAtMigration = `ALTER TABLE tasks ADD COLUMN verified_at TEXT NOT NULL DEFAULT ''`
const verifiedByMigration = `ALTER TABLE tasks ADD COLUMN verified_by TEXT NOT NULL DEFAULT ''`
const staleVerificationReasonMigration = `ALTER TABLE tasks ADD COLUMN stale_verification_reason TEXT NOT NULL DEFAULT ''`

// Blocked-state columns. A non-empty blocked_reason means the task is waiting on
// a human decision and no agent may be spawned for it until the block clears.
const blockedReasonMigration = `ALTER TABLE tasks ADD COLUMN blocked_reason TEXT NOT NULL DEFAULT ''`
const blockedSourceMigration = `ALTER TABLE tasks ADD COLUMN blocked_source TEXT NOT NULL DEFAULT ''`
const blockedAtMigration = `ALTER TABLE tasks ADD COLUMN blocked_at TEXT NOT NULL DEFAULT ''`

// SQLiteStore is a Store implementation backed by a SQLite database.
type SQLiteStore struct {
	db     *sql.DB
	ownsDB bool // when true, Close() closes the underlying *sql.DB
}

// NewSQLiteStore opens (or creates) a SQLite database at dbPath and runs
// schema migrations. Use ":memory:" for an in-memory database (useful in tests).
// DSN PRAGMAs (WAL, busy_timeout, foreign_keys, synchronous, txlock=immediate)
// are applied to every pooled connection via buildSQLiteDSN.
func NewSQLiteStore(dbPath string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite", buildSQLiteDSN(dbPath))
	if err != nil {
		return nil, fmt.Errorf("open sqlite db: %w", err)
	}
	limitMemorySQLitePool(db, dbPath)

	if err := runStoreMigrations(db); err != nil {
		db.Close()
		return nil, err
	}

	// Strip .md suffix so imported filenames are normalized.
	// This is idempotent — safe to run on already-clean DBs.
	if err := migrateStripMdSuffix(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate strip .md suffix: %w", err)
	}

	return &SQLiteStore{db: db, ownsDB: true}, nil
}

// NewSQLiteStoreFromDB creates a SQLiteStore using an existing *sql.DB
// connection pool. The caller is responsible for setting PRAGMAs (WAL,
// busy_timeout, foreign_keys) before calling this — only schema migrations
// are run. Close() on the returned store is a no-op; the caller owns the
// *sql.DB lifecycle.
func NewSQLiteStoreFromDB(db *sql.DB) (*SQLiteStore, error) {
	if err := runStoreMigrations(db); err != nil {
		return nil, err
	}
	return &SQLiteStore{db: db, ownsDB: false}, nil
}

// runStoreMigrations runs all schema migrations on the given *sql.DB.
// It does NOT set PRAGMAs or open the connection — the caller must do that.
func runStoreMigrations(db *sql.DB) error {
	// Migrate: rename plans → tasks (if old table exists).
	migrateRenameTable(db, "plans", "tasks")

	if _, err := db.Exec(schema); err != nil {
		return fmt.Errorf("run schema migrations: %w", err)
	}

	// Add content column if it doesn't exist.
	if err := migrateAddContentColumn(db); err != nil {
		return fmt.Errorf("migrate content column: %w", err)
	}

	if err := migrateAddColumn(db, "clickup_task_id", clickupTaskIDMigration); err != nil {
		return fmt.Errorf("migrate clickup_task_id column: %w", err)
	}
	if err := migrateAddColumn(db, "linear_issue_id", linearIssueIDMigration); err != nil {
		return fmt.Errorf("migrate linear_issue_id column: %w", err)
	}
	if err := migrateAddColumn(db, "linear_identifier", linearIdentifierMigration); err != nil {
		return fmt.Errorf("migrate linear_identifier column: %w", err)
	}
	if err := migrateAddColumn(db, "linear_url", linearURLMigration); err != nil {
		return fmt.Errorf("migrate linear_url column: %w", err)
	}
	if err := migrateAddColumn(db, "linear_team_key", linearTeamKeyMigration); err != nil {
		return fmt.Errorf("migrate linear_team_key column: %w", err)
	}
	if err := migrateAddColumn(db, "linear_project_id", linearProjectIDMigration); err != nil {
		return fmt.Errorf("migrate linear_project_id column: %w", err)
	}
	if _, err := db.Exec(linearIssueIDIndexMigration); err != nil {
		return fmt.Errorf("migrate linear_issue_id index: %w", err)
	}
	if err := migrateAddColumn(db, "review_cycle", reviewCycleMigration); err != nil {
		return fmt.Errorf("migrate review_cycle column: %w", err)
	}
	if err := migrateAddColumn(db, "latest_review_feedback", latestReviewFeedbackMigration); err != nil {
		return fmt.Errorf("migrate latest_review_feedback column: %w", err)
	}
	if err := migrateAddColumn(db, "planning_at", planningAtMigration); err != nil {
		return fmt.Errorf("migrate planning_at column: %w", err)
	}
	if err := migrateAddColumn(db, "implementing_at", implementingAtMigration); err != nil {
		return fmt.Errorf("migrate implementing_at column: %w", err)
	}
	if err := migrateAddColumn(db, "reviewing_at", reviewingAtMigration); err != nil {
		return fmt.Errorf("migrate reviewing_at column: %w", err)
	}
	if err := migrateAddColumn(db, "verifying_at", verifyingAtMigration); err != nil {
		return fmt.Errorf("migrate verifying_at column: %w", err)
	}
	if err := migrateAddColumn(db, "done_at", doneAtMigration); err != nil {
		return fmt.Errorf("migrate done_at column: %w", err)
	}
	if err := migrateAddColumn(db, "execution_phase", executionPhaseMigration); err != nil {
		return fmt.Errorf("migrate execution_phase column: %w", err)
	}
	if err := migrateAddColumn(db, "active_agent_type", activeAgentTypeMigration); err != nil {
		return fmt.Errorf("migrate active_agent_type column: %w", err)
	}
	if err := migrateAddColumn(db, "active_wave", activeWaveMigration); err != nil {
		return fmt.Errorf("migrate active_wave column: %w", err)
	}
	if err := migrateAddColumn(db, "goal", goalMigration); err != nil {
		return fmt.Errorf("migrate goal column: %w", err)
	}
	if err := migrateAddColumn(db, "pr_url", prURLMigration); err != nil {
		return fmt.Errorf("migrate pr_url column: %w", err)
	}
	if err := migrateAddColumn(db, "pr_review_decision", prReviewDecisionMigration); err != nil {
		return fmt.Errorf("migrate pr_review_decision column: %w", err)
	}
	if err := migrateAddColumn(db, "pr_check_status", prCheckStatusMigration); err != nil {
		return fmt.Errorf("migrate pr_check_status column: %w", err)
	}
	for _, m := range []struct{ name, query string }{{"pr_create_state", prCreateStateMigration}, {"pr_create_error", prCreateErrorMigration}, {"pr_create_attempts", prCreateAttemptsMigration}, {"pr_create_attempted_at", prCreateAttemptedAtMigration}} {
		if err := migrateAddColumn(db, m.name, m.query); err != nil {
			return fmt.Errorf("migrate %s column: %w", m.name, err)
		}
	}
	for _, migration := range []struct{ column, query string }{
		{"verified_sha", verifiedSHAMigration}, {"verified_base_sha", verifiedBaseSHAMigration},
		{"verified_at", verifiedAtMigration}, {"verified_by", verifiedByMigration},
		{"stale_verification_reason", staleVerificationReasonMigration},
		{"blocked_reason", blockedReasonMigration}, {"blocked_source", blockedSourceMigration},
		{"blocked_at", blockedAtMigration},
	} {
		if err := migrateAddColumn(db, migration.column, migration.query); err != nil {
			return fmt.Errorf("migrate %s column: %w", migration.column, err)
		}
	}
	if _, err := db.Exec(subtasksTableMigration); err != nil {
		return fmt.Errorf("create subtasks table: %w", err)
	}
	if _, err := db.Exec(prReviewsTableMigration); err != nil {
		return fmt.Errorf("create pr_reviews table: %w", err)
	}
	if _, err := db.Exec(linearTriggersTableMigration); err != nil {
		return fmt.Errorf("create linear_triggers table: %w", err)
	}
	if _, err := db.Exec(linearTriggersUnprocessedIndex); err != nil {
		return fmt.Errorf("create linear_triggers unprocessed index: %w", err)
	}
	if _, err := db.Exec(linearWebhookDeliveriesTableMigration); err != nil {
		return fmt.Errorf("create linear_webhook_deliveries table: %w", err)
	}
	if _, err := db.Exec(linearWebhookDeliveriesReceivedIndex); err != nil {
		return fmt.Errorf("create linear_webhook_deliveries received index: %w", err)
	}
	if _, err := db.Exec(linearCommentCursorTableMigration); err != nil {
		return fmt.Errorf("create linear_comment_cursor table: %w", err)
	}
	// Strip .md suffix so stored filenames are normalized. Called here so that
	// NewSQLiteStoreFromDB also benefits when operating on a shared connection.
	if err := migrateStripMdSuffix(db); err != nil {
		return fmt.Errorf("migrate strip .md suffix: %w", err)
	}
	return nil
}

// migrateAddContentColumn adds the content column to the tasks table if it
// doesn't already exist. This upgrades databases created before the column
// was introduced.
func migrateAddContentColumn(db *sql.DB) error {
	return migrateAddColumn(db, "content", contentMigration)
}

// migrateStripMdSuffix removes a trailing '.md' suffix from task and subtask
// plan filenames already persisted in SQLite. This is the durable compatibility
// boundary for extension-less store keys after legacy DB imports have copied raw
// filenames forward. Deeper FSM/signal layers preserve whatever filename they
// receive and rely on CLI/MCP ingress or this migration for normalization.
// OR IGNORE skips any row where stripping '.md' would collide with an
// already-existing bare-slug entry, so the migration is safe to run on
// databases that were partially updated.
func migrateStripMdSuffix(db *sql.DB) error {
	// Short-circuit when there is nothing to migrate so we don't open a write
	// transaction on an already-clean DB. Opening a write tx fails with SQLite
	// error 8 (SQLITE_READONLY) when the caller is running under a filesystem
	// sandbox that only permits reads (observed with `kas task show` invoked
	// from inside the codex SDK sandbox). Counting via a read-only SELECT is
	// safe in that environment because WAL reads don't require write access.
	var pending int
	if err := db.QueryRow(
		"SELECT " +
			"(SELECT COUNT(*) FROM tasks WHERE filename LIKE '%.md') + " +
			"(SELECT COUNT(*) FROM subtasks WHERE plan_filename LIKE '%.md')",
	).Scan(&pending); err != nil {
		return fmt.Errorf("count .md-suffixed rows: %w", err)
	}
	if pending == 0 {
		return nil
	}

	tx, err := db.Begin()
	if err != nil {
		if isSQLiteReadOnlyError(err) {
			return nil
		}
		return fmt.Errorf("begin strip .md suffix transaction: %w", err)
	}

	if _, err = tx.Exec("PRAGMA defer_foreign_keys = ON"); err != nil {
		_ = tx.Rollback()
		if isSQLiteReadOnlyError(err) {
			return nil
		}
		return fmt.Errorf("defer foreign keys for strip .md migration: %w", err)
	}

	if _, err = tx.Exec("UPDATE OR IGNORE tasks SET filename = SUBSTR(filename, 1, LENGTH(filename) - 3) WHERE filename LIKE '%.md'"); err != nil {
		_ = tx.Rollback()
		if isSQLiteReadOnlyError(err) {
			return nil
		}
		return fmt.Errorf("strip .md suffix from tasks: %w", err)
	}

	if _, err = tx.Exec("UPDATE OR IGNORE subtasks SET plan_filename = SUBSTR(plan_filename, 1, LENGTH(plan_filename) - 3) WHERE plan_filename LIKE '%.md'"); err != nil {
		_ = tx.Rollback()
		if isSQLiteReadOnlyError(err) {
			return nil
		}
		return fmt.Errorf("strip .md suffix from subtasks: %w", err)
	}

	if err = tx.Commit(); err != nil {
		if isSQLiteReadOnlyError(err) {
			return nil
		}
		return fmt.Errorf("commit strip .md suffix migration: %w", err)
	}

	return nil
}

func isSQLiteReadOnlyError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "readonly") || strings.Contains(msg, "read-only") || strings.Contains(msg, "attempt to write a readonly database")
}

// migrateAddColumn adds a column to the tasks table if it doesn't already
// exist, running the provided ALTER TABLE statement when needed.
func migrateAddColumn(db *sql.DB, columnName, alterSQL string) error {
	rows, err := db.Query("PRAGMA table_info(tasks)")
	if err != nil {
		return fmt.Errorf("query table info: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name, colType string
		var notNull int
		var dfltValue sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &colType, &notNull, &dfltValue, &pk); err != nil {
			return fmt.Errorf("scan table info: %w", err)
		}
		if name == columnName {
			return nil // column already exists
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate table info: %w", err)
	}

	// Column doesn't exist — add it.
	if _, err := db.Exec(alterSQL); err != nil {
		return fmt.Errorf("add %s column: %w", columnName, err)
	}
	return nil
}

// migrateRenameTable renames oldName to newName if the old table exists and
// the new table does not. This is idempotent: subsequent runs are no-ops.
func migrateRenameTable(db *sql.DB, oldName, newName string) {
	// Check if old table exists.
	var count int
	err := db.QueryRow("SELECT count(*) FROM sqlite_master WHERE type='table' AND name=?", oldName).Scan(&count)
	if err != nil || count == 0 {
		return // old table doesn't exist, nothing to migrate
	}
	// Check if new table already exists.
	err = db.QueryRow("SELECT count(*) FROM sqlite_master WHERE type='table' AND name=?", newName).Scan(&count)
	if err != nil || count > 0 {
		return // new table already exists
	}
	_, _ = db.Exec(fmt.Sprintf("ALTER TABLE %s RENAME TO %s", oldName, newName))
}

// Close releases the database connection.
func (s *SQLiteStore) Close() error {
	if !s.ownsDB {
		return nil
	}
	return s.db.Close()
}

// Ping verifies the database connection is alive.
func (s *SQLiteStore) Ping() error {
	return s.db.Ping()
}

// Create inserts a new task entry for the given project.
// Returns an error if a task with the same filename already exists in the project.
func (s *SQLiteStore) Create(project string, entry TaskEntry) error {
	const q = `
			INSERT INTO tasks (project, filename, status, description, branch, topic, created_at, implemented, planning_at, implementing_at, reviewing_at, verifying_at, done_at, execution_phase, active_agent_type, active_wave, goal, content, clickup_task_id, linear_issue_id, linear_identifier, linear_url, linear_team_key, linear_project_id, review_cycle, latest_review_feedback, pr_url, pr_review_decision, pr_check_status, pr_create_state, pr_create_error, pr_create_attempts, pr_create_attempted_at, verified_sha, verified_base_sha, verified_at, verified_by, stale_verification_reason, blocked_reason, blocked_source, blocked_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	_, err := s.db.Exec(q,
		project,
		entry.Filename,
		string(entry.Status),
		entry.Description,
		entry.Branch,
		entry.Topic,
		formatTime(entry.CreatedAt),
		entry.Implemented,
		formatTime(entry.PlanningAt),
		formatTime(entry.ImplementingAt),
		formatTime(entry.ReviewingAt),
		formatTime(entry.VerifyingAt),
		formatTime(entry.DoneAt),
		entry.ExecutionState.Phase,
		entry.ExecutionState.ActiveAgentType,
		entry.ExecutionState.ActiveWave,
		entry.Goal,
		entry.Content,
		entry.ClickUpTaskID,
		entry.LinearIssueID,
		entry.LinearIdentifier,
		entry.LinearURL,
		entry.LinearTeamKey,
		entry.LinearProjectID,
		entry.ReviewCycle,
		entry.LatestReviewFeedback,
		entry.PRURL,
		entry.PRReviewDecision,
		entry.PRCheckStatus,
		entry.PRCreateState, entry.PRCreateError, entry.PRCreateAttempts, formatTime(entry.PRCreateAttemptedAt),
		entry.VerifiedSHA, entry.VerifiedBaseSHA, formatTime(entry.VerifiedAt), entry.VerifiedBy, entry.StaleVerificationReason,
		entry.BlockedReason, entry.BlockedSource, formatTime(entry.BlockedAt),
	)
	if err != nil {
		if isUniqueConstraintError(err) {
			return fmt.Errorf("plan already exists: %s/%s", project, entry.Filename)
		}
		return fmt.Errorf("create plan: %w", err)
	}
	return nil
}

// Get retrieves a task entry by project and filename.
// Returns an error if the task is not found.
func (s *SQLiteStore) Get(project, filename string) (TaskEntry, error) {
	const q = `
			SELECT filename, status, description, branch, topic, created_at, implemented, planning_at, implementing_at, reviewing_at, verifying_at, done_at, execution_phase, active_agent_type, active_wave, goal, content, clickup_task_id, linear_issue_id, linear_identifier, linear_url, linear_team_key, linear_project_id, review_cycle, latest_review_feedback, pr_url, pr_review_decision, pr_check_status, pr_create_state, pr_create_error, pr_create_attempts, pr_create_attempted_at, verified_sha, verified_base_sha, verified_at, verified_by, stale_verification_reason, blocked_reason, blocked_source, blocked_at
		FROM tasks
		WHERE project = ? AND filename = ?
	`
	row := s.db.QueryRow(q, project, filename)
	return scanTaskEntry(row)
}

// Update replaces all fields of an existing task entry.
// Returns an error if the task is not found.
func (s *SQLiteStore) Update(project, filename string, entry TaskEntry) error {
	const q = `
		UPDATE tasks
		SET status = ?, description = ?, branch = ?, topic = ?, created_at = ?, implemented = ?, planning_at = ?, implementing_at = ?, reviewing_at = ?, verifying_at = ?, done_at = ?, execution_phase = ?, active_agent_type = ?, active_wave = ?, goal = ?, clickup_task_id = ?, linear_issue_id = ?, linear_identifier = ?, linear_url = ?, linear_team_key = ?, linear_project_id = ?, review_cycle = ?, latest_review_feedback = ?
		WHERE project = ? AND filename = ?
	`
	result, err := s.db.Exec(q,
		string(entry.Status),
		entry.Description,
		entry.Branch,
		entry.Topic,
		formatTime(entry.CreatedAt),
		entry.Implemented,
		formatTime(entry.PlanningAt),
		formatTime(entry.ImplementingAt),
		formatTime(entry.ReviewingAt),
		formatTime(entry.VerifyingAt),
		formatTime(entry.DoneAt),
		entry.ExecutionState.Phase,
		entry.ExecutionState.ActiveAgentType,
		entry.ExecutionState.ActiveWave,
		entry.Goal,
		entry.ClickUpTaskID,
		entry.LinearIssueID,
		entry.LinearIdentifier,
		entry.LinearURL,
		entry.LinearTeamKey,
		entry.LinearProjectID,
		entry.ReviewCycle,
		entry.LatestReviewFeedback,
		project,
		filename,
	)
	if err != nil {
		return fmt.Errorf("update plan: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("update plan rows affected: %w", err)
	}
	if n == 0 {
		return newNotFoundError("plan not found: %s/%s", project, filename)
	}
	return nil
}

// Rename changes the filename of an existing task entry. The parent row in
// tasks plus any derived child rows in subtasks and pr_reviews are updated in
// a single transaction with foreign keys deferred, so renames remain safe for
// tasks that already have ingested content or PR review activity.
// Returns an error if the old filename is not found or the new filename already exists.
func (s *SQLiteStore) Rename(project, oldFilename, newFilename string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin rename transaction: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	if _, err = tx.Exec("PRAGMA defer_foreign_keys = ON"); err != nil {
		return fmt.Errorf("defer foreign keys for rename: %w", err)
	}

	result, err := tx.Exec(
		`UPDATE tasks SET filename = ? WHERE project = ? AND filename = ?`,
		newFilename, project, oldFilename,
	)
	if err != nil {
		if isUniqueConstraintError(err) {
			return fmt.Errorf("plan already exists: %s/%s", project, newFilename)
		}
		return fmt.Errorf("rename plan: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("rename plan rows affected: %w", err)
	}
	if n == 0 {
		return newNotFoundError("plan not found: %s/%s", project, oldFilename)
	}

	if _, err = tx.Exec(
		`UPDATE subtasks SET plan_filename = ? WHERE project = ? AND plan_filename = ?`,
		newFilename, project, oldFilename,
	); err != nil {
		return fmt.Errorf("rename subtasks: %w", err)
	}

	if _, err = tx.Exec(
		`UPDATE pr_reviews SET plan_filename = ? WHERE project = ? AND plan_filename = ?`,
		newFilename, project, oldFilename,
	); err != nil {
		return fmt.Errorf("rename pr_reviews: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit rename: %w", err)
	}
	committed = true
	return nil
}

// Delete removes an existing task entry.
// Returns an error if the task is not found.
func (s *SQLiteStore) Delete(project, filename string) error {
	const q = `DELETE FROM tasks WHERE project = ? AND filename = ?`
	result, err := s.db.Exec(q, project, filename)
	if err != nil {
		return fmt.Errorf("delete plan: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete plan rows affected: %w", err)
	}
	if n == 0 {
		return newNotFoundError("plan not found: %s/%s", project, filename)
	}
	return nil
}

// List returns all task entries for the given project, sorted by filename.
func (s *SQLiteStore) List(project string) ([]TaskEntry, error) {
	const q = `
			SELECT filename, status, description, branch, topic, created_at, implemented, planning_at, implementing_at, reviewing_at, verifying_at, done_at, execution_phase, active_agent_type, active_wave, goal, content, clickup_task_id, linear_issue_id, linear_identifier, linear_url, linear_team_key, linear_project_id, review_cycle, latest_review_feedback, pr_url, pr_review_decision, pr_check_status, pr_create_state, pr_create_error, pr_create_attempts, pr_create_attempted_at, verified_sha, verified_base_sha, verified_at, verified_by, stale_verification_reason, blocked_reason, blocked_source, blocked_at
		FROM tasks
		WHERE project = ?
		ORDER BY filename ASC
	`
	rows, err := s.db.Query(q, project)
	if err != nil {
		return nil, fmt.Errorf("list tasks: %w", err)
	}
	defer rows.Close()
	return scanTaskEntries(rows)
}

// ListByStatus returns all task entries for the given project matching any of
// the provided statuses, sorted by filename.
func (s *SQLiteStore) ListByStatus(project string, statuses ...Status) ([]TaskEntry, error) {
	if len(statuses) == 0 {
		return []TaskEntry{}, nil
	}

	placeholders := make([]string, len(statuses))
	args := make([]any, 0, len(statuses)+1)
	args = append(args, project)
	for i, s := range statuses {
		placeholders[i] = "?"
		args = append(args, string(s))
	}

	q := fmt.Sprintf(`
			SELECT filename, status, description, branch, topic, created_at, implemented, planning_at, implementing_at, reviewing_at, verifying_at, done_at, execution_phase, active_agent_type, active_wave, goal, content, clickup_task_id, linear_issue_id, linear_identifier, linear_url, linear_team_key, linear_project_id, review_cycle, latest_review_feedback, pr_url, pr_review_decision, pr_check_status, pr_create_state, pr_create_error, pr_create_attempts, pr_create_attempted_at, verified_sha, verified_base_sha, verified_at, verified_by, stale_verification_reason, blocked_reason, blocked_source, blocked_at
		FROM tasks
		WHERE project = ? AND status IN (%s)
		ORDER BY filename ASC
	`, strings.Join(placeholders, ", "))

	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, fmt.Errorf("list tasks by status: %w", err)
	}
	defer rows.Close()
	return scanTaskEntries(rows)
}

// ListByTopic returns all task entries for the given project and topic,
// sorted by filename.
func (s *SQLiteStore) ListByTopic(project, topic string) ([]TaskEntry, error) {
	const q = `
			SELECT filename, status, description, branch, topic, created_at, implemented, planning_at, implementing_at, reviewing_at, verifying_at, done_at, execution_phase, active_agent_type, active_wave, goal, content, clickup_task_id, linear_issue_id, linear_identifier, linear_url, linear_team_key, linear_project_id, review_cycle, latest_review_feedback, pr_url, pr_review_decision, pr_check_status, pr_create_state, pr_create_error, pr_create_attempts, pr_create_attempted_at, verified_sha, verified_base_sha, verified_at, verified_by, stale_verification_reason, blocked_reason, blocked_source, blocked_at
		FROM tasks
		WHERE project = ? AND topic = ?
		ORDER BY filename ASC
	`
	rows, err := s.db.Query(q, project, topic)
	if err != nil {
		return nil, fmt.Errorf("list tasks by topic: %w", err)
	}
	defer rows.Close()
	return scanTaskEntries(rows)
}

// ListTopics returns all topic entries for the given project, sorted by name.
func (s *SQLiteStore) ListTopics(project string) ([]TopicEntry, error) {
	const q = `
		SELECT name, created_at
		FROM topics
		WHERE project = ?
		ORDER BY name ASC
	`
	rows, err := s.db.Query(q, project)
	if err != nil {
		return nil, fmt.Errorf("list topics: %w", err)
	}
	defer rows.Close()

	topics := []TopicEntry{}
	for rows.Next() {
		var name, createdAt string
		if err := rows.Scan(&name, &createdAt); err != nil {
			return nil, fmt.Errorf("scan topic: %w", err)
		}
		topics = append(topics, TopicEntry{
			Name:      name,
			CreatedAt: parseTime(createdAt),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate topics: %w", err)
	}
	return topics, nil
}

// CreateTopic inserts a new topic entry for the given project.
// Returns an error if a topic with the same name already exists in the project.
func (s *SQLiteStore) CreateTopic(project string, entry TopicEntry) error {
	const q = `
		INSERT INTO topics (project, name, created_at)
		VALUES (?, ?, ?)
	`
	_, err := s.db.Exec(q, project, entry.Name, formatTime(entry.CreatedAt))
	if err != nil {
		if isUniqueConstraintError(err) {
			return fmt.Errorf("topic already exists: %s/%s", project, entry.Name)
		}
		return fmt.Errorf("create topic: %w", err)
	}
	return nil
}

// GetContent retrieves only the content field for a task entry.
// Returns an error if the task is not found.
func (s *SQLiteStore) GetContent(project, filename string) (string, error) {
	const q = `SELECT content FROM tasks WHERE project = ? AND filename = ?`
	var content string
	err := s.db.QueryRow(q, project, filename).Scan(&content)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", newNotFoundError("plan not found: %s/%s", project, filename)
		}
		return "", fmt.Errorf("get content: %w", err)
	}
	return content, nil
}

// SetContent updates only the content field for an existing task entry.
// Returns an error if the task is not found.
func (s *SQLiteStore) SetContent(project, filename, content string) error {
	const q = `UPDATE tasks SET content = ? WHERE project = ? AND filename = ?`
	result, err := s.db.Exec(q, content, project, filename)
	if err != nil {
		return fmt.Errorf("set content: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("set content rows affected: %w", err)
	}
	if n == 0 {
		return newNotFoundError("plan not found: %s/%s", project, filename)
	}
	return nil
}

// SetClickUpTaskID sets the ClickUp task ID for an existing task entry.
// Returns an error if the task is not found.
func (s *SQLiteStore) SetClickUpTaskID(project, filename, taskID string) error {
	const q = `UPDATE tasks SET clickup_task_id = ? WHERE project = ? AND filename = ?`
	result, err := s.db.Exec(q, taskID, project, filename)
	if err != nil {
		return fmt.Errorf("set clickup_task_id: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("set clickup_task_id rows affected: %w", err)
	}
	if n == 0 {
		return newNotFoundError("plan not found: %s/%s", project, filename)
	}
	return nil
}

// SetLinearLink stores Linear issue coordinates for an existing task entry.
// Returns an error if the task is not found.
func (s *SQLiteStore) SetLinearLink(project, filename string, link LinearLink) error {
	const q = `
		UPDATE tasks
		SET linear_issue_id = ?, linear_identifier = ?, linear_url = ?, linear_team_key = ?, linear_project_id = ?
		WHERE project = ? AND filename = ?
	`
	link = normalizedLinearLink(link)
	result, err := s.db.Exec(q, link.LinearIssueID, link.LinearIdentifier, link.LinearURL, link.LinearTeamKey, link.LinearProjectID, project, filename)
	if err != nil {
		return fmt.Errorf("set linear link: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("set linear link rows affected: %w", err)
	}
	if n == 0 {
		return newNotFoundError("plan not found: %s/%s", project, filename)
	}
	return nil
}

// SetLinearLinkIfNoActiveDuplicate stores Linear issue coordinates unless
// another task in one of the supplied statuses already links the same issue.
// The duplicate check and write run in a single transaction; the returned
// string is the conflicting filename when the write is skipped.
func (s *SQLiteStore) SetLinearLinkIfNoActiveDuplicate(project, filename string, link LinearLink, statuses ...Status) (string, error) {
	link = normalizedLinearLink(link)

	tx, err := s.db.Begin()
	if err != nil {
		return "", fmt.Errorf("begin set linear link transaction: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	var existing string
	if err := tx.QueryRow(`SELECT filename FROM tasks WHERE project = ? AND filename = ?`, project, filename).Scan(&existing); err != nil {
		if err == sql.ErrNoRows {
			return "", newNotFoundError("plan not found: %s/%s", project, filename)
		}
		return "", fmt.Errorf("check linear link target: %w", err)
	}

	if link.LinearIssueID != "" {
		conflict, err := findLinkedTaskTx(tx, project, link.LinearIssueID, filename, statuses...)
		if err != nil {
			return "", err
		}
		if conflict != "" {
			return conflict, nil
		}
	}

	const q = `
		UPDATE tasks
		SET linear_issue_id = ?, linear_identifier = ?, linear_url = ?, linear_team_key = ?, linear_project_id = ?
		WHERE project = ? AND filename = ?
	`
	result, err := tx.Exec(q, link.LinearIssueID, link.LinearIdentifier, link.LinearURL, link.LinearTeamKey, link.LinearProjectID, project, filename)
	if err != nil {
		return "", fmt.Errorf("set linear link: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return "", fmt.Errorf("set linear link rows affected: %w", err)
	}
	if n == 0 {
		return "", newNotFoundError("plan not found: %s/%s", project, filename)
	}
	if err := tx.Commit(); err != nil {
		return "", fmt.Errorf("commit set linear link: %w", err)
	}
	committed = true
	return "", nil
}

// ClearLinearLink clears Linear issue coordinates for an existing task entry.
// Returns an error if the task is not found.
func (s *SQLiteStore) ClearLinearLink(project, filename string) error {
	const q = `
		UPDATE tasks
		SET linear_issue_id = '', linear_identifier = '', linear_url = '', linear_team_key = '', linear_project_id = ''
		WHERE project = ? AND filename = ?
	`
	result, err := s.db.Exec(q, project, filename)
	if err != nil {
		return fmt.Errorf("clear linear link: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("clear linear link rows affected: %w", err)
	}
	if n == 0 {
		return newNotFoundError("plan not found: %s/%s", project, filename)
	}
	return nil
}

// FindLinkedTask returns the filename linked to a Linear issue in a project.
func (s *SQLiteStore) FindLinkedTask(project, issueID string, statuses ...Status) (string, error) {
	filename, err := findLinkedTaskQuery(s.db, project, issueID, "", statuses...)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", newNotFoundError("linear link not found: %s/%s", project, issueID)
		}
		return "", fmt.Errorf("find linked task: %w", err)
	}
	return filename, nil
}

func findLinkedTaskTx(tx *sql.Tx, project, issueID, excludeFilename string, statuses ...Status) (string, error) {
	filename, err := findLinkedTaskQuery(tx, project, issueID, excludeFilename, statuses...)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", nil
		}
		return "", fmt.Errorf("find linked task: %w", err)
	}
	return filename, nil
}

type linkedTaskQuerier interface {
	QueryRow(query string, args ...any) *sql.Row
}

func findLinkedTaskQuery(qr linkedTaskQuerier, project, issueID, excludeFilename string, statuses ...Status) (string, error) {
	if strings.TrimSpace(issueID) == "" {
		return "", sql.ErrNoRows
	}
	args := []any{project, issueID}
	excludeClause := ""
	if excludeFilename != "" {
		excludeClause = " AND filename <> ?"
		args = append(args, excludeFilename)
	}
	statusClause := ""
	if len(statuses) > 0 {
		placeholders := make([]string, len(statuses))
		for i, status := range statuses {
			placeholders[i] = "?"
			args = append(args, string(status))
		}
		statusClause = fmt.Sprintf(" AND status IN (%s)", strings.Join(placeholders, ", "))
	}
	q := fmt.Sprintf(`
		SELECT filename
		FROM tasks INDEXED BY idx_tasks_linear_issue_id
		WHERE project = ? AND linear_issue_id = ?%s%s
		ORDER BY filename ASC
		LIMIT 1
	`, excludeClause, statusClause)
	var filename string
	if err := qr.QueryRow(q, args...).Scan(&filename); err != nil {
		return "", err
	}
	return filename, nil
}

// SetExecutionState persists fine-grained execution lifecycle metadata.
func (s *SQLiteStore) SetExecutionState(project, filename string, state ExecutionState) error {
	const q = `UPDATE tasks SET execution_phase = ?, active_agent_type = ?, active_wave = ? WHERE project = ? AND filename = ?`
	result, err := s.db.Exec(q, state.Phase, state.ActiveAgentType, state.ActiveWave, project, filename)
	if err != nil {
		return fmt.Errorf("set execution state: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("set execution state rows affected: %w", err)
	}
	if n == 0 {
		return newNotFoundError("plan not found: %s/%s", project, filename)
	}
	return nil
}

// IncrementReviewCycle atomically increments the review_cycle counter for an existing task entry.
// Returns an error if the task is not found.
func (s *SQLiteStore) IncrementReviewCycle(project, filename string) error {
	const q = `UPDATE tasks SET review_cycle = review_cycle + 1 WHERE project = ? AND filename = ?`
	result, err := s.db.Exec(q, project, filename)
	if err != nil {
		return fmt.Errorf("increment review_cycle: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("increment review_cycle rows affected: %w", err)
	}
	if n == 0 {
		return newNotFoundError("plan not found: %s/%s", project, filename)
	}
	return nil
}

// SetSubtasks replaces all subtasks for a plan in a transaction.
// Existing subtasks are removed before inserting the supplied rows.
func (s *SQLiteStore) SetSubtasks(project, filename string, subtasks []SubtaskEntry) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin subtasks transaction: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	if _, err = tx.Exec("DELETE FROM subtasks WHERE project = ? AND plan_filename = ?", project, filename); err != nil {
		return fmt.Errorf("delete subtasks: %w", err)
	}

	for _, st := range subtasks {
		if _, err = tx.Exec(
			"INSERT INTO subtasks (project, plan_filename, task_number, title, status) VALUES (?, ?, ?, ?, ?)",
			project, filename, st.TaskNumber, st.Title, string(st.Status),
		); err != nil {
			return fmt.Errorf("insert subtask: %w", err)
		}
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit subtasks: %w", err)
	}
	return nil
}

// GetSubtasks returns all subtasks for a plan, sorted by task_number.
func (s *SQLiteStore) GetSubtasks(project, filename string) ([]SubtaskEntry, error) {
	rows, err := s.db.Query(
		`SELECT task_number, title, status FROM subtasks WHERE project = ? AND plan_filename = ? ORDER BY task_number ASC`,
		project,
		filename,
	)
	if err != nil {
		return nil, fmt.Errorf("list subtasks: %w", err)
	}
	defer rows.Close()

	subtasks := []SubtaskEntry{}
	for rows.Next() {
		var taskNumber int
		var title, status string
		if err := rows.Scan(&taskNumber, &title, &status); err != nil {
			return nil, fmt.Errorf("scan subtask: %w", err)
		}
		subtasks = append(subtasks, SubtaskEntry{
			TaskNumber: taskNumber,
			Title:      title,
			Status:     SubtaskStatus(status),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate subtasks: %w", err)
	}
	return subtasks, nil
}

// UpdateSubtaskStatus updates the status of a specific subtask.
func (s *SQLiteStore) UpdateSubtaskStatus(project, filename string, taskNumber int, status SubtaskStatus) error {
	const q = `
		UPDATE subtasks
		SET status = ?
		WHERE project = ? AND plan_filename = ? AND task_number = ?
	`
	result, err := s.db.Exec(q, string(status), project, filename, taskNumber)
	if err != nil {
		return fmt.Errorf("update subtask status: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("update subtask status rows affected: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("subtask not found: %s/%s#%d", project, filename, taskNumber)
	}
	return nil
}

// SetPhaseTimestamp sets the timestamp for the requested lifecycle phase.
// Known phases are: planning, implementing, reviewing, done.
func (s *SQLiteStore) SetPhaseTimestamp(project, filename, phase string, ts time.Time) error {
	var column string
	switch phase {
	case "planning":
		column = "planning_at"
	case "implementing":
		column = "implementing_at"
	case "reviewing":
		column = "reviewing_at"
	case "verifying":
		column = "verifying_at"
	case "done":
		column = "done_at"
	default:
		return fmt.Errorf("unknown phase: %s", phase)
	}

	query := fmt.Sprintf("UPDATE tasks SET %s = ? WHERE project = ? AND filename = ?", column)
	result, err := s.db.Exec(query, formatTime(ts), project, filename)
	if err != nil {
		return fmt.Errorf("set phase timestamp: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("set phase timestamp rows affected: %w", err)
	}
	if n == 0 {
		return newNotFoundError("plan not found: %s/%s", project, filename)
	}
	return nil
}

// SetPlanGoal sets the goal text for a plan.
func (s *SQLiteStore) SetPlanGoal(project, filename, goal string) error {
	const q = `UPDATE tasks SET goal = ? WHERE project = ? AND filename = ?`
	result, err := s.db.Exec(q, goal, project, filename)
	if err != nil {
		return fmt.Errorf("set plan goal: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("set plan goal rows affected: %w", err)
	}
	if n == 0 {
		return newNotFoundError("plan not found: %s/%s", project, filename)
	}
	return nil
}

// SetPRURL sets the pull request URL for an existing task entry.
// Returns an error if the task is not found.
func (s *SQLiteStore) SetPRURL(project, filename, url string) error {
	const q = `UPDATE tasks SET pr_url = ? WHERE project = ? AND filename = ?`
	result, err := s.db.Exec(q, url, project, filename)
	if err != nil {
		return fmt.Errorf("set pr_url: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("set pr_url rows affected: %w", err)
	}
	if n == 0 {
		return newNotFoundError("plan not found: %s/%s", project, filename)
	}
	return nil
}

func (s *SQLiteStore) SetPRCreateOutcome(project, filename string, outcome PRCreateOutcome) error {
	result, err := s.db.Exec(`UPDATE tasks SET pr_create_state=?, pr_create_error=?, pr_create_attempts=?, pr_create_attempted_at=? WHERE project=? AND filename=?`, outcome.State, outcome.Error, outcome.Attempts, formatTime(outcome.AttemptedAt), project, filename)
	if err != nil {
		return fmt.Errorf("set pr create outcome: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("set pr create outcome rows affected: %w", err)
	}
	if n == 0 {
		return newNotFoundError("plan not found: %s/%s", project, filename)
	}
	return nil
}

func (s *SQLiteStore) ClearPRCreateOutcome(project, filename string) error {
	return s.SetPRCreateOutcome(project, filename, PRCreateOutcome{})
}

// SetPRState sets the review decision and check status for an existing task entry.
// Returns an error if the task is not found.
func (s *SQLiteStore) SetPRState(project, filename, reviewDecision, checkStatus string) error {
	const q = `UPDATE tasks SET pr_review_decision = ?, pr_check_status = ? WHERE project = ? AND filename = ?`
	result, err := s.db.Exec(q, reviewDecision, checkStatus, project, filename)
	if err != nil {
		return fmt.Errorf("set pr_state: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("set pr_state rows affected: %w", err)
	}
	if n == 0 {
		return newNotFoundError("plan not found: %s/%s", project, filename)
	}
	return nil
}

func (s *SQLiteStore) SetVerification(project, filename string, v VerificationRecord) error {
	const q = `UPDATE tasks SET verified_sha = ?, verified_base_sha = ?, verified_at = ?, verified_by = ?, stale_verification_reason = '' WHERE project = ? AND filename = ?`
	result, err := s.db.Exec(q, v.SHA, v.BaseSHA, formatTime(v.At), v.By, project, filename)
	if err != nil {
		return fmt.Errorf("set verification: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("set verification rows affected: %w", err)
	}
	if n == 0 {
		return newNotFoundError("plan not found: %s/%s", project, filename)
	}
	return nil
}

func (s *SQLiteStore) ClearVerification(project, filename, reason string) error {
	const q = `UPDATE tasks SET verified_sha = '', verified_base_sha = '', verified_at = '', verified_by = '', stale_verification_reason = ? WHERE project = ? AND filename = ?`
	result, err := s.db.Exec(q, reason, project, filename)
	if err != nil {
		return fmt.Errorf("clear verification: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("clear verification rows affected: %w", err)
	}
	if n == 0 {
		return newNotFoundError("plan not found: %s/%s", project, filename)
	}
	return nil
}

// SetBlocked marks a task as waiting on a human decision. A blocked task keeps
// its lifecycle status — blocking is orthogonal to the FSM — but no orchestrator
// may spawn an agent for it until the block clears. reason must be non-empty;
// use ClearBlocked to unblock.
func (s *SQLiteStore) SetBlocked(project, filename, reason, source string) error {
	if strings.TrimSpace(reason) == "" {
		return fmt.Errorf("set blocked: reason must not be empty")
	}
	const q = `UPDATE tasks SET blocked_reason = ?, blocked_source = ?, blocked_at = ? WHERE project = ? AND filename = ?`
	result, err := s.db.Exec(q, reason, source, formatTime(time.Now().UTC()), project, filename)
	if err != nil {
		return fmt.Errorf("set blocked: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("set blocked rows affected: %w", err)
	}
	if n == 0 {
		return newNotFoundError("plan not found: %s/%s", project, filename)
	}
	return nil
}

// ClearBlocked removes a decision block. It is idempotent: clearing an
// unblocked task is a no-op that still reports success.
func (s *SQLiteStore) ClearBlocked(project, filename string) error {
	const q = `UPDATE tasks SET blocked_reason = '', blocked_source = '', blocked_at = '' WHERE project = ? AND filename = ?`
	result, err := s.db.Exec(q, project, filename)
	if err != nil {
		return fmt.Errorf("clear blocked: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("clear blocked rows affected: %w", err)
	}
	if n == 0 {
		return newNotFoundError("plan not found: %s/%s", project, filename)
	}
	return nil
}

// RecordPRReview inserts a new PR review record. INSERT OR IGNORE ensures
// repeated polls for the same review ID are idempotent — only the first record wins.
func (s *SQLiteStore) RecordPRReview(project, filename string, reviewID int, state, body, reviewer string) error {
	const q = `
		INSERT OR IGNORE INTO pr_reviews (project, plan_filename, review_id, review_state, review_body, reviewer_login, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`
	_, err := s.db.Exec(q, project, filename, reviewID, state, body, reviewer, formatTime(time.Now().UTC()))
	if err != nil {
		return fmt.Errorf("record pr review: %w", err)
	}
	return nil
}

// IsReviewProcessed returns true if a review record exists for the given reviewID.
// Returns false on any error or if the row is not found.
func (s *SQLiteStore) IsReviewProcessed(project, filename string, reviewID int) bool {
	const q = `SELECT COUNT(*) FROM pr_reviews WHERE project = ? AND plan_filename = ? AND review_id = ?`
	var count int
	err := s.db.QueryRow(q, project, filename, reviewID).Scan(&count)
	if err != nil {
		return false
	}
	return count > 0
}

// MarkReviewReacted sets reaction_posted = 1 for the given review.
// Returns an error if the review row is not found.
func (s *SQLiteStore) MarkReviewReacted(project, filename string, reviewID int) error {
	const q = `UPDATE pr_reviews SET reaction_posted = 1 WHERE project = ? AND plan_filename = ? AND review_id = ?`
	result, err := s.db.Exec(q, project, filename, reviewID)
	if err != nil {
		return fmt.Errorf("mark review reacted: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("mark review reacted rows affected: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("pr review not found: %s/%s#%d", project, filename, reviewID)
	}
	return nil
}

// MarkReviewFixerDispatched sets fixer_dispatched = 1 for the given review.
// Returns an error if the review row is not found.
func (s *SQLiteStore) MarkReviewFixerDispatched(project, filename string, reviewID int) error {
	const q = `UPDATE pr_reviews SET fixer_dispatched = 1 WHERE project = ? AND plan_filename = ? AND review_id = ?`
	result, err := s.db.Exec(q, project, filename, reviewID)
	if err != nil {
		return fmt.Errorf("mark review fixer dispatched: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("mark review fixer dispatched rows affected: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("pr review not found: %s/%s#%d", project, filename, reviewID)
	}
	return nil
}

// ListPendingReviews returns all review entries where fixer_dispatched = 0,
// ordered by review_id ascending. Returns an empty (non-nil) slice when there are no rows.
func (s *SQLiteStore) ListPendingReviews(project, filename string) ([]PRReviewEntry, error) {
	const q = `
		SELECT review_id, review_state, review_body, reviewer_login, reaction_posted, fixer_dispatched, created_at
		FROM pr_reviews
		WHERE project = ? AND plan_filename = ? AND fixer_dispatched = 0
		ORDER BY review_id ASC
	`
	rows, err := s.db.Query(q, project, filename)
	if err != nil {
		return nil, fmt.Errorf("list pending pr reviews: %w", err)
	}
	defer rows.Close()

	entries := []PRReviewEntry{} // non-nil empty slice
	for rows.Next() {
		var e PRReviewEntry
		var reactionPosted, fixerDispatched int
		var createdAt string
		if err := rows.Scan(&e.ReviewID, &e.ReviewState, &e.ReviewBody, &e.ReviewerLogin, &reactionPosted, &fixerDispatched, &createdAt); err != nil {
			return nil, fmt.Errorf("list pending pr reviews: %w", err)
		}
		e.ReactionPosted = reactionPosted != 0
		e.FixerDispatched = fixerDispatched != 0
		e.CreatedAt = parseTime(createdAt)
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list pending pr reviews: %w", err)
	}
	return entries, nil
}

// EnqueueLinearTrigger records an inbound Linear trigger if it has not already been seen.
func (s *SQLiteStore) EnqueueLinearTrigger(project string, e LinearTriggerEntry) (int64, bool, error) {
	const q = `
		INSERT INTO linear_triggers
			(project, linear_issue_id, linear_identifier, command_kind, source_kind, source_id,
			 actor_id, actor_email, task_arg, detected_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (project, linear_issue_id, command_kind, source_id) DO NOTHING
		RETURNING id
	`
	var id int64
	err := s.db.QueryRow(q, project, e.LinearIssueID, e.LinearIdentifier, e.CommandKind, e.SourceKind, e.SourceID, e.ActorID, e.ActorEmail, e.TaskArg, formatTime(e.DetectedAt)).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, fmt.Errorf("enqueue linear trigger: %w", err)
	}
	return id, true, nil
}

// MarkLinearTriggerDispatched marks an enqueued Linear trigger as dispatched.
func (s *SQLiteStore) MarkLinearTriggerDispatched(project string, id int64, targetFilename string) error {
	return s.markLinearTriggerProcessed(project, id, "dispatched", "", targetFilename)
}

// MarkLinearTriggerRejected marks an enqueued Linear trigger as rejected.
func (s *SQLiteStore) MarkLinearTriggerRejected(project string, id int64, reason string) error {
	return s.markLinearTriggerProcessed(project, id, "rejected", reason, "")
}

// MarkLinearTriggerIgnored marks an enqueued Linear trigger as ignored.
func (s *SQLiteStore) MarkLinearTriggerIgnored(project string, id int64, reason string) error {
	return s.markLinearTriggerProcessed(project, id, "ignored", reason, "")
}

// MarkLinearTriggerFailed marks an enqueued Linear trigger as failed.
func (s *SQLiteStore) MarkLinearTriggerFailed(project string, id int64, reason string) error {
	return s.markLinearTriggerProcessed(project, id, "failed", reason, "")
}

func (s *SQLiteStore) markLinearTriggerProcessed(project string, id int64, outcome, reason, targetFilename string) error {
	const q = `
		UPDATE linear_triggers
		SET processed = 1, processed_at = ?, outcome = ?, rejection_reason = ?, target_filename = ?
		WHERE project = ? AND id = ?
	`
	result, err := s.db.Exec(q, formatTime(time.Now().UTC()), outcome, reason, targetFilename, project, id)
	if err != nil {
		return fmt.Errorf("mark linear trigger %s: %w", outcome, err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("mark linear trigger %s rows affected: %w", outcome, err)
	}
	if n == 0 {
		return newNotFoundError("linear trigger not found: %s/%d", project, id)
	}
	return nil
}

// MarkLinearTriggerAck records whether the source was acknowledged.
func (s *SQLiteStore) MarkLinearTriggerAck(project string, id int64, ackState string) error {
	const q = `UPDATE linear_triggers SET ack_state = ? WHERE project = ? AND id = ?`
	result, err := s.db.Exec(q, ackState, project, id)
	if err != nil {
		return fmt.Errorf("mark linear trigger ack: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("mark linear trigger ack rows affected: %w", err)
	}
	if n == 0 {
		return newNotFoundError("linear trigger not found: %s/%d", project, id)
	}
	return nil
}

// ListUnprocessedLinearTriggers returns queued Linear triggers in detection order.
func (s *SQLiteStore) ListUnprocessedLinearTriggers(project string, limit int) ([]LinearTriggerEntry, error) {
	if limit <= 0 {
		limit = 100
	}
	const q = `
		SELECT id, linear_issue_id, linear_identifier, command_kind, source_kind, source_id,
		       actor_id, actor_email, task_arg, detected_at, processed, processed_at,
		       outcome, rejection_reason, target_filename, ack_state
		FROM linear_triggers
		WHERE project = ? AND processed = 0
		ORDER BY detected_at ASC, id ASC
		LIMIT ?
	`
	rows, err := s.db.Query(q, project, limit)
	if err != nil {
		return nil, fmt.Errorf("list unprocessed linear triggers: %w", err)
	}
	defer rows.Close()

	entries := []LinearTriggerEntry{}
	for rows.Next() {
		entry, err := scanLinearTriggerEntry(rows)
		if err != nil {
			return nil, fmt.Errorf("list unprocessed linear triggers: %w", err)
		}
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list unprocessed linear triggers: %w", err)
	}
	return entries, nil
}

// LinearTriggerStats returns recent Linear trigger outcome counts for a project.
func (s *SQLiteStore) LinearTriggerStats(project string, since time.Time) (LinearTriggerStats, error) {
	const q = `
		SELECT detected_at, processed_at, outcome
		FROM linear_triggers
		WHERE project = ? AND detected_at >= ?
	`
	rows, err := s.db.Query(q, project, formatTime(since))
	if err != nil {
		return LinearTriggerStats{}, fmt.Errorf("linear trigger stats: %w", err)
	}
	defer rows.Close()

	var stats LinearTriggerStats
	for rows.Next() {
		var detectedAt, processedAt, outcome string
		if err := rows.Scan(&detectedAt, &processedAt, &outcome); err != nil {
			return LinearTriggerStats{}, fmt.Errorf("linear trigger stats: %w", err)
		}
		for _, at := range []time.Time{parseTime(detectedAt), parseTime(processedAt)} {
			if at.After(stats.LastSeenAt) {
				stats.LastSeenAt = at
			}
		}
		switch outcome {
		case "dispatched":
			stats.Dispatched++
		case "rejected":
			stats.Rejected++
		case "failed":
			stats.Failed++
		}
	}
	if err := rows.Err(); err != nil {
		return LinearTriggerStats{}, fmt.Errorf("linear trigger stats: %w", err)
	}
	return stats, nil
}

// RecordLinearWebhookDelivery records a Linear webhook delivery if it has not already been seen.
func (s *SQLiteStore) RecordLinearWebhookDelivery(project string, d LinearWebhookDelivery) (bool, error) {
	const q = `
		INSERT INTO linear_webhook_deliveries
			(project, delivery_id, linear_event, action, linear_issue_id, source_kind,
			 source_id, status, reason, received_at, processed_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (project, delivery_id) DO NOTHING
	`
	result, err := s.db.Exec(q, project, d.DeliveryID, d.LinearEvent, d.Action, d.LinearIssueID, d.SourceKind, d.SourceID, d.Status, d.Reason, formatTime(d.ReceivedAt), formatTime(d.ProcessedAt))
	if err != nil {
		return false, fmt.Errorf("record linear webhook delivery: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("record linear webhook delivery rows affected: %w", err)
	}
	return n > 0, nil
}

// UpdateLinearWebhookDelivery updates the status for a recorded Linear webhook delivery.
func (s *SQLiteStore) UpdateLinearWebhookDelivery(project, deliveryID, status, reason string) error {
	const q = `
		UPDATE linear_webhook_deliveries
		SET status = ?, reason = ?, processed_at = ?
		WHERE project = ? AND delivery_id = ?
	`
	result, err := s.db.Exec(q, status, reason, formatTime(time.Now().UTC()), project, deliveryID)
	if err != nil {
		return fmt.Errorf("update linear webhook delivery: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("update linear webhook delivery rows affected: %w", err)
	}
	if n == 0 {
		return newNotFoundError("linear webhook delivery not found: %s/%s", project, deliveryID)
	}
	return nil
}

// LinearWebhookDeliveryByID returns one recorded Linear webhook delivery.
func (s *SQLiteStore) LinearWebhookDeliveryByID(project, deliveryID string) (LinearWebhookDelivery, error) {
	const q = `
		SELECT id, delivery_id, linear_event, action, linear_issue_id, source_kind,
		       source_id, status, reason, received_at, processed_at
		FROM linear_webhook_deliveries
		WHERE project = ? AND delivery_id = ?
	`
	delivery, err := scanLinearWebhookDelivery(s.db.QueryRow(q, project, deliveryID))
	if err == sql.ErrNoRows {
		return LinearWebhookDelivery{}, newNotFoundError("linear webhook delivery not found: %s/%s", project, deliveryID)
	}
	if err != nil {
		return LinearWebhookDelivery{}, fmt.Errorf("linear webhook delivery by ID: %w", err)
	}
	return delivery, nil
}

// ListRecentLinearWebhookDeliveries returns recent Linear webhook deliveries newest-first.
func (s *SQLiteStore) ListRecentLinearWebhookDeliveries(project string, limit int) ([]LinearWebhookDelivery, error) {
	if limit <= 0 {
		limit = 100
	}
	const q = `
		SELECT id, delivery_id, linear_event, action, linear_issue_id, source_kind,
		       source_id, status, reason, received_at, processed_at
		FROM linear_webhook_deliveries
		WHERE project = ?
		ORDER BY received_at DESC, id DESC
		LIMIT ?
	`
	rows, err := s.db.Query(q, project, limit)
	if err != nil {
		return nil, fmt.Errorf("list recent linear webhook deliveries: %w", err)
	}
	defer rows.Close()

	deliveries := []LinearWebhookDelivery{}
	for rows.Next() {
		delivery, err := scanLinearWebhookDelivery(rows)
		if err != nil {
			return nil, fmt.Errorf("list recent linear webhook deliveries: %w", err)
		}
		deliveries = append(deliveries, delivery)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list recent linear webhook deliveries: %w", err)
	}
	return deliveries, nil
}

// LinearWebhookStats returns recent Linear webhook delivery status counts for a project.
func (s *SQLiteStore) LinearWebhookStats(project string, since time.Time) (LinearWebhookStats, error) {
	const q = `
		SELECT received_at, processed_at, status
		FROM linear_webhook_deliveries
		WHERE project = ? AND received_at >= ?
	`
	rows, err := s.db.Query(q, project, formatTime(since))
	if err != nil {
		return LinearWebhookStats{}, fmt.Errorf("linear webhook stats: %w", err)
	}
	defer rows.Close()

	var stats LinearWebhookStats
	for rows.Next() {
		var receivedAt, processedAt, status string
		if err := rows.Scan(&receivedAt, &processedAt, &status); err != nil {
			return LinearWebhookStats{}, fmt.Errorf("linear webhook stats: %w", err)
		}
		for _, at := range []time.Time{parseTime(receivedAt), parseTime(processedAt)} {
			if at.After(stats.LastDeliveryAt) {
				stats.LastDeliveryAt = at
			}
		}
		switch status {
		case "accepted":
			stats.Accepted++
		case "duplicate":
			stats.Duplicate++
		case "ignored":
			stats.Ignored++
		case "rejected":
			stats.Rejected++
		case "failed":
			stats.Failed++
		}
	}
	if err := rows.Err(); err != nil {
		return LinearWebhookStats{}, fmt.Errorf("linear webhook stats: %w", err)
	}
	return stats, nil
}

// LastSeenCommentAt returns the last Linear comment cursor for an issue.
func (s *SQLiteStore) LastSeenCommentAt(project, linearIssueID string) (time.Time, error) {
	const q = `SELECT last_seen_at FROM linear_comment_cursor WHERE project = ? AND linear_issue_id = ?`
	var lastSeenAt string
	err := s.db.QueryRow(q, project, linearIssueID).Scan(&lastSeenAt)
	if err == sql.ErrNoRows {
		return time.Time{}, nil
	}
	if err != nil {
		return time.Time{}, fmt.Errorf("last seen linear comment cursor: %w", err)
	}
	return parseTime(lastSeenAt), nil
}

// SetLastSeenCommentAt updates the Linear comment cursor monotonically.
func (s *SQLiteStore) SetLastSeenCommentAt(project, linearIssueID string, at time.Time) error {
	const q = `
		INSERT INTO linear_comment_cursor (project, linear_issue_id, last_seen_at)
		VALUES (?, ?, ?)
		ON CONFLICT (project, linear_issue_id) DO UPDATE SET
			last_seen_at = excluded.last_seen_at
		WHERE excluded.last_seen_at > linear_comment_cursor.last_seen_at
	`
	if _, err := s.db.Exec(q, project, linearIssueID, formatTime(at)); err != nil {
		return fmt.Errorf("set last seen linear comment cursor: %w", err)
	}
	return nil
}

func scanLinearTriggerEntry(scanner interface {
	Scan(dest ...any) error
}) (LinearTriggerEntry, error) {
	var e LinearTriggerEntry
	var detectedAt, processedAt string
	var processed int
	err := scanner.Scan(
		&e.ID,
		&e.LinearIssueID,
		&e.LinearIdentifier,
		&e.CommandKind,
		&e.SourceKind,
		&e.SourceID,
		&e.ActorID,
		&e.ActorEmail,
		&e.TaskArg,
		&detectedAt,
		&processed,
		&processedAt,
		&e.Outcome,
		&e.RejectionReason,
		&e.TargetFilename,
		&e.AckState,
	)
	if err != nil {
		return LinearTriggerEntry{}, err
	}
	e.DetectedAt = parseTime(detectedAt)
	e.Processed = processed != 0
	e.ProcessedAt = parseTime(processedAt)
	return e, nil
}

func scanLinearWebhookDelivery(scanner interface {
	Scan(dest ...any) error
}) (LinearWebhookDelivery, error) {
	var d LinearWebhookDelivery
	var receivedAt, processedAt string
	err := scanner.Scan(
		&d.ID,
		&d.DeliveryID,
		&d.LinearEvent,
		&d.Action,
		&d.LinearIssueID,
		&d.SourceKind,
		&d.SourceID,
		&d.Status,
		&d.Reason,
		&receivedAt,
		&processedAt,
	)
	if err != nil {
		return LinearWebhookDelivery{}, err
	}
	d.ReceivedAt = parseTime(receivedAt)
	d.ProcessedAt = parseTime(processedAt)
	return d, nil
}

// scanTaskEntry scans a single row into a TaskEntry.
func scanTaskEntry(row *sql.Row) (TaskEntry, error) {
	var filename, status, description, branch, topic, createdAt, implemented, planningAt, implementingAt, reviewingAt, verifyingAt, doneAt, executionPhase, activeAgentType, goal, content, clickupTaskID, latestReviewFeedback string
	var linearIssueID, linearIdentifier, linearURL, linearTeamKey, linearProjectID string
	var activeWave, reviewCycle int
	var prURL, prReviewDecision, prCheckStatus string
	var prCreateState, prCreateError, prCreateAttemptedAt string
	var prCreateAttempts int
	var verifiedSHA, verifiedBaseSHA, verifiedAt, verifiedBy, staleVerificationReason string
	var blockedReason, blockedSource, blockedAt string
	if err := row.Scan(
		&filename,
		&status,
		&description,
		&branch,
		&topic,
		&createdAt,
		&implemented,
		&planningAt,
		&implementingAt,
		&reviewingAt,
		&verifyingAt,
		&doneAt,
		&executionPhase,
		&activeAgentType,
		&activeWave,
		&goal,
		&content,
		&clickupTaskID,
		&linearIssueID,
		&linearIdentifier,
		&linearURL,
		&linearTeamKey,
		&linearProjectID,
		&reviewCycle,
		&latestReviewFeedback,
		&prURL,
		&prReviewDecision,
		&prCheckStatus,
		&prCreateState, &prCreateError, &prCreateAttempts, &prCreateAttemptedAt,
		&verifiedSHA, &verifiedBaseSHA, &verifiedAt, &verifiedBy, &staleVerificationReason,
			&blockedReason, &blockedSource, &blockedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return TaskEntry{}, newNotFoundError("plan not found")
		}
		return TaskEntry{}, fmt.Errorf("scan plan: %w", err)
	}
	return TaskEntry{
		Filename:    filename,
		Status:      Status(status),
		Description: description,
		Branch:      branch,
		Topic:       topic,
		CreatedAt:   parseTime(createdAt),
		Implemented: implemented,
		ExecutionState: ExecutionState{
			Phase:           executionPhase,
			ActiveAgentType: activeAgentType,
			ActiveWave:      activeWave,
		},
		PlanningAt:           parseTime(planningAt),
		ImplementingAt:       parseTime(implementingAt),
		ReviewingAt:          parseTime(reviewingAt),
		VerifyingAt:          parseTime(verifyingAt),
		DoneAt:               parseTime(doneAt),
		Goal:                 goal,
		Content:              content,
		ClickUpTaskID:        clickupTaskID,
		LinearIssueID:        linearIssueID,
		LinearIdentifier:     linearIdentifier,
		LinearURL:            linearURL,
		LinearTeamKey:        linearTeamKey,
		LinearProjectID:      linearProjectID,
		ReviewCycle:          reviewCycle,
		LatestReviewFeedback: latestReviewFeedback,
		PRURL:                prURL,
		PRReviewDecision:     prReviewDecision,
		PRCheckStatus:        prCheckStatus,
		PRCreateState:        prCreateState, PRCreateError: prCreateError, PRCreateAttempts: prCreateAttempts, PRCreateAttemptedAt: parseTime(prCreateAttemptedAt),
		VerifiedSHA: verifiedSHA, VerifiedBaseSHA: verifiedBaseSHA, VerifiedAt: parseTime(verifiedAt), VerifiedBy: verifiedBy, StaleVerificationReason: staleVerificationReason,
			BlockedReason: blockedReason, BlockedSource: blockedSource, BlockedAt: parseTime(blockedAt),
	}, nil
}

// scanTaskEntries scans multiple rows into a slice of TaskEntry.
func scanTaskEntries(rows *sql.Rows) ([]TaskEntry, error) {
	entries := []TaskEntry{}
	for rows.Next() {
		var filename, status, description, branch, topic, createdAt, implemented, planningAt, implementingAt, reviewingAt, verifyingAt, doneAt, executionPhase, activeAgentType, goal, content, clickupTaskID, latestReviewFeedback string
		var linearIssueID, linearIdentifier, linearURL, linearTeamKey, linearProjectID string
		var activeWave, reviewCycle int
		var prURL, prReviewDecision, prCheckStatus string
		var prCreateState, prCreateError, prCreateAttemptedAt string
		var prCreateAttempts int
		var verifiedSHA, verifiedBaseSHA, verifiedAt, verifiedBy, staleVerificationReason string
		var blockedReason, blockedSource, blockedAt string
		if err := rows.Scan(
			&filename,
			&status,
			&description,
			&branch,
			&topic,
			&createdAt,
			&implemented,
			&planningAt,
			&implementingAt,
			&reviewingAt,
			&verifyingAt,
			&doneAt,
			&executionPhase,
			&activeAgentType,
			&activeWave,
			&goal,
			&content,
			&clickupTaskID,
			&linearIssueID,
			&linearIdentifier,
			&linearURL,
			&linearTeamKey,
			&linearProjectID,
			&reviewCycle,
			&latestReviewFeedback,
			&prURL,
			&prReviewDecision,
			&prCheckStatus,
			&prCreateState, &prCreateError, &prCreateAttempts, &prCreateAttemptedAt,
			&verifiedSHA, &verifiedBaseSHA, &verifiedAt, &verifiedBy, &staleVerificationReason,
			&blockedReason, &blockedSource, &blockedAt,
		); err != nil {
			return nil, fmt.Errorf("scan plan: %w", err)
		}
		entries = append(entries, TaskEntry{
			Filename:    filename,
			Status:      Status(status),
			Description: description,
			Branch:      branch,
			Topic:       topic,
			CreatedAt:   parseTime(createdAt),
			Implemented: implemented,
			ExecutionState: ExecutionState{
				Phase:           executionPhase,
				ActiveAgentType: activeAgentType,
				ActiveWave:      activeWave,
			},
			PlanningAt:           parseTime(planningAt),
			ImplementingAt:       parseTime(implementingAt),
			ReviewingAt:          parseTime(reviewingAt),
			VerifyingAt:          parseTime(verifyingAt),
			DoneAt:               parseTime(doneAt),
			Goal:                 goal,
			Content:              content,
			ClickUpTaskID:        clickupTaskID,
			LinearIssueID:        linearIssueID,
			LinearIdentifier:     linearIdentifier,
			LinearURL:            linearURL,
			LinearTeamKey:        linearTeamKey,
			LinearProjectID:      linearProjectID,
			ReviewCycle:          reviewCycle,
			LatestReviewFeedback: latestReviewFeedback,
			PRURL:                prURL,
			PRReviewDecision:     prReviewDecision,
			PRCheckStatus:        prCheckStatus,
			PRCreateState:        prCreateState, PRCreateError: prCreateError, PRCreateAttempts: prCreateAttempts, PRCreateAttemptedAt: parseTime(prCreateAttemptedAt),
			VerifiedSHA: verifiedSHA, VerifiedBaseSHA: verifiedBaseSHA, VerifiedAt: parseTime(verifiedAt), VerifiedBy: verifiedBy, StaleVerificationReason: staleVerificationReason,
			BlockedReason: blockedReason, BlockedSource: blockedSource, BlockedAt: parseTime(blockedAt),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate tasks: %w", err)
	}
	return entries, nil
}

// formatTime formats a time.Time as RFC3339 for storage. Zero time returns empty string.
func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339Nano)
}

// parseTime parses an RFC3339 string. Returns zero time on empty or invalid input.
func parseTime(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		return time.Time{}
	}
	return t
}

// isUniqueConstraintError returns true if the error is a SQLite UNIQUE constraint violation.
func isUniqueConstraintError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "UNIQUE constraint failed")
}
