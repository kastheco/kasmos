package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"

	"github.com/kastheco/kasmos/config"
	"github.com/kastheco/kasmos/internal/initcmd/harness"
	"github.com/kastheco/kasmos/internal/initcmd/scaffold"
	"github.com/kastheco/kasmos/internal/initcmd/scaffoldsync"
	"github.com/spf13/cobra"
)

func newScaffoldCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "scaffold",
		Short: "Manage project scaffold files",
	}
	cmd.AddCommand(newScaffoldSyncCmd())
	cmd.AddCommand(newScaffoldWorktreeCmd())
	return cmd
}

func newScaffoldSyncCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Re-sync embedded skills and agent prompt templates from the current binary",
		Long: `Re-syncs embedded skills, agent prompt templates, harness symlinks, and
enforcement hooks from the current binary. Uses existing TOML config for agent
settings — does not re-run the interactive wizard or modify config.`,
		SilenceUsage: true,
		RunE:         runScaffoldSync,
	}
	cmd.Flags().Bool("worktrees", false, "also sync every existing worktree under the repo's .worktrees directory")
	cmd.Flags().Bool("trust", false, "Add the current project to ~/.codex/config.toml as a trusted project")
	return cmd
}

func newScaffoldWorktreeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "worktree [path]",
		Short:        "Re-sync scaffold files into one existing worktree",
		Long:         "Repairs harness scaffold files inside an existing worktree using the current repo's configured agent profiles.",
		Args:         cobra.MaximumNArgs(1),
		SilenceUsage: true,
		RunE:         runScaffoldWorktree,
	}
	cmd.Flags().Bool("trust", false, "Add the worktree to ~/.codex/config.toml as a trusted project")
	return cmd
}

// runScaffoldSync is a thin cobra wrapper: it resolves the checkout root,
// gathers flags, and delegates to scaffoldsync.Run.
func runScaffoldSync(cmd *cobra.Command, args []string) error {
	includeWorktrees, _ := cmd.Flags().GetBool("worktrees")
	trustProject, _ := cmd.Flags().GetBool("trust")

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}
	// Resolve to the nearest checkout root (the directory that contains the .git
	// file or directory). For a normal repo this is the repo root; for a git
	// worktree this is the worktree root — NOT the main repo root. This matches
	// kas setup (initcmd.go) which also scaffolds into os.Getwd() at the checkout
	// root, and avoids scattering scaffold files into subdirectories when the user
	// invokes the command from below the root.
	projectDir := cwd
	if root, rErr := resolveCheckoutRoot(cwd); rErr == nil {
		projectDir = root
	}

	return scaffoldsync.Run(scaffoldsync.Options{
		RepoRoot:         projectDir,
		IncludeWorktrees: includeWorktrees,
		Trust:            trustProject,
		Out:              cmd.OutOrStdout(),
	})
}

// syncScaffoldTarget calls scaffold.SyncScaffold and prints a labelled summary.
// Kept here for use by runScaffoldWorktree and cmd-level tests.
func syncScaffoldTarget(out io.Writer, label, dir string, agents []harness.AgentConfig) error {
	fmt.Fprintf(out, "%s: %s\n", label, dir)
	results, err := scaffold.SyncScaffold(dir, agents)
	if err != nil {
		return fmt.Errorf("sync scaffold: %w", err)
	}

	updated := 0
	unchanged := 0
	for _, r := range results {
		if r.Created {
			fmt.Fprintf(out, "  %-40s updated\n", r.Path)
			updated++
		} else {
			unchanged++
		}
	}
	fmt.Fprintf(out, "\ndone. %d files updated, %d unchanged.\n", updated, unchanged)
	return nil
}

// loadCWDAgentConfigs loads the agent configurations from the project-local
// config.toml resolved from the current working directory. Delegates to
// scaffoldsync.ProjectAgentConfigs after obtaining the repo root via GetConfigDir.
func loadCWDAgentConfigs() ([]harness.AgentConfig, error) {
	configDir, err := config.GetConfigDir()
	if err != nil {
		return nil, err
	}
	// GetConfigDir returns <repoRoot>/.kasmos; its parent is the repo root.
	repoRoot := filepath.Dir(configDir)
	return scaffoldsync.ProjectAgentConfigs(repoRoot)
}

// resolveCheckoutRoot walks up from dir until it finds a directory containing a
// .git file or directory and returns that directory. It stops at the first .git
// entry found, so for a git worktree it returns the worktree root (the directory
// with the .git file), not the main repo root. Falls back to dir on failure.
func resolveCheckoutRoot(dir string) (string, error) {
	for {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("not a git repository (or any parent of %s)", dir)
		}
		dir = parent
	}
}

func agentsContainHarness(agents []harness.AgentConfig, want string) bool {
	for _, a := range agents {
		if a.Harness == want {
			return true
		}
	}
	return false
}

func listExistingWorktrees(repoRoot string) ([]string, error) {
	worktreesDir := filepath.Join(repoRoot, ".worktrees")
	entries, err := os.ReadDir(worktreesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read worktrees dir: %w", err)
	}

	worktrees := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		path := filepath.Join(worktreesDir, entry.Name())
		if _, err := os.Stat(filepath.Join(path, ".git")); err != nil {
			continue
		}
		worktrees = append(worktrees, path)
	}
	sort.Strings(worktrees)
	return worktrees, nil
}

func runScaffoldWorktree(cmd *cobra.Command, args []string) error {
	out := cmd.OutOrStdout()
	agents, err := loadCWDAgentConfigs()
	if err != nil {
		return err
	}

	target := "."
	if len(args) == 1 {
		target = args[0]
	}
	absTarget, err := filepath.Abs(target)
	if err != nil {
		return fmt.Errorf("resolve worktree path: %w", err)
	}
	worktreeDir, err := resolveCheckoutRoot(absTarget)
	if err != nil {
		return fmt.Errorf("resolve worktree root: %w", err)
	}

	if _, err := os.Stat(filepath.Join(worktreeDir, ".git")); err != nil {
		return fmt.Errorf("worktree root missing .git entry: %s", worktreeDir)
	}

	if err := syncScaffoldTarget(out, "Syncing worktree scaffold", worktreeDir, agents); err != nil {
		return err
	}

	trustProject, _ := cmd.Flags().GetBool("trust")
	if trustProject && agentsContainHarness(agents, "codex") {
		home, homeErr := os.UserHomeDir()
		if homeErr != nil {
			fmt.Fprintf(out, "\nWARNING: could not get home dir: %v — skipping codex trust\n", homeErr)
			return nil
		}
		fmt.Fprintln(out, "\nTrusting worktree for codex...")
		result, err := scaffold.EnsureCodexTrustedProjectEntry(home, worktreeDir)
		if err != nil {
			return fmt.Errorf("codex trust: %w", err)
		}
		status := "OK"
		if !result.Created {
			status = "SKIP (exists)"
		}
		fmt.Fprintf(out, "  %-40s %s\n", result.Path, status)
	}

	return nil
}

func init() {
	rootCmd.AddCommand(newScaffoldCmd())
}
