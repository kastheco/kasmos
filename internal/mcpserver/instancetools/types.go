// Package instancetools provides MCP tools for managing kasmos agent instances
// and querying the kasmos daemon. It exposes instance_list, instance_send,
// instance_pause, instance_resume, and daemon_status tools that agents can use
// to manage instances and inspect daemon state via typed MCP calls.
package instancetools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/kastheco/kasmos/internal/livepreview"
)

// CmdRunner abstracts external command execution for testability.
// It exposes both Run (fire-and-forget) and Output (capture stdout).
type CmdRunner interface {
	Run(ctx context.Context, name string, args ...string) error
	Output(ctx context.Context, name string, args ...string) ([]byte, error)
}

// ExecRunner is the real CmdRunner that delegates to os/exec.
type ExecRunner struct{}

// Run executes name with args under the given context, discarding output.
func (r *ExecRunner) Run(ctx context.Context, name string, args ...string) error {
	return exec.CommandContext(ctx, name, args...).Run()
}

// Output runs name with args under the given context and returns its standard output.
func (r *ExecRunner) Output(ctx context.Context, name string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, name, args...).Output()
}

// StateLoader is a type alias for livepreview.StateLoader so both packages
// share the same underlying function type.
type StateLoader = livepreview.StateLoader

// instanceRecord is a type alias for livepreview.Record. Pause, resume, send,
// and list handlers use this alias so they continue to compile without change
// after the struct definition moved to the shared package.
type instanceRecord = livepreview.Record

// instanceWorktree is a type alias for livepreview.Worktree, retained so that
// tests constructing worktree literals (e.g. pause_resume_test.go) continue to
// compile unmodified.
type instanceWorktree = livepreview.Worktree

// instanceStatus is a type alias for livepreview.Status.
type instanceStatus = livepreview.Status

const (
	instanceRunning instanceStatus = livepreview.StatusRunning
	instanceReady   instanceStatus = livepreview.StatusReady
	instanceLoading instanceStatus = livepreview.StatusLoading
	instancePaused  instanceStatus = livepreview.StatusPaused
)

// loadRecords is a package-local convenience wrapper around livepreview.LoadRecords.
func loadRecords(loadState StateLoader) ([]instanceRecord, error) {
	return livepreview.LoadRecords(loadState)
}

// findRecord is a package-local convenience wrapper around livepreview.FindRecord.
func findRecord(records []instanceRecord, title string) (instanceRecord, error) {
	return livepreview.FindRecord(records, title)
}

// validateAction is a package-local convenience wrapper around livepreview.ValidateAction.
func validateAction(rec instanceRecord, action string) error {
	return livepreview.ValidateAction(rec, action)
}

// kasTmuxName is a package-local convenience wrapper around livepreview.SessionName.
func kasTmuxName(title string) string {
	return livepreview.SessionName(title)
}

// updateRecord finds the named instance, applies updater to a copy, and
// persists the modified list back to state. All fields of every record are
// preserved verbatim. This mutating helper stays in the instancetools package
// because it writes state; read-only helpers live in livepreview.
func updateRecord(loadState StateLoader, title string, updater func(*instanceRecord) error) error {
	state := loadState()
	raw := state.GetInstances()
	var records []instanceRecord
	if err := json.Unmarshal(raw, &records); err != nil {
		return fmt.Errorf("parse instances: %w", err)
	}
	found := false
	for i := range records {
		if records[i].Title == title {
			if err := updater(&records[i]); err != nil {
				return err
			}
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("instance not found: %q", title)
	}
	out, err := json.Marshal(records)
	if err != nil {
		return fmt.Errorf("marshal instances: %w", err)
	}
	return state.SaveInstances(out)
}

// daemonSocketPath returns the default Unix domain socket path for the daemon
// control API. Matches the defaultSocketPath() logic in the daemon package:
// prefers $XDG_RUNTIME_DIR/kasmos/kas.sock, then falls back to
// /tmp/kasmos-<uid>/kas.sock.
func daemonSocketPath() string {
	if xdg := os.Getenv("XDG_RUNTIME_DIR"); xdg != "" {
		return filepath.Join(xdg, "kasmos", "kas.sock")
	}
	return filepath.Join(os.TempDir(), fmt.Sprintf("kasmos-%d", os.Getuid()), "kas.sock")
}
