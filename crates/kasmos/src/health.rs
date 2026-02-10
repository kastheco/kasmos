//! Pane health monitoring with crash detection.

use crate::error::Result;
use std::collections::HashSet;
use std::time::Instant;
use tokio::sync::mpsc;
use tokio::time::{Duration, interval};

/// Event emitted when a monitored pane disappears.
#[derive(Debug, Clone)]
pub struct CrashEvent {
    pub wp_id: String,
    pub pane_name: String,
    pub detected_at: Instant,
}

/// Trait for querying live pane status from Zellij.
#[async_trait::async_trait]
pub trait PaneHealthChecker: Send + Sync {
    /// Returns names of all currently live panes.
    async fn list_live_panes(&self) -> Result<Vec<String>>;
}

/// Monitors registered panes and detects crashes via polling.
pub struct HealthMonitor {
    poll_interval: Duration,
    /// Maps pane_name -> wp_id for registered panes
    expected_panes: std::collections::HashMap<String, String>,
    crash_tx: mpsc::Sender<CrashEvent>,
}

impl HealthMonitor {
    pub fn new(poll_interval_secs: u64, crash_tx: mpsc::Sender<CrashEvent>) -> Self {
        Self {
            poll_interval: Duration::from_secs(poll_interval_secs),
            expected_panes: std::collections::HashMap::new(),
            crash_tx,
        }
    }

    /// Register a pane for health monitoring.
    pub fn register_pane(&mut self, wp_id: String, pane_name: String) {
        tracing::debug!(wp_id = %wp_id, pane_name = %pane_name, "Registered pane for health monitoring");
        self.expected_panes.insert(pane_name, wp_id);
    }

    /// Unregister a pane (completed/failed WPs).
    pub fn unregister_pane(&mut self, pane_name: &str) {
        tracing::debug!(pane_name = %pane_name, "Unregistered pane from health monitoring");
        self.expected_panes.remove(pane_name);
    }

    /// Run the health monitor loop. Returns when the cancel token fires.
    /// Uses a shared reference to expected_panes via Arc<RwLock> pattern
    /// but for simplicity, takes ownership and uses channels for register/unregister.
    pub async fn run<C: PaneHealthChecker>(
        mut self,
        checker: C,
        mut shutdown_rx: tokio::sync::watch::Receiver<bool>,
        mut register_rx: mpsc::Receiver<PaneRegistration>,
    ) {
        let mut ticker = interval(self.poll_interval);
        ticker.set_missed_tick_behavior(tokio::time::MissedTickBehavior::Skip);

        loop {
            tokio::select! {
                _ = ticker.tick() => {
                    self.poll_once(&checker).await;
                }
                Some(reg) = register_rx.recv() => {
                    match reg {
                        PaneRegistration::Register { wp_id, pane_name } => {
                            self.register_pane(wp_id, pane_name);
                        }
                        PaneRegistration::Unregister { pane_name } => {
                            self.unregister_pane(&pane_name);
                        }
                    }
                }
                _ = shutdown_rx.changed() => {
                    if *shutdown_rx.borrow() {
                        tracing::info!("Health monitor shutting down");
                        break;
                    }
                }
            }
        }
    }

    async fn poll_once<C: PaneHealthChecker>(&self, checker: &C) {
        // Early exit: nothing to monitor
        if self.expected_panes.is_empty() {
            return;
        }

        let live_panes = match checker.list_live_panes().await {
            Ok(panes) => panes,
            Err(e) => {
                // Continue on poll errors — don't crash the monitor
                tracing::warn!(error = %e, "Health check poll failed, will retry");
                return;
            }
        };

        let live_set: HashSet<&str> = live_panes.iter().map(|s| s.as_str()).collect();

        for (pane_name, wp_id) in &self.expected_panes {
            if !live_set.contains(pane_name.as_str()) {
                tracing::error!(wp_id = %wp_id, pane_name = %pane_name, "Crash detected: pane missing");
                let event = CrashEvent {
                    wp_id: wp_id.clone(),
                    pane_name: pane_name.clone(),
                    detected_at: Instant::now(),
                };
                // Best-effort send — if receiver dropped, monitor should stop
                if self.crash_tx.send(event).await.is_err() {
                    tracing::warn!("Crash event receiver dropped, stopping health monitor");
                    return;
                }
            }
        }
    }
}

/// Registration message for dynamic pane tracking.
#[derive(Debug)]
pub enum PaneRegistration {
    Register { wp_id: String, pane_name: String },
    Unregister { pane_name: String },
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::sync::Arc;
    use tokio::sync::Mutex;

    struct MockChecker {
        live_panes: Arc<Mutex<Vec<String>>>,
    }

    #[async_trait::async_trait]
    impl PaneHealthChecker for MockChecker {
        async fn list_live_panes(&self) -> Result<Vec<String>> {
            Ok(self.live_panes.lock().await.clone())
        }
    }

    #[tokio::test]
    async fn test_crash_detection() {
        let (crash_tx, mut crash_rx) = mpsc::channel(10);
        let mut monitor = HealthMonitor::new(1, crash_tx);
        monitor.register_pane("WP01".into(), "wp01-pane".into());

        // Checker returns empty — pane is missing
        let checker = MockChecker {
            live_panes: Arc::new(Mutex::new(vec![])),
        };

        monitor.poll_once(&checker).await;

        let event = crash_rx.try_recv().expect("should receive crash event");
        assert_eq!(event.wp_id, "WP01");
        assert_eq!(event.pane_name, "wp01-pane");
    }

    #[tokio::test]
    async fn test_no_crash_when_pane_exists() {
        let (crash_tx, mut crash_rx) = mpsc::channel(10);
        let mut monitor = HealthMonitor::new(1, crash_tx);
        monitor.register_pane("WP01".into(), "wp01-pane".into());

        let checker = MockChecker {
            live_panes: Arc::new(Mutex::new(vec!["wp01-pane".into()])),
        };

        monitor.poll_once(&checker).await;

        assert!(crash_rx.try_recv().is_err(), "should not detect crash");
    }

    #[tokio::test]
    async fn test_unregistered_pane_not_monitored() {
        let (crash_tx, mut crash_rx) = mpsc::channel(10);
        let mut monitor = HealthMonitor::new(1, crash_tx);
        monitor.register_pane("WP01".into(), "wp01-pane".into());
        monitor.unregister_pane("wp01-pane");

        let checker = MockChecker {
            live_panes: Arc::new(Mutex::new(vec![])),
        };

        monitor.poll_once(&checker).await;

        assert!(
            crash_rx.try_recv().is_err(),
            "unregistered pane should not trigger crash"
        );
    }

    #[tokio::test]
    async fn test_poll_error_continues() {
        struct FailingChecker;

        #[async_trait::async_trait]
        impl PaneHealthChecker for FailingChecker {
            async fn list_live_panes(&self) -> Result<Vec<String>> {
                Err(crate::KasmosError::Other(anyhow::anyhow!("poll failed")))
            }
        }

        let (crash_tx, mut crash_rx) = mpsc::channel(10);
        let mut monitor = HealthMonitor::new(1, crash_tx);
        monitor.register_pane("WP01".into(), "wp01-pane".into());

        // Should not panic, should continue
        monitor.poll_once(&FailingChecker).await;

        assert!(
            crash_rx.try_recv().is_err(),
            "should not emit crash on poll error"
        );
    }

    #[tokio::test]
    async fn test_empty_expected_skips_poll() {
        let (crash_tx, _crash_rx) = mpsc::channel(10);
        let monitor = HealthMonitor::new(1, crash_tx);

        struct PanicChecker;
        #[async_trait::async_trait]
        impl PaneHealthChecker for PanicChecker {
            async fn list_live_panes(&self) -> Result<Vec<String>> {
                panic!("should not be called");
            }
        }

        // Should not call checker at all
        monitor.poll_once(&PanicChecker).await;
    }
}
