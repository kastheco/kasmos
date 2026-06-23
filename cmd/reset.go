package cmd

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/kastheco/kasmos/config"
	initcmdharness "github.com/kastheco/kasmos/internal/initcmd/harness"
	"github.com/kastheco/kasmos/internal/initcmd/scaffold"
	"github.com/kastheco/kasmos/internal/platform"
	"github.com/spf13/cobra"
)

var (
	repoResetNow                    = time.Now
	repoResetSync                   = defaultRepoResetSync
	repoResetWriteMCP               = defaultRepoResetWriteMCP
	repoResetStopServices           = defaultRepoResetStopServices
	openPromptTTY                   = defaultOpenPromptTTY
	repoResetRestartServicesCommand = platform.RestartServicesCommand
	repoResetStopUserServices       = platform.StopServices
	repoResetStopDaemonByPID        = stopDaemonByPID
)

var managedBackupPaths = []string{
	".kasmos",
	".agents",
	".claude",
	".opencode",
	".codex",
	".worktrees",
	".mcp.json",
}

var repoResetCleanupPaths = []string{
	".agents",
	".claude",
	".opencode",
	".codex",
	".worktrees",
	filepath.Join(".kasmos", "cache"),
	filepath.Join(".kasmos", "signals"),
	".mcp.json",
}

type repoResetOptions struct {
	DryRun          bool
	IgnoreWorktrees bool
	Yes             bool
	Stdout          io.Writer
	Stderr          io.Writer
	Stdin           io.Reader
	Prompt          io.Reader
	BackupRoot      string
	Now             func() time.Time
}

// NewResetCmd returns the top-level reset command.
func NewResetCmd() *cobra.Command {
	var dryRun bool
	var ignoreWorktrees bool
	var yes bool

	cmd := &cobra.Command{
		Use:          "reset [repo-list-file]",
		Short:        "Backup and refresh kasmos scaffold state across repos",
		SilenceUsage: true,
		Args:         cobra.MaximumNArgs(1),
		Long: `Refreshes kasmos-managed scaffold state across one or more repositories.

By default it:
  - creates a timestamped backup tarball per repo
  - preserves .kasmos/config.toml and .kasmos/taskstore.db
  - removes scaffold/runtime directories such as .claude/, .opencode/, .codex/
  - re-syncs scaffold files from the current repo config
  - rewrites .mcp.json to the shared HTTP transport (http://127.0.0.1:7434/mcp)`,
		RunE: func(cmd *cobra.Command, args []string) error {
			prompt := cmd.InOrStdin()
			if !yes && !dryRun && !stdinIsTerminal(cmd.InOrStdin()) {
				tty, err := openPromptTTY()
				if err != nil {
					return fmt.Errorf("stdin is being used for repo input and no tty is available for prompts; rerun with --yes: %w", err)
				}
				defer tty.Close()
				prompt = tty
			}

			return runRepoReset(args, repoResetOptions{
				DryRun:          dryRun,
				IgnoreWorktrees: ignoreWorktrees,
				Yes:             yes,
				Stdout:          cmd.OutOrStdout(),
				Stderr:          cmd.ErrOrStderr(),
				Stdin:           cmd.InOrStdin(),
				Prompt:          prompt,
				Now:             repoResetNow,
			})
		},
	}

	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Print what would happen, but make no changes")
	cmd.Flags().BoolVar(&ignoreWorktrees, "ignore-worktrees", false, "Proceed even if .worktrees or extra git worktrees are present")
	cmd.Flags().BoolVar(&yes, "yes", false, "Skip per-repo confirmation prompts")

	return cmd
}

func runRepoReset(args []string, opts repoResetOptions) error {
	if opts.Stdout == nil {
		opts.Stdout = io.Discard
	}
	if opts.Stderr == nil {
		opts.Stderr = io.Discard
	}
	if opts.Stdin == nil {
		opts.Stdin = os.Stdin
	}
	if opts.Prompt == nil {
		opts.Prompt = opts.Stdin
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}

	backupRoot, err := resolveBackupRoot("", opts.Now)
	if err != nil {
		return err
	}
	if opts.BackupRoot != "" {
		backupRoot = opts.BackupRoot
	}

	repos, err := collectResetRepos(args, opts.Stdin)
	if err != nil {
		return err
	}
	if len(repos) == 0 {
		return fmt.Errorf("no repositories provided")
	}

	if err := repoResetStopServices(opts); err != nil {
		return err
	}

	promptReader := bufio.NewReader(opts.Prompt)
	autoYes := opts.Yes
	failures := 0

	for _, inputRepo := range repos {
		repo, err := filepath.Abs(strings.TrimSpace(inputRepo))
		if err != nil {
			fmt.Fprintf(opts.Stderr, "skip invalid repo %q: %v\n", inputRepo, err)
			failures++
			continue
		}
		root, err := resolveCheckoutRoot(repo)
		if err != nil {
			fmt.Fprintf(opts.Stderr, "skip %s: %v\n", repo, err)
			failures++
			continue
		}

		if err := resetOneRepo(root, promptReader, opts, backupRoot, &autoYes); err != nil {
			fmt.Fprintf(opts.Stderr, "%s\n", err)
			failures++
		}
	}

	fmt.Fprintf(opts.Stdout, "\nbackups root: %s\n", backupRoot)
	fmt.Fprintf(opts.Stdout, "restart services with: %s\n", repoResetRestartServicesCommand())

	if failures > 0 {
		return fmt.Errorf("reset completed with %d failure(s)", failures)
	}
	return nil
}

func resetOneRepo(repo string, promptReader *bufio.Reader, opts repoResetOptions, backupRoot string, autoYes *bool) error {
	fmt.Fprintf(opts.Stdout, "\n==> processing %s\n", repo)

	if !opts.IgnoreWorktrees {
		hasWorktrees, err := repoHasManagedWorktrees(repo)
		if err != nil {
			return fmt.Errorf("check worktrees for %s: %w", repo, err)
		}
		if hasWorktrees {
			fmt.Fprintf(opts.Stdout, "skip: %s has existing worktrees; rerun with --ignore-worktrees if you want to reset it anyway\n", repo)
			return nil
		}
	}

	if !*autoYes && !opts.DryRun {
		ok, all, err := promptForRepo(promptReader, opts.Stdout, repo)
		if err != nil {
			return err
		}
		if all {
			*autoYes = true
		}
		if !ok {
			fmt.Fprintf(opts.Stdout, "skip: %s\n", repo)
			return nil
		}
	}

	backupPath := filepath.Join(backupRoot, filepath.Base(repo)+".tar.gz")
	if opts.DryRun {
		fmt.Fprintf(opts.Stdout, "[dry-run] would create backup: %s\n", backupPath)
	} else {
		if err := createManagedBackup(repo, backupPath); err != nil {
			return fmt.Errorf("backup %s: %w", repo, err)
		}
		fmt.Fprintf(opts.Stdout, "backup created: %s\n", backupPath)
	}

	for _, rel := range repoResetCleanupPaths {
		abs := filepath.Join(repo, rel)
		if opts.DryRun {
			fmt.Fprintf(opts.Stdout, "[dry-run] would remove %s\n", abs)
			continue
		}
		if err := os.RemoveAll(abs); err != nil {
			return fmt.Errorf("remove %s: %w", abs, err)
		}
	}

	if opts.DryRun {
		fmt.Fprintf(opts.Stdout, "[dry-run] would sync scaffold in %s\n", repo)
		fmt.Fprintf(opts.Stdout, "[dry-run] would write %s\n", filepath.Join(repo, ".mcp.json"))
		return nil
	}

	if err := repoResetSync(repo, opts.Stdout); err != nil {
		return fmt.Errorf("sync scaffold in %s: %w", repo, err)
	}
	if err := repoResetWriteMCP(repo); err != nil {
		return fmt.Errorf("write .mcp.json in %s: %w", repo, err)
	}
	fmt.Fprintf(opts.Stdout, "done: %s\n", repo)
	return nil
}

func collectResetRepos(args []string, stdin io.Reader) ([]string, error) {
	if len(args) == 1 {
		return readRepoListFile(args[0])
	}
	if !stdinIsTerminal(stdin) {
		return readRepoLines(stdin)
	}
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("get working directory: %w", err)
	}
	return []string{cwd}, nil
}

func readRepoListFile(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open repo list file: %w", err)
	}
	defer f.Close()
	return readRepoLines(f)
}

func readRepoLines(r io.Reader) ([]string, error) {
	var repos []string
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		repos = append(repos, line)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read repo list: %w", err)
	}
	return repos, nil
}

func stdinIsTerminal(r io.Reader) bool {
	f, ok := r.(*os.File)
	if !ok {
		return false
	}
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

func defaultOpenPromptTTY() (io.ReadCloser, error) {
	return os.Open("/dev/tty")
}

func promptForRepo(r *bufio.Reader, out io.Writer, repo string) (ok bool, all bool, err error) {
	for {
		fmt.Fprintf(out, "reset repo %s? [y]es/[n]o/[a]ll/[q]uit: ", repo)
		line, err := r.ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return false, false, fmt.Errorf("read confirmation: %w", err)
		}
		ans := strings.TrimSpace(strings.ToLower(line))
		switch ans {
		case "y", "yes":
			return true, false, nil
		case "n", "no", "":
			return false, false, nil
		case "q", "quit":
			return false, false, fmt.Errorf("aborted at user request")
		case "a", "all":
			return true, true, nil
		default:
			fmt.Fprintln(out, "please answer y, n, a, or q")
		}
		if errors.Is(err, io.EOF) {
			return false, false, fmt.Errorf("read confirmation: unexpected EOF")
		}
	}
}

func repoHasManagedWorktrees(repo string) (bool, error) {
	if _, err := os.Stat(filepath.Join(repo, ".worktrees")); err == nil {
		return true, nil
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return false, err
	}

	cmd := exec.Command("git", "-C", repo, "worktree", "list", "--porcelain")
	out, err := cmd.Output()
	if err != nil {
		return false, nil
	}
	count := 0
	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(line, "worktree ") {
			count++
		}
	}
	return count > 1, nil
}

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

func resolveBackupRoot(prefix string, now func() time.Time) (string, error) {
	if env := os.Getenv("BACKUP_ROOT"); env != "" {
		return env, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve backup root: %w", err)
	}
	ts := now().Format("20060102-150405")
	if prefix == "" {
		return filepath.Join(home, "kasmos-backups", ts), nil
	}
	return filepath.Join(home, "kasmos-backups", prefix+ts), nil
}

func createManagedBackup(repo, out string) error {
	existing, err := existingManagedPaths(repo)
	if err != nil {
		return err
	}
	if len(existing) == 0 {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return fmt.Errorf("create backup dir: %w", err)
	}
	f, err := os.Create(out)
	if err != nil {
		return fmt.Errorf("create backup file: %w", err)
	}
	defer f.Close()

	gzw := gzip.NewWriter(f)
	defer gzw.Close()
	tw := tar.NewWriter(gzw)
	defer tw.Close()

	for _, rel := range existing {
		if err := addPathToTar(tw, repo, rel); err != nil {
			return err
		}
	}
	return nil
}

func existingManagedPaths(repo string) ([]string, error) {
	paths := make([]string, 0, len(managedBackupPaths))
	for _, rel := range managedBackupPaths {
		_, err := os.Lstat(filepath.Join(repo, rel))
		if err == nil {
			paths = append(paths, rel)
			continue
		}
		if !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("stat %s: %w", filepath.Join(repo, rel), err)
		}
	}
	return paths, nil
}

func addPathToTar(tw *tar.Writer, root, rel string) error {
	base := filepath.Join(root, rel)
	return filepath.WalkDir(base, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		name, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		name = filepath.ToSlash(name)

		link := ""
		if info.Mode()&os.ModeSymlink != 0 {
			link, err = os.Readlink(path)
			if err != nil {
				return err
			}
		}
		hdr, err := tar.FileInfoHeader(info, link)
		if err != nil {
			return err
		}
		hdr.Name = name
		if info.IsDir() && !strings.HasSuffix(hdr.Name, "/") {
			hdr.Name += "/"
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = io.Copy(tw, f)
		return err
	})
}

func extractManagedBackup(repo, backupFile string) error {
	f, err := os.Open(backupFile)
	if err != nil {
		return fmt.Errorf("open backup: %w", err)
	}
	defer f.Close()
	gzr, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("open gzip stream: %w", err)
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("read tar entry: %w", err)
		}
		cleanName := filepath.Clean(hdr.Name)
		if filepath.IsAbs(cleanName) || strings.HasPrefix(cleanName, "..") {
			return fmt.Errorf("refusing to extract unsafe path %q", hdr.Name)
		}
		target := filepath.Join(repo, cleanName)
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return fmt.Errorf("create parent dir: %w", err)
		}

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, os.FileMode(hdr.Mode)); err != nil {
				return fmt.Errorf("create dir %s: %w", target, err)
			}
		case tar.TypeReg, tar.TypeRegA:
			f, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.FileMode(hdr.Mode))
			if err != nil {
				return fmt.Errorf("create file %s: %w", target, err)
			}
			if _, err := io.Copy(f, tr); err != nil {
				_ = f.Close()
				return fmt.Errorf("write file %s: %w", target, err)
			}
			if err := f.Close(); err != nil {
				return fmt.Errorf("close file %s: %w", target, err)
			}
		case tar.TypeSymlink:
			if err := os.RemoveAll(target); err != nil {
				return fmt.Errorf("remove existing path %s: %w", target, err)
			}
			if err := os.Symlink(hdr.Linkname, target); err != nil {
				return fmt.Errorf("create symlink %s: %w", target, err)
			}
		default:
			return fmt.Errorf("unsupported tar entry type %d for %s", hdr.Typeflag, hdr.Name)
		}
	}
}

func defaultRepoResetSync(repo string, out io.Writer) error {
	if out == nil {
		out = io.Discard
	}
	return withWorkingDir(repo, func() error {
		tomlCfg, err := config.LoadTOMLConfig()
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}
		if tomlCfg == nil {
			return fmt.Errorf("no config found — run 'kas setup' first to create .kasmos/config.toml")
		}
		agents := profilesToAgentConfigs(tomlCfg.Profiles)
		if len(agents) == 0 {
			return fmt.Errorf("config has no enabled agents — run 'kas setup' to configure agents")
		}

		fmt.Fprintf(out, "Syncing scaffold: %s\n", repo)
		results, err := scaffold.SyncScaffold(repo, agents)
		if err != nil {
			return fmt.Errorf("sync scaffold: %w", err)
		}
		updated, unchanged := 0, 0
		for _, r := range results {
			if r.Created {
				fmt.Fprintf(out, "  %-40s updated\n", r.Path)
				updated++
			} else {
				unchanged++
			}
		}
		fmt.Fprintf(out, "\ndone. %d files updated, %d unchanged.\n", updated, unchanged)

		registry := initcmdharness.NewRegistry()
		home, homeErr := os.UserHomeDir()
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
				if err := initcmdharness.SyncGlobalSkills(home, name); err != nil {
					fmt.Fprintf(out, "FAILED: %v\n", err)
				} else {
					fmt.Fprintln(out, "OK")
				}
			}
		}

		fmt.Fprintln(out, "\nInstalling enforcement hooks...")
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
	})
}

func profilesToAgentConfigs(profiles map[string]config.AgentProfile) []initcmdharness.AgentConfig {
	configs := make([]initcmdharness.AgentConfig, 0, len(profiles))
	for role, p := range profiles {
		if !p.Enabled || p.Program == "" {
			continue
		}
		cfg := initcmdharness.AgentConfig{
			Role:       config.ScaffoldRoleForProfile(role),
			Harness:    p.Program,
			Model:      p.Model,
			Effort:     p.Effort,
			Enabled:    p.Enabled,
			ExtraFlags: p.Flags,
		}
		if p.Temperature != nil {
			temp := *p.Temperature
			cfg.Temperature = &temp
		}
		configs = append(configs, cfg)
		if role == "chat" {
			for _, target := range []string{"coder", "reviewer", "planner", "architect", "fixer", "master"} {
				dup := cfg
				dup.Role = target
				configs = append(configs, dup)
			}
		}
	}
	return configs
}

func defaultRepoResetWriteMCP(repo string) error {
	_, err := scaffold.EnsureClaudeMCPEntry(repo)
	return err
}

func defaultRepoResetStopServices(opts repoResetOptions) error {
	if opts.DryRun {
		fmt.Fprintln(opts.Stdout, "[dry-run] would stop daemon and user services if running")
		return nil
	}
	// Stop service-manager units first (best-effort).
	if err := repoResetStopUserServices(); err != nil {
		fmt.Fprintf(opts.Stderr, "warning: failed to stop user services: %v\n", err)
	}
	// PID-based shutdown is a fallback for manually daemonized runs only.
	// The foreground service-manager path never writes a PID file, so skip
	// the warning when there is no file.
	pidPath := daemonPIDPath()
	if _, err := os.Stat(pidPath); err == nil {
		if err := repoResetStopDaemonByPID(pidPath); err != nil {
			fmt.Fprintf(opts.Stderr, "warning: failed to stop daemon by pid: %v\n", err)
		}
	}
	return nil
}

func withWorkingDir(dir string, fn func() error) error {
	old, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}
	if err := os.Chdir(dir); err != nil {
		return fmt.Errorf("chdir %s: %w", dir, err)
	}
	defer func() { _ = os.Chdir(old) }()
	return fn()
}
