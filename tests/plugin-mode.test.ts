// R28 · plugin interaction as a base capability.
//
// The user's request, verbatim: 「需要把插件可在搜索框页面交互作为一种基础能力，这
// 个技术能力需要支持两种展现形式：1. 纯文本……2. 现在这种列表……命令行输出的
// 要求：命令行输出的时候，以我们定义好的结构进行输出，按定义好的标准格式输出的，
// 就以列表形式展示；否则，就全当普通文本以第一种方式展示」.
//
// The suite pins the three decisions the capability layer now owns — the form
// (list vs text), the tier (interactive vs display) and the height (min/max,
// scrolling, and the discrete band table) — plus the two plugins' *output*
// rules, which no longer know anything about `LauncherItem`s.
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";
import type { ClipboardEntry } from "../src/clipboard-history.ts";
import { createTranslator } from "../src/i18n.ts";
import {
  LAUNCHER_HEIGHT_BANDS,
  MAX_RESULTS,
  ROW_HEIGHT_TWO_LINE,
  launcherBandIndex,
  resolveLauncherBand,
} from "../src/launcher/result-budget.ts";
import {
  PLUGIN_TEXT_LINE_UNITS,
  PLUGIN_TEXT_MAX_UNITS,
  PLUGIN_TEXT_MIN_UNITS,
  asPluginRows,
  asPluginText,
  pluginRowToItem,
  pluginTextMetrics,
  pluginTierFor,
  pluginViewInteractive,
  pluginViewItems,
  pluginViewRows,
  resolvePluginView,
  type PluginRow,
} from "../src/launcher/plugin-mode.ts";
import { browserSearchRows, browserStatusRow } from "../src/plugins/browser/mode.ts";
import {
  CLIPBOARD_FETCH_LIMIT,
  clipboardModeRows,
  clipboardStatusRow,
} from "../src/plugins/clipboard/mode.ts";

const root = new URL("../", import.meta.url);
const read = (path: string) => readFile(new URL(path, root), "utf8");

const en = createTranslator("en");
const zh = createTranslator("zh");

const browserRow: PluginRow = {
  family: "browser",
  id: "b1",
  title: "Rust",
  subtitle: "https://rust-lang.org",
  url: "https://rust-lang.org",
  profileKey: "default",
};

const clipRow: PluginRow = {
  family: "clipboard",
  id: "c1",
  title: "hello",
  subtitle: "Text",
};

const entry = (id: string, text: string): ClipboardEntry => ({
  id,
  kind: "text",
  text,
  hash: id,
  created_at: 1_700_000_000_000,
  favorite: false,
});

// ── A · the form: structure wins, everything else is text ─────────────────

test("the standard structure is a list, and nothing else is", () => {
  assert.deepEqual(asPluginRows([browserRow, clipRow]), [browserRow, clipRow]);
  // A JSON string of the same structure — the "command-line output" case — is
  // a list too.
  assert.deepEqual(asPluginRows(JSON.stringify([clipRow])), [clipRow]);

  // The counterexample the round exists for: an array of objects that look
  // like rows but do not carry the structure. It must NOT half-parse into a
  // list; it falls to the text form whole.
  assert.equal(asPluginRows([{ id: "x", title: "y" }]), null);
  assert.equal(asPluginRows([browserRow, { id: "x", title: "y" }]), null);
  assert.equal(asPluginRows([{ family: "browser", id: "x", title: "y" }]), null, "no url, no row");
  assert.equal(asPluginRows([{ family: "unknown", id: "x", title: "y" }]), null);

  // Not a list at all.
  assert.equal(asPluginRows([]), null);
  assert.equal(asPluginRows("hello world"), null);
  assert.equal(asPluginRows("{}"), null);
  assert.equal(asPluginRows(null), null);
});

test("plain output reads as text, and a broken structure still shows", () => {
  assert.equal(asPluginText("line one\nline two"), "line one\nline two");
  assert.equal(asPluginText(42), "42");
  assert.equal(asPluginText(null), null);
  assert.equal(asPluginText(undefined), null);
  // A non-conforming structure is printed as JSON rather than dropped.
  assert.match(asPluginText([{ id: "x", title: "y" }]) ?? "", /"id": "x"/);
  // A line that merely starts like JSON but is not: the raw text survives.
  assert.equal(asPluginText("[not json"), "[not json");
});

test("the capability layer resolves the form and the tier", () => {
  const list = resolvePluginView({ output: [browserRow] });
  assert.equal(list?.form, "list");
  assert.equal(list?.tier, "interactive");
  assert.equal(pluginViewInteractive(list), true);

  // All-status rows are display-only by construction.
  const status = resolvePluginView({ output: [browserStatusRow("s", "launcher.browserEmpty", en)] });
  assert.equal(status?.form, "list");
  assert.equal(status?.tier, "display");
  assert.equal(pluginViewInteractive(status), false);

  // A plugin may declare its own tier, and the layer honours it.
  assert.equal(resolvePluginView({ output: [browserRow], tier: "display" })?.tier, "display");
  assert.equal(
    resolvePluginView({ output: [browserStatusRow("s", "launcher.browserEmpty", en)], tier: "interactive" })?.tier,
    "interactive",
  );

  // Text is always display-only, and contributes no rows to the numbered list.
  const text = resolvePluginView({ output: "line one\nline two" });
  assert.equal(text?.form, "text");
  assert.equal(text?.tier, "display");
  assert.equal(pluginViewInteractive(text), false);
  assert.deepEqual(pluginViewItems(text), []);

  // The counterexample at the resolve level.
  const fallback = resolvePluginView({ output: [{ id: "x", title: "y" }] });
  assert.equal(fallback?.form, "text");
  assert.match(fallback?.form === "text" ? fallback.text : "", /"id": "x"/);

  assert.equal(resolvePluginView(null), null);
  assert.equal(resolvePluginView({ output: "" }), null);
  assert.equal(resolvePluginView({ output: null }), null);
  // An empty structure is nothing to say — not the literal `[]` printed as text.
  assert.equal(resolvePluginView({ output: [] }), null);
});

test("a list of only status rows is display-only", () => {
  assert.equal(pluginTierFor([{ family: "browser", id: "x", title: "", url: "", profileKey: "d", disabled: true }]), "display");
  assert.equal(pluginTierFor([browserRow]), "interactive");
  assert.equal(
    pluginTierFor([browserRow, { family: "browser", id: "y", title: "", url: "", profileKey: "d", disabled: true }]),
    "interactive",
    "one runnable row makes the list interactive",
  );
  assert.equal(pluginTierFor([browserRow], "display"), "display");
});

// ── B · the height: floor, ceiling, scroll and the band table ─────────────

test("text has a floor, a ceiling, and scrolls past the ceiling", () => {
  assert.equal(PLUGIN_TEXT_MIN_UNITS, PLUGIN_TEXT_LINE_UNITS * 3);
  assert.equal(PLUGIN_TEXT_MAX_UNITS, MAX_RESULTS * ROW_HEIGHT_TWO_LINE);

  const one = pluginTextMetrics("hello");
  assert.equal(one.lines, 1);
  assert.equal(one.heightUnits, PLUGIN_TEXT_MIN_UNITS, "a short output is still three lines tall");
  assert.equal(one.scrolls, false);

  const long = pluginTextMetrics(Array.from({ length: 40 }, (_, i) => `line ${i}`).join("\n"));
  assert.equal(long.lines, 40);
  assert.equal(long.heightUnits, PLUGIN_TEXT_MAX_UNITS, "past the ceiling the block stops growing");
  assert.equal(long.scrolls, true, "and scrolls inside itself instead");
  assert.equal(long.rows, MAX_RESULTS, "the ceiling is the nine-row budget, like the list's");
});

test("the text form folds into the same band table, with the same hysteresis", () => {
  const rowsFor = (lines: number) =>
    pluginTextMetrics(Array.from({ length: lines }, (_, i) => `l${i}`).join("\n")).rows;

  // A short output lands in a short band; the ceiling lands in the top band,
  // exactly as a nine-row list does.
  assert.equal(launcherBandIndex(rowsFor(1)), launcherBandIndex(2));
  assert.equal(launcherBandIndex(rowsFor(40)), launcherBandIndex(MAX_RESULTS));

  // The band is sticky — the R26-D rule, unchanged for text: once the top band
  // is open, a line or two less does not step the window down.
  const top = LAUNCHER_HEIGHT_BANDS.length - 1;
  assert.equal(resolveLauncherBand(top, rowsFor(9)), top);
  assert.equal(resolveLauncherBand(top, 1), launcherBandIndex(1), "a genuinely short output steps down");
});

test("the view reports the rows the band table reads", () => {
  const list = resolvePluginView({ output: [browserRow, clipRow] });
  assert.equal(pluginViewRows(list), 2);
  const text = resolvePluginView({ output: "a\nb\nc\nd\ne\nf\ng" });
  assert.equal(pluginViewRows(text), pluginTextMetrics("a\nb\nc\nd\ne\nf\ng").rows);
  assert.equal(pluginViewRows(null), 0);
});

// ── C · the row-to-launcher mapping lives in the layer ────────────────────

test("the capability layer owns the row-to-launcher mapping", () => {
  const tab = pluginRowToItem({
    family: "browser",
    id: "tab:chrome:1:2",
    title: "Tab",
    subtitle: "https://x",
    url: "https://x",
    profileKey: "default",
    tab: { browserId: "chrome", windowIndex: 1, tabIndex: 2 },
  });
  assert.equal(tab.type, "browser");
  assert.deepEqual(tab.type === "browser" ? tab.tab : null, {
    browserId: "chrome",
    windowIndex: 1,
    tabIndex: 2,
  });

  const status = pluginRowToItem(browserStatusRow("s", "launcher.browserEmpty", en));
  assert.equal(status.type, "browser");
  assert.equal(status.type === "browser" ? status.disabled : false, true);
  assert.equal(status.type === "browser" ? status.url : "x", "");

  const clip = pluginRowToItem({ family: "clipboard", id: "c", title: "hi", entry: entry("c", "hi") });
  assert.equal(clip.type, "clipboard");
  assert.equal(clip.type === "clipboard" ? clip.entry?.id : null, "c");
  assert.equal(clip.type === "clipboard" ? clip.disabled : true, undefined);
});

// ── D · the two plugins emit data, and only data ──────────────────────────

test("the browser plugin emits rows: merge, groups and soft landings", () => {
  const rows = browserSearchRows({
    bookmarks: [
      { id: "b1", title: "Rust", url: "https://rust-lang.org", profile_key: "default" },
      { id: "b2", title: "Docs", url: "https://doc.rust-lang.org", profile_key: "default" },
    ],
    history: [
      { id: "h1", title: "Rust", url: "https://rust-lang.org", profile_key: "default" },
      { id: "h2", title: "Crate", url: "https://crates.io", profile_key: "default" },
    ],
    tabs: [
      { browser_id: "chrome", window_index: 0, tab_index: 1, title: "Rust tab", url: "https://rust-lang.org", active: true },
    ],
    tabsFailed: false,
    profileKey: "default",
    t: en,
  });
  // The bookmark wins the duplicate URL; the tab is a second group even though
  // its URL matches a bookmark — switch and open-again are different actions.
  assert.deepEqual(rows.map((row) => row.id), ["b1", "b2", "h2", "tab:chrome:0:1"]);

  const failed = browserSearchRows({ bookmarks: [], history: [], tabs: [], tabsFailed: true, profileKey: "default", t: en });
  assert.equal(failed.length, 1);
  assert.equal(failed[0].disabled, true);
  assert.equal(failed[0].title, en("launcher.browserTabsUnavailable"));

  const empty = browserSearchRows({ bookmarks: [], history: [], tabs: [], tabsFailed: false, profileKey: "default", t: en });
  assert.equal(empty.length, 1);
  assert.equal(empty[0].disabled, true);
  assert.equal(empty[0].title, en("launcher.browserEmpty"));
});

test("a browser group never grows past the launcher's row budget", () => {
  const many = (prefix: string) =>
    Array.from({ length: 30 }, (_, i) => ({
      id: `${prefix}${i}`,
      title: `${prefix} ${i}`,
      url: `https://${prefix}.example/${i}`,
      profile_key: "default",
    }));
  const rows = browserSearchRows({
    bookmarks: many("b"),
    history: many("h"),
    tabs: [],
    tabsFailed: false,
    profileKey: "default",
    t: en,
  });
  assert.equal(rows.length, MAX_RESULTS - 1, "the bookmark/history group stops at eight");
});

test("the clipboard plugin emits rows: memory filter, cap and two empty states", () => {
  const now = 1_700_000_000_000;
  const entries = [entry("a", "rust book"), entry("b", "grocery list"), entry("c", "rust compiler")];

  const all = clipboardModeRows(entries, "", en, now);
  assert.deepEqual(all.map((row) => row.id), ["a", "b", "c"]);
  assert.equal(all[0].family, "clipboard");
  assert.equal(all[0].family === "clipboard" ? all[0].entry?.id : null, "a");
  assert.equal(all[0].title, "rust book");

  assert.deepEqual(clipboardModeRows(entries, "rust", en, now).map((row) => row.id), ["a", "c"]);

  // The two empty states are different sentences.
  const emptyFilter = clipboardModeRows(entries, "zzz", en, now);
  assert.equal(emptyFilter.length, 1);
  assert.equal(emptyFilter[0].disabled, true);
  assert.equal(emptyFilter[0].title, en("clipboard.emptyFilter"));
  assert.equal(clipboardModeRows([], "", en, now)[0].title, en("clipboard.empty"));

  const many = Array.from({ length: 30 }, (_, i) => entry(`e${i}`, `note ${i}`));
  assert.equal(clipboardModeRows(many, "", en, now).length, CLIPBOARD_FETCH_LIMIT);
});

test("neither plugin builds launcher items any more", async () => {
  const stripComments = (source: string) =>
    source.replace(/\/\*[\s\S]*?\*\//g, "").replace(/\/\/[^\n]*/g, "");
  for (const file of ["src/plugins/browser/mode.ts", "src/plugins/clipboard/mode.ts"]) {
    const source = stripComments(await read(file));
    assert.doesNotMatch(source, /LauncherItem/, `${file} must emit rows, not launcher items`);
    assert.doesNotMatch(source, /type: "(browser|clipboard)"/, `${file} must not build launcher variants`);
    assert.match(source, /family: "(browser|clipboard)"/, `${file} must emit the standard structure`);
  }
});

// ── E · the surfaces the layer's decisions reach ──────────────────────────

test("the text block's bounds live in the sheet, and the module keeps off the scale knob", async () => {
  const css = await read("src/styles/launcher.css");
  assert.match(
    css,
    /\.launcher-plugin-text\s*\{[^}]*min-height:\s*calc\(var\(--u\) \* var\(--plugin-text-min\)\)/s,
    "the floor is the layer's number, multiplied by the one scale knob",
  );
  assert.match(
    css,
    /\.launcher-plugin-text\s*\{[^}]*max-height:\s*calc\(var\(--u\) \* var\(--plugin-text-max\)\)/s,
    "the ceiling is the layer's number too",
  );
  assert.match(
    css,
    /\.launcher-plugin-text\s*\{[^}]*overflow-y:\s*auto;/s,
    "past the ceiling the block scrolls instead of growing the window",
  );

  const view = await read("src/launcher/PluginTextView.tsx");
  assert.match(view, /"--plugin-text-min": String\(metrics\.minUnits\)/);
  assert.match(view, /"--plugin-text-max": String\(metrics\.maxUnits\)/);
  assert.doesNotMatch(
    view.replace(/\/\*[\s\S]*?\*\//g, "").replace(/\/\/[^\n]*/g, ""),
    /--u\b/,
    "the module hands the sheet numbers; it must not spell the knob itself",
  );
});

test("the launcher draws the text form under the field and sizes the band from the view", async () => {
  const app = await read("src/App.tsx");
  assert.match(app, /pluginText \? \([\s\S]{0,220}<PluginTextView/, "the text form is drawn under the field");
  assert.match(app, /interactive=\{pluginInteractive\}/, "the list tier reaches the row renderer");
  assert.match(
    app,
    /pluginView \? pluginViewRows\(pluginView\) : displayedResults\.length/,
    "the band table reads the view's row count, text included",
  );
  assert.match(
    app,
    /pluginInteractive\s*\n?\s*\? shortcutSlotsWithFixedTail/,
    "a display-only list hands out no numbered shortcuts",
  );
});

test("the keys the two producers print exist in both dictionaries", async () => {
  const i18n = await read("src/i18n.ts");
  for (const key of [
    "launcher.browserEmpty",
    "launcher.browserNoProfile",
    "launcher.browserTabsUnavailable",
    "launcher.browserDisabled",
    "clipboard.pageUnavailable",
    "clipboard.empty",
    "clipboard.emptyFilter",
  ]) {
    const occurrences = i18n.split(`"${key}":`).length - 1;
    assert.equal(occurrences, 2, `${key} must exist in both the en and zh blocks`);
  }
  // …and the status rows really carry the translation, not the key.
  assert.notEqual(zh("clipboard.empty"), en("clipboard.empty"));
  assert.equal(clipboardStatusRow("x", "clipboard.empty", zh).title, zh("clipboard.empty"));
});
