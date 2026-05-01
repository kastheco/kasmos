package check

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// HookStatus describes the pre-push hook installation state for a kasmos clone.
type HookStatus struct {
	Skipped        bool   // true when cwd is not a kasmos clone (no docs-drift-map.yml)
	Configured     bool   // core.hooksPath == ExpectedPath AND HookFileExists
	ExpectedPath   string // always "scripts/git-hooks"
	ActualPath     string // raw value of core.hooksPath ("" when unset)
	HookFileExists bool   // scripts/git-hooks/pre-push exists in repoRoot
}

// gitConfigFn is the seam for reading core.hooksPath. Replaced in tests.
var gitConfigFn = func(repoRoot string) (string, error) {
	cmd := exec.Command("git", "-C", repoRoot, "config", "--get", "core.hooksPath")
	out, err := cmd.Output()
	if err != nil {
		// `git config --get` exits 1 when the key is unset; that is not an error.
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return "", nil
		}
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// SetGitConfigFnForTest replaces the git-config seam with fn and returns the
// previous value so callers can restore it in t.Cleanup. For use in tests only.
func SetGitConfigFnForTest(fn func(string) (string, error)) func(string) (string, error) {
	prev := gitConfigFn
	gitConfigFn = fn
	return prev
}

// CheckPrePushHook inspects repoRoot to determine whether the docs-drift
// pre-push hook is installed and active. Returns Skipped=true when repoRoot
// is not a kasmos clone (used as the gating signal so non-kasmos cwds don't
// register a spurious unhealthy item).
func CheckPrePushHook(repoRoot string) HookStatus {
	status := HookStatus{ExpectedPath: "scripts/git-hooks"}

	// Heuristic: kasmos clones contain docs/docs-drift-map.yml.
	if _, err := os.Stat(filepath.Join(repoRoot, "docs", "docs-drift-map.yml")); err != nil {
		status.Skipped = true
		return status
	}

	if _, err := os.Stat(filepath.Join(repoRoot, "scripts", "git-hooks", "pre-push")); err == nil {
		status.HookFileExists = true
	}

	actual, err := gitConfigFn(repoRoot)
	if err != nil {
		// Git unavailable or repoRoot not a git work tree - skip silently.
		status.Skipped = true
		return status
	}
	status.ActualPath = actual
	status.Configured = (actual == status.ExpectedPath) && status.HookFileExists
	return status
}
