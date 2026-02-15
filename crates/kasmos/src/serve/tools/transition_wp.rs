use crate::serve::KasmosServer;
use anyhow::{Context, Result, bail};
use chrono::Utc;
use schemars::JsonSchema;
use serde::{Deserialize, Serialize};
use serde_yml::{Mapping, Value};
use std::fs::OpenOptions;
use std::os::fd::AsRawFd;
use std::path::{Path, PathBuf};

#[derive(Debug, Clone, Serialize, Deserialize, JsonSchema)]
#[serde(deny_unknown_fields)]
pub struct TransitionWpInput {
    pub feature_slug: String,
    pub wp_id: String,
    pub to_state: TransitionState,
    pub actor: String,
    pub reason: Option<String>,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize, JsonSchema)]
#[serde(rename_all = "snake_case")]
pub enum TransitionState {
    Pending,
    Active,
    ForReview,
    Done,
    Rework,
}

impl TransitionState {
    pub fn as_str(&self) -> &'static str {
        match self {
            Self::Pending => "pending",
            Self::Active => "active",
            Self::ForReview => "for_review",
            Self::Done => "done",
            Self::Rework => "rework",
        }
    }
}

#[derive(Debug, Clone, Serialize, Deserialize, JsonSchema)]
pub struct TransitionWpOutput {
    pub ok: bool,
    pub wp_id: String,
    pub from_state: String,
    pub to_state: String,
}

pub async fn handle(input: TransitionWpInput, server: &KasmosServer) -> Result<TransitionWpOutput> {
    let feature_dir = resolve_feature_dir(
        Path::new(&server.config.paths.specs_root),
        &input.feature_slug,
    )?;
    let wp_file = find_wp_file(&feature_dir, &input.wp_id)?;
    let original = std::fs::read_to_string(&wp_file)
        .with_context(|| format!("Failed to read {}", wp_file.display()))?;

    let from_state = detect_current_state(&original);
    validate_transition(from_state, input.to_state)?;

    if input.to_state == TransitionState::Rework {
        enforce_rejection_cap(
            &original,
            &input.wp_id,
            server.config.agent.review_rejection_cap,
        )?;
    }

    let lane = kasmos_state_to_lane(input.to_state);
    update_task_lane(
        &wp_file,
        lane,
        &input.actor,
        input.reason.as_deref(),
        input.to_state,
    )?;

    tracing::info!(
        wp_id = %input.wp_id,
        from_state = %from_state.as_str(),
        to_state = %input.to_state.as_str(),
        actor = %input.actor,
        "transition_wp applied"
    );

    Ok(TransitionWpOutput {
        ok: true,
        wp_id: input.wp_id,
        from_state: from_state.as_str().to_string(),
        to_state: input.to_state.as_str().to_string(),
    })
}

fn resolve_feature_dir(specs_root: &Path, feature_slug: &str) -> Result<PathBuf> {
    let as_path = PathBuf::from(feature_slug);
    let candidate = if as_path.is_dir() {
        as_path
    } else {
        specs_root.join(feature_slug)
    };

    if !candidate.is_dir() {
        bail!("Feature directory not found: {}", candidate.display());
    }

    candidate
        .canonicalize()
        .with_context(|| format!("Failed to canonicalize {}", candidate.display()))
}

fn find_wp_file(feature_dir: &Path, wp_id: &str) -> Result<PathBuf> {
    let tasks_dir = feature_dir.join("tasks");
    if !tasks_dir.is_dir() {
        bail!("Task directory not found: {}", tasks_dir.display());
    }

    for entry in std::fs::read_dir(&tasks_dir)
        .with_context(|| format!("Failed to read {}", tasks_dir.display()))?
    {
        let entry = entry?;
        let path = entry.path();
        if path.is_file()
            && let Some(name) = path.file_name().and_then(|n| n.to_str())
            && name.starts_with(wp_id)
            && name.ends_with(".md")
        {
            return Ok(path);
        }
    }

    bail!("Work package file not found for {}", wp_id)
}

fn split_frontmatter(content: &str) -> Result<(String, String)> {
    let parts: Vec<&str> = content.splitn(3, "---").collect();
    if parts.len() < 3 {
        bail!("Task file is missing YAML frontmatter");
    }
    Ok((parts[1].trim().to_string(), parts[2].to_string()))
}

fn detect_current_state(content: &str) -> TransitionState {
    let Ok((yaml, _)) = split_frontmatter(content) else {
        return TransitionState::Pending;
    };
    let Ok(value) = serde_yml::from_str::<Value>(&yaml) else {
        return TransitionState::Pending;
    };

    let lane = value
        .as_mapping()
        .and_then(|map| map.get(Value::String("lane".to_string())))
        .and_then(Value::as_str)
        .unwrap_or("planned");

    if lane == "doing" && has_rework_history(&value) {
        return TransitionState::Rework;
    }

    lane_to_kasmos_state(lane)
}

fn has_rework_history(frontmatter: &Value) -> bool {
    let Some(history) = frontmatter
        .as_mapping()
        .and_then(|map| map.get(Value::String("history".to_string())))
        .and_then(Value::as_sequence)
    else {
        return false;
    };

    let mut prev_lane: Option<&str> = None;
    for item in history {
        let lane = item
            .as_mapping()
            .and_then(|m| m.get(Value::String("lane".to_string())))
            .and_then(Value::as_str);
        if prev_lane == Some("for_review") && lane == Some("doing") {
            return true;
        }
        prev_lane = lane;
    }
    false
}

fn lane_to_kasmos_state(lane: &str) -> TransitionState {
    match lane {
        "planned" => TransitionState::Pending,
        "doing" => TransitionState::Active,
        "for_review" => TransitionState::ForReview,
        "done" => TransitionState::Done,
        _ => TransitionState::Pending,
    }
}

fn kasmos_state_to_lane(state: TransitionState) -> &'static str {
    match state {
        TransitionState::Pending => "planned",
        TransitionState::Active => "doing",
        TransitionState::ForReview => "for_review",
        TransitionState::Done => "done",
        TransitionState::Rework => "doing",
    }
}

fn validate_transition(from: TransitionState, to: TransitionState) -> Result<()> {
    let allowed = matches!(
        (from, to),
        (TransitionState::Pending, TransitionState::Pending)
            | (TransitionState::Pending, TransitionState::Active)
            | (TransitionState::Active, TransitionState::Active)
            | (TransitionState::Active, TransitionState::ForReview)
            | (TransitionState::Active, TransitionState::Done)
            | (TransitionState::ForReview, TransitionState::ForReview)
            | (TransitionState::ForReview, TransitionState::Done)
            | (TransitionState::ForReview, TransitionState::Pending)
            | (TransitionState::ForReview, TransitionState::Active)
            | (TransitionState::ForReview, TransitionState::Rework)
            | (TransitionState::Rework, TransitionState::Rework)
            | (TransitionState::Rework, TransitionState::ForReview)
            | (TransitionState::Rework, TransitionState::Done)
            | (TransitionState::Rework, TransitionState::Pending)
            | (TransitionState::Done, TransitionState::Done)
    );

    if !allowed {
        bail!(
            "TRANSITION_NOT_ALLOWED: {} -> {}",
            from.as_str(),
            to.as_str()
        );
    }
    Ok(())
}

fn enforce_rejection_cap(content: &str, wp_id: &str, cap: u32) -> Result<()> {
    let count = rejection_count(content);
    if count >= cap {
        bail!(
            "TRANSITION_NOT_ALLOWED: review rejection cap reached for {} ({}/{})",
            wp_id,
            count,
            cap
        );
    }
    Ok(())
}

fn rejection_count(content: &str) -> u32 {
    let Ok((yaml, _)) = split_frontmatter(content) else {
        return 0;
    };
    let Ok(value) = serde_yml::from_str::<Value>(&yaml) else {
        return 0;
    };

    let Some(history) = value
        .as_mapping()
        .and_then(|map| map.get(Value::String("history".to_string())))
        .and_then(Value::as_sequence)
    else {
        return 0;
    };

    let mut count = 0u32;
    let mut prev_lane: Option<&str> = None;
    for item in history {
        let lane = item
            .as_mapping()
            .and_then(|m| m.get(Value::String("lane".to_string())))
            .and_then(Value::as_str);

        if prev_lane == Some("for_review") && lane == Some("doing") {
            count += 1;
        }
        prev_lane = lane;
    }
    count
}

fn update_task_lane(
    wp_file: &Path,
    new_lane: &str,
    actor: &str,
    reason: Option<&str>,
    to_state: TransitionState,
) -> Result<()> {
    let lock_path = wp_file.with_extension("lock");
    let lock_file = OpenOptions::new()
        .create(true)
        .read(true)
        .write(true)
        .truncate(false)
        .open(&lock_path)
        .with_context(|| format!("Failed to open lock file {}", lock_path.display()))?;

    lock_exclusive_nonblocking(&lock_file)?;

    let write_result = (|| {
        let content = std::fs::read_to_string(wp_file)
            .with_context(|| format!("Failed to read {}", wp_file.display()))?;
        let (yaml, body) = split_frontmatter(&content)?;
        let mut doc: Value = serde_yml::from_str(&yaml)
            .with_context(|| format!("Failed to parse frontmatter in {}", wp_file.display()))?;

        let map = doc
            .as_mapping_mut()
            .ok_or_else(|| anyhow::anyhow!("Frontmatter is not a YAML mapping"))?;
        map.insert(
            Value::String("lane".to_string()),
            Value::String(new_lane.to_string()),
        );

        append_history_entry(map, new_lane, actor, reason, to_state);

        let updated_yaml = serde_yml::to_string(&doc).context("Failed to encode frontmatter")?;
        let next = format!("---\n{}---{}", updated_yaml, body);

        let tmp_path = wp_file.with_extension(format!("tmp.{}", std::process::id()));
        std::fs::write(&tmp_path, next)
            .with_context(|| format!("Failed to write {}", tmp_path.display()))?;
        std::fs::rename(&tmp_path, wp_file)
            .with_context(|| format!("Failed to replace {}", wp_file.display()))?;
        Ok::<(), anyhow::Error>(())
    })();

    unlock_file(&lock_file);
    drop(lock_file);
    let _ = std::fs::remove_file(&lock_path);

    write_result
}

fn append_history_entry(
    map: &mut Mapping,
    lane: &str,
    actor: &str,
    reason: Option<&str>,
    to_state: TransitionState,
) {
    let history_key = Value::String("history".to_string());
    if !map.contains_key(&history_key) {
        map.insert(history_key.clone(), Value::Sequence(Vec::new()));
    }

    let entry = {
        let mut item = Mapping::new();
        item.insert(
            Value::String("timestamp".to_string()),
            Value::String(Utc::now().to_rfc3339()),
        );
        item.insert(
            Value::String("lane".to_string()),
            Value::String(lane.to_string()),
        );
        item.insert(
            Value::String("actor".to_string()),
            Value::String(actor.to_string()),
        );
        item.insert(
            Value::String("shell_pid".to_string()),
            Value::String(String::new()),
        );
        let action = match reason {
            Some(reason) if !reason.trim().is_empty() => {
                format!("transition {} ({})", to_state.as_str(), reason.trim())
            }
            _ => format!("transition {}", to_state.as_str()),
        };
        item.insert(Value::String("action".to_string()), Value::String(action));
        Value::Mapping(item)
    };

    if let Some(Value::Sequence(history)) = map.get_mut(&history_key) {
        history.push(entry);
    }
}

fn lock_exclusive_nonblocking(file: &std::fs::File) -> Result<()> {
    let fd = file.as_raw_fd();
    // SAFETY: fd comes from a live File handle owned by this process.
    let rc = unsafe { libc::flock(fd, libc::LOCK_EX | libc::LOCK_NB) };
    if rc != 0 {
        bail!("TRANSITION_NOT_ALLOWED: concurrent write detected");
    }
    Ok(())
}

fn unlock_file(file: &std::fs::File) {
    let fd = file.as_raw_fd();
    // SAFETY: fd comes from a live File handle owned by this process.
    let _ = unsafe { libc::flock(fd, libc::LOCK_UN) };
}

#[cfg(test)]
mod tests {
    use super::*;
    use tempfile::tempdir;

    fn write_wp(feature_dir: &Path, wp_id: &str, lane: &str, history: &str) {
        let content = format!(
            "---\nwork_package_id: {}\ntitle: test\nlane: {}\ndependencies: []\nhistory:\n{}---\n\n# body\n",
            wp_id, lane, history
        );
        std::fs::write(
            feature_dir.join("tasks").join(format!("{}-test.md", wp_id)),
            content,
        )
        .expect("write wp");
    }

    #[tokio::test]
    async fn transition_updates_lane_and_history() {
        let tmp = tempdir().expect("tempdir");
        let specs_root = tmp.path().join("kitty-specs");
        let feature_dir = specs_root.join("011-alpha");
        std::fs::create_dir_all(feature_dir.join("tasks")).expect("mkdir tasks");
        write_wp(&feature_dir, "WP01", "planned", "  []\n");

        let mut config = crate::config::Config::default();
        config.paths.specs_root = specs_root.display().to_string();
        let server = crate::serve::KasmosServer::new(config).expect("server");

        let output = handle(
            TransitionWpInput {
                feature_slug: "011-alpha".to_string(),
                wp_id: "WP01".to_string(),
                to_state: TransitionState::Active,
                actor: "manager".to_string(),
                reason: Some("start".to_string()),
            },
            &server,
        )
        .await
        .expect("transition");

        assert!(output.ok);
        assert_eq!(output.from_state, "pending");
        assert_eq!(output.to_state, "active");

        let updated =
            std::fs::read_to_string(feature_dir.join("tasks/WP01-test.md")).expect("read");
        assert!(updated.contains("lane: doing"));
        assert!(updated.contains("transition active (start)"));
    }

    #[tokio::test]
    async fn invalid_transition_returns_transition_not_allowed() {
        let tmp = tempdir().expect("tempdir");
        let specs_root = tmp.path().join("kitty-specs");
        let feature_dir = specs_root.join("011-beta");
        std::fs::create_dir_all(feature_dir.join("tasks")).expect("mkdir tasks");
        write_wp(&feature_dir, "WP01", "planned", "  []\n");

        let mut config = crate::config::Config::default();
        config.paths.specs_root = specs_root.display().to_string();
        let server = crate::serve::KasmosServer::new(config).expect("server");

        let err = handle(
            TransitionWpInput {
                feature_slug: "011-beta".to_string(),
                wp_id: "WP01".to_string(),
                to_state: TransitionState::Done,
                actor: "manager".to_string(),
                reason: None,
            },
            &server,
        )
        .await
        .expect_err("must fail");

        assert!(err.to_string().contains("TRANSITION_NOT_ALLOWED"));
    }

    #[tokio::test]
    async fn rework_respects_rejection_cap() {
        let tmp = tempdir().expect("tempdir");
        let specs_root = tmp.path().join("kitty-specs");
        let feature_dir = specs_root.join("011-gamma");
        std::fs::create_dir_all(feature_dir.join("tasks")).expect("mkdir tasks");
        let history =
            "  - lane: for_review\n  - lane: doing\n  - lane: for_review\n  - lane: doing\n";
        write_wp(&feature_dir, "WP01", "for_review", history);

        let mut config = crate::config::Config::default();
        config.paths.specs_root = specs_root.display().to_string();
        config.agent.review_rejection_cap = 2;
        let server = crate::serve::KasmosServer::new(config).expect("server");

        let err = handle(
            TransitionWpInput {
                feature_slug: "011-gamma".to_string(),
                wp_id: "WP01".to_string(),
                to_state: TransitionState::Rework,
                actor: "reviewer".to_string(),
                reason: Some("fix issues".to_string()),
            },
            &server,
        )
        .await
        .expect_err("must fail");

        assert!(err.to_string().contains("review rejection cap reached"));
    }

    #[test]
    fn lane_translation_matches_protocol() {
        assert_eq!(kasmos_state_to_lane(TransitionState::Pending), "planned");
        assert_eq!(kasmos_state_to_lane(TransitionState::Active), "doing");
        assert_eq!(
            kasmos_state_to_lane(TransitionState::ForReview),
            "for_review"
        );
        assert_eq!(kasmos_state_to_lane(TransitionState::Done), "done");
        assert_eq!(kasmos_state_to_lane(TransitionState::Rework), "doing");
    }

    #[test]
    fn rejection_count_counts_for_review_to_doing_cycles() {
        let content = "---\nwork_package_id: WP01\nlane: for_review\nhistory:\n  - lane: planned\n  - lane: doing\n  - lane: for_review\n  - lane: doing\n---\n\n# body\n";
        assert_eq!(rejection_count(content), 1);
    }
}
