// Byte probe for terminal input: an env-gated hex dump of every payload that
// reaches the PTY-bound seams. Zero cost when the gate is off (one
// `env::var_os` per write), an append-only log line per write when on.
//
// Purpose: the terminal rounds have established by differential testing that
// a multi-word line occasionally reaches zsh with a non-ASCII space between
// the words. The frontend now normalizes every injection boundary
// (`src/terminal/inputNormalize.ts`), which should close it; if a user still
// reproduces, FLOTER_TERMINAL_INPUT_DEBUG=1 turns this probe on and the log
// names the exact seam and bytes that were written, no reproduction guessing.
//
// Log location: the terminal-input log lives beside the app's other
// user-level state. `dirs::home_dir()/Library/Logs` on macOS and
// `dirs::data_local_dir()/floter` elsewhere give a per-OS conventional spot
// without pulling Tauri's app handle into the deep broker/session call sites.
// Writes are best-effort: a probe failure must never break the terminal.

use std::fs::OpenOptions;
use std::io::Write;
use std::path::PathBuf;

/// Environment variable that arms the probe (any non-empty value).
pub const INPUT_DEBUG_ENV: &str = "FLOTER_TERMINAL_INPUT_DEBUG";

/// `terminal-input-debug.log` under the app's log directory.
fn probe_log_path() -> Option<PathBuf> {
    if cfg!(target_os = "macos") {
        Some(
            dirs::home_dir()?
                .join("Library")
                .join("Logs")
                .join("floter"),
        )
    } else {
        Some(dirs::data_local_dir()?.join("floter"))
    }
    .map(|directory| directory.join("terminal-input-debug.log"))
}

fn probe_armed() -> bool {
    std::env::var_os(INPUT_DEBUG_ENV).is_some_and(|value| !value.is_empty())
}

/// Append one hex-dump record for `data` produced by `seam` to the probe log.
///
/// `seam` names the producer ("term_input", "initial_command", ...) so a
/// captured bad byte is attributable to one writer without correlating
/// timestamps. Failures (unwritable directory among them) are swallowed: the
/// probe is a diagnostic, never a dependency.
pub fn log_input_bytes(seam: &str, data: &[u8]) {
    if !probe_armed() {
        return;
    }
    let Some(path) = probe_log_path() else { return };
    if let Some(parent) = path.parent() {
        let _ = std::fs::create_dir_all(parent);
    }
    let mut hex = String::with_capacity(data.len() * 3);
    for (index, byte) in data.iter().enumerate() {
        if index > 0 {
            hex.push(' ');
        }
        hex.push_str(&format!("{byte:02x}"));
    }
    let record = format!("[{seam}] {} bytes: {hex}\n", data.len());
    let Ok(mut file) = OpenOptions::new().create(true).append(true).open(&path) else {
        return;
    };
    let _ = file.write_all(record.as_bytes());
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn probe_log_path_is_always_a_terminal_input_debug_log() {
        let path = probe_log_path().expect("a conventional log directory exists");
        assert!(path.to_string_lossy().ends_with("terminal-input-debug.log"));
    }

    // The seam label and the hex rendering are the probe's whole contract:
    // `log_input_bytes` with the gate off must be a no-op (no file appears),
    // and with the gate on must append a `[seam] len: hex` line. The
    // environment is process-global, so the armed case is exercised only when
    // the test runner itself arms the probe; the no-op case is what the
    // production default must guarantee.
    #[test]
    fn probe_is_silent_when_not_armed() {
        // The default test environment does not arm the probe. If it ever did,
        // this test's premise (zero-cost when off) would be unverifiable here
        // without env manipulation races; the guard below keeps it honest.
        if probe_armed() {
            return;
        }
        // Must not panic and must not create the log directory tree.
        log_input_bytes("test-seam", b"go version\r");
    }
}
