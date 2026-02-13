//! TUI event types and crossterm EventStream wrapper.
//!
//! Wraps crossterm's `EventStream` into a tokio-compatible async event source
//! that produces typed `TuiEvent` values for the TUI event loop.

use crossterm::event::{Event as CrosstermEvent, EventStream, KeyEvent, MouseEvent};
use futures_util::StreamExt;

/// Events produced by the terminal for TUI consumption.
#[derive(Debug)]
pub enum TuiEvent {
    /// A key was pressed.
    Key(KeyEvent),
    /// A mouse event occurred.
    Mouse(MouseEvent),
    /// The terminal was resized.
    Resize(u16, u16),
}

/// Wraps crossterm's `EventStream` to produce filtered `TuiEvent` values.
///
/// Only emits `KeyEventKind::Press` events to prevent double-firing on
/// terminals that report key release/repeat events.
pub struct EventHandler {
    stream: EventStream,
}

impl Default for EventHandler {
    fn default() -> Self {
        Self::new()
    }
}

impl EventHandler {
    /// Create a new event handler wrapping a fresh crossterm `EventStream`.
    pub fn new() -> Self {
        Self {
            stream: EventStream::new(),
        }
    }

    /// Wait for the next meaningful terminal event.
    ///
    /// Filters out:
    /// - Key release/repeat events (only press events are emitted)
    /// - Focus gained/lost events
    /// - Paste events
    ///
    /// Returns `None` if the event stream is exhausted.
    pub async fn next(&mut self) -> Option<TuiEvent> {
        loop {
            match self.stream.next().await? {
                Ok(CrosstermEvent::Key(key)) => {
                    if key.kind == crossterm::event::KeyEventKind::Press {
                        return Some(TuiEvent::Key(key));
                    }
                }
                Ok(CrosstermEvent::Mouse(mouse)) => {
                    return Some(TuiEvent::Mouse(mouse));
                }
                Ok(CrosstermEvent::Resize(w, h)) => {
                    return Some(TuiEvent::Resize(w, h));
                }
                Ok(_) => {
                    // Ignore FocusGained, FocusLost, Paste events
                }
                Err(_) => {
                    // Crossterm stream error — skip and continue
                }
            }
        }
    }
}
