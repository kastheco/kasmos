//! Hub TUI module — interactive project command center.
//!
//! Provides a ratatui-based TUI for browsing feature specs, launching
//! OpenCode agent panes, and starting implementation sessions.

pub mod scanner;

/// Run the hub TUI.
///
/// This is the entry point when `kasmos` is invoked with no subcommand.
/// Currently a placeholder — full implementation in WP03.
pub async fn run() -> anyhow::Result<()> {
    println!("Hub TUI placeholder — full implementation coming in WP03");
    Ok(())
}
