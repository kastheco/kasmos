package planstate

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Status is the type-safe status for a plan entry.
type Status string

const (
	// StatusReady is the initial state — plan is queued, no session started yet.
	StatusReady Status = "ready"
	// StatusInProgress means a coder session is actively implementing the plan.
	StatusInProgress Status = "in_progress"
	// StatusDone means the agent has finished implementation and written "done".
	// This is the trigger that causes klique to spawn a reviewer.
	StatusDone Status = "done"
	// StatusReviewing means a reviewer session has been spawned and is running.
	StatusReviewing Status = "reviewing"
	// StatusCompleted is the terminal status set by klique after the reviewer
	// session exits. It is intentionally distinct from StatusDone so that
	// IsDone returns false, breaking the coder→reviewer→done→
	// reviewer infinite spawn cycle.
	StatusCompleted Status = "completed"
)

// PlanEntry is one plan's state in plan-state.json.
type PlanEntry struct {
	Status      Status `json:"status"`
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
	Status   Status
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

// Unfinished returns plans that are not done or completed, sorted by filename.
func (ps *PlanState) Unfinished() []PlanInfo {
	result := make([]PlanInfo, 0, len(ps.Plans))
	for filename, entry := range ps.Plans {
		if entry.Status == StatusDone || entry.Status == StatusCompleted {
			continue
		}
		result = append(result, PlanInfo{Filename: filename, Status: entry.Status})
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Filename < result[j].Filename
	})

	return result
}

// IsDone returns true only if the given plan has status StatusDone.
// StatusCompleted intentionally returns false to prevent re-triggering a reviewer.
func (ps *PlanState) IsDone(filename string) bool {
	entry, ok := ps.Plans[filename]
	if !ok {
		return false
	}
	return entry.Status == StatusDone
}

// SetStatus updates a plan's status and persists to disk.
func (ps *PlanState) SetStatus(filename string, status Status) error {
	if ps.Plans == nil {
		ps.Plans = make(map[string]PlanEntry)
	}

	entry := ps.Plans[filename]
	entry.Status = status
	ps.Plans[filename] = entry

	return ps.save()
}

// DisplayName strips the date prefix and .md extension from a plan filename.
// "2026-02-20-my-feature.md" → "my-feature"
// "plain-plan.md" → "plain-plan"
func DisplayName(filename string) string {
	name := strings.TrimSuffix(filename, ".md")
	if len(name) > 11 && name[4] == '-' && name[7] == '-' && name[10] == '-' {
		name = name[11:]
	}
	return name
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
