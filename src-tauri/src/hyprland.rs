//! R41 · Hyprland (Wayland) window shaping.
//!
//! The user's report, verbatim: 「还有现在在 hyprland 桌面下启动时，默认并不是居中
//! 的悬浮窗，而是一个宽高拉满的窗口。这个有办法做特殊处理吗？让它始终保持悬浮. 而且
//! 如果检查到是这个系统 应该默认关闭失焦隐藏这个逻辑 其他情况下还是默认开启」.
//!
//! Hyprland is a tiling compositor: a new toplevel is tiled (and, when the
//! window asks for a large size, filled) unless a window rule or a runtime
//! dispatcher floats it. Tauri's own geometry calls cannot change that — the
//! compositor owns tiling — so this module adds the one app-side lever that can:
//! `hyprctl dispatch setfloating`, which converts the *active* window to a
//! floating one. The launcher then keeps its normal size and position (the
//! existing `resize_window` / `move_to_default_position` run right after), which
//! is exactly the centered floating panel the user asked for.
//!
//! Everything here is a **best-effort** side effect:
//!
//!   * the detection is a pure function of `HYPRLAND_INSTANCE_SIGNATURE`, so the
//!     node/Rust suites can pin it without a compositor;
//!   * the dispatcher is a bounded control round-trip (`hyprctl` connects to the
//!     running compositor and returns), deliberately synchronous so the float
//!     lands *before* the geometry calls that follow — not a wait on a
//!     user-launched program;
//!   * every failure is ignored. A machine without `hyprctl` on PATH, a
//!     compositor that renamed the socket, or a focus race all degrade to the
//!     pre-R41 behaviour rather than breaking the reveal.
//!
//! Only Hyprland is special-cased. Other Wayland compositors (sway, niri, …)
//! have different tiling models and are deliberately left alone — guessing a
//! dispatcher for them would be a behaviour change with no evidence behind it.

// The floating half of this module only has a caller on Linux (`reveal_window`);
// the detection/default half is used on every platform by `AppSettings`.
#![cfg_attr(not(target_os = "linux"), allow(dead_code))]

use std::process::{Command, Stdio};

/// The environment variable every Hyprland session exports (its value is the
/// instance signature, used to find the compositor's socket).
pub const HYPRLAND_SIGNATURE_ENV: &str = "HYPRLAND_INSTANCE_SIGNATURE";

/// The `hyprctl` dispatches that float and centre the active window, in order.
/// `setfloating` is a *set* (not a toggle), so a reveal that lands on an already
/// floating window is a no-op rather than a flip back to tiling.
pub const FLOATING_DISPATCHES: [&[&str]; 2] = [
    &["dispatch", "setfloating"],
    &["dispatch", "centerwindow"],
];

/// Whether a signature value means "this is a Hyprland session". A missing or
/// blank value is not a Hyprland session: the variable exists only when
/// Hyprland launched the process, and an empty one would be a broken export.
pub fn hyprland_signature_detected(signature: Option<&str>) -> bool {
    signature.is_some_and(|value| !value.trim().is_empty())
}

/// Whether the current process runs under Hyprland, read from the live
/// environment.
pub fn hyprland_detected() -> bool {
    let signature = std::env::var(HYPRLAND_SIGNATURE_ENV).ok();
    hyprland_signature_detected(signature.as_deref())
}

/// The `hide_on_blur` default for a signature: Hyprland wants the panel to stay
/// put (a tiled desktop hides and re-shows windows differently, and the user's
/// request was explicit), every other platform keeps the shipped behaviour.
///
/// This only decides the value a *fresh* settings file is born with; an
/// existing `hide_on_blur` key always wins, and the settings page can change it
/// at any time. It is a function rather than a constant because the answer
/// depends on the environment, and it takes the signature as an argument rather
/// than reading the process environment so it stays testable.
pub fn default_hide_on_blur_for(signature: Option<&str>) -> bool {
    !hyprland_signature_detected(signature)
}

/// [`default_hide_on_blur_for`] against the live environment. This is what
/// `AppSettings::default()` calls.
pub fn default_hide_on_blur() -> bool {
    let signature = std::env::var(HYPRLAND_SIGNATURE_ENV).ok();
    default_hide_on_blur_for(signature.as_deref())
}

/// Float and centre the window that is currently focused, if this is a Hyprland
/// session. Called from `reveal_window` right after `show()` + `set_focus()`, so
/// the launcher is the active window when the dispatcher runs and the geometry
/// calls that follow act on a floating window.
///
/// Best-effort: a failed dispatch changes nothing.
pub fn ensure_floating() {
    if !hyprland_detected() {
        return;
    }
    for args in FLOATING_DISPATCHES {
        let _ = Command::new("hyprctl")
            .args(args)
            .stdin(Stdio::null())
            .stdout(Stdio::null())
            .stderr(Stdio::null())
            .status();
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn only_a_non_empty_signature_is_hyprland() {
        assert!(hyprland_signature_detected(Some("abc123")));
        // The signature is a random token; whitespace around it is not one.
        assert!(hyprland_signature_detected(Some(" abc123 ")));
        assert!(!hyprland_signature_detected(Some("")));
        assert!(!hyprland_signature_detected(Some("   ")));
        assert!(!hyprland_signature_detected(None));
    }

    #[test]
    fn hide_on_blur_defaults_off_only_on_hyprland() {
        assert!(!default_hide_on_blur_for(Some("sig")));
        assert!(default_hide_on_blur_for(None));
        assert!(default_hide_on_blur_for(Some("")));
    }

    #[test]
    fn the_float_dispatch_is_a_set_not_a_toggle() {
        // `togglefloating` would flip an already-floating window back into the
        // tiling layout on a second reveal; `setfloating` is idempotent.
        assert_eq!(FLOATING_DISPATCHES[0], &["dispatch", "setfloating"]);
        assert!(!FLOATING_DISPATCHES[0].contains(&"togglefloating"));
        assert_eq!(FLOATING_DISPATCHES[1], &["dispatch", "centerwindow"]);
    }
}
