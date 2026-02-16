//! Git worktree management for kasmos.
//!
//! Each work package gets its own git worktree on a dedicated branch
//! (`feat/{feature}/{wp_id}`), providing isolation so agents don't
//! interfere with each other.  When a wave N+1 WP depends on wave N
//! results, its worktree is created from the merged state of its
//! dependencies.

use std::path::{Path, PathBuf};
use std::process::Command;

use anyhow::{Context, bail};
use tracing::{debug, info, warn};

/// Git-related errors.
#[derive(thiserror::Error, Debug)]
pub enum GitError {
    /// Git binary not found.
    #[error("Git binary not found in PATH")]
    NotFound,

    /// Not inside a git repository.
    #[error("No git repository found at or above {path}")]
    NoRepo { path: String },

    /// Worktree creation failed.
    #[error("Failed to create worktree for {wp_id}: {reason}")]
    WorktreeCreation { wp_id: String, reason: String },

    /// Branch operation failed.
    #[error("Branch operation failed: {0}")]
    BranchError(String),

    /// Git command failed.
    #[error("Git command failed: {0}")]
    CommandFailed(String),
}

/// Manages git worktrees for kasmos work packages.
pub struct WorktreeManager {
    /// Path to the git binary.
    git_binary: String,
    /// Root of the git repository (contains `.git`).
    repo_root: PathBuf,
    /// Directory where worktrees are created.
    worktrees_dir: PathBuf,
    /// Feature name, used for branch naming.
    feature_name: String,
}

impl WorktreeManager {
    /// Create a new WorktreeManager.
    ///
    /// Discovers the git repo root by walking up from `feature_dir`, and
    /// sets up the worktree directory at `{repo_root}/.worktrees`.
    ///
    /// # Errors
    /// Returns an error if `feature_dir` is not inside a git repository.
    pub fn new(feature_dir: &Path, feature_name: &str) -> anyhow::Result<Self> {
        let git_binary = "git".to_string();
        validate_git_binary(&git_binary)?;

        let repo_root = find_repo_root(feature_dir)
            .with_context(|| format!("Cannot find git repo from {}", feature_dir.display()))?;

        let worktrees_dir = repo_root.join(".worktrees");

        info!(
            repo_root = %repo_root.display(),
            worktrees_dir = %worktrees_dir.display(),
            "Git worktree manager initialized"
        );

        Ok(Self {
            git_binary,
            repo_root,
            worktrees_dir,
            feature_name: feature_name.to_string(),
        })
    }

    /// The directory where worktrees are created.
    pub fn worktrees_dir(&self) -> &Path {
        &self.worktrees_dir
    }

    /// The repository root.
    pub fn repo_root(&self) -> &Path {
        &self.repo_root
    }

    /// Find an existing worktree for the given work package.
    ///
    /// Looks for a spec-kitty-created worktree at
    /// `{worktrees_dir}/{feature_name}-{wp_id}`. Returns `Some(path)` if
    /// a valid worktree exists, `None` otherwise.
    ///
    /// This is the preferred method — spec-kitty creates worktrees via
    /// `spec-kitty implement WPxx`, so kasmos should locate rather than create.
    pub fn find_worktree(&self, wp_id: &str) -> Option<PathBuf> {
        let worktree_path = self
            .worktrees_dir
            .join(format!("{}-{}", self.feature_name, wp_id));

        if worktree_path.exists() && worktree_path.join(".git").exists() {
            debug!(
                wp_id = %wp_id,
                path = %worktree_path.display(),
                "Found existing worktree"
            );
            Some(worktree_path)
        } else {
            debug!(
                wp_id = %wp_id,
                path = %worktree_path.display(),
                "No worktree found"
            );
            None
        }
    }

    /// Ensure a worktree exists for the given work package.
    ///
    /// Creates `{worktrees_dir}/{feature_name}-{wp_id}` on branch
    /// `{feature_name}-{wp_id}` if it doesn't already exist.
    ///
    /// Prefers finding an existing spec-kitty worktree first. Only creates
    /// a new one if none exists (fallback for manual runs without spec-kitty).
    ///
    /// The worktree is created from `base_ref` (typically HEAD or a
    /// dependency branch).
    ///
    /// # Returns
    /// The path to the worktree directory.
    pub fn ensure_worktree(&self, wp_id: &str, base_ref: &str) -> anyhow::Result<PathBuf> {
        // Prefer existing worktree (created by spec-kitty)
        if let Some(path) = self.find_worktree(wp_id) {
            return Ok(path);
        }

        let worktree_path = self
            .worktrees_dir
            .join(format!("{}-{}", self.feature_name, wp_id));
        let branch_name = self.branch_name(wp_id);

        // Directory exists but isn't a valid worktree — remove and recreate
        if worktree_path.exists() {
            warn!(
                wp_id = %wp_id,
                path = %worktree_path.display(),
                "Invalid worktree directory found, removing"
            );
            std::fs::remove_dir_all(&worktree_path).with_context(|| {
                format!(
                    "Failed to remove invalid worktree at {}",
                    worktree_path.display()
                )
            })?;
        }

        // Ensure worktrees directory exists
        std::fs::create_dir_all(&self.worktrees_dir).with_context(|| {
            format!(
                "Failed to create worktrees dir: {}",
                self.worktrees_dir.display()
            )
        })?;

        // Check if the branch already exists
        let branch_exists = self.branch_exists(&branch_name)?;

        if branch_exists {
            // Branch exists (maybe from a previous run) — create worktree from it
            info!(
                wp_id = %wp_id,
                branch = %branch_name,
                "Reusing existing branch for worktree"
            );
            self.run_git(&[
                "worktree",
                "add",
                &worktree_path.display().to_string(),
                &branch_name,
            ])
            .with_context(|| {
                format!("Failed to create worktree for {} on existing branch", wp_id)
            })?;
        } else {
            // Create new branch from base_ref
            info!(
                wp_id = %wp_id,
                branch = %branch_name,
                base_ref = %base_ref,
                "Creating worktree with new branch"
            );
            self.run_git(&[
                "worktree",
                "add",
                "-b",
                &branch_name,
                &worktree_path.display().to_string(),
                base_ref,
            ])
            .with_context(|| {
                format!("Failed to create worktree for {} from {}", wp_id, base_ref)
            })?;
        }

        info!(
            wp_id = %wp_id,
            path = %worktree_path.display(),
            branch = %branch_name,
            "Worktree ready"
        );

        Ok(worktree_path)
    }

    /// Get the branch name for a work package.
    ///
    /// Uses spec-kitty convention: `{feature_name}-{wp_id}`.
    pub fn branch_name(&self, wp_id: &str) -> String {
        format!("{}-{}", self.feature_name, wp_id)
    }

    /// Get the worktree path for a work package.
    pub fn worktree_path(&self, wp_id: &str) -> PathBuf {
        self.worktrees_dir
            .join(format!("{}-{}", self.feature_name, wp_id))
    }

    /// Check if a branch exists (local).
    fn branch_exists(&self, branch_name: &str) -> anyhow::Result<bool> {
        let output = Command::new(&self.git_binary)
            .args([
                "rev-parse",
                "--verify",
                &format!("refs/heads/{}", branch_name),
            ])
            .current_dir(&self.repo_root)
            .output()
            .context("Failed to run git rev-parse")?;

        Ok(output.status.success())
    }

    /// Get the current HEAD ref name (branch name or SHA).
    pub fn current_ref(&self) -> anyhow::Result<String> {
        let output = Command::new(&self.git_binary)
            .args(["symbolic-ref", "--short", "HEAD"])
            .current_dir(&self.repo_root)
            .output()
            .context("Failed to get current ref")?;

        if output.status.success() {
            Ok(String::from_utf8_lossy(&output.stdout).trim().to_string())
        } else {
            // Detached HEAD — return SHA
            let output = Command::new(&self.git_binary)
                .args(["rev-parse", "HEAD"])
                .current_dir(&self.repo_root)
                .output()
                .context("Failed to get HEAD SHA")?;
            Ok(String::from_utf8_lossy(&output.stdout).trim().to_string())
        }
    }

    /// Run a git command in the repo root, returning stdout on success.
    fn run_git(&self, args: &[&str]) -> anyhow::Result<String> {
        debug!(args = ?args, "Running git command");

        let output = Command::new(&self.git_binary)
            .args(args)
            .current_dir(&self.repo_root)
            .output()
            .with_context(|| format!("Failed to execute: git {}", args.join(" ")))?;

        if output.status.success() {
            Ok(String::from_utf8_lossy(&output.stdout).trim().to_string())
        } else {
            let stderr = String::from_utf8_lossy(&output.stderr).trim().to_string();
            bail!("git {} failed: {}", args.join(" "), stderr);
        }
    }

    /// Remove a worktree (for cleanup).
    pub fn remove_worktree(&self, wp_id: &str) -> anyhow::Result<()> {
        let worktree_path = self.worktree_path(wp_id);

        if !worktree_path.exists() {
            return Ok(());
        }

        self.run_git(&[
            "worktree",
            "remove",
            "--force",
            &worktree_path.display().to_string(),
        ])
        .with_context(|| format!("Failed to remove worktree for {}", wp_id))?;

        info!(wp_id = %wp_id, "Worktree removed");
        Ok(())
    }

    /// Prune stale worktree references.
    pub fn prune(&self) -> anyhow::Result<()> {
        self.run_git(&["worktree", "prune"])
            .context("Failed to prune worktrees")?;
        debug!("Pruned stale worktree references");
        Ok(())
    }
}

/// Find the git repository root by walking up from the given path.
///
/// Looks for a `.git` directory (or file, for worktrees) at each ancestor.
pub fn find_repo_root(start: &Path) -> anyhow::Result<PathBuf> {
    let mut current = start.to_path_buf();

    // Canonicalize to resolve symlinks
    if current.exists() {
        current = current.canonicalize().unwrap_or_else(|_| current.clone());
    }

    loop {
        if current.join(".git").exists() {
            return Ok(current);
        }
        if !current.pop() {
            bail!("No git repository found at or above {}", start.display());
        }
    }
}

/// Validate that the git binary is available in PATH.
pub fn validate_git_binary(binary: &str) -> anyhow::Result<()> {
    let output = Command::new(binary)
        .arg("--version")
        .output()
        .with_context(|| format!("Git binary '{}' not found in PATH", binary))?;

    if !output.status.success() {
        bail!("Git binary '{}' is not functional", binary);
    }

    let version = String::from_utf8_lossy(&output.stdout);
    debug!("Git version: {}", version.trim());
    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_find_repo_root_from_cwd() {
        // This test assumes it's running inside a git repo (the kasmos repo itself)
        let cwd = std::env::current_dir().expect("cwd");
        let result = find_repo_root(&cwd);
        assert!(result.is_ok(), "Should find repo root from cwd");
        let root = result.unwrap();
        assert!(root.join(".git").exists(), "Repo root should have .git");
    }

    #[test]
    fn test_find_repo_root_not_a_repo() {
        let result = find_repo_root(Path::new("/tmp"));
        assert!(result.is_err(), "Should fail for /tmp (not a git repo)");
    }

    #[test]
    fn test_validate_git_binary() {
        let result = validate_git_binary("git");
        assert!(result.is_ok(), "git should be in PATH");
    }

    #[test]
    fn test_validate_git_binary_missing() {
        let result = validate_git_binary("nonexistent-git-binary-xyz");
        assert!(result.is_err());
    }

    #[test]
    fn test_worktree_manager_new() {
        // Create manager from the kasmos repo itself
        let cwd = std::env::current_dir().expect("cwd");
        let repo_root = find_repo_root(&cwd).expect("repo root");

        let manager = WorktreeManager::new(&cwd, "test-feature");
        assert!(manager.is_ok());

        let mgr = manager.unwrap();
        assert_eq!(mgr.repo_root(), repo_root);
        assert!(mgr.worktrees_dir().ends_with(".worktrees"));
    }

    #[test]
    fn test_branch_name_format() {
        let cwd = std::env::current_dir().expect("cwd");
        let manager = WorktreeManager::new(&cwd, "002-tui-controller").unwrap();
        assert_eq!(manager.branch_name("WP01"), "002-tui-controller-WP01");
    }

    #[test]
    fn test_worktree_path_format() {
        let cwd = std::env::current_dir().expect("cwd");
        let manager = WorktreeManager::new(&cwd, "002-tui-controller").unwrap();
        let path = manager.worktree_path("WP01");
        assert!(
            path.ends_with("002-tui-controller-WP01"),
            "Path should end with feature-WP: {:?}",
            path
        );
    }

    #[test]
    fn test_current_ref() {
        let cwd = std::env::current_dir().expect("cwd");
        let manager = WorktreeManager::new(&cwd, "test").unwrap();
        let ref_name = manager.current_ref();
        assert!(ref_name.is_ok(), "Should get current ref");
        let name = ref_name.unwrap();
        assert!(!name.is_empty(), "Ref name should not be empty");
    }

    #[test]
    fn test_ensure_worktree_and_cleanup() {
        // Integration test: create a worktree, verify it, then clean up
        let cwd = std::env::current_dir().expect("cwd");
        let manager = WorktreeManager::new(&cwd, "test-kasmos-wt").unwrap();

        let wp_id = "WP-TEST-01";
        let base_ref = "HEAD";

        // Create worktree
        let path = manager.ensure_worktree(wp_id, base_ref);
        assert!(path.is_ok(), "Should create worktree: {:?}", path.err());
        let path = path.unwrap();
        assert!(path.exists(), "Worktree directory should exist");
        assert!(
            path.join(".git").exists(),
            "Worktree should have .git marker"
        );

        // Idempotent: calling again should succeed
        let path2 = manager.ensure_worktree(wp_id, base_ref);
        assert!(path2.is_ok(), "Should be idempotent");
        assert_eq!(path, path2.unwrap());

        // Clean up
        let remove = manager.remove_worktree(wp_id);
        assert!(remove.is_ok(), "Should remove worktree: {:?}", remove.err());
        assert!(!path.exists(), "Worktree directory should be gone");

        // Also clean up the branch
        let branch = manager.branch_name(wp_id);
        let _ = Command::new("git")
            .args(["branch", "-D", &branch])
            .current_dir(manager.repo_root())
            .output();

        // Prune
        let _ = manager.prune();
    }
}
