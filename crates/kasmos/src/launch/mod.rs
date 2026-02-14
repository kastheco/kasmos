//! Launch flow: feature resolution, preflight, layout generation, session bootstrap.

pub mod detect;
pub mod layout;
pub mod session;

pub async fn run(_spec_prefix: Option<&str>) -> anyhow::Result<()> {
    todo!("Launch flow implementation in WP02/WP03")
}
