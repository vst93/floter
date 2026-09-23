// R41 · the plugin filter row belongs to the plugin list, not the search field.
//
// The user's report, verbatim: 「现在两个内置插件在启动的时候进入设置界面，中间的
// 状态切换栏还是会显示出来。这个应该和下面列表一样作为一个整体，而不应该和上面搜索
// 框作为一个整体。」
//
// Before R41 the chips row's visibility was `launcherScope === "browser" ||
// launcherScope === "clipboard"` at each render site, which kept it on screen
// while the plugin's configuration overlay had taken the list's place. This
// file pins the corrected semantics as a pure matrix, and pins the wiring that
// applies it.
//
// Mutations that must turn this file red:
//   * `pluginFilterRowVisible` dropping the `configOpen` check -> the overlay
//     row of the matrix fails;
//   * dropping the `mode !== "collapsed"` check -> the settings/terminal rows
//     fail;
//   * a render site reverting to a bare `launcherScope === "browser"` -> the
//     source assertions fail;
//   * the window height charging the filter by scope again -> the charge
//     assertion fails.

import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

import {
  FILTER_ROW_SCOPES,
  filterRowScope,
  pluginFilterRowVisible,
  type FilterRowState,
} from "../src/launcher/filter-row.ts";

const root = new URL("../", import.meta.url);
const read = (path: string) => readFile(new URL(path, root), "utf8");
const stripJsComments = (src: string) =>
  src.replace(/\/\*[\s\S]*?\*\//g, "").replace(/\/\/[^\n]*/g, "");

const state = (overrides: Partial<FilterRowState> = {}): FilterRowState => ({
  mode: "collapsed",
  scope: null,
  configOpen: false,
  ...overrides,
});

// ── 1 · the visibility matrix ─────────────────────────────────────────────

test("the chips row shows only for a plugin list that is actually on screen", () => {
  // Ordinary search page: no plugin, no chips.
  assert.equal(pluginFilterRowVisible(state()), false);
  // Plugin mode, list showing: the row is the list's filter.
  assert.equal(pluginFilterRowVisible(state({ scope: "browser" })), true);
  assert.equal(pluginFilterRowVisible(state({ scope: "clipboard" })), true);
  // Plugin mode, configuration overlay open: the overlay takes the list's
  // place, so the filter has nothing to filter and is hidden.
  assert.equal(
    pluginFilterRowVisible(state({ scope: "browser", configOpen: true })),
    false,
  );
  assert.equal(
    pluginFilterRowVisible(state({ scope: "clipboard", configOpen: true })),
    false,
  );
  // The settings panel and the terminal render their own trees; the chips row
  // does not exist there even if a stale scope flag is set.
  assert.equal(
    pluginFilterRowVisible(state({ mode: "settings", scope: "browser" })),
    false,
  );
  assert.equal(
    pluginFilterRowVisible(state({ mode: "terminal", scope: "clipboard" })),
    false,
  );
  // An external plugin command has no chip vocabulary, so it never draws one.
  assert.equal(pluginFilterRowVisible(state({ scope: "external" })), false);
});

test("only the two built-in list plugins own a chip row", () => {
  assert.deepEqual([...FILTER_ROW_SCOPES], ["browser", "clipboard"]);
  assert.equal(filterRowScope("browser"), "browser");
  assert.equal(filterRowScope("clipboard"), "clipboard");
  assert.equal(filterRowScope("external"), null);
  assert.equal(filterRowScope(null), null);
});

// ── 2 · the wiring ────────────────────────────────────────────────────────

test("the App gates both chip rows and the height charge on the one predicate", async () => {
  const app = stripJsComments(await read("src/App.tsx"));
  assert.match(
    app,
    /const filterRowVisible = pluginFilterRowVisible\(\{[\s\S]{0,200}?configOpen: pluginConfigOpen && launcherPluginId !== null,[\s\S]{0,40}?\}\);/,
    "the predicate is computed once, from the scope and the overlay",
  );
  assert.match(
    app,
    /filterRowVisible && launcherScope === "browser" && \(\s*<div className="launcher-filter">/,
    "the browser chips are gated on the predicate",
  );
  assert.match(
    app,
    /filterRowVisible && launcherScope === "clipboard" && \(\s*<div className="launcher-filter">/,
    "the clipboard chips are gated on the predicate",
  );
  // The window height charges the filter band by the same predicate, so the
  // row's appearance and the band it occupies can never disagree. R43 · the
  // band is the shared subline (`launcherSubline` = the chips row or the
  // ordinary page's trigger hint), charged through `launcherContentHeight`.
  assert.match(
    app,
    /const launcherSubline = filterRowVisible \|\| triggerHint !== null;/,
    "the subline band is the chips row or the trigger hint, never both",
  );
  assert.match(
    app,
    /const launcherHeight = launcherContentHeight\(\s*launcherHeldUnits,\s*launcherRows,\s*launcherScale,\s*launcherMaxHeight,\s*launcherHasBar,\s*launcherSectionTitle,\s*launcherSubline,\s*\)/,
    "the height charges the filter only when the row is drawn",
  );
  // The predicate lives in the launcher module, not inline in the render.
  assert.match(app, /from "\.\/launcher\/filter-row"/);
});
