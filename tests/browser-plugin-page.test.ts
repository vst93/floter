// R26-B · the browser plugin's registry entry and the pure logic behind it.
//
// Two halves, one file:
//
// * the *registry* contract — `plugin_pages.rs` registers `builtin.browser`
//   with the same id the frontend names. R33 retired the built-in pages and
//   R96 deleted the descriptor's page slot and command allowlist, so the
//   registry row is identity plus the two i18n keys the settings panel renders.
// * the *logic* contract — `src/browser-page.ts`'s normalizers, which is where
//   every decision the launcher mode makes actually lives.

import assert from "node:assert/strict";
import { readFile, stat } from "node:fs/promises";
import test from "node:test";
import { createTranslator } from "../src/i18n.ts";
import {
  DEFAULT_CDP_PORT,
  browserTargets,
  clampHistoryDays,
  isKnownTarget,
  isMacUserAgent,
  normalizeBrowserSettings,
  normalizeBrowserSearchField,
  normalizeBrowserSortOrder,
  normalizeCdpPort,
  normalizeProfiles,
  normalizeSearchRows,
  normalizeTabs,
  openRowArgs,
  pickProfileKey,
  tabActivationArgs,
  tabRowKey,
  tabSubtitle,
  tabWindowCount,
} from "../src/browser-page.ts";

const root = new URL("../", import.meta.url);
const read = (path: string) => readFile(new URL(path, root), "utf8");
const exists = async (path: string) => {
  try {
    await stat(new URL(path, root));
    return true;
  } catch {
    return false;
  }
};

const PAGE_HTML = "plugins/browser/page.html";
const PAGE_MAIN = "src/plugins/browser/main.ts";
const PAGE_CSS = "src/plugins/browser/page.css";

// ── 1 · the registry ───────────────────────────────────────────────────────

test("the backend registers the browser descriptor with the shared id", async () => {
  const rust = await read("src-tauri/src/plugin_pages.rs");
  assert.match(
    rust,
    /pub const BROWSER_PLUGIN_ID: &str = "builtin\.browser";/,
    "the browser plugin id is a literal constant",
  );
  const descriptorsAt = rust.indexOf("static DESCRIPTORS");
  assert.notEqual(descriptorsAt, -1, "the descriptor registry must exist");
  assert.match(
    rust.slice(descriptorsAt),
    /id: BROWSER_PLUGIN_ID,/,
    "the registry must list the browser descriptor",
  );
});

test("the frontend's plugin id mirrors the backend's literal", async () => {
  const rust = await read("src-tauri/src/plugin_pages.rs");
  const frontend = await read("src/builtin-plugins.ts");
  const id = rust.match(/pub const BROWSER_PLUGIN_ID: &str = "([^"]+)";/);
  assert.ok(id);
  assert.ok(
    frontend.includes(`export const BROWSER_PLUGIN_ID = "${id![1]}";`),
    "the two plugin ids must be the same string",
  );
});

// ── 2 · the page (retired and deleted) ────────────────────────────────────
test("the browser page document and source are deleted; the logic module is retained", async () => {
  // R33 · the iframe document and its Vite entry are gone, so nothing could
  // load the built-in page. R76 · the page's own source is gone too; the
  // decisions it used to make live in `src/browser-page.ts`, which is retained
  // and covered by the pure-logic half of this file.
  assert.equal(await exists(PAGE_HTML), false, "the retired document must be gone");
  assert.equal(await exists(PAGE_MAIN), false, "the retired page source must stay deleted");
  assert.equal(await exists(PAGE_CSS), false, "the retired page stylesheet must stay deleted");
  assert.ok(await exists("src/browser-page.ts"), "the retained logic module must stay");
});

test("the build carries no iframe page entry point", async () => {
  const config = await read("vite.config.ts");
  assert.ok(
    !/plugins\/(browser|clipboard)/.test(config),
    "R33 · the retired pages must not be Vite inputs any more",
  );
  assert.ok(
    !config.includes("rollupOptions"),
    "there is one entry point again: the app document",
  );
});

// ── 3 · the pure logic ─────────────────────────────────────────────────────

const profile = (over: Record<string, unknown> = {}) => ({
  browser_id: "chrome",
  browser_name: "Google Chrome",
  profile_key: "chrome/Default",
  profile_dir_name: "Default",
  profile_name: "Default",
  base_dir: "/Users/x/Library/Application Support/Google/Chrome",
  has_bookmarks: true,
  has_history: true,
  ...over,
});

test("profiles and rows are normalized, and malformed entries are dropped", () => {
  const profiles = normalizeProfiles([
    profile(),
    { browser_id: "chrome" }, // no profile_key: unsearchable
    null,
    profile({ browser_id: "edge", browser_name: "Microsoft Edge", profile_key: "edge/Profile 1" }),
  ]);
  assert.equal(profiles.length, 2);
  assert.equal(profiles[1].browser_name, "Microsoft Edge");

  const rows = normalizeSearchRows([
    { id: "a", title: "Rust", url: "https://rust-lang.org/", profile_key: "chrome/Default" },
    { id: "b", title: "no url", url: "" }, // not a result
    { id: "c", title: "", url: "https://example.com/" }, // title falls back to URL
  ]);
  assert.equal(rows.length, 2);
  assert.equal(rows[1].title, "https://example.com/");
});

test("tabs survive without a window concept and keep their identity", () => {
  const tabs = normalizeTabs([
    { browser_id: "chrome", window_index: 0, tab_index: 0, title: "A", url: "https://a/", active: true },
    { browser_id: "chrome", window_index: 0, tab_index: 1, title: "B", url: "https://b/", active: false },
    { browser_id: "chrome", window_index: 0, tab_index: 2, title: "", url: "" },
  ]);
  assert.equal(tabs.length, 2);
  assert.equal(tabRowKey(tabs[0]), "tab:chrome:0:0");
  assert.notEqual(tabRowKey(tabs[0]), tabRowKey(tabs[1]));
  assert.equal(tabWindowCount(tabs), 1);
  // One window: the window label would be noise.
  assert.equal(tabSubtitle(tabs[0], 1), "●  https://a/");
  assert.equal(tabSubtitle(tabs[1], 1), "https://b/");
  // Two windows: the label is information.
  assert.match(tabSubtitle({ ...tabs[0], window_index: 2 }, 2), /^W2/);
});

test("the search profile follows the target, then the data", () => {
  const profiles = normalizeProfiles([
    profile({ browser_id: "brave", profile_key: "brave/Default", has_history: false, has_bookmarks: false }),
    profile({ browser_id: "chrome", profile_key: "chrome/Default", has_history: true }),
  ]);
  assert.equal(pickProfileKey(profiles, "auto"), "chrome/Default");
  assert.equal(pickProfileKey(profiles, "brave"), "brave/Default");
  // A target that is not installed falls back to auto rather than to nothing.
  assert.equal(pickProfileKey(profiles, "edge"), "chrome/Default");
  assert.equal(pickProfileKey([], "auto"), null);
});

test("the target dropdown lists each browser once and validates against it", () => {
  const profiles = normalizeProfiles([
    profile(),
    profile({ profile_key: "chrome/Profile 1" }),
    profile({ browser_id: "edge", browser_name: "Microsoft Edge", profile_key: "edge/Default" }),
  ]);
  assert.deepEqual(browserTargets(profiles), [
    { id: "chrome", name: "Google Chrome" },
    { id: "edge", name: "Microsoft Edge" },
  ]);
  assert.ok(isKnownTarget(profiles, "auto"));
  assert.ok(isKnownTarget(profiles, "edge"));
  assert.ok(!isKnownTarget(profiles, "firefox"));
});

test("settings normalize to what the backend will store", () => {
  assert.deepEqual(normalizeBrowserSettings(undefined), {
    enabled: true,
    target: "auto",
    custom_base_dir: null,
    history_days: 30,
    cdp_enabled: false,
    cdp_port: DEFAULT_CDP_PORT,
    sort_order: "relevance",
    search_fields: "all",
  });
  const settings = normalizeBrowserSettings({
    target: "brave",
    custom_base_dir: "  /data/brave  ",
    history_days: 99999,
    cdp_enabled: true,
    cdp_port: 9333,
  });
  assert.equal(settings.target, "brave");
  assert.equal(settings.custom_base_dir, "/data/brave");
  assert.equal(settings.history_days, 3650);
  assert.equal(settings.cdp_enabled, true);
  assert.equal(settings.cdp_port, 9333);
  // R27: the sort order defaults to the launcher's own ranking and normalizes
  // every other spelling to it, so a hand-edited file cannot leave the list
  // unsorted.
  assert.equal(normalizeBrowserSettings({}).sort_order, "relevance");
  assert.equal(normalizeBrowserSettings({ sort_order: "recent" }).sort_order, "recent");
  assert.equal(normalizeBrowserSettings({ sort_order: "ALPHABETICAL" }).sort_order, "alphabetical");
  assert.equal(normalizeBrowserSettings({ sort_order: "visits" }).sort_order, "visits");
  assert.equal(normalizeBrowserSettings({ sort_order: "sideways" }).sort_order, "relevance");
  assert.equal(normalizeBrowserSortOrder(undefined), "relevance");
  // R32: the search-field setting defaults to `all` (title or URL) and every
  // other spelling normalizes to it, so a hand-edited file cannot make the
  // search match nothing.
  assert.equal(normalizeBrowserSettings({}).search_fields, "all");
  assert.equal(normalizeBrowserSettings({ search_fields: "title" }).search_fields, "title");
  assert.equal(normalizeBrowserSettings({ search_fields: "URL" }).search_fields, "url");
  assert.equal(normalizeBrowserSettings({ search_fields: "body" }).search_fields, "all");
  assert.equal(normalizeBrowserSearchField(undefined), "all");
  // A blank directory is none, not an empty string.
  assert.equal(normalizeBrowserSettings({ custom_base_dir: "   " }).custom_base_dir, null);
  // R26-D: the plugin's switch defaults on and survives an explicit off.
  assert.equal(normalizeBrowserSettings({}).enabled, true);
  assert.equal(normalizeBrowserSettings({ enabled: false }).enabled, false);
  // 0 is a meaningful history window (it disables the filter) and survives.
  assert.equal(clampHistoryDays(0), 0);
  assert.equal(clampHistoryDays(-5), 0);
  assert.equal(clampHistoryDays(Number.NaN), 30);
});

test("a dead debug port falls back to the browser's own default", () => {
  assert.equal(normalizeCdpPort(9333), 9333);
  assert.equal(normalizeCdpPort(0), DEFAULT_CDP_PORT);
  assert.equal(normalizeCdpPort(-1), DEFAULT_CDP_PORT);
  assert.equal(normalizeCdpPort(70000), DEFAULT_CDP_PORT);
  assert.equal(normalizeCdpPort("9222"), DEFAULT_CDP_PORT);
  assert.equal(normalizeCdpPort(undefined), DEFAULT_CDP_PORT);
});

test("the debug-port block is shown only off the Mac", () => {
  assert.equal(isMacUserAgent("Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)"), true);
  assert.equal(isMacUserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64)"), false);
  assert.equal(isMacUserAgent("Mozilla/5.0 (X11; Linux x86_64)"), false);
});

test("row actions send exactly the arguments the commands declare", () => {
  assert.deepEqual(
    openRowArgs({ id: "x", title: "t", url: "https://a/", profile_key: "chrome/Default" }),
    { profileKey: "chrome/Default", url: "https://a/" },
  );
  assert.deepEqual(
    tabActivationArgs({
      browser_id: "chrome",
      window_index: 1,
      tab_index: 2,
      title: "t",
      url: "https://a/",
      active: false,
    }),
    { browserId: "chrome", windowIndex: 1, tabIndex: 2, url: "https://a/" },
  );
});

test("the launcher's tab rows carry the tab identity, not just the URL", async () => {
  // R28 · the rows are the plugin's output now (`plugins/browser/mode.ts`) and
  // the hook only hands the three sources over; the tab identity still rides
  // the row, which is what lets Enter switch to a tab instead of reopening it.
  const catalog = await read("src/hooks/useLauncherCatalog.ts");
  const mode = await read("src/plugins/browser/mode.ts");
  assert.match(catalog, /invoke<BrowserTabRow\[\]>\("browser_list_tabs"/);
  assert.match(mode, /tab: \{\s*browserId: tab\.browser_id,/);
  const actions = await read("src/hooks/useLauncherActions.ts");
  assert.match(actions, /await invoke\("browser_activate_tab", \{/);
  assert.match(actions, /if \(item\.tab\) \{/);
});

test("a failed tab read is absorbed, and the guidance lives on the settings field", async () => {
  const catalog = await read("src/hooks/useLauncherCatalog.ts");
  const mode = await read("src/plugins/browser/mode.ts");
  const schema = await read("src/plugins/config-schema.ts");
  // R31 · the failure is still a value the tab read absorbs — a rejection that
  // could take the bookmark and history fetches with it would be a regression —
  // but it no longer writes a note into the list. The user's verdict was that
  // the list is not the place for setup instructions; the note was removed and
  // the guidance moved to the help text of the field that fixes it.
  assert.match(catalog, /\(\) => \[\] as BrowserTabRow\[\]/);
  assert.doesNotMatch(mode, /browser-tabs-unavailable/);
  assert.doesNotMatch(mode, /tabsFailed/);
  // R71 · “absorbs its own failure” was only half of R26-B's rule; the other
  // half is that it must not *hold up* the two file lists either. The hook used
  // to publish all three sources from one `Promise.all`, so a slow AppleScript
  // read (or a cold browser) delayed every bookmark and history row — the
  // plugin list that appeared seconds late, or not at all. The file reads are
  // published on their own now and the tab group merges in behind them.
  assert.match(
    catalog,
    /setBrowserFetch\(\{ ok: true, profileKey, bookmarks, tabs: \[\] \}\)/,
    "the bookmark list is published without waiting for the tab read (R75 · history is its own needle-keyed read)",
  );
  assert.match(
    catalog,
    /current && current\.ok \? \{ \.\.\.current, tabs \} : current/,
    "the tab group fills itself in when it lands",
  );
  assert.doesNotMatch(
    catalog,
    /const \[bookmarks, history, tabs\] = await Promise\.all/,
    "the three sources are no longer one all-or-nothing await",
  );
  // …the field exists in the schema and its help carries the two mechanisms.
  assert.match(schema, /key: "cdp_enabled", type: "toggle", labelKey: "plugins\.config\.cdpEnabled", helpKey: "plugins\.config\.cdpEnabledHint"/);
  // …and the help is translated, in both languages, with the macOS path named.
  const en = createTranslator("en")("plugins.config.cdpEnabledHint");
  const zh = createTranslator("zh")("plugins.config.cdpEnabledHint");
  assert.match(en, /debug port/i);
  assert.match(en, /macOS/i);
  assert.match(en, /AppleScript/);
  assert.match(zh, /[\u4e00-\u9fff]/);
  assert.match(zh, /macOS/);
  assert.match(zh, /AppleScript/);
});
