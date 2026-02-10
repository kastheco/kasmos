//! Completion event detection for work packages.
//!
//! This module defines the CompletionEvent type that signals when a work package
//! has completed execution, allowing the wave engine to advance the orchestration.

use crate::types::CompletionMethod;

/// Represents a completion event for a work package.
///
/// Emitted by the completion detector when a work package finishes execution,
/// either successfully or with failure.
#[derive(Debug, Clone)]
pub struct CompletionEvent {
    /// ID of the work package that completed.
    pub wp_id: String,

    /// How the completion was detected.
    pub method: CompletionMethod,

    /// Whether the completion was successful.
    pub success: bool,
}

impl CompletionEvent {
    /// Create a new completion event.
    pub fn new(wp_id: String, method: CompletionMethod, success: bool) -> Self {
        Self {
            wp_id,
            method,
            success,
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_completion_event_creation() {
        let event = CompletionEvent::new("WP01".to_string(), CompletionMethod::AutoDetected, true);
        assert_eq!(event.wp_id, "WP01");
        assert_eq!(event.method, CompletionMethod::AutoDetected);
        assert!(event.success);
    }

    #[test]
    fn test_completion_event_failure() {
        let event = CompletionEvent::new("WP02".to_string(), CompletionMethod::FileMarker, false);
        assert_eq!(event.wp_id, "WP02");
        assert!(!event.success);
    }
}
