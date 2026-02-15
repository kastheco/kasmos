//! Repository-wide feature locking for launch and MCP mutations.

use crate::config::LockConfig;
use chrono::{DateTime, Duration, Utc};
use serde::{Deserialize, Serialize};
use std::fmt;
use std::fs::{self, File, OpenOptions};
use std::io::{Read, Write};
use std::os::fd::AsRawFd;
use std::path::{Path, PathBuf};
use std::process::Command;
use std::sync::OnceLock;
use thiserror::Error;

pub const FEATURE_LOCK_CONFLICT_CODE: &str = "FEATURE_LOCK_CONFLICT";
pub const STALE_LOCK_CONFIRMATION_REQUIRED_CODE: &str = "STALE_LOCK_CONFIRMATION_REQUIRED";

static REPO_ROOT_CACHE: OnceLock<PathBuf> = OnceLock::new();

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct LockKey {
    pub repo_root: PathBuf,
    pub feature_slug: String,
}

impl LockKey {
    pub fn new(feature_slug: &str) -> Result<Self, LockError> {
        let repo_root = resolve_repo_root()?;
        Ok(Self {
            repo_root,
            feature_slug: feature_slug.to_string(),
        })
    }

    pub fn as_lock_key(&self) -> String {
        format!("{}::{}", self.repo_root.display(), self.feature_slug)
    }

    pub fn lock_file_path(&self) -> PathBuf {
        self.repo_root
            .join(".kasmos")
            .join("locks")
            .join(format!("{}.lock", self.feature_slug))
    }

    fn advisory_guard_path(&self) -> PathBuf {
        self.repo_root
            .join(".kasmos")
            .join("locks")
            .join(format!("{}.lock.guard", self.feature_slug))
    }

    pub fn ensure_lock_dir(&self) -> Result<(), LockError> {
        let lock_dir = self.repo_root.join(".kasmos").join("locks");
        fs::create_dir_all(&lock_dir).map_err(|source| LockError::Io {
            source,
            context: format!("create lock directory {}", lock_dir.display()),
        })
    }

    #[cfg(test)]
    fn for_repo_root(repo_root: PathBuf, feature_slug: &str) -> Self {
        Self {
            repo_root,
            feature_slug: feature_slug.to_string(),
        }
    }
}

impl fmt::Display for LockKey {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.write_str(&self.as_lock_key())
    }
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "snake_case")]
pub enum LockStatus {
    Active,
    Stale,
    Released,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub struct LockRecord {
    pub lock_key: String,
    pub repo_root: String,
    pub feature_slug: String,
    pub owner_id: String,
    pub owner_session: String,
    pub owner_tab: String,
    pub acquired_at: DateTime<Utc>,
    pub last_heartbeat_at: DateTime<Utc>,
    pub expires_at: DateTime<Utc>,
    pub status: LockStatus,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub enum LockConflict {
    None,
    ActiveOwner(LockRecord),
    Stale(LockRecord),
}

#[derive(Debug, Clone)]
pub struct FeatureLockManager {
    key: LockKey,
    owner_id: String,
    owner_session: String,
    owner_tab: String,
    config: LockConfig,
}

impl FeatureLockManager {
    pub fn new(
        feature_slug: &str,
        owner_session: impl Into<String>,
        owner_tab: impl Into<String>,
        config: LockConfig,
    ) -> Result<Self, LockError> {
        Ok(Self {
            key: LockKey::new(feature_slug)?,
            owner_id: owner_identity(),
            owner_session: owner_session.into(),
            owner_tab: owner_tab.into(),
            config,
        })
    }

    #[cfg(test)]
    fn for_tests(
        repo_root: PathBuf,
        feature_slug: &str,
        owner_session: impl Into<String>,
        owner_tab: impl Into<String>,
        config: LockConfig,
    ) -> Self {
        Self {
            key: LockKey::for_repo_root(repo_root, feature_slug),
            owner_id: owner_identity(),
            owner_session: owner_session.into(),
            owner_tab: owner_tab.into(),
            config,
        }
    }

    pub fn key(&self) -> &LockKey {
        &self.key
    }

    pub fn lock_file_path(&self) -> PathBuf {
        self.key.lock_file_path()
    }

    pub fn acquire(&self, allow_stale_takeover: bool) -> Result<LockRecord, LockError> {
        self.with_advisory_lock(|this| {
            let existing = this.read_record_if_exists()?;
            if let Some(existing) = existing {
                match this.classify_record(existing.clone()) {
                    LockConflict::None => {}
                    LockConflict::ActiveOwner(record) => {
                        return Err(LockError::from_conflict(record));
                    }
                    LockConflict::Stale(record) if !allow_stale_takeover => {
                        return Err(LockError::from_stale_confirmation(record));
                    }
                    LockConflict::Stale(_) => {}
                }
            }

            let now = Utc::now();
            let record = this.build_record(now, LockStatus::Active);
            this.write_record_atomic(&record)?;
            Ok(record)
        })
    }

    pub fn heartbeat(&self) -> Result<LockRecord, LockError> {
        self.with_advisory_lock(|this| {
            let mut record =
                this.read_record_if_exists()?
                    .ok_or_else(|| LockError::MissingLock {
                        path: this.lock_file_path(),
                    })?;

            if record.owner_id != this.owner_id {
                return Err(LockError::NotLockOwner {
                    expected: this.owner_id.clone(),
                    actual: record.owner_id,
                });
            }

            let now = Utc::now();
            record.last_heartbeat_at = now;
            record.expires_at = now + this.stale_threshold();
            record.status = LockStatus::Active;
            this.write_record_atomic(&record)?;
            Ok(record)
        })
    }

    pub fn release(&self) -> Result<(), LockError> {
        self.with_advisory_lock(|this| {
            let maybe_record = this.read_record_if_exists()?;
            if let Some(mut record) = maybe_record {
                if record.owner_id != this.owner_id {
                    return Err(LockError::NotLockOwner {
                        expected: this.owner_id.clone(),
                        actual: record.owner_id,
                    });
                }
                record.status = LockStatus::Released;
                record.last_heartbeat_at = Utc::now();
                record.expires_at = record.last_heartbeat_at;
                this.write_record_atomic(&record)?;
            }

            let lock_path = this.lock_file_path();
            if lock_path.exists() {
                fs::remove_file(&lock_path).map_err(|source| LockError::Io {
                    source,
                    context: format!("remove lock file {}", lock_path.display()),
                })?;
            }
            Ok(())
        })
    }

    pub fn check_conflict(&self) -> Result<LockConflict, LockError> {
        self.with_advisory_lock(|this| {
            let Some(record) = this.read_record_if_exists()? else {
                return Ok(LockConflict::None);
            };
            Ok(this.classify_record(record))
        })
    }

    fn classify_record(&self, mut record: LockRecord) -> LockConflict {
        if record.status != LockStatus::Active {
            return LockConflict::None;
        }
        if is_stale(&record, &self.config) {
            record.status = LockStatus::Stale;
            return LockConflict::Stale(record);
        }
        LockConflict::ActiveOwner(record)
    }

    fn stale_threshold(&self) -> Duration {
        Duration::minutes(self.config.stale_timeout_minutes as i64)
    }

    fn build_record(&self, now: DateTime<Utc>, status: LockStatus) -> LockRecord {
        LockRecord {
            lock_key: self.key.as_lock_key(),
            repo_root: self.key.repo_root.display().to_string(),
            feature_slug: self.key.feature_slug.clone(),
            owner_id: self.owner_id.clone(),
            owner_session: self.owner_session.clone(),
            owner_tab: self.owner_tab.clone(),
            acquired_at: now,
            last_heartbeat_at: now,
            expires_at: now + self.stale_threshold(),
            status,
        }
    }

    fn read_record_if_exists(&self) -> Result<Option<LockRecord>, LockError> {
        let lock_path = self.lock_file_path();
        if !lock_path.exists() {
            return Ok(None);
        }

        let mut file = File::open(&lock_path).map_err(|source| LockError::Io {
            source,
            context: format!("open lock file {}", lock_path.display()),
        })?;
        let mut data = String::new();
        file.read_to_string(&mut data)
            .map_err(|source| LockError::Io {
                source,
                context: format!("read lock file {}", lock_path.display()),
            })?;

        if data.trim().is_empty() {
            return Ok(None);
        }

        let record = serde_json::from_str::<LockRecord>(&data).map_err(|source| {
            LockError::InvalidRecord {
                source,
                path: lock_path,
            }
        })?;
        Ok(Some(record))
    }

    fn write_record_atomic(&self, record: &LockRecord) -> Result<(), LockError> {
        self.key.ensure_lock_dir()?;

        let lock_path = self.lock_file_path();
        let temp_path = lock_path.with_extension(format!("tmp.{}", std::process::id()));
        let payload = serde_json::to_vec_pretty(record)
            .map_err(|source| LockError::SerializeRecord { source })?;

        {
            let mut temp_file = File::create(&temp_path).map_err(|source| LockError::Io {
                source,
                context: format!("create temp lock file {}", temp_path.display()),
            })?;
            temp_file
                .write_all(&payload)
                .map_err(|source| LockError::Io {
                    source,
                    context: format!("write temp lock file {}", temp_path.display()),
                })?;
            temp_file.flush().map_err(|source| LockError::Io {
                source,
                context: format!("flush temp lock file {}", temp_path.display()),
            })?;
            temp_file.sync_all().map_err(|source| LockError::Io {
                source,
                context: format!("sync temp lock file {}", temp_path.display()),
            })?;
        }

        fs::rename(&temp_path, &lock_path).map_err(|source| LockError::Io {
            source,
            context: format!(
                "rename temp lock file {} -> {}",
                temp_path.display(),
                lock_path.display()
            ),
        })?;

        Ok(())
    }

    fn with_advisory_lock<T, F>(&self, f: F) -> Result<T, LockError>
    where
        F: FnOnce(&Self) -> Result<T, LockError>,
    {
        self.key.ensure_lock_dir()?;
        let guard_path = self.key.advisory_guard_path();
        let guard_file = OpenOptions::new()
            .create(true)
            .read(true)
            .write(true)
            .truncate(false)
            .open(&guard_path)
            .map_err(|source| LockError::Io {
                source,
                context: format!("open advisory lock guard {}", guard_path.display()),
            })?;

        flock_exclusive_nonblocking(&guard_file, &guard_path)?;

        let result = f(self);

        let unlock_result = flock_unlock(&guard_file);
        if let Err(source) = unlock_result {
            return Err(LockError::Io {
                source: std::io::Error::other(source),
                context: format!("unlock advisory lock guard {}", guard_path.display()),
            });
        }

        result
    }
}

#[derive(Debug, Error)]
pub enum LockError {
    #[error("failed to resolve repository root via git rev-parse: {reason}")]
    ResolveRepoRoot { reason: String },

    #[error("I/O error while trying to {context}: {source}")]
    Io {
        context: String,
        #[source]
        source: std::io::Error,
    },

    #[error("failed to serialize lock record: {source}")]
    SerializeRecord {
        #[source]
        source: serde_json::Error,
    },

    #[error("invalid lock record in {path}: {source}")]
    InvalidRecord {
        #[source]
        source: serde_json::Error,
        path: PathBuf,
    },

    #[error("failed to acquire advisory lock for {path}: {source}")]
    AdvisoryLockBusy {
        path: PathBuf,
        #[source]
        source: std::io::Error,
    },

    #[error("[{FEATURE_LOCK_CONFLICT_CODE}] {message}")]
    FeatureLockConflict {
        message: String,
        record: Box<LockRecord>,
    },

    #[error("[{STALE_LOCK_CONFIRMATION_REQUIRED_CODE}] {message}")]
    StaleLockConfirmationRequired {
        message: String,
        record: Box<LockRecord>,
    },

    #[error("lock file not found at {path}")]
    MissingLock { path: PathBuf },

    #[error("lock owner mismatch: expected {expected}, found {actual}")]
    NotLockOwner { expected: String, actual: String },
}

impl LockError {
    pub fn code(&self) -> Option<&'static str> {
        match self {
            Self::FeatureLockConflict { .. } => Some(FEATURE_LOCK_CONFLICT_CODE),
            Self::StaleLockConfirmationRequired { .. } => {
                Some(STALE_LOCK_CONFIRMATION_REQUIRED_CODE)
            }
            _ => None,
        }
    }

    pub fn from_conflict(record: LockRecord) -> Self {
        Self::FeatureLockConflict {
            message: format_active_owner_conflict(&record),
            record: Box::new(record),
        }
    }

    pub fn from_stale_confirmation(record: LockRecord) -> Self {
        Self::StaleLockConfirmationRequired {
            message: format_stale_lock_prompt(&record),
            record: Box::new(record),
        }
    }
}

pub fn format_active_owner_conflict(record: &LockRecord) -> String {
    format!(
        "Feature '{}' is already owned by:\n  PID: {}\n  Session: {}\n  Acquired: {}\n  Last heartbeat: {}",
        record.feature_slug,
        record.owner_id,
        record.owner_session,
        record.acquired_at.to_rfc3339(),
        record.last_heartbeat_at.to_rfc3339()
    )
}

pub fn format_stale_lock_prompt(record: &LockRecord) -> String {
    let age = Utc::now() - record.last_heartbeat_at;
    let hours = age.num_hours();
    let minutes = age.num_minutes() % 60;
    format!(
        "Feature '{}' has a stale lock:\n  PID: {} (may be dead)\n  Session: {}\n  Last heartbeat: {} ({}h {}m ago)",
        record.feature_slug,
        record.owner_id,
        record.owner_session,
        record.last_heartbeat_at.to_rfc3339(),
        hours,
        minutes
    )
}

pub fn is_stale(record: &LockRecord, config: &LockConfig) -> bool {
    let now = Utc::now();
    let stale_threshold = Duration::minutes(config.stale_timeout_minutes as i64);
    now - record.last_heartbeat_at > stale_threshold
}

fn resolve_repo_root() -> Result<PathBuf, LockError> {
    if let Some(cached) = REPO_ROOT_CACHE.get() {
        return Ok(cached.clone());
    }

    let output = Command::new("git")
        .args(["rev-parse", "--show-toplevel"])
        .output()
        .map_err(|source| LockError::Io {
            source,
            context: "execute git rev-parse --show-toplevel".to_string(),
        })?;

    if !output.status.success() {
        let reason = String::from_utf8_lossy(&output.stderr).trim().to_string();
        return Err(LockError::ResolveRepoRoot { reason });
    }

    let root = String::from_utf8_lossy(&output.stdout).trim().to_string();
    if root.is_empty() {
        return Err(LockError::ResolveRepoRoot {
            reason: "git returned empty repository root".to_string(),
        });
    }

    let canonical = Path::new(&root)
        .canonicalize()
        .map_err(|source| LockError::Io {
            source,
            context: format!("canonicalize repository root {}", root),
        })?;
    let _ = REPO_ROOT_CACHE.set(canonical.clone());
    Ok(canonical)
}

fn owner_identity() -> String {
    format!("{}@{}", std::process::id(), local_hostname())
}

fn flock_exclusive_nonblocking(file: &File, path: &Path) -> Result<(), LockError> {
    let rc = unsafe { libc::flock(file.as_raw_fd(), libc::LOCK_EX | libc::LOCK_NB) };
    if rc == 0 {
        return Ok(());
    }

    Err(LockError::AdvisoryLockBusy {
        path: path.to_path_buf(),
        source: std::io::Error::last_os_error(),
    })
}

fn flock_unlock(file: &File) -> std::io::Result<()> {
    let rc = unsafe { libc::flock(file.as_raw_fd(), libc::LOCK_UN) };
    if rc == 0 {
        return Ok(());
    }
    Err(std::io::Error::last_os_error())
}

fn local_hostname() -> String {
    if let Ok(hostname) = std::env::var("HOSTNAME") {
        let trimmed = hostname.trim();
        if !trimmed.is_empty() {
            return trimmed.to_string();
        }
    }

    let candidate_path = PathBuf::from("/etc/hostname");
    if let Ok(contents) = fs::read_to_string(&candidate_path) {
        let trimmed = contents.trim();
        if !trimmed.is_empty() {
            return trimmed.to_string();
        }
    }

    let output = Command::new("hostname").output();
    if let Ok(output) = output {
        let out = String::from_utf8_lossy(&output.stdout).trim().to_string();
        if !out.is_empty() {
            return out;
        }
    }

    "unknown-host".to_string()
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::sync::{Arc, Barrier, Mutex};
    use std::thread;

    fn test_lock_config(timeout_minutes: u64) -> LockConfig {
        LockConfig {
            stale_timeout_minutes: timeout_minutes,
        }
    }

    fn manager(repo_root: &Path, feature_slug: &str, session: &str) -> FeatureLockManager {
        FeatureLockManager::for_tests(
            repo_root.to_path_buf(),
            feature_slug,
            session,
            "manager-tab",
            test_lock_config(15),
        )
    }

    #[test]
    fn lock_key_is_deterministic_for_repo_and_feature() {
        let tmp = tempfile::tempdir().expect("create tempdir");
        let key1 = LockKey::for_repo_root(tmp.path().to_path_buf(), "011-feature");
        let key2 = LockKey::for_repo_root(tmp.path().to_path_buf(), "011-feature");

        assert_eq!(key1.as_lock_key(), key2.as_lock_key());
        assert_eq!(
            key1.lock_file_path(),
            tmp.path()
                .join(".kasmos")
                .join("locks")
                .join("011-feature.lock")
        );
    }

    #[test]
    fn acquire_creates_lock_file_with_record() {
        let tmp = tempfile::tempdir().expect("create tempdir");
        let manager = manager(tmp.path(), "011-feature", "kasmos-session");

        let record = manager.acquire(false).expect("acquire lock");

        let stored = manager
            .read_record_if_exists()
            .expect("read lock")
            .expect("record exists");

        assert_eq!(record.feature_slug, "011-feature");
        assert_eq!(stored.status, LockStatus::Active);
        assert_eq!(stored.lock_key, record.lock_key);
    }

    #[test]
    fn heartbeat_updates_timestamps() {
        let tmp = tempfile::tempdir().expect("create tempdir");
        let manager = manager(tmp.path(), "011-feature", "kasmos-session");
        let initial = manager.acquire(false).expect("acquire");

        std::thread::sleep(std::time::Duration::from_millis(5));
        let updated = manager.heartbeat().expect("heartbeat");

        assert!(updated.last_heartbeat_at > initial.last_heartbeat_at);
        assert!(updated.expires_at > initial.expires_at);
    }

    #[test]
    fn active_lock_reports_conflict() {
        let tmp = tempfile::tempdir().expect("create tempdir");
        let manager_a = manager(tmp.path(), "011-feature", "session-a");
        manager_a.acquire(false).expect("manager a acquires");

        let manager_b = FeatureLockManager::for_tests(
            tmp.path().to_path_buf(),
            "011-feature",
            "session-b",
            "manager-tab",
            test_lock_config(15),
        );

        let err = manager_b
            .acquire(false)
            .expect_err("manager b should conflict");
        assert_eq!(err.code(), Some(FEATURE_LOCK_CONFLICT_CODE));
    }

    #[test]
    fn stale_lock_requires_confirmation_then_allows_takeover() {
        let tmp = tempfile::tempdir().expect("create tempdir");
        let manager_a = FeatureLockManager::for_tests(
            tmp.path().to_path_buf(),
            "011-feature",
            "session-a",
            "manager-tab",
            test_lock_config(1),
        );
        let mut record = manager_a.acquire(false).expect("acquire");
        record.last_heartbeat_at = Utc::now() - Duration::minutes(2);
        record.expires_at = record.last_heartbeat_at + Duration::minutes(1);
        manager_a
            .write_record_atomic(&record)
            .expect("write stale record");

        let manager_b = FeatureLockManager::for_tests(
            tmp.path().to_path_buf(),
            "011-feature",
            "session-b",
            "manager-tab",
            test_lock_config(1),
        );

        let err = manager_b
            .acquire(false)
            .expect_err("should require confirmation");
        assert_eq!(err.code(), Some(STALE_LOCK_CONFIRMATION_REQUIRED_CODE));

        let takeover = manager_b
            .acquire(true)
            .expect("takeover should succeed with confirmation");
        assert_eq!(takeover.owner_session, "session-b");
    }

    #[test]
    fn release_removes_lock_file() {
        let tmp = tempfile::tempdir().expect("create tempdir");
        let manager = manager(tmp.path(), "011-feature", "kasmos-session");
        manager.acquire(false).expect("acquire");

        manager.release().expect("release");

        assert!(!manager.lock_file_path().exists());
    }

    #[test]
    fn conflict_classifier_marks_stale_when_heartbeat_old() {
        let tmp = tempfile::tempdir().expect("create tempdir");
        let manager = FeatureLockManager::for_tests(
            tmp.path().to_path_buf(),
            "011-feature",
            "session-a",
            "manager-tab",
            test_lock_config(1),
        );

        let mut record =
            manager.build_record(Utc::now() - Duration::minutes(5), LockStatus::Active);
        record.last_heartbeat_at = Utc::now() - Duration::minutes(5);
        record.expires_at = record.last_heartbeat_at + Duration::minutes(1);

        let conflict = manager.classify_record(record);
        assert!(matches!(conflict, LockConflict::Stale(_)));
    }

    #[test]
    fn concurrent_acquire_allows_single_winner() {
        let tmp = tempfile::tempdir().expect("create tempdir");
        let repo_root = tmp.path().to_path_buf();
        let barrier = Arc::new(Barrier::new(2));
        let outcomes = Arc::new(Mutex::new(Vec::new()));

        let mut handles = Vec::new();
        for i in 0..2 {
            let barrier = Arc::clone(&barrier);
            let outcomes = Arc::clone(&outcomes);
            let repo_root = repo_root.clone();
            handles.push(thread::spawn(move || {
                let manager = FeatureLockManager::for_tests(
                    repo_root,
                    "011-feature",
                    format!("session-{i}"),
                    "manager-tab",
                    test_lock_config(15),
                );
                barrier.wait();
                let outcome = manager.acquire(false);
                outcomes.lock().expect("lock outcomes").push(outcome);
            }));
        }

        for handle in handles {
            handle.join().expect("join");
        }

        let outcomes = outcomes.lock().expect("lock outcomes");
        let success_count = outcomes.iter().filter(|result| result.is_ok()).count();
        assert_eq!(success_count, 1, "exactly one acquire should succeed");
    }
}
