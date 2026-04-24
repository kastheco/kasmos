package tmux

import (
	"os/exec"
	"strings"
)

func commandString(cmd *exec.Cmd) string {
	if cmd == nil {
		return "<nil>"
	}
	return strings.Join(cmd.Args, " ")
}
