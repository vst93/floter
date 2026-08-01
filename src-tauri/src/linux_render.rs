//! Keeping floter startable on Linux machines where WebKitGTK cannot get a GPU.
//!
//! WebKitGTK initializes EGL before it will show anything, and on a few driver
//! and compositor combinations — AMD on Wayland with a recent Mesa is the one
//! reported most often — that initialization fails outright:
//!
//! ```text
//! Could not create default EGL display: EGL_BAD_PARAMETER
//! ```
//!
//! The failure happens inside the web process, below any error this program
//! could catch, and it takes the window with it. So the recovery cannot be a
//! `Result`: it has to be a decision made *before* the webview exists, which is
//! what this module is. It rests on one observation — a start that never
//! reached its window is a start that failed — recorded in a marker file:
//!
//! * [`prepare`] runs before anything touches GTK. It writes the marker, and if
//!   the previous run left one behind it turns the GPU off for this run.
//! * [`mark_started`] clears the marker once the window is up, so a crash later
//!   in the session (or a `SIGKILL`, or a machine losing power) is not mistaken
//!   for a startup failure.
//!
//! The result is a launcher that comes up by itself on the second attempt
//! instead of never coming up at all. `--software-rendering` forces the same
//! state without waiting for a failure, and `--gpu` refuses it — including the
//! automatic one, for the case where the marker is wrong and the user would
//! rather see the crash.

use std::ffi::OsStr;
use std::fs;
use std::path::PathBuf;

/// Environment WebKitGTK reads before it initializes its renderer.
///
/// The first two are WebKit's own escape hatches (skip the DMA-BUF path, skip
/// accelerated compositing) and the last two take Mesa off the hardware driver,
/// for the case where EGL itself is what fails rather than WebKit's use of it.
const SOFTWARE_RENDERING: [(&str, &str); 4] = [
    ("WEBKIT_DISABLE_DMABUF_RENDERER", "1"),
    ("WEBKIT_DISABLE_COMPOSITING_MODE", "1"),
    ("LIBGL_ALWAYS_SOFTWARE", "1"),
    ("GALLIUM_DRIVER", "llvmpipe"),
];

const MARKER_FILE: &str = "startup-in-progress";

/// Bundle identifier from `tauri.conf.json`, repeated because this runs before
/// there is an `AppHandle` to ask. Tauri derives the cache directory from the
/// same string, so both agree on where floter's cache lives.
const IDENTIFIER: &str = "com.v.floter";

/// Decide how this run should render, and remember that it has started.
///
/// Called from `run()` before the Tauri builder, which is the last moment the
/// environment can still be changed: GTK reads these variables when it is
/// initialized and never again.
pub fn prepare<I, S>(args: I)
where
    I: IntoIterator<Item = S>,
    S: AsRef<OsStr>,
{
    let requested = Request::from(args);
    let recovering = requested == Request::Auto && previous_start_failed();

    if requested == Request::Software || recovering {
        if recovering {
            eprintln!(
                "floter: the previous start did not reach its window, so this one runs without \
                 the GPU. Pass --gpu to try hardware rendering again."
            );
        }
        for (key, value) in SOFTWARE_RENDERING {
            // Never over an explicit setting: someone debugging their own
            // combination of variables should get the one they asked for.
            if std::env::var_os(key).is_none() {
                std::env::set_var(key, value);
            }
        }
    }

    // Written even when the GPU is already off, because software rendering is
    // not the only way a start can fail and the next run should know either way.
    write_marker();
}

/// Note that the window came up, so the next start trusts the GPU again.
pub fn mark_started() {
    if let Some(path) = marker_path() {
        let _ = fs::remove_file(path);
    }
}

/// What the command line and environment asked for, if anything.
#[derive(PartialEq, Eq, Clone, Copy)]
enum Request {
    /// Fall back only if the previous start failed.
    Auto,
    Software,
    Gpu,
}

impl Request {
    fn from<I, S>(args: I) -> Self
    where
        I: IntoIterator<Item = S>,
        S: AsRef<OsStr>,
    {
        for arg in args {
            match arg.as_ref().to_string_lossy().as_ref() {
                "--software-rendering" => return Self::Software,
                "--gpu" => return Self::Gpu,
                _ => {}
            }
        }

        match std::env::var("FLOTER_SOFTWARE_RENDERING").as_deref() {
            Ok("1" | "true") => Self::Software,
            Ok("0" | "false") => Self::Gpu,
            _ => Self::Auto,
        }
    }
}

fn marker_path() -> Option<PathBuf> {
    Some(dirs::cache_dir()?.join(IDENTIFIER).join(MARKER_FILE))
}

fn previous_start_failed() -> bool {
    marker_path().is_some_and(|path| path.exists())
}

fn write_marker() {
    let Some(path) = marker_path() else {
        return;
    };
    let Some(parent) = path.parent() else {
        return;
    };
    if fs::create_dir_all(parent).is_ok() {
        let _ = fs::write(path, b"");
    }
}
