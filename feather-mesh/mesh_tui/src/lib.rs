//! Keyboard-first terminal UI for project-scoped Feather Mesh workflows.
//!
//! This crate calls the shared Rust services directly. It never parses CLI
//! output, initializes a legacy registry, or treats terminal text as trusted.

#[cfg(feature = "agent-hosted")]
use std::collections::VecDeque;
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
use ratatui::widgets::{Block, Borders, Clear, List, ListItem, Paragraph, Tabs, Wrap};
use ratatui::{Frame, Terminal};
use thiserror::Error;

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
    for signal in [
        signal_hook::consts::SIGINT,
        signal_hook::consts::SIGTERM,
        signal_hook::consts::SIGHUP,
    ] {
        signals.push(signal_hook::flag::register(signal, terminating.clone())?);
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
    #[cfg(feature = "agent-hosted")]
    Assistant,
}

impl Section {
    #[cfg(not(feature = "agent-hosted"))]
    const ALL: [Self; 7] = [
        Self::Catalog,
        Self::Peers,
        Self::Operations,
        Self::Help,
        Self::Teams,
        Self::Cache,
        Self::Lineage,
    ];

    #[cfg(feature = "agent-hosted")]
    const ALL: [Self; 8] = [
        Self::Catalog,
        Self::Peers,
        Self::Operations,
        Self::Help,
        Self::Teams,
        Self::Cache,
        Self::Lineage,
        Self::Assistant,
    ];

    fn next(self) -> Self {
        let index = Self::ALL
            .iter()
            .position(|section| *section == self)
            .unwrap();
        Self::ALL[(index + 1) % Self::ALL.len()]
    }

    fn index(self) -> usize {
        Self::ALL
            .iter()
            .position(|section| *section == self)
            .unwrap()
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
            #[cfg(feature = "agent-hosted")]
            Self::Assistant => "Assistant",
        }
    }
}

#[derive(Debug, Default)]
struct TextViewState {
    text: Option<String>,
    scroll: u16,
}

#[derive(Debug, Default)]
struct SectionViewState {
    catalog: TextViewState,
    peers: TextViewState,
    operations: TextViewState,
    help: TextViewState,
    teams: TextViewState,
    cache: TextViewState,
    lineage: TextViewState,
}

impl SectionViewState {
    fn get(&self, section: Section) -> &TextViewState {
        match section {
            Section::Catalog => &self.catalog,
            Section::Peers => &self.peers,
            Section::Operations => &self.operations,
            Section::Help => &self.help,
            Section::Teams => &self.teams,
            Section::Cache => &self.cache,
            Section::Lineage => &self.lineage,
            #[cfg(feature = "agent-hosted")]
            Section::Assistant => unreachable!("assistant has dedicated view state"),
        }
    }

    fn get_mut(&mut self, section: Section) -> &mut TextViewState {
        match section {
            Section::Catalog => &mut self.catalog,
            Section::Peers => &mut self.peers,
            Section::Operations => &mut self.operations,
            Section::Help => &mut self.help,
            Section::Teams => &mut self.teams,
            Section::Cache => &mut self.cache,
            Section::Lineage => &mut self.lineage,
            #[cfg(feature = "agent-hosted")]
            Section::Assistant => unreachable!("assistant has dedicated view state"),
        }
    }
}

#[cfg(feature = "agent-hosted")]
const ASSISTANT_TRANSCRIPT_LIMIT: usize = 1024 * 1024;
#[cfg(feature = "agent-hosted")]
const ASSISTANT_EVICTION_NOTICE: &str =
    "[Older assistant entries were removed to keep this view bounded.]";

#[cfg(feature = "agent-hosted")]
#[derive(Debug)]
struct AssistantTranscriptEntry {
    user: Option<String>,
    response: String,
    complete: bool,
}

#[cfg(feature = "agent-hosted")]
#[derive(Debug, Default)]
struct AssistantViewState {
    entries: VecDeque<AssistantTranscriptEntry>,
    scroll_back: u16,
    unread: bool,
    evicted: bool,
    export_text: Option<String>,
}

#[cfg(feature = "agent-hosted")]
impl AssistantViewState {
    fn begin(&mut self, user: String) {
        self.entries.push_back(AssistantTranscriptEntry {
            user: Some(sanitize_terminal_text(&user)),
            response: "Assistant request in progress…".into(),
            complete: false,
        });
        self.scroll_back = 0;
        self.unread = false;
        self.export_text = None;
        self.enforce_limit();
    }

    fn stream(&mut self, text: &str) {
        if let Some(entry) = self.entries.back_mut().filter(|entry| !entry.complete) {
            entry.response = format!(
                "Assistant (streaming; text is unverified until tool evidence):\n{}",
                sanitize_terminal_text(text)
            );
        }
    }

    fn finish(&mut self, text: String, visible: bool) {
        self.export_text = None;
        if let Some(entry) = self.entries.back_mut().filter(|entry| !entry.complete) {
            entry.response = sanitize_terminal_text(&text);
            entry.complete = true;
        } else {
            self.entries.push_back(AssistantTranscriptEntry {
                user: None,
                response: sanitize_terminal_text(&text),
                complete: true,
            });
        }
        self.unread = !visible;
        self.enforce_limit();
    }

    fn cancel(&mut self, visible: bool) {
        if let Some(entry) = self.entries.back_mut().filter(|entry| !entry.complete) {
            entry.response.push_str("\n\n[Assistant request cancelled]");
            entry.complete = true;
            self.unread = !visible;
        }
    }

    fn push_local(&mut self, label: &str, text: String, visible: bool) {
        self.export_text = Some(text.clone());
        self.entries.push_back(AssistantTranscriptEntry {
            user: None,
            response: sanitize_terminal_text(&format!("{label}:\n{text}")),
            complete: true,
        });
        self.scroll_back = 0;
        self.unread = !visible;
        self.enforce_limit();
    }

    fn rendered(&self) -> String {
        let mut text = String::new();
        if self.evicted {
            text.push_str(ASSISTANT_EVICTION_NOTICE);
            text.push_str("\n\n");
        }
        for (index, entry) in self.entries.iter().enumerate() {
            if index > 0 {
                text.push_str("\n\n");
            }
            if let Some(user) = &entry.user {
                text.push_str("You:\n");
                text.push_str(user);
                text.push_str("\n\n");
            }
            text.push_str(&entry.response);
        }
        text
    }

    fn enforce_limit(&mut self) {
        while self.rendered_len() > ASSISTANT_TRANSCRIPT_LIMIT
            && self.entries.len() > 1
            && self.entries.front().is_some_and(|entry| entry.complete)
        {
            self.entries.pop_front();
            self.evicted = true;
        }
    }

    fn rendered_len(&self) -> usize {
        self.entries
            .iter()
            .map(|entry| {
                entry.user.as_ref().map_or(0, |user| user.len() + 7) + entry.response.len() + 2
            })
            .sum::<usize>()
            + usize::from(self.evicted) * (ASSISTANT_EVICTION_NOTICE.len() + 2)
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
    Refreshed {
        page: CatalogPage,
        cache_text: String,
    },
    View(Section, String),
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
    views: SectionViewState,
    status: String,
    review: Option<Review>,
    operations: Vec<String>,
    last_reload: Instant,
    pending: Option<CorePending>,
    session: u64,
    review_scroll: u16,
    query: CatalogQuery,
    history: Vec<Option<String>>,
    draft: Option<serde_json::Value>,
    quit_when_idle: bool,
    no_color: bool,
    #[cfg(feature = "agent-hosted")]
    agent: HostedAgentState,
    #[cfg(feature = "agent-hosted")]
    assistant_view: AssistantViewState,
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
            views: SectionViewState::default(),
            status: format!("Manual mode; agent: {agent_status}"),
            review: None,
            operations: Vec::new(),
            last_reload: Instant::now(),
            pending: None,
            session: 1,
            review_scroll: 0,
            query: CatalogQuery {
                limit: 25,
                ..Default::default()
            },
            history: Vec::new(),
            draft: None,
            quit_when_idle: false,
            no_color: std::env::var_os("NO_COLOR").is_some(),
            #[cfg(feature = "agent-hosted")]
            agent: hosted_agent,
            #[cfg(feature = "agent-hosted")]
            assistant_view: AssistantViewState::default(),
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
                self.views.catalog = TextViewState::default();
                self.status = format!(
                    "{} versions; {} peers; next page: {}",
                    page.entries.len(),
                    page.coverage.len(),
                    page.next_cursor.is_some()
                );
                self.page = Some(page);
                self.last_reload = Instant::now();
            }
            Ok(CoreUpdate::Refreshed { page, cache_text }) => {
                self.selected = 0;
                self.views.catalog = TextViewState::default();
                self.views.cache.text = Some(cache_text);
                self.views.cache.scroll = 0;
                self.status = format!(
                    "Refresh complete: {} versions; {} peers; Cache view updated.",
                    page.entries.len(),
                    page.coverage.len()
                );
                self.page = Some(page);
                self.last_reload = Instant::now();
            }
            Ok(CoreUpdate::View(section, text)) => {
                let view = self.views.get_mut(section);
                view.text = Some(text);
                view.scroll = 0;
                self.status = "Local view ready; PgUp/PgDn scroll, :export NAME saves text.".into();
            }
            Ok(CoreUpdate::Review(review)) => {
                self.review = Some(review);
                self.review_scroll = 0;
                self.status =
                    "Review complete details; PgUp/PgDn scroll; y confirms, n denies.".into();
            }
            Ok(CoreUpdate::Draft(draft)) => {
                self.views.catalog.text = Some(serde_json::to_string_pretty(&draft).unwrap());
                self.views.catalog.scroll = 0;
                self.draft = Some(draft);
                self.section = Section::Catalog;
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
            }
            Ok(CoreUpdate::Status(status)) => self.status = status,
            Ok(CoreUpdate::Recovered(records)) => {
                self.operations = records;
                self.reload();
            }
            Err(error) => {
                self.status = error;
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
    fn select_section(&mut self, section: Section) {
        self.section = section;
        #[cfg(feature = "agent-hosted")]
        if section == Section::Assistant {
            self.assistant_view.unread = false;
        }
        self.section_view();
    }

    fn scroll_active_down(&mut self) {
        #[cfg(feature = "agent-hosted")]
        if self.section == Section::Assistant {
            self.assistant_view.scroll_back = self.assistant_view.scroll_back.saturating_sub(10);
            return;
        }
        let view = self.views.get_mut(self.section);
        view.scroll = view.scroll.saturating_add(10);
    }

    fn scroll_active_up(&mut self) {
        #[cfg(feature = "agent-hosted")]
        if self.section == Section::Assistant {
            self.assistant_view.scroll_back = self.assistant_view.scroll_back.saturating_add(10);
            return;
        }
        let view = self.views.get_mut(self.section);
        view.scroll = view.scroll.saturating_sub(10);
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
                self.select_section(Section::Help);
                false
            }
            KeyCode::Tab => {
                self.select_section(self.section.next());
                false
            }
            KeyCode::PageDown => {
                self.scroll_active_down();
                false
            }
            KeyCode::PageUp => {
                self.scroll_active_up();
                false
            }
            KeyCode::Char(']') if self.section == Section::Catalog => {
                self.page_next();
                false
            }
            KeyCode::Char('[') if self.section == Section::Catalog => {
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
            KeyCode::Char('j') | KeyCode::Down
                if matches!(self.section, Section::Catalog | Section::Lineage) =>
            {
                self.move_selection(1);
                false
            }
            KeyCode::Char('k') | KeyCode::Up
                if matches!(self.section, Section::Catalog | Section::Lineage) =>
            {
                self.move_selection(-1);
                false
            }
            KeyCode::Enter if self.section == Section::Catalog => {
                self.open_selected();
                false
            }
            KeyCode::Char('e') if self.section == Section::Catalog => {
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
            KeyCode::PageDown => self.review_scroll = self.review_scroll.saturating_add(10),
            KeyCode::PageUp => self.review_scroll = self.review_scroll.saturating_sub(10),
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
        self.section = Section::Catalog;
        self.views.catalog.scroll = 0;
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
            let snapshot = refresh(&project).map_err(|e| e.to_string())?;
            let page = query_catalog(&project, query).map_err(|e| e.to_string())?;
            Ok(CoreUpdate::Refreshed {
                page,
                cache_text: serde_json::to_string_pretty(&snapshot).unwrap(),
            })
        });
    }
    fn section_view(&mut self) {
        let root = self.options.project_root.clone();
        let section = self.section;
        if !matches!(section, Section::Teams | Section::Cache) {
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
                _ => unreachable!("only Teams and Cache load section views"),
            };
            Ok(CoreUpdate::View(
                section,
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
        if self.section == Section::Lineage {
            self.views.lineage.scroll = 0;
        }
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
        self.section = Section::Catalog;
        let root = self.options.project_root.clone();
        self.job(
            "Resolving pinned registered inventory…",
            false,
            move || {
                let project = Project::open(root).map_err(|e| e.to_string())?;
                let product = resolve(&project, &reference, &version, None, false)
                    .map_err(|e| e.to_string())?;
                Ok(CoreUpdate::View(
                    Section::Catalog,
                    serde_json::to_string_pretty(&product).unwrap(),
                ))
            },
        );
    }
    fn example_selected(&mut self) {
        if let Some(entry) = self.selected_entry() {
            self.views.catalog.text = Some(example(&self.options.project_root, entry));
            self.views.catalog.scroll = 0;
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
                self.session += 1; self.section = Section::Catalog; self.review = None; self.draft = None; self.page = None;
                self.views = SectionViewState::default(); self.review_scroll = 0;
                self.options.project_root = selected; self.query.cursor = None; self.history.clear();
                #[cfg(feature = "agent-hosted")] {
                    self.agent.stop();
                    self.agent = HostedAgentState::from_profile_request(self.options.agent_profile.as_deref());
                    self.assistant_view = AssistantViewState::default();
                }
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
                if let Some(draft) = &self.draft { self.review = Some(Review::Inspect(draft.clone())); self.review_scroll = 0; }
            }
            ["draft", "save", name] | ["draft", "save", name, "--overwrite"] => {
                if let Some(draft) = &self.draft { self.prepare_save("drafts", name, serde_json::to_string_pretty(draft).unwrap(), words.len() == 4); }
            }
            ["export", name] | ["export", name, "--overwrite"] => {
                if let Some(text) = self.active_export_text() {
                    self.prepare_save("exports", name, text, words.len() == 3);
                } else {
                    self.status = "The active menu has no content to export.".into();
                }
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
                match serde_json::from_str::<CatalogQuery>(json) { Ok(query) => { self.section = Section::Catalog; self.query = query; self.history.clear(); self.reload(); } Err(e) => self.status = e.to_string() }
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
                self.agent = HostedAgentState::from_profile_request(self.options.agent_profile.as_deref());
                self.assistant_view = AssistantViewState::default(); self.status = self.agent.status();
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
                    let usage = serde_json::to_string_pretty(&ready.usage).unwrap();
                    self.section = Section::Assistant;
                    self.assistant_view.push_local("Assistant session usage", usage, true);
                    self.status = "Assistant session usage; export to retain it before switching profiles.".into();
                }
            },
            ["help"] => self.select_section(Section::Help),
            _ => self.status = "Unknown command or arguments; ? shows command help. Quote paths containing spaces.".into(),
        }
    }
    fn active_export_text(&self) -> Option<String> {
        match self.section {
            Section::Catalog => self.views.catalog.text.clone(),
            Section::Peers => self
                .page
                .as_ref()
                .and_then(|page| serde_json::to_string_pretty(&page.coverage).ok()),
            Section::Operations => Some(format!(
                "Journal: {}\n{}",
                self.journal.root().display(),
                self.operations.join("\n\n")
            )),
            Section::Help => Some(controls::HELP.into()),
            Section::Teams | Section::Cache => self.views.get(self.section).text.clone(),
            Section::Lineage => lineage_summary(self),
            #[cfg(feature = "agent-hosted")]
            Section::Assistant => self.assistant_view.export_text.clone().or_else(|| {
                let text = self.assistant_view.rendered();
                (!text.is_empty()).then_some(text)
            }),
        }
    }

    fn show_draft(&mut self) {
        self.section = Section::Catalog;
        self.views.catalog.text = self
            .draft
            .as_ref()
            .map(|d| serde_json::to_string_pretty(d).unwrap());
        self.views.catalog.scroll = 0;
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
        let display_text = user_text.clone();
        match self.agent.start(project_root, user_text) {
            Ok(()) => {
                self.section = Section::Assistant;
                self.assistant_view.begin(display_text);
                self.status = "Assistant request started; manual navigation remains available. Press s to stop generation.".into();
            }
            Err(error) => self.status = error,
        }
    }

    #[cfg(feature = "agent-hosted")]
    fn poll_agent(&mut self) {
        let was_pending = self.agent.is_pending();
        let streamed = match &self.agent {
            HostedAgentState::Ready(ready)
                if ready.pending.is_some() && !ready.streamed.is_empty() =>
            {
                Some(ready.streamed.clone())
            }
            _ => None,
        };
        if let Some(streamed) = streamed {
            self.assistant_view.stream(&streamed);
        }
        if let Some(run) = self.agent.take_completed() {
            let run_id = match &self.agent {
                HostedAgentState::Ready(ready) => ready.next_run_id - 1,
                _ => 0,
            };
            for tool in &run.tools {
                if let mesh_agent::ToolExecution::DraftPatch { draft, .. } = tool {
                    self.draft = Some(draft.clone());
                    self.views.catalog.text = Some(serde_json::to_string_pretty(draft).unwrap());
                    self.views.catalog.scroll = 0;
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
                self.review_scroll = 0;
            }
            let result = format!(
                "Assistant result #{run_id} ({:?}):\n{}\n\nTool activity:\n{}",
                run.state,
                run.text,
                serde_json::to_string_pretty(&run.tools).unwrap_or_else(|_| "unavailable".into())
            );
            self.assistant_view
                .finish(result, self.section == Section::Assistant);
            self.status = if self.review.is_some() {
                "Assistant proposal awaits local review; press y to confirm or n to cancel.".into()
            } else {
                "Assistant response received. Any mutation proposal is never auto-executed.".into()
            };
        } else if was_pending && !self.agent.is_pending() {
            self.assistant_view
                .cancel(self.section == Section::Assistant);
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

    fn is_pending(&self) -> bool {
        matches!(self, Self::Ready(ready) if ready.pending.is_some())
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
        InputMode::None => key_hints(app.section),
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
        render_review(frame, review, area, app.review_scroll);
    }
}

fn key_hints(section: Section) -> String {
    match section {
        Section::Catalog => "/ search | : command | ↑/↓ select | Enter details | e examples | [/] page | Tab menu | q exit".into(),
        Section::Lineage => "/ search | : command | ↑/↓ select | PgUp/PgDn scroll | Tab menu | q exit".into(),
        #[cfg(feature = "agent-hosted")]
        Section::Assistant => "a ask | s stop | / search | PgUp/PgDn transcript | : command | Tab menu | ? help | q exit".into(),
        _ => "/ search | : command | PgUp/PgDn scroll | r refresh | Tab menu | ? help | q exit".into(),
    }
}

fn render_body(frame: &mut Frame, app: &App, area: Rect) {
    let chunks = Layout::vertical([Constraint::Length(1), Constraint::Min(1)]).split(area);
    let titles: Vec<Line<'_>> = Section::ALL
        .iter()
        .map(|section| {
            let title = section.title().to_string();
            #[cfg(feature = "agent-hosted")]
            let title = if *section == Section::Assistant && app.assistant_view.unread {
                format!("{title}*")
            } else {
                title
            };
            Line::from(title)
        })
        .collect();
    let active_style = if app.no_color {
        Style::default().add_modifier(Modifier::REVERSED | Modifier::BOLD)
    } else {
        Style::default()
            .fg(Color::Black)
            .bg(Color::Cyan)
            .add_modifier(Modifier::BOLD)
    };
    frame.render_widget(
        Tabs::new(titles)
            .select(app.section.index())
            .highlight_style(active_style)
            .divider("|"),
        chunks[0],
    );
    match app.section {
        Section::Catalog => render_catalog(frame, app, chunks[1]),
        Section::Lineage => render_lineage(frame, app, chunks[1]),
        #[cfg(feature = "agent-hosted")]
        Section::Assistant => render_assistant(frame, app, chunks[1]),
        _ => render_section(frame, app, chunks[1]),
    }
}

fn render_version_list(frame: &mut Frame, app: &App, area: Rect, title: &str) {
    let entries = app
        .page
        .as_ref()
        .map(|page| page.entries.as_slice())
        .unwrap_or_default();
    let start = app
        .selected
        .saturating_sub(area.height.saturating_sub(4) as usize);
    let list: Vec<_> = entries
        .iter()
        .enumerate()
        .skip(start)
        .map(|(index, entry)| {
            let text = sanitize_terminal_text(&format!(
                "{} {} {}",
                if index == app.selected { ">" } else { " " },
                entry.reference,
                entry.version
            ));
            let style = if index == app.selected {
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
        List::new(list).block(Block::default().title(title).borders(Borders::ALL)),
        area,
    );
}

fn render_catalog(frame: &mut Frame, app: &App, area: Rect) {
    let cols =
        Layout::horizontal([Constraint::Percentage(40), Constraint::Percentage(60)]).split(area);
    render_version_list(frame, app, cols[0], "Versions  [ / ] page");
    let detail = app.views.catalog.text.clone().unwrap_or_else(|| {
        "Enter: metadata/inventory. e: table/raster examples. PgUp/PgDn: scroll. :project \"PATH\": switch and clear context.".into()
    });
    frame.render_widget(
        Paragraph::new(sanitize_terminal_text(&detail))
            .wrap(Wrap { trim: false })
            .scroll((app.views.catalog.scroll, 0))
            .block(Block::default().title("Catalog").borders(Borders::ALL)),
        cols[1],
    );
}

fn lineage_summary(app: &App) -> Option<String> {
    let selected = app.selected_entry()?;
    let mut versions: Vec<_> = app
        .page
        .as_ref()
        .into_iter()
        .flat_map(|page| &page.entries)
        .filter(|entry| entry.reference == selected.reference)
        .map(|entry| entry.version.as_str())
        .collect();
    versions.sort();
    versions.dedup();
    let history = if versions.len() > 1 {
        versions.join(" -> ")
    } else {
        format!(
            "{} (no previous version in this catalog view)",
            selected.version
        )
    };
    Some(format!(
        "Product: {}\nName: {}\n\nVersion lineage\n{}\n\nSelected version: {}",
        selected.reference, selected.name, history, selected.version
    ))
}

fn render_lineage(frame: &mut Frame, app: &App, area: Rect) {
    let cols =
        Layout::horizontal([Constraint::Percentage(40), Constraint::Percentage(60)]).split(area);
    render_version_list(frame, app, cols[0], "Products / versions");

    let text = lineage_summary(app)
        .unwrap_or_else(|| "No catalog products loaded. Press r to refresh.".into());
    frame.render_widget(
        Paragraph::new(sanitize_terminal_text(&text))
            .wrap(Wrap { trim: false })
            .scroll((app.views.lineage.scroll, 0))
            .block(Block::default().title("Lineage").borders(Borders::ALL)),
        cols[1],
    );
}

fn render_section(frame: &mut Frame, app: &App, area: Rect) {
    let (title, text): (String, String) = match app.section {
        Section::Peers => (
            "Peers".into(),
            app.page
                .as_ref()
                .and_then(|page| serde_json::to_string_pretty(&page.coverage).ok())
                .unwrap_or_else(|| "No catalog coverage loaded. Press r to refresh.".into()),
        ),
        Section::Operations => (
            "Operations".into(),
            format!(
                "Journal: {}\n{}",
                app.journal.root().display(),
                app.operations.join("\n\n")
            ),
        ),
        Section::Help => ("Help".into(), controls::HELP.into()),
        Section::Teams | Section::Cache => (
            app.section.title().into(),
            app.views.get(app.section).text.clone().unwrap_or_else(|| {
                "View not loaded. Select this menu again after the current operation finishes."
                    .into()
            }),
        ),
        Section::Lineage => unreachable!("lineage has a dedicated renderer"),
        Section::Catalog => unreachable!("catalog has a dedicated renderer"),
        #[cfg(feature = "agent-hosted")]
        Section::Assistant => unreachable!("assistant has a dedicated renderer"),
    };
    frame.render_widget(
        Paragraph::new(sanitize_terminal_text(&text))
            .wrap(Wrap { trim: false })
            .scroll((app.views.get(app.section).scroll, 0))
            .block(Block::default().title(title).borders(Borders::ALL)),
        area,
    );
}

#[cfg(feature = "agent-hosted")]
fn render_assistant(frame: &mut Frame, app: &App, area: Rect) {
    let mut text = app.assistant_view.rendered();
    if text.is_empty() {
        text = format!(
            "{}\n\nPress a to compose an assistant request. Assistant output stays in this menu while you inspect other views.",
            app.agent.status()
        );
    }
    let inner_width = area.width.saturating_sub(2).max(1) as usize;
    let inner_height = area.height.saturating_sub(2);
    let lines = wrapped_line_count(&text, inner_width);
    let max_scroll = lines.saturating_sub(inner_height);
    let scroll = max_scroll.saturating_sub(app.assistant_view.scroll_back);
    frame.render_widget(
        Paragraph::new(sanitize_terminal_text(&text))
            .wrap(Wrap { trim: false })
            .scroll((scroll, 0))
            .block(Block::default().title("Assistant").borders(Borders::ALL)),
        area,
    );
}

#[cfg(feature = "agent-hosted")]
fn wrapped_line_count(text: &str, width: usize) -> u16 {
    text.lines()
        .map(|line| line.chars().count().max(1).div_ceil(width))
        .sum::<usize>()
        .min(u16::MAX as usize) as u16
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

    fn test_app() -> (tempfile::TempDir, App) {
        let temp = tempfile::tempdir().unwrap();
        let journal = OperationJournal::open(temp.path().join("journal")).unwrap();
        let app = App::new(
            TuiOptions {
                project_root: temp.path().join("project"),
                agent_profile: None,
            },
            journal,
        );
        (temp, app)
    }

    fn screen_text(app: &App, width: u16, height: u16) -> String {
        let backend = TestBackend::new(width, height);
        let mut terminal = Terminal::new(backend).unwrap();
        terminal.draw(|frame| render(frame, app)).unwrap();
        terminal
            .backend()
            .buffer()
            .content
            .iter()
            .map(|cell| cell.symbol())
            .collect()
    }

    fn catalog_page() -> CatalogPage {
        CatalogPage {
            protocol: "feam.catalog.v1".into(),
            entries: vec![
                CatalogEntry {
                    reference: "product://climate/observations".into(),
                    version: "v1".into(),
                    name: "Observations".into(),
                    manifest_revision: 1,
                    data_kind: mesh_core::peer::DataKind::Table,
                    data_format: mesh_core::peer::DataFormat::Parquet,
                    description: "test".into(),
                    limitations: "none".into(),
                    owner_team: "Climate".into(),
                    quality: "production".into(),
                },
                CatalogEntry {
                    reference: "product://climate/observations".into(),
                    version: "v2".into(),
                    name: "Observations".into(),
                    manifest_revision: 2,
                    data_kind: mesh_core::peer::DataKind::Table,
                    data_format: mesh_core::peer::DataFormat::Parquet,
                    description: "test v2".into(),
                    limitations: "none".into(),
                    owner_team: "Climate".into(),
                    quality: "production".into(),
                },
            ],
            coverage: Vec::new(),
            snapshot_fingerprint: "fixture".into(),
            next_cursor: None,
            result_limit: 25,
            truncated: false,
        }
    }

    #[test]
    fn catalog_product_list_is_hidden_from_full_width_menus() {
        let (_temp, mut app) = test_app();
        app.page = Some(catalog_page());
        app.views.catalog.text = Some("CATALOG DETAIL".into());
        let catalog = screen_text(&app, 100, 30);
        assert!(catalog.contains("Versions"));
        assert!(catalog.contains("product://climate/observations"));
        assert!(catalog.contains("CATALOG DETAIL"));

        app.section = Section::Teams;
        app.views.teams.text = Some("TEAM VIEW ONLY".into());
        let teams = screen_text(&app, 100, 30);
        assert!(teams.contains("TEAM VIEW ONLY"));
        assert!(!teams.contains("Versions"));
        assert!(!teams.contains("product://climate/observations"));

        app.handle_key(KeyCode::Enter);
        app.handle_key(KeyCode::Down);
        app.handle_key(KeyCode::Char('e'));
        app.handle_key(KeyCode::Char(']'));
        assert!(app.pending.is_none());
        assert_eq!(app.selected, 0);
        assert_eq!(app.views.catalog.text.as_deref(), Some("CATALOG DETAIL"));
    }

    #[test]
    fn refresh_result_updates_the_cache_view_without_changing_focus() {
        let (_temp, mut app) = test_app();
        app.section = Section::Cache;
        app.views.cache.text = Some("STALE CACHE".into());
        let (sender, receiver) = mpsc::sync_channel(1);
        sender
            .send(Ok(CoreUpdate::Refreshed {
                page: catalog_page(),
                cache_text: "FRESH CACHE SNAPSHOT".into(),
            }))
            .unwrap();
        app.pending = Some(CorePending {
            receiver,
            session: app.session,
            mutation: false,
        });

        app.poll_core();

        assert_eq!(app.section, Section::Cache);
        assert_eq!(
            app.views.cache.text.as_deref(),
            Some("FRESH CACHE SNAPSHOT")
        );
        assert_eq!(app.page.as_ref().unwrap().entries.len(), 2);
        assert!(app.status.contains("Cache view updated"));
        assert!(screen_text(&app, 100, 30).contains("FRESH CACHE SNAPSHOT"));
    }

    #[test]
    fn lineage_has_a_catalog_style_version_browser_and_product_history() {
        let (_temp, mut app) = test_app();
        app.page = Some(catalog_page());
        app.section = Section::Lineage;
        let lineage = screen_text(&app, 100, 30);
        assert!(lineage.contains("Products / versions"));
        assert!(lineage.contains("product://climate/observations v1"));
        assert!(lineage.contains("product://climate/observations v2"));
        assert!(lineage.contains("v1 -> v2"));
        assert!(lineage.contains("Version lineage"));
        assert!(!lineage.contains("Declared lineage references"));
        assert!(!lineage.contains("Press Enter"));

        app.handle_key(KeyCode::Down);
        assert_eq!(app.selected, 1);
        let updated = screen_text(&app, 100, 30);
        assert!(updated.contains("Selected version: v2"));
        app.handle_key(KeyCode::Enter);
        assert!(app.pending.is_none());
    }

    #[test]
    fn section_results_and_scroll_positions_remain_isolated() {
        let (_temp, mut app) = test_app();
        app.views.catalog.scroll = 4;
        app.section = Section::Teams;
        app.views.teams.scroll = 7;
        app.handle_key(KeyCode::PageDown);
        assert_eq!(app.views.teams.scroll, 17);
        assert_eq!(app.views.catalog.scroll, 4);

        let (sender, receiver) = mpsc::sync_channel(1);
        sender
            .send(Ok(CoreUpdate::View(
                Section::Teams,
                "LATE TEAM RESULT".into(),
            )))
            .unwrap();
        app.pending = Some(CorePending {
            receiver,
            session: app.session,
            mutation: false,
        });
        app.section = Section::Help;
        app.poll_core();
        assert_eq!(app.section, Section::Help);
        assert_eq!(app.views.teams.text.as_deref(), Some("LATE TEAM RESULT"));
        assert!(app.views.help.text.is_none());

        app.review = Some(Review::Inspect(serde_json::json!({})));
        app.review_scroll = 2;
        app.handle_key(KeyCode::PageDown);
        assert_eq!(app.review_scroll, 12);
        assert_eq!(app.views.teams.scroll, 0);
    }

    #[cfg(feature = "agent-hosted")]
    #[test]
    fn assistant_is_a_persistent_bounded_menu() {
        let (_temp, mut app) = test_app();
        assert_eq!(Section::ALL.last(), Some(&Section::Assistant));
        app.assistant_view.begin("show observations".into());
        app.assistant_view.stream("partial response");
        app.section = Section::Help;
        app.assistant_view
            .finish("Assistant result #1: COMPLETE RESPONSE".into(), false);
        assert!(app.assistant_view.unread);
        let help = screen_text(&app, 100, 30);
        assert!(help.contains("Assistant*"));
        assert!(!help.contains("COMPLETE RESPONSE"));

        app.select_section(Section::Assistant);
        assert!(!app.assistant_view.unread);
        let assistant = screen_text(&app, 80, 24);
        assert!(assistant.contains("Assistant"));
        assert!(assistant.contains("COMPLETE RESPONSE"));
        assert!(assistant.contains("/ search"));
        assert!(!assistant.contains("Versions"));
        app.handle_key(KeyCode::PageUp);
        assert_eq!(app.assistant_view.scroll_back, 10);

        app.assistant_view
            .push_local("large one", "a".repeat(700_000), true);
        app.assistant_view
            .push_local("large two", "b".repeat(700_000), true);
        assert!(app.assistant_view.evicted);
        assert!(app.assistant_view.rendered_len() <= ASSISTANT_TRANSCRIPT_LIMIT);
        assert!(
            app.assistant_view
                .rendered()
                .contains("Older assistant entries were removed")
        );

        app.command("agent-profile off".into());
        assert!(app.assistant_view.entries.is_empty());
        assert_eq!(app.assistant_view.scroll_back, 0);
        assert!(!app.assistant_view.unread);
    }

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
