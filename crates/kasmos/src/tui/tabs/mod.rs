//! Per-tab rendering modules.
//!
//! Each tab's rendering logic lives in its own submodule, keeping `app.rs`
//! focused on state and lifecycle. The public `render_*` functions are called
//! from `App::render()`.

pub mod dashboard;
pub mod logs;
pub mod review;
