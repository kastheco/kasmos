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

/// Fix rmcp/schemars schemas for MCP client compatibility.
///
/// Applies two post-processing passes to tool schemas:
///
/// 1. **Nullable fix**: rmcp v0.15 uses schemars' `AddNullable` transform which
///    converts `{"type": "null"}` into `{"const": null, "nullable": true}`. Some
///    MCP clients (e.g. OpenCode) reject `nullable` without `type`. This replaces
///    those with `{"type": "null"}`.
///
/// 2. **Integer format fix**: schemars 1.x emits `format: "uint64"` (and similar)
///    for Rust unsigned integer types. AJV (used by OpenCode and other MCP clients)
///    doesn't recognize these as built-in formats and logs warnings. This strips
///    unrecognized integer format annotations.
pub fn fix_tool_schemas(mut tool: rmcp::model::Tool) -> rmcp::model::Tool {
    tool.input_schema = std::sync::Arc::new(fix_schema_map((*tool.input_schema).clone()));
    if let Some(output) = tool.output_schema {
        tool.output_schema = Some(std::sync::Arc::new(fix_schema_map((*output).clone())));
    }
    tool
}

fn fix_schema_map(mut map: serde_json::Map<String, serde_json::Value>) -> serde_json::Map<String, serde_json::Value> {
    for value in map.values_mut() {
        fix_schema_value(value);
    }
    map
}

/// Integer format strings emitted by schemars that AJV doesn't recognize.
const UNRECOGNIZED_INT_FORMATS: &[&str] = &[
    "uint8", "uint16", "uint32", "uint64",
    "int8", "int16", "int32", "int64",
];

fn fix_schema_value(value: &mut serde_json::Value) {
    match value {
        serde_json::Value::Object(map) => {
            // Fix nullable: {const: null, nullable: true} -> {type: "null"}
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

            // Strip integer format annotations that AJV doesn't recognize
            if let Some(fmt) = map.get("format").and_then(|v| v.as_str()) {
                if UNRECOGNIZED_INT_FORMATS.contains(&fmt) {
                    map.remove("format");
                }
            }

            for v in map.values_mut() {
                fix_schema_value(v);
            }
        }
        serde_json::Value::Array(arr) => {
            for v in arr.iter_mut() {
                fix_schema_value(v);
            }
        }
        _ => {}
    }
}
