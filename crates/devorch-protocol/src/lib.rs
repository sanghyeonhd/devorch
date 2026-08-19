//! Devorch protocol contracts.
//!
//! This crate is the shared vocabulary of the whole system: identifiers, the
//! normalized agent event stream, and the candidate change inventory. It has no
//! I/O and no platform-specific dependencies, so every other crate — core,
//! adapters, store, GUI — can depend on it without inheriting a runtime.

pub mod change;
pub mod event;
pub mod id;

pub use change::{check_inventory, ChangeInventory, InventoryViolation};
pub use event::{
    AgentEvent, AgentKind, AgentRuntimeMode, Completed, EventEnvelope, Failed, FailureClass,
    FileChangeKind, FileChanged, Usage,
};
pub use id::{EventId, MissionId, SessionId, TaskId, WorkspaceId};
