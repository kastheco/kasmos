//! MCP tool implementations for the kasmos server.

use anyhow::Result;
use std::path::{Path, PathBuf};

pub mod despawn_worker;
pub mod infer_feature;
pub mod list_features;
pub mod list_workers;
pub mod read_messages;
pub mod spawn_worker;
pub mod transition_wp;
pub mod wait_for_event;
pub mod workflow_status;

fn resolve_feature_dir(specs_root: &Path, feature_slug: &str) -> Result<PathBuf> {
    crate::feature_arg::resolve_existing_feature_dir(specs_root, feature_slug)
}

/// Fix rmcp/schemars nullable schemas for MCP client compatibility.
///
/// rmcp v0.15 uses schemars' `AddNullable` transform which converts
/// `{"type": "null"}` into `{"const": null, "nullable": true}`. Some MCP
/// clients (e.g. OpenCode) reject `nullable` without `type`. This
/// post-processes tool schemas to replace those with `{"type": "null"}`.
pub fn fix_tool_nullable(mut tool: rmcp::model::Tool) -> rmcp::model::Tool {
    tool.input_schema = std::sync::Arc::new(fix_nullable_map((*tool.input_schema).clone()));
    if let Some(output) = tool.output_schema {
        tool.output_schema = Some(std::sync::Arc::new(fix_nullable_map((*output).clone())));
    }
    tool
}

fn fix_nullable_map(mut map: serde_json::Map<String, serde_json::Value>) -> serde_json::Map<String, serde_json::Value> {
    for value in map.values_mut() {
        fix_nullable_value(value);
    }
    map
}

fn fix_nullable_value(value: &mut serde_json::Value) {
    match value {
        serde_json::Value::Object(map) => {
            if map.get("const") == Some(&serde_json::Value::Null)
                && map.contains_key("nullable")
                && !map.contains_key("type")
            {
                map.remove("const");
                map.remove("nullable");
                map.insert(
                    "type".to_string(),
                    serde_json::Value::String("null".to_string()),
                );
            }
            for v in map.values_mut() {
                fix_nullable_value(v);
            }
        }
        serde_json::Value::Array(arr) => {
            for v in arr.iter_mut() {
                fix_nullable_value(v);
            }
        }
        _ => {}
    }
}
