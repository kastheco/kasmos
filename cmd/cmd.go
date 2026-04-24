package cmd

import (
	"os/exec"

	"github.com/kastheco/kasmos/internal/cmdexec"
	"github.com/spf13/cobra"
)

// Executor abstracts command execution for testability.
type Executor = cmdexec.Executor

// Exec is the real executor that delegates to the underlying os/exec.Cmd methods.
type Exec = cmdexec.Exec

// MakeExecutor returns the default real executor.
func MakeExecutor() Executor {
	return cmdexec.Make()
}

// ToString returns a human-readable representation of a command's arguments.
// Returns "<nil>" when cmd is nil.
func ToString(cmd *exec.Cmd) string {
	return cmdexec.ToString(cmd)
}

// NewRootCmd builds the root cobra command with all subcommands attached.
func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "kas",
		Short: "kas - Manage multiple AI agents",
	}
	root.AddCommand(NewTaskCmd())
	root.AddCommand(NewServeCmd())
	root.AddCommand(NewMCPCmd())
	root.AddCommand(NewBrowserCmd())
	root.AddCommand(NewInstanceCmd())
	root.AddCommand(NewAuditCmd())
	root.AddCommand(NewTmuxCmd())
	root.AddCommand(NewSignalCmd())
	root.AddCommand(NewResetCmd())
	root.AddCommand(NewRestoreCmd())
	root.AddCommand(NewDaemonCmd())
	root.AddCommand(NewMonitorCmd())
	root.AddCommand(NewStatusCmd())
	return root
}
