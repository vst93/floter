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
// type is implied by the list they live in) and for system actions ("Floter
// 内置" twice in one screenshot, beside a restart and a clipboard row whose
// icons already said as much), and the left subtitle is dropped whenever it is
// nothing but the row's own type word *or* a transcription of the row's title.
//
// R14, verbatim: 「还是很丑，清爽一点」 — the second half of that rule is the
// round. "QQ音乐 / QQMusic", "Safari浏览器 / Safari", "企业微信 / WeCom": each
// subtitle was the same name in the other language the platform happens to
// know, which is a second line spent on zero information.
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
    // R27 · a clipboard row's type word is the panel's own name; the row's
    // subtitle is its age (or the entry's source), so the word is dropped when
    // the two would say the same thing (see `resultRowContent`).
    case "clipboard":
      return t("system.clipboardHistory");
    case "history":
      return t("launcher.history");
    case "file":
    case "file-more":
      return t("launcher.files");
    // R26-A: a browser row is a bookmark or a history entry; the two share a
    // row shape, and the URL subtitle already says which site it is. R26-B: a
    // live-tab row is neither — its type word is the group heading it sits
    // under, so the row does not claim to be a bookmark.
    case "browser":
      return item.tab ? t("browserPage.tabs") : t("system.browserSearch");
  }
};

/**
 * A string with everything that cannot change *which name* is being said
 * folded away: Unicode compatibility (full-width → ASCII), case, whitespace
 * and punctuation. "QQ 音乐" and "qq音乐" collapse to one key; "WeCom" and
 * "wecom" do too.
 */
export const transcriptionKey = (value: string): string =>
  value
    .normalize("NFKC")
    .toLowerCase()
    .replace(/[\s\p{P}\p{S}]+/gu, "");

/**
 * Whether `subtitle` is the row's title written again — a case, width, spacing
 * or punctuation variant of it — rather than something new about the row.
 */
export const isTranscription = (title: string, subtitle: string): boolean =>
  transcriptionKey(title) === transcriptionKey(subtitle);

/**
 * Whether an application row's subtitle is the platform's *other* name for the
 * same app.
 *
 * The catalog prints `app.name` in the subtitle slot exactly when it differs
 * from the localized title (`app.localizedName || app.name`), so the pair is by
 * construction one app's name in two languages. Folding the strings cannot see
 * that: "企业微信" and "WeCom" share no character at all, and "QQ音乐" /
 * "QQMusic" only share an initialism. The app's own two name fields are the
 * authority, so this asks them rather than guessing from the glyphs. A genuine
 * remark (`app.comment`) is not one of those two names and survives.
 */
const isAlternateAppName = (
  item: Extract<LauncherItem, { type: "app" }>,
  subtitle: string,
): boolean => Boolean(item.app.localizedName) && item.app.name === subtitle;

export type RowContent = {
  /** The right-hand source word, or `null` when the row does not need one. */
  source: string | null;
  /** The left subtitle, or `null` when it would only repeat the type word or
   *  the title. */
  subtitle: string | null;
};

/**
 * The two optional strings a row prints, or `null` for each one the row is
 * better off without.
 *
 * * `source` — applications drop it ("应用程序" beside every app row pushed the
 *   `⌘N` badge off the edge and said nothing the icon did not), and so do
 *   system actions (R14: "Floter 内置" printed beside both the restart and the
 *   clipboard row is the same non-fact twice; the icon and the title carry the
 *   row, and the right edge is left to `⌘N`). A command keeps it — the
 *   extension that contributed the command is real information.
 * * `subtitle` — printed only when it says more than the type word *and* more
 *   than the title. "应用程序" and "打开剪贴板历史面板" are the two kinds that
 *   stay: a type word the row does not need and an action description the title
 *   does not contain. The other name for the same app is neither, so the row
 *   collapses to a single line.
 */
export const resultRowContent = (item: LauncherItem, t: Translate): RowContent => {
  const typeWord = rowTypeWord(item, t);
  const subtitle = item.type === "history" ? t("launcher.history") : item.subtitle;
  const transcription =
    (item.type === "app" && isAlternateAppName(item, subtitle)) ||
    isTranscription(item.title, subtitle);
  return {
    source:
      item.type === "app" || item.type === "system" || item.type === "browser" || item.type === "clipboard"
        ? null
        : typeWord,
    subtitle: subtitle === typeWord || transcription ? null : subtitle,
  };
};
