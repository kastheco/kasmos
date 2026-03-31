package api

import (
	"time"

	"github.com/kastheco/kasmos/config/taskstore"
)

// TaskStatus is the lightweight task metadata returned by the daemon control
// API for TUI status polling.
type TaskStatus struct {
	Filename             string                   `json:"filename"`
	Status               string                   `json:"status"`
	ExecutionState       taskstore.ExecutionState `json:"execution_state,omitempty"`
	Branch               string                   `json:"branch,omitempty"`
	PRURL                string                   `json:"pr_url,omitempty"`
	ReviewCycle          int                      `json:"review_cycle,omitempty"`
	Description          string                   `json:"description,omitempty"`
	Topic                string                   `json:"topic,omitempty"`
	CreatedAt            time.Time                `json:"created_at,omitempty"`
	Implemented          string                   `json:"implemented,omitempty"`
	PlanningAt           time.Time                `json:"planning_at,omitempty"`
	ImplementingAt       time.Time                `json:"implementing_at,omitempty"`
	ReviewingAt          time.Time                `json:"reviewing_at,omitempty"`
	DoneAt               time.Time                `json:"done_at,omitempty"`
	Goal                 string                   `json:"goal,omitempty"`
	ClickUpTaskID        string                   `json:"clickup_task_id,omitempty"`
	LatestReviewFeedback string                   `json:"latest_review_feedback,omitempty"`
	PRReviewDecision     string                   `json:"pr_review_decision,omitempty"`
	PRCheckStatus        string                   `json:"pr_check_status,omitempty"`
}
