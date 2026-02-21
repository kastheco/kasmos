package harness

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// cloneOrPull ensures a git repo exists at dir, cloning from url if absent,
// or pulling (ff-only, best-effort) if already present.
func cloneOrPull(dir, url string) error {
	switch _, err := os.Stat(filepath.Join(dir, ".git")); {
	case err == nil:
		// Repo exists; update best-effort (stale version is acceptable)
		cmd := exec.Command("git", "-C", dir, "pull", "--ff-only")
		if err := cmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "  warning: superpowers update failed (using cached): %v\n", err)
		}
	case os.IsNotExist(err):
		if err := os.MkdirAll(filepath.Dir(dir), 0o755); err != nil {
			return fmt.Errorf("create parent dir: %w", err)
		}
		cmd := exec.Command("git", "clone", url, dir)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("clone %s: %s: %w", url, string(out), err)
		}
	default:
		return fmt.Errorf("check repo at %s: %w", dir, err)
	}
	return nil
}
