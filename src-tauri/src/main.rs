// Prevents additional console window on Windows in release, DO NOT REMOVE!!
#![cfg_attr(not(debug_assertions), windows_subsystem = "windows")]

fn main() {
    let arguments = std::env::args().collect::<Vec<_>>();
    if let Some(result) = floter_lib::prepare_terminal_process(&arguments) {
        if let Err(error) = result {
            tracing::error!("floter terminal helper: {error}");
            std::process::exit(1);
        }
        return;
    }

    // `floter --toggle` is the Wayland escape hatch: a compositor shortcut can
    // only launch a command, so this process pokes the instance already running
    // and exits. It has to be handled before anything else — the point is to
    // never pay for app initialization on that path.
    #[cfg(target_os = "linux")]
    if floter_lib::ipc::wants_toggle(arguments.clone()) {
        if let Err(error) = floter_lib::ipc::send_toggle() {
            tracing::error!("floter --toggle: {error}");
            std::process::exit(1);
        }
        return;
    }

    // `floter clip` opens the clipboard plugin page. A running instance is
    // poked through the control socket and this process exits; when nobody is
    // listening this IS the cold start, so fall through to normal app
    // initialization, which records the pending page request for the frontend.
    #[cfg(target_os = "linux")]
    if floter_lib::ipc::wants_clip(arguments.clone()) && floter_lib::ipc::send_clip().is_ok() {
        return;
    }

    // Avoid initializing GTK/WebKit in an ordinary second Linux process. The
    // single-instance plugin below remains the race-safe fallback while the
    // first process is still creating its socket.
    #[cfg(target_os = "linux")]
    {
        let background = arguments
            .iter()
            .skip(1)
            .any(|argument| argument == "--background");
        let reached = if background {
            floter_lib::ipc::send_ping()
        } else {
            floter_lib::ipc::send_show()
        };
        if reached.is_ok() {
            return;
        }
    }

    floter_lib::run()
}
