// R12 · What a launcher row actually prints.
//
// The user's screenshot, verbatim: "这好看吗？" — the empty-query list showed the
// word "应用程序" seventeen times: once as the subtitle of eight application rows
// and once as the right-hand source of nine. Neither occurrence told the user
// anything the row's own icon did not already say, so this module is the single
// place that decides which of the two strings a row earns.
//
// It is a pure module (no React, no Tauri) so the node suite can pin the rule
// without a DOM: the right-hand source word is dropped for applications (their
// type is implied by the list they live in), and the left subtitle is dropped
// whenever it is nothing but the row's own type word.
import { IS_MAC } from "../shortcuts.ts";
import type { MessageKey, Translate } from "../i18n";
import type { LauncherItem } from "./LauncherResults";

// Where an application came from, read off the shape of its path: `.app`
// bundles on macOS, `.desktop` entries on Linux, Start Menu shortcuts on
// Windows.
export const appSubtitleKey = (path: string): MessageKey => {
  if (IS_MAC) {
    if (path.startsWith("/Applications/")) return "launcher.application";
    if (path.startsWith("/System/Applications/")) return "launcher.systemApplication";
    if (path.includes("/Applications/")) return "launcher.userApplication";
    return "launcher.application";
  }
  if (/^([A-Za-z]:)?[\\/]Users[\\/]/.test(path)) return "launcher.userApplication";
  if (path.startsWith("/home/") || path.startsWith("/root/")) return "launcher.userApplication";
  if (/^\/(usr|opt|var)\//.test(path)) return "launcher.systemApplication";
  return "launcher.application";
};

/**
 * The row's *type word*: the string that says what kind of row this is rather
 * than what the row contains. For an application that is its install location
 * ("应用程序"), for a command the extension that contributed it, for a system
 * action "Floter 内置", and for history / dropped files their own section names.
 *
 * This is exactly the word the subtitle falls back to when the row has nothing
 * more useful to say, so comparing the two is what lets the renderer drop the
 * duplicate.
 */
export const rowTypeWord = (item: LauncherItem, t: Translate): string => {
  switch (item.type) {
    case "app":
      return t(appSubtitleKey(item.app.path));
    case "command":
      return item.sourceName;
    case "system":
      return t("extensions.builtIn");
    case "history":
      return t("launcher.history");
    case "file":
    case "file-more":
      return t("launcher.files");
  }
};

export type RowContent = {
  /** The right-hand source word, or `null` when the row does not need one. */
  source: string | null;
  /** The left subtitle, or `null` when it would only repeat the type word. */
  subtitle: string | null;
};

/**
 * The two optional strings a row prints, or `null` for each one the row is
 * better off without.
 *
 * * `source` — applications drop it: "应用程序" beside every app row pushed the
 *   `⌘N` badge off the edge and said nothing the icon did not. Every other kind
 *   keeps it (a command's extension name, the clipboard row's "Floter 内置").
 * * `subtitle` — printed only when it says more than the type word. An app whose
 *   subtitle is its real name ("WeCom") keeps it; one whose subtitle is just
 *   "应用程序" does not, and the row collapses to a single line.
 */
export const resultRowContent = (item: LauncherItem, t: Translate): RowContent => {
  const typeWord = rowTypeWord(item, t);
  const subtitle = item.type === "history" ? t("launcher.history") : item.subtitle;
  return {
    source: item.type === "app" ? null : typeWord,
    subtitle: subtitle === typeWord ? null : subtitle,
  };
};
