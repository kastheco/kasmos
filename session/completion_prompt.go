package session

import "time"

// CompletionPromptStabilityWindow is the minimum duration a completion-eligible
// prompt state must persist before it counts as a genuine task completion.
// Both the TUI wave monitor and daemon use this constant so they share the same
// threshold regardless of their differing poll intervals.
const CompletionPromptStabilityWindow = 500 * time.Millisecond

// UpdateCompletionPromptState advances or resets the completion-prompt timer based on
// the instance's current state. Call this once per metadata poll pass before calling
// HasStableCompletionPrompt.
//
// The timer starts the first time the instance is in a completion-eligible state
// (prompt detected, not awaiting work, not permission-blocked). It is reset to zero
// whenever any of those conditions is no longer true.
func (i *Instance) UpdateCompletionPromptState(now time.Time) {
	if i.PromptDetected && !i.AwaitingWork && !i.PermissionBlocked {
		if i.CompletionPromptSince.IsZero() {
			i.CompletionPromptSince = now
		}
		return
	}
	i.CompletionPromptSince = time.Time{}
}

// HasStableCompletionPrompt reports whether the instance has been in a
// completion-eligible prompt state for at least CompletionPromptStabilityWindow.
// Returns false if the instance is permission-blocked, awaiting work, or has not
// yet held the prompt long enough.
func (i *Instance) HasStableCompletionPrompt(now time.Time) bool {
	return i.PromptDetected && !i.AwaitingWork && !i.PermissionBlocked &&
		!i.CompletionPromptSince.IsZero() &&
		now.Sub(i.CompletionPromptSince) >= CompletionPromptStabilityWindow
}
