// R10-A · How many rows the launcher shows, and the one row that is always
// there.
//
// The user's report, verbatim: "当下的搜索页面高度太低了，搜索的时候展示的项
// 太少，需要展示 9 项加一个底部的单独进入剪切板的项 一共 10 项". So the budget
// is ten rows: nine matched results plus one fixed row at the tail that opens
// the clipboard history. The fixed row is the launcher's tenth slot — it is not
// a match, so the catalog itself never returns more than nine.
//
// This lives in its own pure module (no React, no Tauri) for the same reason
// `file-drops.ts` does: the budget and the tail row are the two things a review
// can silently delete, and a node test can only pin them if it can import them
// without dragging the app's runtime in.

// The explicit `.ts` is what lets this module be imported by the node suite:
// `../launcher` is a directory and node resolves the extension literally.
import { launcherShortcutSlots } from "../launcher.ts";
import type { Translate } from "../i18n";
import type { LauncherItem } from "./LauncherResults";

/** Nine matched results + the one fixed clipboard row. */
export const MAX_RESULTS = 10;

/** Matched rows when the query also reached applications or power actions:
 *  three catalog commands, leaving six slots for those local matches — the
 *  same 3-to-6 shape the launcher showed when the budget was nine rows. */
export const COMMAND_LIMIT_WITH_MATCHES = 3;

/** The fixed row's id. Stable across query states so a re-render keys it to
 *  the same row. */
export const CLIPBOARD_RESULT_ID = "system-clipboard-fixed";

/** Chrome the launcher draws around the result list: the query row, the action
 *  bar and the window's shadow margin. The App's inline
 *  `--launcher-results-height` cap is `screen.availHeight - this`, so the ten
 *  rows still fit a short display; on an ordinary one the CSS row budget (see
 *  `.launcher-results` in `styles/launcher.css`) is the smaller cap and is what
 *  binds. */
export const RESULTS_VIEWPORT_CHROME = 220;

/** Whether a row is *a* clipboard row — the fixed one, or the one the query
 *  produced by matching the clipboard system command. Either way the list must
 *  not grow a second one. */
export const isClipboardResult = (item: LauncherItem): boolean =>
  item.type === "system" && item.action === "clipboard";

/** The tenth row: the clipboard panel, reachable from every query state —
 *  including the query that matched nothing, which is exactly when a trip to
 *  the clipboard is the useful thing left to offer. */
export const clipboardResultRow = (t: Translate): LauncherItem => ({
  type: "system",
  id: CLIPBOARD_RESULT_ID,
  title: t("system.clipboardHistory"),
  subtitle: t("system.clipboardHistorySubtitle"),
  action: "clipboard",
});

/** Append the fixed clipboard row to the tail, unless the query already put a
 *  clipboard row in the list (typing "clipboard" matches the same action).
 *  Idempotent: a list that already ends in the fixed row is returned as it
 *  stands. */
export const withClipboardResultRow = (
  items: readonly LauncherItem[],
  t: Translate,
): LauncherItem[] =>
  items.some(isClipboardResult) ? [...items] : [...items, clipboardResultRow(t)];

/**
 * The numbered `select_result` slots for the visible rows.
 *
 * The family is `1`-`9` — one digit behind the modifiers, see
 * `matchesResultShortcut` — so a tenth runnable row cannot have a number: a
 * badge reading `⌘10` would name a key combination that can never be pressed.
 * The fixed clipboard row is exactly that tenth row, so it keeps a blank slot
 * and is reached with the arrows and Enter instead. Every matched row above it
 * numbers exactly as it did before.
 */
export const shortcutSlotsWithFixedTail = (
  rows: readonly LauncherItem[],
  runnableFlags: readonly boolean[],
): Array<number | null> => {
  const slots = launcherShortcutSlots([...runnableFlags]);
  const last = rows[rows.length - 1];
  if (last?.id === CLIPBOARD_RESULT_ID && slots.length) slots[slots.length - 1] = null;
  return slots;
};
