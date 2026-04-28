// Package resourcecontrol provides process-wrapper and environment helpers that
// apply the resolved resource-control policy to agent processes.
//
// It owns:
//   - Shell-command wrapping (prepending nice/ionice to a shell string).
//   - exec.Command argv wrapping (prepending nice/ionice to name+args).
//   - Environment merging for build-concurrency variables.
//
// The package imports [config] but intentionally does not import session, session/sdk,
// or session/tmux; those callers depend on this package, not the other way around.
package resourcecontrol
