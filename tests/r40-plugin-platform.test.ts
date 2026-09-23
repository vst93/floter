// R40 · the plugin platform's three extracted primitives: the declarative filter
// axis (`plugins/filter-axis.ts`), the shared status line (`plugins/status.ts`)
// and the shared mode-entry split (`plugins/mode-entry.ts`).
//
// All three are pure, so this suite drives them directly — no DOM, no Tauri. It
// pins the *contract* the built-ins now delegate to, and checks that the
// built-ins' public shapes are unchanged (the R32/R38 cycles, the R30 status
// rows and the R31/R39 entry rule keep their values, families and `kind`), which
// is the zero-behaviour-change half of the round.

import assert from "node:assert/strict";
import test from "node:test";
import { createTranslator } from "../src/i18n.ts";
import {
  BROWSER_FILTER_AXIS,
  BROWSER_FILTERS,
  CLIPBOARD_FILTER_AXIS,
  CLIPBOARD_FILTERS,
  cycleBrowserFilter,
  cycleClipboardFilter,
  pluginModeEntry,
} from "../src/launcher.ts";
import {
  cyclePluginFilter,
  pluginFilterAxis,
  type PluginFilterAxis,
} from "../src/plugins/filter-axis.ts";
import { pluginStatusRow, statusRowBase } from "../src/plugins/status.ts";
import { splitTriggerWord } from "../src/plugins/mode-entry.ts";
import {
  asPluginRows,
  pluginTierFor,
  resolvePluginView,
} from "../src/launcher/plugin-mode.ts";
import { browserStatusRow } from "../src/plugins/browser/mode.ts";
import { clipboardStatusRow } from "../src/plugins/clipboard/mode.ts";

const en = createTranslator("en");
const zh = createTranslator("zh");

// ── A · the filter axis ───────────────────────────────────────────────────

test("an axis carries its ordered values and resolves each one's label", () => {
  const axis: PluginFilterAxis<"a" | "b"> = pluginFilterAxis(["a", "b"] as const, {
    a: "launcher.browserAll",
    b: "launcher.browserHistory",
  });
  assert.deepEqual([...axis.values], ["a", "b"]);
  assert.equal(axis.labelKey("a"), "launcher.browserAll");
  assert.equal(en(axis.labelKey("b")), en("launcher.browserHistory"));
});

test("cyclePluginFilter wraps at both ends, in both directions", () => {
  const axis = pluginFilterAxis(["a", "b", "c"] as const, {
    a: "launcher.browserAll",
    b: "launcher.browserBookmarks",
    c: "launcher.browserHistory",
  });
  assert.equal(cyclePluginFilter(axis, "a", 1), "b");
  assert.equal(cyclePluginFilter(axis, "b", 1), "c");
  assert.equal(cyclePluginFilter(axis, "c", 1), "a", "forward wraps");
  assert.equal(cyclePluginFilter(axis, "a", -1), "c", "back wraps");
  assert.equal(cyclePluginFilter(axis, "c", -1), "b");
});

test("the built-in cycles are the shared axis cycle, value for value", () => {
  for (const kind of BROWSER_FILTERS) {
    for (const direction of [1, -1] as const) {
      assert.equal(
        cycleBrowserFilter(kind, direction),
        cyclePluginFilter(BROWSER_FILTER_AXIS, kind, direction),
      );
    }
  }
  for (const filter of CLIPBOARD_FILTERS) {
    for (const direction of [1, -1] as const) {
      assert.equal(
        cycleClipboardFilter(filter, direction),
        cyclePluginFilter(CLIPBOARD_FILTER_AXIS, filter, direction),
      );
    }
  }
});

test("the built-in axes keep the R32/R38 values and their words", () => {
  assert.deepEqual([...BROWSER_FILTER_AXIS.values], ["all", "bookmarks", "history", "tabs"]);
  assert.deepEqual(BROWSER_FILTER_AXIS.values, BROWSER_FILTERS);
  assert.equal(en(BROWSER_FILTER_AXIS.labelKey("all")), en("launcher.browserAll"));
  assert.equal(en(BROWSER_FILTER_AXIS.labelKey("tabs")), en("launcher.browserTabs"));

  assert.deepEqual(
    [...CLIPBOARD_FILTER_AXIS.values],
    ["all", "favorites", "text", "image", "link", "files"],
  );
  assert.deepEqual(CLIPBOARD_FILTER_AXIS.values, CLIPBOARD_FILTERS);
  assert.equal(en(CLIPBOARD_FILTER_AXIS.labelKey("text")), en("clipboard.typeText"));
  assert.equal(en(CLIPBOARD_FILTER_AXIS.labelKey("files")), en("clipboard.typeFiles"));
});

test("every built-in chip label resolves in both dictionaries", () => {
  for (const kind of BROWSER_FILTERS) {
    assert.ok(en(BROWSER_FILTER_AXIS.labelKey(kind)).length > 0);
    assert.ok(zh(BROWSER_FILTER_AXIS.labelKey(kind)).length > 0);
  }
  for (const filter of CLIPBOARD_FILTERS) {
    assert.ok(en(CLIPBOARD_FILTER_AXIS.labelKey(filter)).length > 0);
    assert.ok(zh(CLIPBOARD_FILTER_AXIS.labelKey(filter)).length > 0);
  }
});

// ── B · the status line ───────────────────────────────────────────────────

test("the status core is information, not a door", () => {
  assert.deepEqual(statusRowBase("s", "Nothing here"), {
    id: "s",
    title: "Nothing here",
    subtitle: "",
    disabled: true,
    kind: "status",
  });
});

test("an external plugin's status row is a valid generic list of one", () => {
  const row = pluginStatusRow("external-idle", "Type arguments");
  assert.equal(row.family, "plugin");
  assert.equal(row.kind, "status");
  assert.equal(row.disabled, true);
  assert.ok(asPluginRows([row]), "the row conforms to the standard structure");
  // A list of only status lines is display-only by construction (R30/R39).
  assert.equal(pluginTierFor([row]), "display");
  const view = resolvePluginView({ output: [row] });
  assert.equal(view?.form, "list");
  if (view?.form === "list") {
    assert.deepEqual(view.items, [{ type: "status", id: "external-idle", title: "Type arguments" }]);
  }
});

test("the built-in status rows keep their family fields and their kind", () => {
  const browser = browserStatusRow("browser-empty", "launcher.browserEmpty", en);
  assert.equal(browser.family, "browser");
  assert.equal(browser.kind, "status");
  assert.equal(browser.disabled, true);
  assert.equal(browser.title, en("launcher.browserEmpty"));
  assert.equal(browser.url, "");
  assert.equal(browser.profileKey, "default");
  assert.ok(asPluginRows([browser]), "a browser status row still validates as a browser row");

  const clipboard = clipboardStatusRow("clipboard-empty", "clipboard.empty", zh);
  assert.equal(clipboard.family, "clipboard");
  assert.equal(clipboard.kind, "status");
  assert.equal(clipboard.disabled, true);
  assert.equal(clipboard.title, zh("clipboard.empty"));
  assert.ok(asPluginRows([clipboard]), "a clipboard status row still validates as a clipboard row");
});

// ── C · the shared mode-entry rule ────────────────────────────────────────

test("a bare word is not an entry; a word and whitespace is", () => {
  assert.equal(splitTriggerWord("bookmarks"), null, "no whitespace, no mode");
  assert.equal(splitTriggerWord("  bookmarks"), null, "a leading space is not a word");
  assert.equal(splitTriggerWord(""), null);
  assert.deepEqual(splitTriggerWord("bookmarks "), { word: "bookmarks", rest: "" });
});

test("the word is lowercased and the rest is left verbatim", () => {
  assert.deepEqual(splitTriggerWord("Bookmarks Rust"), { word: "bookmarks", rest: "Rust" });
  // The rest keeps its own trailing whitespace: each caller trims its own way
  // (the built-in parsers into a needle, the argv splitter into raw arguments).
  // The whitespace run between the word and the rest is consumed entirely.
  assert.deepEqual(splitTriggerWord("history  async  "), { word: "history", rest: "async  " });
  // A newline is whitespace like any other, and the rest may span lines.
  assert.deepEqual(splitTriggerWord("browser\nrust\nasync"), {
    word: "browser",
    rest: "rust\nasync",
  });
});

test("the built-in parsers read the shared split", () => {
  // The same value through the shared rule and the parser's own shape.
  assert.deepEqual(splitTriggerWord("bookmarks rust")?.rest.trim(), "rust");
  const mode = pluginModeEntry("bookmarks rust");
  assert.deepEqual(mode, { mode: { scope: "browser", kind: "bookmarks" }, needle: "rust" });
  const clip = pluginModeEntry("clip hello world");
  assert.deepEqual(clip, {
    mode: { scope: "clipboard", filter: "all" },
    needle: "hello world",
  });
});
