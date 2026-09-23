// R28 · the browser plugin's *output* — and nothing else.
//
// R26-A/R26-B taught the browser mode to build `LauncherItem` rows inside the
// catalog hook: it knew the launcher's item variants, its disabled flag and its
// status-row idiom. This module is the R28 split: the plugin produces standard
// `PluginRow`s (see `launcher/plugin-mode.ts`) and the capability layer decides
// what they look like, whether they are selectable and how tall the window is.
//
// The decisions that stay here are the browser's own — which of bookmarks,
// history and live tabs to fetch, how the two groups merge, when a group is
// empty-but-not-failed — because those are facts about browsers, not about
// launchers.

import type { MessageKey, Translate } from "../../i18n.ts";
import type { PluginRow } from "../../launcher/plugin-mode.ts";
import { MAX_RESULTS } from "../../launcher/result-budget.ts";

/** One row from `browser_search_bookmarks` / `browser_search_history`. The two
 *  commands share a shape; the fields one kind does not use are simply absent
 *  (`#[serde(skip_serializing_if)]` on the Rust side). */
export type BrowserSearchRow = {
  id: string;
  title: string;
  url: string;
  profile_key: string;
};

/** One row from `browser_list_tabs` — a tab the browser has open now.
 *  `window_index`/`tab_index` identify it for `browser_activate_tab`. */
export type BrowserTabRow = {
  browser_id: string;
  window_index: number;
  tab_index: number;
  title: string;
  url: string;
  active: boolean;
};

/** How many browser rows to fetch. The launcher renders at most eight matched
 *  rows (`MAX_RESULTS - 1`), and the merge drops duplicate URLs, so a small
 *  over-fetch keeps the visible list full without an unbounded query.
 *
 * R29 · raised to the backend's own `MAX_LIMIT` (500): the inline mode now
 *  fetches once and pages the held rows client-side (`paginatePluginRows`), so
 *  the fetch has to cover the pages the user may scroll through. 200 is a
 *  deliberate half of that ceiling — deep enough for a long history without
 *  reading a half-megabyte of bookmarks on every keystroke pause.
 *
 * R30 · and the inline mode actually passes it (see `useLauncherCatalog`). R29
 *  raised this constant and left the hook calling the function without a
 *  `limit`, so every browser list was still capped at the *default* group
 *  ceiling below — eight rows, or nine with a status line — which is one page:
 *  the emission was never taller than the viewport, `paginatePluginRows` never
 *  found a remainder, no `page` block was attached, and the scroll-to-load-more
 *  path was dead for the plugin the round was written for. */
export const BROWSER_FETCH_LIMIT = 200;

/** The ceiling on one browser group. Bookmarks and history share the first
 *  group (bookmarks win, history fills); the live tabs are a second, so a
 *  browser with nothing open cannot hide the bookmarks and a browser with a
 *  full bookmark bar cannot hide the tabs. The launcher list scrolls when the
 *  two groups together outgrow its box. */
export const BROWSER_GROUP_LIMIT = MAX_RESULTS - 1;

/** A status line: the soft landing for "no profile", "nothing matched" and "the
 *  plugin is off". Information, not a door — `kind` says so to the capability
 *  layer (which draws it as the launcher's muted note, R30) and `disabled` says
 *  so to the tier rule (a list of only these is display-only). R31 · the tab
 *  read's failure is no longer one of these; the guidance moved to the settings
 *  field (see {@link browserSearchRows}). */
export const browserStatusRow = (id: string, key: MessageKey, t: Translate): PluginRow => ({
  family: "browser",
  id,
  title: t(key),
  subtitle: "",
  url: "",
  profileKey: "default",
  disabled: true,
  kind: "status",
});

/**
 * Merge the three browser sources into one list of rows.
 *
 * Bookmarks and history are one group (a URL that is both is one result, and
 * the curated bookmark wins); live tabs are a second group, deliberately *not*
 * deduplicated against the first — a tab that is also a bookmark is two
 * different actions (switch to it vs. open it again), so both rows stay.
 *
 * R31 · a failed tab read (no debug port, browser closed, AppleScript timed
 * out) leaves the tab group empty and **says nothing in the list**. R26-B had it
 * emit a disabled line, and R30 moved that line to the head as a banner; the
 * user's verdict was that the list is not the place for setup instructions
 * (「顶部的提示有点生硬，可以只在设置页面相关配置上提示就行了」). The note read as a
 * result sitting among results, and the guidance it carried is now the help text
 * of the very field that fixes it (`plugins.config.cdpEnabledHint`). Bookmarks
 * and history keep rendering whatever happens, and a genuinely empty search
 * still falls to the one `browser-empty` note below — a search outcome, not a
 * configuration nag. The `kind: "status"` capability itself is untouched; the
 * browser simply does not emit one for this case any more.
 */
export const browserSearchRows = (options: {
  bookmarks: readonly BrowserSearchRow[];
  history: readonly BrowserSearchRow[];
  tabs: readonly BrowserTabRow[];
  profileKey: string;
  t: Translate;
  /** R29 · the ceiling on each of the two groups. Defaults to
   *  {@link BROWSER_GROUP_LIMIT}; the launcher's inline mode raises it and
   *  pages the result client-side (`paginatePluginRows`), so the fetch is one
   *  call while the *display* stays windowed. */
  limit?: number;
}): PluginRow[] => {
  const { bookmarks, history, tabs, profileKey, t } = options;
  const cap = options.limit ?? BROWSER_GROUP_LIMIT;
  const seen = new Set<string>();
  const rows: PluginRow[] = [];
  // Bookmarks first, then history: a URL that is both is one result and the
  // curated bookmark wins. R31 · each row remembers which list it came from,
  // so the launcher can mark a bookmark with a bookmark glyph and a history
  // entry with a clock — the merge is the only place that knows, because the
  // two lists become one row list here.
  const merged: { row: BrowserSearchRow; source: "bookmark" | "history" }[] = [
    ...bookmarks.map((row) => ({ row, source: "bookmark" as const })),
    ...history.map((row) => ({ row, source: "history" as const })),
  ];
  for (const { row, source } of merged) {
    if (!row.url || seen.has(row.url)) continue;
    seen.add(row.url);
    rows.push({
      family: "browser",
      id: row.id,
      title: row.title || row.url,
      subtitle: row.url,
      url: row.url,
      profileKey: row.profile_key,
      source,
    });
    if (rows.length >= cap) break;
  }
  for (const tab of tabs) {
    if (!tab.url && !tab.title) continue;
    rows.push({
      family: "browser",
      id: `tab:${tab.browser_id}:${tab.window_index}:${tab.tab_index}`,
      title: tab.title || tab.url,
      subtitle: tab.url,
      url: tab.url,
      profileKey,
      tab: {
        browserId: tab.browser_id,
        windowIndex: tab.window_index,
        tabIndex: tab.tab_index,
      },
    });
    if (rows.length >= cap * 2) break;
  }
  // R31 · a failed tab read is no longer a row. R30 drew it as a status note at
  // the head of the list (「标签页不可用——请查看浏览器插件设置」), and the user's
  // verdict was that the list is the wrong place for setup instructions — the
  // note reads as a result and the guidance belongs on the settings field that
  // fixes it (see `plugins.config.cdpEnabledHint`). Bookmarks and history keep
  // rendering whatever happens; an empty result still falls to the one empty
  // note below, which is a search outcome, not a configuration nag.
  return rows.length ? rows : [browserStatusRow("browser-empty", "launcher.browserEmpty", t)];
};
