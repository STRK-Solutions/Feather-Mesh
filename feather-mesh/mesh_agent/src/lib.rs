//! Provider-neutral, bounded agent harness for Feather Mesh.
//!
//! `mesh_agent` has no terminal authority. It exposes typed proposals and
//! review-bound operations; `mesh_tui` owns confirmations and rendering.

pub mod config;
pub mod disclosure;
pub mod evaluation;
pub mod harness;
pub mod provider;
pub mod tools;

pub use config::{AgentConfig, AgentProfile, ProfileError};
pub use harness::{AgentEvent, AgentHarness, AgentRun, AgentRunState};
#[cfg(feature = "hosted")]
pub use provider::RouterProvider;
pub use provider::{CancellationToken, FakeProvider, ModelProvider, ProviderError, ProviderEvent};
pub use tools::{ConfirmableOperation, ToolExecution, ToolExecutor, ToolProposal};
