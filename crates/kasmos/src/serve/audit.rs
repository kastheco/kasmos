//! Audit logging for MCP tool invocations.

use crate::config::AuditConfig;
use anyhow::{Context, Result};
use chrono::{DateTime, Duration, Utc};
use serde::{Deserialize, Serialize};
use serde_json::Value;
use std::ffi::OsStr;
use std::fs::{File, OpenOptions};
use std::io::{BufRead, BufReader, Write};
use std::path::{Path, PathBuf};

const RETENTION_CHECK_EVERY_WRITES: u64 = 64;

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct AuditEntry {
    pub timestamp: DateTime<Utc>,
    pub actor: String,
    pub action: String,
    pub feature_slug: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub wp_id: Option<String>,
    pub status: String,
    pub summary: String,
    pub details: Value,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub debug_payload: Option<Value>,
}

impl AuditEntry {
    pub fn new(actor: &str, action: &str, feature_slug: &str) -> Self {
        Self {
            timestamp: Utc::now(),
            actor: actor.to_string(),
            action: action.to_string(),
            feature_slug: feature_slug.to_string(),
            wp_id: None,
            status: String::new(),
            summary: String::new(),
            details: Value::Null,
            debug_payload: None,
        }
    }

    pub fn with_wp_id(mut self, wp_id: impl Into<String>) -> Self {
        self.wp_id = Some(wp_id.into());
        self
    }

    pub fn with_status(mut self, status: impl Into<String>) -> Self {
        self.status = status.into();
        self
    }

    pub fn with_summary(mut self, summary: impl Into<String>) -> Self {
        self.summary = summary.into();
        self
    }

    pub fn with_details(mut self, details: Value) -> Self {
        self.details = details;
        self
    }

    pub fn with_debug_payload(mut self, payload: Value, enabled: bool) -> Self {
        if enabled {
            self.debug_payload = Some(payload);
        }
        self
    }
}

#[derive(Debug)]
pub struct AuditWriter {
    feature_slug: String,
    path: PathBuf,
    config: AuditConfig,
    file: Option<File>,
    writes_since_retention_check: u64,
}

impl AuditWriter {
    pub fn new(
        feature_dir: &Path,
        feature_slug: impl Into<String>,
        config: &AuditConfig,
    ) -> Result<Self> {
        let audit_dir = feature_dir.join(".kasmos");
        std::fs::create_dir_all(&audit_dir)
            .with_context(|| format!("failed creating audit dir {}", audit_dir.display()))?;
        let path = audit_dir.join("messages.jsonl");

        Ok(Self {
            feature_slug: feature_slug.into(),
            path,
            config: config.clone(),
            file: None,
            writes_since_retention_check: 0,
        })
    }

    pub fn feature_slug(&self) -> &str {
        &self.feature_slug
    }

    pub fn write_entry(&mut self, entry: &AuditEntry) -> Result<()> {
        let mut writable_entry = entry.clone();
        writable_entry.debug_payload =
            self.prepare_debug_payload(writable_entry.debug_payload.take());
        let json =
            serde_json::to_string(&writable_entry).context("failed to serialize audit entry")?;

        self.maybe_rotate()?;
        let file = self.get_or_open_file()?;
        writeln!(file, "{json}").context("failed writing audit entry")?;
        file.flush().context("failed flushing audit entry")?;
        self.writes_since_retention_check = self.writes_since_retention_check.saturating_add(1);
        Ok(())
    }

    pub fn check_retention(&self) -> Result<bool> {
        if !self.path.exists() {
            return Ok(false);
        }

        let metadata = std::fs::metadata(&self.path)
            .with_context(|| format!("failed reading metadata for {}", self.path.display()))?;
        if metadata.len() > self.config.max_bytes {
            return Ok(true);
        }

        if let Some(oldest) = self.read_oldest_entry_timestamp()? {
            let age = Utc::now() - oldest;
            if age > Duration::days(self.config.max_age_days as i64) {
                return Ok(true);
            }
        }

        Ok(false)
    }

    pub fn rotate(&mut self) -> Result<()> {
        self.file = None;

        if !self.path.exists() {
            return Ok(());
        }

        let archive_name = format!("messages.{}.jsonl", Utc::now().format("%Y%m%d-%H%M%S"));
        let archive_path = self.path.with_file_name(archive_name);
        std::fs::rename(&self.path, &archive_path).with_context(|| {
            format!(
                "failed rotating audit file {} to {}",
                self.path.display(),
                archive_path.display()
            )
        })?;

        self.prune_old_archives()?;
        self.writes_since_retention_check = 0;
        Ok(())
    }

    fn maybe_rotate(&mut self) -> Result<()> {
        if self.writes_since_retention_check < RETENTION_CHECK_EVERY_WRITES {
            return Ok(());
        }

        self.writes_since_retention_check = 0;
        if self.check_retention()? {
            self.rotate()?;
        }
        Ok(())
    }

    fn prepare_debug_payload(&self, payload: Option<Value>) -> Option<Value> {
        if !self.config.debug_full_payload {
            return None;
        }

        payload.map(redact_sensitive_values)
    }

    fn get_or_open_file(&mut self) -> Result<&mut File> {
        if self.file.is_none() {
            let file = OpenOptions::new()
                .create(true)
                .append(true)
                .open(&self.path)
                .with_context(|| format!("failed opening audit file {}", self.path.display()))?;
            self.file = Some(file);
        }

        self.file
            .as_mut()
            .context("audit file handle must exist after open")
    }

    fn read_oldest_entry_timestamp(&self) -> Result<Option<DateTime<Utc>>> {
        if !self.path.exists() {
            return Ok(None);
        }

        let file = File::open(&self.path)
            .with_context(|| format!("failed opening audit file {}", self.path.display()))?;
        let mut reader = BufReader::new(file);
        let mut line = String::new();
        let bytes = reader
            .read_line(&mut line)
            .context("failed reading oldest audit line")?;
        if bytes == 0 {
            return Ok(None);
        }

        let entry: AuditEntry = serde_json::from_str(line.trim())
            .context("failed parsing oldest audit entry for retention check")?;
        Ok(Some(entry.timestamp))
    }

    fn prune_old_archives(&self) -> Result<()> {
        let Some(audit_dir) = self.path.parent() else {
            return Ok(());
        };

        let max_age = Duration::days(self.config.max_age_days as i64);
        for entry in std::fs::read_dir(audit_dir)
            .with_context(|| format!("failed reading audit dir {}", audit_dir.display()))?
        {
            let entry = entry?;
            let path = entry.path();
            if path.extension() != Some(OsStr::new("jsonl")) {
                continue;
            }

            let Some(name) = path.file_name().and_then(|v| v.to_str()) else {
                continue;
            };
            if !name.starts_with("messages.") || name == "messages.jsonl" {
                continue;
            }

            let metadata = std::fs::metadata(&path)?;
            let modified: DateTime<Utc> = metadata.modified()?.into();
            if Utc::now() - modified > max_age {
                std::fs::remove_file(&path).with_context(|| {
                    format!("failed pruning old audit archive {}", path.display())
                })?;
            }
        }

        Ok(())
    }
}

pub fn resolve_feature_dir(specs_root: &Path, feature_slug: &str) -> PathBuf {
    if specs_root
        .file_name()
        .and_then(|name| name.to_str())
        .is_some_and(|name| name == feature_slug)
    {
        specs_root.to_path_buf()
    } else {
        specs_root.join(feature_slug)
    }
}

fn redact_sensitive_values(value: Value) -> Value {
    match value {
        Value::Object(map) => {
            let mut redacted = serde_json::Map::with_capacity(map.len());
            for (key, val) in map {
                if is_sensitive_key(&key) {
                    redacted.insert(key, Value::String("[REDACTED]".to_string()));
                } else {
                    redacted.insert(key, redact_sensitive_values(val));
                }
            }
            Value::Object(redacted)
        }
        Value::Array(items) => {
            Value::Array(items.into_iter().map(redact_sensitive_values).collect())
        }
        other => other,
    }
}

fn is_sensitive_key(key: &str) -> bool {
    let normalized = key.to_ascii_lowercase();
    [
        "token",
        "secret",
        "password",
        "apikey",
        "api_key",
        "authorization",
    ]
    .iter()
    .any(|needle| normalized.contains(needle))
}

#[cfg(test)]
mod tests {
    use super::*;
    use tempfile::tempdir;

    fn new_writer(config: &AuditConfig) -> (tempfile::TempDir, AuditWriter) {
        let tmp = tempdir().expect("tempdir");
        let feature = tmp.path().join("011-feature");
        std::fs::create_dir_all(&feature).expect("feature dir");
        let writer = AuditWriter::new(&feature, "011-feature", config).expect("writer");
        (tmp, writer)
    }

    #[test]
    fn default_mode_strips_debug_payload() {
        let config = AuditConfig {
            metadata_only: true,
            debug_full_payload: false,
            max_bytes: 1024,
            max_age_days: 14,
        };
        let (_tmp, mut writer) = new_writer(&config);

        let entry = AuditEntry::new("manager", "spawn_worker", "011-feature")
            .with_status("ok")
            .with_summary("spawned")
            .with_debug_payload(
                serde_json::json!({"token": "super-secret", "prompt": "do work"}),
                true,
            );

        writer.write_entry(&entry).expect("write entry");

        let contents = std::fs::read_to_string(writer.path).expect("read audit");
        let parsed: Value = serde_json::from_str(contents.trim()).expect("json");
        assert!(parsed.get("debug_payload").is_none());
    }

    #[test]
    fn debug_mode_keeps_payload_with_redaction() {
        let config = AuditConfig {
            metadata_only: true,
            debug_full_payload: true,
            max_bytes: 1024,
            max_age_days: 14,
        };
        let (_tmp, mut writer) = new_writer(&config);

        let entry = AuditEntry::new("manager", "spawn_worker", "011-feature")
            .with_status("ok")
            .with_summary("spawned")
            .with_debug_payload(
                serde_json::json!({"token": "super-secret", "prompt": "do work"}),
                true,
            );

        writer.write_entry(&entry).expect("write entry");

        let contents = std::fs::read_to_string(writer.path).expect("read audit");
        let parsed: Value = serde_json::from_str(contents.trim()).expect("json");
        let payload = parsed.get("debug_payload").expect("debug payload");
        assert_eq!(payload["token"], Value::String("[REDACTED]".to_string()));
        assert_eq!(payload["prompt"], Value::String("do work".to_string()));
    }

    #[test]
    fn retention_triggers_on_size_threshold() {
        let config = AuditConfig {
            metadata_only: true,
            debug_full_payload: false,
            max_bytes: 1,
            max_age_days: 14,
        };
        let (_tmp, mut writer) = new_writer(&config);
        let entry = AuditEntry::new("manager", "spawn_worker", "011-feature")
            .with_status("ok")
            .with_summary("spawned");
        writer.write_entry(&entry).expect("write entry");

        assert!(writer.check_retention().expect("retention check"));
    }

    #[test]
    fn retention_triggers_on_age_threshold() {
        let config = AuditConfig {
            metadata_only: true,
            debug_full_payload: false,
            max_bytes: 1024,
            max_age_days: 1,
        };
        let (_tmp, mut writer) = new_writer(&config);
        let stale_entry = AuditEntry {
            timestamp: Utc::now() - Duration::days(2),
            actor: "manager".to_string(),
            action: "spawn_worker".to_string(),
            feature_slug: "011-feature".to_string(),
            wp_id: None,
            status: "ok".to_string(),
            summary: "spawned".to_string(),
            details: Value::Null,
            debug_payload: None,
        };
        writer.write_entry(&stale_entry).expect("write entry");

        assert!(writer.check_retention().expect("retention check"));
    }

    #[test]
    fn rotation_creates_archive_file() {
        let config = AuditConfig {
            metadata_only: true,
            debug_full_payload: false,
            max_bytes: 1024,
            max_age_days: 14,
        };
        let (_tmp, mut writer) = new_writer(&config);
        let entry = AuditEntry::new("manager", "spawn_worker", "011-feature")
            .with_status("ok")
            .with_summary("spawned");
        writer.write_entry(&entry).expect("write entry");

        writer.rotate().expect("rotate");

        let audit_dir = writer.path.parent().expect("audit dir");
        let mut archives = Vec::new();
        for entry in std::fs::read_dir(audit_dir).expect("read dir") {
            let entry = entry.expect("entry");
            let name = entry.file_name();
            let name = name.to_string_lossy();
            if name.starts_with("messages.") && name.ends_with(".jsonl") && name != "messages.jsonl"
            {
                archives.push(name.to_string());
            }
        }

        assert!(!archives.is_empty());
        assert!(!writer.path.exists());
    }

    #[test]
    fn writes_are_append_only() {
        let config = AuditConfig {
            metadata_only: true,
            debug_full_payload: false,
            max_bytes: 1024,
            max_age_days: 14,
        };
        let (_tmp, mut writer) = new_writer(&config);

        let first = AuditEntry::new("manager", "spawn_worker", "011-feature")
            .with_status("ok")
            .with_summary("first");
        let second = AuditEntry::new("manager", "despawn_worker", "011-feature")
            .with_status("ok")
            .with_summary("second");

        writer.write_entry(&first).expect("write first");
        writer.write_entry(&second).expect("write second");

        let contents = std::fs::read_to_string(writer.path).expect("read audit");
        let lines = contents.lines().collect::<Vec<_>>();
        assert_eq!(lines.len(), 2);

        let first_json: Value = serde_json::from_str(lines[0]).expect("first json");
        let second_json: Value = serde_json::from_str(lines[1]).expect("second json");
        assert_eq!(first_json["summary"], Value::String("first".to_string()));
        assert_eq!(second_json["summary"], Value::String("second".to_string()));
    }
}
