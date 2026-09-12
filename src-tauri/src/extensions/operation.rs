//! Shared types for operation progress reporting and cancellation.
//!
//! Progress events are emitted on the Tauri event bus under the name
//! `"extension-op-progress"` at the key phase transitions of install,
//! uninstall, repair, and reprobe. Cancellation is signalled through a
//! single `CancelToken` held in `ExtensionState` for the lifetime of the
//! active operation; the cancel command flips the atomic flag and returns
//! immediately while the operation checks it at every async yield point.

use serde::Serialize;
use std::sync::{
    atomic::{AtomicBool, Ordering},
    Arc,
};

/// Lightweight cancel token backed by an `Arc<AtomicBool>`.
///
/// The command layer mints one before every long operation and stores it in
/// `ExtensionState::active_cancel`.  Inner functions call `is_cancelled()`
/// at each major await point and return `Err("Operation cancelled")` when
/// the flag is set.
#[derive(Clone, Debug, Default)]
pub(crate) struct CancelToken(Arc<AtomicBool>);

impl CancelToken {
    pub(crate) fn new() -> Self {
        Self(Arc::new(AtomicBool::new(false)))
    }

    pub(crate) fn is_cancelled(&self) -> bool {
        self.0.load(Ordering::Relaxed)
    }

    pub(crate) fn cancel(&self) {
        self.0.store(true, Ordering::Relaxed);
    }
}

/// Payload emitted on `"extension-op-progress"`.
#[derive(Debug, Clone, Serialize)]
#[serde(rename_all = "camelCase")]
pub(crate) struct OperationProgress {
    /// Extension this operation targets.
    pub(crate) extension_id: String,
    /// Operation kind: "install" | "uninstall" | "repair" | "reprobe".
    pub(crate) kind: String,
    /// Human-readable phase label shown in the UI.
    pub(crate) phase: String,
    /// Optional 0-100 completion estimate.
    pub(crate) percent: Option<u8>,
}
