// R34 · the numbered `⌘N` badges follow the scroll viewport.
//
// The user's report, verbatim: 「cmd+n 的快捷键应该更随滚动翻页页适应，目的是快捷
// 选择所见的列表，例外是是按住 cmd 聚焦的底部额外终端打开项。因为应用的概念就是
// 搜索 + 终端」. Until this round the slots were assigned to the first nine rows of
// the list, so scrolling a long (plugin, or file-drop) list left the rows on
// screen with no number and `⌘N` reached rows the user could not see. The fix
// assigns `1`-`9` to the first nine runnable rows *inside the scroller's
// viewport*, so scrolling renumbers the list to what is on screen.
//
// R36 · the family's tenth key, `⌘0`, is the bottom fixed item's (see
// `FIXED_TAIL_SLOT`); a plugin list with no tail spends it on its tenth
// viewport row instead. Both readings keep the invariant that `0` belongs to
// exactly one row.
//
// The two fixed bottom items are deliberately outside that numbering:
//   · the clipboard tail row keeps `⌘0` (`FIXED_TAIL_SLOT`) wherever the list is
//     scrolled, exactly as R19 gave it a fixed number; and
//   · the terminal action bar keeps its own `⌘↩` / hold-`⌘` binding (it is not a
//     numbered row at all).
// Both are the "search + terminal" concept's fixed anchors, so scrolling must
// not move them.
//
// The mapping is a pure function (`shortcutSlotsWithFixedTail` +
// `visibleRowRange` in `result-budget.ts`), so it is driven here without a DOM.
import assert from "node:assert/strict";
import test from "node:test";
import {
  CLIPBOARD_RESULT_ID,
  FIXED_TAIL_SLOT,
  MAX_RESULTS,
  resultIndexForSlot,
  shortcutSlotsWithFixedTail,
  visibleRowRange,
} from "../src/launcher/result-budget.ts";
import type { LauncherItem } from "../src/launcher/LauncherResults.tsx";

/** A minimal runnable row; the slot map only reads the tail row's id. */
const row = (id: string): LauncherItem =>
  ({ type: "app", id, title: id, subtitle: "", app: {} as never }) as LauncherItem;

const appRows = (count: number, from = 0): LauncherItem[] =>
  Array.from({ length: count }, (_, i) => row(`app-${from + i}`));

const clipboardRow = (): LauncherItem =>
  ({ type: "system", id: CLIPBOARD_RESULT_ID, title: "clip", subtitle: "", action: "clipboard" }) as LauncherItem;

/** The composed list: matched rows plus the fixed clipboard tail. */
const withTail = (count: number): LauncherItem[] => [...appRows(count), clipboardRow()];

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

test("R34 · 1-9 number the first nine runnable rows in the viewport", () => {
  const rows = withTail(9); // nine matches + the fixed tail = ten rows
  const flags = rows.map(() => true);

  // A list that fits: the whole list, exactly as before R34.
  assert.deepEqual(
    shortcutSlotsWithFixedTail(rows, flags, { start: 0, end: rows.length }),
    [1, 2, 3, 4, 5, 6, 7, 8, 9, FIXED_TAIL_SLOT],
  );

  // Scrolled down: the numbers follow the rows on screen; the rows above lose
  // their badges; the tail keeps `⌘0` regardless.
  assert.deepEqual(
    shortcutSlotsWithFixedTail(rows, flags, { start: 2, end: rows.length }),
    [null, null, 1, 2, 3, 4, 5, 6, 7, FIXED_TAIL_SLOT],
  );

  // A short viewport numbers only what it can see ("视口不足 9 行时只编可见的").
  assert.deepEqual(
    shortcutSlotsWithFixedTail(rows, flags, { start: 0, end: 3 }),
    [1, 2, 3, null, null, null, null, null, null, FIXED_TAIL_SLOT],
  );
});

test("R34 · a non-runnable row is skipped and takes no number", () => {
  const rows = [row("a"), row("status"), row("b")];
  const flags = [true, false, true];
  assert.deepEqual(
    shortcutSlotsWithFixedTail(rows, flags, { start: 0, end: 3 }),
    [1, null, 2],
  );
  // …and its absence does not shift a later row out of the nine.
  const many = withTail(9);
  const manyFlags = many.map((_, i) => i !== 3);
  const slots = shortcutSlotsWithFixedTail(many, manyFlags, { start: 0, end: many.length });
  assert.equal(slots[3], null, "the skipped row carries no badge");
  assert.deepEqual(slots.slice(0, 3), [1, 2, 3]);
  assert.equal(slots[4], 4, "the row after the skip continues the sequence");
  assert.equal(slots[9], FIXED_TAIL_SLOT, "and the tail still owns `0`");
});

// ── 3 · the fixed bottom items ───────────────────────────────────────────

test("R34 · the fixed clipboard tail keeps ⌘0 wherever the list is scrolled", () => {
  const rows = withTail(20); // a list longer than the slab, so it scrolls
  const flags = rows.map(() => true);

  const top = shortcutSlotsWithFixedTail(rows, flags, { start: 0, end: MAX_RESULTS });
  assert.equal(top[top.length - 1], FIXED_TAIL_SLOT, "the tail is ⌘0 at the top");

  const scrolled = shortcutSlotsWithFixedTail(rows, flags, { start: 10, end: 18 });
  assert.equal(scrolled[scrolled.length - 1], FIXED_TAIL_SLOT, "and still ⌘0 after scrolling");
  assert.equal(
    scrolled.filter((slot) => slot === FIXED_TAIL_SLOT).length,
    1,
    "no viewport row can steal the tail's slot",
  );
  // The eight on-screen rows take 1-8; the rows above are blank.
  assert.deepEqual(scrolled.slice(10, 18), [1, 2, 3, 4, 5, 6, 7, 8]);
  assert.ok(scrolled.slice(0, 10).every((slot) => slot === null));
  // The key handler maps `⌘0` to the tail's own index, not to a visible row.
  assert.equal(resultIndexForSlot(scrolled, 0), rows.length - 1);
  assert.equal(resultIndexForSlot(scrolled, 9), -1, "the tail is not reachable through `⌘9`");
});

test("R34 · a plugin list with no tail numbers only its viewport", () => {
  // A paged plugin list has no fixed clipboard row.
  const rows = appRows(24);
  const flags = rows.map(() => true);
  const first = shortcutSlotsWithFixedTail(rows, flags, { start: 0, end: 9 });
  assert.deepEqual(first.slice(0, 9), [1, 2, 3, 4, 5, 6, 7, 8, 9]);
  assert.ok(first.slice(9).every((slot) => slot === null));

  // R36 · a full ten-row viewport spends the tenth key on its tenth row, since
  // there is no fixed tail to own it.
  const full = shortcutSlotsWithFixedTail(rows, flags, { start: 0, end: MAX_RESULTS });
  assert.deepEqual(full.slice(0, MAX_RESULTS), [1, 2, 3, 4, 5, 6, 7, 8, 9, FIXED_TAIL_SLOT]);
  assert.equal(resultIndexForSlot(full, 0), MAX_RESULTS - 1, "⌘0 is the tenth viewport row");

  // Scroll a page down: the numbers move with the viewport.
  const second = shortcutSlotsWithFixedTail(rows, flags, { start: 10, end: 18 });
  assert.deepEqual(second.slice(10, 18), [1, 2, 3, 4, 5, 6, 7, 8]);
  assert.ok(second.slice(0, 10).every((slot) => slot === null));
  assert.ok(second.slice(18).every((slot) => slot === null));

  // `⌘1` runs the first row of the *current* viewport, and `⌘8` the eighth.
  assert.equal(resultIndexForSlot(second, 1), 10);
  assert.equal(resultIndexForSlot(second, 8), 17);
  assert.equal(resultIndexForSlot(second, 9), -1, "a short viewport has no ninth slot");
  assert.equal(resultIndexForSlot(second, 0), -1, "…and no tenth row to spend `0` on");
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
    /shortcutSlotsWithFixedTail\(displayedResults, displayedRunnableFlags, visibleResultRange\)/,
    "the slots are computed from the viewport",
  );
  assert.match(app, /onVisibleRowsChange=\{setVisibleResultRange\}/, "and the scroller feeds it");
});
