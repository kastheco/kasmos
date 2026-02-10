//! Unix signal handling for graceful shutdown.

/// Installs SIGINT/SIGTERM handlers that trigger shutdown.
///
/// First signal triggers graceful shutdown. Second SIGINT forces immediate exit.
pub fn setup_signal_handlers(
    shutdown_trigger: Box<dyn Fn() + Send + Sync>,
) -> tokio::task::JoinHandle<()> {
    tokio::spawn(async move {
        let mut sigint = tokio::signal::unix::signal(tokio::signal::unix::SignalKind::interrupt())
            .expect("Failed to install SIGINT handler");

        let mut sigterm = tokio::signal::unix::signal(tokio::signal::unix::SignalKind::terminate())
            .expect("Failed to install SIGTERM handler");

        // Wait for first signal
        tokio::select! {
            _ = sigint.recv() => {
                tracing::info!("Received SIGINT — initiating graceful shutdown");
            }
            _ = sigterm.recv() => {
                tracing::info!("Received SIGTERM — initiating graceful shutdown");
            }
        }

        // Trigger graceful shutdown
        shutdown_trigger();

        // Wait for second SIGINT (force exit)
        tokio::select! {
            _ = sigint.recv() => {
                tracing::error!("Received second SIGINT — forcing immediate exit");
                std::process::exit(130); // 128 + SIGINT(2)
            }
            _ = sigterm.recv() => {
                tracing::error!("Received second SIGTERM — forcing immediate exit");
                std::process::exit(143); // 128 + SIGTERM(15)
            }
        }
    })
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_shutdown_flag_works() {
        use std::sync::Arc;
        use std::sync::atomic::{AtomicBool, Ordering};

        let flag = Arc::new(AtomicBool::new(false));
        let flag_clone = flag.clone();

        let trigger: Box<dyn Fn() + Send + Sync> = Box::new(move || {
            flag_clone.store(true, Ordering::SeqCst);
        });

        trigger();
        assert!(flag.load(Ordering::SeqCst));
    }
}
