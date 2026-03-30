package api

import "github.com/kastheco/kasmos/config/taskstore"

// TaskStatus is the lightweight task metadata returned by the daemon control
// API for TUI status polling.
type TaskStatus struct {
	Filename       string                   `json:"filename"`
	Status         string                   `json:"status"`
	ExecutionState taskstore.ExecutionState `json:"execution_state,omitempty"`
	Branch         string                   `json:"branch,omitempty"`
	PRURL          string                   `json:"pr_url,omitempty"`
	ReviewCycle    int                      `json:"review_cycle,omitempty"`
	Description    string                   `json:"description,omitempty"`
	Topic          string                   `json:"topic,omitempty"`
}
