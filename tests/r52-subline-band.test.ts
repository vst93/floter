// R52 · the empty subline band on the ordinary search page is collapsed.
//
// The user's report, verbatim: 「现在输入框下方有一行多余的空白间隙，是 tab 进行
// 切换的公用板块吧？像这种没有切换的情况下应该隐藏」. The band the user points at
// is the chips row's slot: `.launcher-filter` (role=tablist) is only ever drawn
// inside a plugin scope (browser / clipboard / calculator), so on the ordinary
// search page there is no such row — yet the *drawn* gap under the field was
// still stacked as if one sat there:
//
//   field row bottom
//   + field row's `margin-bottom`          4u   (the field's own breath, kept)
//   + `.launcher-bottom`'s top inset       4u   (a subline's separation)
//   + `.launcher-results`' scroll-edge      4px (a scroller reservation)
//   + the section title's top padding      6u   (the title's own top, kept)
//
// The middle two exist only to separate a subline from the list, so when no
// subline is drawn `App.tsx` writes `.collapsed-card--no-subline` and the sheet
// zeroes them. The window height reads the *same* predicate (`filterRowVisible`)
// through `launcherContentHeight`, so the slab gives back exactly the 4u + 4px
// the sheet stopped drawing — no slack, no clip. With a chips row present every
// rule here is inert and the chips keep their exact geometry.
//
// Mutations that must turn this file red:
//   * the sheet rule dropping off (band comes back) -> "the sheet collapses";
//   * the App modifier dropping off -> "the App writes the modifier";
//   * `launcherContentHeight` charging the insets with no chips row
//     -> "the window gives back exactly the band";
//   * the chips row's own box moving -> "the chips keep their geometry".

import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

const root = new URL("../", import.meta.url);
const read = (path: string) => readFile(new URL(path, root), "utf8");
const stripCssComments = (css: string) => css.replace(/\/\*[\s\S]*?\*\//g, "");
const stripJsComments = (src: string) =>
  src.replace(/\/\*[\s\S]*?\*\//g, "").replace(/\/\/[^\n]*/g, "");

const rule = (css: string, selector: string) => {
  const escaped = selector.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
  const match = new RegExp(`${escaped}\\s*\\{([^}]*)\\}`, "s").exec(css);
  assert.ok(match, `${selector} must exist`);
  return match![1];
};

const units = (value: string, property: string) => {
  const match = /calc\(var\(--u\)\s*\*\s*(\d+(?:\.\d+)?)\)/.exec(value);
  assert.ok(match, `${property} must be expressed in units, got "${value}"`);
  return Number(match![1]);
};

// ── 1 · the sheet collapses the two subline insets ────────────────────────

test("R52 · the no-subline card drops the panel inset and the scroll-edge", async () => {
  const css = stripCssComments(await read("src/styles/launcher.css"));

  // The panel's 4u top inset — the piece that "completes the 8u breath" and
  // separates a subline from the list — collapses to zero when there is none.
  const bottom = rule(css, ".collapsed-card--no-subline .launcher-bottom");
  assert.match(bottom, /padding-top:\s*0;/, "the panel top inset collapses (was 4u)");
  assert.ok(
    !/padding:\s*calc/.test(bottom),
    "only the top collapses — the sides and the 2u tail are untouched",
  );

  // The scroller's scroll-edge reservation collapses with it.
  const results = rule(css, ".collapsed-card--no-subline .launcher-results");
  assert.match(results, /padding-top:\s*0;/, "the scroll-edge reservation collapses (was 4px)");

  // The base rules — the ones a *chips* page draws — are unchanged, so the band
  // is only ever removed, never re-derived.
  const baseBottom = rule(css, ".launcher-bottom");
  const basePadding = /padding:\s*([^;]+);/.exec(baseBottom)![1];
  assert.match(
    basePadding,
    /^calc\(var\(--u\)\s*\*\s*4\)/,
    "the panel keeps its 4u top inset",
  );
  assert.match(
    rule(css, ".launcher-results"),
    /padding:\s*var\(--scroll-edge\)\s+0\s+0;/,
    "the scroller keeps its scroll-edge top padding by default",
  );
});

test("R52 · the chips row itself does not move", async () => {
  const css = stripCssComments(await read("src/styles/launcher.css"));
  const filter = rule(css, ".launcher-filter");
  assert.equal(
    units(/min-height:\s*([^;]+);/.exec(filter)![1], "filter min-height"),
    24,
    "the chips row is still its 24u band",
  );
  assert.equal(
    units(/margin-bottom:\s*([^;]+);/.exec(filter)![1], "filter margin-bottom"),
    4,
    "and its 4u breath below",
  );
  // No rule anywhere in the no-subline block may target the chips row: the
  // three plugin modes are byte-identical to their pre-R52 geometry.
  for (const match of css.matchAll(/\.collapsed-card--no-subline\s+([^{]+)\{/g)) {
    assert.doesNotMatch(match[1], /launcher-filter/, "the chips row is out of scope for R52");
  }
});

// ── 2 · the App writes the modifier from the one predicate ────────────────

test("R52 · the App gates the modifier on the same `filterRowVisible` the height reads", async () => {
  const app = stripJsComments(await read("src/App.tsx"));
  // The modifier is emitted inline on the card's own class list.
  assert.match(
    app,
    /collapsed-card\$\{hasQuery \? " collapsed-card--filled" : ""\}\$\{filterRowVisible \? "" : " collapsed-card--no-subline"\}/,
    "the card carries the no-subline modifier exactly when no chips row is drawn",
  );
  // …and the window height reads the same predicate, so drawn and charged agree.
  assert.match(
    app,
    /const launcherHeight = launcherContentHeight\(\s*launcherHeldUnits,\s*launcherRows,\s*launcherScale,\s*launcherMaxHeight,\s*launcherHasBar,\s*launcherSectionTitle,\s*filterRowVisible,\s*\)/,
  );
});

// ── 3 · the window gives back exactly the band ────────────────────────────

test("R52 · a chips-free window gives back exactly 4u + 4px", async () => {
  const budget = await import("../src/launcher/result-budget.ts");
  const {
    LAUNCHER_FILTER_UNITS,
    LAUNCHER_SUBLINE_INSET_CHROME,
    LAUNCHER_SUBLINE_INSET_UNITS,
    LAUNCHER_WINDOW_HEIGHT,
    MAX_RESULTS,
    ROW_HEIGHT_COMPACT,
    ROW_HEIGHT_TWO_LINE,
    launcherContentHeight,
    launcherRowChrome,
  } = budget;

  // The insets are the sheet's own two numbers: the panel's 4u inset and the
  // scroller's 4px reservation.
  assert.equal(LAUNCHER_SUBLINE_INSET_UNITS, 4);
  assert.equal(LAUNCHER_SUBLINE_INSET_CHROME, 4);

  // The R52 scene (empty query, ten compact recent rows, the title) now goes
  // through R58's honest chrome: 484px with the chips row absent, 520px with it.
  // The 8px the no-chips page gives back is still exactly the two insets.
  const withChips = launcherContentHeight(
    10 * ROW_HEIGHT_COMPACT, MAX_RESULTS, 1, LAUNCHER_WINDOW_HEIGHT, true, true, true,
  );
  const noChips = launcherContentHeight(
    10 * ROW_HEIGHT_COMPACT, MAX_RESULTS, 1, LAUNCHER_WINDOW_HEIGHT, true, true, false,
  );
  assert.equal(noChips, 484, "the ordinary page's ten compact rows are 484px");
  assert.equal(withChips, 520, "the chips page's are 520px");
  assert.equal(
    withChips - noChips,
    LAUNCHER_FILTER_UNITS + LAUNCHER_SUBLINE_INSET_UNITS + LAUNCHER_SUBLINE_INSET_CHROME,
    "the difference is the chips band plus the two insets the bare page gives back",
  );

  // The ten two-line rows: 574px with a chips row, 538px without — both under
  // the R25 ceiling (583px), which is what a ceiling is for. R58's whole point
  // is that these are the sheets' own sums, not the ceiling's slack.
  const slabWithChips = launcherContentHeight(
    10 * ROW_HEIGHT_TWO_LINE, MAX_RESULTS, 1, LAUNCHER_WINDOW_HEIGHT, true, false, true,
  );
  const slabNoChips = launcherContentHeight(
    10 * ROW_HEIGHT_TWO_LINE, MAX_RESULTS, 1, LAUNCHER_WINDOW_HEIGHT, true, false, false,
  );
  assert.equal(slabWithChips, 574, "the chips page's ten two-line rows are 574px");
  assert.equal(slabNoChips, 538, "the ordinary page's are 538px");
  assert.equal(
    slabWithChips - slabNoChips,
    LAUNCHER_FILTER_UNITS + LAUNCHER_SUBLINE_INSET_UNITS + LAUNCHER_SUBLINE_INSET_CHROME,
    "and the two pages still differ by exactly the chips band plus the two insets",
  );
  assert.ok(
    slabWithChips < LAUNCHER_WINDOW_HEIGHT,
    "the worst drawn slab stays under the R25 ceiling, so the ceiling never binds on an ordinary display",
  );

  // The unit part scales and the fixed part is added once, with and without the
  // insets, at every interface step.
  for (const scale of [0.8, 0.9, 1, 1.1, 1.25]) {
    const chrome = launcherRowChrome(4, false);
    const chips = launcherContentHeight(100, 4, scale, 10_000, true, false, true);
    const bare = launcherContentHeight(100, 4, scale, 10_000, true, false, false);
    assert.equal(
      chips,
      Math.ceil((66 + 100 + 45 + LAUNCHER_FILTER_UNITS) * scale + chrome),
      `the chips height is ` +
        `(66u + list + 45u + 28u) × ${scale} + ${chrome}px`,
    );
    assert.equal(
      bare,
      Math.ceil(
        (66 - LAUNCHER_SUBLINE_INSET_UNITS + 100 + 45) * scale +
          chrome -
          LAUNCHER_SUBLINE_INSET_CHROME,
      ),
      `the bare height is (62u + list + 45u) × ${scale} + ${chrome - 4}px`,
    );
    assert.ok(bare < chips, `the bare page is shorter at ${scale}`);
  }
});

// ── 4 · the height still never exceeds the display cap ────────────────────

test("R52 · the display cap still binds on a short display", async () => {
  const budget = await import("../src/launcher/result-budget.ts");
  const { MAX_RESULTS, ROW_HEIGHT_TWO_LINE, launcherContentHeight } = budget;
  assert.equal(
    launcherContentHeight(10 * ROW_HEIGHT_TWO_LINE, MAX_RESULTS, 1, 400, true, true, false),
    400,
    "the cap wins with no chips row too",
  );
});
