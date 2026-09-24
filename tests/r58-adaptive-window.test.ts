// R58 · the window is the page it holds — at every count, and at every display.
//
// The user's report, verbatim: 「窗口列表的高度为什么没有自适应」, with a screenshot
// of a query matching ten applications plus the bottom shell row (「a — 在终端中
// 运行」): a band of empty sunken panel between the last row and the pinned action
// bar — the desktop reading through the glass as ghost lines — and the bar sitting
// on the window's bottom edge.
//
// The cause was the last second authority in the height chain. `launcherRowChrome`
// charged the R25 *ceiling* (`RESULTS_LIST_CHROME + 2` = 52px) for any count at the
// budget (10): 2px frame + the 4px scroll-edge + nine 1px gaps + the 26px
// empty-query heading + 11px of slack. A *query* page draws no heading, and with no
// chips row (R52) no scroll-edge either, so its list is 2 + 9 = 15px of chrome: the
// window was 37px taller than the page it held, and the leftover landed between the
// last row and the bar, where the eye reads it as 「空带」. Every shorter count was
// already honest (R43), which is why the band appeared at exactly ten matches.
//
// The 37px also broke the R43 hysteresis' own budget: the 9→10 step became
// 35u + 37px = 72u — more than the one worst-case row (42u) a shrink may absorb —
// so crossing the boundary jumped the window, and coming back down it held the
// band until the content fell a further 42u. One formula now covers every count:
// what the sheet draws, with the empty-query heading charged exactly when
// `sectionTitle` says the sheet draws it.
//
// The screenshots' numbers (measured in a headless Chromium at the window height
// the App asks for, 720×495, with the real sheets): the query page draws 458px of
// card — 2px frame + 56u field + 4u breath + ten 34u rows + nine 1px gaps + the 3u
// gap and 42u bar + the 2u tail — and the window is 458px. Before this round it was
// 495px with a 37px band (and a 40px gap above the bar, its own 3u margin included).
//
// Mutations that must turn this file red:
//   * restoring the `count >= MAX_RESULTS` ceiling in `launcherRowChrome`
//     -> "the ten-row chrome is the sheet's own tally" and every drawn-parity case;
//   * charging the section title when the query page does not draw it
//     -> "the empty-query heading is charged exactly when it is drawn";
//   * dropping `pendingSystemAction` from the clip's predicate -> the App-block test;
//   * making the window's ceiling independent of the list's again -> the cap test.

import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

const root = new URL("../", import.meta.url);
const read = (path: string) => readFile(new URL(path, root), "utf8");
const stripJsComments = (src: string) =>
  src.replace(/\/\*[\s\S]*?\*\//g, "").replace(/\/\/[^\n]*/g, "");

/**
 * What the sheets draw for a launcher page, in window pixels at the default
 * interface step. Every term names the rule that owns it; the module side of the
 * comparison is `launcherContentHeight` (see `launcher/result-budget.ts`).
 *
 *   `.collapsed-card`           1px border top + bottom          2
 *   `.collapsed-card__input-row` min-height (R37)               56
 *   …its `margin-bottom` (R24's breath)                          4
 *   `.launcher-filter`           min-height + margin (chips only) 28
 *   `.launcher-bottom`           padding-top 4u / -bottom 2u    4/2
 *   `.launcher-results`          padding-top: var(--scroll-edge) 4
 *   …its `gap` (nine gaps for ten rows)                          1
 *   `.launcher-section-title`    (empty query only)             26
 *   `.launcher-action-bar`       margin-top 3u + height 42u      45
 *
 * R52 · with no chips row the sheet writes `.collapsed-card--no-subline`, which
 * zeroes the panel's top inset and the scroller's scroll-edge — so those two terms
 * are drawn *and* charged only when a chips row exists.
 */
type Page = {
  rows: number;
  rowHeight: number;
  bar: boolean;
  title: boolean;
  chips: boolean;
};

const FRAME = 2;
const FIELD = 56;
const BREATH = 4;
const CHIPS = 24 + 4;
const PANEL_TOP = 4;
const PANEL_TAIL = 2;
const SCROLL_EDGE = 4;
const GAP = 1;
const TITLE = 26;
const BAR = 3 + 42;

const drawn = ({ rows, rowHeight, bar, title, chips }: Page): number =>
  FRAME +
  FIELD +
  BREATH +
  (chips ? CHIPS : 0) +
  (chips ? PANEL_TOP : 0) +
  (chips ? SCROLL_EDGE : 0) +
  (title ? TITLE : 0) +
  rows * rowHeight +
  Math.max(0, rows - 1) * GAP +
  (bar ? BAR : 0) +
  PANEL_TAIL;

// ── 1 · one chrome tally, at every count ──────────────────────────────────

test("R58 · the ten-row chrome is the sheet's own tally, not the R25 ceiling", async () => {
  const budget = await import("../src/launcher/result-budget.ts");
  const {
    LAUNCHER_SECTION_TITLE_CHROME,
    LAUNCHER_WINDOW_HEIGHT,
    LAUNCHER_WINDOW_HEIGHT_CHROME,
    MAX_RESULTS,
    RESULTS_LIST_CHROME,
    launcherRowChrome,
  } = budget;

  // The ceiling the module still exports is the *display* ceiling's part, and it
  // is a real ceiling: 2px frame + the 4px band + nine gaps + the title + 11px.
  assert.equal(RESULTS_LIST_CHROME, 50);
  assert.equal(LAUNCHER_WINDOW_HEIGHT_CHROME, 52);
  assert.equal(LAUNCHER_WINDOW_HEIGHT, 583);

  // The chrome is the same function at every count: frame + scroll-edge +
  // (count - 1) gaps, plus the heading only when the page draws one.
  for (let rows = 0; rows <= MAX_RESULTS + 2; rows += 1) {
    const count = Math.min(rows, MAX_RESULTS);
    assert.equal(
      launcherRowChrome(rows),
      FRAME + SCROLL_EDGE + Math.max(0, count - 1),
      `${rows} rows are charged the chrome the sheet draws for them`,
    );
    assert.equal(
      launcherRowChrome(rows, true) - launcherRowChrome(rows),
      LAUNCHER_SECTION_TITLE_CHROME,
      "the empty-query heading is the only chrome term the count cannot know",
    );
  }
  assert.equal(launcherRowChrome(MAX_RESULTS), 15, "ten rows of a query page draw 15px of chrome");

  // The ceiling is still a ceiling: the worst page the sheets can draw is 9px
  // under it, so it never binds on an ordinary display.
  const widest = drawn({ rows: MAX_RESULTS, rowHeight: 42, bar: true, title: false, chips: true });
  assert.equal(widest, 574);
  assert.ok(widest < LAUNCHER_WINDOW_HEIGHT, "the display ceiling clears the widest page");
});

// ── 2 · the window is the page, at every count and in every state ─────────

test("R58 · the window height is the page's own height, never a band", async () => {
  const budget = await import("../src/launcher/result-budget.ts");
  const { LAUNCHER_STATUS_UNITS, LAUNCHER_WINDOW_HEIGHT, MAX_RESULTS, launcherContentHeight } = budget;

  const heightOf = (page: Page, cap = LAUNCHER_WINDOW_HEIGHT) =>
    launcherContentHeight(
      page.rows * page.rowHeight,
      page.rows,
      1,
      cap,
      page.bar,
      page.title,
      page.chips,
    );

  // The screenshot scene: query `a`, ten compact applications, the shell row.
  const screenshot: Page = { rows: 10, rowHeight: 34, bar: true, title: false, chips: false };
  assert.equal(drawn(screenshot), 458, "the sheets draw a 458px card");
  assert.equal(heightOf(screenshot), 458, "…and the window is that card");

  // The empty launcher's own state (the title, no bar) and every count around
  // the budget, with and without the bar: the two numbers are one number.
  const pages: Page[] = [];
  for (let rows = 1; rows <= MAX_RESULTS; rows += 1) {
    for (const bar of [true, false]) {
      for (const rowHeight of [34, 42]) {
        pages.push({ rows, rowHeight, bar, title: false, chips: false });
      }
    }
  }
  pages.push({ rows: 10, rowHeight: 34, bar: false, title: true, chips: false });
  pages.push({ rows: 3, rowHeight: 34, bar: true, title: false, chips: true });
  pages.push({ rows: 10, rowHeight: 42, bar: true, title: false, chips: true });
  pages.push({ rows: 1, rowHeight: LAUNCHER_STATUS_UNITS, bar: true, title: false, chips: false });
  // …and the R52 report's scene (a banner row above the plugin rows) still holds.
  pages.push({ rows: 4, rowHeight: 42, bar: false, title: false, chips: true });

  for (const page of pages) {
    assert.equal(
      heightOf(page),
      drawn(page),
      `a ${page.rows}-row page (${page.rowHeight}u rows, bar ${page.bar}, title ${page.title}, chips ${page.chips}) draws ${drawn(page)}px`,
    );
  }
});

// ── 3 · the 9→10 boundary stays inside the hysteresis' own budget ─────────

test("R58 · crossing the ten-row budget is one row, so it cannot jump the window", async () => {
  const budget = await import("../src/launcher/result-budget.ts");
  const {
    LAUNCHER_WINDOW_HEIGHT,
    MAX_RESULTS,
    ROW_HEIGHT_COMPACT,
    ROW_HEIGHT_TWO_LINE,
    launcherContentHeight,
    resolveLauncherUnits,
  } = budget;

  const windowAt = (rows: number) =>
    launcherContentHeight(rows * ROW_HEIGHT_COMPACT, rows, 1, LAUNCHER_WINDOW_HEIGHT, true, false, false);

  const step = windowAt(MAX_RESULTS) - windowAt(MAX_RESULTS - 1);
  assert.equal(
    step,
    ROW_HEIGHT_COMPACT + 1,
    "the tenth row costs one compact row and the 1px grid gap it brings",
  );
  assert.ok(
    step <= ROW_HEIGHT_TWO_LINE,
    `a ${step}u step must fit the hysteresis' ${ROW_HEIGHT_TWO_LINE}u worst-case row, or crossing the boundary moves the window`,
  );

  // The pre-R58 numbers, for the record: the ceiling's 37px made it 72u, which is
  // why the shrink was never absorbed and the band stayed held.
  const heldAtNine = resolveLauncherUnits(9 * ROW_HEIGHT_COMPACT, 9 * ROW_HEIGHT_COMPACT);
  const heldAfterTen = resolveLauncherUnits(heldAtNine, 10 * ROW_HEIGHT_COMPACT);
  const heldBackAtNine = resolveLauncherUnits(heldAfterTen, 9 * ROW_HEIGHT_COMPACT);
  assert.equal(heldAfterTen, 10 * ROW_HEIGHT_COMPACT, "the tenth row grows the held total immediately");
  assert.equal(
    heldBackAtNine,
    heldAfterTen,
    "…and dropping it is absorbed — the one-row hysteresis, unchanged",
  );
  assert.equal(
    launcherContentHeight(heldAtNine, MAX_RESULTS - 1, 1, LAUNCHER_WINDOW_HEIGHT, true, false, false),
    windowAt(MAX_RESULTS - 1),
  );
  // The held total is at most one worst-case row above the content, so the
  // absorber can never leave a band wider than the row it is designed to hold.
  assert.ok(heldBackAtNine - 9 * ROW_HEIGHT_COMPACT <= ROW_HEIGHT_TWO_LINE);
});

// ── 4 · the list's ceiling and the window's ceiling are one number ────────

test("R58 · a capped list and a capped window cannot disagree", async () => {
  const budget = await import("../src/launcher/result-budget.ts");
  const {
    LAUNCHER_WINDOW_HEIGHT,
    RESULTS_VIEWPORT_CHROME,
    launcherChromeHeight,
    launcherContentHeight,
    launcherResultsCeiling,
    launcherWindowCap,
  } = budget;

  // The list's ceiling is the display's: what the App writes into
  // `--launcher-results-height`. It binds only on a work area under
  // `RESULTS_VIEWPORT_CHROME + the worst list` — an ordinary display clears it.
  assert.equal(RESULTS_VIEWPORT_CHROME, 226);
  assert.equal(launcherResultsCeiling(800), 574);
  assert.equal(launcherResultsCeiling(500), 274);
  assert.equal(launcherResultsCeiling(120), 84, "never under one row's worth of list");

  // The window's ceiling is the list's ceiling plus this state's chrome, and the
  // R25 slab still wins when it is smaller — an ordinary display, unchanged.
  const chrome = launcherChromeHeight(1, true, false, false);
  assert.equal(chrome, 109, "field 56 + breath 4 + bar 45 + tail 2 + frame 2");
  assert.equal(launcherWindowCap(LAUNCHER_WINDOW_HEIGHT, 574, chrome), LAUNCHER_WINDOW_HEIGHT);

  // On a display where the list's ceiling binds, the window is exactly the
  // ceiling plus the chrome, so the list fills the panel edge to edge: no band,
  // and the list scrolls instead of the card overflowing.
  const shortCap = launcherWindowCap(LAUNCHER_WINDOW_HEIGHT, launcherResultsCeiling(500), chrome);
  assert.equal(shortCap, 274 + 109);
  const cappedWindow = launcherContentHeight(10 * 34, 10, 1, shortCap, true, false, false);
  assert.equal(cappedWindow, shortCap, "a capped window is its cap, never shorter");
  assert.equal(
    cappedWindow - chrome,
    274,
    "…which leaves the list exactly its own ceiling — the two numbers are one decision",
  );
  // The band the old pair left: the window capped itself at `availHeight - 24`
  // (476 here) while the list capped itself at 274, so 93px of panel sat empty
  // below a list that was already cutting its last row.
  assert.equal((500 - 24) - (274 + chrome), 93, "the pre-R58 disagreement, in pixels");
});

// ── 5 · the App's own wiring ─────────────────────────────────────────────

test("R58 · the App derives the window ceiling from the list's, and one predicate opens the clip", async () => {
  const app = stripJsComments(await read("src/App.tsx"));

  // One ceiling helper, one cap derivation: a capped list and a capped window are
  // the same decision, and the CSS var the sheet reads is written from it.
  assert.match(
    app,
    /const launcherListCeiling = launcherResultsCeiling\(window\.screen\.availHeight\);/,
    "the list's ceiling comes from the module",
  );
  assert.match(
    app,
    /--launcher-results-height": `\$\{launcherListCeiling\}px`/,
    "…and is what the sheet's scroller caps itself at",
  );
  assert.match(
    app,
    /const launcherMaxHeight = launcherWindowCap\(\s*launcherWindowHeight\(launcherScale\),\s*launcherListCeiling,\s*launcherChromeHeight\(launcherScale, launcherHasBar, launcherSectionTitle, filterRowVisible\),\s*\);/,
    "the window's ceiling is the list's plus this state's chrome",
  );

  // The alert row (feedback, scan failure, restart/shutdown confirmation) is one
  // boolean for both the charge and the sheet: R52 closed this hole for plugin
  // scopes, and the confirmation banner was the third branch — charged as a row
  // while the clip's own predicate could leave it closed.
  assert.match(
    app,
    /const launcherAlertRow = Boolean\(\s*launcherFeedback \|\| \(appsError && !launcherScope\) \|\| pendingSystemAction,\s*\);/,
    "the alert row is declared once",
  );
  assert.match(
    app,
    /const launcherChromeRows =\s*\(showOnboardingTip && !launcherScope \? 1 : 0\) \+ \(launcherAlertRow \? 1 : 0\);/,
    "…and is what the chrome row is charged for",
  );
  assert.match(
    app,
    /displayedResults\.length > 0 \|\| pluginText !== null \|\| launcherAlertRow\s*\?\s*"launcher-bottom-clip launcher-bottom-clip--open"/,
    "…and the same boolean is what opens the clip that draws it",
  );

  // The height still reads the drawn/chrome predicates, unchanged by this round.
  assert.match(
    app,
    /const launcherHeight = launcherContentHeight\(\s*launcherHeldUnits,\s*launcherRows,\s*launcherScale,\s*launcherMaxHeight,\s*launcherHasBar,\s*launcherSectionTitle,\s*filterRowVisible,\s*\);/,
    "one predicate drives the drawing and the billing (R52's discipline)",
  );
});
