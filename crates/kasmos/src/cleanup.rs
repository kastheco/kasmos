//! Cleanup of transient orchestration artifacts.

use std::path::Path;
use tracing;

/// Files/directories to REMOVE during cleanup (transient artifacts).
const TRANSIENT_ARTIFACTS: &[&str] = &["layout.kdl", "cmd.pipe", "prompts", "scripts"];

/// Files to PRESERVE during cleanup (persistent state).
const PRESERVED_ARTIFACTS: &[&str] = &["state.json", "report.md", "run.lock"];

/// Removes transient artifacts from the kasmos directory while preserving state files.
///
/// Transient (removed): layout.kdl, cmd.pipe, prompts/, scripts/
/// Preserved (kept): state.json, report.md, run.lock
///
/// This function is idempotent — safe to call multiple times.
pub fn cleanup_artifacts(kasmos_dir: &Path) {
    // Guard: directory doesn't exist — nothing to clean
    if !kasmos_dir.exists() {
        tracing::debug!(path = %kasmos_dir.display(), "Kasmos dir does not exist, nothing to clean");
        return;
    }

    for artifact in TRANSIENT_ARTIFACTS {
        let path = kasmos_dir.join(artifact);

        if !path.exists() {
            continue;
        }

        if path.is_dir() {
            match std::fs::remove_dir_all(&path) {
                Ok(()) => tracing::info!(path = %path.display(), "Removed directory"),
                Err(e) => {
                    tracing::warn!(path = %path.display(), error = %e, "Failed to remove directory")
                }
            }
        } else {
            match std::fs::remove_file(&path) {
                Ok(()) => tracing::info!(path = %path.display(), "Removed file"),
                Err(e) => {
                    tracing::warn!(path = %path.display(), error = %e, "Failed to remove file")
                }
            }
        }
    }

    tracing::info!(path = %kasmos_dir.display(), "Artifact cleanup complete");
}

/// Returns list of artifact names that would be preserved.
pub fn preserved_artifacts() -> &'static [&'static str] {
    PRESERVED_ARTIFACTS
}

/// Returns list of artifact names that would be removed.
pub fn transient_artifacts() -> &'static [&'static str] {
    TRANSIENT_ARTIFACTS
}

#[cfg(test)]
mod tests {
    use super::*;
    use tempfile::TempDir;

    #[test]
    fn test_cleanup_removes_transient_artifacts() {
        let temp_dir = TempDir::new().expect("tmp");
        let kasmos = temp_dir.path();

        // Create transient artifacts
        std::fs::write(kasmos.join("layout.kdl"), "layout {}").expect("write");
        std::fs::write(kasmos.join("cmd.pipe"), "").expect("write");
        std::fs::create_dir(kasmos.join("prompts")).expect("dir");
        std::fs::write(kasmos.join("prompts/WP01.md"), "prompt").expect("write");
        std::fs::create_dir(kasmos.join("scripts")).expect("dir");
        std::fs::write(kasmos.join("scripts/setup.sh"), "#!/bin/sh").expect("write");

        cleanup_artifacts(kasmos);

        assert!(!kasmos.join("layout.kdl").exists());
        assert!(!kasmos.join("cmd.pipe").exists());
        assert!(!kasmos.join("prompts").exists());
        assert!(!kasmos.join("scripts").exists());
    }

    #[test]
    fn test_cleanup_preserves_state_files() {
        let temp_dir = TempDir::new().expect("tmp");
        let kasmos = temp_dir.path();

        // Create preserved artifacts
        std::fs::write(kasmos.join("state.json"), "{}").expect("write");
        std::fs::write(kasmos.join("report.md"), "# Report").expect("write");
        std::fs::write(kasmos.join("run.lock"), "12345").expect("write");

        // Also create a transient one
        std::fs::write(kasmos.join("layout.kdl"), "layout {}").expect("write");

        cleanup_artifacts(kasmos);

        // Preserved files should still exist
        assert!(kasmos.join("state.json").exists(), "state.json preserved");
        assert!(kasmos.join("report.md").exists(), "report.md preserved");
        assert!(kasmos.join("run.lock").exists(), "run.lock preserved");

        // Transient should be gone
        assert!(!kasmos.join("layout.kdl").exists());
    }

    #[test]
    fn test_cleanup_idempotent() {
        let temp_dir = TempDir::new().expect("tmp");
        let kasmos = temp_dir.path();

        std::fs::write(kasmos.join("layout.kdl"), "layout {}").expect("write");

        // First cleanup
        cleanup_artifacts(kasmos);
        assert!(!kasmos.join("layout.kdl").exists());

        // Second cleanup should not panic or error
        cleanup_artifacts(kasmos);
    }

    #[test]
    fn test_cleanup_missing_dir() {
        // Should not panic when directory doesn't exist
        cleanup_artifacts(Path::new("/nonexistent/path/kasmos"));
    }

    #[test]
    fn test_cleanup_empty_dir() {
        let temp_dir = TempDir::new().expect("tmp");
        // No artifacts to clean — should complete without error
        cleanup_artifacts(temp_dir.path());
    }

    #[test]
    fn test_artifact_lists() {
        assert!(transient_artifacts().contains(&"layout.kdl"));
        assert!(transient_artifacts().contains(&"cmd.pipe"));
        assert!(preserved_artifacts().contains(&"state.json"));
        assert!(preserved_artifacts().contains(&"report.md"));
    }
}
