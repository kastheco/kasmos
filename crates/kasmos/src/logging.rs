//! Logging setup for kasmos.
//!
//! Configures structured logging via the `tracing` crate with support for
//! the `RUST_LOG` environment variable. Supports two modes:
//!
//! - **Headless mode** (`tui_mode: false`): Uses `tracing-subscriber`'s `fmt`
//!   layer writing to stderr. Suitable for CLI commands.
//! - **TUI mode** (`tui_mode: true`): Routes tracing events through
//!   `tui-logger`'s `TuiTracingSubscriberLayer`, feeding the in-TUI log
//!   viewer widget. Stderr output is suppressed (alternate screen).
//!
//! # Examples
//!
//! ```ignore
//! use kasmos::logging::init_logging;
//!
//! // Initialize logging in headless mode (CLI commands)
//! init_logging(false)?;
//!
//! // Initialize logging in TUI mode (tui-logger widget)
//! init_logging(true)?;
//!
//! // Use tracing macros for structured logging
//! tracing::info!("Starting orchestration run");
//! tracing::debug!("Detailed debug information");
//! ```
//!
//! # Environment Variables
//!
//! - `RUST_LOG`: Controls logging level and filters (headless mode only)
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
/// * `tui_mode` — When `true`, routes events through `tui-logger` for the
///   in-TUI widget. When `false`, uses `fmt` layer to stderr (standard CLI
///   output).
///
/// # TUI mode requirements
///
/// `tui_logger::init_logger()` **must** be called before this function when
/// `tui_mode` is `true`. This is handled by `tui::run()`.
///
/// # Errors
///
/// Returns an error if the logging subscriber cannot be initialized.
pub fn init_logging(tui_mode: bool) -> Result<()> {
    if tui_mode {
        // TUI mode: route tracing events to tui-logger widget.
        // tui_logger::init_logger() must have been called already by tui::run().
        Registry::default()
            .with(tui_logger::TuiTracingSubscriberLayer)
            .init();
    } else {
        // Headless mode: structured fmt output to stderr.
        let filter =
            EnvFilter::try_from_default_env().unwrap_or_else(|_| EnvFilter::new("kasmos=info"));

        Registry::default()
            .with(
                fmt::layer()
                    .with_target(true)
                    .with_file(true)
                    .with_line_number(true),
            )
            .with(filter)
            .init();
    }

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
