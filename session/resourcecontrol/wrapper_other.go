//go:build !linux && !darwin

package resourcecontrol

// platformShellPrefix returns an empty string on unsupported platforms; the
// caller (WrapShellCommand) returns the command unchanged.
func (w *Wrapper) platformShellPrefix() string { return "" }

// platformWrapExec returns the original name and args unchanged on unsupported
// platforms.
func (w *Wrapper) platformWrapExec(name string, args []string) (string, []string) {
	return name, args
}
