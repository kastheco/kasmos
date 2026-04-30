package lineartrigger

import (
	"strings"

	"github.com/kastheco/kasmos/config/taskfsm"
	"github.com/kastheco/kasmos/config/taskparser"
	"github.com/kastheco/kasmos/config/taskstore"
	"github.com/kastheco/kasmos/internal/linear"
)

// linkedTaskFinder is the task-store lookup needed before create dispatch.
type linkedTaskFinder interface {
	FindLinkedTask(project, issueID string, statuses ...taskstore.Status) (string, error)
}

// ReadinessResult explains whether a trigger can dispatch against a task entry.
type ReadinessResult struct {
	OK            bool
	Reason        string
	CurrentStatus taskstore.Status
}

// Validator checks task and issue readiness for trigger dispatch.
type Validator struct {
	cfg     Config
	store   linkedTaskFinder
	project string
}

// NewValidator returns a readiness validator. Store may be nil when create
// duplicate-link checks are performed by a later dispatcher boundary.
func NewValidator(cfg Config, store linkedTaskFinder, project string) *Validator {
	return &Validator{cfg: cfg, store: store, project: project}
}

// Validate reports whether verb may run for entry and issue.
func (v *Validator) Validate(verb Verb, entry taskstore.TaskEntry, issue linear.Issue) ReadinessResult {
	result := ReadinessResult{CurrentStatus: entry.Status}
	switch verb {
	case VerbHelp, VerbStatus:
		result.OK = true
		return result
	case VerbLink:
		if entry.Filename == "" {
			result.Reason = "unlinked_target"
			return result
		}
		if entry.LinearIssueID != "" && entry.LinearIssueID != issue.ID {
			result.Reason = "already_linked"
			return result
		}
		result.OK = true
		return result
	case VerbCreate:
		if v.hasActiveLinkedTask(issue.ID) {
			result.Reason = "duplicate_link"
			return result
		}
		if entry.Filename != "" {
			result.Reason = "duplicate_link"
			return result
		}
		result.OK = true
		return result
	case VerbPlan:
		return validatePlanEntry(entry, issue.ID)
	case VerbStart:
		return v.validateStartEntry(entry, issue)
	default:
		result.Reason = "invalid_transition"
		return result
	}
}

func (v *Validator) hasActiveLinkedTask(issueID string) bool {
	if v.store == nil || v.project == "" || issueID == "" {
		return false
	}
	_, err := v.store.FindLinkedTask(
		v.project,
		issueID,
		taskstore.StatusReady,
		taskstore.StatusPlanning,
		taskstore.StatusImplementing,
		taskstore.StatusReviewing,
		taskstore.StatusVerifying,
	)
	return err == nil
}

func validatePlanEntry(entry taskstore.TaskEntry, issueID string) ReadinessResult {
	result := ReadinessResult{CurrentStatus: entry.Status}
	if entry.LinearIssueID != issueID {
		result.Reason = "unlinked_target"
		return result
	}
	if entry.Status != taskstore.StatusReady {
		result.Reason = "invalid_transition"
		return result
	}
	if strings.TrimSpace(entry.Content) == "" {
		result.Reason = "missing_plan_content"
		return result
	}
	result.OK = true
	return result
}

func (v *Validator) validateStartEntry(entry taskstore.TaskEntry, issue linear.Issue) ReadinessResult {
	result := validatePlanEntry(entry, issue.ID)
	if !result.OK {
		return result
	}
	if strings.TrimSpace(entry.ExecutionState.Phase) != string(taskfsm.ExecutionPhasePlanned) {
		result.OK = false
		result.Reason = "invalid_transition"
		return result
	}
	if _, err := taskparser.Parse(entry.Content); err != nil {
		result.OK = false
		result.Reason = "unparseable_plan"
		return result
	}
	if v.cfg.StartGuard.RequireStartLabel && !issueHasLabel(issue, v.cfg.Labels.Start) {
		result.OK = false
		result.Reason = "missing_start_label"
		return result
	}
	return result
}

func issueHasLabel(issue linear.Issue, labelID string) bool {
	for _, label := range issue.Labels {
		if label.ID == labelID {
			return true
		}
	}
	return false
}
