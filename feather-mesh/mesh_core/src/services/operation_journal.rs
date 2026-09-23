//! Minimal private operation journal for interactive recovery.
//!
//! It deliberately records no credential, provider request, or conversation
//! content. A pending intent means only that reconciliation is required; it is
//! never evidence that a mutation committed or failed.

use std::fs::{self, OpenOptions};
use std::io::Write;
use std::path::{Path, PathBuf};

use chrono::Utc;
use serde::{Deserialize, Serialize};
use thiserror::Error;

use super::interactive_operations::{OperationResult, OperationState, RecoveryIdentity};

#[derive(Debug, Error)]
pub enum JournalError {
    #[error("operation journal must be private: {0}")]
    Insecure(PathBuf),
    #[error("operation journal error: {0}")]
    Io(#[from] std::io::Error),
    #[error("operation journal serialization error: {0}")]
    Json(#[from] serde_json::Error),
}

pub type JournalResult<T> = Result<T, JournalError>;

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct JournalIntent {
    pub operation_id: String,
    pub operation: String,
    pub prepared_fingerprint: String,
    pub recorded_at: String,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub recovery: Option<RecoveryIdentity>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct JournalOutcome {
    #[serde(flatten)]
    pub intent: JournalIntent,
    pub state: OperationState,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub error_kind: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub message: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub result: Option<serde_json::Value>,
    pub completed_at: String,
}

#[derive(Debug, Clone)]
pub struct OperationJournal {
    root: PathBuf,
}

impl OperationJournal {
    /// Opens a caller-selected per-user/session directory. The project tree is
    /// intentionally not a valid journal location in the TUI controller.
    pub fn open(root: impl AsRef<Path>) -> JournalResult<Self> {
        let root = root.as_ref().to_path_buf();
        create_private_directory(&root)?;
        Ok(Self { root })
    }

    pub fn root(&self) -> &Path {
        &self.root
    }

    /// Persists and syncs intent before a mutation starts. If this fails, the
    /// controller must not call the mutation service.
    pub fn record_intent(
        &self,
        operation_id: impl Into<String>,
        operation: impl Into<String>,
        prepared_fingerprint: impl Into<String>,
    ) -> JournalResult<JournalIntent> {
        self.record_recoverable_intent(operation_id, operation, prepared_fingerprint, None)
    }

    pub fn record_recoverable_intent(
        &self,
        operation_id: impl Into<String>,
        operation: impl Into<String>,
        prepared_fingerprint: impl Into<String>,
        recovery: Option<RecoveryIdentity>,
    ) -> JournalResult<JournalIntent> {
        let operation_id = operation_id.into();
        validate_id(&operation_id)?;
        self.prune_completed(100)?;
        let intent = JournalIntent {
            operation_id,
            operation: operation.into(),
            prepared_fingerprint: prepared_fingerprint.into(),
            recorded_at: Utc::now().to_rfc3339(),
            recovery,
        };
        let path = self.pending_path(&intent.operation_id);
        let bytes = serde_json::to_vec_pretty(&intent)?;
        write_new_private_file(&path, &bytes)?;
        sync_directory(&self.root)?;
        Ok(intent)
    }

    /// Makes an authoritative result durable after the core operation returns.
    /// If this write fails after a committed result, callers must show
    /// `committed_with_followup_error` rather than claiming the operation was
    /// rolled back.
    pub fn record_outcome<T: Serialize>(
        &self,
        intent: JournalIntent,
        outcome: &OperationResult<T>,
    ) -> JournalResult<JournalOutcome> {
        validate_id(&intent.operation_id)?;
        if intent.operation_id != outcome.operation_id {
            return Err(JournalError::Insecure(self.root.clone()));
        }
        let record = JournalOutcome {
            intent,
            state: outcome.state,
            error_kind: outcome.error_kind.clone(),
            message: outcome.message.clone(),
            result: outcome
                .result
                .as_ref()
                .map(serde_json::to_value)
                .transpose()?,
            completed_at: Utc::now().to_rfc3339(),
        };
        let final_path = self.final_path(&record.intent.operation_id);
        let temporary = self
            .root
            .join(format!(".{}.tmp", record.intent.operation_id));
        write_new_private_file(&temporary, &serde_json::to_vec_pretty(&record)?)?;
        fs::rename(&temporary, &final_path)?;
        sync_directory(&self.root)?;
        if record.state != OperationState::OutcomeUnknown {
            fs::remove_file(self.pending_path(&record.intent.operation_id))?;
        }
        sync_directory(&self.root)?;
        Ok(record)
    }

    /// Returns unresolved intents. Reconciliation is intentionally a separate
    /// service/UI decision because receipts and manifests are operation-specific.
    pub fn pending(&self) -> JournalResult<Vec<JournalIntent>> {
        let mut intents: Vec<JournalIntent> = Vec::new();
        for entry in fs::read_dir(&self.root)? {
            let entry = entry?;
            let path = entry.path();
            if path
                .extension()
                .is_some_and(|extension| extension == "pending")
            {
                if intents.len() >= 1000 {
                    return Err(JournalError::Insecure(self.root.clone()));
                }
                ensure_private_permissions(&path)?;
                if fs::symlink_metadata(&path)?.file_type().is_symlink()
                    || fs::metadata(&path)?.len() > 1024 * 1024
                {
                    return Err(JournalError::Insecure(path));
                }
                intents.push(serde_json::from_slice(&fs::read(path)?)?);
            }
        }
        intents.sort_by(|left, right| left.operation_id.cmp(&right.operation_id));
        Ok(intents)
    }

    pub fn prune_completed(&self, keep: usize) -> JournalResult<()> {
        let mut completed = Vec::new();
        for entry in fs::read_dir(&self.root)? {
            let path = entry?.path();
            if path.extension().is_some_and(|e| e == "json")
                && !path.with_extension("pending").exists()
            {
                completed.push(path);
            }
        }
        completed.sort();
        let remove = completed.len().saturating_sub(keep);
        for path in completed.into_iter().take(remove) {
            fs::remove_file(path)?;
        }
        Ok(())
    }

    fn pending_path(&self, operation_id: &str) -> PathBuf {
        self.root.join(format!("{operation_id}.pending"))
    }

    fn final_path(&self, operation_id: &str) -> PathBuf {
        self.root.join(format!("{operation_id}.json"))
    }
}

fn write_new_private_file(path: &Path, bytes: &[u8]) -> JournalResult<()> {
    if bytes.len() > 1024 * 1024 {
        return Err(JournalError::Insecure(path.to_path_buf()));
    }
    let mut options = OpenOptions::new();
    options.write(true).create_new(true);
    #[cfg(unix)]
    {
        use std::os::unix::fs::OpenOptionsExt;
        options.mode(0o600);
    }
    let mut file = options.open(path)?;
    set_private_file_permissions(path)?;
    file.write_all(bytes)?;
    file.sync_all()?;
    Ok(())
}

fn create_private_directory(path: &Path) -> JournalResult<()> {
    if !path.exists() {
        #[cfg(unix)]
        {
            use std::os::unix::fs::DirBuilderExt;
            let mut builder = fs::DirBuilder::new();
            builder.recursive(true).mode(0o700).create(path)?;
        }
        #[cfg(not(unix))]
        fs::create_dir_all(path)?;
    }
    if !path.is_dir() || fs::symlink_metadata(path)?.file_type().is_symlink() {
        return Err(JournalError::Insecure(path.to_path_buf()));
    }
    ensure_private_permissions(path)
}

fn set_private_file_permissions(path: &Path) -> JournalResult<()> {
    #[cfg(unix)]
    {
        use std::os::unix::fs::PermissionsExt;
        fs::set_permissions(path, fs::Permissions::from_mode(0o600))?;
    }
    Ok(())
}

fn ensure_private_permissions(path: &Path) -> JournalResult<()> {
    #[cfg(unix)]
    {
        use std::os::unix::fs::PermissionsExt;
        if fs::metadata(path)?.permissions().mode() & 0o077 != 0 {
            return Err(JournalError::Insecure(path.to_path_buf()));
        }
    }
    Ok(())
}

fn validate_id(id: &str) -> JournalResult<()> {
    if id.is_empty()
        || id.len() > 128
        || !id.bytes().all(|b| b.is_ascii_alphanumeric() || b == b'-')
    {
        return Err(JournalError::Insecure(PathBuf::from(
            "invalid operation id",
        )));
    }
    Ok(())
}
fn sync_directory(path: &Path) -> JournalResult<()> {
    #[cfg(unix)]
    fs::File::open(path)?.sync_all()?;
    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn pending_intent_is_visible_until_an_outcome_is_durable() {
        let temp = tempfile::tempdir().unwrap();
        let journal = OperationJournal::open(temp.path().join("journal")).unwrap();
        let intent = journal.record_intent("op-1", "stage", "digest").unwrap();
        assert_eq!(journal.pending().unwrap().len(), 1);
        let outcome: OperationResult<()> = OperationResult {
            operation_id: "op-1".into(),
            state: OperationState::Committed,
            result: Some(()),
            error_kind: None,
            message: None,
        };
        journal.record_outcome(intent, &outcome).unwrap();
        assert!(journal.pending().unwrap().is_empty());
    }
}

#[cfg(test)]
mod recovery_tests {
    use super::*;
    #[test]
    fn unknown_results_remain_pending_and_ids_cannot_escape_the_private_directory() {
        let temp = tempfile::tempdir().unwrap();
        let journal = OperationJournal::open(temp.path().join("journal")).unwrap();
        assert!(
            journal
                .record_intent("oversized", "stage", "x".repeat(1024 * 1024))
                .is_err()
        );
        assert!(!journal.root().join("oversized.pending").exists());
        assert!(
            journal
                .record_intent("../escape", "stage", "digest")
                .is_err()
        );
        let intent = journal
            .record_intent("op-unknown", "stage", "digest")
            .unwrap();
        let result: OperationResult<()> = OperationResult {
            operation_id: "op-unknown".into(),
            state: OperationState::OutcomeUnknown,
            result: None,
            error_kind: Some("filesystem_error".into()),
            message: None,
        };
        journal.record_outcome(intent, &result).unwrap();
        assert_eq!(journal.pending().unwrap().len(), 1);
        assert!(
            journal
                .record_intent("op-unknown", "stage", "digest")
                .is_err()
        );
    }
    #[test]
    fn completed_retention_does_not_discard_unresolved_intents() {
        let temp = tempfile::tempdir().unwrap();
        let journal = OperationJournal::open(temp.path().join("journal")).unwrap();
        journal.record_intent("pending", "stage", "digest").unwrap();
        for n in 0..4 {
            let id = format!("op-{n}");
            let intent = journal.record_intent(&id, "stage", "digest").unwrap();
            let result = OperationResult {
                operation_id: id,
                state: OperationState::Committed,
                result: Some(()),
                error_kind: None,
                message: None,
            };
            journal.record_outcome(intent, &result).unwrap();
        }
        journal.prune_completed(2).unwrap();
        assert_eq!(journal.pending().unwrap().len(), 1);
        assert_eq!(fs::read_dir(journal.root()).unwrap().count(), 3);
    }
}
