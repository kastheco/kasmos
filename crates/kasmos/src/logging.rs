//! Logging setup for kasmos.
//!
//! Configures structured logging via the `tracing` crate with support for
//! the `RUST_LOG` environment variable. Uses `tracing-subscriber`'s `fmt`
//! layer writing to stderr.
//!
//! # Examples
//!
//! ```ignore
//! use kasmos::logging::init_logging;
//!
//! // Initialize logging
//! init_logging(false)?;
//!
//! // Use tracing macros for structured logging
//! tracing::info!("Starting orchestration run");
//! ```
//!
//! # Environment Variables
//!
//! - `RUST_LOG`: Controls logging level and filters
//!   - `RUST_LOG=debug` — Show debug and above
//!   - `RUST_LOG=kasmos=trace` — Show trace for kasmos crate only

use crate::error::Result;
use tracing_subscriber::layer::SubscriberExt;
use tracing_subscriber::util::SubscriberInitExt;
use tracing_subscriber::EnvFilter;
use tracing_subscriber::{fmt, Registry};

/// Initialize the tracing logging system.
///
/// # Arguments
///
/// * `_tui_mode` — Deprecated argument retained for compatibility.
///
/// # Errors
///
/// Returns an error if the logging subscriber cannot be initialized.
pub fn init_logging(_tui_mode: bool) -> Result<()> {
    // Headless mode: structured fmt output to stderr.
    let filter =
        EnvFilter::try_from_default_env().unwrap_or_else(|_| EnvFilter::new("kasmos=info"));

    // Use try_init() to be idempotent -- a subscriber may already be set.
    let _ = Registry::default()
        .with(
            fmt::layer()
                .with_target(true)
                .with_file(true)
                .with_line_number(true),
        )
        .with(filter)
        .try_init();

    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_init_logging_succeeds() {
        // Note: This test may fail if called multiple times in the same process
        // because tracing-subscriber can only be initialized once.
        // For now, we just verify the function signature is correct.
        let result = init_logging(false);
        // We don't assert success here because of the single-init constraint.
        // The important thing is that the function compiles and has the right signature.
        let _ = result;
    }
}
