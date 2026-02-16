//! MCP server: kasmos serve (stdio transport).

pub mod audit;
pub mod dashboard;
pub mod lock;
pub mod messages;
pub mod registry;
pub mod tools;

use crate::config::Config;
use crate::feature_arg::feature_dir_from_specs_root;
use crate::serve::audit::{AuditEntry, AuditWriter};
use crate::serve::tools::despawn_worker::{DespawnWorkerInput, DespawnWorkerOutput};
use crate::serve::tools::infer_feature::{InferFeatureInput, InferFeatureOutput};
use crate::serve::tools::list_features::{ListFeaturesInput, ListFeaturesOutput};
use crate::serve::tools::list_workers::{ListWorkersInput, ListWorkersOutput};
use crate::serve::tools::read_messages::{ReadMessagesInput, ReadMessagesOutput};
use crate::serve::tools::spawn_worker::{SpawnWorkerInput, SpawnWorkerOutput};
use crate::serve::tools::transition_wp::{TransitionWpInput, TransitionWpOutput};
use crate::serve::tools::wait_for_event::{WaitForEventInput, WaitForEventOutput};
use crate::serve::tools::workflow_status::{WorkflowStatusInput, WorkflowStatusOutput};
use anyhow::Context;
use rmcp::handler::server::tool::ToolRouter;
use rmcp::handler::server::wrapper::Parameters;
use rmcp::model::{ErrorData, ServerCapabilities, ServerInfo};
use rmcp::transport::io::stdio;
use rmcp::{Json, ServerHandler, ServiceExt, tool, tool_handler, tool_router};
use tokio::sync::{Mutex, RwLock};

use self::registry::WorkerRegistry;

#[derive(Debug)]
pub struct KasmosServer {
    pub(crate) config: Config,
    pub(crate) registry: std::sync::Arc<RwLock<WorkerRegistry>>,
    pub(crate) message_cursor: std::sync::Arc<RwLock<u64>>,
    pub(crate) feature_slug: Option<String>,
    pub(crate) audit: std::sync::Arc<Mutex<Option<AuditWriter>>>,
    tool_router: ToolRouter<Self>,
}

impl KasmosServer {
    pub fn new(config: Config) -> anyhow::Result<Self> {
        let feature_slug = infer_feature_from_specs_root(&config.paths.specs_root);
        let audit = if let Some(slug) = feature_slug.as_deref() {
            let feature_dir =
                feature_dir_from_specs_root(std::path::Path::new(&config.paths.specs_root), slug);
            Some(
                AuditWriter::new(&feature_dir, slug.to_string(), &config.audit)
                    .with_context(|| format!("failed to initialize audit writer for {slug}"))?,
            )
        } else {
            None
        };

        Ok(Self {
            config,
            registry: std::sync::Arc::new(RwLock::new(WorkerRegistry::new())),
            message_cursor: std::sync::Arc::new(RwLock::new(0)),
            feature_slug,
            audit: std::sync::Arc::new(Mutex::new(audit)),
            tool_router: Self::tool_router(),
        })
    }

    pub async fn emit_audit(&self, entry: AuditEntry) {
        let mut audit = self.audit.lock().await;

        if audit
            .as_ref()
            .is_some_and(|writer| writer.feature_slug() != entry.feature_slug.as_str())
        {
            *audit = None;
        }

        if audit.is_none() {
            let feature_dir = feature_dir_from_specs_root(
                std::path::Path::new(&self.config.paths.specs_root),
                &entry.feature_slug,
            );
            match AuditWriter::new(&feature_dir, entry.feature_slug.clone(), &self.config.audit) {
                Ok(writer) => *audit = Some(writer),
                Err(err) => {
                    tracing::warn!(error = %err, "failed to initialize audit writer");
                    return;
                }
            }
        }

        if let Some(writer) = audit.as_mut()
            && let Err(err) = writer.write_entry(&entry)
        {
            tracing::warn!(error = %err, "failed to write audit entry");
        }
    }

    pub async fn emit_audit_error(
        &self,
        feature_slug: &str,
        wp_id: Option<&str>,
        action: &str,
        summary: &str,
        details: serde_json::Value,
    ) {
        let mut entry = AuditEntry::new("kasmos-serve", action, feature_slug)
            .with_status("error")
            .with_summary(summary)
            .with_details(details);
        if let Some(wp_id) = wp_id {
            entry = entry.with_wp_id(wp_id);
        }
        self.emit_audit(entry).await;
    }
}

#[tool_router]
impl KasmosServer {
    #[tool(
        name = "spawn_worker",
        description = "Spawn a planner/coder/reviewer/release worker pane"
    )]
    async fn spawn_worker(
        &self,
        Parameters(input): Parameters<SpawnWorkerInput>,
    ) -> Result<Json<SpawnWorkerOutput>, ErrorData> {
        let output = match tools::spawn_worker::handle(self, input.clone()).await {
            Ok(output) => output,
            Err(err) => {
                self.emit_audit_error(
                    &input.feature_slug,
                    Some(&input.wp_id),
                    "spawn_worker",
                    "worker spawn failed",
                    serde_json::json!({"error": err.to_string()}),
                )
                .await;
                return Err(internal_error(err));
            }
        };
        Ok(Json(output))
    }

    #[tool(
        name = "despawn_worker",
        description = "Close a worker pane and remove it from registry"
    )]
    async fn despawn_worker(
        &self,
        Parameters(input): Parameters<DespawnWorkerInput>,
    ) -> Result<Json<DespawnWorkerOutput>, ErrorData> {
        let output = match tools::despawn_worker::handle(self, input.clone()).await {
            Ok(output) => output,
            Err(err) => {
                if let Some(feature_slug) = self.feature_slug.as_deref() {
                    self.emit_audit_error(
                        feature_slug,
                        Some(&input.wp_id),
                        "despawn_worker",
                        "worker despawn failed",
                        serde_json::json!({"error": err.to_string()}),
                    )
                    .await;
                }
                return Err(internal_error(err));
            }
        };
        Ok(Json(output))
    }

    #[tool(
        name = "list_workers",
        description = "List workers tracked by this manager instance"
    )]
    async fn list_workers(
        &self,
        Parameters(input): Parameters<ListWorkersInput>,
    ) -> Result<Json<ListWorkersOutput>, ErrorData> {
        let output = tools::list_workers::handle(self, input)
            .await
            .map_err(internal_error)?;
        Ok(Json(output))
    }

    #[tool(
        name = "read_messages",
        description = "Read and parse message-log pane events"
    )]
    async fn read_messages(
        &self,
        Parameters(input): Parameters<ReadMessagesInput>,
    ) -> Result<Json<ReadMessagesOutput>, ErrorData> {
        let output = tools::read_messages::handle(self, input)
            .await
            .map_err(internal_error)?;
        Ok(Json(output))
    }

    #[tool(
        name = "wait_for_event",
        description = "Block until matching event appears or timeout is reached"
    )]
    async fn wait_for_event(
        &self,
        Parameters(input): Parameters<WaitForEventInput>,
    ) -> Result<Json<WaitForEventOutput>, ErrorData> {
        let output = tools::wait_for_event::handle(self, input)
            .await
            .map_err(internal_error)?;
        Ok(Json(output))
    }

    #[tool(
        name = "workflow_status",
        description = "Return feature phase, wave status, and active lock metadata"
    )]
    async fn workflow_status(
        &self,
        Parameters(input): Parameters<WorkflowStatusInput>,
    ) -> Result<Json<WorkflowStatusOutput>, ErrorData> {
        let output = tools::workflow_status::handle(input, self)
            .await
            .map_err(internal_error)?;
        Ok(Json(output))
    }

    #[tool(
        name = "transition_wp",
        description = "Validate and apply WP lane transitions in task files"
    )]
    async fn transition_wp(
        &self,
        Parameters(input): Parameters<TransitionWpInput>,
    ) -> Result<Json<TransitionWpOutput>, ErrorData> {
        let output = tools::transition_wp::handle(input, self)
            .await
            .map_err(map_transition_error)?;
        Ok(Json(output))
    }

    #[tool(
        name = "list_features",
        description = "List known feature specs and artifact availability"
    )]
    async fn list_features(
        &self,
        Parameters(_input): Parameters<ListFeaturesInput>,
    ) -> Result<Json<ListFeaturesOutput>, ErrorData> {
        let output = tools::list_features::handle(&self.config)
            .await
            .map_err(internal_error)?;
        Ok(Json(output))
    }

    #[tool(
        name = "infer_feature",
        description = "Infer feature slug from arg, branch, and cwd context"
    )]
    async fn infer_feature(
        &self,
        Parameters(input): Parameters<InferFeatureInput>,
    ) -> Result<Json<InferFeatureOutput>, ErrorData> {
        let output = tools::infer_feature::handle(&self.config, input)
            .await
            .map_err(internal_error)?;
        Ok(Json(output))
    }
}

#[tool_handler]
impl ServerHandler for KasmosServer {
    fn get_info(&self) -> ServerInfo {
        ServerInfo {
            server_info: rmcp::model::Implementation {
                name: "kasmos".to_string(),
                title: None,
                version: env!("CARGO_PKG_VERSION").to_string(),
                description: None,
                icons: None,
                website_url: None,
            },
            instructions: Some(
                "Kasmos MCP server for orchestrating planner/coder/reviewer/release workers"
                    .to_string(),
            ),
            capabilities: ServerCapabilities::builder().enable_tools().build(),
            ..Default::default()
        }
    }
}

pub async fn run() -> anyhow::Result<()> {
    let config = Config::load().context("failed to load config for serve mode")?;
    let service = KasmosServer::new(config)?
        .serve(stdio())
        .await
        .context("failed to start MCP stdio server")?;
    let _quit_reason = service.waiting().await?;
    Ok(())
}

fn internal_error(err: anyhow::Error) -> ErrorData {
    ErrorData::internal_error(format!("INTERNAL_ERROR: {}", err), None)
}

fn map_transition_error(err: anyhow::Error) -> ErrorData {
    let message = err.to_string();
    if message.starts_with("TRANSITION_NOT_ALLOWED:") {
        return ErrorData::invalid_params(message, None);
    }
    internal_error(err)
}

fn infer_feature_from_specs_root(specs_root: &str) -> Option<String> {
    let candidate = std::path::Path::new(specs_root)
        .file_name()
        .and_then(|name| name.to_str())?;

    if looks_like_feature_slug(candidate) {
        Some(candidate.to_string())
    } else {
        None
    }
}

fn looks_like_feature_slug(value: &str) -> bool {
    let Some((prefix, rest)) = value.split_once('-') else {
        return false;
    };

    prefix.len() == 3 && prefix.chars().all(|ch| ch.is_ascii_digit()) && !rest.is_empty()
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::serve::registry::AgentRole;
    use rmcp::handler::server::tool::parse_json_object;

    #[test]
    fn server_registers_all_contract_tools() {
        let server = KasmosServer::new(Config::default()).expect("server init");
        let mut names = server
            .tool_router
            .list_all()
            .into_iter()
            .map(|tool| tool.name.to_string())
            .collect::<Vec<_>>();
        names.sort();

        assert_eq!(
            names,
            vec![
                "despawn_worker",
                "infer_feature",
                "list_features",
                "list_workers",
                "read_messages",
                "spawn_worker",
                "transition_wp",
                "wait_for_event",
                "workflow_status"
            ]
        );
    }

    #[test]
    fn spawn_worker_input_rejects_invalid_payloads() {
        let invalid = serde_json::json!({
            "wp_id": "WP04",
            "role": "coder",
            "prompt": "do work",
            "feature_slug": "011-feature",
            "unexpected": true
        })
        .as_object()
        .cloned()
        .expect("object");

        let err = parse_json_object::<SpawnWorkerInput>(invalid).expect_err("must fail");
        assert_eq!(err.code, rmcp::model::ErrorCode::INVALID_PARAMS);
    }

    #[tokio::test]
    async fn list_features_and_infer_feature_shapes_are_contract_compatible() {
        let tmp = tempfile::tempdir().expect("tempdir");
        let specs_root = tmp.path().join("kitty-specs");
        std::fs::create_dir_all(specs_root.join("011-alpha")).expect("create feature");
        std::fs::write(specs_root.join("011-alpha/spec.md"), "# spec").expect("write spec");
        std::fs::create_dir_all(specs_root.join("011-alpha/tasks")).expect("create tasks");
        std::fs::write(
            specs_root.join("011-alpha/tasks/WP01.md"),
            "---\nlane: planned\n---",
        )
        .expect("write task");

        let mut config = Config::default();
        config.paths.specs_root = specs_root.display().to_string();

        let features = crate::serve::tools::list_features::handle(&config)
            .await
            .expect("list features");
        assert!(features.ok);
        assert_eq!(features.features.len(), 1);
        assert_eq!(features.features[0].slug, "011-alpha");
        assert!(features.features[0].has_spec);
        assert!(features.features[0].has_tasks);

        let inferred = crate::serve::tools::infer_feature::handle(
            &config,
            InferFeatureInput {
                spec_prefix: Some("011".to_string()),
            },
        )
        .await
        .expect("infer feature");
        assert!(inferred.ok);
        assert!(matches!(
            inferred.source,
            crate::serve::tools::infer_feature::InferFeatureSource::Arg
        ));
        assert_eq!(inferred.feature_slug.as_deref(), Some("011-alpha"));
    }

    #[tokio::test]
    async fn worker_registry_tools_handle_valid_inputs() {
        let server = KasmosServer::new(Config::default()).expect("server init");

        let spawn = crate::serve::tools::spawn_worker::handle(
            &server,
            SpawnWorkerInput {
                wp_id: "WP04".to_string(),
                role: AgentRole::Coder,
                prompt: "implement".to_string(),
                feature_slug: "011-mcp-agent-swarm-orchestration".to_string(),
                worktree_path: None,
            },
        )
        .await
        .expect("spawn");
        assert!(spawn.ok);

        let workers =
            crate::serve::tools::list_workers::handle(&server, ListWorkersInput { status: None })
                .await
                .expect("list");
        assert_eq!(workers.workers.len(), 1);

        let despawn = crate::serve::tools::despawn_worker::handle(
            &server,
            DespawnWorkerInput {
                wp_id: "WP04".to_string(),
                role: AgentRole::Coder,
                reason: None,
            },
        )
        .await
        .expect("despawn");
        assert!(despawn.ok);
        assert!(despawn.removed);
    }

    #[test]
    fn feature_slug_inference_requires_feature_like_directory_name() {
        assert_eq!(infer_feature_from_specs_root("kitty-specs"), None);
        assert_eq!(
            infer_feature_from_specs_root("/tmp/kitty-specs/011-mcp-agent-swarm-orchestration"),
            Some("011-mcp-agent-swarm-orchestration".to_string())
        );
        assert_eq!(
            infer_feature_from_specs_root("/tmp/kitty-specs/feature"),
            None
        );
    }
}
