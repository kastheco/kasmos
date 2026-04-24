// Package cmdexec provides the shared command execution abstraction used by
// packages that need to run external commands without coupling to the Cobra cmd
// package.
package cmdexec

import (
	"os/exec"
	"strings"
)

// Executor abstracts command execution for testability.
type Executor interface {
	Run(cmd *exec.Cmd) error
	Output(cmd *exec.Cmd) ([]byte, error)
}

// Exec is the real executor that delegates to the underlying os/exec.Cmd
// methods.
type Exec struct{}

// Run executes the command and waits for it to complete.
func (e Exec) Run(cmd *exec.Cmd) error {
	return cmd.Run()
}

// Output runs the command and returns its standard output.
func (e Exec) Output(cmd *exec.Cmd) ([]byte, error) {
	return cmd.Output()
}

// Make returns the default real executor.
func Make() Executor {
	return Exec{}
}

// ToString returns a human-readable representation of a command's arguments.
// Returns "<nil>" when cmd is nil.
func ToString(cmd *exec.Cmd) string {
	if cmd == nil {
		return "<nil>"
	}
	return strings.Join(cmd.Args, " ")
}
