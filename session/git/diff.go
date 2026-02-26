package git

import (
	"fmt"
	"os"
	"strings"
)

// DiffStats holds statistics about the changes in a diff
type DiffStats struct {
	// Content is the full diff content
	Content string
	// Added is the number of added lines
	Added int
	// Removed is the number of removed lines
	Removed int
	// Error holds any error that occurred during diff computation
	// This allows propagating setup errors (like missing base commit) without breaking the flow
	Error error
}

func (d *DiffStats) IsEmpty() bool {
	return d.Added == 0 && d.Removed == 0 && d.Content == ""
}

// Diff returns the git diff between the worktree and the base branch along with statistics
func (g *GitWorktree) Diff() *DiffStats {
	stats := &DiffStats{}

	// Bail out early if the worktree directory no longer exists on disk
	// (e.g. cleaned up externally or after pause). Avoids spamming git
	// errors every tick.
	if _, err := os.Stat(g.worktreePath); err != nil {
		stats.Error = fmt.Errorf("worktree path gone: %w", err)
		return stats
	}

	base := g.GetBaseCommitSHA()
	if base == "" {
		stats.Error = fmt.Errorf("no base commit SHA available")
		return stats
	}

	// Diff tracked changes (read-only, does not touch the index).
	content, err := g.runGitCommand(g.worktreePath, "--no-pager", "diff", base)
	if err != nil {
		stats.Error = err
		return stats
	}
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
			stats.Added++
		} else if strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---") {
			stats.Removed++
		}
	}
	stats.Content = content

	return stats
}
