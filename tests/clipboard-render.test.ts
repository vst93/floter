// The clipboard page used to rebuild every row on every render: `render()`
// called `content.replaceChildren()` and `renderRow` built 200+ fresh nodes
// for a full history on each keystroke, selection move or 2s poll. These
// assertions lock in the incremental model — rows are keyed by `data-row-id`,
// repainted in place when their paint key changes, and reordered without
// recreating the nodes that stayed put. Like the other plugin-page tests they
// read the source, since the page owns a DOM (and imports CSS) the node test
// runner cannot instantiate.
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

const root = new URL("../", import.meta.url);
const read = (path: string) => readFile(new URL(path, root), "utf8");

test("the clipboard list is reconciled in place, never rebuilt wholesale", async () => {
  const page = await read("src/plugins/clipboard/main.ts");
  // The old full rebuild — an empty `replaceChildren()` wiping the list before
  // re-adding every row — must not come back.
  assert.ok(
    !page.includes("content.replaceChildren();"),
    "render() must not clear the content container wholesale",
  );
  assert.match(page, /reconcileList\(filtered, now\)/, "render() must reconcile the list with one pass-wide `now`");
  assert.match(page, /const reconcileList = \(/, "reconcileList must exist");
  // The list/empty/failure bodies are the genuine replacements: one node each.
  assert.match(page, /content\.replaceChildren\(list\)/, "only the list container is (re)created once");
});

test("rows are keyed by data-row-id and patched instead of recreated", async () => {
  const page = await read("src/plugins/clipboard/main.ts");
  // Every row advertises its entry id so reconcile can match it.
  assert.match(page, /button\.dataset\.rowId = entry\.id/, "rows must carry data-row-id");
  // A per-row paint key decides whether a node needs repainting at all.
  assert.match(page, /const rowPaintKey = \(/, "a row paint key must gate repaints");
  assert.match(page, /const rowPaintKeys = new WeakMap/, "paint keys must be cached per row");
  assert.match(page, /const applyRowState = \(/, "rows must be updatable in place");
  // The unchanged branch never touches the DOM, and the ordering pass leaves
  // already-ordered nodes untouched (no append of every child each render).
  assert.match(
    page,
    /rowPaintKeys\.get\(row\)\s*\n?\s*!==\s*rowPaintKey\(/,
    "an unchanged row must skip repainting",
  );
  assert.match(page, /list\.insertBefore\(row, cursor\)/, "only misplaced rows may move");
  assert.ok(
    !/list!\.append\(row\)/.test(page),
    "reconcile must not re-append every row on each pass",
  );
});

test("thumbnail bytes are gated on scrolling into view (no bulk prefetch)", async () => {
  const page = await read("src/plugins/clipboard/main.ts");
  assert.match(page, /new IntersectionObserver\(/, "candidate rows must be observed for visibility");
  assert.match(page, /const thumbnailVisible = new Set<string>\(\)/, "visibility must be tracked");
  assert.match(
    page,
    /if \(thumbnailVisible\.has\(entry\.id\)\) wanted\.push\(entry\)/,
    "the load queue must be gated on visibility",
  );
  // The pre-lazy-loader shape — every uncached candidate queued regardless of
  // visibility — must not come back: the queue is no longer built straight off
  // `entries` / `queue.shift()`.
  assert.ok(
    !/const queue = entries\.filter\(/.test(page),
    "unconditional bulk prefetch must be gone",
  );
  assert.ok(
    !/queue\.shift\(\)/.test(page),
    "the queue must be the visibility-gated `wanted` list, not every candidate",
  );
  assert.match(page, /const wanted: ClipboardEntry\[\] = \[\]/, "the queue starts empty and is filled only by the gate");
  // The concurrency cap survives the lazy-loading change.
  assert.match(page, /const slots = Math\.max\(0, 4 - thumbnailPending\.size\)/, "the 4-way cap must remain");
  // The observer must be torn down with the page.
  assert.match(page, /thumbnailObserver\.disconnect\(\)/, "the observer must be disconnected on pagehide");
});
