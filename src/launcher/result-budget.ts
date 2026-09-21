// R10-A · How many rows the launcher shows, and the one row that is always
// there.
//
// The user's report, verbatim: "当下的搜索页面高度太低了，搜索的时候展示的项
// 太少，需要展示 9 项加一个底部的单独进入剪切板的项 一共 10 项". So the budget
// is ten rows: nine matched results plus one fixed row at the tail that opens
// the clipboard history. The fixed row is the launcher's tenth slot — it is not
// a match, so the catalog itself never returns more than nine.
//
// R19 · the user re-read that same screenshot and counted the rows: "列表项也有
// 问题，除了末尾的 cmd+回车的终端执行，上方一共有 10 项了，应该最大只能有 9
// 项". Nine above the action bar is the whole list, so the budget is nine rows:
// **eight matched results plus the fixed clipboard row**. The tail row keeps the
// last number of the `1`-`9` family (`FIXED_TAIL_SLOT`), which is what finally
// lets it carry a real `⌘9` badge instead of a blank one.
//
// This lives in its own pure module (no React, no Tauri) for the same reason
// `file-drops.ts` does: the budget and the tail row are the two things a review
// can silently delete, and a node test can only pin them if it can import them
// without dragging the app's runtime in.

// The explicit `.ts` is what lets this module be imported by the node suite:
// `../launcher` is a directory and node resolves the extension literally.
import type { Translate } from "../i18n";
import type { LauncherItem } from "./LauncherResults";

/** Eight matched results + the one fixed clipboard row. */
export const MAX_RESULTS = 9;

/** The number the fixed clipboard row always carries: the last slot of the
 *  `1`-`9` family. It does not depend on how many rows were matched — the tail
 *  is the ninth row, so it is `⌘9` in every query state (see
 *  `shortcutSlotsWithFixedTail`). */
export const FIXED_TAIL_SLOT = MAX_RESULTS;

/** Matched rows when the query also reached applications or power actions:
 *  three catalog commands, leaving five slots for those local matches — the
 *  same 3-to-5 shape the launcher showed when the budget was eight rows. */
export const COMMAND_LIMIT_WITH_MATCHES = 3;

/** The two heights a result row is drawn at, in `--u` units. This pair is the
 *  single source of the numbers `styles/launcher.css` writes as
 *  `calc(var(--u) * N)` on `.launcher-result` and `.launcher-result--compact`;
 *  `tests/launcher-ten-rows.test.ts` asserts the three agree.
 *
 *  A row that prints a subtitle is two-line and takes the first; a row whose
 *  title stands alone (see `row-content.ts`) collapses to the second. */
export const ROW_HEIGHT_TWO_LINE = 42;
export const ROW_HEIGHT_COMPACT = 34;

/** The list's ceiling in `--u` units: **every** row at its tallest height.
 *
 *  R20 · the ceiling used to be computed from the *compact* height alone —
 *  `8 × ROW_HEIGHT_COMPACT + ROW_HEIGHT_TWO_LINE` = 314u — which is the height
 *  of the list in the state that happened to be photographed, not the height it
 *  can reach. A query that matches commands is exactly the state that breaks it:
 *  every command row carries a description, so all nine rows are two-line, the
 *  content is 28u taller than the ceiling it was measured against, and the list
 *  scrolls inside a cap that was sized for shorter rows. The ceiling is a
 *  *ceiling*, so it is now the worst case: `MAX_RESULTS × ROW_HEIGHT_TWO_LINE`
 *  = 378u. */
export const RESULTS_LIST_HEIGHT = MAX_RESULTS * ROW_HEIGHT_TWO_LINE;

/** What the ceiling adds on top of the rows: the fixed pixels that do not scale
 *  with the unit — the 14px scroll-edge reservation the scroller keeps as top
 *  padding (see `--scroll-edge` in `styles/base.css`), the nine 1px grid gaps
 *  around nine rows, and the empty-query section title (a `--text-body` line at
 *  1.4 plus its 6/4px padding ≈ 26px). `14 + 9 + 26 = 49`, and the constant
 *  keeps a pixel of slack at 50. */
export const RESULTS_LIST_CHROME = 50;

/** The fixed row's id. Stable across query states so a re-render keys it to
 *  the same row. */
export const CLIPBOARD_RESULT_ID = "system-clipboard-fixed";

/** Chrome the launcher draws around the result list: the query row, the action
 *  bar and the window's shadow margin. The App's inline
 *  `--launcher-results-height` cap is `screen.availHeight - this`, so the nine
 *  rows still fit a short display; on an ordinary one the CSS row budget (see
 *  `.launcher-results` in `styles/launcher.css`) is the smaller cap and is what
 *  binds.
 *
 *  R19 · re-audited against the sheet, segment by segment, because R18 changed
 *  the query block without touching this number. At the default interface step
 *  (`--ui-scale: 1`, so the unit is 1px):
 *
 *    `.collapsed-card__input-row`  56u  (min-height, pinned since R10)
 *    R18 breath below it            8u  (`margin-bottom` on the input row)
 *    `.launcher-bottom` padding     6u  (4u top + 2u bottom — R21: the tail
 *                                      pairs with the last row's own leading,
 *                                      see `styles/launcher.css`)
 *    action bar row                42u  (height — it is a row, see below)
 *    feedback row                  30u  (min-height, may appear)
 *    card margin / rounding slack   7u
 *    ─────────────────────────────────
 *                                 149u
 *
 *  R20 · the R19 audit above read the action bar as 30u (the *feedback* row's
 *  floor). `.launcher-action-bar` declares `height: calc(var(--u) * 42)`, so the
 *  chrome is 12u more than that audit claimed — the correction is in the list,
 *  not in the constant. `RESULTS_VIEWPORT_CHROME` is a **floor** for a short
 *  display, not a measurement, and it only has to be at least the chrome it
 *  stands for so that the cap it writes is never larger than the window can
 *  hold: 232u ≥ 149u, by 83u of slack. What R20 has to check is that the cap
 *  does not bind on an ordinary display, i.e. that
 *  `availHeight - 232 ≥ RESULTS_LIST_HEIGHT × 1 + RESULTS_LIST_CHROME` — the
 *  worst-case list plus its fixed chrome, `378 + 50 = 428px`. That holds for
 *  every display taller than 660px of work area, and it is asserted in
 *  `tests/launcher-ten-rows.test.ts`. */
export const RESULTS_VIEWPORT_CHROME = 232;

/** Whether a row is *a* clipboard row — the fixed one, or the one the query
 *  produced by matching the clipboard system command. Either way the list must
 *  not grow a second one. */
export const isClipboardResult = (item: LauncherItem): boolean =>
  item.type === "system" && item.action === "clipboard";

/** The tail row: the clipboard panel, reachable from every query state —
 *  including the query that matched nothing, which is exactly when a trip to
 *  the clipboard is the useful thing left to offer. It is the ninth and last
 *  row of the budget, so `⌘9` opens it. */
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
 * The numbered `select_result` slots for the visible rows — the single source
 * of the `⌘N` → row mapping, for the badges and the key handler alike.
 *
 * The family is `1`-`9` — one digit behind the modifiers, see
 * `matchesResultShortcut`. The fixed clipboard row is the ninth row, so R19
 * gives it `FIXED_TAIL_SLOT` (`9`) rather than a blank badge: the slot is real
 * now, because the budget is nine rows and the tail is the last of them. The
 * matched rows above it number `1`-`8` in order, skipping the rows that are
 * not runnable (`launcherShortcutSlots`' own rule) and never reaching `9` —
 * otherwise a list grown past the budget (dropped files prepended) could hand
 * `⌘9` to a match and take the clipboard panel off the keyboard.
 */
export const shortcutSlotsWithFixedTail = (
  rows: readonly LauncherItem[],
  runnableFlags: readonly boolean[],
): Array<number | null> => {
  const lastIndex = rows.length - 1;
  const tailOwnsLastSlot = rows[lastIndex]?.id === CLIPBOARD_RESULT_ID;
  let next = 0;
  return rows.map((_, index) => {
    if (tailOwnsLastSlot && index === lastIndex) return FIXED_TAIL_SLOT;
    if (!runnableFlags[index]) return null;
    next += 1;
    return next < FIXED_TAIL_SLOT ? next : null;
  });
};

/**
 * The row a numbered shortcut runs: the index in the composed list whose slot
 * is `slot`, or `-1` when no visible row carries that number.
 *
 * Every `⌘N` press goes through here, so the key and the badge it matches can
 * never be derived from two different rules.
 */
export const resultIndexForSlot = (
  slots: readonly (number | null)[],
  slot: number,
): number => slots.indexOf(slot);
