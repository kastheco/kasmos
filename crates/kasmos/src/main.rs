use kasmos::init_logging;

fn main() -> kasmos::Result<()> {
    init_logging()?;
    tracing::info!("kasmos orchestrator initialized");
    Ok(())
}
