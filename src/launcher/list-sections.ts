// R72 · which rows print a section heading above themselves.
//
// The launcher's list is a sequence, and four kinds of row begin a *block* that
// the renderer announces with a `.launcher-section-title` line:
//
//   · `history`  — the last few typed commands, below the recents on the empty
//     page and below a query's app matches otherwise;
//   · `files`    — a dropped file's group (its row kinds, `file` and
//     `file-more`, are one block);
//   · `pluginGroup` — an external plugin's own `group` field.
//
// The rule lived twice before this round: once inline in `LauncherResults` (to
// draw) and nowhere at all in the height accounting (to charge) — so a list with
// a heading was drawn a heading taller than the window holding it and its last
// row was cut in half. The user's second screenshot is exactly that (the
// browser's 标签页 list: the group heading above three rows, the fourth row
// clipped), and the ordinary page had the same hole for a query that reached
// history.
//
// Pure, and the renderer and the band table both read it, so "drawn" and
// "charged" cannot drift apart again.
//
// The *page-level* heading — the empty query's 「最近启动」 — is deliberately not
// part of this: it is drawn once above the whole list (`launcherSectionTitle` in
// `App.tsx`), not per block, and it is charged by the caller that draws it.

import type { LauncherItem } from "./LauncherResults.ts";

/** The kinds of block a row can start.
 *
 *  R73 · the browser's live tabs used to be one of them (`browserTabs`), with a
 *  「打开的标签页」 heading printed above them. The user removed it: 「中间特殊的
 *  "打开的标签页" 提示可以去掉」. The tab rows are told apart by their own glyph and
 *  subtitle, the chips already filter to them, and a heading in the middle of a
 *  ten-row list costs a row of the budget for a word the list does not need. */
export type ListSectionKind = "history" | "files" | "pluginGroup";

/** One row's own group name, when the block is an external plugin's `group`. */
const pluginGroupOf = (item: LauncherItem): string | null =>
  item.type === "plugin" ? item.group ?? null : null;

const isFileRow = (item: LauncherItem): boolean =>
  item.type === "file" || item.type === "file-more";

/**
 * One entry per row: the block kind that row starts, or `null` for a row that
 * continues the block above it.
 */
export const listSectionStarts = (
  items: readonly LauncherItem[],
): Array<ListSectionKind | null> =>
  items.map((item, index) => {
    const previous = index > 0 ? items[index - 1] : null;
    if (item.type === "history") {
      return previous?.type === "history" ? null : "history";
    }
    if (isFileRow(item)) {
      return previous && isFileRow(previous) ? null : "files";
    }
    const group = pluginGroupOf(item);
    if (group !== null) {
      return previous && pluginGroupOf(previous) === group ? null : "pluginGroup";
    }
    return null;
  });

/** How many section headings the list draws above its own rows. The height
 *  accounting charges `LAUNCHER_SECTION_TITLE_CHROME` for each. */
export const listSectionTitleCount = (items: readonly LauncherItem[]): number =>
  listSectionStarts(items).reduce((total, kind) => total + (kind === null ? 0 : 1), 0);
