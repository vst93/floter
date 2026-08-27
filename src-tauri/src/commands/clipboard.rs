//! Direct system clipboard access for the terminal.
//!
//! The webview's `navigator.clipboard` is not dependable here: WebKitGTK
//! implements neither `readText` nor `writeText` without extra opt-ins, and
//! WKWebView gates reads behind user-gesture policies that reject programmatic
//! paste from the keydown path. Every platform floter ships on is already
//! covered by arboard through the clipboard-history monitor, so copy and paste
//! go through the same crate instead of the JS API.
//!
//! These commands intentionally bypass the history store: the monitor captures
//! copies made *through them* like any other system-wide change, so a
//! terminal Ctrl+Shift+C lands in the history too — one code path, no
//! double-recording logic.

/// Put text on the system clipboard.
///
/// Clipboard access can fail because another application holds it open; that
/// surfaces to the caller as an error string, never a panic.
#[tauri::command]
pub fn clipboard_write_text(text: String) -> Result<(), String> {
    let mut clipboard = arboard::Clipboard::new().map_err(|error| error.to_string())?;
    clipboard.set_text(text).map_err(|error| error.to_string())
}

/// Read text back off the system clipboard.
///
/// Returns an error string when the clipboard holds non-text content or
/// another application keeps the platform API busy; the caller treats both as
/// "nothing to paste".
#[tauri::command]
pub fn clipboard_read_text() -> Result<String, String> {
    let mut clipboard = arboard::Clipboard::new().map_err(|error| error.to_string())?;
    clipboard.get_text().map_err(|error| error.to_string())
}
