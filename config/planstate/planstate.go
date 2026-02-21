package planstate

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// PlanEntry is one plan's state in plan-state.json.
type PlanEntry struct {
	Status      string `json:"status"`
	Implemented string `json:"implemented,omitempty"`
}

// PlanState holds all plan entries and the directory they were loaded from.
type PlanState struct {
	Dir   string
	Plans map[string]PlanEntry
}

// PlanInfo is a plan entry with its filename attached, for display.
type PlanInfo struct {
	Filename string
	Status   string
}

const stateFile = "plan-state.json"

// Load reads plan-state.json from dir. Returns empty state if file missing.
func Load(dir string) (*PlanState, error) {
	path := filepath.Join(dir, stateFile)
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &PlanState{Dir: dir, Plans: make(map[string]PlanEntry)}, nil
		}
		return nil, fmt.Errorf("read plan state: %w", err)
	}

	var plans map[string]PlanEntry
	if err := json.Unmarshal(data, &plans); err != nil {
		return nil, fmt.Errorf("parse plan state: %w", err)
	}

	if plans == nil {
		plans = make(map[string]PlanEntry)
	}

	return &PlanState{Dir: dir, Plans: plans}, nil
}

// Unfinished returns plans that are not done, sorted by filename.
func (ps *PlanState) Unfinished() []PlanInfo {
	result := make([]PlanInfo, 0, len(ps.Plans))
	for filename, entry := range ps.Plans {
		if entry.Status == "done" {
			continue
		}
		result = append(result, PlanInfo{Filename: filename, Status: entry.Status})
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Filename < result[j].Filename
	})

	return result
}

// AllTasksDone returns true if the given plan has status done.
func (ps *PlanState) AllTasksDone(filename string) bool {
	entry, ok := ps.Plans[filename]
	if !ok {
		return false
	}
	return entry.Status == "done"
}

// SetStatus updates a plan's status and persists to disk.
func (ps *PlanState) SetStatus(filename, status string) error {
	if ps.Plans == nil {
		ps.Plans = make(map[string]PlanEntry)
	}

	entry := ps.Plans[filename]
	entry.Status = status
	ps.Plans[filename] = entry

	return ps.save()
}

func (ps *PlanState) save() error {
	data, err := json.MarshalIndent(ps.Plans, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal plan state: %w", err)
	}

	path := filepath.Join(ps.Dir, stateFile)
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		return fmt.Errorf("write plan state: %w", err)
	}

	return nil
}
