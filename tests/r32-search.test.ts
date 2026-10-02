// R32 · the launcher's token search, the browser range filter, and the two
// ways the field leaves a plugin mode.
//
// The user's report, verbatim: 「同时输入框中搜索时需要额外加个逻辑，当搜索框为空时
// 在删除应该也退出插件。再就是希望增加页面过滤的选项，按 tab 或点击可以切换，在
// 全部、书签、历史、Tab 几个中间切换…再就是搜索逻辑需要支持空格分割的且逻辑，设置
// 中增加对搜索内容的配置，可以配置 全部、标题、url」.
//
// Three things are pinned here: the shared AND rule every list now uses
// (`plugins/search.ts`), the four-value browser filter and its Tab cycle, and
// the empty-word Backspace that leaves a plugin. The App/hook wiring is pinned
// at the source, the same arrangement the rest of the launcher suite uses.

import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";
import { createTranslator } from "../src/i18n.ts";
import {
  BROWSER_FILTERS,
  cycleBrowserFilter,
  pluginModeExitOnBackspace,
} from "../src/launcher.ts";
import {
  LAUNCHER_FILTER_UNITS,
  launcherRowUnits,
} from "../src/launcher/result-budget.ts";
import { matchesTokens, searchTokens } from "../src/plugins/search.ts";
import {
  browserRowMatches,
  browserSearchRows,
  type BrowserSearchRow,
  type BrowserTabRow,
} from "../src/plugins/browser/mode.ts";
import { filterClipboardEntries, type ClipboardEntry } from "../src/clipboard-history.ts";
import {
  DEFAULT_BROWSER_SEARCH_FIELD,
  normalizeBrowserSearchField,
} from "../src/browser-page.ts";
import { browserConfigSchema, configDefaults, configField } from "../src/plugins/config-schema.ts";

const root = new URL("../", import.meta.url);
const read = (path: string) => readFile(new URL(path, root), "utf8");
const stripJsComments = (src: string) =>
  src.replace(/\/\*[\s\S]*?\*\//g, "").replace(/\/\/[^\n]*/g, "");
const en = createTranslator("en");

// ── A · the shared AND rule ───────────────────────────────────────────────

test("a needle is split into lowercased AND tokens", () => {
  assert.deepEqual(searchTokens(""), []);
  assert.deepEqual(searchTokens("   "), []);
  assert.deepEqual(searchTokens("Rust"), ["rust"]);
  assert.deepEqual(searchTokens("  Rust   Async  "), ["rust", "async"]);
});

test("every token must be found in one of the fields", () => {
  assert.equal(matchesTokens([], ["anything"]), true, "an empty query matches everything");
  assert.equal(matchesTokens(["rust"], ["Async Rust"]), true);
  assert.equal(matchesTokens(["async", "rust"], ["Async Rust"]), true);
  assert.equal(matchesTokens(["async", "python"], ["Async Rust"]), false);
  // A token may land in a different field than its neighbour.
  assert.equal(matchesTokens(["async", "rust-lang"], ["Async Rust", "https://rust-lang.org"]), true);
  // A missing field cannot satisfy a token.
  assert.equal(matchesTokens(["rust"], [null, undefined, ""]), false);
});

// ── B · the browser list filters through the same rule ────────────────────

const bookmark = (id: string, title: string, url: string): BrowserSearchRow => ({
  id,
  title,
  url,
  profile_key: "chrome/Default",
});

const rowsFor = (options: {
  needle?: string;
  searchField?: "all" | "title" | "url";
  /** R75 · history is the backend's filtered answer now, so a test passes
   *  whatever the backend would have returned for the needle — the memory rule
   *  is not applied to it. */
  history?: BrowserSearchRow[];
}) =>
  browserSearchRows({
    bookmarks: [
      bookmark("b1", "Async Rust", "https://rust-lang.example/one"),
      bookmark("b2", "Python Docs", "https://beta.example/two"),
    ],
    history: options.history ?? [],
    tabs: [
      {
        browser_id: "chrome",
        window_index: 0,
        tab_index: 0,
        title: "Async in depth",
        url: "https://delta.example/four",
        active: true,
      } satisfies BrowserTabRow,
    ],
    profileKey: "chrome/Default",
    t: en,
    needle: options.needle,
    searchField: options.searchField,
  });

test("a multi-word browser query is an AND, not a literal phrase", () => {
  const rows = rowsFor({ needle: "rust async" });
  // Only b1's title carries both tokens; no URL carries either and the tab
  // carries "async" alone. History is not in this list: it is the backend's
  // own filtered answer (pinned below), so the memory AND does not touch it.
  assert.deepEqual(rows.map((row) => row.id), ["b1"]);
});

test("history is the backend's answer and is not filtered again in memory", () => {
  // R75 · the backend applies the token AND *before* its SQL `LIMIT`, which is
  // what makes a match older than the fetch window reachable. Re-applying the
  // rule here would be a second, silently different ranking — and it could only
  // ever drop rows the backend already approved.
  const history = [bookmark("h1", "Rust Book", "https://gamma.example/three")];
  assert.deepEqual(
    rowsFor({ needle: "python docs", history }).map((row) => row.id),
    ["b2", "h1"],
    "h1 survives although the memory rule would drop it; b2 is the memory-filtered bookmark",
  );
  // …while the bookmark source is still filtered in memory, unchanged.
  assert.deepEqual(rowsFor({ needle: "python docs" }).map((row) => row.id), ["b2"]);
});

test("the search field narrows where a token may land", () => {
  // `rust-lang` is in the URL only; a title search must not find it.
  assert.deepEqual(
    rowsFor({ needle: "rust-lang", searchField: "url" }).map((row) => row.id),
    ["b1"],
  );
  assert.equal(
    rowsFor({ needle: "rust-lang", searchField: "title" }).some(
      (row) => row.family === "browser" && row.kind !== "status",
    ),
    false,
    "a URL-only token is invisible to a title search",
  );
  // `Python Docs` is a title; a URL search must not find it.
  assert.deepEqual(
    rowsFor({ needle: "python docs", searchField: "title" }).map((row) => row.id),
    ["b2"],
  );
  assert.equal(
    rowsFor({ needle: "python docs", searchField: "url" }).some(
      (row) => row.family === "browser" && row.kind !== "status",
    ),
    false,
  );
});

test("a live tab obeys the same rule as a bookmark", () => {
  const rows = rowsFor({ needle: "async depth", searchField: "title" });
  assert.deepEqual(rows.map((row) => row.id), ["tab:chrome:0:0"]);
});

test("browserRowMatches is the memory rule for bookmarks and live tabs", () => {
  const row = { title: "Async Rust", url: "https://rust-lang.org" };
  assert.equal(browserRowMatches(row, ["async", "rust"], "all"), true);
  assert.equal(browserRowMatches(row, ["async", "rust"], "title"), true);
  assert.equal(browserRowMatches(row, ["rust-lang"], "title"), false);
  assert.equal(browserRowMatches(row, ["rust-lang"], "url"), true);
  assert.equal(browserRowMatches(row, [], "title"), true);
});

// ── C · the clipboard list takes the same AND rule ────────────────────────

const entry = (id: string, text: string): ClipboardEntry => ({
  id,
  kind: "text",
  text,
  hash: id,
  favorite: false,
  created_at: 0,
});

test("the clipboard filter is an AND over the entry's own fields", () => {
  const entries = [entry("a", "hello world"), entry("b", "hello there"), entry("c", "world peace")];
  assert.deepEqual(filterClipboardEntries(entries, "hello world").map((e) => e.id), ["a"]);
  assert.deepEqual(filterClipboardEntries(entries, "hello").map((e) => e.id), ["a", "b"]);
  assert.deepEqual(filterClipboardEntries(entries, "HELLO WORLD").map((e) => e.id), ["a"]);
  assert.equal(filterClipboardEntries(entries, "hello mars").length, 0);
  // An empty (or whitespace) query is no filter.
  assert.equal(filterClipboardEntries(entries, "   ").length, 3);
});

// ── D · the four-value range filter and its Tab cycle ─────────────────────

test("the browser filter cycles all → bookmarks → history → tabs and wraps", () => {
  assert.deepEqual(BROWSER_FILTERS, ["all", "bookmarks", "history", "tabs"]);
  assert.equal(cycleBrowserFilter("all", 1), "bookmarks");
  assert.equal(cycleBrowserFilter("bookmarks", 1), "history");
  assert.equal(cycleBrowserFilter("history", 1), "tabs");
  assert.equal(cycleBrowserFilter("tabs", 1), "all", "Tab wraps forward");
  assert.equal(cycleBrowserFilter("all", -1), "tabs", "Shift+Tab wraps back");
  assert.equal(cycleBrowserFilter("history", -1), "bookmarks");
});

test("the chips row is chrome: it is charged by the band and is not a result row", () => {
  assert.equal(
    launcherRowUnits(3, true, true) - launcherRowUnits(3, true, false),
    LAUNCHER_FILTER_UNITS,
    "the filter adds a fixed amount of band height, whatever the row count",
  );
});

// ── E · an empty word's Backspace leaves the plugin ───────────────────────

test("only an already-empty plugin field exits on Backspace", () => {
  assert.equal(pluginModeExitOnBackspace(null, ""), false, "no mode, nothing to leave");
  assert.equal(pluginModeExitOnBackspace({ scope: "browser", kind: "all" }, "x"), false);
  assert.equal(pluginModeExitOnBackspace({ scope: "browser", kind: "all" }, ""), true);
  assert.equal(pluginModeExitOnBackspace({ scope: "clipboard", filter: "all" }, ""), true);
});

// ── F · the search-field configuration ────────────────────────────────────

test("the browser schema declares the search-field radio, default all", () => {
  const field = configField(browserConfigSchema(), "search_fields");
  assert.ok(field && field.type === "radio", "the field is a radio");
  assert.deepEqual(field.options.map((option) => option.value), ["all", "title", "url"]);
  assert.equal(configDefaults(browserConfigSchema()).search_fields, "all");
  assert.equal(DEFAULT_BROWSER_SEARCH_FIELD, "all");
  assert.equal(normalizeBrowserSearchField("title"), "title");
  assert.equal(normalizeBrowserSearchField("URL"), "url");
  assert.equal(normalizeBrowserSearchField("body"), "all");
});

// ── G · the wiring, pinned at the source ──────────────────────────────────

test("the browser scope draws the chips and never the recent heading", async () => {
  const app = stripJsComments(await read("src/App.tsx"));
  assert.match(app, /launcherScope === "browser" && \(/, "the chips render only in the browser scope");
  assert.match(app, /BROWSER_FILTERS\.map\(\(kind\)/, "one chip per range filter");
  assert.match(app, /setBrowserFilter\(kind\)/, "a click sets the filter");
  assert.match(
    app,
    /const launcherSectionTitle =\s*launcherPanelOpen &&\s*!launcherScope &&\s*!query\.trim\(\) &&\s*!fileRows\.length &&\s*settings\.show_recent_in_launcher;/,
    "the empty-query heading is the ordinary search page's, never a plugin's — and it goes with the recents it labels",
  );
  // The other ordinary-search-only rows are gated on the scope too. (The
  // empty-page integrations hint that used to sit beside them is gone — the user
  // asked for it to go: 「不要那个欢迎使用的提示」.)
  // R73 · the first-run tip is gone entirely (the user: 「搜索框下方的 欢迎使用的提醒
  // 去掉，它在一些场景下影响到了布局」), so the scan-error row is the only remaining
  // ordinary-search-only one to pin here.
  assert.match(app, /\{appsError && !launcherScope && \(/);
});

test("Tab cycles the filter in the browser scope, in both directions", async () => {
  const actions = stripJsComments(await read("src/hooks/useLauncherActions.ts"));
  assert.match(actions, /if \(event\.key === "Tab" && browserScope\) \{/);
  assert.match(actions, /cycleBrowserFilter\(event\.shiftKey \? -1 : 1\);/);
  const app = stripJsComments(await read("src/App.tsx"));
  assert.match(app, /browserScope: launcherScope === "browser",/);
});

test("an empty field's Backspace is resolved on both keyboard paths", async () => {
  const app = stripJsComments(await read("src/App.tsx"));
  assert.match(app, /const onPluginModeBackspace = useCallback\(/);
  assert.match(app, /pluginModeExitOnBackspace\(pluginModeRef\.current, query\)/);
  assert.match(
    app,
    /if \(onLauncherDismiss\(event\.nativeEvent\)\) return;[\s\S]{0,200}?if \(onPluginModeBackspace\(event\.nativeEvent\)\) return;/,
    "the input asks the backspace rule after Esc / Cmd+W",
  );
  const keyboard = stripJsComments(await read("src/hooks/useAppKeyboard.ts"));
  assert.match(
    keyboard,
    /if \(onPluginModeBackspace\(event\)\) return;[\s\S]{0,120}?if \(event\.key === "Backspace"\)/,
    "the window fallback asks the rule before its own backspace edit",
  );
});

test("the launcher reads the search field from settings, in memory and in SQL", async () => {
  const app = stripJsComments(await read("src/App.tsx"));
  assert.match(app, /browserSearchField: settings\.browser_plugin\.search_fields,/);
  assert.match(app, /onBrowserSettingsChange=\{\(block\) => changeGeneralSetting\("browser_plugin", block\)\}/);
  const catalog = stripJsComments(await read("src/hooks/useLauncherCatalog.ts"));
  // R75 · the history read carries the needle's tokens and the configured
  // field into the backend, keyed on the needle (not just the range filter).
  assert.match(catalog, /invoke<BrowserSearchRow\[\]>\("browser_search_history", \{/);
  assert.match(catalog, /tokens: searchTokens\(browserNeedle\),/);
  assert.match(catalog, /searchField: browserSearchField,/);
  assert.match(catalog, /const browserNeedle = browserMode\?\.needle \?\? "";/);
  assert.match(catalog, /browserNeedle, browserSearchField, t\]/);
  // …debounced on the catalog's own window, not a new mechanism.
  assert.match(
    catalog,
    /searchTokens\(browserNeedle\),[\s\S]{0,500}?\}, CATALOG_SEARCH_DELAY\)/,
    "the history read reuses the catalog debounce",
  );
  // …while the bookmark read still fetches once with an empty query and is
  // filtered in memory by the memo.
  assert.match(catalog, /invoke<BrowserSearchRow\[\]>\("browser_search_bookmarks", \{/);
  assert.match(catalog, /query: "",/);
});

test("the new R32 copy exists in both languages", async () => {
  const source = await read("src/i18n.ts");
  for (const key of [
    "launcher.browserAll",
    "launcher.browserTabs",
    "launcher.browserFilter",
    "plugins.config.searchFields",
    "plugins.config.searchFieldsHint",
    "plugins.config.searchFieldsAll",
    "plugins.config.searchFieldsTitle",
    "plugins.config.searchFieldsUrl",
  ]) {
    const occurrences = source.split(`"${key}"`).length - 1;
    assert.equal(occurrences, 2, `${key} must be declared in both dictionaries`);
  }
});
