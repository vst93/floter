// R52 · a feedback row is charged as a row in *every* scope, plugin scopes
// included.
//
// The user's report, verbatim: 「这类提示会造成显示bug」, with a screenshot of the
// calculator scope after Enter on an invalid expression (`adsad`): the red
// 「表达式无效」 banner appears under the chips, but the history rows read as
// ghosts through the translucent banner and the last row is cut by the window's
// bottom edge (only half of it shows).
//
// The cause is a single missing term. `launcherRows` already counted the
// feedback row (`launcherChromeRows`), but the plugin branch of
// `launcherListUnitsRaw` priced only `pluginViewRows(pluginView)` — the
// chrome row was counted and never charged. The window was therefore
// `42u` of glass short of its own content; `.launcher-bottom` overflowed, the
// scroller's last row was clipped by the card's bottom edge, and the
// translucent warning banner over a list that was still painting read as ghost
// text. The ordinary page had added `launcherChromeRows` all along; the plugin
// branch now does too, through the one `launcherChromeUnits` term.
//
// The numbers, for `N` plugin result rows and a filter (chips) row, at the
// default step:
//
//   drawn content (from the sheets)
//     2px frame + 56u field + 4u margin + 24u chips + 4u chips breath
//     + 4u panel top + 4px scroll-edge + 30u feedback + N×42u
//     + (N-1) 1px gaps + 2u panel tail
//     = 129 + 43N
//
//   window with the chrome row charged (the fix)
//     (66u + (N+1)×42u + 28u) + launcherRowChrome(N+1)
//     = 142 + 43N   →  13px of slack, never a clip
//
//   window with it missing (the bug)
//     (66u + N×42u + 28u) + launcherRowChrome(N+1)
//     = 100 + 43N  →  29px short of the content it must hold
//
// R58 · the slack in the second line is the R25 ceiling's (and is gone for the
// ordinary page, which now charges the chrome the sheet draws at every count —
// see `tests/r58-adaptive-window.test.ts`). What this file pins is unaffected:
// the charged term exists in *every* scope, the window is never shorter than its
// own content, and the missing term is what the report saw.
//
// Mutations that must turn this file red:
//   * dropping `+ launcherChromeUnits` from the pluginView branch of the App
//     -> "the plugin branch charges the chrome row";
//   * charging the feedback row only on the ordinary page -> the arithmetic
//     assertions below;
//   * a future edit that makes the window shorter than its own content
//     -> "the window never clips".

import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

const root = new URL("../", import.meta.url);
const read = (path: string) => readFile(new URL(path, root), "utf8");
const stripJsComments = (src: string) =>
  src.replace(/\/\*[\s\S]*?\*\//g, "").replace(/\/\/[^\n]*/g, "");

// ── 1 · the App charges the chrome row on the plugin path ─────────────────

test("R52 · the plugin branch charges the chrome row like the ordinary one", async () => {
  const app = stripJsComments(await read("src/App.tsx"));

  // One term, so the two branches cannot drift apart again.
  assert.match(
    app,
    /const launcherChromeUnits = launcherChromeRows \* ROW_HEIGHT_TWO_LINE;/,
    "the chrome row is priced once",
  );
  assert.match(
    app,
    /pluginView\s*\?\s*pluginViewRows\(pluginView\) \* ROW_HEIGHT_TWO_LINE \+ launcherChromeUnits/,
    "a plugin scope's list units include the feedback / tip row",
  );
  assert.match(
    app,
    /launcherListUnits\(displayedResults\.map\(launcherRowHeightUnits\)\) \+ launcherChromeUnits/,
    "…and the ordinary page keeps the same term",
  );
  // The config overlay takes the list's place and draws no feedback row, so it
  // deliberately stays without the term.
  assert.match(
    app,
    /pluginConfigOpen && launcherPluginId\s*\?\s*\(1 \+ \(pluginConfigSchema\(launcherPluginId\)\?\.fields\.length \?\? 0\)\) \* ROW_HEIGHT_TWO_LINE/,
    "the config overlay's rows are its own budget",
  );
});

// ── 2 · the arithmetic: window ≥ content, in scope with a banner ──────────

test("R52 · a plugin scope with feedback is never shorter than its content", async () => {
  const budget = await import("../src/launcher/result-budget.ts");
  const {
    LAUNCHER_FILTER_UNITS,
    LAUNCHER_STATUS_UNITS,
    LAUNCHER_WINDOW_HEIGHT,
    ROW_HEIGHT_TWO_LINE,
    launcherContentHeight,
    launcherRowChrome,
  } = budget;

  // The pieces the sheets draw, in units, that are *not* the chrome
  // `launcherRowChrome` owns: the 66u the height already folds in (field +
  // margin + panel top + tail), the chips row, the feedback band, and the rows.
  // `launcherRowChrome` adds the 2px frame, the 4px scroll-edge and the gaps.
  const drawnUnitsFor = (rows: number) =>
    66 /* field + margin + panel top + tail */ +
    LAUNCHER_FILTER_UNITS +
    LAUNCHER_STATUS_UNITS +
    rows * ROW_HEIGHT_TWO_LINE;

  for (const rows of [0, 1, 2, 3, 5, 9]) {
    const chrome = launcherRowChrome(rows + 1, false);
    // What the sheets actually draw: the chrome row (feedback) is a 30u band,
    // not a 42u result row, so the honest content is the drawn units plus the
    // frame, the scroller's scroll-edge and the real (rows - 1) result gaps.
    const content = Math.ceil(drawnUnitsFor(rows) + 2 + 4 + Math.max(0, rows - 1));
    // The window the App computes: `pluginViewRows × 42u` plus the chrome row
    // charged at the worst-case row height, through `launcherContentHeight`.
    const window = launcherContentHeight(
      (rows + 1) * ROW_HEIGHT_TWO_LINE,
      rows + 1,
      1,
      LAUNCHER_WINDOW_HEIGHT,
      false,
      false,
      true,
    );
    assert.ok(
      window >= content,
      `a plugin scope with ${rows} rows and a banner must hold ${content}px, got ${window}px`,
    );
  }

  // The three-row screenshot scene, spelled out: 258px of content, 271px window.
  const rows = 3;
  const content = Math.ceil(drawnUnitsFor(rows) + 2 + 4 + (rows - 1));
  const window = launcherContentHeight(
    (rows + 1) * ROW_HEIGHT_TWO_LINE,
    rows + 1,
    1,
    LAUNCHER_WINDOW_HEIGHT,
    false,
    false,
    true,
  );
  assert.equal(content, 258, "the calculator scene draws 258px");
  assert.equal(window, 271, "…and the window is the 271px slab that holds it");

  // The old formula — only the plugin's own rows — is what the report saw: a
  // window 29px short, which is the clipped row and the ghost band.
  const buggy = launcherContentHeight(
    rows * ROW_HEIGHT_TWO_LINE,
    rows + 1,
    1,
    LAUNCHER_WINDOW_HEIGHT,
    false,
    false,
    true,
  );
  assert.equal(buggy, 229, "the pre-R52 window was 229px");
  assert.ok(buggy < content, "…which is 29px short of the 258px it had to hold");
});

// ── 3 · the ordinary page's feedback accounting is untouched ──────────────

test("R52 · the ordinary page's feedback row still costs one worst-case row", async () => {
  const budget = await import("../src/launcher/result-budget.ts");
  const {
    LAUNCHER_STATUS_UNITS,
    LAUNCHER_WINDOW_HEIGHT,
    ROW_HEIGHT_COMPACT,
    ROW_HEIGHT_TWO_LINE,
    launcherContentHeight,
  } = budget;

  const plain = launcherContentHeight(
    3 * ROW_HEIGHT_COMPACT,
    3,
    1,
    LAUNCHER_WINDOW_HEIGHT,
    false,
    false,
    false,
  );
  const withFeedback = launcherContentHeight(
    3 * ROW_HEIGHT_COMPACT + ROW_HEIGHT_TWO_LINE,
    4,
    1,
    LAUNCHER_WINDOW_HEIGHT,
    false,
    false,
    false,
  );
  // The feedback row is charged as a 42u row (R43's worst-case chrome row), not
  // as its drawn 30u: a font landing late must not clip the banner.
  assert.equal(
    withFeedback - plain,
    ROW_HEIGHT_TWO_LINE + 1,
    "the banner adds one worst-case row plus the grid gap it counts",
  );
  assert.ok(
    withFeedback >= plain + LAUNCHER_STATUS_UNITS,
    "the window always covers the 30u banner it draws",
  );
});
