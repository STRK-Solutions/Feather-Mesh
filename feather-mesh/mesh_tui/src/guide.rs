use std::collections::VecDeque;

pub const CURRICULUM_ID: &str = "feam.guided-tutorial.v4";
pub const LESSON_COUNT: usize = 15;
pub const OBSERVATION_LIMIT: usize = 64;
const SUMMARY_LIMIT: usize = 512;

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum GuideCommand {
    Init,
    Project,
    Resolve,
    Filter,
    DraftNewTable,
    DraftNewRaster,
    DraftLoad,
    DraftSet,
    DraftSetAssets,
    DraftValidate,
    DraftSave,
    Export,
    Publish,
    Stage,
    Withdraw,
    Recover,
    AgentProfile,
    AgentDraft,
    IntegrityConsent,
    Usage,
    Help,
    Unknown,
}

impl GuideCommand {
    pub fn from_words(words: &[&str]) -> Self {
        match words {
            ["init", ..] => Self::Init,
            ["project", ..] => Self::Project,
            ["resolve", ..] => Self::Resolve,
            ["filter", ..] => Self::Filter,
            ["draft", "new", "table"] => Self::DraftNewTable,
            ["draft", "new", "raster"] => Self::DraftNewRaster,
            ["draft", "load", ..] => Self::DraftLoad,
            ["draft", "set", "/assets", ..] => Self::DraftSetAssets,
            ["draft", "set", ..] => Self::DraftSet,
            ["draft", "validate"] => Self::DraftValidate,
            ["draft", "save", ..] => Self::DraftSave,
            ["export", ..] => Self::Export,
            ["publish", ..] => Self::Publish,
            ["stage", ..] => Self::Stage,
            ["withdraw", ..] => Self::Withdraw,
            ["recover"] => Self::Recover,
            ["agent-profile", ..] => Self::AgentProfile,
            ["agent-draft"] => Self::AgentDraft,
            ["integrity-consent", ..] => Self::IntegrityConsent,
            ["usage"] => Self::Usage,
            ["help"] => Self::Help,
            _ => Self::Unknown,
        }
    }

    fn label(self) -> &'static str {
        match self {
            Self::Init => "project initialization",
            Self::Project => "project switch",
            Self::Resolve => "pinned resolve",
            Self::Filter => "catalog filter",
            Self::DraftNewTable => "new table draft",
            Self::DraftNewRaster => "new raster draft",
            Self::DraftLoad => "draft load",
            Self::DraftSet => "draft field edit",
            Self::DraftSetAssets => "explicit draft assets",
            Self::DraftValidate => "draft validation",
            Self::DraftSave => "reviewed draft save",
            Self::Export => "reviewed export",
            Self::Publish => "publication preparation",
            Self::Stage => "staging preparation",
            Self::Withdraw => "withdrawal preparation",
            Self::Recover => "recovery reconciliation",
            Self::AgentProfile => "assistant profile selection",
            Self::AgentDraft => "draft handle exposure",
            Self::IntegrityConsent => "one-use integrity consent",
            Self::Usage => "assistant usage view",
            Self::Help => "help command",
            Self::Unknown => "unknown command",
        }
    }
}

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum PageDirection {
    Next,
    Previous,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum ReviewKind {
    Inspection,
    Publication,
    Staging,
    Withdrawal,
    Export,
}

impl ReviewKind {
    pub fn label(self) -> &'static str {
        match self {
            Self::Inspection => "inspection",
            Self::Publication => "publication",
            Self::Staging => "staging",
            Self::Withdrawal => "withdrawal",
            Self::Export => "local save/export",
        }
    }
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub enum GuideAction {
    HelpOpened,
    SectionChanged(String),
    SelectionMoved,
    Scrolled,
    SearchOpened,
    SearchCompleted,
    PageAttempt(PageDirection),
    RefreshCompleted,
    ProductOpened,
    ExamplesGenerated,
    TablePreviewed,
    ResponseStopRequested,
    Command(GuideCommand),
    ReviewOpened(ReviewKind),
    ReviewDenied(ReviewKind),
    ReviewConfirmed(ReviewKind),
    OperationCommitted(ReviewKind),
    RecoveryCompleted,
}

impl GuideAction {
    fn label(&self) -> String {
        match self {
            Self::HelpOpened => "help opened".into(),
            Self::SectionChanged(section) => format!("selected {section} view"),
            Self::SelectionMoved => "selection moved".into(),
            Self::Scrolled => "view scrolled".into(),
            Self::SearchOpened => "search opened".into(),
            Self::SearchCompleted => "search completed".into(),
            Self::PageAttempt(PageDirection::Next) => "next-page attempt".into(),
            Self::PageAttempt(PageDirection::Previous) => "previous-page attempt".into(),
            Self::RefreshCompleted => "peer refresh completed".into(),
            Self::ProductOpened => "pinned product opened".into(),
            Self::ExamplesGenerated => "examples generated".into(),
            Self::TablePreviewed => "table metadata previewed".into(),
            Self::ResponseStopRequested => "assistant stop requested".into(),
            Self::Command(command) => command.label().into(),
            Self::ReviewOpened(kind) => format!("{} review opened", kind.label()),
            Self::ReviewDenied(kind) => format!("{} review denied", kind.label()),
            Self::ReviewConfirmed(kind) => format!("{} review confirmed", kind.label()),
            Self::OperationCommitted(kind) => format!("{} committed", kind.label()),
            Self::RecoveryCompleted => "recovery completed".into(),
        }
    }
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub enum ObservationResult {
    Success(String),
    Failure(String),
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct GuideObservation {
    pub action: GuideAction,
    pub result: ObservationResult,
}

impl GuideObservation {
    pub fn success(action: GuideAction, summary: impl Into<String>) -> Self {
        Self {
            action,
            result: ObservationResult::Success(bound(summary.into())),
        }
    }

    pub fn failure(action: GuideAction, error: impl Into<String>) -> Self {
        Self {
            action,
            result: ObservationResult::Failure(sanitize_error(&error.into())),
        }
    }

    pub fn command(words: &[&str], succeeded: bool, summary: &str) -> Self {
        let action = GuideAction::Command(GuideCommand::from_words(words));
        if succeeded {
            Self::success(action, summary)
        } else {
            Self::failure(action, summary)
        }
    }

    pub fn summary(&self) -> String {
        match &self.result {
            ObservationResult::Success(summary) => {
                format!("Observed {}: {summary}", self.action.label())
            }
            ObservationResult::Failure(error) => {
                format!("Observed {} failure: {error}", self.action.label())
            }
        }
    }
}

fn bound(value: String) -> String {
    value
        .chars()
        .filter(|c| !c.is_control() || c.is_whitespace())
        .take(SUMMARY_LIMIT)
        .collect()
}

/// Errors cross the same conceptual disclosure boundary as tool results. Keep
/// only a bounded category-style summary; never include paths, remote bodies,
/// credentials, environment values, draft values, destinations, or reasons.
fn sanitize_error(value: &str) -> String {
    let lower = value.to_ascii_lowercase();
    let category = if lower.contains("validation") || lower.contains("invalid") {
        "validation failed"
    } else if lower.contains("denied") || lower.contains("cancel") {
        "action was denied before commit"
    } else if lower.contains("not found") || lower.contains("unavailable") {
        "required local item was unavailable"
    } else if lower.contains("exists") || lower.contains("changed") {
        "reviewed state changed or already exists"
    } else {
        "local action failed; inspect the status area for private details"
    };
    bound(category.into())
}

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum Applicability {
    Always,
    CatalogData,
    TableSelection,
    DisposableDemoOnly,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum ExpectedEvent {
    Help,
    AnySection,
    Selection,
    Scroll,
    SearchOpen,
    SearchResult,
    Page(PageDirection),
    Refresh,
    ProductOpen,
    Examples,
    Preview,
    Command(GuideCommand),
    ReviewOpened(ReviewKind),
    ReviewDenied(ReviewKind),
    Committed(ReviewKind),
    Recovery,
}

impl ExpectedEvent {
    fn matches(self, action: &GuideAction) -> bool {
        match (self, action) {
            (Self::Help, GuideAction::HelpOpened)
            | (Self::AnySection, GuideAction::SectionChanged(_))
            | (Self::Selection, GuideAction::SelectionMoved)
            | (Self::Scroll, GuideAction::Scrolled)
            | (Self::SearchOpen, GuideAction::SearchOpened)
            | (Self::SearchResult, GuideAction::SearchCompleted)
            | (Self::Refresh, GuideAction::RefreshCompleted)
            | (Self::ProductOpen, GuideAction::ProductOpened)
            | (Self::Examples, GuideAction::ExamplesGenerated)
            | (Self::Recovery, GuideAction::RecoveryCompleted)
            | (Self::Preview, GuideAction::TablePreviewed) => true,
            (Self::Page(expected), GuideAction::PageAttempt(actual)) => expected == *actual,
            (Self::Command(expected), GuideAction::Command(actual)) => expected == *actual,
            (Self::ReviewOpened(expected), GuideAction::ReviewOpened(actual)) => {
                expected == *actual
            }
            (Self::ReviewDenied(expected), GuideAction::ReviewDenied(actual)) => {
                expected == *actual
            }
            (Self::Committed(expected), GuideAction::OperationCommitted(actual)) => {
                expected == *actual
            }
            _ => false,
        }
    }
}

#[derive(Debug, Clone, Copy)]
pub struct CurriculumStep {
    pub lesson: usize,
    pub lesson_title: &'static str,
    pub id: &'static str,
    pub title: &'static str,
    pub instruction: &'static str,
    pub expected: ExpectedEvent,
    pub success: &'static str,
    pub failure: &'static str,
    pub applicability: Applicability,
}

macro_rules! step {
    ($lesson:literal, $lesson_title:literal, $id:literal, $title:literal, $instruction:literal, $expected:expr, $success:literal, $app:expr) => {
        CurriculumStep {
            lesson: $lesson,
            lesson_title: $lesson_title,
            id: $id,
            title: $title,
            instruction: $instruction,
            expected: $expected,
            success: $success,
            failure: "Check the status message, fix the problem, and try again. A failed action does not finish the lesson.",
            applicability: $app,
        }
    };
}

/// The ordering and exact commands are application-owned. The model receives
/// one current step at a time and cannot add, remove, reorder, or satisfy it.
pub static CURRICULUM: &[CurriculumStep] = &[
    step!(
        1,
        "Screen and navigation",
        "orientation.help",
        "Open Help",
        "Press ?. Help lists every key and command. Press x whenever you want to skip one action.",
        ExpectedEvent::Help,
        "Help opened",
        Applicability::Always
    ),
    step!(
        1,
        "Screen and navigation",
        "orientation.tabs",
        "Change views",
        "Press Tab once. Tab moves through Catalog, Peers, Operations, Help, Teams, Cache, Lineage, and Assistant.",
        ExpectedEvent::AnySection,
        "another view opened",
        Applicability::Always
    ),
    step!(
        1,
        "Screen and navigation",
        "orientation.scroll",
        "Scroll a view",
        "Press PgDn or Fn+Down once. PgUp or Fn+Up scrolls back. If your terminal does not send that key, press x.",
        ExpectedEvent::Scroll,
        "the current view received a scroll action",
        Applicability::Always
    ),
    step!(
        2,
        "Search",
        "catalog.search-open",
        "Open search",
        "Press / to open the search box.",
        ExpectedEvent::SearchOpen,
        "catalog search opened",
        Applicability::Always
    ),
    step!(
        2,
        "Search",
        "catalog.search",
        "Search the catalog",
        "Type observations, then press Enter. Wait for the results.",
        ExpectedEvent::SearchResult,
        "the local catalog returned the observations search result",
        Applicability::CatalogData
    ),
    step!(
        2,
        "Search",
        "catalog.select",
        "Move the selection",
        "Press j or Down once. Use k or Up to move back.",
        ExpectedEvent::Selection,
        "the selected version changed",
        Applicability::CatalogData
    ),
    step!(
        2,
        "Search",
        "catalog.page",
        "Try the next page",
        "Press ]. Feather Mesh will say whether another page exists. [ goes back.",
        ExpectedEvent::Page(PageDirection::Next),
        "the page attempt reported whether a next page exists",
        Applicability::CatalogData
    ),
    step!(
        2,
        "Search",
        "catalog.refresh",
        "Refresh peers",
        "Press r and wait. This refreshes the peer catalog and Cache view.",
        ExpectedEvent::Refresh,
        "peer refresh completed",
        Applicability::Always
    ),
    step!(
        3,
        "Open and use a product",
        "catalog.open",
        "Open a product",
        "Press Enter. This opens the selected exact version.",
        ExpectedEvent::ProductOpen,
        "the selected pinned version resolved locally",
        Applicability::CatalogData
    ),
    step!(
        3,
        "Open and use a product",
        "product.resolve",
        "Resolve a version",
        "Run :resolve product://climate/observations v1. This finds the exact version. It does not copy data.",
        ExpectedEvent::Command(GuideCommand::Resolve),
        "the exact reference and version resolved locally",
        Applicability::CatalogData
    ),
    step!(
        3,
        "Open and use a product",
        "product.examples",
        "Show examples",
        "Press e. Feather Mesh shows CLI and Python examples for the selected product.",
        ExpectedEvent::Examples,
        "CLI and Python examples were shown",
        Applicability::TableSelection
    ),
    step!(
        3,
        "Open and use a product",
        "product.preview",
        "Preview the table",
        "Press p. You will see the table size and columns.",
        ExpectedEvent::Preview,
        "the resolver returned bounded table metadata",
        Applicability::TableSelection
    ),
    step!(
        4,
        "Filters and views",
        "views.filter",
        "Apply a filter",
        "Run :filter '{\"text\":\"observations\",\"owner_team\":\"Climate\",\"limit\":25}'.",
        ExpectedEvent::Command(GuideCommand::Filter),
        "the supported filter parsed and the bounded catalog reloaded",
        Applicability::CatalogData
    ),
    step!(
        4,
        "Filters and views",
        "views.filter-clear",
        "Clear the filter",
        "Run :filter '{\"limit\":25}'. This removes the text and team filters.",
        ExpectedEvent::Command(GuideCommand::Filter),
        "the filter was cleared",
        Applicability::CatalogData
    ),
    step!(
        4,
        "Filters and views",
        "views.visit",
        "Visit another view",
        "Press Tab once. Keep using Tab later to see Peers, Operations, Teams, Cache, Lineage, Help, and Assistant.",
        ExpectedEvent::AnySection,
        "another view opened",
        Applicability::Always
    ),
    step!(
        5,
        "Reviewed exports",
        "export.review",
        "Review an export",
        "Run :export guide-view.txt. Feather Mesh opens a local save review. Nothing is saved yet.",
        ExpectedEvent::ReviewOpened(ReviewKind::Export),
        "the export review opened without saving",
        Applicability::Always
    ),
    step!(
        5,
        "Reviewed exports",
        "export.deny",
        "Cancel the export",
        "Press n. The file is not saved. Run the command again with --overwrite only when replacing a file on purpose.",
        ExpectedEvent::ReviewDenied(ReviewKind::Export),
        "the export was denied before saving",
        Applicability::Always
    ),
    step!(
        6,
        "Draft templates",
        "draft.new-table",
        "Create a table draft",
        "Run :draft new table. This creates an empty table template.",
        ExpectedEvent::Command(GuideCommand::DraftNewTable),
        "a table draft template was created",
        Applicability::DisposableDemoOnly
    ),
    step!(
        6,
        "Draft templates",
        "draft.new-raster",
        "Create a raster draft",
        "Run :draft new raster. This shows the raster template.",
        ExpectedEvent::Command(GuideCommand::DraftNewRaster),
        "a raster draft template was created",
        Applicability::DisposableDemoOnly
    ),
    step!(
        6,
        "Draft templates",
        "draft.load",
        "Load the demo draft",
        "Run :draft load \"guided-product.json\". This ready draft uses real test metadata and one Parquet file.",
        ExpectedEvent::Command(GuideCommand::DraftLoad),
        "the ready demo product draft loaded",
        Applicability::DisposableDemoOnly
    ),
    step!(
        7,
        "Edit a draft",
        "draft.edit",
        "Edit one field",
        "Run :draft set /description '\"Guided tutorial table\"'. JSON Pointer selects the field; the last argument is JSON.",
        ExpectedEvent::Command(GuideCommand::DraftSet),
        "the description was changed",
        Applicability::DisposableDemoOnly
    ),
    step!(
        7,
        "Edit a draft",
        "draft.assets",
        "Set the asset list",
        "Run :draft set /assets '[{\"id\":\"data\",\"path\":\"datasets/guided-observations/v1/data.parquet\",\"role\":\"data\",\"media_type\":\"application/vnd.apache.parquet\"}]'. Assets must be real files under the serving directory.",
        ExpectedEvent::Command(GuideCommand::DraftSetAssets),
        "the explicit asset list was set",
        Applicability::DisposableDemoOnly
    ),
    step!(
        7,
        "Edit a draft",
        "draft.handle",
        "Share a draft handle",
        "Run :agent-draft. This gives the normal assistant a bounded handle to the selected draft. Guided mode still has no tools.",
        ExpectedEvent::Command(GuideCommand::AgentDraft),
        "a bounded draft handle was registered",
        Applicability::DisposableDemoOnly
    ),
    step!(
        8,
        "Validate and save a draft",
        "draft.validation-review",
        "Open validation review",
        "Run :draft validate. A review opens before Feather Mesh reads or hashes the file. Nothing is published yet.",
        ExpectedEvent::ReviewOpened(ReviewKind::Inspection),
        "the validation consent review opened with no publication or mutation",
        Applicability::DisposableDemoOnly
    ),
    step!(
        8,
        "Validate and save a draft",
        "draft.validation-confirm",
        "Validate the product",
        "Press y and wait. Feather Mesh checks the metadata and Parquet file. A publication review opens only if validation succeeds.",
        ExpectedEvent::ReviewOpened(ReviewKind::Publication),
        "validation succeeded and publication review opened without committing",
        Applicability::DisposableDemoOnly
    ),
    step!(
        8,
        "Validate and save a draft",
        "publication.deny",
        "Close the publication review",
        "Press n. Validation succeeded, but the product is not published.",
        ExpectedEvent::ReviewDenied(ReviewKind::Publication),
        "publication was denied and the product was not added",
        Applicability::DisposableDemoOnly
    ),
    step!(
        8,
        "Validate and save a draft",
        "draft.save",
        "Review a draft save",
        "Run :draft save guided-copy.json. This saves a working copy under .feam/drafts after review.",
        ExpectedEvent::ReviewOpened(ReviewKind::Export),
        "the draft save review opened",
        Applicability::DisposableDemoOnly
    ),
    step!(
        8,
        "Validate and save a draft",
        "draft.save-deny",
        "Cancel the draft save",
        "Press n. The working copy is not saved.",
        ExpectedEvent::ReviewDenied(ReviewKind::Export),
        "the draft save was denied",
        Applicability::DisposableDemoOnly
    ),
    step!(
        9,
        "Publish a product",
        "publication.retry",
        "Start publication",
        "Run :publish \"guided-product.json\". A file-check review opens first.",
        ExpectedEvent::ReviewOpened(ReviewKind::Inspection),
        "a fresh inspection review opened without retained approval",
        Applicability::DisposableDemoOnly
    ),
    step!(
        9,
        "Publish a product",
        "publication.revalidate",
        "Check the files",
        "Press y and wait. Feather Mesh checks the metadata and Parquet file, then opens the publication review.",
        ExpectedEvent::ReviewOpened(ReviewKind::Publication),
        "the fresh validation succeeded and opened a new publication review",
        Applicability::DisposableDemoOnly
    ),
    step!(
        9,
        "Publish a product",
        "publication.confirm",
        "Publish the product",
        "Press y and wait. The lesson finishes only after Feather Mesh reports a committed publication with an operation ID.",
        ExpectedEvent::Committed(ReviewKind::Publication),
        "the authoritative manifest reports committed publication",
        Applicability::DisposableDemoOnly
    ),
    step!(
        10,
        "Verify the publication",
        "publication.verify",
        "Find the new product",
        "Run :resolve product://climate/guided-observations v1. This proves the new product is registered in the manifest.",
        ExpectedEvent::Command(GuideCommand::Resolve),
        "the newly published product resolved from the authoritative manifest",
        Applicability::DisposableDemoOnly
    ),
    step!(
        11,
        "Stage a reviewed copy",
        "stage.review",
        "Open a staging review",
        "Run :stage product://climate/guided-observations v1 \"DESTINATION\". Replace DESTINATION with a new test folder. Nothing is copied yet.",
        ExpectedEvent::ReviewOpened(ReviewKind::Staging),
        "the staging review opened without copying",
        Applicability::DisposableDemoOnly
    ),
    step!(
        11,
        "Stage a reviewed copy",
        "stage.deny",
        "Deny staging once",
        "Press n. No files are copied and the approval is discarded.",
        ExpectedEvent::ReviewDenied(ReviewKind::Staging),
        "staging was denied before commit",
        Applicability::DisposableDemoOnly
    ),
    step!(
        11,
        "Stage a reviewed copy",
        "stage.review-again",
        "Open a fresh staging review",
        "Run the same :stage command again with the same empty test folder.",
        ExpectedEvent::ReviewOpened(ReviewKind::Staging),
        "a fresh staging review opened",
        Applicability::DisposableDemoOnly
    ),
    step!(
        11,
        "Stage a reviewed copy",
        "stage.confirm",
        "Confirm staging",
        "Press y and wait. Success requires a committed result and operation ID.",
        ExpectedEvent::Committed(ReviewKind::Staging),
        "the staged copy committed",
        Applicability::DisposableDemoOnly
    ),
    step!(
        12,
        "Withdraw a product",
        "withdraw.review",
        "Open a withdrawal review",
        "Run :withdraw product://climate/guided-observations v1 tutorial-complete. The reason stays private. Nothing changes yet.",
        ExpectedEvent::ReviewOpened(ReviewKind::Withdrawal),
        "the withdrawal review opened without changing lifecycle",
        Applicability::DisposableDemoOnly
    ),
    step!(
        12,
        "Withdraw a product",
        "withdraw.deny",
        "Deny withdrawal once",
        "Press n. The product stays active and the approval is discarded.",
        ExpectedEvent::ReviewDenied(ReviewKind::Withdrawal),
        "withdrawal was denied before commit",
        Applicability::DisposableDemoOnly
    ),
    step!(
        12,
        "Withdraw a product",
        "withdraw.review-again",
        "Open a fresh withdrawal review",
        "Run :withdraw product://climate/guided-observations v1 tutorial-complete again.",
        ExpectedEvent::ReviewOpened(ReviewKind::Withdrawal),
        "a fresh withdrawal review opened",
        Applicability::DisposableDemoOnly
    ),
    step!(
        12,
        "Withdraw a product",
        "withdraw.confirm",
        "Confirm withdrawal",
        "Press y and wait. Success requires a committed result and operation ID.",
        ExpectedEvent::Committed(ReviewKind::Withdrawal),
        "the product withdrawal committed",
        Applicability::DisposableDemoOnly
    ),
    step!(
        13,
        "Recovery and integrity",
        "controls.recover",
        "Check recovery",
        "Run :recover. It checks unfinished journal records. It never replays a write.",
        ExpectedEvent::Recovery,
        "recovery completed without replay",
        Applicability::Always
    ),
    step!(
        13,
        "Recovery and integrity",
        "controls.integrity",
        "Allow one integrity check",
        "Run :integrity-consent product://climate/observations v1. This allows one full read for this exact version.",
        ExpectedEvent::Command(GuideCommand::IntegrityConsent),
        "one exact-version integrity check was allowed",
        Applicability::CatalogData
    ),
    step!(
        14,
        "Assistant controls",
        "controls.usage",
        "Show usage",
        "Run :usage. Guided and normal assistant turns share this usage and cost list.",
        ExpectedEvent::Command(GuideCommand::Usage),
        "the shared usage list opened",
        Applicability::Always
    ),
    step!(
        14,
        "Assistant controls",
        "controls.help",
        "Review assistant controls",
        "Run :help. Use a to ask, s to stop a response, and :guide start or :guide stop to show or hide Guide. :guide skip skips one action. :tutorial start, stop, and skip are compatibility names.",
        ExpectedEvent::Command(GuideCommand::Help),
        "assistant and guide controls were reviewed",
        Applicability::Always
    ),
    step!(
        15,
        "Project and profile controls",
        "controls.context",
        "Review reset commands",
        "Run :help once more. After the guide, :init NAMESPACE [SERVING_DIR] creates a project, :project \"ROOT\" switches projects, and :agent-profile NAME|off changes the assistant. These reset the chat and Guide, so try them afterward. :guide restart restarts lessons; reset the disposable demo before repeating writes. q exits cleanly.",
        ExpectedEvent::Command(GuideCommand::Help),
        "project, profile, restart, and exit controls were reviewed",
        Applicability::Always
    ),
];

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum GuideRunState {
    Explaining,
    Waiting,
    Result,
    Complete,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum ObservationDisposition {
    Advanced,
    Retry,
    Unrelated,
    Completed,
}

#[derive(Debug)]
pub struct GuideState {
    active: bool,
    index: usize,
    state: GuideRunState,
    last_result: Option<String>,
    pending: VecDeque<GuideObservation>,
    dropped: usize,
    skipped: usize,
    ordinary_transition_pending: bool,
}

impl Default for GuideState {
    fn default() -> Self {
        Self {
            active: false,
            index: 0,
            state: GuideRunState::Waiting,
            last_result: None,
            pending: VecDeque::new(),
            dropped: 0,
            skipped: 0,
            ordinary_transition_pending: false,
        }
    }
}

impl GuideState {
    pub fn active(&self) -> bool {
        self.active
    }

    pub fn start(&mut self) {
        self.active = true;
        self.state = GuideRunState::Explaining;
        self.ordinary_transition_pending = false;
    }

    pub fn stop(&mut self) {
        if self.active {
            self.ordinary_transition_pending = true;
        }
        self.active = false;
    }

    pub fn restart(&mut self) {
        *self = Self::default();
        self.active = true;
        self.state = GuideRunState::Explaining;
    }

    pub fn reset(&mut self) {
        *self = Self::default();
    }

    pub fn current(&self) -> Option<&'static CurriculumStep> {
        CURRICULUM.get(self.index)
    }

    pub fn position(&self) -> (usize, usize) {
        (
            self.current()
                .map(|step| step.lesson)
                .unwrap_or(LESSON_COUNT),
            LESSON_COUNT,
        )
    }

    pub fn action_position(&self) -> (usize, usize) {
        let Some(current) = self.current() else {
            return (0, 0);
        };
        let action = CURRICULUM[..=self.index]
            .iter()
            .filter(|step| step.lesson == current.lesson)
            .count();
        let total = CURRICULUM
            .iter()
            .filter(|step| step.lesson == current.lesson)
            .count();
        (action, total)
    }

    pub fn state(&self) -> GuideRunState {
        self.state
    }

    /// Settles a guided model response. Completing the curriculum also turns
    /// guidance off so the next `a` request uses the ordinary tool-enabled
    /// assistant without creating a new conversation or usage ledger.
    pub fn finish_response(&mut self) -> bool {
        if self.index >= CURRICULUM.len() {
            self.state = GuideRunState::Complete;
            self.active = false;
            self.ordinary_transition_pending = true;
            true
        } else {
            self.state = GuideRunState::Waiting;
            false
        }
    }

    pub fn last_result(&self) -> Option<&str> {
        self.last_result.as_deref()
    }

    pub fn skipped(&self) -> usize {
        self.skipped
    }

    pub fn take_ordinary_transition(&mut self) -> bool {
        std::mem::take(&mut self.ordinary_transition_pending)
    }

    pub fn enqueue(&mut self, observation: GuideObservation) {
        if self.pending.len() == OBSERVATION_LIMIT {
            self.pending.pop_front();
            self.dropped += 1;
        }
        self.pending.push_back(observation);
    }

    pub fn pop(&mut self) -> Option<GuideObservation> {
        self.pending.pop_front()
    }

    pub fn skip(&mut self) -> Option<&'static CurriculumStep> {
        let skipped = self.current()?;
        self.pending.clear();
        self.last_result = Some(format!("Skipped lesson: {}", skipped.title));
        self.skipped += 1;
        self.index += 1;
        self.state = if self.index >= CURRICULUM.len() {
            GuideRunState::Complete
        } else {
            GuideRunState::Explaining
        };
        Some(skipped)
    }

    pub fn apply(&mut self, observation: &GuideObservation) -> ObservationDisposition {
        let Some(step) = self.current() else {
            self.state = GuideRunState::Complete;
            return ObservationDisposition::Completed;
        };
        self.last_result = Some(observation.summary());
        self.state = GuideRunState::Result;
        if matches!(observation.result, ObservationResult::Failure(_)) {
            return ObservationDisposition::Retry;
        }
        if !step.expected.matches(&observation.action) {
            return ObservationDisposition::Unrelated;
        }
        self.index += 1;
        if self.index >= CURRICULUM.len() {
            self.state = GuideRunState::Complete;
            ObservationDisposition::Completed
        } else {
            ObservationDisposition::Advanced
        }
    }

    pub fn initial_prompt(&self) -> String {
        let step = self.current().expect("active guide has a current step");
        let (action, actions) = self.action_position();
        format!(
            "Guided curriculum {CURRICULUM_ID}. Explain only this action, then wait. Use short sentences and simple words. Stay under 80 words. Do not add commands, paths, flags, results, or actions. Lesson {} of {}: {}. Action {} of {} [{}] {}. Instruction: {} Success criterion: {} Applicability: {:?}.",
            step.lesson,
            LESSON_COUNT,
            step.lesson_title,
            action,
            actions,
            step.id,
            step.title,
            step.instruction,
            step.success,
            step.applicability
        )
    }

    pub fn skip_prompt(&self, skipped: &CurriculumStep) -> String {
        if let Some(next) = self.current() {
            let (action, actions) = self.action_position();
            format!(
                "Guided curriculum {CURRICULUM_ID}. The user skipped [{}] {}. Do not call it completed and do not congratulate. Explain only the next action using short sentences and simple words, then wait. Stay under 80 words. Lesson {} of {}: {}. Action {} of {} [{}] {}. Instruction: {} Success criterion: {}.",
                skipped.id,
                skipped.title,
                next.lesson,
                LESSON_COUNT,
                next.lesson_title,
                action,
                actions,
                next.id,
                next.title,
                next.instruction,
                next.success
            )
        } else {
            format!(
                "Guided curriculum {CURRICULUM_ID}. The user skipped the final action [{}] {}. Do not call it completed. Briefly say the guide ended with {} skipped action(s). Mention that Help and the Assistant transcript are still available.",
                skipped.id, skipped.title, self.skipped
            )
        }
    }

    pub fn observation_prompt(
        &self,
        observation: &GuideObservation,
        disposition: ObservationDisposition,
    ) -> String {
        let observed = observation.summary();
        match disposition {
            ObservationDisposition::Advanced => {
                let next = self.current().unwrap();
                let (action, actions) = self.action_position();
                format!(
                    "Guided curriculum {CURRICULUM_ID}. The application verified success: {observed}. Briefly congratulate only for that verified result. Then explain only the next action using short sentences and simple words, and wait. Stay under 80 words. Do not add commands, paths, flags, results, or actions. Lesson {} of {}: {}. Action {} of {} [{}] {}. Instruction: {} Success criterion: {}.",
                    next.lesson,
                    LESSON_COUNT,
                    next.lesson_title,
                    action,
                    actions,
                    next.id,
                    next.title,
                    next.instruction,
                    next.success
                )
            }
            ObservationDisposition::Retry => {
                let step = self.current().unwrap();
                format!(
                    "Guided curriculum {CURRICULUM_ID}. The application observed failure: {observed}. Do not congratulate or advance. Use short sentences and simple words. Explain the retry without inventing details: {} Current instruction: {}",
                    step.failure, step.instruction
                )
            }
            ObservationDisposition::Unrelated => {
                let step = self.current().unwrap();
                format!(
                    "Guided curriculum {CURRICULUM_ID}. The application observed an unrelated action: {observed}. Use simple words. Acknowledge it briefly, do not treat it as success, then repeat only the current instruction: {}",
                    step.instruction
                )
            }
            ObservationDisposition::Completed => format!(
                "Guided curriculum {CURRICULUM_ID}. The application verified the final action: {observed}. In simple words, summarize the completed command tour. Mention that ? keeps the full command list and that the same Assistant is now available for normal questions with its bounded tools. Stay under 100 words. Do not claim any unobserved operation."
            ),
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn matching_success_advances_but_failure_and_unrelated_do_not_disclose_arguments() {
        assert_eq!(LESSON_COUNT, 15);
        assert_eq!(CURRICULUM.len(), 45);

        let mut guide = GuideState::default();
        guide.start();
        let failure = GuideObservation::failure(
            GuideAction::HelpOpened,
            "validation failed at /private/alice/project with API_KEY=secret",
        );
        assert_eq!(guide.apply(&failure), ObservationDisposition::Retry);
        assert_eq!(guide.position().0, 1);

        let unrelated = GuideObservation::command(
            &["stage", "product://climate/x", "v1", "/secret/destination"],
            true,
            "review prepared",
        );
        assert_eq!(guide.apply(&unrelated), ObservationDisposition::Unrelated);
        assert_eq!(guide.position().0, 1);
        let disclosed = unrelated.summary();
        assert!(!disclosed.contains("/secret/destination"));
        assert!(!disclosed.contains("product://climate/x"));

        let success = GuideObservation::success(GuideAction::HelpOpened, "Help view opened");
        assert_eq!(guide.apply(&success), ObservationDisposition::Advanced);
        assert_eq!(guide.position(), (1, 15));
        assert_eq!(guide.action_position(), (2, 3));
        assert!(!failure.summary().contains("/private/alice"));
        assert!(!failure.summary().contains("secret"));
    }

    #[test]
    fn skipping_marks_the_lesson_and_discards_old_observations() {
        let mut guide = GuideState::default();
        guide.start();
        guide.enqueue(GuideObservation::success(
            GuideAction::HelpOpened,
            "old result",
        ));

        let skipped = guide.skip().unwrap();
        assert_eq!(skipped.id, "orientation.help");
        assert_eq!(guide.position(), (1, 15));
        assert_eq!(guide.action_position(), (2, 3));
        assert_eq!(guide.skipped(), 1);
        assert_eq!(guide.last_result(), Some("Skipped lesson: Open Help"));
        assert!(guide.pop().is_none());

        let prompt = guide.skip_prompt(skipped);
        assert!(prompt.contains("The user skipped"));
        assert!(prompt.contains("Do not call it completed"));
        assert!(prompt.contains("Change views"));

        while guide.current().is_some() {
            guide.skip();
        }
        assert!(guide.active());
        assert!(guide.finish_response());
        assert!(!guide.active());
        assert!(guide.take_ordinary_transition());
        assert!(!guide.take_ordinary_transition());
    }

    #[test]
    fn curriculum_mentions_every_help_command() {
        let text = CURRICULUM
            .iter()
            .map(|step| step.instruction)
            .collect::<Vec<_>>()
            .join("\n");
        for command in [
            ":init",
            ":project",
            ":resolve",
            ":filter",
            ":publish",
            ":draft new table",
            ":draft new raster",
            ":draft load",
            ":draft set /description",
            ":draft set /assets",
            ":draft validate",
            ":draft save",
            ":export",
            ":stage",
            ":withdraw",
            ":recover",
            ":guide skip",
            ":guide restart",
            ":tutorial",
            ":agent-profile",
            ":agent-draft",
            ":integrity-consent",
            ":usage",
            ":help",
        ] {
            assert!(text.contains(command), "missing command {command}");
        }
    }
}
