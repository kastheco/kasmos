//! State machine implementation for work package and run states.
//!
//! Enforces valid state transitions and provides clear error messages for invalid ones.

use crate::error::{Result, StateError};
use crate::types::{RunState, WPState};

impl WPState {
    /// Check if a transition to the target state is valid.
    ///
    /// Valid transitions:
    /// - Pending → Active (when wave launches)
    /// - Active → Completed (on completion detection)
    /// - Active → Failed (on crash/error)
    /// - Active → Paused (on pause command)
    /// - Paused → Active (on resume command)
    /// - Failed → Pending (on retry command)
    /// - Failed → Active (on restart command)
    pub fn can_transition_to(&self, target: &WPState) -> bool {
        matches!(
            (self, target),
            // Pending transitions
            (WPState::Pending, WPState::Active) |
            // Active transitions
            (WPState::Active, WPState::Completed) |
            (WPState::Active, WPState::Failed) |
            (WPState::Active, WPState::Paused) |
            (WPState::Active, WPState::ForReview) |
            // Paused transitions
            (WPState::Paused, WPState::Active) |
            // Failed transitions
            (WPState::Failed, WPState::Pending) |
            (WPState::Failed, WPState::Active) |
            (WPState::Failed, WPState::Completed) |
            // ForReview transitions
            (WPState::ForReview, WPState::Completed) |  // approve
            (WPState::ForReview, WPState::Active) |     // reject + relaunch
            (WPState::ForReview, WPState::Pending) |    // reject + hold
            // Self-transitions (idempotent)
            (WPState::Completed, WPState::Completed) |
            (WPState::Failed, WPState::Failed) |
            (WPState::Pending, WPState::Pending) |
            (WPState::Active, WPState::Active) |
            (WPState::Paused, WPState::Paused) |
            (WPState::ForReview, WPState::ForReview)
        )
    }

    /// Transition to a target state, returning an error if invalid.
    pub fn transition(&self, target: WPState, wp_id: &str) -> Result<WPState> {
        if self.can_transition_to(&target) {
            Ok(target)
        } else {
            Err(StateError::InvalidTransition {
                wp_id: wp_id.to_string(),
                from: *self,
                to: target,
            }
            .into())
        }
    }
}

impl RunState {
    /// Check if a transition to the target state is valid.
    ///
    /// Valid transitions:
    /// - Initializing → Running
    /// - Running → Paused (wave-gated boundary)
    /// - Paused → Running (operator confirms)
    /// - Running → Completed (all WPs done)
    /// - Running → Failed (unrecoverable error)
    /// - Running → Aborted (operator abort)
    pub fn can_transition_to(&self, target: &RunState) -> bool {
        matches!(
            (self, target),
            // Initializing transitions
            (RunState::Initializing, RunState::Running) |
            // Running transitions
            (RunState::Running, RunState::Paused) |
            (RunState::Running, RunState::Completed) |
            (RunState::Running, RunState::Failed) |
            (RunState::Running, RunState::Aborted) |
            // Paused transitions
            (RunState::Paused, RunState::Running) |
            // Self-transitions (idempotent)
            (RunState::Completed, RunState::Completed) |
            (RunState::Failed, RunState::Failed) |
            (RunState::Aborted, RunState::Aborted) |
            (RunState::Initializing, RunState::Initializing) |
            (RunState::Running, RunState::Running) |
            (RunState::Paused, RunState::Paused)
        )
    }

    /// Transition to a target state, returning an error if invalid.
    pub fn transition(&self, target: RunState) -> Result<RunState> {
        if self.can_transition_to(&target) {
            Ok(target)
        } else {
            Err(StateError::InvalidTransition {
                wp_id: "run".to_string(),
                from: WPState::Pending, // Placeholder; RunState doesn't use this error directly
                to: WPState::Pending,
            }
            .into())
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    // ============ WPState Tests ============

    #[test]
    fn test_wp_pending_to_active() {
        assert!(WPState::Pending.can_transition_to(&WPState::Active));
        assert!(WPState::Pending.transition(WPState::Active, "WP01").is_ok());
    }

    #[test]
    fn test_wp_active_to_completed() {
        assert!(WPState::Active.can_transition_to(&WPState::Completed));
        assert!(
            WPState::Active
                .transition(WPState::Completed, "WP01")
                .is_ok()
        );
    }

    #[test]
    fn test_wp_active_to_failed() {
        assert!(WPState::Active.can_transition_to(&WPState::Failed));
        assert!(WPState::Active.transition(WPState::Failed, "WP01").is_ok());
    }

    #[test]
    fn test_wp_active_to_paused() {
        assert!(WPState::Active.can_transition_to(&WPState::Paused));
        assert!(WPState::Active.transition(WPState::Paused, "WP01").is_ok());
    }

    #[test]
    fn test_wp_paused_to_active() {
        assert!(WPState::Paused.can_transition_to(&WPState::Active));
        assert!(WPState::Paused.transition(WPState::Active, "WP01").is_ok());
    }

    #[test]
    fn test_wp_failed_to_pending() {
        assert!(WPState::Failed.can_transition_to(&WPState::Pending));
        assert!(WPState::Failed.transition(WPState::Pending, "WP01").is_ok());
    }

    #[test]
    fn test_wp_failed_to_active() {
        assert!(WPState::Failed.can_transition_to(&WPState::Active));
        assert!(WPState::Failed.transition(WPState::Active, "WP01").is_ok());
    }

    #[test]
    fn test_wp_invalid_completed_to_active() {
        assert!(!WPState::Completed.can_transition_to(&WPState::Active));
        assert!(
            WPState::Completed
                .transition(WPState::Active, "WP01")
                .is_err()
        );
    }

    #[test]
    fn test_wp_invalid_pending_to_completed() {
        assert!(!WPState::Pending.can_transition_to(&WPState::Completed));
        assert!(
            WPState::Pending
                .transition(WPState::Completed, "WP01")
                .is_err()
        );
    }

    #[test]
    fn test_wp_invalid_pending_to_failed() {
        assert!(!WPState::Pending.can_transition_to(&WPState::Failed));
        assert!(
            WPState::Pending
                .transition(WPState::Failed, "WP01")
                .is_err()
        );
    }

    #[test]
    fn test_wp_invalid_completed_to_failed() {
        assert!(!WPState::Completed.can_transition_to(&WPState::Failed));
        assert!(
            WPState::Completed
                .transition(WPState::Failed, "WP01")
                .is_err()
        );
    }

    #[test]
    fn test_wp_self_transition_completed() {
        assert!(WPState::Completed.can_transition_to(&WPState::Completed));
        assert!(
            WPState::Completed
                .transition(WPState::Completed, "WP01")
                .is_ok()
        );
    }

    #[test]
    fn test_wp_self_transition_active() {
        assert!(WPState::Active.can_transition_to(&WPState::Active));
        assert!(WPState::Active.transition(WPState::Active, "WP01").is_ok());
    }

    // ============ RunState Tests ============

    #[test]
    fn test_run_initializing_to_running() {
        assert!(RunState::Initializing.can_transition_to(&RunState::Running));
        assert!(RunState::Initializing.transition(RunState::Running).is_ok());
    }

    #[test]
    fn test_run_running_to_paused() {
        assert!(RunState::Running.can_transition_to(&RunState::Paused));
        assert!(RunState::Running.transition(RunState::Paused).is_ok());
    }

    #[test]
    fn test_run_paused_to_running() {
        assert!(RunState::Paused.can_transition_to(&RunState::Running));
        assert!(RunState::Paused.transition(RunState::Running).is_ok());
    }

    #[test]
    fn test_run_running_to_completed() {
        assert!(RunState::Running.can_transition_to(&RunState::Completed));
        assert!(RunState::Running.transition(RunState::Completed).is_ok());
    }

    #[test]
    fn test_run_running_to_failed() {
        assert!(RunState::Running.can_transition_to(&RunState::Failed));
        assert!(RunState::Running.transition(RunState::Failed).is_ok());
    }

    #[test]
    fn test_run_running_to_aborted() {
        assert!(RunState::Running.can_transition_to(&RunState::Aborted));
        assert!(RunState::Running.transition(RunState::Aborted).is_ok());
    }

    #[test]
    fn test_run_invalid_completed_to_running() {
        assert!(!RunState::Completed.can_transition_to(&RunState::Running));
        assert!(RunState::Completed.transition(RunState::Running).is_err());
    }

    #[test]
    fn test_run_invalid_initializing_to_paused() {
        assert!(!RunState::Initializing.can_transition_to(&RunState::Paused));
        assert!(RunState::Initializing.transition(RunState::Paused).is_err());
    }

    #[test]
    fn test_run_invalid_failed_to_running() {
        assert!(!RunState::Failed.can_transition_to(&RunState::Running));
        assert!(RunState::Failed.transition(RunState::Running).is_err());
    }

    #[test]
    fn test_run_self_transition_running() {
        assert!(RunState::Running.can_transition_to(&RunState::Running));
        assert!(RunState::Running.transition(RunState::Running).is_ok());
    }

    #[test]
    fn test_run_self_transition_completed() {
        assert!(RunState::Completed.can_transition_to(&RunState::Completed));
        assert!(RunState::Completed.transition(RunState::Completed).is_ok());
    }

    // ============ ForReview WPState Tests ============

    #[test]
    fn test_wp_active_to_for_review() {
        assert!(WPState::Active.can_transition_to(&WPState::ForReview));
        assert!(
            WPState::Active
                .transition(WPState::ForReview, "WP01")
                .is_ok()
        );
    }

    #[test]
    fn test_wp_for_review_to_completed() {
        assert!(WPState::ForReview.can_transition_to(&WPState::Completed));
        assert!(
            WPState::ForReview
                .transition(WPState::Completed, "WP01")
                .is_ok()
        );
    }

    #[test]
    fn test_wp_for_review_to_active() {
        assert!(WPState::ForReview.can_transition_to(&WPState::Active));
        assert!(
            WPState::ForReview
                .transition(WPState::Active, "WP01")
                .is_ok()
        );
    }

    #[test]
    fn test_wp_for_review_to_pending() {
        assert!(WPState::ForReview.can_transition_to(&WPState::Pending));
        assert!(
            WPState::ForReview
                .transition(WPState::Pending, "WP01")
                .is_ok()
        );
    }

    #[test]
    fn test_wp_for_review_self_transition() {
        assert!(WPState::ForReview.can_transition_to(&WPState::ForReview));
        assert!(
            WPState::ForReview
                .transition(WPState::ForReview, "WP01")
                .is_ok()
        );
    }

    #[test]
    fn test_wp_invalid_for_review_to_failed() {
        assert!(!WPState::ForReview.can_transition_to(&WPState::Failed));
        assert!(
            WPState::ForReview
                .transition(WPState::Failed, "WP01")
                .is_err()
        );
    }

    #[test]
    fn test_wp_invalid_pending_to_for_review() {
        assert!(!WPState::Pending.can_transition_to(&WPState::ForReview));
        assert!(
            WPState::Pending
                .transition(WPState::ForReview, "WP01")
                .is_err()
        );
    }

    #[test]
    fn test_wp_invalid_completed_to_for_review() {
        assert!(!WPState::Completed.can_transition_to(&WPState::ForReview));
        assert!(
            WPState::Completed
                .transition(WPState::ForReview, "WP01")
                .is_err()
        );
    }
}
