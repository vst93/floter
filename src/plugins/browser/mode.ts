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
 *  over-fetch keeps the visible list full without an unbounded query. */
export const BROWSER_FETCH_LIMIT = 24;

/** The ceiling on one browser group. Bookmarks and history share the first
 *  group (bookmarks win, history fills); the live tabs are a second, so a
 *  browser with nothing open cannot hide the bookmarks and a browser with a
 *  full bookmark bar cannot hide the tabs. The launcher list scrolls when the
 *  two groups together outgrow its box. */
export const BROWSER_GROUP_LIMIT = MAX_RESULTS - 1;

/** A status line: the soft landing for "no profile", "nothing matched", "the
 *  plugin is off" and "the tab read failed". Information, not a door — the
 *  capability layer turns a list of only these into the display tier. */
export const browserStatusRow = (id: string, key: MessageKey, t: Translate): PluginRow => ({
  family: "browser",
  id,
  title: t(key),
  subtitle: "",
  url: "",
  profileKey: "default",
  disabled: true,
});

/**
 * Merge the three browser sources into one list of rows.
 *
 * Bookmarks and history are one group (a URL that is both is one result, and
 * the curated bookmark wins); live tabs are a second group, deliberately *not*
 * deduplicated against the first — a tab that is also a bookmark is two
 * different actions (switch to it vs. open it again), so both rows stay.
 *
 * When the tab read failed outright (no debug port, browser closed, AppleScript
 * timed out) the group is empty and one disabled line says so, rather than the
 * user wondering why a running browser's tabs are missing.
 */
export const browserSearchRows = (options: {
  bookmarks: readonly BrowserSearchRow[];
  history: readonly BrowserSearchRow[];
  tabs: readonly BrowserTabRow[];
  /** Whether the tab read failed, as opposed to simply returning nothing. */
  tabsFailed: boolean;
  profileKey: string;
  t: Translate;
}): PluginRow[] => {
  const { bookmarks, history, tabs, tabsFailed, profileKey, t } = options;
  const seen = new Set<string>();
  const rows: PluginRow[] = [];
  for (const row of [...bookmarks, ...history]) {
    if (!row.url || seen.has(row.url)) continue;
    seen.add(row.url);
    rows.push({
      family: "browser",
      id: row.id,
      title: row.title || row.url,
      subtitle: row.url,
      url: row.url,
      profileKey: row.profile_key,
    });
    if (rows.length >= BROWSER_GROUP_LIMIT) break;
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
    if (rows.length >= BROWSER_GROUP_LIMIT * 2) break;
  }
  if (!tabs.length && tabsFailed) {
    rows.push(browserStatusRow("browser-tabs-unavailable", "launcher.browserTabsUnavailable", t));
  }
  return rows.length ? rows : [browserStatusRow("browser-empty", "launcher.browserEmpty", t)];
};
