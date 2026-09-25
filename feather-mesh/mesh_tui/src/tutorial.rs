use std::path::PathBuf;
use serde::{Serialize, Deserialize};

#[derive(Debug, Clone, Serialize, Deserialize)]
pub enum TutorialStep {
    Welcome,
    ProjectBinding,
    AssetSelection,
    MetadataValidation,
    PublishReview,
    AddPeer,
    DiscoverVersion,
    TablePreview,
    Summary,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct TutorialCheckpoint {
    pub project_root: Option<PathBuf>,
    pub serving_root: Option<PathBuf>,
    pub selected_assets: Vec<PathBuf>,
    pub selected_product: Option<String>,
    pub selected_version: Option<String>,
    pub step: TutorialStep,
}

impl Default for TutorialCheckpoint {
    fn default() -> Self {
        Self {
            project_root: None,
            serving_root: None,
            selected_assets: Vec::new(),
            selected_product: None,
            selected_version: None,
            step: TutorialStep::Welcome,
        }
    }
}

pub struct TutorialController {
    pub checkpoint: TutorialCheckpoint,
    pub checkpoint_path: PathBuf,
}

impl TutorialController {
    pub fn new(checkpoint_path: PathBuf) -> Self {
        let checkpoint = if checkpoint_path.exists() {
            std::fs::read_to_string(&checkpoint_path)
                .ok()
                .and_then(|s| serde_json::from_str(&s).ok())
                .unwrap_or_default()
        } else {
            TutorialCheckpoint::default()
        };

        // ensure checkpoint has a sensible project_root if unspecified
        let mut controller = Self { checkpoint, checkpoint_path };
        if controller.checkpoint.project_root.is_none() {
            controller.checkpoint.project_root = Some(std::env::current_dir().unwrap_or_else(|_| PathBuf::from(".")));
        }
        controller
    }

    pub fn save_checkpoint(&self) -> std::io::Result<()> {
        let s = serde_json::to_string_pretty(&self.checkpoint).map_err(|e| std::io::Error::new(std::io::ErrorKind::Other, e))?;
        std::fs::create_dir_all(
            self.checkpoint_path.parent().unwrap_or_else(|| std::path::Path::new(".")),
        )?;
        std::fs::write(&self.checkpoint_path, s)?;
        Ok(())
    }

    pub fn reset(&mut self) -> std::io::Result<()> {
        self.checkpoint = TutorialCheckpoint::default();
        let _ = std::fs::remove_file(&self.checkpoint_path);
        Ok(())
    }

    // Placeholder: called by TUI to advance to next step after verification
    pub fn advance(&mut self) {
        use TutorialStep::*;
        self.checkpoint.step = match self.checkpoint.step {
            Welcome => ProjectBinding,
            ProjectBinding => AssetSelection,
            AssetSelection => MetadataValidation,
            MetadataValidation => PublishReview,
            PublishReview => AddPeer,
            AddPeer => DiscoverVersion,
            DiscoverVersion => TablePreview,
            TablePreview => Summary,
            Summary => Summary,
        };
    }

    // Placeholder: perform a lightweight validation (no heavy IO here)
    pub fn validate_project(&self) -> Result<(), String> {
        if let Some(root) = &self.checkpoint.project_root {
            if root.exists() { Ok(()) } else { Err("project root does not exist".into()) }
        } else {
            Err("project root not set".into())
        }
    }
}

// Minimal README for the tutorial module (exposed for maintainers)
#[doc(hidden)]
pub const README: &str = "Tutorial controller stubs: provides checkpointing and step state.\n";
