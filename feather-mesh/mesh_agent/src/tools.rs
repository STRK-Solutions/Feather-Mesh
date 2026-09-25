//! Typed `feam.agent.tools.v1` tools.
//!
//! Project roots, confirmation, and arbitrary output paths are deliberately
//! absent from provider-facing arguments. The executor injects the selected
//! project and returns only bounded, policy-filtered summaries.

use std::collections::{BTreeMap, BTreeSet};
use std::path::PathBuf;

use serde::de::DeserializeOwned;
use serde::{Deserialize, Serialize};

use mesh_core::peer::{Project, refresh, resolve};
use mesh_core::services::catalog_service::{CatalogQuery, query_catalog};
use mesh_core::services::interactive_operations::{
    PreparedPublication, PreparedStage, PreparedWithdrawal, prepare_stage, prepare_withdrawal,
};

use crate::provider::{ProviderToolCall, ToolSchema};

pub const AGENT_TOOLS_PROTOCOL_VERSION: &str = "feam.agent.tools.v1";
pub type ToolProposal = ProviderToolCall;

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(tag = "status", rename_all = "snake_case")]
pub enum ToolExecution {
    Clarification {
        question: String,
        choices: Vec<String>,
    },
    Read {
        tool: String,
        result: serde_json::Value,
    },
    AwaitingReview {
        tool: String,
        operation_id: String,
        review: serde_json::Value,
    },
    DraftPatch {
        handle: String,
        draft: serde_json::Value,
    },
    Rejected {
        kind: String,
        message: String,
    },
}

impl ToolExecution {
    pub fn model_summary(&self) -> serde_json::Value {
        match self {
            Self::Clarification { question, choices } => {
                serde_json::json!({"status":"clarification_required","question":question,"choices":choices})
            }
            Self::Read { tool, result } => serde_json::json!({
                "protocol": AGENT_TOOLS_PROTOCOL_VERSION,
                "tool": tool,
                "status": "ok",
                "result": result,
            }),
            Self::AwaitingReview {
                tool, operation_id, ..
            } => serde_json::json!({
                "protocol": AGENT_TOOLS_PROTOCOL_VERSION,
                "tool": tool,
                "status": "awaiting_user_review",
                "operation_id": operation_id,
            }),
            Self::DraftPatch { handle, .. } => {
                serde_json::json!({"status":"draft_patch_requires_local_validation", "handle":handle})
            }
            Self::Rejected { kind, message: _ } => serde_json::json!({
                "protocol": AGENT_TOOLS_PROTOCOL_VERSION,
                "status": "rejected",
                "error": {"kind": kind},
            }),
        }
    }
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub enum ConfirmableOperation {
    Publication(Box<PreparedPublication>),
    Stage(Box<PreparedStage>),
    Withdrawal(Box<PreparedWithdrawal>),
}

pub struct ToolExecutor {
    project_root: PathBuf,
    prepared: BTreeMap<String, ConfirmableOperation>,
    provider_call_ids: BTreeSet<String>,
    drafts: BTreeMap<String, serde_json::Value>,
    integrity_consent: BTreeSet<(String, String)>,
}

impl ToolExecutor {
    pub fn new(project_root: impl Into<PathBuf>) -> Self {
        Self {
            project_root: project_root.into(),
            prepared: BTreeMap::new(),
            provider_call_ids: BTreeSet::new(),
            drafts: BTreeMap::new(),
            integrity_consent: BTreeSet::new(),
        }
    }

    pub fn project_root(&self) -> &PathBuf {
        &self.project_root
    }

    pub fn reset_project(&mut self, project_root: impl Into<PathBuf>) {
        self.project_root = project_root.into();
        self.prepared.clear();
        self.provider_call_ids.clear();
        self.drafts.clear();
        self.integrity_consent.clear();
    }

    /// The UI calls these methods after a user selects local destination or
    /// withdrawal fields. They are not provider-callable tools.
    pub fn prepare_stage_for_review(
        &mut self,
        reference: impl Into<String>,
        version: impl Into<String>,
        destination: impl Into<PathBuf>,
        overwrite: bool,
    ) -> Result<String, String> {
        let prepared = prepare_stage(
            self.project_root.clone(),
            reference,
            version,
            destination,
            overwrite,
        )
        .map_err(|error| error.to_string())?;
        let id = prepared.operation_id.clone();
        self.prepared
            .insert(id.clone(), ConfirmableOperation::Stage(Box::new(prepared)));
        Ok(id)
    }

    pub fn prepare_withdrawal_for_review(
        &mut self,
        reference: impl Into<String>,
        version: impl Into<String>,
        reason: impl Into<String>,
    ) -> Result<String, String> {
        let prepared = prepare_withdrawal(self.project_root.clone(), reference, version, reason)
            .map_err(|error| error.to_string())?;
        let id = prepared.operation_id.clone();
        self.prepared.insert(
            id.clone(),
            ConfirmableOperation::Withdrawal(Box::new(prepared)),
        );
        Ok(id)
    }

    pub fn invalidate_handles(&mut self) {
        self.prepared.clear();
        self.drafts.clear();
        self.integrity_consent.clear();
    }
    pub fn register_operation(&mut self, operation: ConfirmableOperation) -> String {
        let id = match &operation {
            ConfirmableOperation::Publication(p) => &p.operation_id,
            ConfirmableOperation::Stage(p) => &p.operation_id,
            ConfirmableOperation::Withdrawal(p) => &p.operation_id,
        }
        .clone();
        if self.prepared.len() >= 16 {
            self.prepared.clear();
        }
        self.prepared.insert(id.clone(), operation);
        id
    }
    pub fn register_draft(&mut self, handle: String, draft: serde_json::Value) {
        if self.drafts.len() >= 16 {
            self.drafts.clear();
        }
        self.drafts.insert(handle, draft);
    }
    pub fn grant_integrity_consent(&mut self, reference: String, version: String) {
        self.integrity_consent.insert((reference, version));
    }
    pub fn local_handles(&self) -> serde_json::Value {
        serde_json::json!({"operations":self.prepared.iter().map(|(id,op)| serde_json::json!({"handle":id,"summary":review_summary(op)})).collect::<Vec<_>>(),"drafts":self.drafts.keys().collect::<Vec<_>>()})
    }

    /// Transfers a prepared mutation to the local confirmation controller. A
    /// provider sees only the opaque operation ID and never gets this value.
    pub fn take_prepared_operation(&mut self, operation_id: &str) -> Option<ConfirmableOperation> {
        self.prepared.remove(operation_id)
    }

    pub fn execute(&mut self, proposal: ToolProposal) -> ToolExecution {
        if self.provider_call_ids.len() >= 256 {
            return rejected("session_limit", "start a new session after 256 calls");
        }
        if proposal.id.len() > 256 || proposal.arguments.to_string().len() > 65536 {
            return rejected("bad_arguments", "oversized call");
        }
        if !self.provider_call_ids.insert(proposal.id.clone()) {
            return rejected(
                "duplicate_provider_call",
                "the provider call ID was already handled",
            );
        }
        match proposal.name.as_str() {
            "user.clarify" => {
                #[derive(Deserialize)]
                #[serde(deny_unknown_fields)]
                struct Clarify {
                    question: String,
                    #[serde(default)]
                    choices: Vec<String>,
                }
                match parse::<Clarify>(proposal.arguments) {
                    Ok(args)
                        if !args.question.trim().is_empty()
                            && args.question.len() <= 1024
                            && args.choices.len() <= 10
                            && args.choices.iter().all(|c| c.len() <= 256) =>
                    {
                        ToolExecution::Clarification {
                            question: args.question,
                            choices: args.choices,
                        }
                    }
                    Ok(_) => rejected("bad_arguments", "clarification exceeds bounds"),
                    Err(error) => error,
                }
            }
            "catalog.search" => self.catalog_search(proposal.arguments),
            "product.inspect" => self.product_inspect(proposal.arguments, false),
            "product.resolve" => self.product_inspect(proposal.arguments, true),
            "peers.status" => self.peers_status(proposal.arguments),
            "catalog.refresh" => self.catalog_refresh(proposal.arguments),
            "help.lookup" => self.help_lookup(proposal.arguments),
            "publication.validate" => self.publication_validate(proposal.arguments),
            "publication.publish" => {
                self.review_existing("publication.publish", proposal.arguments)
            }
            "product.stage" => self.review_existing("product.stage", proposal.arguments),
            "product.withdraw" => self.review_existing("product.withdraw", proposal.arguments),
            _ => rejected("unknown_tool", "tool is not part of feam.agent.tools.v1"),
        }
    }

    fn catalog_search(&self, arguments: serde_json::Value) -> ToolExecution {
        let query: CatalogQuery = match parse(arguments) {
            Ok(query) => query,
            Err(error) => return error,
        };
        let project = match Project::open(&self.project_root) {
            Ok(project) => project,
            Err(error) => return peer_error(error),
        };
        match query_catalog(&project, query) {
            Ok(page) => read(
                "catalog.search",
                serde_json::to_value(page).expect("catalog page serializes"),
            ),
            Err(error) => peer_error(error),
        }
    }

    fn product_inspect(&mut self, arguments: serde_json::Value, resolving: bool) -> ToolExecution {
        let args: ProductArgs = match parse(arguments) {
            Ok(args) => args,
            Err(error) => return error,
        };
        if args.verify_integrity
            && !self
                .integrity_consent
                .remove(&(args.reference.clone(), args.version.clone()))
        {
            return rejected(
                "integrity_consent_required",
                "full integrity verification can read every asset; require explicit local consent",
            );
        }
        let project = match Project::open(&self.project_root) {
            Ok(project) => project,
            Err(error) => return peer_error(error),
        };
        match resolve(
            &project,
            &args.reference,
            &args.version,
            None,
            resolving && args.verify_integrity,
        ) {
            Ok(product) => read(
                if resolving {
                    "product.resolve"
                } else {
                    "product.inspect"
                },
                permitted_product_summary(&product),
            ),
            Err(error) => peer_error(error),
        }
    }

    fn peers_status(&self, arguments: serde_json::Value) -> ToolExecution {
        if !arguments.is_null() && arguments != serde_json::json!({}) {
            return rejected("bad_arguments", "peers.status accepts no arguments");
        }
        let project = match Project::open(&self.project_root) {
            Ok(project) => project,
            Err(error) => return peer_error(error),
        };
        match query_catalog(
            &project,
            CatalogQuery {
                limit: 1,
                ..Default::default()
            },
        ) {
            Ok(page) => read(
                "peers.status",
                serde_json::json!({"coverage": page.coverage}),
            ),
            Err(error) => peer_error(error),
        }
    }

    fn catalog_refresh(&self, arguments: serde_json::Value) -> ToolExecution {
        if !arguments.is_null() && arguments != serde_json::json!({}) {
            return rejected("bad_arguments", "catalog.refresh accepts no arguments");
        }
        let project = match Project::open(&self.project_root) {
            Ok(project) => project,
            Err(error) => return peer_error(error),
        };
        match refresh(&project) {
            Ok(snapshot) => read(
                "catalog.refresh",
                serde_json::to_value(snapshot).expect("cache snapshot serializes"),
            ),
            Err(error) => peer_error(error),
        }
    }

    fn help_lookup(&self, arguments: serde_json::Value) -> ToolExecution {
        let args: HelpArgs = match parse(arguments) {
            Ok(args) => args,
            Err(error) => return error,
        };
        let excerpt = match args.topic.as_str() {
            "resolve" => {
                "Resolve requires an explicit product version and performs no copy. Use the SDK or CLI example from the local UI for direct reads."
            }
            "stage" => {
                "Staging copies only the pinned registered inventory after a local review. It cannot be confirmed by a model tool argument."
            }
            "publish" => {
                "Publication validates producer-supplied scientific, policy, lineage, contact, and inventory metadata before a manifest commit."
            }
            "withdraw" => {
                "Only the selected project's local namespace can withdraw an active version; provider bytes remain intact."
            }
            _ => {
                return rejected(
                    "unknown_help_topic",
                    "supported topics: resolve, stage, publish, withdraw",
                );
            }
        };
        read(
            "help.lookup",
            serde_json::json!({"topic": args.topic, "excerpt": excerpt}),
        )
    }

    fn publication_validate(&mut self, arguments: serde_json::Value) -> ToolExecution {
        #[derive(Deserialize)]
        #[serde(deny_unknown_fields)]
        struct Patch {
            handle: String,
            patch: serde_json::Map<String, serde_json::Value>,
        }
        let args: Patch = match parse(arguments) {
            Ok(args) => args,
            Err(error) => return error,
        };
        let Some(draft) = self.drafts.get(&args.handle) else {
            return rejected("unknown_handle", "select a local draft first");
        };
        let mut draft = draft.clone();
        for (key, value) in args.patch {
            if ![
                "name",
                "version",
                "description",
                "intended_use",
                "limitations",
                "owner_team",
                "producer",
                "contact",
                "usage_policy",
                "classification",
                "quality",
                "lineage",
                "table",
                "raster",
            ]
            .contains(&key.as_str())
            {
                return rejected(
                    "bad_arguments",
                    "patch may only edit scientific/policy metadata; assets and namespace are selected locally",
                );
            }
            draft[key] = value;
        }
        ToolExecution::DraftPatch {
            handle: args.handle,
            draft,
        }
    }

    fn review_existing(&self, tool: &str, arguments: serde_json::Value) -> ToolExecution {
        let args: HandleArgs = match parse(arguments) {
            Ok(args) => args,
            Err(error) => return error,
        };
        let Some(operation) = self.prepared.get(&args.handle) else {
            return rejected(
                "unknown_handle",
                "review handles are local to the selected session and expire on project change",
            );
        };
        let matches_tool = matches!(
            (tool, operation),
            ("publication.publish", ConfirmableOperation::Publication(_))
                | ("product.stage", ConfirmableOperation::Stage(_))
                | ("product.withdraw", ConfirmableOperation::Withdrawal(_))
        );
        if !matches_tool {
            return rejected(
                "handle_kind_mismatch",
                "review handle does not match this mutation tool",
            );
        }
        ToolExecution::AwaitingReview {
            tool: tool.into(),
            operation_id: args.handle,
            review: review_summary(operation),
        }
    }
}

#[derive(Debug, Deserialize)]
#[serde(deny_unknown_fields)]
struct ProductArgs {
    reference: String,
    version: String,
    #[serde(default)]
    verify_integrity: bool,
}

#[derive(Debug, Deserialize)]
#[serde(deny_unknown_fields)]
struct HelpArgs {
    topic: String,
}

#[derive(Debug, Deserialize)]
#[serde(deny_unknown_fields)]
struct HandleArgs {
    handle: String,
}

fn parse<T: DeserializeOwned>(arguments: serde_json::Value) -> Result<T, ToolExecution> {
    serde_json::from_value(arguments).map_err(|error| rejected("bad_arguments", error.to_string()))
}

fn read(tool: impl Into<String>, result: serde_json::Value) -> ToolExecution {
    ToolExecution::Read {
        tool: tool.into(),
        result,
    }
}

fn rejected(kind: impl Into<String>, message: impl Into<String>) -> ToolExecution {
    ToolExecution::Rejected {
        kind: kind.into(),
        message: message.into(),
    }
}

fn peer_error(error: mesh_core::peer::PeerError) -> ToolExecution {
    rejected(error.kind(), error.to_string())
}

fn permitted_product_summary(product: &mesh_core::peer::ResolvedProduct) -> serde_json::Value {
    serde_json::json!({
        "reference": format!("product://{}/{}", product.namespace, product.product_id),
        "version": product.version,
        "manifest_revision": product.manifest_revision,
        "data_kind": product.data_kind,
        "data_format": product.data_format,
        "description": product.description,
        "limitations": product.limitations,
        "usage_policy": product.usage_policy,
        "classification": product.classification,
        "quality": product.quality,
        "lineage": product.lineage,
        "assets": product.assets.iter().map(|asset| serde_json::json!({
            "id": asset.id, "role": asset.role, "media_type": asset.media_type, "size": asset.size
        })).collect::<Vec<_>>(),
        "freshness": product.freshness,
        "integrity_verified": product.integrity_verified,
    })
}

fn review_summary(operation: &ConfirmableOperation) -> serde_json::Value {
    match operation {
        ConfirmableOperation::Publication(prepared) => serde_json::json!({
            "tool": "publication.publish",
            "status": "locally_validated_awaiting_review",
            "operation_id": prepared.operation_id,
            "reference": format!("product://{}/{}", prepared.request.namespace, prepared.request.product_id),
            "version": prepared.request.version,
            "estimated_asset_bytes": prepared.estimated_asset_bytes,
            "expected_manifest_revision": prepared.expected_manifest_revision,
        }),
        ConfirmableOperation::Stage(prepared) => serde_json::json!({
            "tool": "product.stage",
            "status": "locally_prepared_awaiting_review",
            "operation_id": prepared.operation_id,
            "reference": prepared.reference,
            "version": prepared.version,
            "estimated_asset_bytes": prepared.estimated_asset_bytes,
            "destination": "selected locally; inspect in the review screen",
            "overwrite": prepared.overwrite,
        }),
        ConfirmableOperation::Withdrawal(prepared) => serde_json::json!({
            "tool": "product.withdraw",
            "status": "locally_prepared_awaiting_review",
            "operation_id": prepared.operation_id,
            "reference": prepared.reference,
            "version": prepared.version,
            "reason": prepared.reason,
        }),
    }
}

pub fn tool_schemas() -> Vec<ToolSchema> {
    let object = |properties: serde_json::Value, required: Vec<&str>| {
        serde_json::json!({
            "type": "object", "properties": properties, "required": required, "additionalProperties": false
        })
    };
    vec![
        ToolSchema {
            name: "user.clarify".into(),
            description: "Ask the local user to choose a missing product, exact version, destination, draft or scientific fact. Use this for ambiguous requests instead of guessing. Ends the turn awaiting a user response.".into(),
            parameters: object(serde_json::json!({"question":{"type":"string","maxLength":1024},"choices":{"type":"array","maxItems":10,"items":{"type":"string","maxLength":256}}}),vec!["question"]),
        },
        ToolSchema {
            name: "catalog.search".into(),
            description: "Search bounded registered catalog records.".into(),
            parameters: object(
                serde_json::json!({"text":{"type":"string"},"namespace":{"type":"string"},"limit":{"type":"integer","minimum":1,"maximum":100},"cursor":{"type":"string"},"data_kind":{"type":"string","enum":["table","raster"]},"data_format":{"type":"string","enum":["parquet","geotiff"]},"quality":{"type":"string","description":"Scientific maturity/quality, such as production, qualified or unverified."},"classification":{"type":"string","description":"Data visibility/sensitivity, such as public, internal or restricted; not scientific quality."},"owner_team":{"type":"string"}}),
                vec![],
            ),
        },
        ToolSchema {
            name: "product.inspect".into(),
            description: "Inspect one explicit registered product version.".into(),
            parameters: object(
                serde_json::json!({"reference":{"type":"string"},"version":{"type":"string"},"verify_integrity":{"type":"boolean","description":"Defaults to false for metadata-only inspection. True reads/hashes all registered bytes and requires prior local integrity consent."}}),
                vec!["reference", "version"],
            ),
        },
        ToolSchema {
            name: "product.resolve".into(),
            description: "Resolve one explicit registered version without copying it.".into(),
            parameters: object(
                serde_json::json!({"reference":{"type":"string"},"version":{"type":"string"},"verify_integrity":{"type":"boolean","description":"Defaults to false for metadata-only inspection. True reads/hashes all registered bytes and requires prior local integrity consent."}}),
                vec!["reference", "version"],
            ),
        },
        ToolSchema {
            name: "peers.status".into(),
            description: "Show configured peer coverage and errors.".into(),
            parameters: object(serde_json::json!({}), vec![]),
        },
        ToolSchema {
            name: "catalog.refresh".into(),
            description: "Refresh local peer observation status.".into(),
            parameters: object(serde_json::json!({}), vec![]),
        },
        ToolSchema {
            name: "help.lookup".into(),
            description: "Read a bounded local help excerpt.".into(),
            parameters: object(
                serde_json::json!({"topic":{"type":"string","enum":["resolve","stage","publish","withdraw"]}}),
                vec!["topic"],
            ),
        },
        ToolSchema {
            name: "publication.validate".into(),
            description: "Propose bounded scientific/policy edits to a locally selected draft. Returns the patch for local inspection/validation consent; does not publish or select assets. Use only user-supplied scientific facts.".into(),
            parameters: object(serde_json::json!({"handle":{"type":"string"},"patch":publication_patch_schema()}), vec!["handle","patch"]),
        },
        ToolSchema {
            name: "publication.publish".into(),
            description:
                "Open a local review using an operations handle whose tool is publication.publish. Its metadata and inventory have ALREADY passed local validation. Private fields are intentionally omitted; do not request or invent them or revalidate this operation handle as a draft. This only opens review; it cannot publish directly."
                    .into(),
            parameters: object(
                serde_json::json!({"handle":{"type":"string"}}),
                vec!["handle"],
            ),
        },
        ToolSchema {
            name: "product.stage".into(),
            description:
                "Open a local review using an operations handle whose tool is product.stage. Exact version, destination and overwrite choice are ALREADY selected locally; the private path is intentionally omitted. Use the existing handle without requesting the path again. This only opens review; it cannot copy directly."
                    .into(),
            parameters: object(
                serde_json::json!({"handle":{"type":"string"}}),
                vec!["handle"],
            ),
        },
        ToolSchema {
            name: "product.withdraw".into(),
            description:
                "Open a local review using an operations handle whose tool is product.withdraw. Exact version and reason are ALREADY selected locally; the private reason is intentionally omitted. No extra reason text is needed from the model. This only opens review; it cannot withdraw directly."
                    .into(),
            parameters: object(
                serde_json::json!({"handle":{"type":"string"}}),
                vec!["handle"],
            ),
        },
    ]
}

fn publication_patch_schema() -> serde_json::Value {
    let string_map = serde_json::json!({"type":"object","additionalProperties":{"type":"string"}});
    let mut properties = serde_json::Map::new();
    for field in [
        "name",
        "version",
        "description",
        "intended_use",
        "limitations",
        "owner_team",
        "producer",
        "contact",
        "usage_policy",
        "classification",
        "quality",
    ] {
        properties.insert(
            field.into(),
            serde_json::json!({"type":"string","maxLength":2048}),
        );
    }
    properties.insert("table".into(),serde_json::json!({"type":"object","additionalProperties":false,"properties":{"column_meanings":string_map,"column_units":string_map,"partition_columns":{"type":"array","items":{"type":"string"}}},"required":["column_meanings","column_units"]}));
    properties.insert("raster".into(),serde_json::json!({"type":"object","additionalProperties":false,"properties":{"datetime":{"type":["string","null"]},"start_datetime":{"type":"string"},"end_datetime":{"type":"string"},"bbox":{"type":"array","minItems":4,"maxItems":4,"items":{"type":"number"}},"semantics":string_map},"required":["datetime","bbox","semantics"],"oneOf":[{"properties":{"datetime":{"type":"string"}},"not":{"anyOf":[{"required":["start_datetime"]},{"required":["end_datetime"]}]}},{"properties":{"datetime":{"type":"null"}},"required":["start_datetime","end_datetime"]}]}));
    properties.insert("lineage".into(),serde_json::json!({"type":"array","items":{"type":"object","properties":{"product":{"type":"string"},"version":{"type":"string"}},"required":["product"],"additionalProperties":false}}));
    serde_json::json!({"type":"object","properties":properties,"additionalProperties":false})
}
