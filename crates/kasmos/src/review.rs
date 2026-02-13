//! Review policy and failure typing utilities.

use crate::types::WPState;
use serde::{Deserialize, Serialize};

/// Automation policy for work packages entering `for_review`.
#[derive(Debug, Clone, Copy, Default, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum ReviewAutomationPolicy {
    /// Never run automation; operator reviews manually.
    ManualOnly,
    /// Run automation and require manual approval for completion.
    #[default]
    AutoThenManualApprove,
    /// Run automation and mark done when automation succeeds.
    AutoAndMarkDone,
}

/// Typed review automation failure category.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum ReviewFailureType {
    CommandMissing,
    Timeout,
    NonZeroExit,
    ParserError,
}

/// Notification severity for review failures.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum ReviewFailureSeverity {
    Warn,
    Error,
}

/// Decision emitted for a `for_review` transition.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub struct ReviewPolicyDecision {
    /// Whether automation should run.
    pub run_automation: bool,
    /// Whether successful automation should auto-complete the WP.
    pub auto_mark_done: bool,
}

/// Executor for review policy decisions.
#[derive(Debug, Clone, Copy)]
pub struct ReviewPolicyExecutor {
    policy: ReviewAutomationPolicy,
}

impl ReviewPolicyExecutor {
    /// Create an executor for the given policy.
    pub fn new(policy: ReviewAutomationPolicy) -> Self {
        Self { policy }
    }

    /// Return the current policy.
    pub fn policy(&self) -> ReviewAutomationPolicy {
        self.policy
    }

    /// Evaluate actions required when a WP enters `for_review`.
    pub fn on_for_review_transition(&self) -> ReviewPolicyDecision {
        match self.policy {
            ReviewAutomationPolicy::ManualOnly => ReviewPolicyDecision {
                run_automation: false,
                auto_mark_done: false,
            },
            ReviewAutomationPolicy::AutoThenManualApprove => ReviewPolicyDecision {
                run_automation: true,
                auto_mark_done: false,
            },
            ReviewAutomationPolicy::AutoAndMarkDone => ReviewPolicyDecision {
                run_automation: true,
                auto_mark_done: true,
            },
        }
    }

    /// Apply post-review state behavior for successful automation.
    pub fn apply_post_review_success(&self, state: WPState) -> WPState {
        if state != WPState::ForReview {
            return state;
        }

        if self.on_for_review_transition().auto_mark_done {
            WPState::Completed
        } else {
            WPState::ForReview
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_manual_only_policy_decision() {
        let executor = ReviewPolicyExecutor::new(ReviewAutomationPolicy::ManualOnly);
        let decision = executor.on_for_review_transition();
        assert!(!decision.run_automation);
        assert!(!decision.auto_mark_done);
        assert_eq!(
            executor.apply_post_review_success(WPState::ForReview),
            WPState::ForReview
        );
    }

    #[test]
    fn test_auto_then_manual_policy_decision() {
        let executor = ReviewPolicyExecutor::new(ReviewAutomationPolicy::AutoThenManualApprove);
        let decision = executor.on_for_review_transition();
        assert!(decision.run_automation);
        assert!(!decision.auto_mark_done);
        assert_eq!(
            executor.apply_post_review_success(WPState::ForReview),
            WPState::ForReview
        );
    }

    #[test]
    fn test_auto_and_mark_done_policy_decision() {
        let executor = ReviewPolicyExecutor::new(ReviewAutomationPolicy::AutoAndMarkDone);
        let decision = executor.on_for_review_transition();
        assert!(decision.run_automation);
        assert!(decision.auto_mark_done);
        assert_eq!(
            executor.apply_post_review_success(WPState::ForReview),
            WPState::Completed
        );
    }

    #[test]
    fn test_non_for_review_state_unchanged() {
        let executor = ReviewPolicyExecutor::new(ReviewAutomationPolicy::AutoAndMarkDone);
        assert_eq!(
            executor.apply_post_review_success(WPState::Active),
            WPState::Active
        );
    }
}
