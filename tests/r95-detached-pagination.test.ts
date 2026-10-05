// R95 · the detached plugin window's list pages like the launcher's.
//
// R92's difference table named the biggest remaining gap between the three
// plugin containers: `DetachedPluginApp.tsx` resolved its view and rendered
// **every** row the run emitted, while the launcher windowed the same emission
// through `pagePluginEmission`. A long plugin list in a pinned window could not
// be scrolled past the first page, because there was no second page. This round
// wires the detached view to the same protocol rule, the same page size and the
// same first window, and triggers the next page from the scroller's own
// geometry — the launcher's R29 pattern, not a second one.
//
// The node runner has no DOM, so the pure protocol half is driven directly and
// the JSX wiring is read off the source, exactly as the neighbouring R93 suite
// does. Mutations this file exists to catch:
//
//   * rendering the raw emission (no `pagePluginEmission`) → the truncation and
//     wiring guards go red;
//   * a scroll to the bottom that never appends — or appends twice → the
//     increment and trigger guards go red;
//   * a page size (or a first window) invented in the detached view instead of
//     the launcher's constant → the "one constant" guard goes red;
//   * a footer/spinner added to the detached list → the no-new-chrome guard
//     goes red.

import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

import {
  PLUGIN_INITIAL_PAGES,
  PLUGIN_LOAD_MORE_THRESHOLD,
  PLUGIN_PAGE_SIZE,
  pagePluginEmission,
  pluginViewHasMore,
  pluginViewItems,
  resolvePluginView,
  type PluginRow,
} from "../src/launcher/plugin-mode.ts";

const root = new URL("../", import.meta.url);
const read = (path: string) => readFile(new URL(path, root), "utf8");
const stripJsComments = (source: string) =>
  source.replace(/\/\*[\s\S]*?\*\//g, "").replace(/^\s*\/\/.*$/gm, "");

const row = (id: string): PluginRow => ({
  family: "plugin",
  id,
  title: id,
  subtitle: "",
});
const rows = (n: number) => Array.from({ length: n }, (_, i) => row(`r${i}`));

// ── A · the first paint is one window, not the whole list ─────────────────

test("R95 · the detached first paint is one window of pages, not every row", () => {
  const emission = pagePluginEmission({ output: rows(60) }, PLUGIN_INITIAL_PAGES)!;
  const view = resolvePluginView(emission);
  assert.equal(
    pluginViewItems(view).length,
    PLUGIN_INITIAL_PAGES * PLUGIN_PAGE_SIZE,
    "the first paint shows exactly the initial window",
  );
  assert.ok(pluginViewItems(view).length < 60, "the tail of a long list is held back");
  assert.equal(pluginViewHasMore(view), true, "and the list reports that more exist");
});

// ── B · one scroll to the bottom appends exactly one page ─────────────────

test("R95 · one scroll-to-bottom appends exactly one page, as a stable prefix", () => {
  const first = pagePluginEmission({ output: rows(60) }, PLUGIN_INITIAL_PAGES)!;
  const second = pagePluginEmission({ output: rows(60) }, PLUGIN_INITIAL_PAGES + 1)!;
  const firstRows = first.output as PluginRow[];
  const secondRows = second.output as PluginRow[];
  assert.equal(secondRows.length, firstRows.length + PLUGIN_PAGE_SIZE, "one page arrives");
  assert.deepEqual(
    secondRows.slice(0, firstRows.length),
    firstRows,
    "the prefix is stable: a row the user already sees never moves",
  );
  assert.equal(pluginViewHasMore(resolvePluginView(second)), true, "sixty rows are not the end");
});

// ── C · a list that fits one page is byte-for-byte the old resolve ────────

test("R95 · a list of at most one page is returned untouched — no block, every row", () => {
  for (const length of [1, PLUGIN_PAGE_SIZE]) {
    const emission = { output: rows(length) };
    const paged = pagePluginEmission(emission, PLUGIN_INITIAL_PAGES);
    // Identity, not just equality: the pre-R95 render path is literally the
    // object it was handed, so the ≤-page-size case cannot drift.
    assert.equal(paged, emission, `${length} rows: the emission is returned as-is`);
    assert.equal(paged!.page, undefined, `${length} rows: no pagination block is attached`);
    assert.deepEqual(
      resolvePluginView(paged),
      resolvePluginView(emission),
      `${length} rows: the resolved view is identical`,
    );
    assert.equal(
      pluginViewItems(resolvePluginView(paged)).length,
      length,
      `${length} rows: every row renders`,
    );
    assert.equal(pluginViewHasMore(resolvePluginView(paged)), false);
  }
});

test("R95 · a non-list body never pages, whatever the page count", () => {
  const text = { output: "one line of plugin output" };
  assert.equal(pagePluginEmission(text, 9), text, "text is returned as-is");
  assert.equal(pagePluginEmission(null, 9), null);
  // An empty structure is not a pageable list either.
  assert.equal(pluginViewHasMore(resolvePluginView(pagePluginEmission({ output: [] }, 9))), false);
});

// ── D · the wiring: one rule, one page size, one first window ─────────────

test("R95 · the detached view pages through the launcher's own rule and constants", async () => {
  const view = stripJsComments(await read("src/plugin-window/DetachedPluginApp.tsx"));
  // The protocol helper, not a second implementation.
  assert.match(
    view,
    /pagePluginEmission\(\{ output: text \}, pluginPages\)/,
    "the list arm windows with pagePluginEmission",
  );
  // The same first window the launcher starts from, and the reset a new request
  // earns (a new request is a new result set).
  assert.match(view, /useState\(PLUGIN_INITIAL_PAGES\)/);
  assert.match(view, /setPluginPages\(PLUGIN_INITIAL_PAGES\)/);
  // The page size is never re-derived here: the view names no `PLUGIN_PAGE_SIZE`
  // of its own and no literal page size. The one number comes from
  // `plugin-mode.ts` through `pagePluginEmission`.
  assert.doesNotMatch(view, /PLUGIN_PAGE_SIZE/, "the page size is the protocol's, not a local copy");
});

test("R95 · the body scroller owns the scroll-to-bottom trigger", async () => {
  const view = stripJsComments(await read("src/plugin-window/DetachedPluginApp.tsx"));
  assert.match(view, /onScroll=\{onBodyScroll\}/, "the body listens for scroll");
  assert.match(
    view,
    /remaining <= PLUGIN_LOAD_MORE_THRESHOLD/,
    "the trigger reads the shared threshold",
  );
  assert.match(view, /setPluginPages\(\(pages\) => pages \+ 1\)/, "one page per trigger");
  // The guard: no second append while one is in flight, and none at all when the
  // list is complete.
  assert.match(view, /if \(pluginLoadingRef\.current \|\| !pluginHasMoreRef\.current\) return;/);
  assert.match(view, /if \(!pluginHasMoreRef\.current\) return;/);
});

test("R95 · a new request resets the window to its first pages", async () => {
  const view = stripJsComments(await read("src/plugin-window/DetachedPluginApp.tsx"));
  // The reset sits inside `run`, the one place a new request lands — before the
  // state machine commits it, so the first paint of the new list is page one.
  assert.match(
    view,
    /const run = useCallback\(\(request: DetachRequest\) => \{[\s\S]{0,200}?setPluginPages\(PLUGIN_INITIAL_PAGES\);/,
    "the window resets per request, not per keystroke and not never",
  );
});

// ── E · the one row-rendering close-out: the muted status note ────────────

test("R95 · a status row is a muted note, not a list row", async () => {
  const view = stripJsComments(await read("src/plugin-window/DetachedPluginApp.tsx"));
  // The launcher draws a `status` item as a muted sentence with no row plate
  // (`.launcher-status`); the detached list now does the same, with the window's
  // own existing status class — no new stylesheet rule.
  assert.match(view, /item\.type === "status" \? \(/);
  assert.match(
    view,
    /<li key=\{item\.id\} className="plugin-window__status" title=\{item\.title\}>/,
    "the status row wears the window's muted status line, not the row plate",
  );
});

// ── F · no new chrome: no footer, no spinner, no i18n ─────────────────────

test("R95 · the pagination adds no footer, spinner or i18n key", async () => {
  const view = stripJsComments(await read("src/plugin-window/DetachedPluginApp.tsx"));
  // The scroll itself is the affordance (the launcher's quiet "more" state), so
  // the detached window ports neither the footer element nor its spinner.
  assert.doesNotMatch(view, /spinner/i);
  assert.doesNotMatch(view, /plugin-footer/);
  assert.doesNotMatch(view, /pluginLoadingMore|pluginEnd/);
  // And no new message key was invented for this round.
  assert.doesNotMatch(view, /t\("[a-z]+\.[a-zA-Z]*[Pp]ag/);
});
