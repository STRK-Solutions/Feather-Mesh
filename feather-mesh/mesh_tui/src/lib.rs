//! Keyboard-first terminal UI for project-scoped Feather Mesh workflows.
//!
//! This crate calls the shared Rust services directly. It never parses CLI
//! output, initializes a legacy registry, or treats terminal text as trusted.

use std::io::{self, IsTerminal, Stdout};
use std::path::PathBuf;
use std::sync::mpsc::{self, TryRecvError};
use std::thread;
use std::time::{Duration, Instant};

use crossterm::event::{self, Event, KeyCode, KeyEventKind};
use crossterm::execute;
use crossterm::terminal::{
    EnterAlternateScreen, LeaveAlternateScreen, disable_raw_mode, enable_raw_mode,
};
use mesh_core::peer::{Project, refresh, resolve};
use mesh_core::services::interactive_operations::{RecoveryIdentity, path_state, reconcile};
mod controls;
use controls::{example, save_local_text, split_command};
use mesh_core::services::catalog_service::{
    CatalogEntry, CatalogPage, CatalogQuery, query_catalog,
};
use mesh_core::services::interactive_operations::{
    PreparedPublication, PreparedStage, PreparedWithdrawal, execute_publication, execute_stage,
    execute_withdrawal, prepare_publication, prepare_stage, prepare_withdrawal,
};
use mesh_core::services::operation_journal::OperationJournal;
use ratatui::backend::CrosstermBackend;
use ratatui::layout::{Constraint, Direction, Layout, Rect};
use ratatui::style::{Color, Modifier, Style};
use ratatui::text::{Line, Span};
use ratatui::widgets::{Block, Borders, Clear, List, ListItem, Paragraph, Wrap};
use ratatui::{Frame, Terminal};
use thiserror::Error;
pub mod tutorial;
use tutorial::TutorialController;

#[cfg(feature = "agent-hosted")]
use mesh_agent::{
    AgentConfig, AgentHarness, AgentProfile, AgentRun, CancellationToken, ConfirmableOperation,
    RouterProvider,
};

pub const TUI_FEATURE_GUIDANCE: &str = "rebuild with `--features tui` to enable `feam tui`";

#[derive(Debug, Clone)]
pub struct TuiOptions {
    pub project_root: PathBuf,
    /// `None` means manual mode. Hosted configuration is deliberately outside
    /// the project and is loaded only by the `agent-hosted` feature.
    pub agent_profile: Option<String>,
}

#[derive(Debug, Error)]
pub enum TuiError {
    #[error(
        "feam tui needs an interactive terminal; use existing CLI commands for non-terminal input/output"
    )]
    NotTerminal,
    #[error(transparent)]
    Peer(#[from] mesh_core::peer::PeerError),
    #[error(transparent)]
    Journal(#[from] mesh_core::services::operation_journal::JournalError),
    #[error(transparent)]
    Io(#[from] io::Error),
}

pub fn run(mut options: TuiOptions) -> Result<(), TuiError> {
    options.project_root = std::path::absolute(&options.project_root)?;
    if !io::stdin().is_terminal() || !io::stdout().is_terminal() {
        return Err(TuiError::NotTerminal);
    }
    // A locked-down environment can forbid the normal user state directory.
    // Browsing remains useful there; retain a stable private fallback journal.
    let journal = OperationJournal::open(default_journal_root())
        .or_else(|_| OperationJournal::open(fallback_journal_root()))?;
    if journal.root().starts_with(&options.project_root) {
        return Err(TuiError::Journal(
            mesh_core::services::operation_journal::JournalError::Insecure(
                journal.root().to_path_buf(),
            ),
        ));
    }
    let mut app = App::new(options, journal);
    app.recover();
    let mut terminal = TerminalGuard::enter()?;
    let tick = Duration::from_millis(25);
    let terminating = std::sync::Arc::new(std::sync::atomic::AtomicBool::new(false));
    let mut signals = Vec::new();
    let mut signal_consts = Vec::new();
    signal_consts.push(signal_hook::consts::SIGINT);
    signal_consts.push(signal_hook::consts::SIGTERM);
    #[cfg(unix)]
    {
        signal_consts.push(signal_hook::consts::SIGHUP);
    }
    for sig in signal_consts {
        signals.push(signal_hook::flag::register(sig, terminating.clone())?);
    }
    loop {
        app.poll_core();
        #[cfg(feature = "agent-hosted")]
        app.poll_agent();
        if terminating.load(std::sync::atomic::Ordering::Relaxed) && app.request_quit() {
            break;
        }
        if app.quit_when_idle && app.pending.is_none() {
            break;
        }
        terminal.terminal.draw(|frame| render(frame, &app))?;
        if event::poll(tick)?
            && let Event::Key(key) = event::read()?
            && key.kind == KeyEventKind::Press
            && app.handle_key(
                if key
                    .modifiers
                    .contains(crossterm::event::KeyModifiers::CONTROL)
                    && key.code == KeyCode::Char('c')
                {
                    KeyCode::Esc
                } else {
                    key.code
                },
            )
        {
            break;
        }
    }
    for id in signals {
        signal_hook::low_level::unregister(id);
    }
    Ok(())
}

struct TerminalGuard {
    terminal: Terminal<CrosstermBackend<Stdout>>,
}

impl TerminalGuard {
    fn enter() -> Result<Self, io::Error> {
        enable_raw_mode()?;
        let mut stdout = io::stdout();
        if let Err(error) = execute!(stdout, EnterAlternateScreen) {
            let _ = disable_raw_mode();
            return Err(error);
        }
        let backend = CrosstermBackend::new(stdout);
        let mut terminal = match Terminal::new(backend) {
            Ok(t) => t,
            Err(e) => {
                let _ = execute!(io::stdout(), LeaveAlternateScreen);
                let _ = disable_raw_mode();
                return Err(e);
            }
        };
        if let Err(error) = terminal.clear() {
            let _ = execute!(terminal.backend_mut(), LeaveAlternateScreen);
            let _ = disable_raw_mode();
            return Err(error);
        }
        Ok(Self { terminal })
    }
}

impl Drop for TerminalGuard {
    fn drop(&mut self) {
        let _ = disable_raw_mode();
        let _ = execute!(self.terminal.backend_mut(), LeaveAlternateScreen);
        let _ = self.terminal.show_cursor();
    }
}

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
enum Section {
    Catalog,
    Peers,
    Operations,
    Help,
    Teams,
    Cache,
    Lineage,
}

impl Section {
    fn next(self) -> Self {
        match self {
            Self::Catalog => Self::Peers,
            Self::Peers => Self::Operations,
            Self::Operations => Self::Help,
            Self::Help => Self::Teams,
            Self::Teams => Self::Cache,
            Self::Cache => Self::Lineage,
            Self::Lineage => Self::Catalog,
        }
    }

    fn title(self) -> &'static str {
        match self {
            Self::Catalog => "Catalog",
            Self::Peers => "Peers",
            Self::Operations => "Operations",
            Self::Help => "Help",
            Self::Teams => "Teams",
            Self::Cache => "Cache",
            Self::Lineage => "Lineage",
        }
    }
}

#[derive(Debug, Clone)]
enum InputMode {
    None,
    Search(String),
    Command(String),
    #[cfg(feature = "agent-hosted")]
    Agent(String),
}

#[derive(Debug, Clone)]
enum Review {
    Publication(Box<PreparedPublication>),
    Stage(Box<PreparedStage>),
    Withdrawal(Box<PreparedWithdrawal>),
    Inspect(serde_json::Value),
    Save {
        path: PathBuf,
        text: String,
        state: String,
        overwrite: bool,
    },
}

impl Review {
    fn operation_id(&self) -> &str {
        match self {
            Self::Publication(value) => &value.operation_id,
            Self::Stage(value) => &value.operation_id,
            Self::Withdrawal(value) => &value.operation_id,
            Self::Inspect(_) => "local-inspection",
            Self::Save { .. } => "local-export",
        }
    }

    fn kind(&self) -> &'static str {
        match self {
            Self::Publication(_) => "publication",
            Self::Stage(_) => "staging",
            Self::Withdrawal(_) => "withdrawal",
            Self::Inspect(_) => "inspection",
            Self::Save { .. } => "export",
        }
    }

    fn fingerprint(&self) -> &str {
        match self {
            Self::Publication(value) => &value.draft_fingerprint,
            Self::Stage(value) => &value.inventory_fingerprint,
            Self::Withdrawal(value) => &value.configuration_fingerprint,
            Self::Inspect(_) | Self::Save { .. } => "local",
        }
    }

    fn summary(&self) -> String {
        match self {
            Self::Publication(p) => format!(
                "Publish {} bytes; exact validated metadata and inventory:\n{}",
                p.estimated_asset_bytes,
                serde_json::to_string_pretty(p).unwrap()
            ),
            Self::Stage(p) => format!(
                "Copy {} assets / {} bytes; destination exists: {}; overwrite: {}\nDestination: {}\nReceipt: {}\n{}",
                p.assets.len(),
                p.estimated_asset_bytes,
                p.destination_exists,
                p.overwrite,
                p.destination.display(),
                p.receipt_path.display(),
                serde_json::to_string_pretty(&p.assets).unwrap()
            ),
            Self::Withdrawal(p) => format!(
                "Withdraw {} {} at revision {}\nReason: {}",
                p.reference, p.version, p.expected_manifest_revision, p.reason
            ),
            Self::Inspect(draft) => format!(
                "Validate selected publication assets. This inspects formats and may hash every selected byte. No publication occurs until the subsequent validated review.\n{}",
                serde_json::to_string_pretty(draft).unwrap()
            ),
            Self::Save {
                path,
                text,
                overwrite,
                ..
            } => format!(
                "Save local text to {}\nOverwrite: {}\n{}",
                path.display(),
                overwrite,
                text
            ),
        }
    }
    fn recovery(&self) -> Option<RecoveryIdentity> {
        match self {
            Self::Publication(p) => Some(RecoveryIdentity::from(p.as_ref())),
            Self::Stage(p) => Some(RecoveryIdentity::from(p.as_ref())),
            Self::Withdrawal(p) => Some(RecoveryIdentity::from(p.as_ref())),
            _ => None,
        }
    }
}

enum CoreUpdate {
    Catalog(CatalogPage),
    Detail(String),
    Review(Review),
    Draft(serde_json::Value),
    Operation(String),
    Status(String),
    Recovered(Vec<String>),
}
struct CorePending {
    receiver: mpsc::Receiver<Result<CoreUpdate, String>>,
    session: u64,
    mutation: bool,
}

struct App {
    options: TuiOptions,
    journal: OperationJournal,
    section: Section,
    input: InputMode,
    page: Option<CatalogPage>,
    selected: usize,
    detail: Option<String>,
    status: String,
    review: Option<Review>,
    operations: Vec<String>,
    last_reload: Instant,
    pending: Option<CorePending>,
    session: u64,
    scroll: u16,
    query: CatalogQuery,
    history: Vec<Option<String>>,
    draft: Option<serde_json::Value>,
    quit_when_idle: bool,
    no_color: bool,
    tutorial: Option<TutorialController>,
    #[cfg(feature = "agent-hosted")]
    agent: HostedAgentState,
}

impl App {
    fn new(options: TuiOptions, journal: OperationJournal) -> Self {
        let agent_status = options
            .agent_profile
            .as_deref()
            .map(|profile| format!("profile {profile} (requires agent-hosted build)"))
            .unwrap_or_else(|| "off".into());
        #[cfg(feature = "agent-hosted")]
        let hosted_agent = HostedAgentState::from_profile_request(options.agent_profile.as_deref());
        Self {
            options,
            journal,
            section: Section::Catalog,
            input: InputMode::None,
            page: None,
            selected: 0,
            detail: None,
            status: format!("Manual mode; agent: {agent_status}"),
            review: None,
            operations: Vec::new(),
            last_reload: Instant::now(),
            pending: None,
            session: 1,
            scroll: 0,
            query: CatalogQuery {
                limit: 25,
                ..Default::default()
            },
            history: Vec::new(),
            draft: None,
            quit_when_idle: false,
            no_color: std::env::var_os("NO_COLOR").is_some(),
            tutorial: None,
            #[cfg(feature = "agent-hosted")]
            agent: hosted_agent,
        }
    }

    fn job(
        &mut self,
        label: &str,
        mutation: bool,
        work: impl FnOnce() -> Result<CoreUpdate, String> + Send + 'static,
    ) {
        if self.pending.is_some() {
            self.status = "A core operation is still running; navigation remains available.".into();
            return;
        }
        let (sender, receiver) = mpsc::sync_channel(1);
        self.pending = Some(CorePending {
            receiver,
            session: self.session,
            mutation,
        });
        self.status = label.into();
        thread::spawn(move || {
            let result = std::panic::catch_unwind(std::panic::AssertUnwindSafe(work))
                .unwrap_or_else(|_| {
                    Err("Worker panicked; any mutation outcome requires reconciliation.".into())
                });
            let _ = sender.send(result);
        });
    }
    fn poll_core(&mut self) {
        let Some(pending) = &self.pending else {
            return;
        };
        let session = pending.session;
        let update = match pending.receiver.try_recv() {
            Ok(update) => update,
            Err(TryRecvError::Empty) => return,
            Err(TryRecvError::Disconnected) => {
                Err("Core worker disconnected; reconcile any operation.".into())
            }
        };
        self.pending = None;
        if session != self.session {
            return;
        }
        match update {
            Ok(CoreUpdate::Catalog(page)) => {
                self.selected = 0;
                self.detail = None;
                self.scroll = 0;
                self.status = format!(
                    "{} versions; {} peers; next page: {}",
                    page.entries.len(),
                    page.coverage.len(),
                    page.next_cursor.is_some()
                );
                self.page = Some(page);
                self.last_reload = Instant::now();
            }
            Ok(CoreUpdate::Detail(text)) => {
                self.detail = Some(text);
                self.scroll = 0;
                self.status = "Local view ready; PgUp/PgDn scroll, :export NAME saves text.".into();
            }
            Ok(CoreUpdate::Review(review)) => {
                self.review = Some(review);
                self.scroll = 0;
                self.status =
                    "Review complete details; PgUp/PgDn scroll; y confirms, n denies.".into();
            }
            Ok(CoreUpdate::Draft(draft)) => {
                self.detail = Some(serde_json::to_string_pretty(&draft).unwrap());
                self.draft = Some(draft);
                self.scroll = 0;
                self.status =
                    "Draft loaded; :draft set /field JSON, :draft validate, :draft save NAME"
                        .into();
            }
            Ok(CoreUpdate::Operation(result)) => {
                #[cfg(feature = "agent-hosted")]
                if let Ok(outcome) = serde_json::from_str::<serde_json::Value>(&result)
                    && let Some(id) = outcome["operation_id"].as_str()
                    && let Ok(harness) = self.agent.harness_mut(self.options.project_root.clone())
                {
                    let _ = harness.record_review_outcome(id, outcome.clone());
                }
                self.operations.push(result.clone());
                if self.operations.len() > 100 {
                    self.operations.remove(0);
                }
                self.status = serde_json::from_str::<serde_json::Value>(&result)
                    .ok()
                    .map(|v| {
                        format!(
                            "Operation {}: {}",
                            v["state"].as_str().unwrap_or("unknown"),
                            v["operation_id"].as_str().unwrap_or("unknown")
                        )
                    })
                    .unwrap_or_else(|| result.clone());
                self.section = Section::Operations;
                self.page = None;
            }
            Ok(CoreUpdate::Status(status)) => self.status = status,
            Ok(CoreUpdate::Recovered(records)) => {
                self.operations = records;
                self.reload();
            }
            Err(error) => {
                self.status = error;
                self.detail = Some(self.status.clone());
            }
        }
        // If a detail is not already shown, and a tutorial is active, show summary.
        if self.detail.is_none() {
            if let Some(ctrl) = &self.tutorial {
                let summary = format!("Tutorial step: {:?}", ctrl.checkpoint.step);
                self.detail = Some(summary);
            }
        }
    }
    fn recover(&mut self) {
        let journal = self.journal.clone();
        self.job("Reading private recovery journal…", false, move || {
            let mut records = Vec::new();
            for intent in journal.pending().map_err(|e| e.to_string())? {
                let outcome = intent.recovery.as_ref().map(reconcile).transpose().map_err(|e| e.to_string());
                records.push(format!("{}: {:?}. Observed current state; no approval retained and no retry performed.", intent.operation_id, outcome));
            }
            Ok(CoreUpdate::Recovered(records))
        });
    }
    fn reload(&mut self) {
        let root = self.options.project_root.clone();
        let query = self.query.clone();
        self.job("Reading bounded catalog…", false, move || {
            let project = Project::open(&root).map_err(|e| {
                format!("{e}. To initialize deliberately use :init NAMESPACE [SERVING_DIR].")
            })?;
            query_catalog(&project, query)
                .map(CoreUpdate::Catalog)
                .map_err(|e| e.to_string())
        });
    }
    fn request_quit(&mut self) -> bool {
        #[cfg(feature = "agent-hosted")]
        self.agent.stop();
        if self.pending.as_ref().is_some_and(|p| p.mutation) {
            self.quit_when_idle = true;
            self.status = "Finishing started operation and journal before exit…".into();
            false
        } else {
            true
        }
    }
    fn handle_key(&mut self, code: KeyCode) -> bool {
        #[cfg(feature = "agent-hosted")]
        self.poll_agent();
        if self.review.is_some() {
            return self.handle_review_key(code);
        }
        if matches!(self.input, InputMode::Search(_)) {
            return self.handle_search_input(code);
        }
        if matches!(self.input, InputMode::Command(_)) {
            return self.handle_command_input(code);
        }
        #[cfg(feature = "agent-hosted")]
        if matches!(self.input, InputMode::Agent(_)) {
            return self.handle_agent_input(code);
        }
        match code {
            KeyCode::Char('q') | KeyCode::Esc => self.request_quit(),
            KeyCode::Char('?') => {
                self.section = Section::Help;
                false
            }
            KeyCode::Tab => {
                self.section = self.section.next();
                self.scroll = 0;
                self.section_view();
                false
            }
            KeyCode::PageDown => {
                self.scroll = self.scroll.saturating_add(10);
                false
            }
            KeyCode::PageUp => {
                self.scroll = self.scroll.saturating_sub(10);
                false
            }
            KeyCode::Char(']') => {
                self.page_next();
                false
            }
            KeyCode::Char('[') => {
                self.page_previous();
                false
            }
            KeyCode::Char('/') => {
                self.input = InputMode::Search(String::new());
                false
            }
            KeyCode::Char(':') => {
                self.input = InputMode::Command(String::new());
                false
            }
            KeyCode::Char('r') => {
                self.refresh();
                false
            }
            KeyCode::Char('j') | KeyCode::Down => {
                self.move_selection(1);
                false
            }
            KeyCode::Char('k') | KeyCode::Up => {
                self.move_selection(-1);
                false
            }
            KeyCode::Enter => {
                self.open_selected();
                false
            }
            KeyCode::Char('e') => {
                self.example_selected();
                false
            }
            #[cfg(feature = "agent-hosted")]
            KeyCode::Char('a') => {
                if self.agent.is_ready() {
                    self.input = InputMode::Agent(String::new());
                } else {
                    self.status = self.agent.status();
                }
                false
            }
            #[cfg(feature = "agent-hosted")]
            KeyCode::Char('s') => {
                self.agent.stop();
                self.status = "Stopping assistant generation; any started data operation continues until its shared service finishes.".into();
                false
            }
            _ => false,
        }
    }

    fn handle_search_input(&mut self, code: KeyCode) -> bool {
        let InputMode::Search(value) = &mut self.input else {
            return false;
        };
        match code {
            KeyCode::Esc => self.input = InputMode::None,
            KeyCode::Backspace => {
                value.pop();
            }
            KeyCode::Enter => {
                let submitted = std::mem::take(value);
                self.input = InputMode::None;
                self.search(submitted);
            }
            KeyCode::Char(character) if value.len() < 16384 => value.push(character),
            _ => {}
        }
        false
    }

    fn handle_command_input(&mut self, code: KeyCode) -> bool {
        let InputMode::Command(value) = &mut self.input else {
            return false;
        };
        match code {
            KeyCode::Esc => self.input = InputMode::None,
            KeyCode::Backspace => {
                value.pop();
            }
            KeyCode::Enter => {
                let submitted = std::mem::take(value);
                self.input = InputMode::None;
                self.command(submitted);
            }
            KeyCode::Char(character) if value.len() < 16384 => value.push(character),
            _ => {}
        }
        false
    }

    

    #[cfg(feature = "agent-hosted")]
    fn handle_agent_input(&mut self, code: KeyCode) -> bool {
        let InputMode::Agent(value) = &mut self.input else {
            return false;
        };
        match code {
            KeyCode::Esc => self.input = InputMode::None,
            KeyCode::Backspace => {
                value.pop();
            }
            KeyCode::Enter => {
                let submitted = std::mem::take(value);
                self.input = InputMode::None;
                self.start_agent(submitted);
            }
            KeyCode::Char(character) if value.len() < 16384 => value.push(character),
            _ => {}
        }
        false
    }

    fn handle_review_key(&mut self, code: KeyCode) -> bool {
        match code {
            KeyCode::PageDown => self.scroll = self.scroll.saturating_add(10),
            KeyCode::PageUp => self.scroll = self.scroll.saturating_sub(10),
            KeyCode::Char('n') | KeyCode::Char('s') | KeyCode::Esc => {
                self.status = "Operation cancelled before commit.".into();
                #[cfg(feature = "agent-hosted")]
                if let Some(review) = &self.review
                    && let Ok(harness) = self.agent.harness_mut(self.options.project_root.clone())
                {
                    let _ = harness.record_review_outcome(
                        review.operation_id(),
                        serde_json::json!({"operation_id":review.operation_id(),"status":"denied"}),
                    );
                }
                self.review = None;
            }
            KeyCode::Char('h') =>
            {
                #[cfg(feature = "agent-hosted")]
                if let Some(review) = self.review.clone() {
                    let operation = match review {
                        Review::Publication(p) => Some(ConfirmableOperation::Publication(p)),
                        Review::Stage(p) => Some(ConfirmableOperation::Stage(p)),
                        Review::Withdrawal(p) => Some(ConfirmableOperation::Withdrawal(p)),
                        _ => None,
                    };
                    if let Some(operation) = operation
                        && let Ok(harness) =
                            self.agent.harness_mut(self.options.project_root.clone())
                    {
                        let id = harness.executor_mut().register_operation(operation);
                        self.review = None;
                        self.status = format!(
                            "Selected handle {id}; press a to ask the assistant. No approval given."
                        );
                    }
                }
            }
            KeyCode::Char('y') => self.confirm_review(),
            _ => {}
        }
        false
    }

    fn search(&mut self, text: String) {
        self.query.text = (!text.trim().is_empty()).then_some(text);
        self.query.cursor = None;
        self.history.clear();
        self.reload();
    }
    fn page_next(&mut self) {
        if self.pending.is_some() {
            return;
        }
        if let Some(cursor) = self.page.as_ref().and_then(|p| p.next_cursor.clone()) {
            self.history.push(self.query.cursor.clone());
            self.query.cursor = Some(cursor);
            self.reload();
        }
    }
    fn page_previous(&mut self) {
        if self.pending.is_some() {
            return;
        }
        if let Some(cursor) = self.history.pop() {
            self.query.cursor = cursor;
            self.reload();
        }
    }
    fn refresh(&mut self) {
        self.query.cursor = None;
        self.history.clear();
        let root = self.options.project_root.clone();
        let query = self.query.clone();
        self.job("Refreshing peer observations…", false, move || {
            let project = Project::open(root).map_err(|e| e.to_string())?;
            refresh(&project).map_err(|e| e.to_string())?;
            query_catalog(&project, query)
                .map(CoreUpdate::Catalog)
                .map_err(|e| e.to_string())
        });
    }
    fn section_view(&mut self) {
        let root = self.options.project_root.clone();
        let section = self.section;
        let entry = self.selected_entry().cloned();
        if !matches!(section, Section::Teams | Section::Cache | Section::Lineage) {
            return;
        }
        self.job("Reading local view…", false, move || {
            let project = Project::open(root).map_err(|e| e.to_string())?;
            let value = match section {
                Section::Teams => serde_json::to_value(
                    mesh_core::services::catalog_service::catalog_teams(&project)
                        .map_err(|e| e.to_string())?,
                )
                .unwrap(),
                Section::Cache => serde_json::to_value(
                    mesh_core::peer::cache_status(&project).map_err(|e| e.to_string())?,
                )
                .unwrap(),
                _ => {
                    let e = entry.ok_or("Select a pinned version first")?;
                    serde_json::to_value(
                        resolve(&project, &e.reference, &e.version, None, false)
                            .map_err(|e| e.to_string())?
                            .lineage,
                    )
                    .unwrap()
                }
            };
            Ok(CoreUpdate::Detail(
                serde_json::to_string_pretty(&value).unwrap(),
            ))
        });
    }
    fn move_selection(&mut self, delta: isize) {
        let len = self.page.as_ref().map_or(0, |page| page.entries.len());
        if len == 0 {
            return;
        }
        self.selected = (self.selected as isize + delta).clamp(0, len as isize - 1) as usize;
    }

    fn selected_entry(&self) -> Option<&CatalogEntry> {
        self.page.as_ref()?.entries.get(self.selected)
    }

    fn open_selected(&mut self) {
        if let Some(e) = self.selected_entry().cloned() {
            self.resolve_view(e.reference, e.version);
        }
    }
    fn resolve_view(&mut self, reference: String, version: String) {
        let root = self.options.project_root.clone();
        self.job(
            "Resolving pinned registered inventory…",
            false,
            move || {
                let project = Project::open(root).map_err(|e| e.to_string())?;
                let product = resolve(&project, &reference, &version, None, false)
                    .map_err(|e| e.to_string())?;
                Ok(CoreUpdate::Detail(
                    serde_json::to_string_pretty(&product).unwrap(),
                ))
            },
        );
    }
    fn example_selected(&mut self) {
        if let Some(entry) = self.selected_entry() {
            self.detail = Some(example(&self.options.project_root, entry));
            self.scroll = 0;
            self.status =
                "Examples generated locally; :export NAME saves to .feam/exports/NAME.".into();
        }
    }
    fn command(&mut self, input: String) {
        let args = match split_command(&input) {
            Ok(args) => args,
            Err(e) => {
                self.status = e;
                return;
            }
        };
        let words: Vec<_> = args.iter().map(String::as_str).collect();
        let root = self.options.project_root.clone();
        if self.pending.is_some() {
            self.status = "Wait for the current operation; navigation remains available.".into();
            return;
        }
        #[cfg(feature = "agent-hosted")]
        if matches!(&self.agent, HostedAgentState::Ready(ready) if ready.pending.is_some())
            && words.first().is_some_and(|c| {
                [
                    "publish",
                    "draft",
                    "stage",
                    "withdraw",
                    "init",
                    "export",
                    "agent-draft",
                    "integrity-consent",
                ]
                .contains(c)
            })
        {
            self.status =
                "Stop or finish the active assistant run before preparing a local mutation.".into();
            return;
        }
        match words.as_slice() {
            ["init", namespace] | ["init", namespace, _] => {
                let namespace = namespace.to_string(); let serving = args.get(2).map(PathBuf::from);
                self.job("Initializing selected project…", true, move || Project::init(root, namespace, serving).map(|_| CoreUpdate::Status("Project initialized; press r to load.".into())).map_err(|e| e.to_string()));
            }
            ["project", path] => {
                let selected = match std::path::absolute(path) { Ok(path) => path, Err(error) => { self.status = error.to_string(); return; } };
                let selected = std::fs::canonicalize(&selected).unwrap_or(selected);
                let journal = std::fs::canonicalize(self.journal.root()).unwrap_or_else(|_| self.journal.root().to_path_buf());
                if journal.starts_with(&selected) { self.status = "Select a project outside the private operation journal.".into(); return; }
                self.session += 1; self.review = None; self.draft = None; self.page = None; self.detail = None;
                self.options.project_root = selected; self.query.cursor = None; self.history.clear();
                #[cfg(feature = "agent-hosted")] { self.agent.stop(); self.agent = HostedAgentState::from_profile_request(self.options.agent_profile.as_deref()); }
                self.reload();
            }
            ["resolve", reference, version] => self.resolve_view(reference.to_string(), version.to_string()),
            ["publish", path] | ["draft", "load", path] => {
                let path = PathBuf::from(path); let publish = words[0] == "publish";
                self.job("Loading bounded publication draft…", false, move || {
                    use std::io::Read;
                    let mut bytes = Vec::new();
                    std::fs::File::open(&path).map_err(|e| e.to_string())?.take(1024 * 1024 + 1).read_to_end(&mut bytes).map_err(|e| e.to_string())?;
                    if bytes.len() > 1024 * 1024 { return Err("Draft exceeds 1 MiB".into()); }
                    let value = serde_json::from_slice(&bytes).map_err(|e| e.to_string())?;
                    Ok(if publish { CoreUpdate::Review(Review::Inspect(value)) } else { CoreUpdate::Draft(value) })
                });
            }
            ["draft", "new", kind] if ["table", "raster"].contains(kind) => {
                self.draft = Some(controls::draft_template(kind)); self.show_draft();
            }
            ["draft", "set", pointer, json] => {
                let result = serde_json::from_str(json).map_err(|e| e.to_string()).and_then(|value| controls::patch_draft(self.draft.as_mut().ok_or("Load or create a draft first")?, pointer, value));
                match result { Ok(()) => { self.review = None; self.show_draft(); } Err(e) => self.status = e }
            }
            ["draft", "validate"] => {
                if let Some(draft) = &self.draft { self.review = Some(Review::Inspect(draft.clone())); self.scroll = 0; }
            }
            ["draft", "save", name] | ["draft", "save", name, "--overwrite"] => {
                if let Some(draft) = &self.draft { self.prepare_save("drafts", name, serde_json::to_string_pretty(draft).unwrap(), words.len() == 4); }
            }
            ["export", name] | ["export", name, "--overwrite"] => {
                if let Some(text) = self.detail.clone() { self.prepare_save("exports", name, text, words.len() == 3); }
            }
            ["stage", reference, version, destination] | ["stage", reference, version, destination, "--overwrite"] => {
                let (reference, version, destination) = (reference.to_string(), version.to_string(), PathBuf::from(destination)); let overwrite = words.len() == 5;
                self.job("Checking staging inventory and destination…", false, move || prepare_stage(root, reference, version, destination, overwrite).map(|p| CoreUpdate::Review(Review::Stage(Box::new(p)))).map_err(|e| e.to_string()));
            }
            ["withdraw", reference, version, reason @ ..] if !reason.is_empty() => {
                let (reference, version, reason) = (reference.to_string(), version.to_string(), reason.join(" "));
                self.job("Checking local withdrawal…", false, move || prepare_withdrawal(root, reference, version, reason).map(|p| CoreUpdate::Review(Review::Withdrawal(Box::new(p)))).map_err(|e| e.to_string()));
            }
            ["filter", json] => {
                match serde_json::from_str::<CatalogQuery>(json) { Ok(query) => { self.query = query; self.history.clear(); self.reload(); } Err(e) => self.status = e.to_string() }
            }
            #[cfg(feature = "agent-hosted")]
            ["agent-draft"] => {
                if let Some(draft) = self.draft.clone() && let Ok(harness) = self.agent.harness_mut(root) {
                    harness.executor_mut().register_draft("selected-draft".into(), draft); self.status = "Draft handle selected-draft exposed; ask for a metadata patch with a.".into();
                }
            }
            #[cfg(feature = "agent-hosted")]
            ["agent-profile", profile] => {
                self.agent.stop(); self.review = None; self.session += 1;
                self.options.agent_profile = if *profile == "off" { None } else { Some(profile.to_string()) };
                self.agent = HostedAgentState::from_profile_request(self.options.agent_profile.as_deref()); self.status = self.agent.status();
            }
            #[cfg(feature = "agent-hosted")]
            ["integrity-consent", reference, version] => {
                if let Ok(harness) = self.agent.harness_mut(root) {
                    harness.executor_mut().grant_integrity_consent(reference.to_string(),version.to_string());
                    self.status = "One full-asset integrity read authorized for the exact selected version.".into();
                }
            }
            ["recover"] => self.recover(),
            #[cfg(feature = "agent-hosted")]
            ["usage"] => {
                if let HostedAgentState::Ready(ready) = &self.agent {
                    self.detail = Some(serde_json::to_string_pretty(&ready.usage).unwrap());
                    self.status = "Assistant session usage; export to retain it before switching profiles.".into();
                }
            },
            ["help"] => self.section = Section::Help,
            ["tutorial"] | ["tutorial", "start"] => {
                let root = self.options.project_root.clone();
                let path = root.join(".feam").join("tutorials").join("tutorial.json");
                self.tutorial = Some(TutorialController::new(path));
                self.status = "Tutorial started; use :tutorial resume or :tutorial reset".into();
            }
            ["tutorial", "resume"] => {
                let root = self.options.project_root.clone();
                let path = root.join(".feam").join("tutorials").join("tutorial.json");
                self.tutorial = Some(TutorialController::new(path));
                self.status = "Tutorial resumed".into();
            }
            ["tutorial", "reset"] => {
                let root = self.options.project_root.clone();
                let path = root.join(".feam").join("tutorials").join("tutorial.json");
                let mut ctrl = TutorialController::new(path);
                let _ = ctrl.reset();
                self.tutorial = Some(ctrl);
                self.status = "Tutorial reset".into();
            }
            ["tutorial", "advance"] => {
                if let Some(ctrl) = &mut self.tutorial {
                    ctrl.advance();
                    let _ = ctrl.save_checkpoint();
                    self.status = format!("Advanced to {:?}", ctrl.checkpoint.step);
                } else {
                    self.status = "No active tutorial; start with :tutorial start".into();
                }
            }
            ["tutorial", "preview"] => {
                if let Some(ctrl) = &self.tutorial {
                    let assets = ctrl.checkpoint.selected_assets.clone();
                    let request = mesh_core::services::table_preview::TablePreviewRequest {
                        assets,
                        columns: Vec::new(),
                        limit: 25,
                    };
                    let _ = self.job("Running table preview…", false, move || {
                        match mesh_core::services::table_preview::run_preview(request) {
                            Ok(result) => Ok(CoreUpdate::Detail(serde_json::to_string_pretty(&result).unwrap())),
                            Err(e) => Err(e),
                        }
                    });
                } else {
                    self.status = "No active tutorial; start with :tutorial start".into();
                }
            }
            _ => self.status = "Unknown command or arguments; ? shows command help. Quote paths containing spaces.".into(),
        }
        // tutorial command handling fallthrough is above; no need to return a value here.
    }
    fn show_draft(&mut self) {
        self.detail = self
            .draft
            .as_ref()
            .map(|d| serde_json::to_string_pretty(d).unwrap());
        self.scroll = 0;
        self.status = "Edit with :draft set /field JSON; explicit assets via /assets; :draft validate checks required semantics.".into();
    }
    fn prepare_save(&mut self, folder: &str, name: &str, text: String, overwrite: bool) {
        if name.is_empty()
            || !name
                .bytes()
                .all(|b| b.is_ascii_alphanumeric() || b"-_.".contains(&b))
            || name == "."
            || name == ".."
        {
            self.status = "Use a simple local filename without paths.".into();
            return;
        }
        let path = self
            .options
            .project_root
            .join(".feam")
            .join(folder)
            .join(name);
        self.job("Checking local save destination…", false, move || {
            let state = path_state(&path).map_err(|e| e.to_string())?;
            if path.exists() && !overwrite {
                return Err("File exists; explicit --overwrite required.".into());
            }
            Ok(CoreUpdate::Review(Review::Save {
                path,
                text,
                state,
                overwrite,
            }))
        });
    }
    fn confirm_review(&mut self) {
        let Some(review) = self.review.take() else {
            return;
        };
        let root = self.options.project_root.clone();
        let journal = self.journal.clone();
        match review {
            Review::Inspect(value) => self.job(
                "Inspecting formats and hashing selected assets; this read may be expensive…",
                false,
                move || {
                    let request = serde_json::from_value(value)
                        .map_err(|e| format!("Incomplete/invalid draft: {e}"))?;
                    prepare_publication(root, request)
                        .map(|p| CoreUpdate::Review(Review::Publication(Box::new(p))))
                        .map_err(|e| e.to_string())
                },
            ),
            Review::Save {
                path,
                text,
                state,
                overwrite,
            } => self.job("Saving reviewed local text…", true, move || {
                save_local_text(&root, &path, &text, &state, overwrite)?;
                Ok(CoreUpdate::Status(format!("Saved {}", path.display())))
            }),
            review => self.job(
                "Mutation running; it will finish before normal exit. No automatic retry.",
                true,
                move || {
                    let intent = journal
                        .record_recoverable_intent(
                            review.operation_id(),
                            review.kind(),
                            review.fingerprint(),
                            review.recovery(),
                        )
                        .map_err(|e| format!("No mutation started: {e}"))?;
                    let outcome = match review {
                        Review::Publication(p) => serde_json::to_value(execute_publication(&p)),
                        Review::Stage(p) => serde_json::to_value(execute_stage(&p)),
                        Review::Withdrawal(p) => serde_json::to_value(execute_withdrawal(&p)),
                        _ => unreachable!(),
                    }
                    .map_err(|e| e.to_string())?;
                    let result: mesh_core::services::interactive_operations::OperationResult<
                        serde_json::Value,
                    > = serde_json::from_value(outcome.clone()).map_err(|e| e.to_string())?;
                    match journal.record_outcome(intent, &result) {
                        Ok(_) => Ok(CoreUpdate::Operation(outcome.to_string())),
                        Err(e) => {
                            let mut reported = outcome;
                            if reported["state"] == "committed" {
                                reported["state"] =
                                    serde_json::json!("committed_with_followup_error");
                            }
                            reported["journal_followup_error"] =
                                serde_json::json!(format!("{e}; reconcile before retry"));
                            Ok(CoreUpdate::Operation(reported.to_string()))
                        }
                    }
                },
            ),
        }
    }

    #[cfg(feature = "agent-hosted")]
    fn start_agent(&mut self, user_text: String) {
        if self.pending.is_some() || self.review.is_some() {
            self.status = "Finish the active local operation first.".into();
            return;
        }
        let project_root = self.options.project_root.clone();
        match self.agent.start(project_root, user_text) {
            Ok(()) => {
                self.section = Section::Catalog;
                self.detail = Some("Assistant request in progress…".into());
                self.scroll = 0;
                self.status = "Assistant request started; manual navigation remains available. Press s to stop generation.".into();
            }
            Err(error) => self.status = error,
        }
    }

    #[cfg(feature = "agent-hosted")]
    fn poll_agent(&mut self) {
        if let HostedAgentState::Ready(ready) = &self.agent
            && ready.pending.is_some()
            && !ready.streamed.is_empty()
        {
            self.detail = Some(format!(
                "Assistant (streaming; text is unverified until tool evidence):\n{}",
                ready.streamed
            ));
        }
        if let Some(run) = self.agent.take_completed() {
            let run_id = match &self.agent {
                HostedAgentState::Ready(ready) => ready.next_run_id - 1,
                _ => 0,
            };
            for tool in &run.tools {
                if let mesh_agent::ToolExecution::DraftPatch { draft, .. } = tool {
                    self.draft = Some(draft.clone());
                    self.detail = Some(serde_json::to_string_pretty(draft).unwrap());
                }
            }
            if let Some(operation) = run.pending_operation.clone()
                && self.review.is_none()
                && !self.pending.as_ref().is_some_and(|p| p.mutation)
            {
                self.review = Some(match operation {
                    ConfirmableOperation::Publication(value) => Review::Publication(value),
                    ConfirmableOperation::Stage(value) => Review::Stage(value),
                    ConfirmableOperation::Withdrawal(value) => Review::Withdrawal(value),
                });
            }
            self.detail = Some(sanitize_terminal_text(&format!(
                "Assistant result #{run_id} ({:?}):\n{}\n\nTool activity:\n{}",
                run.state,
                run.text,
                serde_json::to_string_pretty(&run.tools).unwrap_or_else(|_| "unavailable".into())
            )));
            self.status = if self.review.is_some() {
                "Assistant proposal awaits local review; press y to confirm or n to cancel.".into()
            } else {
                "Assistant response received. Any mutation proposal is never auto-executed.".into()
            };
        }
    }
}

#[cfg(feature = "agent-hosted")]
struct AgentPending {
    receiver: mpsc::Receiver<AgentWorkerMessage>,
    cancellation: CancellationToken,
    run_id: u64,
}

#[cfg(feature = "agent-hosted")]
enum HostedAgentState {
    Off,
    Disabled(String),
    Ready(Box<HostedAgentReady>),
}

#[cfg(feature = "agent-hosted")]
struct HostedAgentReady {
    profile: AgentProfile,
    provider: mesh_agent::provider::SessionProvider,
    next_run_id: u64,
    pending: Option<AgentPending>,
    harness: Option<AgentHarness<mesh_agent::provider::SessionProvider>>,
    streamed: String,
    usage: Vec<serde_json::Value>,
}

#[cfg(feature = "agent-hosted")]
impl HostedAgentState {
    fn from_profile_request(requested: Option<&str>) -> Self {
        let Some(requested) = requested else {
            return Self::Off;
        };
        if requested == "__fake__" {
            let result = std::env::var_os("FEAM_TUI_REPLAY")
                .ok_or("Set FEAM_TUI_REPLAY to an explicit local replay event file".to_string())
                .and_then(|path| {
                    mesh_agent::provider::ReplayProvider::load(&PathBuf::from(path))
                        .map_err(|e| e.to_string())
                });
            return match result {
                Ok(replay) => {
                    let profile: AgentProfile = serde_json::from_value(serde_json::json!({"backend":"router","base_url":"https://unused.invalid","model":"DETERMINISTIC REPLAY (no inference)","api_key_env":"UNUSED","context_policy":"synthetic-demo","allow_user_text":true,"max_context_chars":32000})).unwrap();
                    Self::Ready(Box::new(HostedAgentReady {
                        profile,
                        provider: mesh_agent::provider::SessionProvider::Replay(replay),
                        next_run_id: 1,
                        pending: None,
                        harness: None,
                        streamed: String::new(),
                        usage: Vec::new(),
                    }))
                }
                Err(e) => Self::Disabled(e),
            };
        }
        let Some(path) = AgentConfig::default_path() else {
            return Self::Disabled(
                "assistant disabled: no user configuration directory is available".into(),
            );
        };
        let requested = (!requested.is_empty()).then_some(requested);
        let config = match AgentConfig::load(&path) {
            Ok(config) => config,
            Err(error) => {
                return Self::Disabled(format!(
                    "assistant disabled: {error}; manual mode remains available"
                ));
            }
        };
        let (_, profile) = match config.selected(requested) {
            Ok(selected) => selected,
            Err(error) => {
                return Self::Disabled(format!(
                    "assistant disabled: {error}; manual mode remains available"
                ));
            }
        };
        let profile = profile.clone();
        match RouterProvider::from_profile(profile.clone()) {
            Ok(provider) => Self::Ready(Box::new(HostedAgentReady {
                profile,
                provider: mesh_agent::provider::SessionProvider::Router(Box::new(provider)),
                next_run_id: 1,
                pending: None,
                harness: None,
                streamed: String::new(),
                usage: Vec::new(),
            })),
            Err(error) => Self::Disabled(format!(
                "assistant disabled: {error}; manual mode remains available"
            )),
        }
    }

    fn is_ready(&self) -> bool {
        matches!(self, Self::Ready(ready) if ready.pending.is_none())
    }

    fn status(&self) -> String {
        match self {
            Self::Off => {
                "assistant is off; launch with --agent hosted to request a configured profile"
                    .into()
            }
            Self::Disabled(message) => message.clone(),
            Self::Ready(ready) => {
                if ready.pending.is_some() {
                    "assistant request is running".into()
                } else {
                    format!(
                        "assistant ready: {} ({})",
                        ready.profile.model, ready.profile.context_policy
                    )
                }
            }
        }
    }

    fn harness_mut(
        &mut self,
        root: PathBuf,
    ) -> Result<&mut AgentHarness<mesh_agent::provider::SessionProvider>, String> {
        let Self::Ready(ready) = self else {
            return Err(self.status());
        };
        if ready.pending.is_some() {
            return Err("Assistant is busy; stop or finish the active request first.".into());
        }
        Ok(ready.harness.get_or_insert_with(|| {
            AgentHarness::new(ready.profile.clone(), ready.provider.clone(), root)
        }))
    }
    fn start(&mut self, project_root: PathBuf, user_text: String) -> Result<(), String> {
        self.harness_mut(project_root)?;
        let Self::Ready(ready) = self else {
            unreachable!()
        };
        let mut harness = ready.harness.take().unwrap();
        let cancellation = CancellationToken::default();
        harness.set_cancellation(cancellation.clone());
        let (sender, receiver) = mpsc::sync_channel(32);
        let run_id = ready.next_run_id;
        ready.next_run_id += 1;
        ready.streamed.clear();
        thread::spawn(move || {
            let run = harness.run_streamed(&user_text, &mut |event| {
                let _ = sender.try_send(AgentWorkerMessage::Event(run_id, event));
            });
            let _ = sender.send(AgentWorkerMessage::Complete(run_id, Box::new(harness), run));
        });
        ready.pending = Some(AgentPending {
            receiver,
            cancellation,
            run_id,
        });
        Ok(())
    }
    fn stop(&mut self) {
        if let Self::Ready(ready) = self {
            if let Some(pending) = &ready.pending {
                pending.cancellation.cancel();
            }
            if let Some(harness) = &mut ready.harness {
                harness.invalidate();
            }
        }
    }
    fn take_completed(&mut self) -> Option<AgentRun> {
        let Self::Ready(ready) = self else {
            return None;
        };
        let active = ready.pending.as_ref()?;
        for _ in 0..32 {
            match active.receiver.try_recv() {
                Ok(AgentWorkerMessage::Event(id, event))
                    if id == active.run_id && !active.cancellation.is_cancelled() =>
                {
                    match event {
                        mesh_agent::AgentEvent::Text(text) => {
                            if ready.streamed.len() + text.len() <= ready.profile.max_response_chars
                            {
                                ready.streamed.push_str(&text);
                            }
                        }
                        mesh_agent::AgentEvent::ToolStarted(name) => {
                            ready.streamed.push_str(&format!("\n[tool: {name}]\n"))
                        }
                        _ => (),
                    }
                }
                Ok(AgentWorkerMessage::Complete(id, mut harness, run)) => {
                    let cancelled = active.cancellation.is_cancelled() || id != active.run_id;
                    if ready.usage.len() == 100 {
                        ready.usage.remove(0);
                    }
                    ready.usage.push(serde_json::json!({"run_id": id, "cancelled": cancelled, "usage": run.usage, "cost_usd": run.estimated_cost_usd}));
                    if cancelled {
                        harness.invalidate();
                    }
                    ready.harness = Some(*harness);
                    ready.pending = None;
                    return (!cancelled).then_some(run);
                }
                Ok(_) => (),
                Err(TryRecvError::Empty) => return None,
                Err(TryRecvError::Disconnected) => {
                    ready.pending = None;
                    return None;
                }
            }
        }
        None
    }
}
#[cfg(feature = "agent-hosted")]
enum AgentWorkerMessage {
    Event(u64, mesh_agent::AgentEvent),
    Complete(
        u64,
        Box<AgentHarness<mesh_agent::provider::SessionProvider>>,
        AgentRun,
    ),
}

fn render(frame: &mut Frame, app: &App) {
    let area = frame.area();
    let rows = Layout::default()
        .direction(Direction::Vertical)
        .constraints([
            Constraint::Length(4),
            Constraint::Min(1),
            Constraint::Length(3),
        ])
        .split(area);
    let peer_summary = app
        .page
        .as_ref()
        .map(|page| {
            let available = page.coverage.iter().filter(|peer| peer.available).count();
            format!("Peers: {available}/{} available", page.coverage.len())
        })
        .unwrap_or_else(|| "Peers: unavailable".into());
    frame.render_widget(
        Paragraph::new(vec![
            Line::from(vec![
                Span::styled(" feam ", Style::default().add_modifier(Modifier::BOLD)),
                Span::raw(format!(
                    "{} | {peer_summary}",
                    sanitize_terminal_text(
                        &app.options
                            .project_root
                            .file_name()
                            .unwrap_or_default()
                            .to_string_lossy()
                    )
                )),
            ]),
            Line::from(sanitize_terminal_text(&app.status)),
        ])
        .block(Block::default().borders(Borders::ALL)),
        rows[0],
    );
    if area.width < 80 || area.height < 24 {
        frame.render_widget(
            Paragraph::new(
                "Terminal is too small; use at least 80×24 or existing feam CLI commands.",
            )
            .block(Block::default().borders(Borders::ALL)),
            rows[1],
        );
    } else {
        render_body(frame, app, rows[1]);
    }
    let prompt = match &app.input {
        InputMode::None => "/ search | : command | Enter details | e examples | r refresh | Tab section | ? help | q exit".into(),
        InputMode::Search(value) => format!("Search: {value}"),
        InputMode::Command(value) => format!("Command: {value}"),
        #[cfg(feature = "agent-hosted")]
        InputMode::Agent(value) => format!("Assistant: {value}"),
    };
    frame.render_widget(
        Paragraph::new(sanitize_terminal_text(&prompt))
            .block(Block::default().borders(Borders::ALL)),
        rows[2],
    );
    if let Some(review) = &app.review {
        render_review(frame, review, area, app.scroll);
    }
}

fn render_body(frame: &mut Frame, app: &App, area: Rect) {
    let sections = "Catalog  Peers  Operations  Help  Teams  Cache  Lineage";
    let chunks = Layout::vertical([Constraint::Length(1), Constraint::Min(1)]).split(area);
    frame.render_widget(
        Paragraph::new(format!("[{}] {sections}", app.section.title())),
        chunks[0],
    );
    let cols = Layout::horizontal([Constraint::Percentage(40), Constraint::Percentage(60)])
        .split(chunks[1]);
    let entries = app
        .page
        .as_ref()
        .map(|p| p.entries.as_slice())
        .unwrap_or_default();
    let start = app
        .selected
        .saturating_sub(cols[0].height.saturating_sub(4) as usize);
    let list: Vec<_> = entries
        .iter()
        .enumerate()
        .skip(start)
        .map(|(i, e)| {
            let text = sanitize_terminal_text(&format!(
                "{} {} {}",
                if i == app.selected { ">" } else { " " },
                e.reference,
                e.version
            ));
            let style = if i == app.selected {
                if app.no_color {
                    Style::default().add_modifier(Modifier::REVERSED)
                } else {
                    Style::default().fg(Color::Black).bg(Color::Cyan)
                }
            } else {
                Style::default()
            };
            ListItem::new(text).style(style)
        })
        .collect();
    frame.render_widget(
        List::new(list).block(
            Block::default()
                .title("Versions  [ / ] page")
                .borders(Borders::ALL),
        ),
        cols[0],
    );
    let detail = match app.section {
        Section::Peers => app.page.as_ref().map(|p| serde_json::to_string_pretty(&p.coverage).unwrap()).unwrap_or_default(),
        Section::Operations => format!("Journal: {}\n{}", app.journal.root().display(), app.operations.join("\n\n")),
        Section::Help => controls::HELP.into(),
        _ => app.detail.clone().unwrap_or_else(|| "Enter: metadata/inventory. e: table/raster examples. PgUp/PgDn: scroll. :project \"PATH\": switch and clear context.".into()),
    };
    frame.render_widget(
        Paragraph::new(sanitize_terminal_text(&detail))
            .wrap(Wrap { trim: false })
            .scroll((app.scroll, 0))
            .block(
                Block::default()
                    .title(app.section.title())
                    .borders(Borders::ALL),
            ),
        cols[1],
    );
}

fn render_review(frame: &mut Frame, review: &Review, area: Rect, scroll: u16) {
    let popup = Rect {
        x: area.x + 1,
        y: area.y + 1,
        width: area.width.saturating_sub(2),
        height: area.height.saturating_sub(2),
    };
    frame.render_widget(Clear, popup);
    let rows = Layout::vertical([Constraint::Length(3), Constraint::Min(1)]).split(popup);
    frame.render_widget(Paragraph::new("y confirm | n deny | h assistant handle | PgUp/PgDn full details\nChanged state requires a fresh review. No approval survives restart.").wrap(Wrap { trim: false }), rows[0]);
    frame.render_widget(
        Paragraph::new(sanitize_terminal_text(&format!(
            "Operation: {}\n{}",
            review.operation_id(),
            review.summary()
        )))
        .scroll((scroll, 0))
        .wrap(Wrap { trim: false })
        .block(Block::default().title("Review").borders(Borders::ALL)),
        rows[1],
    );
}

fn sanitize_terminal_text(value: &str) -> String {
    value
        .chars()
        .map(|character| {
            if character == '\n' || character == '\t' || !character.is_control() {
                character
            } else {
                '�'
            }
        })
        .collect()
}

fn fallback_journal_root() -> PathBuf {
    use std::hash::{Hash, Hasher};
    let mut hash = std::collections::hash_map::DefaultHasher::new();
    std::env::var_os("HOME").hash(&mut hash);
    std::env::temp_dir().join(format!("feam-tui-journal-{:x}", hash.finish()))
}

fn default_journal_root() -> PathBuf {
    if let Some(root) = std::env::var_os("FEAM_TUI_JOURNAL_DIR") {
        return PathBuf::from(root);
    }
    let base = std::env::var_os("XDG_STATE_HOME")
        .map(PathBuf::from)
        .or_else(|| std::env::var_os("HOME").map(|home| PathBuf::from(home).join(".local/state")))
        .unwrap_or_else(std::env::temp_dir);
    base.join("feam/tui/operations")
}

#[cfg(test)]
mod tests {
    use super::*;
    use ratatui::backend::TestBackend;

    #[test]
    fn external_terminal_controls_are_not_rendered() {
        assert_eq!(sanitize_terminal_text("safe\u{1b}[2Jtext"), "safe�[2Jtext");
    }

    #[test]
    fn missing_project_requires_explicit_initialization() {
        let temp = tempfile::tempdir().unwrap();
        let journal = OperationJournal::open(temp.path().join("journal")).unwrap();
        let mut app = App::new(
            TuiOptions {
                project_root: temp.path().join("missing"),
                agent_profile: None,
            },
            journal,
        );
        app.reload();
        while app.pending.is_some() {
            thread::sleep(Duration::from_millis(5));
            app.poll_core();
        }
        assert!(app.status.contains("init"));
        let backend = TestBackend::new(100, 30);
        let mut terminal = Terminal::new(backend).unwrap();
        terminal.draw(|frame| render(frame, &app)).unwrap();
    }
}

#[cfg(test)]
mod workflow_tests {
    use super::test_fixture as fixture;
    use super::*;
    fn settled(app: &mut App) {
        let start = Instant::now();
        while app.pending.is_some() {
            assert!(start.elapsed() < Duration::from_secs(5));
            thread::sleep(Duration::from_millis(5));
            app.poll_core();
        }
    }
    fn fixture_app() -> (tempfile::TempDir, App) {
        let temp = tempfile::tempdir().unwrap();
        let root = temp.path().join("provider with spaces");
        let project = Project::init(&root, "climate".into(), Some("serving".into())).unwrap();
        let request = fixture::table_request();
        for asset in &request.assets {
            let path = project.serving_root().unwrap().join(&asset.path);
            std::fs::create_dir_all(path.parent().unwrap()).unwrap();
            fixture::write_parquet(&path, &[1, 2]);
        }
        let journal = OperationJournal::open(temp.path().join("journal")).unwrap();
        let mut app = App::new(
            TuiOptions {
                project_root: root,
                agent_profile: None,
            },
            journal,
        );
        app.draft = Some(serde_json::to_value(request).unwrap());
        (temp, app)
    }
    #[test]
    fn manual_publication_stage_denial_withdrawal_and_recovery() {
        let (temp, mut app) = fixture_app();
        app.command("draft validate".into());
        assert!(matches!(app.review, Some(Review::Inspect(_))));
        app.handle_review_key(KeyCode::Char('y'));
        settled(&mut app);
        assert!(matches!(app.review, Some(Review::Publication(_))));
        app.handle_review_key(KeyCode::Char('n'));
        assert!(
            !app.options
                .project_root
                .join("serving/manifest.json")
                .exists()
        );
        app.command("draft validate".into());
        app.handle_review_key(KeyCode::Char('y'));
        settled(&mut app);
        app.handle_review_key(KeyCode::Char('y'));
        app.handle_review_key(KeyCode::Char('y'));
        settled(&mut app);
        assert!(app.status.contains("committed"));
        let destination = temp.path().join("copied data");
        let command = format!(
            "stage product://climate/observations v1 '{}'",
            destination.display()
        );
        app.command(command.clone());
        settled(&mut app);
        app.handle_review_key(KeyCode::Char('n'));
        assert!(!destination.exists());
        app.command(command);
        settled(&mut app);
        app.handle_review_key(KeyCode::Char('y'));
        settled(&mut app);
        assert!(destination.join("part-000").exists());
        assert!(temp.path().join("copied data.feam-receipt.json").is_file());
        app.command("withdraw product://other/observations v1 wrong-namespace".into());
        settled(&mut app);
        assert!(app.review.is_none());
        app.command("withdraw product://climate/observations v1 superseded".into());
        settled(&mut app);
        app.handle_review_key(KeyCode::Char('y'));
        settled(&mut app);
        assert!(matches!(
            resolve(
                &Project::open(&app.options.project_root).unwrap(),
                "product://climate/observations",
                "v1",
                None,
                false
            ),
            Err(mesh_core::peer::PeerError::Withdrawn(_))
        ));
        assert!(app.journal.pending().unwrap().is_empty());
        app.recover();
        settled(&mut app);
    }
    #[test]
    fn lost_journal_result_preserves_structured_committed_followup_error() {
        let (_temp, mut app) = fixture_app();
        app.command("draft validate".into());
        app.handle_review_key(KeyCode::Char('y'));
        settled(&mut app);
        let id = app.review.as_ref().unwrap().operation_id().to_owned();
        std::fs::write(app.journal.root().join(format!(".{id}.tmp")), b"collision").unwrap();
        app.handle_review_key(KeyCode::Char('y'));
        settled(&mut app);
        assert!(app.status.contains("committed_with_followup_error"));
        let outcome: serde_json::Value =
            serde_json::from_str(app.operations.last().unwrap()).unwrap();
        assert_eq!(outcome["operation_id"], id);
        assert_eq!(outcome["state"], "committed_with_followup_error");
        let intents = app.journal.pending().unwrap();
        assert_eq!(intents.len(), 1);
        assert_eq!(
            reconcile(intents[0].recovery.as_ref().unwrap()).unwrap(),
            mesh_core::services::interactive_operations::OperationState::Committed
        );
    }

    #[test]
    fn restart_reconciles_lost_commit_result_without_replaying_write() {
        let (_temp, app) = fixture_app();
        let prepared = prepare_publication(
            &app.options.project_root,
            serde_json::from_value(app.draft.clone().unwrap()).unwrap(),
        )
        .unwrap();
        app.journal
            .record_recoverable_intent(
                &prepared.operation_id,
                "publication",
                "test",
                Some(RecoveryIdentity::from(&prepared)),
            )
            .unwrap();
        assert_eq!(
            execute_publication(&prepared).state,
            mesh_core::services::interactive_operations::OperationState::Committed
        );
        let path = app.options.project_root.join("serving/manifest.json");
        let committed = std::fs::read(&path).unwrap();
        let root = app.options.project_root.clone();
        let journal_root = app.journal.root().to_path_buf();
        drop(app);
        let mut restarted = App::new(
            TuiOptions {
                project_root: root,
                agent_profile: None,
            },
            OperationJournal::open(journal_root).unwrap(),
        );
        restarted.recover();
        settled(&mut restarted);
        assert!(
            restarted
                .operations
                .iter()
                .any(|entry| entry.contains("Committed"))
        );
        assert!(restarted.review.is_none());
        assert_eq!(std::fs::read(path).unwrap(), committed);
        assert_eq!(restarted.journal.pending().unwrap().len(), 1);
    }

    #[test]
    fn worker_keeps_input_responsive_and_switch_invalidates_state() {
        let (_temp, mut app) = fixture_app();
        app.job("slow read", false, || {
            thread::sleep(Duration::from_millis(150));
            Ok(CoreUpdate::Status("done".into()))
        });
        let start = Instant::now();
        app.handle_key(KeyCode::Char('?'));
        assert!(start.elapsed() < Duration::from_millis(100));
        assert_eq!(app.section, Section::Help);
        settled(&mut app);
        app.review = Some(Review::Inspect(serde_json::json!({})));
        app.command("project '/tmp/absent feam project'".into());
        assert!(app.review.is_none());
        assert!(app.draft.is_none());
        settled(&mut app);
    }
    #[test]
    fn full_review_renders_controls_at_eighty_by_twenty_four() {
        use ratatui::backend::TestBackend;
        let (_temp, mut app) = fixture_app();
        app.command("draft validate".into());
        let mut terminal = Terminal::new(TestBackend::new(80, 24)).unwrap();
        terminal.draw(|f| render(f, &app)).unwrap();
        let text = terminal
            .backend()
            .buffer()
            .content
            .iter()
            .map(|c| c.symbol())
            .collect::<String>();
        assert!(text.contains("y confirm"));
        assert!(text.contains("PgUp/PgDn"));
    }
}

#[cfg(test)]
#[path = "../../mesh_core/tests/support/mod.rs"]
mod test_fixture;

/// Lifecycle probe used only by the separately built PTY test executable.
#[cfg(feature = "test-driver")]
pub fn terminal_lifecycle_probe(panic: bool) -> Result<(), TuiError> {
    let _guard = TerminalGuard::enter()?;
    if panic {
        panic!("intentional terminal lifecycle test panic");
    }
    Err(TuiError::Io(io::Error::other(
        "intentional terminal lifecycle test error",
    )))
}
