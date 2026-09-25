//! Bounded semantic research submission. This never observes keyboard events or
//! sends terminal buffers, user text, paths, tool arguments or model reasoning.
use std::io::{Read, Write};
use std::path::{Path, PathBuf};
use std::sync::Arc;
use std::sync::atomic::{AtomicBool, AtomicU8, AtomicUsize, Ordering};
use std::sync::mpsc::{self, SyncSender};
use std::thread;
use std::time::{Duration, Instant};

use chrono::Utc;
use serde::Deserialize;
use serde_json::{Value, json};
use sha2::{Digest, Sha256};

const OFF: u8 = 0;
const CONNECTING: u8 = 1;
const READY: u8 = 2;
const FAILED: u8 = 3;

#[derive(Clone, Deserialize)]
#[serde(deny_unknown_fields)]
struct Config {
    socket: PathBuf,
    capability_file: PathBuf,
    deployment_id: String,
    workspace_id: String,
    generation: u64,
    participant_id: String,
    software_sha256: String,
    profile_sha256: String,
    dataset_sha256: String,
    #[serde(default)]
    synthetic: bool,
}

pub(crate) struct Capture {
    sender: Option<SyncSender<Value>>,
    state: Arc<AtomicU8>,
    lost: Arc<AtomicBool>,
    pending: Arc<AtomicUsize>,
    stop: Arc<AtomicBool>,
    worker: Option<thread::JoinHandle<()>>,
    config: Option<Config>,
    conversation: String,
    stream: String,
    request: String,
    sequence: u64,
}

fn private_bytes(path: &Path) -> Result<Vec<u8>, ()> {
    let metadata = std::fs::symlink_metadata(path).map_err(|_| ())?;
    if !metadata.is_file() || metadata.len() > 16 * 1024 {
        return Err(());
    }
    #[cfg(unix)]
    {
        use std::os::unix::fs::PermissionsExt;
        if metadata.permissions().mode() & 0o027 != 0 {
            return Err(());
        }
    }
    std::fs::read(path).map_err(|_| ())
}
fn valid_id(value: &str) -> bool {
    value.len() == 36
        && value.bytes().enumerate().all(|(index, b)| {
            if [8, 13, 18, 23].contains(&index) {
                b == b'-'
            } else {
                b.is_ascii_digit() || (b'a'..=b'f').contains(&b)
            }
        })
}
fn hash(value: &[u8]) -> String {
    format!("{:x}", Sha256::digest(value))
}
fn digest_id(value: &[u8]) -> String {
    let h = hash(value);
    format!(
        "{}-{}-4{}-a{}-{}",
        &h[..8],
        &h[8..12],
        &h[13..16],
        &h[17..20],
        &h[20..32]
    )
}
fn random_id() -> Result<String, ()> {
    #[cfg(unix)]
    {
        let mut bytes = [0_u8; 32];
        std::fs::File::open("/dev/urandom")
            .map_err(|_| ())?
            .read_exact(&mut bytes)
            .map_err(|_| ())?;
        Ok(digest_id(&bytes))
    }
    #[cfg(not(unix))]
    {
        Err(())
    }
}

impl Capture {
    pub(crate) fn from_environment() -> Self {
        let Some(path) = std::env::var_os("FEAM_EVENT_CONFIG") else {
            return Self::disabled();
        };
        match private_bytes(Path::new(&path))
            .and_then(|bytes| serde_json::from_slice::<Config>(&bytes).map_err(|_| ()))
            .and_then(Self::start)
        {
            Ok(capture) => capture,
            Err(()) => {
                let capture = Self::disabled();
                capture.state.store(FAILED, Ordering::Release);
                capture
            }
        }
    }
    #[cfg(test)]
    pub(crate) fn failed_for_test() -> Self {
        let capture = Self::disabled();
        capture.state.store(FAILED, Ordering::Release);
        capture
    }
    fn disabled() -> Self {
        Self {
            sender: None,
            state: Arc::new(AtomicU8::new(OFF)),
            lost: Arc::new(AtomicBool::new(false)),
            pending: Arc::new(AtomicUsize::new(0)),
            stop: Arc::new(AtomicBool::new(false)),
            worker: None,
            config: None,
            conversation: String::new(),
            stream: String::new(),
            request: String::new(),
            sequence: 0,
        }
    }
    fn start(config: Config) -> Result<Self, ()> {
        if !config.socket.is_absolute()
            || !config.capability_file.is_absolute()
            || config.generation == 0
            || [
                &config.deployment_id,
                &config.workspace_id,
                &config.participant_id,
            ]
            .iter()
            .any(|v| !valid_id(v))
            || [
                &config.software_sha256,
                &config.profile_sha256,
                &config.dataset_sha256,
            ]
            .iter()
            .any(|v| {
                v.len() != 64
                    || !v
                        .bytes()
                        .all(|b| b.is_ascii_digit() || (b'a'..=b'f').contains(&b))
            })
        {
            return Err(());
        }
        let mut capture = Self::disabled();
        capture.conversation = random_id()?;
        capture.stream = random_id()?;
        capture.request = random_id()?;
        capture.config = Some(config.clone());
        capture.state.store(CONNECTING, Ordering::Release);
        let (sender, receiver) = mpsc::sync_channel::<Value>(32);
        capture.sender = Some(sender);
        let (state, pending, stop, lost) = (
            capture.state.clone(),
            capture.pending.clone(),
            capture.stop.clone(),
            capture.lost.clone(),
        );
        capture.worker = Some(thread::spawn(move || {
            while !stop.load(Ordering::Acquire) {
                let Ok(event) = receiver.recv_timeout(Duration::from_millis(100)) else {
                    continue;
                };
                while !stop.load(Ordering::Acquire) {
                    if let Ok(gap) = submit(&config, &event) {
                        if gap {
                            lost.store(true, Ordering::Release);
                        }
                        pending.fetch_sub(1, Ordering::AcqRel);
                        state.store(READY, Ordering::Release);
                        break;
                    }
                    state.store(FAILED, Ordering::Release);
                    thread::sleep(Duration::from_millis(250));
                }
            }
        }));
        capture.emit("request", "session", "started", None, None);
        Ok(capture)
    }
    pub(crate) fn ready(&self) -> bool {
        !self.lost.load(Ordering::Acquire)
            && matches!(self.state.load(Ordering::Acquire), OFF | READY)
    }
    pub(crate) fn status(&self) -> &'static str {
        if self.lost.load(Ordering::Acquire) {
            return "recording gap; new actions paused";
        }
        match self.state.load(Ordering::Acquire) {
            OFF => "recording off",
            CONNECTING => "recording connecting; new actions paused",
            READY => "recording structured events",
            _ => "recording unavailable; new actions paused",
        }
    }
    pub(crate) fn begin_request(&mut self) -> Option<String> {
        if !self.ready() {
            return None;
        }
        if self.sender.is_some() {
            match random_id() {
                Ok(id) => self.request = id,
                Err(()) => {
                    self.lost.store(true, Ordering::Release);
                    return None;
                }
            }
            Some(self.request.clone())
        } else {
            None
        }
    }
    #[cfg(feature = "agent-hosted")]
    pub(crate) fn provider_request(&mut self, id: String) {
        if valid_id(&id) {
            self.request = id;
            self.emit("request", "model", "dispatched", None, None);
        }
    }
    pub(crate) fn emit(
        &mut self,
        kind: &str,
        tool: &str,
        decision: &str,
        operation: Option<&str>,
        outcome: Option<&str>,
    ) {
        let (Some(sender), Some(config)) = (&self.sender, &self.config) else {
            return;
        };
        self.sequence += 1;
        let proposal =
            operation.map(|id| digest_id(format!("{}:{id}", self.conversation).as_bytes()));
        let tool = if tool.len() <= 64
            && tool
                .bytes()
                .all(|b| b.is_ascii_alphanumeric() || b == b'.' || b == b'_')
        {
            tool
        } else {
            "unknown_tool"
        };
        let payload = json!({"tool": tool, "decision": decision, "outcome": outcome.unwrap_or("")});
        let event = json!({"protocol":"feam.web.event.v1","event_id":digest_id(format!("{}:{}",self.stream,self.sequence).as_bytes()),"stream_id":self.stream,"sequence":self.sequence,"deployment_id":config.deployment_id,"workspace_id":config.workspace_id,"generation":config.generation,"participant_id":config.participant_id,"conversation_id":self.conversation,"request_id":self.request,"proposal_id":proposal,"occurred_at":Utc::now().to_rfc3339_opts(chrono::SecondsFormat::Millis,true),"kind":kind,"trust":"client_reported","software_sha256":config.software_sha256,"profile_sha256":config.profile_sha256,"dataset_sha256":config.dataset_sha256,"payload_sha256":hash(serde_json::to_string(&payload).unwrap_or_default().as_bytes()),"payload":payload,"synthetic":config.synthetic});
        self.pending.fetch_add(1, Ordering::AcqRel);
        if sender.try_send(event).is_err() {
            self.pending.fetch_sub(1, Ordering::AcqRel);
            self.lost.store(true, Ordering::Release);
        }
    }
    pub(crate) fn finish(&self, timeout: Duration) -> bool {
        let start = Instant::now();
        while self.pending.load(Ordering::Acquire) > 0 && start.elapsed() < timeout {
            thread::sleep(Duration::from_millis(10));
        }
        self.pending.load(Ordering::Acquire) == 0 && !self.lost.load(Ordering::Acquire)
    }
}
impl Drop for Capture {
    fn drop(&mut self) {
        self.stop.store(true, Ordering::Release);
        self.sender.take();
        if let Some(worker) = self.worker.take() {
            let _ = worker.join();
        }
    }
}

#[cfg(unix)]
fn submit(config: &Config, event: &Value) -> Result<bool, ()> {
    use std::os::unix::net::UnixStream;
    let token = String::from_utf8(private_bytes(&config.capability_file)?).map_err(|_| ())?;
    let token = token.trim();
    if token.len() != 43
        || !token
            .bytes()
            .all(|b| b.is_ascii_alphanumeric() || b == b'_' || b == b'-')
    {
        return Err(());
    }
    let body = serde_json::to_vec(event).map_err(|_| ())?;
    if body.len() > 64 * 1024 {
        return Err(());
    }
    let raw =
        socket2::Socket::new(socket2::Domain::UNIX, socket2::Type::STREAM, None).map_err(|_| ())?;
    let address = socket2::SockAddr::unix(&config.socket).map_err(|_| ())?;
    raw.connect_timeout(&address, Duration::from_secs(2))
        .map_err(|_| ())?;
    let descriptor: std::os::fd::OwnedFd = raw.into();
    let mut socket = UnixStream::from(descriptor);
    socket
        .set_read_timeout(Some(Duration::from_secs(2)))
        .map_err(|_| ())?;
    socket
        .set_write_timeout(Some(Duration::from_secs(2)))
        .map_err(|_| ())?;
    write!(socket, "POST /v1/events HTTP/1.1\r\nHost: collector\r\nAuthorization: Bearer {token}\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n", body.len()).map_err(|_| ())?;
    socket.write_all(&body).map_err(|_| ())?;
    let mut response = Vec::new();
    let deadline = Instant::now() + Duration::from_secs(2);
    let mut expected_length = None;
    loop {
        let remaining = deadline.checked_duration_since(Instant::now()).ok_or(())?;
        socket.set_read_timeout(Some(remaining)).map_err(|_| ())?;
        let mut chunk = [0_u8; 1024];
        let n = socket.read(&mut chunk).map_err(|_| ())?;
        if n == 0 {
            break;
        }
        response.extend_from_slice(&chunk[..n]);
        if response.len() > 16384 {
            return Err(());
        }
        if expected_length.is_none()
            && let Some(header_end) = response.windows(4).position(|w| w == b"\r\n\r\n")
        {
            let headers = std::str::from_utf8(&response[..header_end]).map_err(|_| ())?;
            let mut length = None;
            for line in headers.lines().skip(1) {
                let (name, value) = line.split_once(':').ok_or(())?;
                if name.eq_ignore_ascii_case("transfer-encoding") {
                    return Err(());
                }
                if name.eq_ignore_ascii_case("content-length") {
                    if length.is_some() {
                        return Err(());
                    }
                    length = Some(value.trim().parse::<usize>().map_err(|_| ())?);
                }
            }
            let length = length.ok_or(())?;
            if length > 16384 {
                return Err(());
            }
            expected_length = Some(header_end.checked_add(4 + length).ok_or(())?);
            if expected_length.is_some_and(|length| length > 16384) {
                return Err(());
            }
        }
        if let Some(length) = expected_length
            && response.len() >= length
        {
            if response.len() != length {
                return Err(());
            }
            break;
        }
    }
    if expected_length != Some(response.len()) {
        return Err(());
    }
    let response = String::from_utf8(response).map_err(|_| ())?;
    let (headers, body) = response.split_once("\r\n\r\n").ok_or(())?;
    if !headers.starts_with("HTTP/1.1 200 ") && !headers.starts_with("HTTP/1.0 200 ") {
        return Err(());
    }
    let ack: Value = serde_json::from_str(body).map_err(|_| ())?;
    if ack["event_id"] != event["event_id"]
        || ack["sequence"] != event["sequence"]
        || !["durable", "duplicate"].contains(&ack["status"].as_str().unwrap_or(""))
    {
        return Err(());
    }
    Ok(ack["gap"].as_bool().unwrap_or(false))
}
#[cfg(not(unix))]
fn submit(_: &Config, _: &Value) -> Result<bool, ()> {
    Err(())
}

#[cfg(test)]
mod tests {
    use super::*;
    #[test]
    fn disabled_capture_preserves_manual_workflows() {
        let c = Capture::disabled();
        assert!(c.ready());
        assert_eq!(c.status(), "recording off");
    }
    #[test]
    fn identifiers_are_opaque_and_valid() {
        let id = random_id().unwrap();
        assert!(valid_id(&id));
        assert_ne!(id, random_id().unwrap());
        assert!(!valid_id("/private/path"));
    }
    #[cfg(unix)]
    #[test]
    fn private_config_and_token_reject_public_or_symlink_files() {
        use std::os::unix::fs::{PermissionsExt, symlink};
        let dir = tempfile::tempdir().unwrap();
        let path = dir.path().join("private");
        std::fs::write(&path, b"data").unwrap();
        std::fs::set_permissions(&path, std::fs::Permissions::from_mode(0o644)).unwrap();
        assert!(private_bytes(&path).is_err());
        std::fs::set_permissions(&path, std::fs::Permissions::from_mode(0o600)).unwrap();
        assert!(private_bytes(&path).is_ok());
        let link = dir.path().join("link");
        symlink(path, &link).unwrap();
        assert!(private_bytes(&link).is_err());
    }
    #[cfg(unix)]
    #[test]
    fn semantic_events_retry_exactly_and_never_capture_private_text() {
        use std::io::{BufRead, BufReader};
        use std::os::unix::fs::PermissionsExt;
        use std::os::unix::net::UnixListener;
        let dir = tempfile::tempdir().unwrap();
        let socket = dir.path().join("events.sock");
        let token = dir.path().join("capability");
        std::fs::write(&token, "a".repeat(43)).unwrap();
        std::fs::set_permissions(&token, std::fs::Permissions::from_mode(0o640)).unwrap();
        let listener = UnixListener::bind(&socket).unwrap();
        let worker = thread::spawn(move || {
            let mut events = Vec::new();
            for index in 0..5 {
                let (mut stream, _) = listener.accept().unwrap();
                let mut reader = BufReader::new(stream.try_clone().unwrap());
                let mut length = 0;
                loop {
                    let mut line = String::new();
                    reader.read_line(&mut line).unwrap();
                    if line == "\r\n" {
                        break;
                    }
                    if let Some(value) = line.strip_prefix("Content-Length: ") {
                        length = value.trim().parse().unwrap();
                    }
                }
                let mut body = vec![0; length];
                reader.read_exact(&mut body).unwrap();
                let event: Value = serde_json::from_slice(&body).unwrap();
                events.push(event.clone());
                if index == 0 {
                    stream.write_all(b"HTTP/1.1 503 Unavailable\r\nContent-Length: 0\r\nConnection: close\r\n\r\n").unwrap();
                } else {
                    let body=json!({"event_id":event["event_id"],"sequence":event["sequence"],"status":"durable"}).to_string();
                    write!(
                        stream,
                        "HTTP/1.1 200 OK\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{}",
                        body.len(),
                        body
                    )
                    .unwrap();
                }
            }
            events
        });
        let id = || random_id().unwrap();
        let config = Config {
            socket,
            capability_file: token,
            deployment_id: id(),
            workspace_id: id(),
            generation: 1,
            participant_id: id(),
            software_sha256: "a".repeat(64),
            profile_sha256: "b".repeat(64),
            dataset_sha256: "c".repeat(64),
            synthetic: true,
        };
        let mut capture = Capture::start(config).unwrap();
        let deadline = Instant::now() + Duration::from_secs(3);
        while !capture.ready() && Instant::now() < deadline {
            thread::sleep(Duration::from_millis(10));
        }
        assert!(capture.ready());
        for decision in ["presented", "edited", "denied"] {
            capture.emit(
                "review",
                "publication",
                decision,
                Some("operation-private-/Users/person/source"),
                None,
            );
        }
        assert!(
            capture.finish(Duration::from_secs(5)),
            "status={} pending={} lost={}",
            capture.status(),
            capture.pending.load(Ordering::Acquire),
            capture.lost.load(Ordering::Acquire)
        );
        drop(capture);
        let events = worker.join().unwrap();
        assert_eq!(events[0], events[1]);
        assert_eq!(events[4]["sequence"], 4);
        for event in events {
            assert_eq!(event["trust"], "client_reported");
            assert!(valid_id(
                event["proposal_id"]
                    .as_str()
                    .unwrap_or(event["event_id"].as_str().unwrap())
            ));
            assert!(!event.to_string().contains("/Users/"));
            assert!(!event.to_string().contains("operation-private"));
        }
    }
}
