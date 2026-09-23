// R29 · the pagination protocol.
//
// The user's report: 「书签搜索目前好像没有办法进行滚动翻页，这个也需要兼容。从这个
// 协议层定义，然后做成通用性。」 The suite pins the protocol the capability layer
// now owns — the `page` block on an emission, the cursor/hasMore rules, the
// append-with-dedupe merge, the windowing helper and the footer's four states —
// plus the two plugins' ability to hand over more than one viewport, which is
// what makes a scroll-to-load-more possible at all.

import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";
import { createTranslator } from "../src/i18n.ts";
import { FIXED_TAIL_SLOT, MAX_RESULTS, RESULTS_LIST_HEIGHT, ROW_HEIGHT_TWO_LINE, shortcutSlotsWithFixedTail } from "../src/launcher/result-budget.ts";
import {
  PLUGIN_INITIAL_PAGES,
  PLUGIN_LOAD_MORE_THRESHOLD,
  PLUGIN_PAGE_SIZE,
  mergePluginRows,
  normalizePluginPage,
  pagePluginEmission,
  paginatePluginRows,
  pluginFooterState,
  pluginViewHasMore,
  pluginViewItems,
  pluginViewPage,
  resolvePluginView,
  type PluginRow,
} from "../src/launcher/plugin-mode.ts";
import { BROWSER_FETCH_LIMIT, browserSearchRows } from "../src/plugins/browser/mode.ts";
import { clipboardModeRows } from "../src/plugins/clipboard/mode.ts";

const en = createTranslator("en");

const row = (id: string): PluginRow => ({
  family: "clipboard",
  id,
  title: id,
  subtitle: "",
});

// ── A · the protocol block ────────────────────────────────────────────────

test("a list emission carries its pagination block through the capability layer", () => {
  const view = resolvePluginView({
    output: [row("a")],
    page: { cursor: "1", hasMore: true },
  });
  assert.ok(view && view.form === "list");
  assert.deepEqual(view.page, { cursor: "1", hasMore: true });
  assert.equal(pluginViewHasMore(view), true);
  assert.deepEqual(pluginViewPage(view), { cursor: "1", hasMore: true });
});

test("an emission without a page block is a complete list (pre-R29 behaviour)", () => {
  const view = resolvePluginView({ output: [row("a")] });
  assert.ok(view && view.form === "list");
  assert.equal(view.page, null);
  assert.equal(pluginViewHasMore(view), false);
  assert.equal(pluginFooterState(view.page, false), null);
});

test("a malformed page block is dropped rather than guessed", () => {
  assert.equal(normalizePluginPage(undefined), null);
  // A non-string cursor is the start; a non-boolean hasMore is not "true".
  assert.deepEqual(
    normalizePluginPage({ cursor: 7 as unknown as string, hasMore: "yes" as unknown as boolean }),
    { cursor: null, hasMore: false },
  );
  assert.deepEqual(normalizePluginPage({ cursor: "x", hasMore: true }), {
    cursor: "x",
    hasMore: true,
  });
});

test("the text form never pages", () => {
  const view = resolvePluginView({ output: "just text", page: { cursor: "1", hasMore: true } });
  assert.ok(view && view.form === "text");
  assert.equal(pluginViewPage(view), null);
  assert.equal(pluginViewHasMore(view), false);
});

// ── B · the window ────────────────────────────────────────────────────────

test("the page size is one viewport and the first emission is more than one", () => {
  // The first emission must exceed the box, or the scroll trigger that asks for
  // the next page could never fire — the rows would fit exactly.
  assert.equal(PLUGIN_PAGE_SIZE, MAX_RESULTS);
  assert.ok(PLUGIN_INITIAL_PAGES >= 2, "the first emission fills more than the viewport");
  assert.ok(PLUGIN_INITIAL_PAGES * PLUGIN_PAGE_SIZE > MAX_RESULTS);
  assert.ok(PLUGIN_LOAD_MORE_THRESHOLD > 0);
});

test("windowing is a growing prefix: cursor, hasMore and stable indices", () => {
  const rows = Array.from({ length: 60 }, (_, i) => row(`r${i}`));

  const first = paginatePluginRows(rows, PLUGIN_INITIAL_PAGES);
  assert.equal(first.rows.length, PLUGIN_INITIAL_PAGES * PLUGIN_PAGE_SIZE);
  assert.equal(first.page.hasMore, true);
  assert.equal(first.page.cursor, String(first.rows.length));

  const second = paginatePluginRows(rows, PLUGIN_INITIAL_PAGES + 1);
  // The prefix grows: the first page's rows keep their positions.
  assert.deepEqual(second.rows.slice(0, first.rows.length), first.rows);
  assert.ok(second.rows.length > first.rows.length);

  // The last page reports the end: cursor null, hasMore false.
  const last = paginatePluginRows(rows, 10);
  assert.equal(last.rows.length, rows.length);
  assert.equal(last.page.hasMore, false);
  assert.equal(last.page.cursor, null);
});

test("an emission smaller than one page never pages", () => {
  const rows = Array.from({ length: PLUGIN_PAGE_SIZE }, (_, i) => row(`s${i}`));
  const page = paginatePluginRows(rows, PLUGIN_INITIAL_PAGES);
  assert.equal(page.rows.length, rows.length);
  assert.equal(page.page.hasMore, false);
});

// ── C · the merge ─────────────────────────────────────────────────────────

test("pages append in order and a repeated id is dropped", () => {
  const merged = mergePluginRows([row("a"), row("b")], [row("b"), row("c")]);
  assert.deepEqual(merged.map((r) => r.id), ["a", "b", "c"]);
  // The first occurrence keeps its position — a selected row never moves.
  assert.deepEqual(mergePluginRows([row("x")], [row("x")]).map((r) => r.id), ["x"]);
  assert.deepEqual(mergePluginRows([], [row("n")]).map((r) => r.id), ["n"]);
});

// ── D · the footer ────────────────────────────────────────────────────────

test("the footer has exactly four states", () => {
  assert.equal(pluginFooterState(null, false), null);
  assert.equal(pluginFooterState({ cursor: "1", hasMore: true }, true), "loading");
  assert.equal(pluginFooterState({ cursor: "1", hasMore: true }, false), "more");
  assert.equal(pluginFooterState({ cursor: null, hasMore: false }, false), "end");
  // Loading wins over the end: a fetch in flight is still loading.
  assert.equal(pluginFooterState({ cursor: null, hasMore: false }, true), "loading");
});

// ── E · the two plugins can hand over more than a viewport ────────────────

test("the browser plugin returns more than one group when the fetch allows it", () => {
  const many = (prefix: string) =>
    Array.from({ length: 40 }, (_, i) => ({
      id: `${prefix}${i}`,
      title: `${prefix} ${i}`,
      url: `https://${prefix}.example/${i}`,
      profile_key: "default",
    }));
  const full = browserSearchRows({
    bookmarks: many("b"),
    history: many("h"),
    tabs: [],
    profileKey: "default",
    t: en,
    limit: 200,
  });
  assert.ok(
    full.length > PLUGIN_INITIAL_PAGES * PLUGIN_PAGE_SIZE,
    "a long history must be pageable, not clipped to one viewport",
  );
  // The default ceiling is unchanged: the non-paged caller still gets a screen.
  const dflt = browserSearchRows({
    bookmarks: many("b"),
    history: many("h"),
    tabs: [],
    profileKey: "default",
    t: en,
  });
  assert.equal(dflt.length, MAX_RESULTS - 1);
});

test("the clipboard plugin returns the whole history when the fetch allows it", () => {
  const entries = Array.from({ length: 120 }, (_, i) => ({
    id: `e${i}`,
    kind: "text" as const,
    text: `note ${i}`,
    created_at: 1_700_000_000_000 - i * 1000,
    favorite: false,
  }));
  const full = clipboardModeRows(entries, "", en, 1_700_000_000_000, 500);
  assert.equal(full.length, entries.length);
  // The default stays the nine-row viewport budget.
  assert.equal(clipboardModeRows(entries, "", en, 1_700_000_000_000).length, MAX_RESULTS - 1);
});

// ── F · the wiring, pinned at the source ──────────────────────────────────

test("the list scroller owns the scroll-to-bottom trigger", async () => {
  const source = await readFile(new URL("../src/launcher/LauncherResults.tsx", import.meta.url), "utf8");
  assert.match(source, /onScroll=/, "the scroller must listen for scroll");
  assert.match(source, /PLUGIN_LOAD_MORE_THRESHOLD/, "the trigger reads the shared threshold");
  assert.match(source, /pluginFooterState/, "the footer is the protocol's own rule");
  assert.match(source, /launcher-plugin-footer/, "the footer has its own element");
});

test("the catalog hook windows the plugin rows and resets per query", async () => {
  const source = await readFile(new URL("../src/hooks/useLauncherCatalog.ts", import.meta.url), "utf8");
  assert.match(source, /pagePluginEmission/, "the hook pages with the protocol helper");
  assert.match(source, /PLUGIN_INITIAL_PAGES/, "the window starts at the first pages");
  assert.match(source, /loadMorePluginPage/, "the hook exposes the load-more action");
  assert.match(source, /MAX_CLIPBOARD_MAX_ITEMS/, "the clipboard fetch is the whole bounded history");
  // R30 · the browser fetch must be the pageable one. R29 raised
  // `BROWSER_FETCH_LIMIT` and left the hook calling `browserSearchRows` without
  // it, so every browser list stayed at the default nine-row group ceiling —
  // one page, no remainder, no `page` block, no scroll. This is that line.
  assert.match(
    source,
    /browserSearchRows\(\{[\s\S]{0,400}?limit: BROWSER_FETCH_LIMIT/,
    "the inline browser fetch asks for the pageable limit, not the default group cap",
  );
});

// ── G · R30 · the window is the load, and the render is the window ────────

test("every loaded row is rendered, and the box is what hides the tail", () => {
  // The user's report: 「滚动加载还是没有」. Two halves to the repair, and this is
  // the first: the window says how much has been *loaded* and all of it is
  // drawn, so the content is taller than the box and the scroller can move. (The
  // second half — the fetch that makes the window more than one page — is pinned
  // by "the browser fetch is pageable" below.)
  const rows = Array.from({ length: 60 }, (_, i) => row(`r${i}`));
  const emission = pagePluginEmission({ output: rows }, PLUGIN_INITIAL_PAGES);
  assert.ok(emission, "a pageable emission survives the window");
  const loaded = emission!.output as PluginRow[];
  assert.equal(loaded.length, PLUGIN_INITIAL_PAGES * PLUGIN_PAGE_SIZE, "two pages are loaded");

  const view = resolvePluginView(emission);
  assert.equal(
    pluginViewItems(view).length,
    loaded.length,
    "the render count is the loaded count — nothing is sliced for the box",
  );

  // …and the box really is shorter than that: the list's ceiling is ten rows,
  // so the loaded content is taller than the viewport and the scroller can
  // actually move. This is the condition the trigger needs.
  assert.ok(
    loaded.length * ROW_HEIGHT_TWO_LINE > RESULTS_LIST_HEIGHT,
    "the loaded content must exceed the ten-row viewport, or scrollTop can never leave 0",
  );
  assert.equal(pluginViewHasMore(view), true, "and the list reports that more exist");
});

test("one scroll-to-bottom grows the window by exactly one page", () => {
  const rows = Array.from({ length: 60 }, (_, i) => row(`r${i}`));
  const first = pagePluginEmission({ output: rows }, PLUGIN_INITIAL_PAGES)!;
  const second = pagePluginEmission({ output: rows }, PLUGIN_INITIAL_PAGES + 1)!;

  const firstRows = first.output as PluginRow[];
  const secondRows = second.output as PluginRow[];
  assert.equal(secondRows.length, firstRows.length + PLUGIN_PAGE_SIZE, "one page arrives");
  // The prefix is stable: a row the user already sees never moves.
  assert.deepEqual(secondRows.slice(0, firstRows.length), firstRows);
  assert.equal(pluginViewHasMore(resolvePluginView(second)), true, "sixty rows are not the end");
});

test("a list that fits the window is complete: no block, no footer, every row", () => {
  const rows = Array.from({ length: 20 }, (_, i) => row(`r${i}`));
  const emission = pagePluginEmission({ output: rows }, PLUGIN_INITIAL_PAGES + 1)!;
  const view = resolvePluginView(emission);
  assert.equal(pluginViewPage(view), null, "a complete list carries no pagination block");
  assert.equal(pluginViewItems(view).length, rows.length, "and every row is rendered");
  assert.equal(pluginFooterState(pluginViewPage(view), false), null, "so there is no footer");
  assert.equal(
    pagePluginEmission({ output: rows.slice(0, 3) }, 2)!.page,
    undefined,
    "a three-row list is complete on the first page",
  );
});

test("a plugin that pages itself keeps its own cursor", () => {
  const rows = Array.from({ length: 60 }, (_, i) => row(`r${i}`));
  const declared = { cursor: "opaque-token", hasMore: true };
  const emission = pagePluginEmission({ output: rows, page: declared }, 1)!;
  assert.deepEqual(emission.page, declared, "the plugin's token is not replaced by an offset");
  assert.equal(
    (emission.output as PluginRow[]).length,
    rows.length,
    "and a self-paging plugin's emission is not windowed in memory",
  );
  assert.equal(pagePluginEmission({ output: "just text" }, 1)!.page, undefined, "text never pages");
  assert.equal(pagePluginEmission(null, 1), null);
});

test("the browser fetch is pageable — the R29 root cause, pinned", () => {
  const many = (prefix: string) =>
    Array.from({ length: 60 }, (_, i) => ({
      id: `${prefix}${i}`,
      title: `${prefix} ${i}`,
      url: `https://${prefix}.example/${i}`,
      profile_key: "default",
    }));
  const sources = {
    bookmarks: many("b"),
    history: many("h"),
    tabs: [],
    profileKey: "default",
    t: en,
  };
  // The default group ceiling is one page — nine rows (R31 removed the tab
  // status line, so a failed tab read no longer adds a tenth) — which is
  // exactly the state the user's screenshot was in: an
  // emission that never exceeded the viewport, so no `page` block was ever
  // attached and the scroll-to-load path was dead. This is the R29 root cause.
  const capped = pagePluginEmission({ output: browserSearchRows(sources) }, PLUGIN_INITIAL_PAGES)!;
  assert.equal(capped.page, undefined, "the default ceiling is one page, so nothing pages");
  assert.ok(
    (capped.output as PluginRow[]).length <= PLUGIN_PAGE_SIZE,
    "and the whole list fits the box",
  );
  // The inline mode's limit is what makes the same fetch pageable.
  const inline = pagePluginEmission(
    { output: browserSearchRows({ ...sources, limit: BROWSER_FETCH_LIMIT }) },
    PLUGIN_INITIAL_PAGES,
  )!;
  assert.equal(inline.page?.hasMore, true, "the pageable fetch has more than one page");
  assert.equal((inline.output as PluginRow[]).length, PLUGIN_INITIAL_PAGES * PLUGIN_PAGE_SIZE);
});

test("⌘N badges stay on the first viewport, however long the list grows", () => {
  // R36's budget: the family is 1-9 plus 0, and a plugin list has no fixed
  // clipboard row, so the tenth viewport row takes `0` and nothing past it
  // carries a badge — loaded or not, scrolled to or not.
  const items = pluginViewItems(
    resolvePluginView(pagePluginEmission({ output: Array.from({ length: 60 }, (_, i) => row(`r${i}`)) }, 3)),
  );
  const slots = shortcutSlotsWithFixedTail(items, items.map(() => true));
  assert.deepEqual(slots.slice(0, 9), [1, 2, 3, 4, 5, 6, 7, 8, 9]);
  assert.equal(slots[9], FIXED_TAIL_SLOT, "the tenth row spends the family's last key");
  assert.ok(slots.slice(10).every((slot) => slot === null), "the eleventh row onward carries no badge");
});
