package cmd

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// TestMain is a regression guard against cmd-package tests leaking writes to
// the user's live <repo>/.kasmos/state.json. It snapshots the state file
// before the test suite runs and re-verifies it afterward. Any mismatch fails
// loudly with the "testClobberedLiveState" marker so it's impossible to miss
// in CI output.
//
// Background: production code calls config.SaveState which resolves to
// GetConfigDir()/state.json. GetConfigDir is cwd-anchored, so when go test is
// invoked from /home/kas/dev/kasmos, any test that constructs a real
// *config.State and passes it to production code that calls SaveInstances
// writes to the user's LIVE state.json and wipes whatever was there before.
//
// The structural fix is to stop using *config.State in tests — use
// inMemoryStateManager from instance_test.go instead. This TestMain exists to
// catch future regressions when a newly added test accidentally returns to
// the old pattern.
func TestMain(m *testing.M) {
	before, beforeErr := snapshotLiveStateFile()

	code := m.Run()

	after, afterErr := snapshotLiveStateFile()

	// The file may not exist at all — that's fine as long as it stays that
	// way. An afterErr that differs from beforeErr (e.g. file now exists, or
	// stat succeeds differently) indicates a write leak.
	if beforeErr == nil && afterErr == nil {
		if before != after {
			fmt.Fprintf(os.Stderr, "testClobberedLiveState: .kasmos/state.json was modified during cmd tests\n"+
				"  before: %s\n"+
				"  after:  %s\n"+
				"This means a test wrote through config.SaveState (via *config.State.SaveInstances).\n"+
				"Use inMemoryStateManager (see cmd/instance_test.go) for test fixtures instead.\n",
				before, after)
			os.Exit(1)
		}
	} else if (beforeErr == nil) != (afterErr == nil) {
		fmt.Fprintf(os.Stderr, "testClobberedLiveState: .kasmos/state.json existence changed during cmd tests\n"+
			"  before: err=%v\n"+
			"  after:  err=%v\n",
			beforeErr, afterErr)
		os.Exit(1)
	}

	os.Exit(code)
}

// snapshotLiveStateFile returns a hex-encoded sha256 of the repo-anchored
// state.json contents. When go test is invoked from a directory inside a
// kasmos checkout the path resolves to <repo>/.kasmos/state.json — exactly
// the file that gets clobbered by a leaking test.
//
// Returns ("", err) with err from os.Stat when the file does not exist; the
// caller treats equal err values (both missing) as a pass.
func snapshotLiveStateFile() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	// Walk upward looking for a .kasmos directory; stop at filesystem root.
	// This is intentionally simpler than config.GetConfigDir (which calls
	// git) to avoid depending on production code from a regression guard.
	dir := cwd
	for {
		candidate := filepath.Join(dir, ".kasmos", "state.json")
		if info, statErr := os.Stat(candidate); statErr == nil && !info.IsDir() {
			data, readErr := os.ReadFile(candidate)
			if readErr != nil {
				return "", readErr
			}
			sum := sha256.Sum256(data)
			return hex.EncodeToString(sum[:]), nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", os.ErrNotExist
		}
		dir = parent
	}
}
