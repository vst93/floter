// R34 · the numbered `⌘N` badges follow the scroll viewport; R37 makes the
// assignment purely in order.
//
// The user's report, verbatim: 「cmd+n 的快捷键应该更随滚动翻页页适应，目的是快捷
// 选择所见的列表，例外是是按住 cmd 聚焦的底部额外终端打开项。因为应用的概念就是
// 搜索 + 终端」. Until R34 the slots were assigned to the first nine rows of the
// list, so scrolling a long (plugin, or file-drop) list left the rows on
// screen with no number and `⌘N` reached rows the user could not see. R34
// assigns `1`-`9` to the first nine runnable rows *inside the scroller's
// viewport*, so scrolling renumbers the list to what is on screen.
//
// R36 grew the family a tenth key, `0`, right beside `9` on the number row.
//
// R37 · the user rejected the one exception R36 left: 「现在搜索页面中 剪切板这项被
// 固定放到末尾，并且总是显示，还固定为了 cmd+0，这不对…同时快捷键也要按顺序安排」.
// The clipboard's fixed tail row is deleted (it is an ordinary result
// contributor now — see `launcher/result-budget.ts`), `FIXED_TAIL_SLOT`
// retires, and the tenth key is simply the tenth runnable row in the viewport.
// Nothing is reserved: `1`-`9` then `0`, in order, for whatever rows are on
// screen.
//
// The one fixed bottom item that remains is the terminal action bar, and it is
// deliberately *outside* this numbering: it keeps its own `⌘↩` / hold-`⌘`
// binding and is not a numbered row at all.
//
// The mapping is a pure function (`resultShortcutSlots` + `visibleRowRange` in
// `result-budget.ts`), so it is driven here without a DOM.
import assert from "node:assert/strict";
import test from "node:test";
import {
  LAST_RESULT_SLOT,
  MAX_RESULTS,
  resultIndexForSlot,
  resultShortcutSlots,
  visibleRowRange,
} from "../src/launcher/result-budget.ts";
import type { LauncherItem } from "../src/launcher/LauncherResults.tsx";

/** A minimal runnable row. */
const row = (id: string): LauncherItem =>
  ({ type: "app", id, title: id, subtitle: "", app: {} as never }) as LauncherItem;

const appRows = (count: number, from = 0): LauncherItem[] =>
  Array.from({ length: count }, (_, i) => row(`app-${from + i}`));

// ── 1 · the visible range ────────────────────────────────────────────────

test("R34 · the visible range starts on the first fully visible row", () => {
  const spans = Array.from({ length: 10 }, (_, i) => ({ top: i * 42, height: 42 }));

  // A list that fits: the whole list is the viewport.
  assert.deepEqual(visibleRowRange(spans, 0, 420), { start: 0, end: 10 });
  assert.deepEqual(visibleRowRange(spans, 0, 1000), { start: 0, end: 10 });

  // Scrolled exactly two rows: rows 2..9 are the window (8 rows).
  assert.deepEqual(visibleRowRange(spans, 84, 336), { start: 2, end: 10 });

  // A row clipped by the top edge is skipped, not numbered.
  assert.deepEqual(visibleRowRange(spans, 20, 336), { start: 1, end: 9 });

  // A partially visible row at the bottom still counts (its top is in the box).
  assert.deepEqual(visibleRowRange(spans, 0, 50), { start: 0, end: 2 });

  // The bottom edge is exclusive: a row whose top is at the edge is out.
  assert.deepEqual(visibleRowRange(spans, 0, 42), { start: 0, end: 1 });

  // A status note (`null`) is not a row; the range spans the real rows around it.
  const gapped = [spans[0], null, spans[2], spans[3]];
  assert.deepEqual(visibleRowRange(gapped, 0, 1000), { start: 0, end: 4 });

  // Nothing measured (an empty list) yields an empty range.
  assert.deepEqual(visibleRowRange([], 0, 100), { start: 0, end: 0 });
});

// ── 2 · the viewport → slot mapping ──────────────────────────────────────

test("R37 · 1-9 then 0 number the runnable rows in the viewport, in order", () => {
  const rows = appRows(10);
  const flags = rows.map(() => true);

  // A list that fits: the whole list, `1`-`9` then `0`.
  assert.deepEqual(
    resultShortcutSlots(rows, flags, { start: 0, end: rows.length }),
    [1, 2, 3, 4, 5, 6, 7, 8, 9, LAST_RESULT_SLOT],
  );

  // Scrolled down: the numbers follow the rows on screen; the rows above lose
  // their badges, and the tenth *visible* row takes `0`.
  assert.deepEqual(
    resultShortcutSlots(rows, flags, { start: 2, end: rows.length }),
    [null, null, 1, 2, 3, 4, 5, 6, 7, 8],
  );

  // A short viewport numbers only what it can see ("视口不足 9 行时只编可见的").
  assert.deepEqual(
    resultShortcutSlots(rows, flags, { start: 0, end: 3 }),
    [1, 2, 3, null, null, null, null, null, null, null],
  );

  // A full ten-row viewport spends `0` on its tenth row — there is no reserved
  // slot to take it away any more (R37).
  const ten = appRows(12);
  assert.deepEqual(
    resultShortcutSlots(ten, ten.map(() => true), { start: 0, end: MAX_RESULTS }),
    [1, 2, 3, 4, 5, 6, 7, 8, 9, LAST_RESULT_SLOT, null, null],
  );
});

test("R37 · a non-runnable row is skipped and takes no number", () => {
  const rows = [row("a"), row("status"), row("b")];
  const flags = [true, false, true];
  assert.deepEqual(resultShortcutSlots(rows, flags, { start: 0, end: 3 }), [1, null, 2]);
  // …and its absence does not shift a later row out of the family.
  const many = appRows(10);
  const manyFlags = many.map((_, i) => i !== 3);
  const slots = resultShortcutSlots(many, manyFlags, { start: 0, end: many.length });
  assert.equal(slots[3], null, "the skipped row carries no badge");
  assert.deepEqual(slots.slice(0, 3), [1, 2, 3]);
  assert.equal(slots[4], 4, "the row after the skip continues the sequence");
  // The skip removes a row, so only nine runnable rows remain: the family's
  // ninth key lands on the last one, and `0` is not spent at all.
  assert.equal(slots[9], 9, "nine runnable rows number `1`-`9`");
  assert.ok(
    slots.every((slot) => slot !== LAST_RESULT_SLOT),
    "with only nine runnable rows there is no tenth to spend `0` on",
  );
});

// ── 3 · the numbering is the viewport's, and nothing is reserved ──────────

test("R37 · scrolling renumbers the whole family, `0` included", () => {
  const rows = appRows(20);
  const flags = rows.map(() => true);

  const top = resultShortcutSlots(rows, flags, { start: 0, end: MAX_RESULTS });
  assert.deepEqual(top.slice(0, 10), [1, 2, 3, 4, 5, 6, 7, 8, 9, LAST_RESULT_SLOT]);
  assert.ok(top.slice(10).every((slot) => slot === null), "past the family, no badge");
  assert.equal(resultIndexForSlot(top, 0), 9, "`⌘0` is the tenth visible row");

  // A page down: the numbers move with the viewport, `0` included.
  const scrolled = resultShortcutSlots(rows, flags, { start: 10, end: 18 });
  assert.deepEqual(scrolled.slice(10, 18), [1, 2, 3, 4, 5, 6, 7, 8]);
  assert.ok(scrolled.slice(0, 10).every((slot) => slot === null));
  assert.ok(scrolled.slice(18).every((slot) => slot === null));
  assert.equal(resultIndexForSlot(scrolled, 1), 10, "`⌘1` is the first row of the viewport");
  assert.equal(resultIndexForSlot(scrolled, 8), 17, "`⌘8` is the eighth");
  assert.equal(resultIndexForSlot(scrolled, 9), -1, "a short viewport has no ninth slot");
  assert.equal(resultIndexForSlot(scrolled, 0), -1, "…and no tenth row to spend `0` on");

  // A full ten-row page: the tenth on-screen row takes `0`, wherever it is.
  const page = resultShortcutSlots(rows, flags, { start: 5, end: 15 });
  assert.equal(page[14], LAST_RESULT_SLOT, "the tenth row of the page owns `0`");
  assert.equal(resultIndexForSlot(page, 0), 14);
  assert.equal(
    page.filter((slot) => slot === LAST_RESULT_SLOT).length,
    1,
    "`0` is handed to exactly one row",
  );
});

test("R37 · a matched clipboard row is an ordinary row and numbers in order", () => {
  // The clipboard entry the catalog contributes is a `system` row like
  // `restart`; nothing in the slot map may special-case it (that was R36's
  // `FIXED_TAIL_SLOT`, retired here).
  const clipboard = {
    type: "system",
    id: "system-clipboard",
    title: "clip",
    subtitle: "",
    action: "clipboard",
  } as LauncherItem;
  const rows = [row("a"), clipboard, row("b")];
  assert.deepEqual(
    resultShortcutSlots(rows, rows.map(() => true)),
    [1, 2, 3],
    "the clipboard row numbers by its position, like every other row",
  );
  assert.equal(resultIndexForSlot([1, 2, 3], 2), 1, "`⌘2` reaches it where it sits");
});

test("R34 · the slot map is the key handler's one source", async () => {
  const { readFile } = await import("node:fs/promises");
  const read = (path: string) => readFile(new URL(`../${path}`, import.meta.url), "utf8");
  const actions = await read("src/hooks/useLauncherActions.ts");
  const keyboard = await read("src/hooks/useAppKeyboard.ts");
  for (const source of [actions, keyboard]) {
    assert.match(
      source,
      /resultIndexForSlot\(resultShortcutSlots, resultNumber\)/,
      "every ⌘N routes through the shared slot → index map",
    );
  }
});

test("R34 · the scroller re-measures on scroll and App feeds the slots from it", async () => {
  const { readFile } = await import("node:fs/promises");
  const read = (path: string) => readFile(new URL(`../${path}`, import.meta.url), "utf8");
  const renderer = await read("src/launcher/LauncherResults.tsx");
  const app = await read("src/App.tsx");

  // The scroller reports its visible rows from the pure range function, and a
  // scroll schedules a throttled re-measure.
  assert.match(
    renderer,
    /visibleRowRange\(spans, list\.scrollTop, list\.clientHeight\)/,
    "the component measures the scroller and hands it to the pure range function",
  );
  assert.match(renderer, /onScroll=\{\(event\) => \{[\s\S]*?scheduleVisibleRows\(\)/, "every scroll re-measures");
  assert.match(renderer, /requestAnimationFrame\(run\)/, "the re-measure is frame-throttled");
  assert.match(
    renderer,
    /onVisibleRowsChange\(range\)/,
    "the measured range is reported to App",
  );

  // App stores the reported range and computes the slots from it, so the
  // badges and the key handler share one map.
  assert.match(
    app,
    /const \[visibleResultRange, setVisibleResultRange\] = useState<VisibleRowRange>/,
    "App holds the reported viewport",
  );
  assert.match(
    app,
    /resultShortcutSlots\(displayedResults, displayedRunnableFlags, visibleResultRange\)/,
    "the slots are computed from the viewport",
  );
  assert.match(app, /onVisibleRowsChange=\{setVisibleResultRange\}/, "and the scroller feeds it");
});
