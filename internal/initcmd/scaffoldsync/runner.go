// Package scaffoldsync implements the full "kas scaffold sync" orchestration path
// as a reusable library. The package exposes Run and ProjectAgentConfigs so that
// HTTP handlers and other callers can trigger a scaffold sync without going through
// the cobra CLI.
package scaffoldsync

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"

	"github.com/kastheco/kasmos/config"
	"github.com/kastheco/kasmos/internal/initcmd/harness"
	"github.com/kastheco/kasmos/internal/initcmd/scaffold"
)

// Options configures a scaffold-sync run.
type Options struct {
	// RepoRoot is the main checkout root to sync (the directory that contains .git).
	// For a normal repo this is the repo root; for a git worktree the worktree root.
	RepoRoot string
	// IncludeWorktrees, when true, also syncs every existing worktree under
	// <main-repo>/.worktrees.
	IncludeWorktrees bool
	// Trust, when true, adds the project to ~/.codex/config.toml as a trusted
	// project (non-fatal when the home directory cannot be determined).
	Trust bool
	// HomeDir overrides os.UserHomeDir() for global skill sync and codex trust.
	// Useful in tests to avoid touching the real home directory. When empty,
	// os.UserHomeDir() is used.
	HomeDir string
	// Out receives human-readable progress output. Defaults to io.Discard when nil.
	Out io.Writer
}

// Run performs the full scaffold-sync operation: syncs scaffold files for the
// main checkout (and optionally sibling worktrees), refreshes global personal
// skills, trusts the project for codex when requested, and configures enforcement
// hooks according to the project's config.toml.
func Run(opts Options) error {
	out := opts.Out
	if out == nil {
		out = io.Discard
	}

	agents, err := ProjectAgentConfigs(opts.RepoRoot)
	if err != nil {
		return err
	}

	tomlCfgPath := filepath.Join(opts.RepoRoot, ".kasmos", config.TOMLConfigFileName)
	tomlCfg, err := config.LoadTOMLConfigFrom(tomlCfgPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	if err := syncTarget(out, "Syncing scaffold", opts.RepoRoot, agents); err != nil {
		return err
	}

	if opts.IncludeWorktrees {
		repoRoot, err := config.ResolveRepoRoot(opts.RepoRoot)
		if err != nil {
			return fmt.Errorf("resolve repo root for worktrees: %w", err)
		}
		worktrees, err := listExistingWorktrees(repoRoot)
		if err != nil {
			return fmt.Errorf("list worktrees: %w", err)
		}
		for _, worktreeDir := range worktrees {
			if filepath.Clean(worktreeDir) == filepath.Clean(opts.RepoRoot) {
				continue
			}
			if err := syncTarget(out, "Syncing worktree scaffold", worktreeDir, agents); err != nil {
				return err
			}
		}
	}

	// Sync global skills to harness directories.
	registry := harness.NewRegistry()
	home, homeErr := resolveHomeDir(opts.HomeDir)
	if homeErr != nil {
		fmt.Fprintf(out, "\nWARNING: could not get home dir: %v — skipping global skill sync\n", homeErr)
	} else {
		fmt.Fprintln(out, "\nSyncing personal skills...")
		for _, name := range registry.All() {
			h := registry.Get(name)
			if _, found := h.Detect(); !found {
				fmt.Fprintf(out, "  %-12s SKIP (not installed)\n", name)
				continue
			}
			fmt.Fprintf(out, "  %-12s ", name)
			if err := harness.SyncGlobalSkills(home, name); err != nil {
				fmt.Fprintf(out, "FAILED: %v\n", err)
			} else {
				fmt.Fprintln(out, "OK")
			}
		}
	}

	// Trust project for codex when requested.
	if opts.Trust && agentsContainHarness(agents, "codex") {
		home, homeErr := resolveHomeDir(opts.HomeDir)
		if homeErr != nil {
			fmt.Fprintf(out, "\nWARNING: could not get home dir: %v — skipping codex trust\n", homeErr)
		} else {
			fmt.Fprintln(out, "\nTrusting project for codex...")
			result, err := scaffold.EnsureCodexTrustedProjectEntry(home, opts.RepoRoot)
			if err != nil {
				return fmt.Errorf("codex trust: %w", err)
			}
			status := "OK"
			if !result.Created {
				status = "SKIP (exists)"
			}
			fmt.Fprintf(out, "  %-40s %s\n", result.Path, status)
		}
	}

	// Install or uninstall enforcement hooks for each detected harness.
	fmt.Fprintln(out, "\nConfiguring enforcement hooks...")
	for _, name := range registry.All() {
		h := registry.Get(name)
		if _, found := h.Detect(); !found {
			fmt.Fprintf(out, "  %-12s SKIP (not installed)\n", name)
			continue
		}
		fmt.Fprintf(out, "  %-12s ", name)
		if config.IsEnforcementEnabled(tomlCfg.Enforcement, name) {
			if err := h.InstallEnforcement(); err != nil {
				fmt.Fprintf(out, "FAILED: %v\n", err)
			} else {
				fmt.Fprintln(out, "OK")
			}
		} else {
			if err := h.UninstallEnforcement(); err != nil {
				fmt.Fprintf(out, "FAILED: %v\n", err)
			} else {
				fmt.Fprintln(out, "REMOVED (enforcement disabled)")
			}
		}
	}

	return nil
}

// ProjectAgentConfigs loads and returns the enabled agent configurations from
// <repoRoot>/.kasmos/config.toml. It preserves the user-facing error strings
// expected by callers: "no config found" when the file is absent and
// "config has no enabled agents" when all profiles are disabled.
func ProjectAgentConfigs(repoRoot string) ([]harness.AgentConfig, error) {
	path := filepath.Join(repoRoot, ".kasmos", config.TOMLConfigFileName)
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("no config found — run 'kas setup' first to create .kasmos/config.toml")
		}
		return nil, fmt.Errorf("stat config: %w", err)
	}
	tomlCfg, err := config.LoadTOMLConfigFrom(path)
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}
	agents := profilesToAgentConfigs(tomlCfg.Profiles)
	if len(agents) == 0 {
		return nil, fmt.Errorf("config has no enabled agents — run 'kas setup' to configure agents")
	}
	return agents, nil
}

// profilesToAgentConfigs converts a map of AgentProfile (keyed by role name)
// to a deterministic slice of harness.AgentConfig, including only enabled profiles.
//
// The "chat" role is special: it fans out to every harness program present among
// ALL non-chat profiles (enabled or disabled), mirroring wizard.State.ToAgentConfigs.
// If no other harnesses are present, it falls back to chat's own stored Program.
func profilesToAgentConfigs(profiles map[string]config.AgentProfile) []harness.AgentConfig {
	if len(profiles) == 0 {
		return nil
	}

	// Collect the distinct harness programs used by all non-chat profiles,
	// regardless of enabled state.
	harnessSet := map[string]struct{}{}
	for role, p := range profiles {
		if role == "chat" {
			continue
		}
		if p.Program != "" {
			harnessSet[p.Program] = struct{}{}
		}
	}

	roles := make([]string, 0, len(profiles))
	for role := range profiles {
		roles = append(roles, role)
	}
	sort.Strings(roles)

	configs := make([]harness.AgentConfig, 0, len(roles))
	for _, role := range roles {
		p := profiles[role]
		if !p.Enabled {
			continue
		}
		if role == "chat" {
			// Fan chat out to every harness present in the project.
			chatHarnesses := make([]string, 0, len(harnessSet))
			for h := range harnessSet {
				chatHarnesses = append(chatHarnesses, h)
			}
			sort.Strings(chatHarnesses)
			if len(chatHarnesses) == 0 && p.Program != "" {
				// Fallback: no other agents; use chat's own program.
				chatHarnesses = []string{p.Program}
			}
			for _, h := range chatHarnesses {
				configs = append(configs, harness.AgentConfig{
					Role:        role,
					Harness:     h,
					Model:       p.Model,
					Temperature: p.Temperature,
					Effort:      p.Effort,
					Enabled:     p.Enabled,
					ExtraFlags:  p.Flags,
				})
			}
			continue
		}
		configs = append(configs, harness.AgentConfig{
			Role:        config.ScaffoldRoleForProfile(role),
			Harness:     p.Program,
			Model:       p.Model,
			Temperature: p.Temperature,
			Effort:      p.Effort,
			Enabled:     p.Enabled,
			ExtraFlags:  p.Flags,
		})
	}
	return configs
}

// syncTarget syncs the scaffold into dir, printing a labelled header and a
// summary line to out.
func syncTarget(out io.Writer, label, dir string, agents []harness.AgentConfig) error {
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

// listExistingWorktrees returns the paths of all directories under
// <repoRoot>/.worktrees that contain a .git marker.
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

// agentsContainHarness reports whether any agent in the slice uses the named harness.
func agentsContainHarness(agents []harness.AgentConfig, want string) bool {
	for _, a := range agents {
		if a.Harness == want {
			return true
		}
	}
	return false
}

// resolveHomeDir returns override when non-empty, otherwise os.UserHomeDir().
func resolveHomeDir(override string) (string, error) {
	if override != "" {
		return override, nil
	}
	return os.UserHomeDir()
}
