//! Review-bound operations for interactive callers.
//!
//! A prepared value is data for a review screen, not an authority token. Every
//! execution reopens the project and validates the bound configuration and
//! manifest revision before invoking the existing peer service.

use std::fs;
use std::path::{Path, PathBuf};
use std::sync::atomic::{AtomicU64, Ordering};
use std::time::{SystemTime, UNIX_EPOCH};

use serde::{Deserialize, Serialize};
use sha2::{Digest, Sha256};

use super::peer_access::{
    PeerError, PeerResult, Project, PublicationRequest, PublicationResponse, PublishedVersion,
    ResolvedAsset, ResolvedProduct, StageReceipt, absolute_path, configuration_fingerprint,
    local_manifest_revision, preflight_stage, publication_fingerprint, publish_reviewed,
    receipt_path, resolve, stage, withdraw_qualified,
};

static NEXT_OPERATION: AtomicU64 = AtomicU64::new(1);

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum OperationState {
    Committed,
    FailedBeforeCommit,
    CommittedWithFollowupError,
    OutcomeUnknown,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct OperationResult<T> {
    pub operation_id: String,
    pub state: OperationState,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub result: Option<T>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub error_kind: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub message: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct PreparedPublication {
    pub operation_id: String,
    pub project_root: PathBuf,
    pub configuration_fingerprint: String,
    pub expected_manifest_revision: u64,
    pub draft_fingerprint: String,
    pub request: PublicationRequest,
    pub estimated_asset_bytes: u64,
    pub validated: PublishedVersion,
    pub inventory_fingerprint: String,
    pub source_state: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct PreparedStage {
    pub operation_id: String,
    pub project_root: PathBuf,
    pub configuration_fingerprint: String,
    pub reference: String,
    pub version: String,
    pub expected_manifest_revision: u64,
    pub inventory_fingerprint: String,
    pub destination: PathBuf,
    pub overwrite: bool,
    pub estimated_asset_bytes: u64,
    pub receipt_path: PathBuf,
    pub assets: Vec<ResolvedAsset>,
    pub destination_state: String,
    pub receipt_state: String,
    pub destination_exists: bool,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct PreparedWithdrawal {
    pub operation_id: String,
    pub project_root: PathBuf,
    pub configuration_fingerprint: String,
    pub reference: String,
    pub version: String,
    pub reason: String,
    pub expected_manifest_revision: u64,
}

/// Minimal private recovery identity. It contains no conversation or authority
/// to retry. Reconciliation observes current state and never performs a write.
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(tag = "kind", rename_all = "snake_case")]
pub enum RecoveryIdentity {
    Publication {
        project_root: PathBuf,
        reference: String,
        version: String,
        revision: u64,
        inventory_fingerprint: String,
    },
    Stage {
        project_root: PathBuf,
        reference: String,
        version: String,
        revision: u64,
        destination: PathBuf,
        receipt_state: String,
        assets: Vec<ResolvedAsset>,
    },
    Withdrawal {
        project_root: PathBuf,
        reference: String,
        version: String,
        revision: u64,
        reason: String,
    },
}

impl From<&PreparedPublication> for RecoveryIdentity {
    fn from(p: &PreparedPublication) -> Self {
        Self::Publication {
            project_root: p.project_root.clone(),
            reference: super::peer_access::reference(&p.request.namespace, &p.request.product_id),
            version: p.request.version.clone(),
            revision: p.expected_manifest_revision,
            inventory_fingerprint: p.inventory_fingerprint.clone(),
        }
    }
}
impl From<&PreparedStage> for RecoveryIdentity {
    fn from(p: &PreparedStage) -> Self {
        Self::Stage {
            project_root: p.project_root.clone(),
            reference: p.reference.clone(),
            version: p.version.clone(),
            revision: p.expected_manifest_revision,
            destination: p.destination.clone(),
            receipt_state: p.receipt_state.clone(),
            assets: p.assets.clone(),
        }
    }
}
impl From<&PreparedWithdrawal> for RecoveryIdentity {
    fn from(p: &PreparedWithdrawal) -> Self {
        Self::Withdrawal {
            project_root: p.project_root.clone(),
            reference: p.reference.clone(),
            version: p.version.clone(),
            revision: p.expected_manifest_revision,
            reason: p.reason.clone(),
        }
    }
}

pub fn reconcile(identity: &RecoveryIdentity) -> PeerResult<OperationState> {
    use super::peer_access::{Lifecycle, list_registered_with_coverage};
    match identity {
        RecoveryIdentity::Stage {
            reference,
            version,
            revision,
            destination,
            receipt_state,
            assets,
            ..
        } => {
            let path = receipt_path(destination);
            if !path.is_file() || path_state(&path)? == *receipt_state || !destination.exists() {
                return Ok(OperationState::OutcomeUnknown);
            }
            let receipt: StageReceipt = serde_json::from_slice(&super::peer_access::read_bounded(
                &path,
                super::peer_access::MAX_MANIFEST_BYTES,
            )?)?;
            if receipt.reference != *reference
                || receipt.version != *version
                || receipt.manifest_revision != *revision
                || receipt.output_path != *destination
                || fingerprint(&receipt.assets)? != fingerprint(assets)?
            {
                return Ok(OperationState::OutcomeUnknown);
            }
            for asset in assets {
                let output = if assets.len() == 1 {
                    destination.clone()
                } else {
                    destination.join(&asset.id)
                };
                if fs::symlink_metadata(&output)?.file_type().is_symlink()
                    || fs::metadata(&output)?.len() != asset.size
                {
                    return Ok(OperationState::OutcomeUnknown);
                }
                if let Some(digest) = &asset.sha256 {
                    if super::peer_access::sha256_file(&output)? != *digest {
                        return Ok(OperationState::OutcomeUnknown);
                    }
                } else {
                    return Ok(OperationState::OutcomeUnknown);
                }
            }
            Ok(OperationState::Committed)
        }
        RecoveryIdentity::Publication {
            project_root,
            reference,
            version,
            revision,
            ..
        }
        | RecoveryIdentity::Withdrawal {
            project_root,
            reference,
            version,
            revision,
            ..
        } => {
            let project = Project::open(project_root)?;
            let (namespace, product_id) = super::peer_access::parse_reference(reference)?;
            if namespace != project.namespace() {
                return Ok(OperationState::OutcomeUnknown);
            }
            let listing = list_registered_with_coverage(&project)?;
            for (ns, product, current_revision) in listing.entries {
                if ns != namespace || product.id != product_id || current_revision <= *revision {
                    continue;
                }
                if let Some(record) = product.versions.iter().find(|v| v.version == *version) {
                    let matches = match identity {
                        RecoveryIdentity::Publication {
                            inventory_fingerprint,
                            ..
                        } => publication_fingerprint(record)? == *inventory_fingerprint,
                        RecoveryIdentity::Withdrawal { reason, .. } => {
                            record.lifecycle == Lifecycle::Withdrawn
                                && record.withdrawal_reason.as_ref() == Some(reason)
                        }
                        _ => false,
                    };
                    if matches {
                        return Ok(OperationState::Committed);
                    }
                }
            }
            Ok(OperationState::OutcomeUnknown)
        }
    }
}

pub fn prepare_publication(
    project_root: impl Into<PathBuf>,
    request: PublicationRequest,
) -> PeerResult<PreparedPublication> {
    let project_root = project_root.into();
    let project = Project::open(&project_root)?;
    let candidate = super::peer_access::validate_publication(&project, &request)?;
    let source_state = publication_source_state(&project, &request)?;
    Ok(PreparedPublication {
        operation_id: next_operation_id(),
        project_root: project.root().to_path_buf(),
        configuration_fingerprint: configuration_fingerprint(&project)?,
        expected_manifest_revision: local_manifest_revision(&project)?,
        draft_fingerprint: fingerprint(&request)?,
        estimated_asset_bytes: candidate.assets.iter().map(|asset| asset.size).sum(),
        inventory_fingerprint: publication_fingerprint(&candidate)?,
        validated: candidate,
        source_state,
        request,
    })
}

pub fn prepare_stage(
    project_root: impl Into<PathBuf>,
    reference: impl Into<String>,
    version: impl Into<String>,
    destination: impl Into<PathBuf>,
    overwrite: bool,
) -> PeerResult<PreparedStage> {
    let project_root = project_root.into();
    let project = Project::open(&project_root)?;
    let reference = reference.into();
    let version = version.into();
    let resolved = resolve(&project, &reference, &version, None, false)?;
    let destination = absolute_path(&destination.into())?;
    preflight_stage(&resolved, &destination, overwrite)?;
    let receipt_path = receipt_path(&destination);
    Ok(PreparedStage {
        operation_id: next_operation_id(),
        project_root: project.root().to_path_buf(),
        configuration_fingerprint: configuration_fingerprint(&project)?,
        reference,
        version,
        expected_manifest_revision: resolved.manifest_revision,
        inventory_fingerprint: inventory_fingerprint(&resolved)?,
        destination_state: path_state(&destination)?,
        receipt_state: path_state(&receipt_path)?,
        destination_exists: destination.exists(),
        destination,
        overwrite,
        estimated_asset_bytes: resolved.assets.iter().map(|asset| asset.size).sum(),
        receipt_path,
        assets: resolved.assets,
    })
}

pub fn prepare_withdrawal(
    project_root: impl Into<PathBuf>,
    reference: impl Into<String>,
    version: impl Into<String>,
    reason: impl Into<String>,
) -> PeerResult<PreparedWithdrawal> {
    let project_root = project_root.into();
    let project = Project::open(&project_root)?;
    let reference = reference.into();
    let (namespace, _) = super::peer_access::parse_reference(&reference)?;
    if namespace != project.namespace() {
        return Err(PeerError::Policy(format!(
            "only local namespace '{}' may withdraw a product",
            project.namespace()
        )));
    }
    let reason = reason.into();
    if reason.trim().is_empty() {
        return Err(PeerError::Validation {
            field: "reason".into(),
            message: "must not be blank".into(),
        });
    }
    let version = version.into();
    resolve(&project, &reference, &version, None, false)?;
    Ok(PreparedWithdrawal {
        operation_id: next_operation_id(),
        project_root: project.root().to_path_buf(),
        configuration_fingerprint: configuration_fingerprint(&project)?,
        reference,
        version,
        reason,
        expected_manifest_revision: local_manifest_revision(&project)?,
    })
}

pub fn execute_publication(prepared: &PreparedPublication) -> OperationResult<PublicationResponse> {
    execute_with_project(
        prepared.operation_id.clone(),
        &prepared.project_root,
        &prepared.configuration_fingerprint,
        |project| {
            if fingerprint(&prepared.request)? != prepared.draft_fingerprint
                || publication_source_state(project, &prepared.request)? != prepared.source_state
            {
                return Err(PeerError::Conflict(
                    "draft or source state changed; review again".into(),
                ));
            }
            publish_reviewed(
                project,
                &prepared.request,
                Some(prepared.expected_manifest_revision),
                Some(&prepared.inventory_fingerprint),
            )
        },
    )
}

pub fn execute_stage(prepared: &PreparedStage) -> OperationResult<StageReceipt> {
    let mut outcome = execute_with_project(
        prepared.operation_id.clone(),
        &prepared.project_root,
        &prepared.configuration_fingerprint,
        |project| {
            if path_state(&prepared.destination)? != prepared.destination_state
                || path_state(&prepared.receipt_path)? != prepared.receipt_state
            {
                return Err(PeerError::Conflict(
                    "destination or receipt changed; review again".into(),
                ));
            }
            let current = resolve(project, &prepared.reference, &prepared.version, None, false)?;
            if current.manifest_revision != prepared.expected_manifest_revision
                || inventory_fingerprint(&current)? != prepared.inventory_fingerprint
            {
                return Err(PeerError::Conflict(
                    "resolved inventory changed while staging awaited confirmation".into(),
                ));
            }
            stage(&current, &prepared.destination, prepared.overwrite)
        },
    );
    if outcome.state == OperationState::OutcomeUnknown
        && reconcile(&RecoveryIdentity::from(prepared))
            .is_ok_and(|s| s == OperationState::Committed)
    {
        outcome.state = OperationState::CommittedWithFollowupError;
    }
    outcome
}

pub fn execute_withdrawal(prepared: &PreparedWithdrawal) -> OperationResult<PublicationResponse> {
    execute_with_project(
        prepared.operation_id.clone(),
        &prepared.project_root,
        &prepared.configuration_fingerprint,
        |project| {
            withdraw_qualified(
                project,
                &prepared.reference,
                &prepared.version,
                &prepared.reason,
                Some(prepared.expected_manifest_revision),
            )
        },
    )
}

fn execute_with_project<T>(
    operation_id: String,
    root: &PathBuf,
    expected_configuration_fingerprint: &str,
    execute: impl FnOnce(&Project) -> PeerResult<T>,
) -> OperationResult<T> {
    let outcome = (|| -> PeerResult<T> {
        let project = Project::open(root)?;
        if configuration_fingerprint(&project)? != expected_configuration_fingerprint {
            return Err(PeerError::Conflict(
                "project configuration changed while operation awaited confirmation".into(),
            ));
        }
        execute(&project)
    })();
    match outcome {
        Ok(result) => OperationResult {
            operation_id,
            state: OperationState::Committed,
            result: Some(result),
            error_kind: None,
            message: None,
        },
        Err(error) => OperationResult {
            operation_id,
            state: outcome_state(&error),
            result: None,
            error_kind: Some(error.kind().into()),
            message: Some(error.to_string()),
        },
    }
}

fn outcome_state(error: &PeerError) -> OperationState {
    match error {
        // A filesystem failure can happen while an atomic rename/copy is in
        // flight. The caller must reconcile manifest/receipt state rather than
        // assuming rollback or automatically retrying.
        PeerError::Io(_) => OperationState::OutcomeUnknown,
        _ => OperationState::FailedBeforeCommit,
    }
}

fn publication_source_state(project: &Project, request: &PublicationRequest) -> PeerResult<String> {
    let root = project.serving_root()?;
    fingerprint(
        &request
            .assets
            .iter()
            .map(|a| path_state(&root.join(&a.path)))
            .collect::<PeerResult<Vec<_>>>()?,
    )
}

/// Bounded metadata snapshot; never hashes destination bytes or follows links.
/// Inodes, ctime and mtime bind same-size rewrites on Unix. A final filesystem
/// race remains between this check and the shared atomic copy/rename workflow.
pub fn path_state(path: &Path) -> PeerResult<String> {
    fn visit(path: &Path, rows: &mut Vec<String>) -> PeerResult<()> {
        if rows.len() >= 10_000 {
            return Err(PeerError::Policy(
                "review supports at most 10000 destination entries".into(),
            ));
        }
        let m = match fs::symlink_metadata(path) {
            Ok(m) => m,
            Err(e) if e.kind() == std::io::ErrorKind::NotFound => {
                rows.push(format!("{}:absent", path.display()));
                return Ok(());
            }
            Err(e) => return Err(e.into()),
        };
        rows.push(format!(
            "{}:{:?}:{}:{:?}:{:?}",
            path.display(),
            m.file_type(),
            m.len(),
            m.modified(),
            fs::canonicalize(path)
        ));
        #[cfg(unix)]
        {
            use std::os::unix::fs::MetadataExt;
            rows.push(format!(
                "{}:{}:{}:{}:{}:{}",
                m.dev(),
                m.ino(),
                m.ctime(),
                m.ctime_nsec(),
                m.mtime(),
                m.mtime_nsec()
            ));
        }
        if m.is_dir() {
            let mut children = Vec::new();
            for entry in fs::read_dir(path)? {
                if children.len() >= 10_000 {
                    return Err(PeerError::Policy(
                        "destination directory exceeds review limit".into(),
                    ));
                }
                children.push(entry?.path());
            }
            children.sort();
            for child in children {
                visit(&child, rows)?;
            }
        }
        Ok(())
    }
    let mut rows = vec![format!("parent:{:?}", path.parent().map(fs::canonicalize))];
    visit(path, &mut rows)?;
    fingerprint(&rows)
}

fn fingerprint<T: Serialize>(value: &T) -> PeerResult<String> {
    let bytes = serde_json::to_vec(value)?;
    let mut hash = Sha256::new();
    hash.update(bytes);
    Ok(format!("{:x}", hash.finalize()))
}

fn inventory_fingerprint(resolved: &ResolvedProduct) -> PeerResult<String> {
    fingerprint(&(
        &resolved.namespace,
        &resolved.product_id,
        &resolved.version,
        resolved.manifest_revision,
        &resolved.assets,
    ))
}

fn next_operation_id() -> String {
    let sequence = NEXT_OPERATION.fetch_add(1, Ordering::Relaxed);
    let nanos = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .map(|duration| duration.as_nanos())
        .unwrap_or_default();
    format!("feam-op-{nanos:x}-{sequence:x}")
}
