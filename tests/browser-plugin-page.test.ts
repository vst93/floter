// R26-B · the browser plugin's page, its registry entry, and the pure logic
// behind it.
//
// Three halves, one file:
//
// * the *registry* contract — `plugin_pages.rs` registers `builtin.browser`
//   with a page that exists on disk and an allowlist whose every command the
//   backend actually exposes. This is the failure mode that breaks the plugin:
//   a descriptor naming a command nobody registered, or a page that 404s
//   inside the sandbox.
// * the *page* contract — the document exists, loads its entry point, goes
//   through the bridge and never touches a Tauri API (the plugin-page red
//   line).
// * the *logic* contract — `src/browser-page.ts`'s normalizers, which is where
//   every decision the page makes actually lives.

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

/** The commands the descriptor must grant, exactly the task's list. */
const REQUIRED_COMMANDS = [
  "browser_discover",
  "browser_search_bookmarks",
  "browser_search_history",
  "browser_list_tabs",
  "browser_activate_tab",
  "browser_open_url",
  "browser_get_settings",
  "browser_set_settings",
];

// ── 1 · the registry ───────────────────────────────────────────────────────

test("the backend registers the browser page with exactly its allowlist", async () => {
  const rust = await read("src-tauri/src/plugin_pages.rs");
  assert.match(
    rust,
    /pub const BROWSER_PLUGIN_ID: &str = "builtin\.browser";/,
    "the browser plugin id is a literal constant",
  );
  assert.match(rust, /id: BROWSER_PLUGIN_ID,/);
  // R33 · the built-in page path is retired; the allowlist stays (the overlay
  // and the launcher mode invoke these commands directly).
  assert.match(rust, /page: "",/, "no built-in page path may be registered");

  // The allowlist literal, read from the file rather than re-derived, so a
  // command added in one place and not the other fails here.
  const block = rust.match(/const BROWSER_COMMANDS: &\[&str\] = &\[([\s\S]*?)\];/);
  assert.ok(block, "BROWSER_COMMANDS must be declared");
  const allowed = [...block![1].matchAll(/"([^"]+)"/g)].map((match) => match[1]);
  assert.deepEqual(allowed.sort(), [...REQUIRED_COMMANDS].sort());
});

test("every allowlisted browser command is registered with the backend", async () => {
  const rust = await read("src-tauri/src/plugin_pages.rs");
  const lib = await read("src-tauri/src/lib.rs");
  const block = rust.match(/const BROWSER_COMMANDS: &\[&str\] = &\[([\s\S]*?)\];/);
  const allowed = [...block![1].matchAll(/"([^"]+)"/g)].map((match) => match[1]);
  // The invoke handler lists them by their Rust paths; a command the handler
  // never sees would be an "unknown command" at runtime.
  for (const command of allowed) {
    assert.ok(
      lib.includes(`::${command},`),
      `${command} is allowlisted but never registered in lib.rs`,
    );
  }
  // The two R26-B commands are the tab pair, and they live in the tabs module.
  assert.match(lib, /browser_data::tabs::browser_list_tabs,/);
  assert.match(lib, /browser_data::tabs::browser_activate_tab,/);
  assert.match(lib, /commands::config::browser_get_settings,/);
  assert.match(lib, /commands::config::browser_set_settings,/);
});

test("the two plugin pages never share a command", async () => {
  const rust = await read("src-tauri/src/plugin_pages.rs");
  const commandsOf = (name: string) => {
    const block = rust.match(new RegExp(`const ${name}: &\\[&str\\] = &\\[([\\s\\S]*?)\\];`));
    assert.ok(block, `${name} must be declared`);
    return [...block![1].matchAll(/"([^"]+)"/g)].map((match) => match[1]);
  };
  const clipboard = new Set(commandsOf("CLIPBOARD_COMMANDS"));
  for (const command of commandsOf("BROWSER_COMMANDS")) {
    assert.ok(!clipboard.has(command), `${command} is granted to both pages`);
  }
});

test("no built-in page path is registered any more", async () => {
  const rust = await read("src-tauri/src/plugin_pages.rs");
  const pages = [...rust.matchAll(/page: "([^"]*)"/g)].map((match) => match[1]);
  assert.ok(pages.length >= 2, "both descriptors must still exist");
  for (const page of pages) {
    assert.equal(page, "", "R33 · every built-in page slot must be empty");
  }
  // And the documents the slots used to name are gone from the tree.
  assert.equal(await exists("plugins/browser/page.html"), false);
  assert.equal(await exists("plugins/clipboard/index.html"), false);
});

test("the frontend's plugin id mirrors the backend's literal", async () => {
  const rust = await read("src-tauri/src/plugin_pages.rs");
  const frontend = await read("src/plugin-pages.ts");
  const id = rust.match(/pub const BROWSER_PLUGIN_ID: &str = "([^"]+)";/);
  assert.ok(id);
  assert.ok(
    frontend.includes(`export const BROWSER_PLUGIN_ID = "${id![1]}";`),
    "the two plugin ids must be the same string",
  );
});

// ── 2 · the page ───────────────────────────────────────────────────────────

test("the browser page document is retired; its logic module is retained", async () => {
  // R33 · the iframe document and its Vite entry are gone, so nothing can load
  // the built-in page. The page's own source stays in the tree because the
  // published bridge protocol (and its tests) still exercise it; a future
  // external page loader would mount the same host.
  assert.equal(await exists(PAGE_HTML), false, "the retired document must be gone");
  assert.ok(await exists(PAGE_MAIN), "the retained page source must stay");
  assert.ok(await exists(PAGE_CSS), "the retained page stylesheet must stay");
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

test("the page goes through the bridge and touches no Tauri API", async () => {
  const main = await read(PAGE_MAIN);
  assert.match(main, /import "\.\/page\.css";/);
  assert.match(main, /PLUGIN_PAGE_PROTOCOL/, "the handshake must name the protocol");
  assert.match(main, /\{ \[BRIDGE_TAG\]: "frame-ready", protocol: PLUGIN_PAGE_PROTOCOL \}/);
  assert.match(main, /\{ \[BRIDGE_TAG\]: "invoke"/);
  // The plugin-page red line: a sandboxed page has no Tauri surface, and a page
  // that reached for one would be a capability the allowlist never granted.
  assert.ok(
    !main.includes("@tauri-apps"),
    "the browser page must not import a Tauri API",
  );
  // Every command the page runs is one of the allowlisted eight.
  const invoked = [...main.matchAll(/invokeCommand<[^>]*>\(\s*"([^"]+)"/g)].map((m) => m[1]);
  assert.ok(invoked.length >= 5, `expected the page to invoke commands, saw ${invoked.length}`);
  for (const command of invoked) {
    assert.ok(
      REQUIRED_COMMANDS.includes(command),
      `${command} is invoked by the page but not allowlisted`,
    );
  }
  // The settings card is on the page, and it writes through the narrow pair.
  assert.match(main, /"browser_get_settings"/);
  assert.match(main, /"browser_set_settings"/);
});

test("the page renders the three groups and the settings card", async () => {
  const main = await read(PAGE_MAIN);
  assert.match(main, /t\("launcher\.browserBookmarks"\)/);
  assert.match(main, /t\("launcher\.browserHistory"\)/);
  assert.match(main, /t\("browserPage\.tabs"\)/);
  assert.match(main, /t\("browserPage\.settings"\)/);
  // The debug-port block is Windows/Linux only; macOS reads tabs natively.
  assert.match(main, /if \(!isMac\)/);
  assert.match(main, /t\("browserPage\.cdp"\)/);
  // No native directory picker: the iframe has no Tauri dialog, so the custom
  // directory is a text path plus a reset button.
  assert.match(main, /t\("browserPage\.useDefaultDir"\)/);
});

test("every string the page asks for exists in both dictionaries", async () => {
  const main = await read(PAGE_MAIN);
  const en = createTranslator("en");
  const zh = createTranslator("zh");
  // `(?<![\w.$])` keeps `params.get("lang")` out of the key set — the only
  // `t("…")` shapes that count are calls to the page's own translator.
  const keys = [...main.matchAll(/(?<![\w.$])t\("([^"]+)"\)/g)].map((match) => match[1]);
  assert.ok(keys.length > 0);
  for (const key of new Set(keys)) {
    const english = en(key as never);
    const chinese = zh(key as never);
    assert.notEqual(english, key, `${key} is missing from the English dictionary`);
    assert.notEqual(chinese, key, `${key} is missing from the Chinese dictionary`);
    assert.ok(english.length > 0 && chinese.length > 0, `${key} must not be blank`);
  }
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
