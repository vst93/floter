//! R85 · the detached plugin window's geometry, remembered across sessions.
//!
//! R84 shipped the second window at a hard 720×480 in the system's default
//! spot, so every pin after a drag or a move to the second monitor started
//! over. This module is the memory: the window's `{width, height, x, y}` in
//! logical coordinates lives under the app data dir as
//! `plugin-window-geometry.json`, written atomically (tempfile + rename, the
//! same discipline as `calculator_history` and `clipboard_history`), and read
//! back before the builder runs. A corrupt or missing file recovers as "no
//! geometry" — the window opens at the defaults rather than taking the
//! launcher down with it.
//!
//! ## Why Rust owns the listener
//!
//! `detach_plugin_window` attaches the recorder to the window itself, not to
//! the page. The page can be mid-navigation, or already gone, while the user
//! drags the native frame; the OS keeps delivering `Moved`/`Resized` either
//! way. A frontend `listen` would miss exactly the gestures this feature is
//! for.
//!
//! ## Debounce
//!
//! A drag fires a `Moved` per frame. Rather than write on every one, a single
//! worker thread coalesces the burst: it blocks on the latest geometry and
//! keeps absorbing newer ones until the channel goes quiet for [`DEBOUNCE`],
//! then writes once. A burst of N events costs one disk write, and the value
//! written is always the newest.
//!
//! ## Legality is "can be shown", not "guess"
//!
//! [`resolve_placement`] is a pure function so the rules are testable without
//! a live window:
//!
//! - size is clamped up to the builder's minimum (360×240) and down to the
//!   primary display's bounds when those are known; with no monitor to measure
//!   against it only clamps the floor and never invents a ceiling;
//! - a position that lands inside any known display is restored verbatim; one
//!   that lands outside every display (the unplugged-second-monitor case) is
//!   dropped and the system picks the spot, while the size still restores.

use serde::{Deserialize, Serialize};
use std::io::Write;
use std::path::{Path, PathBuf};
use std::sync::mpsc::{self, Receiver, RecvTimeoutError, Sender};
use std::sync::OnceLock;
use std::time::Duration;

/// The minimum inner size, matching the builder's `min_inner_size`.
pub const MIN_WIDTH: f64 = 360.0;
pub const MIN_HEIGHT: f64 = 240.0;
/// The size the window opens at when nothing usable was remembered.
pub const DEFAULT_WIDTH: f64 = 720.0;
pub const DEFAULT_HEIGHT: f64 = 480.0;

/// How long a burst of move/resize events must go quiet before it is written.
const DEBOUNCE: Duration = Duration::from_millis(400);

/// The file name inside the app data dir; the reader and the writer both go
/// through it so a rename cannot split the pair.
const GEOMETRY_FILE: &str = "plugin-window-geometry.json";

/// One remembered placement, in logical coordinates.
#[derive(Debug, Clone, Copy, PartialEq, Serialize, Deserialize)]
pub struct WindowGeometry {
    pub width: f64,
    pub height: f64,
    pub x: f64,
    pub y: f64,
}

/// A monitor's bounds in logical coordinates — the space [`WindowGeometry`]
/// lives in. Tauri hands monitors back in physical pixels, so the conversion
/// divides by that monitor's own scale factor (the same per-monitor scale the
/// cursor/panel matching in `lib.rs` has to account for).
#[derive(Debug, Clone, Copy, PartialEq)]
pub struct DisplayArea {
    pub x: f64,
    pub y: f64,
    pub width: f64,
    pub height: f64,
}

impl DisplayArea {
    /// Half-open, so a point on the seam between two screens belongs to exactly
    /// one of them — the rule `monitor_containing` already uses.
    fn contains(&self, x: f64, y: f64) -> bool {
        x >= self.x && x < self.x + self.width && y >= self.y && y < self.y + self.height
    }
}

/// What the builder should actually apply: a size always, a position only when
/// it is known to be on a display.
#[derive(Debug, Clone, Copy, PartialEq)]
pub struct Placement {
    pub width: f64,
    pub height: f64,
    pub position: Option<(f64, f64)>,
}

impl Default for Placement {
    fn default() -> Self {
        Placement {
            width: DEFAULT_WIDTH,
            height: DEFAULT_HEIGHT,
            position: None,
        }
    }
}

/// Turn a stored geometry into something safe to apply. Pure, so the clamping
/// and off-screen rules are pinned by unit tests rather than by dragging a
/// window across a real desk.
///
/// `primary` bounds the size when it is known; `monitors` is the set of
/// displays a position is allowed to land on. Both are optional inputs because
/// a platform may refuse to name any monitor — in that case the size only
/// clamps its floor and the position is dropped rather than guessed.
pub fn resolve_placement(
    stored: Option<WindowGeometry>,
    primary: Option<DisplayArea>,
    monitors: &[DisplayArea],
) -> Placement {
    let Some(stored) = stored else {
        return Placement::default();
    };

    let width = clamp_size(
        stored.width,
        DEFAULT_WIDTH,
        MIN_WIDTH,
        primary.map(|area| area.width),
    );
    let height = clamp_size(
        stored.height,
        DEFAULT_HEIGHT,
        MIN_HEIGHT,
        primary.map(|area| area.height),
    );

    let position = if stored.x.is_finite()
        && stored.y.is_finite()
        && monitors
            .iter()
            .any(|area| area.contains(stored.x, stored.y))
    {
        Some((stored.x, stored.y))
    } else {
        None
    };

    Placement {
        width,
        height,
        position,
    }
}

/// Clamp one dimension. A non-finite or non-positive reading is unusable and
/// falls back to the default; a missing ceiling means "do not invent one", not
/// "clamp to zero".
fn clamp_size(value: f64, fallback: f64, minimum: f64, maximum: Option<f64>) -> f64 {
    let value = if value.is_finite() && value > 0.0 {
        value
    } else {
        fallback
    };
    match maximum {
        Some(max) if max.is_finite() && max >= minimum => value.clamp(minimum, max),
        _ => value.max(minimum),
    }
}

/// The app data directory the geometry shares with `calculator_history`.
fn store_dir() -> Option<PathBuf> {
    dirs::config_dir().map(|dir| dir.join("floter"))
}

fn geometry_file() -> Option<PathBuf> {
    store_dir().map(|dir| dir.join(GEOMETRY_FILE))
}

/// Read the remembered geometry. A missing, unreadable, truncated or
/// type-mismatched file is "nothing remembered", never an error the caller has
/// to handle.
pub fn load_geometry() -> Option<WindowGeometry> {
    geometry_file().and_then(|path| load_from(&path))
}

fn load_from(path: &Path) -> Option<WindowGeometry> {
    std::fs::read(path)
        .ok()
        .and_then(|bytes| serde_json::from_slice::<WindowGeometry>(&bytes).ok())
}

/// Replace the geometry atomically. A crash mid-write leaves the previous
/// file intact.
fn save_geometry(geometry: &WindowGeometry) -> Result<(), String> {
    let Some(path) = geometry_file() else {
        return Err("No app data directory".to_string());
    };
    save_to(&path, geometry)
}

fn save_to(path: &Path, geometry: &WindowGeometry) -> Result<(), String> {
    let Some(dir) = path.parent() else {
        return Err("No parent directory".to_string());
    };
    std::fs::create_dir_all(dir).map_err(|error| error.to_string())?;
    let content = serde_json::to_vec_pretty(geometry).map_err(|error| error.to_string())?;

    let mut temporary = tempfile::NamedTempFile::new_in(dir).map_err(|error| error.to_string())?;
    temporary
        .write_all(&content)
        .and_then(|_| temporary.flush())
        .and_then(|_| temporary.as_file().sync_all())
        .map_err(|error| error.to_string())?;
    temporary.persist(path).map_err(|error| error.to_string())?;
    Ok(())
}

/// The coalescing writer, started once and shared by every window that ever
/// needs to record a geometry.
static GEOMETRY_SENDER: OnceLock<Sender<WindowGeometry>> = OnceLock::new();

fn sender() -> &'static Sender<WindowGeometry> {
    GEOMETRY_SENDER.get_or_init(|| {
        let (tx, rx) = mpsc::channel();
        std::thread::spawn(move || drain(rx));
        tx
    })
}

/// Record a geometry for persistence. Non-blocking: the caller (a window event
/// handler) hands the value over and returns; the worker decides when the burst
/// has settled.
pub fn note_geometry(geometry: WindowGeometry) {
    let _ = sender().send(geometry);
}

/// Absorb every geometry that arrives within one debounce window and write the
/// last one. A send that lands while a write is in flight simply starts the
/// next window, so the newest value is never lost to a race.
fn drain(rx: Receiver<WindowGeometry>) {
    while let Ok(mut latest) = rx.recv() {
        loop {
            match rx.recv_timeout(DEBOUNCE) {
                Ok(newer) => latest = newer,
                Err(RecvTimeoutError::Timeout) => break,
                Err(RecvTimeoutError::Disconnected) => {
                    let _ = save_geometry(&latest);
                    return;
                }
            }
        }
        if let Err(error) = save_geometry(&latest) {
            tracing::warn!("failed to persist plugin window geometry: {error}");
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn stored(width: f64, height: f64, x: f64, y: f64) -> WindowGeometry {
        WindowGeometry {
            width,
            height,
            x,
            y,
        }
    }

    fn display(x: f64, y: f64, width: f64, height: f64) -> DisplayArea {
        DisplayArea {
            x,
            y,
            width,
            height,
        }
    }

    /// A fresh window with nothing remembered opens at the R84 defaults and
    /// lets the system place it.
    #[test]
    fn nothing_remembered_opens_at_the_defaults() {
        assert_eq!(
            resolve_placement(None, Some(display(0.0, 0.0, 1920.0, 1080.0)), &[]),
            Placement::default()
        );
    }

    /// A size below the builder's minimum is lifted to the minimum; the stored
    /// position, which is on screen, rides along.
    #[test]
    fn a_size_below_the_minimum_is_clamped_up() {
        let monitors = [display(0.0, 0.0, 1920.0, 1080.0)];
        let placement = resolve_placement(
            Some(stored(100.0, 80.0, 40.0, 60.0)),
            Some(monitors[0]),
            &monitors,
        );
        assert_eq!(placement.width, MIN_WIDTH);
        assert_eq!(placement.height, MIN_HEIGHT);
        assert_eq!(placement.position, Some((40.0, 60.0)));
    }

    /// A size larger than the primary display is clamped to it — a window that
    /// cannot fit is not a window the user can use.
    #[test]
    fn a_size_above_the_primary_display_is_clamped_down() {
        let primary = display(0.0, 0.0, 1920.0, 1080.0);
        let placement = resolve_placement(
            Some(stored(4000.0, 3000.0, 10.0, 10.0)),
            Some(primary),
            &[primary],
        );
        assert_eq!(placement.width, 1920.0);
        assert_eq!(placement.height, 1080.0);
    }

    /// Without a monitor to measure against, only the floor is clamped: the
    /// stored size is kept as-is rather than guessed down to some invented
    /// ceiling.
    #[test]
    fn without_monitor_information_only_the_floor_is_clamped() {
        let placement = resolve_placement(Some(stored(3000.0, 2000.0, 5.0, 5.0)), None, &[]);
        assert_eq!(placement.width, 3000.0);
        assert_eq!(placement.height, 2000.0);
        // No display is known, so the position cannot be vouched for.
        assert_eq!(placement.position, None);

        let tiny = resolve_placement(Some(stored(10.0, 10.0, 0.0, 0.0)), None, &[]);
        assert_eq!(tiny.width, MIN_WIDTH);
        assert_eq!(tiny.height, MIN_HEIGHT);
    }

    /// A position on the second monitor is restored verbatim, bounds and all.
    #[test]
    fn a_position_on_a_known_display_is_restored_verbatim() {
        let monitors = [
            display(0.0, 0.0, 1920.0, 1080.0),
            display(1920.0, 0.0, 2560.0, 1440.0),
        ];
        let placement = resolve_placement(
            Some(stored(800.0, 600.0, 2000.0, 300.0)),
            Some(monitors[0]),
            &monitors,
        );
        assert_eq!(placement.position, Some((2000.0, 300.0)));
        assert_eq!(placement.width, 800.0);
        assert_eq!(placement.height, 600.0);
    }

    /// The unplugged-second-monitor case: the saved position is on no display
    /// any more, so it is dropped and the system picks the spot — but the size
    /// the user chose still comes back.
    #[test]
    fn a_position_on_a_removed_display_is_dropped_but_the_size_survives() {
        let monitors = [display(0.0, 0.0, 1920.0, 1080.0)];
        let placement = resolve_placement(
            Some(stored(900.0, 700.0, 2400.0, 200.0)),
            Some(monitors[0]),
            &monitors,
        );
        assert_eq!(placement.position, None);
        assert_eq!(placement.width, 900.0);
        assert_eq!(placement.height, 700.0);
    }

    /// Non-finite numbers cannot come from the JSON file, but a caller (or a
    /// future format) could hand them over; they fall back rather than
    /// propagating NaN into a window size.
    #[test]
    fn a_non_finite_geometry_falls_back_to_defaults() {
        let monitors = [display(0.0, 0.0, 1920.0, 1080.0)];
        let placement = resolve_placement(
            Some(stored(f64::NAN, f64::INFINITY, f64::NAN, 10.0)),
            Some(monitors[0]),
            &monitors,
        );
        assert_eq!(placement.width, DEFAULT_WIDTH);
        assert_eq!(placement.height, DEFAULT_HEIGHT);
        assert_eq!(placement.position, None);
    }

    /// A corrupt file (truncated, wrong types, missing fields) reads back as
    /// "nothing remembered" instead of panicking or erroring the detach path.
    #[test]
    fn a_corrupt_file_recovers_as_no_geometry() {
        let dir = tempfile::tempdir().unwrap();
        let path = dir.path().join(GEOMETRY_FILE);
        for content in [
            "",
            "{\"width\": 800, \"height\": 600, \"x\": 10,",
            "{\"width\": \"wide\", \"height\": 600, \"x\": 10, \"y\": 10}",
            "{\"width\": 800, \"height\": 600}",
            "not json at all",
        ] {
            std::fs::write(&path, content).unwrap();
            assert_eq!(load_from(&path), None, "content: {content:?}");
        }
    }

    /// The write path round-trips through the same file the reader uses, and a
    /// missing file is simply "nothing remembered".
    #[test]
    fn a_saved_geometry_round_trips() {
        let dir = tempfile::tempdir().unwrap();
        let path = dir.path().join(GEOMETRY_FILE);
        assert_eq!(load_from(&path), None);

        let geometry = stored(1024.0, 768.0, 128.0, 96.0);
        save_to(&path, &geometry).unwrap();
        assert_eq!(load_from(&path), Some(geometry));
    }

    /// A half-open seam belongs to exactly one display, so a window parked on
    /// the boundary is still vouched for by the right-hand screen.
    #[test]
    fn a_position_on_a_display_seam_belongs_to_the_right_screen() {
        let monitors = [
            display(0.0, 0.0, 1920.0, 1080.0),
            display(1920.0, 0.0, 1280.0, 1024.0),
        ];
        let placement = resolve_placement(
            Some(stored(600.0, 400.0, 1920.0, 0.0)),
            Some(monitors[0]),
            &monitors,
        );
        assert_eq!(placement.position, Some((1920.0, 0.0)));
    }
}
