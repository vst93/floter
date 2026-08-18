//! Control socket, so a compositor shortcut can summon the panel on Wayland.
//!
//! `tauri-plugin-global-shortcut` takes its bindings through X11, and X11 grabs
//! are silently ignored under Wayland: the plugin reports success, the key never
//! fires. That is how the protocol is designed — only the compositor may own a
//! global binding — so the escape hatch is to let the compositor own it. The
//! user points a custom shortcut (GNOME Settings → Keyboard, KDE System
//! Settings → Shortcuts, ...) at `floter --toggle`; that short-lived process
//! writes one line to this socket and exits, and the instance already running
//! toggles the panel.
//!
//! The protocol is deliberately the simplest thing that works: newline
//! delimited commands, of which only `toggle` is understood.

use std::io::{BufRead, BufReader, Error, ErrorKind, Write};
use std::os::unix::net::{UnixListener, UnixStream};
use std::path::{Path, PathBuf};
use std::sync::atomic::{AtomicBool, Ordering};
use tauri::{AppHandle, Manager};

/// Whether this process is the one holding the listener. Only the owner may
/// unlink the socket on exit — an instance that failed to bind because another
/// one is already live must leave that live socket alone.
static OWNS_SOCKET: AtomicBool = AtomicBool::new(false);

/// Path of the control socket.
///
/// `$XDG_RUNTIME_DIR` is the right home for it: the directory is per-user,
/// mode `0700`, and cleared when the session ends. Sessions that provide no
/// runtime dir fall back to `/tmp`, where the uid has to go in the name because
/// a bare `/tmp/floter.sock` would be owned by whoever logged in first and
/// unusable by everyone else.
pub fn socket_path() -> PathBuf {
    if let Some(dir) = std::env::var_os("XDG_RUNTIME_DIR") {
        let dir = PathBuf::from(dir);
        if dir.is_absolute() {
            return dir.join("floter.sock");
        }
    }
    PathBuf::from(format!("/tmp/floter-{}.sock", uid()))
}

/// This process' effective uid, read off `/proc` rather than pulled in with a
/// libc dependency. `/proc` is always mounted on the only platform this module
/// is compiled for.
fn uid() -> u32 {
    use std::os::unix::fs::MetadataExt;

    std::fs::metadata("/proc/self")
        .map(|metadata| metadata.uid())
        .unwrap_or(0)
}

/// Ask the running instance to toggle the panel.
///
/// This is the whole of the `--toggle` CLI path, and it stays deliberately
/// cheap: one connect, one write, exit. No window, no tray, no event loop —
/// a compositor shortcut has to feel like a key press, not like a launch.
pub fn send_toggle() -> Result<(), String> {
    send_command_to(&socket_path(), "toggle")
}

pub fn send_show() -> Result<(), String> {
    send_command_to(&socket_path(), "show")
}

pub fn send_ping() -> Result<(), String> {
    send_command_to(&socket_path(), "ping")
}

fn send_command_to(path: &Path, command: &str) -> Result<(), String> {
    let mut stream = UnixStream::connect(path).map_err(|error| {
        format!(
            "cannot reach a running floter at {}: {error}",
            path.display()
        )
    })?;
    stream
        .write_all(format!("{command}\n").as_bytes())
        .and_then(|()| stream.flush())
        .map_err(|error| error.to_string())
}

/// Start serving the control socket on a background thread.
///
/// Failure is not fatal. The socket is a convenience for sessions where the
/// global shortcut cannot work; an app that could not bind it is still fully
/// usable through the tray, the X11 shortcut, or a later `--toggle` once the
/// conflict is gone. So a warning goes to stderr and startup continues.
pub fn serve(app: &AppHandle) {
    let path = socket_path();
    let listener = match bind(&path) {
        Ok(listener) => listener,
        Err(error) => {
            eprintln!(
                "failed to listen on {}: {error} (`floter --toggle` will not work)",
                path.display()
            );
            return;
        }
    };
    OWNS_SOCKET.store(true, Ordering::SeqCst);

    let app = app.clone();
    let spawned = std::thread::Builder::new()
        .name("floter-ipc".into())
        .spawn(move || {
            // `incoming()` never ends; the thread is detached and dies with the
            // process, which is also when the socket is unlinked.
            for stream in listener.incoming().flatten() {
                serve_connection(&app, stream);
            }
        });
    if let Err(error) = spawned {
        eprintln!("failed to spawn the ipc thread: {error}");
    }
}

/// Bind the listener, reclaiming a socket left behind by a crashed instance.
///
/// A Unix socket node outlives the process that created it, so `AddrInUse`
/// means one of two very different things. Connecting is the only way to tell
/// them apart: a refused connection is a stale node and may be replaced, while
/// a successful one means a live instance owns the binding and this process must
/// keep its hands off.
fn bind(path: &Path) -> std::io::Result<UnixListener> {
    match UnixListener::bind(path) {
        Ok(listener) => Ok(listener),
        Err(error) if error.kind() == ErrorKind::AddrInUse => {
            if UnixStream::connect(path).is_ok() {
                return Err(Error::new(
                    ErrorKind::AddrInUse,
                    "another floter instance owns the socket",
                ));
            }
            std::fs::remove_file(path)?;
            UnixListener::bind(path)
        }
        Err(error) => Err(error),
    }
}

/// Read newline-delimited commands off one connection.
///
/// Unknown lines are ignored rather than reported, so a newer CLI talking to an
/// older instance degrades quietly instead of erroring out.
fn serve_connection(app: &AppHandle, stream: UnixStream) {
    let reader = BufReader::new(stream);
    for line in reader.lines().map_while(Result::ok) {
        let command = line.trim();
        if command == "ping" {
            continue;
        }
        if command != "toggle" && command != "show" {
            continue;
        }
        let toggle = command == "toggle";
        // Showing and hiding windows has to happen on the main thread; this is
        // an arbitrary background thread.
        let handle = app.clone();
        let _ = app.run_on_main_thread(move || {
            if toggle {
                crate::toggle_window_visibility(&handle);
            } else if let Some(window) = handle.get_webview_window("main") {
                let state = handle.state::<crate::AppState>();
                let _ = crate::reveal_saved_mode(&window, &state);
            }
        });
    }
}

/// Unlink the socket on shutdown so the next start binds without having to
/// reclaim a stale node. A no-op in an instance that never owned the listener.
pub fn cleanup() {
    if OWNS_SOCKET.swap(false, Ordering::SeqCst) {
        let _ = std::fs::remove_file(socket_path());
    }
}

/// Whether the given argument list asks for the `--toggle` CLI path.
pub fn wants_toggle<I: IntoIterator<Item = String>>(args: I) -> bool {
    args.into_iter().skip(1).any(|arg| arg == "--toggle")
}

#[cfg(test)]
mod tests {
    use super::*;

    /// A socket path of our own, so the tests never touch the one a real
    /// instance may be holding in `$XDG_RUNTIME_DIR`.
    fn scratch_path(name: &str) -> PathBuf {
        let path = std::env::temp_dir().join(format!(
            "floter-test-{}-{name}.sock",
            std::process::id()
        ));
        let _ = std::fs::remove_file(&path);
        path
    }

    #[test]
    fn recognizes_the_toggle_flag() {
        let args = |list: &[&str]| {
            list.iter()
                .map(|arg| (*arg).to_string())
                .collect::<Vec<_>>()
        };

        assert!(wants_toggle(args(&["floter", "--toggle"])));
        assert!(!wants_toggle(args(&["floter"])));
        assert!(!wants_toggle(args(&["floter", "--version"])));
        // The program name is not a flag, even when it is spelled like one.
        assert!(!wants_toggle(args(&["--toggle"])));
    }

    #[test]
    fn delivers_a_toggle_line() {
        let path = scratch_path("delivers");
        let listener = bind(&path).expect("bind");

        send_command_to(&path, "toggle").expect("send");

        let (stream, _) = listener.accept().expect("accept");
        let lines: Vec<String> = BufReader::new(stream)
            .lines()
            .map_while(Result::ok)
            .collect();
        assert_eq!(lines, vec!["toggle".to_string()]);

        let _ = std::fs::remove_file(&path);
    }

    #[test]
    fn reclaims_a_socket_left_by_a_crash() {
        let path = scratch_path("reclaims");
        // Dropping a listener leaves its filesystem node behind, which is
        // exactly the state a crashed instance leaves the socket in.
        drop(bind(&path).expect("first bind"));
        assert!(path.exists());

        let reclaimed = bind(&path);

        assert!(reclaimed.is_ok(), "stale socket should be replaced");
        let _ = std::fs::remove_file(&path);
    }

    #[test]
    fn refuses_a_socket_a_live_instance_owns() {
        let path = scratch_path("refuses");
        let _listener = bind(&path).expect("first bind");

        let error = bind(&path).expect_err("second bind should fail");

        assert_eq!(error.kind(), ErrorKind::AddrInUse);
        let _ = std::fs::remove_file(&path);
    }

    #[test]
    fn cleanup_leaves_a_socket_this_process_does_not_own() {
        let path = socket_path();
        let existed = path.exists();

        // `serve` never ran, so nothing may be unlinked.
        cleanup();

        assert_eq!(path.exists(), existed);
    }
}
