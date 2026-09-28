// R66 · the launcher's list is the height of the rows it actually draws — for
// plugin lists too, and zero rows when there is nothing to draw.
//
// The user's report, verbatim: 「现在搜索页的列表高度还是没有完全根据列表项自适应，
// 还是会有多余空白的情况」. R43 had fixed this for the *ordinary* page (each row at
// its own drawn height, compact or two-line) and R58 for the window/list ceiling
// fork, but two over-charges survived, and both leave a band of sunken panel where
// the eye reads blank:
//
//   1. **a plugin list priced every row `42u`.** R43 changed the ordinary branch
//      to `launcherListUnits(items.map(launcherRowHeightUnits))` and left the
//      plugin branch on `pluginViewRows × 42u`, so a list of compact rows — the
//      shape a direct-output command's `{ id, title }` output takes, drawn at 34u)
//      sat in a window 8u per row taller than its glass (12u per status row).
//      Measured in a headless Chromium against the real sheets: a five-row
//      compact list drew 238px and the App asked for 278px — a 40px band.
//   2. **a collapsed panel still billed a row.** `launcherRows`' `Math.max(1, …)`
//      and `resolveLauncherUnits`' 34u floor are there for the empty-query
//      recents, but a query that matched nothing draws *no* panel at all (the
//      clip closes), so the window kept 34u plus the 26px empty-query heading the
//      collapsed clip never paints. Measured against the module: the collapsed
//      window was 98px (and 124px with the un-drawn heading) over a 62px card.
//
// Both fixes are one rule: what the sheet draws. The plugin branch now prices its
// items' own heights, and the panel's own open predicate (`launcherPanelOpen`,
// the clip's own test) gates the row count, the unit total and the heading.
//
// Mutations that must turn this file red:
//   * a plugin list pricing `pluginViewRows × 42u` again -> the plugin-list case;
//   * the `Math.max(1, …)` / 34u floor coming back for a closed panel -> the
//     collapsed case (the window would jump to 96px / 122px);
//   * the heading charged while the clip is closed -> the empty-field case.
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

const root = new URL("../", import.meta.url);
const read = (path: string) => readFile(new URL(path, root), "utf8");
const stripJsComments = (src: string) =>
  src.replace(/\/\*[\s\S]*?\*\//g, "").replace(/\/\/[^\n]*/g, "");

// The sheets' own geometry (see `tests/r58-adaptive-window.test.ts` for the
// segment-by-segment audit). Every term here is a rule in `launcher.css`.
const FRAME = 2;
const FIELD = 56;
const BREATH = 4;
const CHIPS = 24 + 4;
const PANEL_TOP = 4;
const PANEL_TAIL = 2;
const SCROLL_EDGE = 4;
const GAP = 1;
const TITLE = 26;

/** What the sheets draw for a card whose field is followed by an optional chip
 *  row, `heights` of result rows and an optional section heading. `open: false`
 *  is the collapsed clip: only the field band is painted. */
const drawn = ({
  open,
  chips = false,
  heights = [],
  title = false,
}: {
  open: boolean;
  chips?: boolean;
  heights?: number[];
  title?: boolean;
}): number => {
  if (!open) return FRAME + FIELD + BREATH;
  const rows = heights.length;
  return (
    FRAME +
    FIELD +
    BREATH +
    (chips ? CHIPS + PANEL_TOP + SCROLL_EDGE : 0) +
    (title ? TITLE : 0) +
    heights.reduce((sum, height) => sum + height, 0) +
    Math.max(0, rows - 1) * GAP +
    PANEL_TAIL
  );
};

// ── 1 · a plugin list is its rows' own heights ────────────────────────────

test("R66 · a plugin list of compact rows is exactly its drawn card", async () => {
  const { launcherContentHeight, launcherListUnits } = await import("../src/launcher/result-budget.ts");

  // The direct-output shape: five `{ id, title }` rows, each drawn compact (34u,
  // no subtitle — see `row-content.ts`). No chips row, no action bar.
  const heights = [34, 34, 34, 34, 34];
  const before = launcherContentHeight(
    heights.length * 42, // the R43 plugin branch: every row worst-case
    heights.length,
    1,
    Number.POSITIVE_INFINITY,
    false,
    false,
    false,
  );
  const after = launcherContentHeight(
    launcherListUnits(heights),
    heights.length,
    1,
    Number.POSITIVE_INFINITY,
    false,
    false,
    false,
  );
  assert.equal(drawn({ open: true, heights }), 238, "the sheets draw a five-row compact list at 238px");
  assert.equal(after, 238, "…and the window is that card");
  assert.equal(before, 278, "the 42u-per-row billing was 40px taller");

  // A status line is 30u, not a result row: one status item draws 94px.
  assert.equal(
    launcherContentHeight(30, 1, 1, Number.POSITIVE_INFINITY, false, false, false),
    drawn({ open: true, heights: [30] }),
    "a status row is its own 30u",
  );
});

test("R66 · a plugin list with the chips row still matches, at every height mix", async () => {
  const { launcherContentHeight, launcherListUnits } = await import("../src/launcher/result-budget.ts");

  const cases: number[][] = [
    [42, 42, 42],
    [34, 34, 34, 34, 34],
    [30, 34, 42],
    [42],
  ];
  for (const heights of cases) {
    for (const chips of [false, true]) {
      assert.equal(
        launcherContentHeight(
          launcherListUnits(heights),
          heights.length,
          1,
          Number.POSITIVE_INFINITY,
          false,
          false,
          chips,
        ),
        drawn({ open: true, chips, heights }),
        `[${heights.join(", ")}] with chips ${chips} is the card the sheets draw`,
      );
    }
  }
});

// ── 2 · a collapsed panel draws (and bills) no list ───────────────────────

test("R66 · a collapsed panel is the field band, not a phantom row", async () => {
  const { launcherContentHeight, ROW_HEIGHT_COMPACT } = await import("../src/launcher/result-budget.ts");

  const collapsed = launcherContentHeight(0, 0, 1, Number.POSITIVE_INFINITY, false, false, false);
  const drawnCard = drawn({ open: false });
  // The collapsed card is the 56u field, its 4u breath and the 2px frame. The
  // formula still folds the panel's 2u tail into its unit part, so the window has
  // to be allowed to be that 2u over — but nowhere near the 34u phantom row (96)
  // or the 26px heading (122) the old floors billed.
  assert.equal(drawnCard, 62, "the collapsed card draws 62px");
  assert.ok(collapsed <= drawnCard + 2, `a collapsed window may be at most 2px over its 62px card, got ${collapsed}`);
  assert.ok(collapsed < drawnCard + ROW_HEIGHT_COMPACT, "…and must never bill a whole phantom row");

  // The empty field with no recents is the same case: the heading is inside the
  // collapsed clip and must not be charged. The old numbers, for the record.
  const oldPhantom = launcherContentHeight(ROW_HEIGHT_COMPACT, 1, 1, Number.POSITIVE_INFINITY, false, false, false);
  const oldWithHeading = launcherContentHeight(
    ROW_HEIGHT_COMPACT,
    1,
    1,
    Number.POSITIVE_INFINITY,
    false,
    true,
    false,
  );
  assert.equal(oldPhantom, 98, "the phantom-row window was 98px over a 62px card");
  assert.equal(oldWithHeading, 124, "…and the un-drawn heading made the empty field 124px");
});

// ── 3 · the App wiring ────────────────────────────────────────────────────

test("R66 · the sticky absorber only holds while the content changes, then settles", async () => {
  const budget = await import("../src/launcher/result-budget.ts");
  const { LAUNCHER_SHRINK_SETTLE_MS, launcherContentHeight, resolveLauncherUnits } = budget;

  const app = stripJsComments(await read("src/App.tsx"));
  // The settle is keyed on the content's own total and reads one constant; when
  // it fires the height is the raw total, not the resolver's held one.
  assert.match(app, /\}, \[launcherListUnitsRaw\]\);/, "the beat restarts on every content change");
  assert.match(
    app,
    /window\.setTimeout\(\s*\(\) => setLauncherSettledUnits\(launcherListUnitsRaw\),\s*LAUNCHER_SHRINK_SETTLE_MS,\s*\)/,
  );
  assert.match(app, /const launcherSettled = launcherSettledUnits === launcherListUnitsRaw;/, "…stored as the total it settled at, so a change is immediately unsettled");
  assert.match(
    app,
    /launcherSettled\s*\?\s*launcherListUnitsRaw\s*:\s*resolveLauncherUnits\(launcherUnitsRef\.current, launcherListUnitsRaw\)/,
    "…and the settled branch is the exact total",
  );
  assert.ok(
    LAUNCHER_SHRINK_SETTLE_MS > 0 && LAUNCHER_SHRINK_SETTLE_MS <= 400,
    "the beat is short — a flicker fix, not a second state",
  );

  // The user's own state, in units: the empty-query recents (8 two-line + 2
  // compact) hold 404u while the typed page (4 two-line + 6 compact) is 372u — a
  // 32u hold, because 404 - 372 is under one worst-case row. While the content is
  // changing the absorber keeps the taller total (the band the user photographed);
  // the settled height is the content's own.
  const recents = 8 * 42 + 2 * 34;
  const typed = 4 * 42 + 6 * 34;
  assert.equal(recents, 404);
  assert.equal(typed, 372);
  assert.equal(resolveLauncherUnits(recents, typed), recents, "the typing-time hold is the band");
  const held = launcherContentHeight(recents, 10, 0.9, Number.POSITIVE_INFINITY, true, false, false);
  const settled = launcherContentHeight(typed, 10, 0.9, Number.POSITIVE_INFINITY, true, false, false);
  assert.ok(held - settled >= 24, `the settle removes the band: ${held} -> ${settled}`);
});

test("R66 · the App prices a plugin list per row and collapses with the panel", async () => {
  const app = stripJsComments(await read("src/App.tsx"));

  // One predicate for "something is drawn below the field", and it is the clip's
  // own — no second spelling to drift.
  assert.match(
    app,
    /const launcherPanelOpen =\s*displayedResults\.length > 0 \|\| pluginText !== null \|\| launcherAlertRow;/,
    "the panel's open predicate is declared once",
  );
  assert.match(
    app,
    /displayedResults\.length > 0 \|\| pluginText !== null \|\| launcherAlertRow\s*\?\s*"launcher-bottom-clip launcher-bottom-clip--open"/,
    "…and is the clip's own test",
  );

  // The plugin list prices each item at its own height, the way the ordinary
  // page does; the text form keeps its band price.
  assert.match(
    app,
    /pluginView\.form === "list"\s*\?\s*launcherListUnits\(pluginView\.items\.map\(launcherRowHeightUnits\)\)\s*:\s*pluginViewRows\(pluginView\) \* ROW_HEIGHT_TWO_LINE/,
    "a plugin list is the sum of its rows' heights",
  );
  assert.doesNotMatch(
    app,
    /pluginView\s*\?\s*pluginViewRows\(pluginView\) \* ROW_HEIGHT_TWO_LINE/,
    "the 42u-per-plugin-row billing is gone",
  );

  // The collapsed panel zeroes the list contribution, the row count and the
  // heading — and skips the sticky resolver's own 34u floor.
  assert.match(
    app,
    /const launcherListUnitsRaw =\s*\(launcherPanelOpen \? launcherListContentUnits : 0\) \+ launcherChromeUnits;/,
    "a collapsed panel contributes no list units",
  );
  assert.match(
    app,
    /const launcherHeldUnits = launcherHasContent\s*\?\s*launcherSettled\s*\?\s*launcherListUnitsRaw\s*:\s*resolveLauncherUnits\(launcherUnitsRef\.current, launcherListUnitsRaw\)\s*:\s*0;/,
    "…and settles to the exact total, skipping the sticky floor",
  );
  assert.match(
    app,
    /const launcherSectionTitle =\s*launcherPanelOpen && !launcherScope && !query\.trim\(\) && !fileRows\.length;/,
    "…and the heading is charged only when its panel is drawn",
  );
});

test("R66 · the item-height rule is the row renderer's own, so the two cannot disagree", async () => {
  // Both the App's billing and `LauncherResults`' `compact` class come from
  // `resultRowContent(...).subtitle === null`; a status row is the launcher's
  // 30u note. This is the invariant R43 wrote for the ordinary page, and the one
  // the plugin branch was missing.
  const app = stripJsComments(await read("src/App.tsx"));
  assert.match(
    app,
    /const launcherRowHeightUnits = \(item: LauncherItem\): number => \{\s*if \(item\.type === "status"\) return LAUNCHER_STATUS_UNITS;\s*return resultRowContent\(item, t\)\.subtitle === null\s*\?\s*ROW_HEIGHT_COMPACT\s*:\s*ROW_HEIGHT_TWO_LINE;/,
    "one function prices every row, ordinary or plugin",
  );
  const budget = await import("../src/launcher/result-budget.ts");
  const css = await read("src/styles/launcher.css");
  assert.equal(budget.ROW_HEIGHT_TWO_LINE, 42);
  assert.equal(budget.ROW_HEIGHT_COMPACT, 34);
  assert.equal(budget.LAUNCHER_STATUS_UNITS, 30);
  assert.match(css, /\.launcher-result \{[\s\S]*?height: calc\(var\(--u\) \* 42\);/);
  assert.match(css, /\.launcher-result--compact \{[\s\S]*?height: calc\(var\(--u\) \* 34\);/);
  assert.match(css, /\.launcher-status \{[\s\S]*?min-height: calc\(var\(--u\) \* 30\);/);
});
