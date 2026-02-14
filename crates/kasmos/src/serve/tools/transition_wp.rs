use schemars::JsonSchema;
use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, Serialize, Deserialize, JsonSchema)]
#[serde(deny_unknown_fields)]
pub struct TransitionWpInput {
    pub feature_slug: String,
    pub wp_id: String,
    pub to_state: TransitionState,
    pub actor: String,
    pub reason: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize, JsonSchema)]
#[serde(rename_all = "snake_case")]
pub enum TransitionState {
    Pending,
    Active,
    ForReview,
    Done,
    Rework,
}

impl TransitionState {
    pub fn as_str(&self) -> &'static str {
        match self {
            Self::Pending => "pending",
            Self::Active => "active",
            Self::ForReview => "for_review",
            Self::Done => "done",
            Self::Rework => "rework",
        }
    }
}

#[derive(Debug, Clone, Serialize, Deserialize, JsonSchema)]
pub struct TransitionWpOutput {
    pub ok: bool,
    pub wp_id: String,
    pub from_state: String,
    pub to_state: String,
}
