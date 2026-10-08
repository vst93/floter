//! R150 · the application icon's light/dark variants, chosen in Settings.
//!
//! Floter is a menu-bar/tray resident: on macOS it runs as an `Accessory` app
//! with no Dock icon, so the icon the user actually sees is the tray icon, and
//! on Windows/Linux the same image is the window's taskbar icon. The icon shown
//! by the *bundle* (Finder, the installer) is fixed at build time and cannot
//! change at runtime, so the two runtime variants live here as PNGs embedded in
//! the binary and [`crate::apply_app_icon`] installs the one the `app_icon`
//! setting names.
//!
//! Both PNGs are rendered from `icons/icon-dark.svg` / `icons/icon-light.svg`,
//! the same source as the packaged `icon.png` (the dark mark). The default is
//! `dark`: every build through R149 shipped the black mark, so a settings file
//! with no `app_icon` key has to come back black rather than transparent or
//! whichever variant happens to sort first.

use tauri::image::Image;

/// The two appearances, in the order the Settings picker paints them.
pub const APP_ICON_APPEARANCES: [&str; 2] = ["dark", "light"];

/// The shipped appearance: the black mark every build through R149 used.
pub const DEFAULT_APP_ICON: &str = "dark";

/// Map any stored value onto a shipped appearance.
///
/// An unknown or hand-edited value lands on [`DEFAULT_APP_ICON`] rather than on
/// whichever variant happens to be first in the list — a user who never chose
/// an icon must not get a different one after a hand edit, exactly as
/// `commands::config::normalize_ui_scale` treats an unknown step.
pub fn normalize_app_icon(value: &str) -> &'static str {
    APP_ICON_APPEARANCES
        .iter()
        .copied()
        .find(|appearance| *appearance == value)
        .unwrap_or(DEFAULT_APP_ICON)
}

/// The embedded PNG for an appearance. The value is normalized first, so an
/// unknown string can only ever return the default's bytes.
pub fn icon_bytes(appearance: &str) -> &'static [u8] {
    match normalize_app_icon(appearance) {
        "light" => include_bytes!("../icons/icon-light.png"),
        _ => include_bytes!("../icons/icon-dark.png"),
    }
}

/// Decode the embedded PNG for an appearance into a Tauri image. Owned data,
/// so the returned image borrows nothing and can be handed to a tray or window
/// on any thread.
pub fn image_for(appearance: &str) -> tauri::Result<Image<'static>> {
    Image::from_bytes(icon_bytes(appearance))
}

#[cfg(test)]
mod tests {
    use super::{
        icon_bytes, image_for, normalize_app_icon, APP_ICON_APPEARANCES, DEFAULT_APP_ICON,
    };

    /// The migration lock: an absent/unknown key is the black mark, not a
    /// blank icon and not the light variant.
    #[test]
    fn the_default_is_the_dark_mark() {
        assert_eq!(DEFAULT_APP_ICON, "dark");
        assert_eq!(normalize_app_icon(""), DEFAULT_APP_ICON);
        assert_eq!(normalize_app_icon("dark"), "dark");
        assert_eq!(normalize_app_icon("light"), "light");
    }

    /// A value the build does not know falls back to the default instead of
    /// leaking through to a match arm that would pick the wrong bytes.
    #[test]
    fn unknown_values_fall_back_to_the_default() {
        for unknown in ["", "Dark", "LIGHT", "auto", "system", "midnight"] {
            assert_eq!(normalize_app_icon(unknown), DEFAULT_APP_ICON, "{unknown}");
            assert_eq!(
                icon_bytes(unknown),
                icon_bytes(DEFAULT_APP_ICON),
                "{unknown}"
            );
        }
    }

    /// Both variants really are embedded PNGs, and they are not the same file:
    /// a copy-paste that shipped one image twice would otherwise pass every
    /// value-level assertion above.
    #[test]
    fn each_appearance_carries_its_own_embedded_png() {
        const PNG_SIGNATURE: [u8; 8] = [0x89, b'P', b'N', b'G', 0x0d, 0x0a, 0x1a, 0x0a];
        for appearance in APP_ICON_APPEARANCES {
            let bytes = icon_bytes(appearance);
            assert!(
                bytes.starts_with(&PNG_SIGNATURE),
                "{appearance} must be a PNG"
            );
        }
        assert_ne!(
            icon_bytes("dark"),
            icon_bytes("light"),
            "the two appearances must not share one image"
        );
    }

    /// The decoder the tray and window paths call has to accept both embedded
    /// files; a corrupt or truncated asset fails here rather than at runtime.
    #[test]
    fn both_appearances_decode_into_a_square_image() {
        for appearance in APP_ICON_APPEARANCES {
            let image = image_for(appearance).expect("the embedded icon must decode");
            assert_eq!(image.width(), 512, "{appearance} width");
            assert_eq!(image.height(), 512, "{appearance} height");
        }
    }
}
