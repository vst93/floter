//! R91 · more than one detached plugin window.
//!
//! R84-R90 shipped a single second window (`plugin-detached`): a detach while
//! it was open replaced its content, because there was only one window to
//! replace. The user's ruling (direction 3, second half) retires that
//! single-instance assumption: pinning a *different* thing opens another
//! window, and the taskbar grows one entry per instance — the user manages
//! their own desktop, so there is no instance ceiling. Pinning the *same* thing
//! still reuses the window that already shows it (「再钉同一个 → 原窗换内容」),
//! so the R84-R90 replacement behaviour survives for the case it was written
//! for.
//!
//! Everything in this module is pure — a label allocator and the content
//! identity a window is keyed by — so a plain Rust unit test can pin the
//! decisions a review could silently delete. The stateful half (the per-label
//! slots, the label → key map, the geometry watcher) lives in `lib.rs`.

use crate::DetachRequest;

/// The label of the *first* detached window. R84-R90 spelled this literal in
/// the builder, the capability file (`capabilities/plugin-detached.json`) and
/// the frontend's render branch (`src/main.tsx`, via its own
/// `PLUGIN_WINDOW_LABEL`); it is still the label the first instance gets, so
/// those anchors are untouched. The second instance is `plugin-detached-2`,
/// the third `plugin-detached-3`, and so on.
pub const PLUGIN_WINDOW_LABEL: &str = "plugin-detached";

/// The lowest free instance index, as a label.
///
/// Pure on purpose: the caller passes the labels of the windows that are open
/// right now, and the answer is the base label for index 1 (the R84-R90
/// literal) or `plugin-detached-N` for the lowest `N ≥ 2` that is not taken.
///
/// There is no registry to keep in sync — a window is destroyed without any
/// bookkeeping and the next detach re-scans — so a closed window's index is
/// free again the moment it is gone.
pub fn next_plugin_window_label(existing_labels: &[String]) -> String {
    let prefix = format!("{PLUGIN_WINDOW_LABEL}-");
    let mut taken = std::collections::HashSet::new();
    for label in existing_labels {
        if label == PLUGIN_WINDOW_LABEL {
            taken.insert(1u32);
        } else if let Some(rest) = label.strip_prefix(&prefix) {
            if let Ok(index) = rest.parse::<u32>() {
                if index >= 2 {
                    taken.insert(index);
                }
            }
        }
    }
    let mut index = 1u32;
    while taken.contains(&index) {
        index += 1;
    }
    if index == 1 {
        PLUGIN_WINDOW_LABEL.to_string()
    } else {
        format!("{PLUGIN_WINDOW_LABEL}-{index}")
    }
}

/// The identity a detached window is keyed by: the same key names its
/// remembered geometry (the v2 store in `plugin_window_geometry`) and decides
/// whether a detach replaces an open window or opens a new one.
///
/// The external arm is identified by the command it runs (`extensionId` +
/// `commandId`) — not its label, not its argv. Pinning the same command twice
/// is the same window; pinning it with different arguments is *still* the same
/// window, whose newest run replaces the old one (the R84-R90 behaviour). The
/// two halves are joined by a NUL, a byte no id can contain, so `("a", "bc")`
/// and `("ab", "c")` cannot collide.
///
/// The text arm has no command, so its title is its identity. The title is
/// trimmed so a stray space does not split one pinned note into two windows.
pub fn content_key(request: &DetachRequest) -> String {
    match request {
        DetachRequest::External {
            extension_id,
            command_id,
            ..
        } => format!("{extension_id}\u{0}{command_id}"),
        DetachRequest::Text { title, .. } => title.trim().to_string(),
    }
}

/// One label's parked request. R91 · the slot is per label now (it was a single
/// `Option` through R84-R90), and it remembers whether it has already been
/// handed over: the detached page pulls its request on mount, and a remount
/// must never re-run what the user already consumed.
#[derive(Debug, Clone, PartialEq)]
pub struct PendingSlot {
    pub request: DetachRequest,
    pub delivered: bool,
}

impl PendingSlot {
    /// Hand the request over exactly once. The first call flips `delivered` and
    /// returns the request; every later call returns `None`, so a remount never
    /// re-runs a command the user already consumed. This is the one rule the
    /// pull-slot contract stands on, kept here where a unit test reaches it.
    pub fn take(&mut self) -> Option<DetachRequest> {
        if self.delivered {
            return None;
        }
        self.delivered = true;
        Some(self.request.clone())
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn labels(values: &[&str]) -> Vec<String> {
        values.iter().map(|value| (*value).to_string()).collect()
    }

    /// The first instance keeps R84-R90's exact literal, whatever unrelated
    /// windows exist around it.
    #[test]
    fn the_first_instance_is_the_base_label() {
        assert_eq!(next_plugin_window_label(&[]), PLUGIN_WINDOW_LABEL);
        assert_eq!(
            next_plugin_window_label(&labels(&["main", "something-else"])),
            PLUGIN_WINDOW_LABEL
        );
    }

    /// The allocator returns the lowest free index, not "one past the highest":
    /// with the base and `-2` taken, the next window is `-3`; with only `-3`
    /// taken, the base is still free and comes first.
    #[test]
    fn the_allocator_fills_the_lowest_free_index() {
        assert_eq!(
            next_plugin_window_label(&labels(&["plugin-detached", "plugin-detached-2"])),
            "plugin-detached-3"
        );
        assert_eq!(
            next_plugin_window_label(&labels(&["plugin-detached-3"])),
            PLUGIN_WINDOW_LABEL
        );
        // A closed window frees its index again — the scan is the registry.
        assert_eq!(
            next_plugin_window_label(&labels(&["plugin-detached", "plugin-detached-3"])),
            "plugin-detached-2"
        );
    }

    /// Labels that merely start with the base but are not an instance index
    /// (`plugin-detached-window`, a bare trailing dash, `-1`) neither consume
    /// an index nor confuse the scan.
    #[test]
    fn non_instance_labels_do_not_take_an_index() {
        assert_eq!(
            next_plugin_window_label(&labels(&[
                "plugin-detached-window",
                "plugin-detached-",
                "plugin-detached-1",
                "plugin-detached-2x",
            ])),
            PLUGIN_WINDOW_LABEL
        );
    }

    /// The external identity is the command, not the label or the argv: the
    /// same command with different arguments keys the same window.
    #[test]
    fn the_external_key_is_the_command_identity() {
        let first = DetachRequest::External {
            extension_id: "local.tool".into(),
            command_id: "run".into(),
            command_label: "My Tool".into(),
            args: vec!["--flag".into()],
        };
        let renamed = DetachRequest::External {
            extension_id: "local.tool".into(),
            command_id: "run".into(),
            command_label: "Renamed".into(),
            args: vec!["--other".into()],
        };
        let other = DetachRequest::External {
            extension_id: "local.tool".into(),
            command_id: "other".into(),
            command_label: "My Tool".into(),
            args: vec![],
        };
        assert_eq!(content_key(&first), content_key(&renamed));
        assert_ne!(content_key(&first), content_key(&other));
        // The NUL separator makes the split unambiguous.
        let ab_c = DetachRequest::External {
            extension_id: "ab".into(),
            command_id: "c".into(),
            command_label: "x".into(),
            args: vec![],
        };
        let a_bc = DetachRequest::External {
            extension_id: "a".into(),
            command_id: "bc".into(),
            command_label: "x".into(),
            args: vec![],
        };
        assert_ne!(content_key(&ab_c), content_key(&a_bc));
    }

    /// The text identity is the trimmed title: the body is content, not
    /// identity, so the same title with a new snapshot is the same window.
    #[test]
    fn the_text_key_is_the_trimmed_title() {
        let first = DetachRequest::Text {
            title: "Note".into(),
            text: "one".into(),
        };
        let rewritten = DetachRequest::Text {
            title: " Note ".into(),
            text: "two".into(),
        };
        let other = DetachRequest::Text {
            title: "Other".into(),
            text: "one".into(),
        };
        assert_eq!(content_key(&first), "Note");
        assert_eq!(content_key(&first), content_key(&rewritten));
        assert_ne!(content_key(&first), content_key(&other));
    }

    /// R84/R90 · the pull-slot contract, R91 · now per label: a parked request
    /// is handed over exactly once, a second pull sees `None` (a remount never
    /// re-runs what the user already consumed), and the flag is what remembers
    /// that.
    #[test]
    fn a_slot_hands_its_request_over_exactly_once() {
        let mut slot = PendingSlot {
            request: DetachRequest::External {
                extension_id: "local.tool".into(),
                command_id: "run".into(),
                command_label: "My Tool".into(),
                args: vec!["--flag".into()],
            },
            delivered: false,
        };
        let parked = slot.request.clone();
        assert_eq!(slot.take().as_ref(), Some(&parked));
        assert!(slot.take().is_none());
        assert!(slot.take().is_none());
        assert!(slot.delivered);
    }
}
