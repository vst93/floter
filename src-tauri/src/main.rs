// Prevents additional console window on Windows in release, DO NOT REMOVE!!
#![cfg_attr(not(debug_assertions), windows_subsystem = "windows")]

fn main() {
    // `floter --toggle` is the Wayland escape hatch: a compositor shortcut can
    // only launch a command, so this process pokes the instance already running
    // and exits. It has to be handled before anything else — the point is to
    // never pay for app initialization on that path.
    #[cfg(target_os = "linux")]
    if floter_lib::ipc::wants_toggle(std::env::args()) {
        if let Err(error) = floter_lib::ipc::send_toggle() {
            eprintln!("floter --toggle: {error}");
            std::process::exit(1);
        }
        return;
    }

    floter_lib::run()
}
