package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/kastheco/kasmos/log"
)

// testSaveStateAllowed is flipped on by tests that legitimately need to
// exercise disk-backed SaveState (e.g. integration tests that want to verify
// round-trip persistence). Default is 0 (disallowed).
//
// This guard exists because SaveState resolves to
// <repo-root>/.kasmos/state.json via GetConfigDir, which means ANY test that
// constructs a real *config.State and triggers SaveInstances clobbers the
// user's live instance list when go test runs from inside a kasmos checkout.
// A single leaking test wipes running agents from the admin UI and the TUI
// lose visibility of them. See cmd/cmd_testmain_test.go for the regression
// guard at the cmd-package level.
var testSaveStateAllowed atomic.Bool

// AllowSaveStateInTest opts the current test process into real SaveState
// writes. Used by integration tests that genuinely need disk persistence.
// Most unit tests should NOT call this — use an in-memory StateManager
// (e.g. cmd/instance_test.go's inMemoryStateManager) instead.
//
// The t argument is required so the function can only be called from test
// code; it is not used at runtime.
func AllowSaveStateInTest(_ *testing.T) {
	testSaveStateAllowed.Store(true)
}

const (
	// StateFileName is the name of the JSON state file within the config dir.
	StateFileName = "state.json"
	// InstancesFileName is the legacy per-file instances store name.
	InstancesFileName = "instances.json"
)

// InstanceStorage is the interface for reading and writing serialised instance data.
type InstanceStorage interface {
	// SaveInstances persists the raw instance JSON blob.
	SaveInstances(instancesJSON json.RawMessage) error
	// GetInstances returns the current raw instance JSON blob.
	GetInstances() json.RawMessage
	// DeleteAllInstances resets the stored instances to an empty list.
	DeleteAllInstances() error
}

// AppState is the interface for reading and writing application-level state.
type AppState interface {
	// GetHelpScreensSeen returns the bitmask of help screens that have been shown.
	GetHelpScreensSeen() uint32
	// SetHelpScreensSeen stores an updated bitmask and persists it.
	SetHelpScreensSeen(seen uint32) error
}

// StateManager is the unified interface combining instance storage and app state.
type StateManager interface {
	InstanceStorage
	AppState
}

// State is the on-disk representation of application state.
type State struct {
	// HelpScreensSeen is a bitmask tracking which help screens the user has seen.
	HelpScreensSeen uint32 `json:"help_screens_seen"`
	// InstancesData holds the serialised instance list as a raw JSON value.
	InstancesData json.RawMessage `json:"instances"`
}

// DefaultState returns an initial State with no help screens seen and an empty instances list.
func DefaultState() *State {
	return &State{
		HelpScreensSeen: 0,
		InstancesData:   json.RawMessage("[]"),
	}
}

// LoadState reads state.json from the config directory. When the file is absent it
// creates and persists a default. On parse errors it returns a default without saving.
func LoadState() *State {
	dir, err := GetConfigDir()
	if err != nil {
		log.ErrorLog.Printf("failed to get config directory: %v", err)
		return DefaultState()
	}

	data, readErr := os.ReadFile(filepath.Join(dir, StateFileName))
	if readErr != nil {
		if os.IsNotExist(readErr) {
			def := DefaultState()
			if saveErr := SaveState(def); saveErr != nil {
				log.WarningLog.Printf("failed to save default state: %v", saveErr)
			}
			return def
		}
		log.WarningLog.Printf("failed to get state file: %v", readErr)
		return DefaultState()
	}

	var s State
	if unmarshalErr := json.Unmarshal(data, &s); unmarshalErr != nil {
		log.ErrorLog.Printf("failed to parse state file: %v", unmarshalErr)
		return DefaultState()
	}

	return &s
}

// SaveState serialises s as indented JSON and writes it to the config directory.
// Under `go test` it refuses to write unless a test has explicitly opted in
// via AllowSaveStateInTest, preventing leaking tests from clobbering the
// user's live <repo>/.kasmos/state.json.
func SaveState(s *State) error {
	if testing.Testing() && !testSaveStateAllowed.Load() {
		return fmt.Errorf("config.SaveState refused during test: use an in-memory StateManager, " +
			"or call config.AllowSaveStateInTest(t) if the test genuinely needs disk persistence")
	}
	dir, err := GetConfigDir()
	if err != nil {
		return fmt.Errorf("failed to get config directory: %w", err)
	}
	if mkErr := os.MkdirAll(dir, 0755); mkErr != nil {
		return fmt.Errorf("failed to create config directory: %w", mkErr)
	}
	data, marshalErr := json.MarshalIndent(s, "", "  ")
	if marshalErr != nil {
		return fmt.Errorf("failed to marshal state: %w", marshalErr)
	}
	return os.WriteFile(filepath.Join(dir, StateFileName), data, 0644)
}

// SaveInstances implements InstanceStorage: replaces the stored instances and persists.
func (s *State) SaveInstances(instancesJSON json.RawMessage) error {
	s.InstancesData = instancesJSON
	return SaveState(s)
}

// GetInstances implements InstanceStorage: returns the raw instance JSON blob.
func (s *State) GetInstances() json.RawMessage {
	return s.InstancesData
}

// DeleteAllInstances implements InstanceStorage: resets instances to an empty list.
func (s *State) DeleteAllInstances() error {
	s.InstancesData = json.RawMessage("[]")
	return SaveState(s)
}

// GetHelpScreensSeen implements AppState: returns the seen-help-screens bitmask.
func (s *State) GetHelpScreensSeen() uint32 {
	return s.HelpScreensSeen
}

// SetHelpScreensSeen implements AppState: stores an updated bitmask and persists.
func (s *State) SetHelpScreensSeen(seen uint32) error {
	s.HelpScreensSeen = seen
	return SaveState(s)
}
