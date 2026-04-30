package ui

import (
	"github.com/kastheco/kasmos/config/taskstate"
	"github.com/kastheco/kasmos/session"
)

// InstancePresentation classifies an instance for default-view rendering.
// It is the single source of truth used by every layer that decides whether
// an instance is active/idle/retired in the sidebar. Centralising the rules
// here keeps the navigation panel, sidebar status aggregation, and the
// app-side preview-attach logic from drifting apart.
type InstancePresentation int

const (
	// PresentationActive — running, loading, ready, blocked, or otherwise
	// live work the user wants visible.
	PresentationActive InstancePresentation = iota
	// PresentationRetired — instance has exited, or its task is done/cancelled
	// with no live instance still working it.
	PresentationRetired
	// PresentationIdle — explicitly paused with nothing pending.
	PresentationIdle
)

// DeriveInstancePresentation classifies inst given the optional plan status.
// Pass status="" and hasEntry=false for solo agents that have no task entry.
// Status values match taskstate.Status* constants.
//
// The output is mutually exclusive: an instance is exactly one of Active,
// Retired, or Idle. Active mirrors the historical navInstanceActive predicate;
// Retired mirrors navInstanceRetired (given the instance is not Active);
// Idle is the residual (paused-with-nothing-pending or unclassified).
func DeriveInstancePresentation(inst *session.Instance, status string, hasEntry bool) InstancePresentation {
	if inst == nil {
		return PresentationIdle
	}

	// Active: any live, non-paused, non-completed instance the user wants
	// visible. Status==Running/Loading covers transient pre-Started states;
	// Started() covers Ready/Blocked/etc.; SDK mode falls through as Active
	// because the SDK lifecycle is not Started-flag-driven.
	if !inst.Paused() && !inst.Exited && !inst.ImplementationComplete {
		if inst.Status == session.Running || inst.Status == session.Loading {
			return PresentationActive
		}
		if inst.Started() {
			return PresentationActive
		}
		if session.NormalizeExecutionMode(inst.ExecutionMode) == session.ExecutionModeSDK {
			return PresentationActive
		}
	}

	// Retired: instance has exited, or its task is done/cancelled. Only
	// reached when not Active, so a live agent on a done plan stays visible.
	if inst.Exited {
		return PresentationRetired
	}
	if hasEntry {
		s := taskstate.Status(status)
		if s == taskstate.StatusDone || s == taskstate.StatusCancelled {
			return PresentationRetired
		}
	}

	// Idle: explicitly paused with nothing pending. A paused reviewer/coder
	// on an active task stays uncategorized so the existing render paths
	// keep showing it as in-progress.
	if inst.Status == session.Paused {
		s := taskstate.Status(status)
		if !hasEntry || (s != taskstate.StatusReviewing && s != taskstate.StatusImplementing && s != taskstate.StatusVerifying) {
			return PresentationIdle
		}
	}
	return PresentationIdle
}
