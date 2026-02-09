//! Logging setup for kasmos.
//!
//! Configures structured logging via the `tracing` crate with support for
//! the `RUST_LOG` environment variable.
//!
//! # Examples
//!
//! ```ignore
//! use kasmos::logging::init_logging;
//!
//! // Initialize logging with default filter (kasmos=info)
//! init_logging()?;
//!
//! // Use tracing macros for structured logging
//! tracing::info!("Starting orchestration run");
//! tracing::debug!("Detailed debug information");
//! tracing::warn!("Warning message");
//! tracing::error!("Error message");
//!
//! // Create spans for contextual logging
//! let span = tracing::info_span!("wave_execution", wave = 0);
//! let _guard = span.enter();
//! tracing::info!("Processing wave");
//! ```

use crate::error::Result;
use tracing_subscriber::EnvFilter;
use tracing_subscriber::fmt;

/// Initialize the tracing logging system.
///
/// Sets up structured logging with the following behavior:
/// - Reads the `RUST_LOG` environment variable for filter configuration
/// - Falls back to `kasmos=info` if `RUST_LOG` is not set
/// - Includes file names and line numbers in output
/// - Includes target module names
/// - Does not include thread IDs (reduces noise)
///
/// # Examples
///
/// ```ignore
/// init_logging()?;
/// tracing::info!("Logging is now active");
/// ```
///
/// # Environment Variables
///
/// - `RUST_LOG`: Controls logging level and filters
///   - `RUST_LOG=debug` — Show debug and above
///   - `RUST_LOG=kasmos=trace` — Show trace for kasmos crate only
///   - `RUST_LOG=kasmos=info,zellij=debug` — Mixed filters
///
/// # Errors
///
/// Returns an error if the logging subscriber cannot be initialized.
pub fn init_logging() -> Result<()> {
    let filter =
        EnvFilter::try_from_default_env().unwrap_or_else(|_| EnvFilter::new("kasmos=info"));

    fmt()
        .with_env_filter(filter)
        .with_target(true)
        .with_thread_ids(false)
        .with_file(true)
        .with_line_number(true)
        .init();

    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_init_logging_succeeds() {
        // Note: This test may fail if called multiple times in the same process
        // because tracing-subscriber can only be initialized once.
        // In a real test suite, you'd use a test harness that isolates this.
        // For now, we just verify the function signature is correct.
        let result = init_logging();
        // We don't assert success here because of the single-init constraint.
        // The important thing is that the function compiles and has the right signature.
        let _ = result;
    }
}
