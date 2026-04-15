// Package livepreview provides shared primitives for reading kasmos instance
// state and executing tmux capture-pane. Both the MCP instancetools handlers
// and the HTTP live-preview handler import this package so they share the same
// types and logic without duplication.
package livepreview

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/kastheco/kasmos/config"
)

// Status mirrors session.Status (int iota) without importing session.
// Values must stay in sync with session package constants:
//
//	Running = 0, Ready = 1, Loading = 2, Paused = 3
type Status int

const (
	StatusRunning Status = 0
	StatusReady   Status = 1
	StatusLoading Status = 2
	StatusPaused  Status = 3
)

// Worktree holds git worktree metadata. It fully mirrors session.GitWorktreeData
// so round-trip serialisation is lossless.
type Worktree struct {
	RepoPath      string `json:"repo_path"`
	WorktreePath  string `json:"worktree_path"`
	SessionName   string `json:"session_name"`
	BranchName    string `json:"branch_name"`
	BaseCommitSHA string `json:"base_commit_sha"`
}

// Record is a read-only mirror of session.InstanceData containing all fields
// required for lossless round-trip serialisation. Every field present in
// InstanceData must appear here; omitting a field causes silent data loss when
// the state file is rewritten by pause/resume. ExecutionMode is held as a
// plain string so livepreview stays session-import free.
type Record struct {
	Title        string    `json:"title"`
	DisplayTitle string    `json:"display_title,omitempty"`
	Path         string    `json:"path"`
	Branch       string    `json:"branch"`
	Status       Status    `json:"status"`
	Height       int       `json:"height"`
	Width        int       `json:"width"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	Program      string    `json:"program"`
	AutoYes      bool      `json:"auto_yes"`

	// SkipPermissions, when true, passes --permission-mode bypassPermissions to Claude.
	SkipPermissions bool `json:"skip_permissions"`

	// ExecutionMode mirrors session.ExecutionMode ("tmux" or "headless").
	ExecutionMode string `json:"execution_mode,omitempty"`

	// Optional plan/orchestration fields — must stay in sync with InstanceData.
	TaskFile               string `json:"task_file,omitempty"`
	AgentType              string `json:"agent_type,omitempty"`
	TaskNumber             int    `json:"task_number,omitempty"`
	WaveNumber             int    `json:"wave_number,omitempty"`
	PeerCount              int    `json:"peer_count,omitempty"`
	WaveTaskIndex          int    `json:"wave_task_index,omitempty"`
	WaveTaskCount          int    `json:"wave_task_count,omitempty"`
	IsReviewer             bool   `json:"is_reviewer,omitempty"`
	ImplementationComplete bool   `json:"implementation_complete,omitempty"`
	SoloAgent              bool   `json:"solo_agent,omitempty"`
	QueuedPrompt           string `json:"queued_prompt,omitempty"`
	ReviewCycle            int    `json:"review_cycle,omitempty"`
	ClaudeNoFlicker        bool   `json:"claude_no_flicker,omitempty"`

	Worktree Worktree `json:"worktree"`
}

// UnmarshalJSON implements a custom unmarshaler that handles the historical
// rename from the "plan_file" JSON key to "task_file", mirroring
// session.InstanceData.UnmarshalJSON.
func (r *Record) UnmarshalJSON(data []byte) error {
	type Alias Record
	aux := &struct {
		*Alias
		PlanFile string `json:"plan_file,omitempty"`
	}{
		Alias: (*Alias)(r),
	}
	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}
	if r.TaskFile == "" && aux.PlanFile != "" {
		r.TaskFile = aux.PlanFile
	}
	return nil
}

// StateLoader is a function that returns a fresh StateManager snapshot.
// Each call should return a consistent view of the current state.
type StateLoader func() config.StateManager

// LoadRecords reads and parses the raw instance JSON from the state loader.
func LoadRecords(loadState StateLoader) ([]Record, error) {
	state := loadState()
	raw := state.GetInstances()
	var records []Record
	if err := json.Unmarshal(raw, &records); err != nil {
		return nil, fmt.Errorf("parse instances: %w", err)
	}
	return records, nil
}

// FindRecord finds an instance record by title. It first tries an exact
// match, then falls back to a substring match. Returns an error when no match
// is found or when the substring matches more than one record (ambiguous).
func FindRecord(records []Record, title string) (Record, error) {
	// Exact match takes precedence.
	for _, r := range records {
		if r.Title == title {
			return r, nil
		}
	}

	// Substring fallback.
	var matches []Record
	for _, r := range records {
		if strings.Contains(r.Title, title) {
			matches = append(matches, r)
		}
	}
	switch len(matches) {
	case 0:
		return Record{}, fmt.Errorf("instance not found: %q", title)
	case 1:
		return matches[0], nil
	default:
		names := make([]string, len(matches))
		for i, m := range matches {
			names[i] = m.Title
		}
		return Record{}, fmt.Errorf("ambiguous instance %q matches: %s", title, strings.Join(names, ", "))
	}
}

// StatusLabel converts a Status to a lowercase text label.
func StatusLabel(s Status) string {
	switch s {
	case StatusRunning:
		return "running"
	case StatusReady:
		return "ready"
	case StatusLoading:
		return "loading"
	case StatusPaused:
		return "paused"
	default:
		return fmt.Sprintf("unknown(%d)", s)
	}
}

// whiteSpaceRe matches one or more whitespace characters for session name sanitisation.
var whiteSpaceRe = regexp.MustCompile(`\s+`)

// SessionName converts a human-readable instance title to the kas_-prefixed tmux
// session name used by the session package. It replicates toKasTmuxName from
// session/tmux without importing that package (which would create a cycle).
func SessionName(title string) string {
	name := whiteSpaceRe.ReplaceAllString(title, "")
	name = strings.ReplaceAll(name, ".", "_")
	return "kas_" + name
}

// ValidateAction checks whether the instance is in a state compatible with the
// requested action and returns an error when it is not.
//
//   - kill:    allowed in any status
//   - pause:   not allowed when already paused
//   - resume:  only allowed when paused
//   - send:    only allowed in running/ready tmux-mode instances (not loading, paused, or headless)
//   - capture: not allowed when paused or in headless mode
func ValidateAction(rec Record, action string) error {
	switch action {
	case "kill":
		return nil
	case "pause":
		if rec.Status == StatusPaused {
			return fmt.Errorf("instance is already paused")
		}
		return nil
	case "resume":
		if rec.Status != StatusPaused {
			return fmt.Errorf("can only resume paused instances (current status: %s)", StatusLabel(rec.Status))
		}
		return nil
	case "send":
		if rec.Status == StatusPaused {
			return fmt.Errorf("cannot send prompt to a paused instance")
		}
		if rec.Status == StatusLoading {
			return fmt.Errorf("cannot send prompt to a loading instance")
		}
		if config.NormalizeExecutionMode(rec.ExecutionMode) == config.ExecutionModeHeadless {
			return fmt.Errorf("cannot send prompt to a headless instance")
		}
		return nil
	case "capture":
		if rec.Status == StatusPaused {
			return fmt.Errorf("cannot capture pane from a paused instance")
		}
		if config.NormalizeExecutionMode(rec.ExecutionMode) == config.ExecutionModeHeadless {
			return fmt.Errorf("cannot capture pane from a headless instance")
		}
		return nil
	default:
		return fmt.Errorf("unknown action: %q", action)
	}
}
