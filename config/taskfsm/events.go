package taskfsm

import (
	"sort"
	"strings"
)

// primaryEventNames maps the canonical public token for each event to its Event
// constant. "review_changes" is kept as the published token for
// ReviewChangesRequested so existing CLI/MCP help text is unchanged.
var primaryEventNames = map[string]Event{
	"plan_start":         PlanStart,
	"planner_finished":   PlannerFinished,
	"implement_start":    ImplementStart,
	"implement_finished": ImplementFinished,
	"review_approved":    ReviewApproved,
	"review_changes":     ReviewChangesRequested, // primary published token
	"verify_approved":    VerifyApproved,
	"verify_failed":      VerifyFailed,
	"request_review":     RequestReview,
	"start_over":         StartOver,
	"reimplement":        Reimplement,
	"cancel":             Cancel,
	"reopen":             Reopen,
}

// eventAliases maps additional accepted tokens to their Event constants.
// These aliases are accepted by EventByName but are not returned by EventNames.
var eventAliases = map[string]Event{
	"review_changes_requested": ReviewChangesRequested,
}

// EventByName looks up a lifecycle event by name. It trims whitespace,
// normalizes hyphens to underscores, and accepts all primary tokens as well as
// registered aliases (e.g. "review_changes_requested" → ReviewChangesRequested).
// Returns (event, true) on success, ("", false) if no match is found.
func EventByName(raw string) (Event, bool) {
	normalized := strings.ReplaceAll(strings.TrimSpace(raw), "-", "_")
	if ev, ok := primaryEventNames[normalized]; ok {
		return ev, true
	}
	if ev, ok := eventAliases[normalized]; ok {
		return ev, true
	}
	return "", false
}

// EventNames returns the primary public tokens in deterministic sorted order.
// Aliases (e.g. "review_changes_requested") are intentionally excluded so that
// help text and error messages remain stable.
func EventNames() []string {
	names := make([]string, 0, len(primaryEventNames))
	for k := range primaryEventNames {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}
