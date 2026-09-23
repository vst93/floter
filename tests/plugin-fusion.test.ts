// R27 · the built-in plugins fuse with the search box, and each gets a settings
// page of its own.
//
// The user's report, verbatim: 「新的插件勉强算是能用吧，但整体交互还是不行，需要
// 调整一下现在的内置插件，现有的"剪切板"和"书签搜索"这两个插件需要和搜索框进行融
// 合」. R26-A had already given the browser plugin an inline result mode
// (`bookmarks ` / `browser `); this round gives the clipboard plugin the same
// door (`clip `), gives both modes the scope glyph at the left of the field, and
// grows the two plugin pages into one shared settings-card design.
//
// The suite has no DOM, so the paint-level assertions read the sources (the same
// arrangement the R26-A browser tests use) and the decisions are imported
// directly.
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";
import { parseBrowserMode, parseClipboardMode, pluginScope } from "../src/launcher.ts";
import {
  clampClipboardMaxItems,
  DEFAULT_CLIPBOARD_MAX_ITEMS,
  MAX_CLIPBOARD_MAX_ITEMS,
  MIN_CLIPBOARD_MAX_ITEMS,
  normalizeClipboardSettings,
} from "../src/clipboard-history.ts";
import {
  BROWSER_SORT_ORDERS,
  DEFAULT_BROWSER_SORT_ORDER,
  normalizeBrowserSortOrder,
} from "../src/browser-page.ts";

const root = new URL("../", import.meta.url);
const read = (path: string) => readFile(new URL(path, root), "utf8");
const stripComments = (css: string) => css.replace(/\/\*[\s\S]*?\*\//g, "");
const stripJsComments = (src: string) =>
  src.replace(/\/\*[\s\S]*?\*\//g, "").replace(/\/\/[^\n]*/g, "");

// ── A · the clipboard mode is the browser mode's twin ─────────────────────

test("the clipboard mode is entered by a trigger word and a space", () => {
  // The bare word is not the mode: it is the shell's own command (and on
  // Windows `clip` really is one), so only the word *plus a space* enters.
  assert.equal(parseClipboardMode("clip"), null);
  assert.equal(parseClipboardMode("clipboard"), null);
  assert.equal(parseClipboardMode("剪贴板"), null);
  assert.equal(parseClipboardMode(""), null);

  assert.deepEqual(parseClipboardMode("clip "), { needle: "" });
  assert.deepEqual(parseClipboardMode("clipboard "), { needle: "" });
  assert.deepEqual(parseClipboardMode("剪贴板 "), { needle: "" });
  assert.deepEqual(parseClipboardMode("clip  rust  "), { needle: "rust" });
  assert.deepEqual(parseClipboardMode("clipboard https://x"), { needle: "https://x" });
  // Case is folded, exactly as the browser mode folds its trigger words.
  assert.deepEqual(parseClipboardMode("CLIP hello"), { needle: "hello" });

  // The two modes do not overlap: a browser trigger never enters the clipboard
  // mode, and vice versa.
  assert.equal(parseClipboardMode("bookmarks rust"), null);
  assert.equal(parseClipboardMode("browser rust"), null);
  assert.equal(parseBrowserMode("clip rust"), null);
});

test("the scope glyph names the plugin the field is searching", () => {
  assert.equal(pluginScope("clip rust"), "clipboard");
  assert.equal(pluginScope("clip "), "clipboard");
  assert.equal(pluginScope("剪贴板 "), "clipboard");
  assert.equal(pluginScope("bookmarks rust"), "browser");
  assert.equal(pluginScope("browser "), "browser");
  assert.equal(pluginScope("history rust"), "browser");
  // Outside a mode there is no scope, so the field is drawn exactly as it was
  // before this round.
  assert.equal(pluginScope("clip"), null);
  assert.equal(pluginScope("git status"), null);
  assert.equal(pluginScope(""), null);
});

test("the launcher draws the scope glyph inside the field, and only in a mode", async () => {
  const app = stripJsComments(await read("src/App.tsx"));
  assert.match(
    app,
    /const launcherScope = pluginScope\(query\);/,
    "the scope is derived from the query in one place",
  );
  // The glyph is conditional: no mode, no node — so a launcher outside a plugin
  // is pixel-identical to R26.
  assert.match(
    app,
    /\{launcherScope && \(\s*<span className="collapsed-card__scope"/,
    "the scope glyph is rendered only while a plugin mode owns the field",
  );
  // Lucide glyphs, drawn the way every other panel icon is: inline SVG at the
  // 24-grid, stroke inheriting the row's colour.
  assert.match(app, /Clipboard as ClipboardIcon/);
  assert.match(app, /Globe as GlobeIcon/);
  assert.match(app, /<ClipboardIcon size=\{16\} strokeWidth=\{1\.8\} \/>/);
  assert.match(app, /<GlobeIcon size=\{16\} strokeWidth=\{1\.8\} \/>/);
  // The placeholder and the aria-label both follow the scope, so the field says
  // what it searches now instead of leaving the meaning to the glyph alone.
  assert.match(app, /launcherScope === "clipboard"\s*\?\s*t\("input\.placeholderClipboard"\)/);
  assert.match(app, /launcherScope === "browser"\s*\?\s*t\("input\.placeholderBrowser"\)/);
  assert.match(app, /aria-label=\{placeholder\}/);

  const css = stripComments(await read("src/styles/launcher.css"));
  assert.match(
    css,
    /\.collapsed-card__scope\s*\{[^}]*flex:\s*0 0 auto;/s,
    "the glyph is a fixed-size piece of furniture beside the field",
  );
  assert.match(
    css,
    /\.collapsed-card__scope\s*\{[^}]*pointer-events:\s*none;/s,
    "the glyph is not a control — it must not eat the field's clicks",
  );
});

test("inside a plugin scope the list is the plugin's own content", async () => {
  const app = stripJsComments(await read("src/App.tsx"));
  // R27 · the fixed clipboard tail row is a launcher-wide affordance. Inside a
  // plugin scope it would be the panel's door standing inside the panel, so the
  // composed list is the plugin's rows verbatim.
  assert.match(
    app,
    /launcherScope\s*\?\s*\[\.\.\.launcherResults\]\s*:\s*withClipboardResultRow\(/,
    "the tail row is appended in every state except a plugin scope",
  );

  const catalog = stripJsComments(await read("src/hooks/useLauncherCatalog.ts"));
  // R28 · the mode owns the whole list through the capability layer: the view
  // the layer resolved is what the numbered list renders, and a text-form
  // emission contributes no rows at all.
  assert.match(
    catalog,
    /if \(browserMode \|\| clipboardMode\) return pluginView \? pluginViewItems\(pluginView\) : \[\];/,
    "the plugin's view owns the numbered list",
  );
  assert.match(
    catalog,
    /if \(browserMode \|\| clipboardMode\) return null;/,
    "neither plugin mode offers a shell action bar",
  );
  // The entries are fetched once per mode entry and filtered in memory, so
  // typing inside the mode costs no IPC and no resize. R28 · the filter is the
  // plugin's own output rule now (`plugins/clipboard/mode.ts`).
  assert.match(
    catalog,
    /invoke<unknown\[\]>\("clipboard_get_entries", \{ filter: null \}\)/,
    "the mode reads the whole history once",
  );
  const clipboardMode = stripJsComments(await read("src/plugins/clipboard/mode.ts"));
  assert.match(
    clipboardMode,
    /filterClipboardEntries\(\[\.\.\.entries\], needle\)/,
    "the needle filters the fetched history in memory",
  );
  // The plugin's own switch soft-closes the mode, the way the browser's does.
  assert.match(
    catalog,
    /clipboardStatusRow\("clipboard-disabled", "clipboard\.pageUnavailable", t\)/,
    "a disabled clipboard plugin is a status row, not an error",
  );
});

test("a clipboard row copies its entry and closes the launcher", async () => {
  const actions = stripJsComments(await read("src/hooks/useLauncherActions.ts"));
  assert.match(
    actions,
    /const copyClipboardEntry = async \(id: string\)/,
    "one handler, the same act the panel's own row performs",
  );
  assert.match(actions, /invoke\("clipboard_copy_entry", \{ id \}\)/);
  assert.match(
    actions,
    /if \(item\.type === "clipboard"\) \{\s*if \(item\.disabled \|\| !item\.entry\) return;\s*void copyClipboardEntry\(item\.entry\.id\);/,
    "the row is runnable, and a status row is not",
  );

  const results = stripJsComments(await read("src/launcher/LauncherResults.tsx"));
  assert.match(
    results,
    /item\.type === "clipboard" \? \(\s*<SystemActionIcon action="clipboard" \/>/,
    "a clipboard row wears the clipboard glyph, like the system row it mirrors",
  );
  assert.match(
    results,
    /\(item\.type === "clipboard" && item\.disabled === true\)/,
    "a disabled clipboard row is dimmed and skipped like a disabled browser row",
  );
});

// ── B · one settings card, two plugin pages ───────────────────────────────

test("the clipboard capacity clamps to the range the backend honours", () => {
  assert.equal(DEFAULT_CLIPBOARD_MAX_ITEMS, 300);
  assert.equal(clampClipboardMaxItems(300), 300);
  assert.equal(clampClipboardMaxItems(MIN_CLIPBOARD_MAX_ITEMS - 1), MIN_CLIPBOARD_MAX_ITEMS);
  assert.equal(clampClipboardMaxItems(MAX_CLIPBOARD_MAX_ITEMS + 1), MAX_CLIPBOARD_MAX_ITEMS);
  assert.equal(clampClipboardMaxItems(42.7), 42);
  // A typo — an empty box, a NaN — is the shipped default, never "keep none".
  assert.equal(clampClipboardMaxItems(Number.NaN), DEFAULT_CLIPBOARD_MAX_ITEMS);
  assert.equal(clampClipboardMaxItems(Number.POSITIVE_INFINITY), DEFAULT_CLIPBOARD_MAX_ITEMS);

  assert.deepEqual(normalizeClipboardSettings(undefined), { max_items: 300 });
  assert.deepEqual(normalizeClipboardSettings({ max_items: 50 }), { max_items: 50 });
  assert.deepEqual(normalizeClipboardSettings({ max_items: 0 }), { max_items: 10 });
  assert.deepEqual(normalizeClipboardSettings({ max_items: "80" }), { max_items: 300 });
});

test("the clipboard page owns its settings card, on the shared sheet", async () => {
  const page = stripJsComments(await read("src/plugins/clipboard/main.ts"));
  // The narrow pair, not the whole-app settings object: the page has no
  // business rewriting fields it does not own.
  assert.match(page, /invokeCommand<unknown>\("clipboard_get_settings"\)/);
  assert.match(
    page,
    /invokeCommand<unknown>\("clipboard_set_settings", \{ settings: next \}\)/,
  );
  // The card is the shared `.plugin-settings` design, not a second lookalike.
  assert.match(page, /card\.className = "plugin-settings";/);
  assert.match(page, /"plugin-field__control plugin-field__control--number"/);
  assert.match(page, /input\.min = String\(MIN_CLIPBOARD_MAX_ITEMS\)/);
  assert.match(page, /input\.max = String\(MAX_CLIPBOARD_MAX_ITEMS\)/);
  // The toggle lives in the topbar beside the field, like the browser page's.
  assert.match(page, /class="clipboard-panel__settings"/);
  assert.match(page, /settingsToggle\.classList\.toggle\("clipboard-panel__settings--on", settingsOpen\)/);

  const browser = stripJsComments(await read("src/plugins/browser/main.ts"));
  // The browser card is the same sheet after this round — its old per-page
  // class names are gone, not kept as aliases.
  assert.match(browser, /el\("div", "plugin-settings"\)/);
  assert.match(browser, /el\("label", "plugin-field"\)/);
  assert.equal(
    /browser-page__settings|browser-field/.test(browser),
    false,
    "the browser card must use the shared class names, not its own",
  );
  // …and it now offers the sort order the launcher reads from the stored
  // settings.
  assert.match(browser, /for \(const order of BROWSER_SORT_ORDERS\)/);
  assert.match(browser, /sort_order: sort\.value as BrowserSortOrder/);
});

test("both plugin sheets import the one card stylesheet", async () => {
  const shared = stripComments(await read("src/plugins/settings-card.css"));
  assert.match(shared, /\.plugin-settings\s*\{/);
  assert.match(shared, /\.plugin-field__control\s*\{/);
  assert.match(shared, /\.plugin-settings__notice--error\s*\{/);
  for (const page of ["src/plugins/browser/page.css", "src/plugins/clipboard/page.css"]) {
    const css = await read(page);
    assert.match(
      css,
      /@import "\.\.\/settings-card\.css";/,
      `${page} must import the shared card sheet`,
    );
    // The card's rules must not be restated locally: one source, two pages.
    assert.equal(
      /\.plugin-settings\s*\{/.test(stripComments(css)),
      false,
      `${page} must not redeclare the shared card`,
    );
  }
});

test("the browser sort orders are the four the settings file accepts", () => {
  assert.deepEqual([...BROWSER_SORT_ORDERS], [
    "relevance",
    "recent",
    "alphabetical",
    "visits",
  ]);
  assert.equal(DEFAULT_BROWSER_SORT_ORDER, "relevance");
  assert.equal(normalizeBrowserSortOrder("recent"), "recent");
  assert.equal(normalizeBrowserSortOrder("ALPHABETICAL"), "alphabetical");
  assert.equal(normalizeBrowserSortOrder("visits"), "visits");
  assert.equal(normalizeBrowserSortOrder("relevance"), "relevance");
  assert.equal(normalizeBrowserSortOrder("sideways"), "relevance");
  assert.equal(normalizeBrowserSortOrder(undefined), "relevance");
  assert.equal(normalizeBrowserSortOrder(7), "relevance");
});

// ── The i18n keys the two surfaces need ───────────────────────────────────

test("the new keys exist in both dictionaries", async () => {
  const i18n = stripJsComments(await read("src/i18n.ts"));
  const keys = [
    "input.placeholderClipboard",
    "input.placeholderBrowser",
    "clipboardPage.settings",
    "clipboardPage.settingsSaved",
    "clipboardPage.settingsFailed",
    "clipboardPage.maxItems",
    "clipboardPage.maxItemsHint",
    "clipboardPage.maxItemsValue",
    "settings.browserSort",
    "settings.browserSortHint",
    "settings.browserSortRelevance",
    "settings.browserSortRecent",
    "settings.browserSortAlphabetical",
    "settings.browserSortVisits",
  ];
  for (const key of keys) {
    const occurrences = i18n.split(`"${key}":`).length - 1;
    assert.equal(occurrences, 2, `${key} must exist in both the en and zh blocks`);
  }
});
