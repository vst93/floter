//! R45 · the tray icon's identity on Linux, derived in one place.
//!
//! `tray-icon` 0.24.2's GTK backend hands libayatana-appindicator a single
//! string — `format!("tray-icon tray app {}", id)` — and the library derives
//! *every* name a StatusNotifier host can route by from that one value:
//!
//! * the SNI `Id` property,
//! * the SNI object path `/org/ayatana/NotificationItem/<clean id>`,
//! * the dbusmenu object path `<object path>/Menu`,
//! * and, one layer up in `tray-icon` itself, the on-disk icon
//!   `$XDG_RUNTIME_DIR/tray-icon/tray-icon-<id>-0.png`.
//!
//! Two Tauri apps that pass the same tray id therefore publish the *same*
//! object path, the *same* menu path and the *same* icon file. This app used to
//! pass the fixed literal `main-tray` — an app-independent string another
//! project can reach by copying the same tray snippet — so a second Tauri
//! process that picked the same literal published the same identity: a host
//! that keys its registry by that string (or that loads the shared icon file)
//! lets the later registration take over the earlier one, and its menu shows up
//! under this app's icon. An app that registers a StatusNotifierItem directly,
//! under the spec's `org.kde.StatusNotifierItem-<pid>-<n>` name at
//! `/StatusNotifierItem`, shares no field with an appindicator id at all, which
//! is why only a second appindicator (Tauri) app collided.
//!
//! [`TRAY_ICON_ID`] is the bundle identifier plus a `.tray` suffix. The
//! identifier is already globally unique, and no other app reaches this value
//! by keeping a template default, so the collision is gone no matter what the
//! other process registers.

/// The id handed to `TrayIconBuilder` and looked up again by `tray_by_id`.
///
/// Must stay `"<tauri.conf.json identifier>.tray"`; `the_tray_id_is_the_bundle_identifier_plus_a_suffix`
/// reads the config file back and fails if the two ever drift apart.
pub const TRAY_ICON_ID: &str = "com.v.floter.tray";

/// The string `tray-icon` passes to `AppIndicator::new`.
///
/// Mirrors `tray-icon/src/platform_impl/gtk/mod.rs`, which is the only place
/// the crate composes an appindicator id.
#[cfg(any(target_os = "linux", test))]
pub fn appindicator_id() -> String {
    format!("tray-icon tray app {TRAY_ICON_ID}")
}

/// libayatana-appindicator's `clean_id`: every non-alphanumeric byte of the
/// appindicator id becomes `_`, and both the item object path and the dbusmenu
/// path are built from it.
#[cfg(any(target_os = "linux", test))]
pub fn appindicator_clean_id() -> String {
    appindicator_id()
        .chars()
        .map(|c| if c.is_ascii_alphanumeric() { c } else { '_' })
        .collect()
}

/// The StatusNotifierItem object path this app registers.
#[cfg(any(target_os = "linux", test))]
pub fn sni_object_path() -> String {
    format!(
        "/org/ayatana/NotificationItem/{}",
        appindicator_clean_id()
    )
}

/// The dbusmenu object path the SNI `Menu` property points at.
#[cfg(any(target_os = "linux", test))]
pub fn sni_menu_path() -> String {
    format!("{}/Menu", sni_object_path())
}

/// Where `tray-icon`'s GTK backend writes this icon: its `temp_icon_path` with
/// the default (unset) temp dir and counter 0.
#[cfg(any(target_os = "linux", test))]
pub fn icon_file_path() -> std::path::PathBuf {
    dirs::runtime_dir()
        .unwrap_or_else(std::env::temp_dir)
        .join("tray-icon")
        .join(format!("tray-icon-{TRAY_ICON_ID}-0.png"))
}

/// Read back the one artifact the GTK backend writes synchronously while it
/// builds the SNI identity: the icon file.
///
/// `tray-icon` names that file from the same id libayatana-appindicator builds
/// the object path from, so a file at [`icon_file_path`] is proof that this
/// process registered under [`TRAY_ICON_ID`] and not under a shared default.
/// Called once, right after the tray is built; a missing file is a warning,
/// never a startup failure.
#[cfg(target_os = "linux")]
pub fn verify_registration() {
    let path = icon_file_path();
    if path.is_file() {
        tracing::debug!(
            "tray identity: id='{}' sni='{}' menu='{}' icon='{}'",
            appindicator_id(),
            sni_object_path(),
            sni_menu_path(),
            path.display()
        );
    } else {
        tracing::warn!(
            "tray identity: expected an icon at '{}' for id '{}' — the tray may not have registered",
            path.display(),
            appindicator_id()
        );
    }
}

#[cfg(test)]
mod tests {
    use super::{
        appindicator_clean_id, appindicator_id, icon_file_path, sni_menu_path, sni_object_path,
        TRAY_ICON_ID,
    };

    /// The tray id is the one value every other identity is derived from, so it
    /// has to be the app's own identifier and not a literal another project can
    /// type. Read the identifier back out of the config instead of trusting a
    /// second copy of the string.
    #[test]
    fn the_tray_id_is_the_bundle_identifier_plus_a_suffix() {
        let config: serde_json::Value =
            serde_json::from_str(include_str!("../tauri.conf.json")).expect("tauri.conf.json");
        let identifier = config["identifier"].as_str().expect("identifier");
        assert_eq!(TRAY_ICON_ID, format!("{identifier}.tray"));
    }

    /// The regression this round exists for: the id used to be the shared
    /// example literal, and every name below was therefore identical in two
    /// unrelated Tauri apps.
    #[test]
    fn no_identity_field_is_the_shared_example_literal() {
        for shared in ["main-tray", "main", "tray"] {
            assert_ne!(TRAY_ICON_ID, shared);
            assert_ne!(appindicator_id(), format!("tray-icon tray app {shared}"));
            assert_ne!(
                appindicator_clean_id(),
                format!("tray_icon_tray_app_{}", shared.replace('-', "_"))
            );
        }
        // Spelled out for the one literal that actually shipped.
        assert_ne!(appindicator_clean_id(), "tray_icon_tray_app_main_tray");
    }

    /// The exact strings a StatusNotifier host sees, spelled out so a change in
    /// either derivation is a diff here rather than a silent re-collision.
    #[test]
    fn the_registered_names_are_the_ones_floter_owns() {
        assert_eq!(appindicator_id(), "tray-icon tray app com.v.floter.tray");
        assert_eq!(appindicator_clean_id(), "tray_icon_tray_app_com_v_floter_tray");
        assert_eq!(
            sni_object_path(),
            "/org/ayatana/NotificationItem/tray_icon_tray_app_com_v_floter_tray"
        );
        assert_eq!(
            sni_menu_path(),
            "/org/ayatana/NotificationItem/tray_icon_tray_app_com_v_floter_tray/Menu"
        );
        assert!(icon_file_path()
            .to_string_lossy()
            .ends_with("tray-icon/tray-icon-com.v.floter.tray-0.png"));
    }

    /// `clean_id` only ever emits `[A-Za-z0-9_]`, which is what makes the
    /// object path a legal D-Bus path for any id we choose.
    #[test]
    fn the_clean_id_is_a_legal_object_path_segment() {
        let clean = appindicator_clean_id();
        assert!(!clean.is_empty());
        assert!(clean.chars().all(|c| c.is_ascii_alphanumeric() || c == '_'));
    }
}
