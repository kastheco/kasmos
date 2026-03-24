package cmd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

type repoRestoreOptions struct {
	DryRun     bool
	Yes        bool
	Stdout     io.Writer
	Stderr     io.Writer
	Prompt     io.Reader
	BackupRoot string
}

// NewRestoreCmd returns the top-level restore command.
func NewRestoreCmd() *cobra.Command {
	var dryRun bool
	var yes bool

	cmd := &cobra.Command{
		Use:          "restore <backup-tar.gz> <repo>",
		Short:        "Restore one kasmos repo backup into one repo",
		SilenceUsage: true,
		Args:         cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRepoRestore(args[0], args[1], repoRestoreOptions{
				DryRun: dryRun,
				Yes:    yes,
				Stdout: cmd.OutOrStdout(),
				Stderr: cmd.ErrOrStderr(),
				Prompt: cmd.InOrStdin(),
			})
		},
	}

	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Print what would happen, but make no changes")
	cmd.Flags().BoolVar(&yes, "yes", false, "Skip confirmation prompt")
	return cmd
}

func runRepoRestore(backupFile, repo string, opts repoRestoreOptions) error {
	if opts.Stdout == nil {
		opts.Stdout = io.Discard
	}
	if opts.Stderr == nil {
		opts.Stderr = io.Discard
	}
	if opts.Prompt == nil {
		opts.Prompt = os.Stdin
	}

	backupFile, err := filepath.Abs(backupFile)
	if err != nil {
		return fmt.Errorf("resolve backup file: %w", err)
	}
	if _, err := os.Stat(backupFile); err != nil {
		return fmt.Errorf("backup file not found: %w", err)
	}

	repo, err = filepath.Abs(repo)
	if err != nil {
		return fmt.Errorf("resolve repo path: %w", err)
	}
	if _, err := os.Stat(repo); err != nil {
		return fmt.Errorf("repo path not found: %w", err)
	}

	if !opts.Yes && !opts.DryRun {
		promptReader := opts.Prompt
		if !stdinIsTerminal(promptReader) {
			tty, err := openPromptTTY()
			if err != nil {
				return fmt.Errorf("no tty available for restore confirmation; rerun with --yes: %w", err)
			}
			defer tty.Close()
			promptReader = tty
		}
		ok, err := promptForRestore(promptReader, opts.Stdout, backupFile, repo)
		if err != nil {
			return err
		}
		if !ok {
			fmt.Fprintln(opts.Stdout, "restore cancelled")
			return nil
		}
	}

	backupRoot, err := resolveBackupRoot("restore-", repoResetNow)
	if err != nil {
		return err
	}
	if opts.BackupRoot != "" {
		backupRoot = opts.BackupRoot
	}
	preRestore := filepath.Join(backupRoot, filepath.Base(repo)+"-pre-restore.tar.gz")

	if opts.DryRun {
		fmt.Fprintf(opts.Stdout, "[dry-run] would create pre-restore backup: %s\n", preRestore)
		for _, rel := range managedBackupPaths {
			fmt.Fprintf(opts.Stdout, "[dry-run] would remove %s\n", filepath.Join(repo, rel))
		}
		fmt.Fprintf(opts.Stdout, "[dry-run] would extract %s into %s\n", backupFile, repo)
		fmt.Fprintf(opts.Stdout, "pre-restore backups root: %s\n", backupRoot)
		return nil
	}

	if err := createManagedBackup(repo, preRestore); err != nil {
		return fmt.Errorf("create pre-restore backup: %w", err)
	}
	fmt.Fprintf(opts.Stdout, "pre-restore backup created: %s\n", preRestore)

	for _, rel := range managedBackupPaths {
		if err := os.RemoveAll(filepath.Join(repo, rel)); err != nil {
			return fmt.Errorf("remove %s: %w", filepath.Join(repo, rel), err)
		}
	}
	if err := extractManagedBackup(repo, backupFile); err != nil {
		return fmt.Errorf("restore backup into %s: %w", repo, err)
	}
	fmt.Fprintf(opts.Stdout, "restore complete: %s\n", repo)
	fmt.Fprintf(opts.Stdout, "pre-restore backups root: %s\n", backupRoot)
	return nil
}

func promptForRestore(r io.Reader, out io.Writer, backupFile, repo string) (bool, error) {
	reader := io.Reader(r)
	buf, ok := reader.(*os.File)
	if ok {
		reader = buf
	}
	scanner := bufio.NewScanner(reader)
	for {
		fmt.Fprintf(out, "restore %s into %s? [y/N]: ", backupFile, repo)
		if !scanner.Scan() {
			if err := scanner.Err(); err != nil {
				return false, fmt.Errorf("read confirmation: %w", err)
			}
			return false, nil
		}
		ans := strings.TrimSpace(strings.ToLower(scanner.Text()))
		switch ans {
		case "y", "yes":
			return true, nil
		case "", "n", "no":
			return false, nil
		default:
			fmt.Fprintln(out, "please answer y or n")
		}
	}
}
