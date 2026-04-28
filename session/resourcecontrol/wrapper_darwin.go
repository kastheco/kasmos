//go:build darwin

package resourcecontrol

import "fmt"

// platformShellPrefix returns a shell-safe prefix string for use with
// WrapShellCommand on macOS. Only nice(1) is used; ionice is not available.
func (w *Wrapper) platformShellPrefix() string {
	return fmt.Sprintf("nice -n %d", w.policy.Nice)
}

// platformWrapExec returns name+args prepended with nice argv on macOS.
func (w *Wrapper) platformWrapExec(name string, args []string) (string, []string) {
	nice, err := w.lookPath("nice")
	if err != nil {
		w.emitWarn("nice not found in PATH; running command without resource wrapper")
		return name, args
	}
	newArgs := []string{"-n", fmt.Sprintf("%d", w.policy.Nice), name}
	newArgs = append(newArgs, args...)
	return nice, newArgs
}
