// R7-10b · the frontend's copy of the system-notification contract.
//
// The notification itself is painted by the OS and raised by Rust (see
// `src-tauri/src/notifications.rs`): the background paths that finish while the
// panel is hidden live on the Rust side, so sending from there is what keeps
// the frontend out of a decision it cannot observe. That leaves exactly two
// things this module owns, and both are pure:
//
//   1. the *words*, in both languages, so the sentence a user sees in their
//      notification centre is reviewable next to the rest of the app's copy
//      rather than buried in a Rust match arm;
//   2. the vocabulary (`NOTIFICATION_ACTIONS`, `NOTIFICATION_OUTCOMES`,
//      `NOTIFICATION_PLUGIN_NAMES`), which the node suite checks against the
//      Rust tables so the two sides cannot drift apart silently.
//
// The rule the whole round turns on — a system notification is for background
// completions only, never a second copy of an on-screen toast — lives in Rust
// because that is where the panel's visibility is known. `shouldNotify` below
// is its mirror, so the frontend can read the same four-quadrant table without
// having to re-derive it.
//
// There is no delivery here, and no `@tauri-apps/plugin-notification`
// dependency: adding one would create a second way to raise a notification,
// which is the one thing this module must not do.
//
// The copy itself is *not* declared here — it lives in `src/i18n.ts` with every
// other user-visible string. This module is the typed accessor over it, so the
// node suite can compare the app's words against the Rust table that actually
// paints them (see `tests/system-notifications.test.ts`).

import { createTranslator, isMessageKey, type MessageKey } from "./i18n.ts";

/** The notification title: the app's own name, in both languages. */
export const NOTIFICATION_TITLE = "floter";

/** How a background operation ended. Mirrors `notifications::Outcome`. */
export type NotificationOutcome = "success" | "failure";

/** Which background action finished. Mirrors `notifications::CompletionAction`. */
export type NotificationAction =
  | "install"
  | "uninstall"
  | "repair"
  | "reprobe"
  | "clearHistory";

/** Every action, in the same order as the Rust `CompletionAction::ALL`. */
export const NOTIFICATION_ACTIONS: readonly NotificationAction[] = [
  "install",
  "uninstall",
  "repair",
  "reprobe",
  "clearHistory",
];

/** Every outcome, in the same order as the Rust enum. */
export const NOTIFICATION_OUTCOMES: readonly NotificationOutcome[] = ["success", "failure"];

/** Whether the panel is on screen. Mirrors `notifications::Foreground`. */
export type PanelForeground = "visible" | "hidden";

/**
 * The one place the frontend states the round's rule, mirroring the Rust
 * `should_notify`:
 *
 * | | success | failure |
 * |---|---|---|
 * | visible | no | no |
 * | hidden | **yes** | **yes** |
 *
 * A visible panel means the user can see the toast/drawer, so a system
 * notification would be the same fact reported twice. Only a hidden panel
 * leaves the OS as the sole surface. The outcome is accepted (and ignored)
 * rather than dropped so the four-quadrant table is spelled out in the type,
 * matching the Rust signature a future "failures only" round would edit.
 */
export const shouldNotify = (foreground: PanelForeground, _outcome: NotificationOutcome): boolean =>
  foreground === "hidden";

/**
 * The built-in plugin-page ids this module has a notification name for,
 * mirroring `plugin_pages.rs`' registry. The node suite checks it against the
 * Rust registry so a new plugin page cannot ship without a way to be named in
 * a notification.
 */
export const NOTIFICATION_PLUGIN_IDS: readonly string[] = [
  "builtin.clipboard",
  "builtin.browser",
  // R51 · the calculator is a registered descriptor now, so a notification
  // about it (a failed history clear) must be able to name it.
  "builtin.calculator",
];

/** What a notification is about, mirroring `notifications::Subject`. */
export type NotificationSubject =
  | { kind: "integration"; name: string }
  | { kind: "integrations" }
  | { kind: "plugin"; id: string };

/** A language tag selects the Chinese dictionary when it starts with `zh`,
 * region suffix and all — the same rule the rest of the app uses. */
export const isChinese = (language: string): boolean => language.toLowerCase().startsWith("zh");

/** The dictionary key of the body template for one (action, outcome). */
export const notificationBodyKey = (
  action: NotificationAction,
  outcome: NotificationOutcome,
): MessageKey => `notification.${action}.${outcome}` as MessageKey;

/** The raw template for one (action, outcome) in one language. */
export const notificationTemplate = (
  action: NotificationAction,
  outcome: NotificationOutcome,
  language: string,
): string => createTranslator(isChinese(language) ? "zh" : "en")(notificationBodyKey(action, outcome));

/** The dictionary key naming a subject: the plural kind, or a plugin page's
 * registered name. An integration carries its own name and needs no key. */
export const notificationSubjectKey = (subject: NotificationSubject): MessageKey | null => {
  switch (subject.kind) {
    case "integration":
      return null;
    case "integrations":
      return "notification.subject.integrations";
    case "plugin":
      return isMessageKey(`notification.plugin.${subject.id}`)
        ? (`notification.plugin.${subject.id}` as MessageKey)
        : "notification.subject.plugin";
  }
};

/** The name to print for a subject: the integration's own name, the plural
 * kind, or the plugin page's bilingual name. */
export const subjectName = (subject: NotificationSubject, language: string): string => {
  const translate = createTranslator(isChinese(language) ? "zh" : "en");
  if (subject.kind === "integration") {
    return subject.name.trim() === ""
      ? translate("notification.subject.integrations")
      : subject.name;
  }
  const key = notificationSubjectKey(subject);
  return key ? translate(key) : "";
};

/** The notification body for one completion, in the given language. */
export const notificationBody = (
  subject: NotificationSubject,
  action: NotificationAction,
  outcome: NotificationOutcome,
  language: string,
): string =>
  notificationTemplate(action, outcome, language).replace(
    "{name}",
    subjectName(subject, language),
  );

/** The (title, body) pair the OS is handed. */
export const notificationCopy = (
  subject: NotificationSubject,
  action: NotificationAction,
  outcome: NotificationOutcome,
  language: string,
): { title: string; body: string } => ({
  title: NOTIFICATION_TITLE,
  body: notificationBody(subject, action, outcome, language),
});
