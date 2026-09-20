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

    // `floter register <cmd> [--yes]` is the terminal spelling of the
    // `register` trigger, and the only one that can *finish* a connection. It
    // is handled before the generic trigger path because it has to run
    // synchronously: the user typed it, so it prints its disclosure and its
    // error to stdout and exits with a code, instead of parking a request for
    // a window. Nothing here parses the arguments — `deep_link` normalizes and
    // validates them exactly as it does for a URL.
    //
    // A `None` here means the invocation was an *offer* (a name outside the
    // curated list, without `--yes`): the review surface is the right answer, so
    // the generic path below forwards it as a link and opens the window on it.
    //
    // On Linux a running instance is reached over the control socket, which is
    // also where the terminal transport is recorded; elsewhere (and when
    // nobody is listening) this process performs the connection itself.
    #[cfg(target_os = "linux")]
    if let Some(code) =
        floter_lib::deep_link::register_cli(&arguments, |url, origin| match origin {
            floter_lib::deep_link::RegisterOrigin::Terminal => {
                floter_lib::ipc::send_terminal_deep_link(url)
            }
            floter_lib::deep_link::RegisterOrigin::Link => floter_lib::ipc::send_deep_link(url),
        })
    {
        std::process::exit(code);
    }
    #[cfg(not(target_os = "linux"))]
    if let Some(code) = floter_lib::deep_link::register_cli(&arguments, |_, _| Err(String::new())) {
        std::process::exit(code);
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

    // An external trigger (`floter open`, `floter connect …`, the
    // `floter register <cmd>` offer, or the `floter://…` URL the desktop file
    // hands over) is normalized to its URL and forwarded to the running
    // instance over the control socket, exactly like `--toggle` and `clip`. No
    // second parser: the normalization and the routing both live in
    // `deep_link`.
    //
    // This path is always a **link**. The one terminal-only outcome — a
    // `register` the user already confirmed — was handled above, so whatever
    // arrives here is an offer, and an offer is what a link means.
    #[cfg(target_os = "linux")]
    if let Some(url) = floter_lib::deep_link::canonical_argument(&arguments) {
        if floter_lib::ipc::send_deep_link(&url).is_ok() {
            return;
        }
        // Nobody is listening: this process is the cold start and the setup
        // hook below dispatches the same URL itself.
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
