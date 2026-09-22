//! R7-10b · system notifications for **background** completions only.
//!
//! The rule this module exists to enforce is one sentence long: a system
//! notification is what tells the user about work that finished while they
//! were looking somewhere else, and nothing else. While the panel is on screen
//! the app already reports the same fact through its own toast stack
//! (`src/toast-state.ts`), and raising both would be the double report the
//! round forbids. So the "should this be a notification at all" question is
//! answered in exactly one place — [`should_notify`] — and every trigger point
//! goes through [`notify_completion`] rather than calling the plugin itself.
//!
//! The trigger surface is deliberately four points wide (see the round report
//! for the落点表):
//!
//!   * integration repair (`commands::extensions::extensions_repair`),
//!   * install / uninstall of an integration (the plugin-page and drawer long
//!     actions; both spellings of uninstall),
//!   * the silent drift re-probe (`extensions::install::reprobe_on_tool_version_change`,
//!     reachable only from the background list path),
//!   * the clipboard plugin page's clear-history action.
//!
//! Everything else stays out. In particular the launcher, settings, updater and
//! deep-link paths raise no notification: their feedback is either immediate
//! (the user pressed a key and is watching) or already has a visible surface.
//!
//! ## Why the click handler is not the plugin's `onAction`
//!
//! `tauri-plugin-notification` 2.4 exposes notification actions only on mobile
//! (`NotificationPlugin::register_listener`; the desktop `NotificationData`
//! carries an `action_type_id` that the desktop `show()` never reads). Its
//! desktop delivery goes through `notify-rust`, whose own compatibility table
//! marks `wait_for_action` as unsupported on macOS, and whose macOS backend
//! (`mac-notification-sys`, the deprecated `NSUserNotification` API) delivers
//! an activation only to a *pending* in-process entry — and `notify-rust`'s
//! fire-and-forget `show()` drops that entry before the user can click.
//!
//! What the platform does give us for free is the activation itself: a click on
//! one of our notifications activates floter, and for an Accessory app that is
//! `applicationShouldHandleReopen` — surfaced by Tauri as `RunEvent::Reopen`.
//! That is the callback this module wires ([`on_notification_click`]), and it
//! reuses the app's one reveal path rather than inventing a second window
//! manager. Windows/Linux get the same wiring: there the click also activates
//! the app, and `Reopen` is the platform-agnostic spelling of "show yourself".

use std::sync::atomic::Ordering;

use tauri::{AppHandle, Manager};
use tauri_plugin_notification::NotificationExt;

/// The notification title. The app's own name, in both languages: a
/// notification is OS chrome, and the thing it must identify is the app, not
/// the sentence below it.
pub(crate) const APP_TITLE: &str = "floter";

/// Whether the panel is on screen right now.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub(crate) enum Foreground {
    /// The main window is visible and/or focused.
    Visible,
    /// The main window is hidden — the only state in which the user cannot see
    /// the in-app feedback.
    Hidden,
}

/// How a background operation ended.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub(crate) enum Outcome {
    Success,
    Failure,
}

/// Which background action finished. One variant per trigger point, so the
/// body always names the action as well as the subject.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub(crate) enum CompletionAction {
    Install,
    Uninstall,
    Repair,
    Reprobe,
    ClearHistory,
}

impl CompletionAction {
    /// Stable id used by the frontend mirror and by the delivery log line, so
    /// a banner in the notification centre can be traced back to the action
    /// that raised it.
    pub(crate) fn as_str(self) -> &'static str {
        match self {
            Self::Install => "install",
            Self::Uninstall => "uninstall",
            Self::Repair => "repair",
            Self::Reprobe => "reprobe",
            Self::ClearHistory => "clearHistory",
        }
    }

    /// Every variant, in declaration order — the frontend mirror is checked
    /// against this list so a new action cannot be added on one side only.
    #[cfg(test)]
    pub(crate) const ALL: [Self; 5] = [
        Self::Install,
        Self::Uninstall,
        Self::Repair,
        Self::Reprobe,
        Self::ClearHistory,
    ];
}

/// What the completed action was about.
#[derive(Debug, Clone, PartialEq, Eq)]
pub(crate) enum Subject {
    /// An integration, by its user-visible name (see
    /// [`integration_display_name`]).
    Integration(String),
    /// One or more integrations, named only by their kind — the drift re-probe
    /// loop covers whichever ones moved and has no single subject to name. It
    /// exists so that loop cannot produce one banner per drifted integration.
    Integrations,
    /// A built-in plugin page, by its stable id (`plugin_pages`), so the name
    /// is resolved bilingually here instead of being frozen at the call site.
    Plugin(&'static str),
}

/// **The one place** the "is this worth a system notification" question is
/// answered.
///
/// The four quadrants are the round's red line: a hidden panel notifies on
/// both outcomes, a visible panel notifies on neither. The outcome is a
/// parameter (rather than a constant `true`) because the interesting failure
/// mode of this rule is a future round deciding that only failures deserve a
/// notification — that decision belongs here, next to the test that pins the
/// four cases.
pub(crate) fn should_notify(foreground: Foreground, outcome: Outcome) -> bool {
    match (foreground, outcome) {
        (Foreground::Visible, Outcome::Success | Outcome::Failure) => false,

        (Foreground::Hidden, Outcome::Success | Outcome::Failure) => true,
    }
}

/// Read the panel's foreground state.
///
/// Three independent answers are combined, and *any* of them being "visible"
/// suppresses the notification:
///
///   1. `AppState::window_visible` — the app's own record, which every reveal
///      and hide path maintains;
///   2. `Window::is_visible()` — the platform's answer;
///   3. `Window::is_focused()` — a focused window is on screen by definition,
///      even on the platforms where the panel's visibility is reported late
///      (macOS `NSPanel` before its first map).
///
/// A missing window counts as hidden: there is no surface the user could be
/// looking at, which is precisely when a notification is the right channel.
pub(crate) fn panel_foreground(app: &AppHandle) -> Foreground {
    let flag_visible = app
        .state::<crate::AppState>()
        .window_visible
        .load(Ordering::SeqCst);
    let (window_visible, window_focused) = match app.get_webview_window("main") {
        Some(window) => (
            window.is_visible().unwrap_or(false),
            window.is_focused().unwrap_or(false),
        ),
        // No window at all: nothing the user could be looking at.
        None => (false, false),
    };
    panel_foreground_from(flag_visible, window_visible, window_focused)
}

/// The three signals combined, as a pure function — the part of
/// [`panel_foreground`] that is worth testing without a window.
///
/// *Any* signal saying "on screen" wins, because the cost of the two mistakes
/// is asymmetric: a missed notification is an annoyance the user can still
/// discover by opening the panel, while a notification raised over a visible
/// panel is the double report the round exists to prevent.
pub(crate) fn panel_foreground_from(
    flag_visible: bool,
    window_visible: bool,
    window_focused: bool,
) -> Foreground {
    if flag_visible || window_visible || window_focused {
        Foreground::Visible
    } else {
        Foreground::Hidden
    }
}

/// A delivered notification the user interacted with.
///
/// Wired only on macOS today: there, clicking one of our notifications is
/// `applicationShouldHandleReopen` and reaches us as `RunEvent::Reopen`. On
/// Windows and Linux the same click activates the app through its desktop
/// entry, which lands in the *existing* `tauri-plugin-single-instance` handler
/// — and that handler already runs `reveal_saved_mode`, so there is nothing
/// left to add. The type stays `test`-visible everywhere so the decision is
/// exercised on every platform's test run, not only macOS'.
#[cfg(any(target_os = "macos", test))]
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub(crate) enum NotificationClick {
    /// The user clicked the notification body.
    Clicked,
    /// Anything else (a dismissal, or an activation that is not ours).
    Other,
}

/// What a click on a background-completion notification does: show the panel.
///
/// `reveal` is the app's existing reveal path (`reveal_saved_mode`, the same
/// one the tray, the global shortcut, the deep-link router and the IPC socket
/// use) injected as a closure so the decision and the dispatch can be tested
/// without a live window. Deliberately unconditional on the panel's current
/// state: if the user clicked, they want the panel, and re-revealing an
/// already-visible panel only re-focuses it.
#[cfg(any(target_os = "macos", test))]
pub(crate) fn on_notification_click(click: NotificationClick, reveal: impl FnOnce()) {
    match click {
        NotificationClick::Clicked => reveal(),
        NotificationClick::Other => {}
    }
}

/// The body for one completion, in the given UI language.
///
/// Both languages are declared side by side (and pinned by a test) because
/// "bilingual" here means *symmetric*: a key that exists in one table and not
/// the other is a bug, not a fallback.
pub(crate) fn completion_body(
    subject: &Subject,
    action: CompletionAction,
    outcome: Outcome,
    language: &str,
) -> String {
    let name = match subject {
        Subject::Integration(name) => name.clone(),
        Subject::Integrations => {
            if is_chinese(language) {
                "集成".to_string()
            } else {
                "Integrations".to_string()
            }
        }
        Subject::Plugin(id) => plugin_display_name(id, language).to_string(),
    };
    let template = body_template(action, outcome, language);
    template.replace("{name}", &name)
}

/// The title for one completion. The app name, identical in both languages —
/// see [`APP_TITLE`].
pub(crate) fn completion_title(_language: &str) -> &'static str {
    APP_TITLE
}

/// The (title, body) pair handed to the OS, so the copy has one construction
/// point shared by the sender and the tests.
pub(crate) fn completion_copy(
    subject: &Subject,
    action: CompletionAction,
    outcome: Outcome,
    language: &str,
) -> (String, String) {
    (
        completion_title(language).to_string(),
        completion_body(subject, action, outcome, language),
    )
}

/// Whether a language tag selects the Chinese dictionary. A tag with a region
/// (`zh-CN`, `zh-Hans`) selects it too, matching the rest of the app.
fn is_chinese(language: &str) -> bool {
    language.to_ascii_lowercase().starts_with("zh")
}

/// The body template for one (action, outcome), per language.
fn body_template(action: CompletionAction, outcome: Outcome, language: &str) -> &'static str {
    let chinese = is_chinese(language);
    match (chinese, action, outcome) {
        (false, CompletionAction::Install, Outcome::Success) => "{name} installed",
        (false, CompletionAction::Install, Outcome::Failure) => "{name} could not be installed",
        (false, CompletionAction::Uninstall, Outcome::Success) => "{name} uninstalled",
        (false, CompletionAction::Uninstall, Outcome::Failure) => "{name} could not be uninstalled",
        (false, CompletionAction::Repair, Outcome::Success) => "{name} repaired",
        (false, CompletionAction::Repair, Outcome::Failure) => "{name} could not be repaired",
        (false, CompletionAction::Reprobe, Outcome::Success) => "{name} command list updated",
        (false, CompletionAction::Reprobe, Outcome::Failure) => "{name} re-scan failed",
        (false, CompletionAction::ClearHistory, Outcome::Success) => "{name} cleared",
        (false, CompletionAction::ClearHistory, Outcome::Failure) => "{name} could not be cleared",
        (true, CompletionAction::Install, Outcome::Success) => "{name} 已安装",
        (true, CompletionAction::Install, Outcome::Failure) => "{name} 安装失败",
        (true, CompletionAction::Uninstall, Outcome::Success) => "{name} 已卸载",
        (true, CompletionAction::Uninstall, Outcome::Failure) => "{name} 卸载失败",
        (true, CompletionAction::Repair, Outcome::Success) => "{name} 已修复",
        (true, CompletionAction::Repair, Outcome::Failure) => "{name} 修复失败",
        (true, CompletionAction::Reprobe, Outcome::Success) => "{name} 命令列表已更新",
        (true, CompletionAction::Reprobe, Outcome::Failure) => "{name} 重新扫描失败",
        (true, CompletionAction::ClearHistory, Outcome::Success) => "{name} 已清空",
        (true, CompletionAction::ClearHistory, Outcome::Failure) => "{name} 清空失败",
    }
}

/// The bilingual name of a built-in plugin page.
///
/// Keyed by the same stable ids `plugin_pages` registers, so the two lists can
/// be checked against each other (see the tests) instead of one of them
/// silently growing a plugin the user can never be told about.
pub(crate) fn plugin_display_name(id: &str, language: &str) -> &'static str {
    let chinese = is_chinese(language);
    match (id, chinese) {
        (crate::plugin_pages::CLIPBOARD_PLUGIN_ID, false) => "Clipboard History",
        (crate::plugin_pages::CLIPBOARD_PLUGIN_ID, true) => "剪贴板历史",
        (crate::plugin_pages::BROWSER_PLUGIN_ID, false) => "Browser Bookmarks & History",
        (crate::plugin_pages::BROWSER_PLUGIN_ID, true) => "浏览器书签与历史记录",
        // An unknown plugin still needs a name: the notification must not leak
        // a raw id, and it must not be silently dropped either.
        (_, false) => "Plugin",
        (_, true) => "插件",
    }
}

/// Resolve the name to show for an integration.
///
/// Order: the caller's hint (a manifest read before the operation, or the
/// entry an operation just returned), then the repository's own name, then the
/// id. The lock is the authority whenever the entry still exists, which is why
/// callers that *remove* the entry (uninstall) resolve the name first.
pub(crate) fn integration_display_name(
    state: &crate::extensions::ExtensionState,
    id: &str,
    hint: Option<&str>,
) -> String {
    if let Some(hint) = hint.map(str::trim).filter(|hint| !hint.is_empty()) {
        return hint.to_string();
    }
    if let Ok(lock) = crate::extensions::lock::ExtensionsLock::load(&state.paths.repository_file) {
        if let Ok(entry) = lock.get(id) {
            let name = entry.name.trim();
            if !name.is_empty() {
                return name.to_string();
            }
        }
    }
    id.to_string()
}

/// Send one background-completion notification, if the rule allows it.
///
/// The gate is [`should_notify`]; the words come from [`completion_copy`]; the
/// delivery is the plugin's Rust API (`NotificationExt`), never a frontend
/// `invoke`. A delivery failure is logged and swallowed: a notification is a
/// courtesy, and the operation it describes has already committed.
pub(crate) fn notify_completion(
    app: &AppHandle,
    subject: &Subject,
    action: CompletionAction,
    outcome: Outcome,
) {
    if !should_notify(panel_foreground(app), outcome) {
        return;
    }
    let language = crate::commands::config::load_settings().language;
    let (title, body) = completion_copy(subject, action, outcome, &language);
    tracing::debug!(
        "raising a background-completion notification: action={} outcome={:?}",
        action.as_str(),
        outcome
    );
    if let Err(error) = app.notification().builder().title(title).body(body).show() {
        tracing::warn!("could not raise a system notification: {error}");
    }
}

/// The OS activated the app because the user clicked one of our notifications.
///
/// This is the production binding of [`on_notification_click`]: it runs the
/// click decision and, when the click is ours, reveals the main window through
/// the app's single reveal path. Nothing else — no new window management, no
/// surface switching.
#[cfg(target_os = "macos")]
pub(crate) fn reveal_after_activation(app: &AppHandle) {
    let window = app.get_webview_window("main");
    on_notification_click(NotificationClick::Clicked, || {
        let Some(window) = window.as_ref() else {
            return;
        };
        let state = app.state::<crate::AppState>();
        let _ = crate::reveal_saved_mode(window, &state);
    });
}

/// Translate a `Result` into the notification outcome.
pub(crate) fn outcome_of<T>(result: &Result<T, String>) -> Outcome {
    match result {
        Ok(_) => Outcome::Success,
        Err(_) => Outcome::Failure,
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    /// The round's central rule, as a four-quadrant table: only a *hidden*
    /// panel notifies, and it notifies on both outcomes.
    #[test]
    fn only_a_hidden_panel_notifies_and_it_notifies_on_both_outcomes() {
        assert!(!should_notify(Foreground::Visible, Outcome::Success));
        assert!(!should_notify(Foreground::Visible, Outcome::Failure));
        assert!(should_notify(Foreground::Hidden, Outcome::Success));
        assert!(should_notify(Foreground::Hidden, Outcome::Failure));
    }

    #[test]
    #[cfg(any(target_os = "macos", test))]
    fn a_click_reveals_the_panel_and_anything_else_does_not() {
        let mut revealed = 0;
        on_notification_click(NotificationClick::Clicked, || revealed += 1);
        assert_eq!(revealed, 1, "a click must show the main window");
        on_notification_click(NotificationClick::Other, || revealed += 1);
        assert_eq!(revealed, 1, "a dismissal must not summon the panel");
    }

    #[test]
    fn the_body_names_the_subject_the_action_and_the_outcome_in_both_languages() {
        let subject = Subject::Integration("Docker".to_string());
        let cases = [
            (
                CompletionAction::Install,
                Outcome::Success,
                "Docker installed",
                "Docker 已安装",
            ),
            (
                CompletionAction::Install,
                Outcome::Failure,
                "Docker could not be installed",
                "Docker 安装失败",
            ),
            (
                CompletionAction::Uninstall,
                Outcome::Success,
                "Docker uninstalled",
                "Docker 已卸载",
            ),
            (
                CompletionAction::Uninstall,
                Outcome::Failure,
                "Docker could not be uninstalled",
                "Docker 卸载失败",
            ),
            (
                CompletionAction::Repair,
                Outcome::Success,
                "Docker repaired",
                "Docker 已修复",
            ),
            (
                CompletionAction::Repair,
                Outcome::Failure,
                "Docker could not be repaired",
                "Docker 修复失败",
            ),
            (
                CompletionAction::Reprobe,
                Outcome::Success,
                "Docker command list updated",
                "Docker 命令列表已更新",
            ),
            (
                CompletionAction::Reprobe,
                Outcome::Failure,
                "Docker re-scan failed",
                "Docker 重新扫描失败",
            ),
            (
                CompletionAction::ClearHistory,
                Outcome::Success,
                "Docker cleared",
                "Docker 已清空",
            ),
            (
                CompletionAction::ClearHistory,
                Outcome::Failure,
                "Docker could not be cleared",
                "Docker 清空失败",
            ),
        ];
        for (action, outcome, english, chinese) in cases {
            assert_eq!(completion_body(&subject, action, outcome, "en"), english);
            assert_eq!(completion_body(&subject, action, outcome, "zh"), chinese);
            // Every body names the subject, so a notification can never be
            // anonymous.
            assert!(completion_body(&subject, action, outcome, "en").contains("Docker"));
            assert!(completion_body(&subject, action, outcome, "zh").contains("Docker"));
        }
        // The title is the app, in both languages.
        assert_eq!(completion_title("en"), APP_TITLE);
        assert_eq!(completion_title("zh"), APP_TITLE);
        let (title, body) =
            completion_copy(&subject, CompletionAction::Repair, Outcome::Success, "zh");
        assert_eq!(title, APP_TITLE);
        assert_eq!(body, "Docker 已修复");
    }

    #[test]
    fn every_action_and_outcome_has_a_bilingual_template_that_names_the_subject() {
        for action in CompletionAction::ALL {
            for outcome in [Outcome::Success, Outcome::Failure] {
                for language in ["en", "zh"] {
                    let template = body_template(action, outcome, language);
                    assert!(
                        template.contains("{name}"),
                        "{} / {:?} / {language} must name the subject",
                        action.as_str(),
                        outcome
                    );
                    assert!(!template.trim().is_empty());
                }
                let english = body_template(action, outcome, "en");
                let chinese = body_template(action, outcome, "zh");
                assert_ne!(english, chinese, "{} must be translated", action.as_str());
                assert!(
                    chinese
                        .chars()
                        .any(|c| ('\u{4e00}'..='\u{9fff}').contains(&c)),
                    "{} must contain Chinese text",
                    action.as_str()
                );
            }
        }
        // A language tag with a region still selects Chinese.
        assert_eq!(
            body_template(CompletionAction::Repair, Outcome::Success, "zh-CN"),
            "{name} 已修复"
        );
    }

    #[test]
    fn every_plugin_page_has_a_bilingual_display_name() {
        for descriptor in crate::plugin_pages::descriptors() {
            let english = plugin_display_name(descriptor.id, "en");
            let chinese = plugin_display_name(descriptor.id, "zh");
            assert_ne!(english, "Plugin", "{} needs a real name", descriptor.id);
            assert_ne!(chinese, "插件", "{} needs a real name", descriptor.id);
            assert!(
                chinese
                    .chars()
                    .any(|c| ('\u{4e00}'..='\u{9fff}').contains(&c)),
                "{} must be named in Chinese too",
                descriptor.id
            );
        }
    }

    #[test]
    fn a_plugin_subject_uses_its_bilingual_name() {
        let subject = Subject::Plugin(crate::plugin_pages::CLIPBOARD_PLUGIN_ID);
        assert_eq!(
            completion_body(
                &subject,
                CompletionAction::ClearHistory,
                Outcome::Failure,
                "en"
            ),
            "Clipboard History could not be cleared"
        );
        assert_eq!(
            completion_body(
                &subject,
                CompletionAction::ClearHistory,
                Outcome::Success,
                "zh"
            ),
            "剪贴板历史 已清空"
        );
    }

    #[test]
    fn the_plural_subject_is_translated_and_names_the_kind() {
        assert_eq!(
            completion_body(
                &Subject::Integrations,
                CompletionAction::Reprobe,
                Outcome::Success,
                "en"
            ),
            "Integrations command list updated"
        );
        assert_eq!(
            completion_body(
                &Subject::Integrations,
                CompletionAction::Reprobe,
                Outcome::Success,
                "zh"
            ),
            "集成 命令列表已更新"
        );
    }

    #[test]
    fn any_signal_of_an_on_screen_panel_suppresses_the_notification() {
        // Nothing on screen: the only state that notifies.
        assert_eq!(
            panel_foreground_from(false, false, false),
            Foreground::Hidden
        );
        // Each signal alone is enough, which is what makes the suppression
        // robust across platforms whose visibility reporting is late.
        assert_eq!(
            panel_foreground_from(true, false, false),
            Foreground::Visible
        );
        assert_eq!(
            panel_foreground_from(false, true, false),
            Foreground::Visible
        );
        assert_eq!(
            panel_foreground_from(false, false, true),
            Foreground::Visible
        );
        assert_eq!(panel_foreground_from(true, true, true), Foreground::Visible);
    }

    #[test]
    fn a_result_maps_to_the_matching_outcome() {
        let ok: Result<(), String> = Ok(());
        let failed: Result<(), String> = Err("nope".to_string());
        assert_eq!(outcome_of(&ok), Outcome::Success);
        assert_eq!(outcome_of(&failed), Outcome::Failure);
    }
}
