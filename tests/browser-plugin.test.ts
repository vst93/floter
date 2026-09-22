// R26-A · the browser plugin's launcher surface and its data layer's shape.
//
// Two halves, one file:
//
// * the *frontend* contract — the trigger vocabulary that enters the browser
//   result mode (`parseBrowserMode`), the action bar's claim on the bare
//   trigger words, and the catalog row that advertises the plugin. These are
//   pure functions and source assertions, so they run without a DOM.
// * the *Rust* contract — the settings block, the five commands and the
//   three-platform discovery table. The node suite cannot execute Rust, but it
//   can pin the names the frontend invokes against the names the backend
//   registers, which is the failure mode that actually breaks the plugin.

import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";
import { classifyActionBar, parseBrowserMode } from "../src/launcher.ts";
import { createTranslator } from "../src/i18n.ts";

const root = new URL("../", import.meta.url);
const read = (path: string) => readFile(new URL(path, root), "utf8");

const en = createTranslator("en");
const zh = createTranslator("zh");

test("the browser mode needs the trigger word plus a space", () => {
  // Bare word: the action bar / system row, not the mode.
  assert.equal(parseBrowserMode("bookmarks"), null);
  assert.equal(parseBrowserMode("browser"), null);
  assert.equal(parseBrowserMode("history"), null);
  // Trigger + space: inside the mode, with an empty needle (the default view).
  assert.deepEqual(parseBrowserMode("bookmarks "), { kind: "bookmarks", needle: "" });
  assert.deepEqual(parseBrowserMode("browser "), { kind: "all", needle: "" });
  // Trigger + text: the needle.
  assert.deepEqual(parseBrowserMode("bookmarks rust"), { kind: "bookmarks", needle: "rust" });
  assert.deepEqual(parseBrowserMode("history  async  "), { kind: "history", needle: "async" });
});

test("every trigger word maps to its own source", () => {
  assert.deepEqual(parseBrowserMode("bookmark x"), { kind: "bookmarks", needle: "x" });
  assert.deepEqual(parseBrowserMode("书签 文档"), { kind: "bookmarks", needle: "文档" });
  assert.deepEqual(parseBrowserMode("浏览器 文档"), { kind: "all", needle: "文档" });
  assert.deepEqual(parseBrowserMode("hist x"), { kind: "history", needle: "x" });
  assert.deepEqual(parseBrowserMode("历史记录 x"), { kind: "history", needle: "x" });
});

test("a query that is not a trigger stays out of the mode", () => {
  assert.equal(parseBrowserMode(""), null);
  assert.equal(parseBrowserMode("chrome"), null);
  assert.equal(parseBrowserMode("bookmarklet rust"), null);
  // An ordinary command line with a space is not a browser query.
  assert.equal(parseBrowserMode("git commit"), null);
});

test("`history` stays the shell command until it has an argument", () => {
  // The one collision the vocabulary has to respect: `history` runs the shell
  // builtin, `history rust` searches the browser. A regression here would make
  // the shell command unreachable.
  assert.equal(parseBrowserMode("history"), null);
  assert.equal(classifyActionBar("history"), "shell");
  assert.deepEqual(parseBrowserMode("history rust"), { kind: "history", needle: "rust" });
});

test("the action bar claims the bare browser trigger words", () => {
  assert.equal(classifyActionBar("browser"), "browser");
  assert.equal(classifyActionBar("bookmarks"), "browser");
  assert.equal(classifyActionBar("bookmark"), "browser");
  assert.equal(classifyActionBar("浏览器"), "browser");
  assert.equal(classifyActionBar("书签"), "browser");
  // And the trigger + space is claimed by the mode, not the action bar — the
  // hook returns a null action bar while the mode is on.
  assert.equal(classifyActionBar("browser "), "browser");
});

test("the catalog declares one browser system row wired to the mode", async () => {
  const source = await read("src/hooks/useLauncherCatalog.ts");
  assert.match(source, /action: "browser"/, "the browser row is in SYSTEM_COMMANDS");
  assert.match(source, /titleKey: "system\.browserSearch"/);
  assert.match(source, /subtitleKey: "system\.browserSearchSubtitle"/);
  // The row advertises the plugin in both languages, like the power rows.
  assert.match(source, /"browser bookmarks"/);
  assert.match(source, /"浏览器书签"/);
  // Bare `history` must not be a search name: it would shadow the shell
  // command's own row.
  assert.doesNotMatch(source, /searchNames: \[[^\]]*"history"/);
});

test("entering the mode rewrites the query to the trigger the parser owns", async () => {
  const source = await read("src/hooks/useLauncherActions.ts");
  assert.match(source, /item\.action === "browser"[\s\S]{0,220}setQuery\("browser "\)/);
  // A browser row opens through the backend, not the generic URL opener.
  assert.match(source, /invoke\("browser_open_url", \{ profileKey, url \}\)/);
  assert.match(source, /showLauncherFeedback\("launcher\.error\.browser"\)/);
});

test("the two new i18n keys carry a translation in both languages", () => {
  for (const key of [
    "system.browserSearch",
    "system.browserSearchSubtitle",
    "launcher.browserEmpty",
    "launcher.browserNoProfile",
    "launcher.browserBookmarks",
    "launcher.browserHistory",
    "settings.browser",
    "settings.browserHint",
    "settings.browserTarget",
    "settings.browserTargetAuto",
    "settings.browserCustomDir",
    "settings.browserHistoryDays",
  ] as const) {
    assert.notEqual(en(key), key, `${key} is translated in en`);
    assert.notEqual(zh(key), key, `${key} is translated in zh`);
    assert.notEqual(zh(key), en(key), `${key} is actually translated, not copied`);
  }
});

test("the Rust settings block exists with the shipped defaults", async () => {
  const source = await read("src-tauri/src/commands/config.rs");
  assert.match(source, /pub struct BrowserPluginSettings/);
  assert.match(source, /pub browser_plugin: BrowserPluginSettings/);
  // Auto-detect, no custom directory, a 30-day window: the plugin has to be
  // usable without the user ever opening its settings page.
  assert.match(source, /target: "auto"\.to_string\(\)/);
  assert.match(source, /custom_base_dir: None/);
  assert.match(source, /history_days: 30/);
  // The field carries `#[serde(default)]`, so an older config keeps loading.
  assert.match(source, /#\[serde\(default\)\]\s*pub browser_plugin: BrowserPluginSettings/);
});

test("the five browser commands are registered with the names the frontend invokes", async () => {
  const lib = await read("src-tauri/src/lib.rs");
  const mod = await read("src-tauri/src/browser_data/mod.rs");
  for (const command of [
    "browser_discover",
    "browser_default_profile",
    "browser_search_bookmarks",
    "browser_search_history",
    "browser_open_url",
  ]) {
    assert.match(mod, new RegExp(`pub fn ${command}\\(`), `${command} is defined`);
    assert.match(lib, new RegExp(`browser_data::${command},`), `${command} is registered`);
  }
});

test("the discovery table names the shipped browsers on all three platforms", async () => {
  const source = await read("src-tauri/src/browser_data/discover.rs");
  for (const browser of ["chrome", "edge", "brave", "chromium"]) {
    assert.match(source, new RegExp(`"${browser}"`), `${browser} is discovered`);
  }
  // One `#[cfg]` branch per platform, each with the platform's own paths.
  assert.match(source, /#\[cfg\(target_os = "macos"\)\]/);
  assert.match(source, /#\[cfg\(target_os = "windows"\)\]/);
  assert.match(source, /#\[cfg\(target_os = "linux"\)\]/);
  assert.match(source, /Library"\)\.join\("Application Support"\)/);
  assert.match(source, /"User Data"/);
  assert.match(source, /\.join\("\.config"\)/);
});

test("the history reader copies before it opens, and never panics on a bad file", async () => {
  const source = await read("src-tauri/src/browser_data/history.rs");
  assert.match(source, /tempfile::tempdir\(\)/, "the copy goes to a temp directory");
  assert.match(source, /std::fs::copy\(source, destination\)/);
  // `-wal` / `-shm` are copied too, or the newest commits are missed.
  assert.match(source, /"-wal", "-shm"/);
  assert.match(source, /SQLITE_OPEN_READ_ONLY/);
  // No `unwrap`/`expect` on the read path: every failure is an error string.
  const readPath = source.slice(0, source.indexOf("#[cfg(test)]"));
  assert.doesNotMatch(readPath, /\.unwrap\(\)|\.expect\(/);
});

test("the bookmarks parser flattens all three roots and converts the epoch", async () => {
  const source = await read("src-tauri/src/browser_data/bookmarks.rs");
  assert.match(source, /get\("roots"\)/, "the roots object is read");
  assert.match(source, /"folder" =>/, "folders recurse");
  assert.match(source, /"url" =>/, "URLs are emitted");
  assert.match(source, /chromium_time_to_unix/, "date_added is converted");
});

test("the launcher item type gained exactly one browser variant", async () => {
  const source = await read("src/launcher/LauncherResults.tsx");
  assert.match(source, /type: "browser"/);
  assert.match(source, /disabled\?: boolean/);
  // The row is rendered with a globe, not the terminal fallback.
  assert.match(source, /item\.type === "browser" \? \(\s*<GlobeIcon/);
});
