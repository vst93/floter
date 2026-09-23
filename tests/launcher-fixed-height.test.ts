// R25 · the launcher's window is a fixed slab, not a rubber band.
//
// The user's report, verbatim: 「现在搜索页面输入进行过滤时页面整体有抖动的情况，不
// 像原生应用」. The mechanism was structural — every keystroke changed the row
// count, the row count was in `useLauncherHeight`'s dependency list, and the
// hook answered by measuring the card and calling `setSize` on the native
// window. R20's ResizeObserver then re-measured after each of those resizes,
// so one keystroke could resize the window two or three times. No native
// launcher does that: Raycast, Alfred, Spotlight and tinycast all keep a fixed
// window and scroll the list inside it, with the action bar pinned to the
// bottom edge.
//
// These tests pin the three halves of the fix:
//
//   1. the height is a **constant** derived from the same ten-row budget the
//      list is built from (`LAUNCHER_WINDOW_HEIGHT`), and the constant is the
//      sum of the sheet's own segments — not a number that happens to look
//      right today;
//   2. the **window** does not move: one `setSize` when the launcher opens, and
//      none for any number of keystrokes, tips or feedback rows after that;
//   3. the **card** fills that window (`height: 100%`) and its action bar is
//      pinned to the bottom edge, so a short result set leaves sunken panel
//      material above the bar instead of shrinking the window.
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

const root = new URL("../", import.meta.url);
const read = (path: string) => readFile(new URL(path, root), "utf8");

const stripComments = (css: string) => css.replace(/\/\*[\s\S]*?\*\//g, "");

const rule = (css: string, selector: string) => {
  const escaped = selector.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
  const match = new RegExp(`${escaped}\\s*\\{([^}]*)\\}`, "s").exec(css);
  assert.ok(match, `${selector} must exist`);
  return match![1];
};

// `calc(var(--u) * N)` → N. Every box in the launcher is written this way, so a
// literal pixel value here would be the bug, not the fix.
const units = (value: string, property: string) => {
  const match = /calc\(var\(--u\)\s*\*\s*(\d+(?:\.\d+)?)\)/.exec(value);
  assert.ok(match, `${property} must be expressed in units, got "${value}"`);
  return Number(match![1]);
};

test("the launcher's window height is the ten-row budget, segment by segment", async () => {
  const launcher = stripComments(await read("src/styles/launcher.css"));
  const budget = await import("../src/launcher/result-budget.ts");
  const {
    LAUNCHER_WINDOW_HEIGHT,
    LAUNCHER_WINDOW_HEIGHT_CHROME,
    LAUNCHER_WINDOW_HEIGHT_UNITS,
    MAX_RESULTS,
    RESULTS_LIST_CHROME,
    launcherWindowHeight,
  } = budget;

  // Every segment of the slab, read from the sheet that draws it. If a future
  // round moves one of these boxes, this sum moves with it and the constant has
  // to be re-derived — which is the point of asserting the sum rather than the
  // number.
  const inputRow = rule(launcher, ".collapsed-card__input-row");
  const field = units(/min-height:\s*([^;]+);/.exec(inputRow)![1], "input row min-height");
  const breath = units(/margin-bottom:\s*([^;]+);/.exec(inputRow)![1], "input row margin-bottom");
  const bottom = rule(launcher, ".launcher-bottom");
  const padding = /padding:\s*([^;]+);/.exec(bottom)![1];
  const insets = [...padding.matchAll(/calc\(var\(--u\)\s*\*\s*(\d+)\)/g)].map((m) => Number(m[1]));
  assert.equal(insets.length, 3, ".launcher-bottom's padding is three unit insets");
  const panelTop = insets[0];
  const panelBottom = insets[insets.length - 1];
  const list = rule(launcher, ".launcher-results");
  const ceiling = /max-height:\s*min\(\s*([^,]+),/.exec(list)![1].trim();
  const ceilingParts = /calc\(var\(--u\)\s*\*\s*(\d+)\s*\+\s*(\d+)px\)/.exec(ceiling);
  assert.ok(ceilingParts, `the ceiling must be units + fixed chrome, got "${ceiling}"`);
  const listUnits = Number(ceilingParts![1]);
  const listChrome = Number(ceilingParts![2]);
  const bar = rule(launcher, ".launcher-action-bar");
  const barGap = units(/margin-top:\s*([^;]+);/.exec(bar)![1], "action bar margin-top");
  const barHeight = units(/height:\s*([^;]+);/.exec(bar)![1], "action bar height");

  assert.equal(field, 42, "the field's row is R23's 42u");
  assert.equal(breath, 4, "R24's breath is 4u below the field…");
  assert.equal(panelTop, 4, "…plus the panel's own 4u inset");
  assert.equal(listUnits, 378, "the list's ceiling is R20's nine two-line rows");
  assert.equal(listChrome, RESULTS_LIST_CHROME, "with the module's own fixed chrome");
  assert.equal(barGap, 3, "the action bar keeps R18's constant 3u gap");
  assert.equal(barHeight, 42, "the action bar is a row");
  assert.equal(panelBottom, 2, "R21's tail");

  assert.equal(
    field + breath + panelTop + listUnits + barGap + barHeight + panelBottom,
    LAUNCHER_WINDOW_HEIGHT_UNITS,
    "the constant's unit part is the sum of the segments the sheet draws",
  );
  assert.equal(
    LAUNCHER_WINDOW_HEIGHT_CHROME,
    RESULTS_LIST_CHROME + 2,
    "the fixed part is the list's own chrome plus the card's 1px frame top and bottom",
  );
  assert.equal(LAUNCHER_WINDOW_HEIGHT, 527, "475u + 52px at the default interface step");

  // …and it clears the tallest card the sheets can actually produce. The list's
  // real chrome is 11px under its own 50px ceiling (4px band + nine 1px gaps +
  // a 26px section title), so the window never clips its card.
  const band = 4;
  const titleLine = 26;
  const worstCaseCard = LAUNCHER_WINDOW_HEIGHT_UNITS + band + MAX_RESULTS + titleLine + 2;
  assert.ok(
    LAUNCHER_WINDOW_HEIGHT >= worstCaseCard,
    `a ${LAUNCHER_WINDOW_HEIGHT}px window must hold the worst-case card (${worstCaseCard}px)`,
  );

  // The step multiplies the unit part only, and rounds up — a window a pixel
  // short of its card is a clipped card. The fixed chrome does not scale, which
  // is exactly how `calc(var(--u) * N + Mpx)` behaves in the sheet.
  assert.equal(launcherWindowHeight(1), LAUNCHER_WINDOW_HEIGHT);
  assert.equal(launcherWindowHeight(1.1), Math.ceil(475 * 1.1 + 52));
  assert.equal(launcherWindowHeight(1.25), Math.ceil(475 * 1.25 + 52));
  for (const scale of [1, 1.1, 1.25]) {
    assert.equal(
      launcherWindowHeight(scale),
      Math.ceil(LAUNCHER_WINDOW_HEIGHT_UNITS * scale) + LAUNCHER_WINDOW_HEIGHT_CHROME,
      "the chrome is added once, unscaled",
    );
  }
});

test("the App hands the band height to every collapsed sync, clamped to the display", async () => {
  const app = (await read("src/App.tsx")).replace(/\/\*[\s\S]*?\*\//g, "").replace(/\/\/.*$/gm, "");
  assert.match(
    app,
    /const launcherMaxHeight = Math\.min\(\s*launcherWindowHeight\(launcherScale\),\s*Math\.max\(240, window\.screen\.availHeight - 24\),\s*\);/,
    "the full slab is the budget at the current step, clamped to the work area",
  );
  assert.match(
    app,
    /const launcherHeldRows = resolveLauncherRows\(launcherRowsRef\.current, launcherRows\);/,
    "the row count is resolved from the raw count with the sticky hysteresis",
  );
  assert.match(
    app,
    /const launcherHeight = launcherRowHeight\(\s*launcherHeldRows,\s*launcherScale,\s*launcherMaxHeight,\s*launcherHasBar,\s*launcherSectionTitle,\s*launcherScope === "browser",\s*\);/,
    "the window height is the row count's, never above the full slab, with the bar, the section title and the browser filter charged only when they are drawn",
  );
  assert.match(
    app,
    /launcherHeightRef\.current = launcherHeight;/,
    "the once-registered listeners read the height through a ref, never a stale closure",
  );
  assert.match(
    app,
    /useLauncherHeight\(mode, collapsedCardRef, launcherHeight, \[/,
    "the hook is given the band's height, not a measurement",
  );
  assert.match(
    app,
    /syncLauncherHeight\(collapsedCardRef, launcherHeightRef\.current\)/,
    "every imperative collapsed sync asks for the same height",
  );
  assert.doesNotMatch(
    app,
    /syncLauncherHeight\(collapsedCardRef\)/,
    "no collapsed path may fall back to a bare, measured sync",
  );
  // The inline viewport cap the short-display path uses is untouched: it is a
  // *list* cap, and it still binds inside the (smaller) window.
  assert.match(
    app,
    /--launcher-results-height": `\$\{Math\.max\(84, window\.screen\.availHeight - RESULTS_VIEWPORT_CHROME\)\}px`/,
    "the short-display list cap is still written from the shared chrome constant",
  );
});

test("the card fills the window and pins its action bar to the bottom edge", async () => {
  const launcher = stripComments(await read("src/styles/launcher.css"));

  // The card is the window, not a floating card inside it. `min-height` would
  // leave a card shorter than its window in the empty state; `height` is the
  // slab, and the list inside it scrolls instead of the window shrinking.
  const card = rule(launcher, ".collapsed-card");
  assert.equal(
    /height:\s*100%;/.test(card),
    true,
    "the card is exactly the window's height (R25)",
  );
  assert.doesNotMatch(
    card,
    /min-height:\s*100%/,
    "and it is no longer `min-height`-ed into a window that followed its measurement",
  );
  assert.match(card, /overflow:\s*hidden/, "the slab clips rather than overflowing the window");

  // The panel below the field fills the rest of the card…
  const clip = rule(launcher, ".launcher-bottom-clip--open");
  assert.equal(
    /flex:\s*1 1 auto;/.test(clip),
    true,
    "the open clip takes the height the field and the breath leave behind",
  );
  const bottom = rule(launcher, ".launcher-bottom");
  assert.match(bottom, /display:\s*flex/, "the panel is a column");
  assert.match(bottom, /min-height:\s*0/, "…that may shrink below its content");

  // …and inside it the list is the flexible row while the action bar is pinned.
  const options = rule(launcher, ".launcher-options");
  assert.match(options, /display:\s*flex/, "the options column is a flex box");
  assert.match(
    options,
    /justify-content:\s*space-between/,
    "the free space is pushed between the list and the bar, so the bar sits on the bottom edge",
  );
  assert.match(options, /flex:\s*1 1 auto/, "and the column fills the panel");
  assert.match(options, /min-height:\s*0/, "with the list free to shrink and scroll");

  const list = rule(launcher, ".launcher-results");
  assert.match(list, /overflow-y:\s*auto/, "the list is the scroller");
  assert.match(list, /min-height:\s*0/, "and may shrink below its content");
  // The bar keeps its own constant 3u gap — it is now the *minimum* gap, with
  // the leftover height pushed above it by `space-between` (see the R25 note in
  // the sheet). It is still declared on the bar, so the pinned metric
  // `ui-scale.test.ts` reads is unchanged.
  const bar = rule(launcher, ".launcher-action-bar");
  assert.equal(units(/margin-top:\s*([^;]+);/.exec(bar)![1], "action bar margin-top"), 3);
  assert.match(
    bar,
    /flex:\s*0 0 auto/,
    "and the bar itself may not be squeezed — the pinned footer keeps its 42u",
  );
});

test("the platform's shell reservation is read from the sheet, never re-spelled", async () => {
  // Windows and Linux pad the shell so the card's shadow has room; macOS does
  // not. The window has to be taller than the card by exactly that padding, so
  // the hook reads it off the shell element. Spelling the numbers in TypeScript
  // would be a second source of truth for a sheet decision.
  const base = stripComments(await read("src/styles/base.css"));
  assert.match(
    base,
    /\.platform-windows \.collapsed-shell\s*\{[^}]*padding:/s,
    "the Windows shell reserves room for the card's shadow",
  );
  assert.match(
    base,
    /\.platform-linux \.collapsed-shell\s*\{[^}]*padding:/s,
    "…and so does the Linux one",
  );

  const hook = await read("src/hooks/useLauncherHeight.ts");
  assert.match(
    hook,
    /const shellStyle = getComputedStyle\(shell\);[\s\S]{0,200}parseFloat\(shellStyle\.paddingTop\)[\s\S]{0,120}parseFloat\(shellStyle\.paddingBottom\)/,
    "the reservation is the shell's own computed padding",
  );
  assert.match(
    hook,
    /const base = windowHeight \+ shellPaddingHeight\(card\);/,
    "the reservation rides on the target side, so the window still does not move for it",
  );
  assert.match(
    hook,
    /measured > current \+ 1/,
    "R26-D: the measurement is an overflow guard against the *current* window, not the budget",
  );
});

// The `setSize` payload shape the double receives: `LogicalSize` serializes as
// the `Logical` variant through `toJSON()` (see `ui-scale-steps.test.ts`).
type SetSizeArgs = {
  label: string;
  value: { toJSON(): { Logical: { width: number; height: number } } };
};

test("collapsed mode asks for the window height once, and typing never asks again", async () => {
  const globalWindow = globalThis as unknown as {
    window: unknown;
    getComputedStyle: (el: unknown) => Record<string, string>;
    requestAnimationFrame: (cb: () => void) => number;
  };
  const previousWindow = globalWindow.window;
  const previousGetComputedStyle = globalWindow.getComputedStyle;
  const previousRequestAnimationFrame = globalWindow.requestAnimationFrame;

  const sizes: number[] = [];
  const fakeWindow = {
    // The height `show_input` left behind: the bare input row. The launcher has
    // to ask for its slab on top of it.
    innerHeight: 58,
    __TAURI_INTERNALS__: {
      metadata: { currentWindow: { label: "main" } },
      invoke: async (cmd: string, args: unknown) => {
        if (cmd === "plugin:window|set_size") {
          const { height } = (args as SetSizeArgs).value.toJSON().Logical;
          sizes.push(height);
          // The platform lands the resize, and `window.innerHeight` is what the
          // guard reads — so the double has to move with it.
          fakeWindow.innerHeight = height;
        }
        return undefined;
      },
      transformCallback: () => 1,
      unregisterCallback: () => undefined,
    },
  };
  globalWindow.window = fakeWindow;
  const shell = { paddingTop: 4, paddingBottom: 12 };
  globalWindow.getComputedStyle = (el: unknown) =>
    el === shell
      ? // The Windows shell: 4u above the card, 12u below it for the shadow.
        {
          display: "block",
          borderTopWidth: "0px",
          borderBottomWidth: "0px",
          paddingTop: "4px",
          paddingBottom: "12px",
        }
      : {
          display: "block",
          borderTopWidth: "1px",
          borderBottomWidth: "1px",
          paddingTop: "0px",
          paddingBottom: "0px",
        };
  // The settle re-measure is scheduled for the next paint; stub the frame out so
  // it cannot run after the stubs are restored. The assertion below is about the
  // *asks*, and an unrun frame asks for nothing.
  globalWindow.requestAnimationFrame = () => 0;

  const tick = () => new Promise((resolve) => setTimeout(resolve, 5));
  const ref = (children: Array<{ offsetTop: number; offsetHeight: number }>) =>
    ({ current: { children, parentElement: null } }) as unknown as {
      current: HTMLDivElement | null;
    };
  // A card whose last laid-out child ends at this offset, plus the 2px frame.
  const cardOf = (contentHeight: number) => ref([{ offsetTop: 0, offsetHeight: contentHeight }]);

  try {
    const { syncLauncherHeight } = await import("../src/hooks/useLauncherHeight.ts");
    const { LAUNCHER_WINDOW_HEIGHT } = await import("../src/launcher/result-budget.ts");

    // ── the open ───────────────────────────────────────────────────────────
    // Three paths sync the launcher when it opens (the mode effect, the layout
    // effect and the reveal), and all three run inside the same keystroke-free
    // window. They are one native resize.
    syncLauncherHeight(cardOf(120), LAUNCHER_WINDOW_HEIGHT);
    syncLauncherHeight(cardOf(120), LAUNCHER_WINDOW_HEIGHT);
    syncLauncherHeight(cardOf(120), LAUNCHER_WINDOW_HEIGHT);
    await tick();
    assert.deepEqual(
      sizes,
      [LAUNCHER_WINDOW_HEIGHT],
      "opening the launcher resizes the window exactly once, to the constant",
    );

    // ── the typing ─────────────────────────────────────────────────────────
    // Nine queries, from an empty match set to a full nine-row list, plus the
    // two states that used to move the window on their own: the onboarding tip
    // arriving above the list and a feedback row appearing under it. Every one
    // of these changes the card's content; none of them may touch the window.
    for (const contentHeight of [120, 210, 300, 390, 440, 300, 210, 120, 440]) {
      syncLauncherHeight(cardOf(contentHeight), LAUNCHER_WINDOW_HEIGHT);
    }
    await tick();
    assert.deepEqual(
      sizes,
      [LAUNCHER_WINDOW_HEIGHT],
      "no keystroke, tip or feedback row resizes the window — the shake is gone",
    );

    // ── the guard does not wedge ───────────────────────────────────────────
    // Settings owns its own `setSize`, so a round trip through it leaves the
    // window at a height the launcher has to correct when it comes back.
    fakeWindow.innerHeight = 600;
    syncLauncherHeight(cardOf(300), LAUNCHER_WINDOW_HEIGHT);
    await tick();
    assert.deepEqual(
      sizes,
      [LAUNCHER_WINDOW_HEIGHT, LAUNCHER_WINDOW_HEIGHT],
      "a window resized behind the launcher's back is still corrected",
    );

    // ── the one case the constant cannot express ───────────────────────────
    // A card whose content genuinely outgrew the budget (a tip above a full
    // list, a font landing late) is raised to fit rather than clipped.
    syncLauncherHeight(cardOf(600), LAUNCHER_WINDOW_HEIGHT);
    await tick();
    assert.deepEqual(
      sizes,
      [LAUNCHER_WINDOW_HEIGHT, LAUNCHER_WINDOW_HEIGHT, 602],
      "a card taller than the budget is not clipped",
    );

    // ── the platform's shell reservation ───────────────────────────────────
    // Windows and Linux pad `.collapsed-shell` so the card's shadow has
    // somewhere to land, and that padding comes out of the window's height
    // before the card sees any of it. It is a constant of the sheet, so it is
    // added to the constant — the window does not move for it either.
    fakeWindow.innerHeight = 602;
    syncLauncherHeight(
      {
        current: { children: [{ offsetTop: 0, offsetHeight: 120 }], parentElement: shell },
      } as unknown as { current: HTMLDivElement | null },
      LAUNCHER_WINDOW_HEIGHT,
    );
    await tick();
    assert.deepEqual(
      sizes,
      [LAUNCHER_WINDOW_HEIGHT, LAUNCHER_WINDOW_HEIGHT, 602, LAUNCHER_WINDOW_HEIGHT + 16],
      "the window is the budget plus the shell's 4u + 12u reservation",
    );
  } finally {
    globalWindow.window = previousWindow;
    globalWindow.getComputedStyle = previousGetComputedStyle;
    globalWindow.requestAnimationFrame = previousRequestAnimationFrame;
  }
});

// R34 · the slab's sizes are now per-row.
//
// R26-D quantised the window into four bands (1 / 3 / 6 / 9 rows); the user's
// next report is that the quantisation itself is the problem — 「搜索页高度应该随
// 着选项列表高度自适应，现在最大高度就行了」. Four, five, seven and eight rows
// each landed in the next band up and left empty rows under the list. The fix
// keeps the band model's two properties (a keystroke never moves the window
// inside a row count; a boundary oscillation does not flap) and makes the row
// count the height.
test("R34 · the window height is per-row, and the row count is sticky", async () => {
  const budget = await import("../src/launcher/result-budget.ts");
  const {
    LAUNCHER_WINDOW_HEIGHT,
    clampLauncherRows,
    launcherRowHeight,
    launcherRowUnits,
    MAX_RESULTS,
    resolveLauncherRows,
  } = budget;

  // A count *is* the height: each row adds exactly one worst-case row, so four
  // rows are four rows tall instead of landing in a six-row band.
  for (let rows = 1; rows <= MAX_RESULTS; rows += 1) {
    assert.equal(clampLauncherRows(rows), rows);
    assert.ok(
      launcherRowUnits(rows) >= 97 + rows * 42,
      `${rows} rows must hold their rows without scrolling`,
    );
    if (rows > 1) {
      assert.equal(
        launcherRowUnits(rows) - launcherRowUnits(rows - 1),
        42,
        "each extra row adds exactly one worst-case row",
      );
    }
  }
  // The top is still the R25 slab.
  assert.equal(launcherRowUnits(MAX_RESULTS, true), 475);
  assert.equal(launcherRowHeight(MAX_RESULTS, 1, LAUNCHER_WINDOW_HEIGHT), LAUNCHER_WINDOW_HEIGHT);
  assert.equal(launcherRowHeight(MAX_RESULTS, 1, LAUNCHER_WINDOW_HEIGHT), 527);
  assert.ok(
    launcherRowHeight(8, 1, LAUNCHER_WINDOW_HEIGHT) <
      launcherRowHeight(MAX_RESULTS, 1, LAUNCHER_WINDOW_HEIGHT),
    "eight rows are genuinely shorter than nine",
  );

  // Growing is immediate; shrinking waits a row below the held count. The 1↔2
  // boundary the user named never oscillates.
  assert.equal(resolveLauncherRows(1, 4), 4, "4 rows opens four rows");
  assert.equal(resolveLauncherRows(4, 4), 4, "4 rows keeps four rows");
  assert.equal(resolveLauncherRows(4, 3), 4, "3 rows is absorbed (hysteresis)");
  assert.equal(resolveLauncherRows(4, 2), 2, "2 rows steps down");
  assert.equal(resolveLauncherRows(2, 1), 2, "1 row is absorbed at the 1↔2 edge");
  assert.equal(resolveLauncherRows(2, 2), 2, "2 rows keeps it");
  assert.equal(resolveLauncherRows(9, 1), 1, "a genuinely short list steps down");
  // Clamped at both ends.
  assert.equal(resolveLauncherRows(9, 40), MAX_RESULTS, "never taller than the slab");
  assert.equal(resolveLauncherRows(1, 0), 1, "never shorter than one row");

  // The typing session the fix is about: a one-row shrink does not move the
  // window, the same count never does, and a two-row drop moves it once.
  let held = resolveLauncherRows(1, 9);
  const moves: number[] = [];
  const step = (next: number) => {
    const resolved = resolveLauncherRows(held, next);
    if (resolved !== held) {
      held = resolved;
      moves.push(launcherRowHeight(held, 1, LAUNCHER_WINDOW_HEIGHT));
    }
  };
  for (const count of [9, 8, 9, 8]) step(count);
  assert.deepEqual(moves, [], "8↔9 never moves the window");
  step(7);
  assert.equal(moves.length, 1, "dropping two rows moves it once");
  // Growing back is a real content change and lands immediately; the
  // oscillation that follows it never moves again (the shrink is absorbed).
  step(8);
  assert.equal(moves.length, 2, "7→8 grows once");
  for (const count of [7, 8, 7, 8]) step(count);
  assert.equal(moves.length, 2, "7↔8 does not flap after the first growth");
});

// R27 · the height charges the action bar only when the bar is drawn.
//
// The user's third report on this area is 「当选项很少或者没有的时候，底部还是会
// 强制留出一段高度」. R26-D gave the window discrete sizes, but every band still
// reserved the action bar's 45u — and the bar is hidden in exactly the states
// the report is about: a query that matched nothing (no matched row, so no
// shell fallback) and both plugin modes (`useLauncherCatalog` returns no action
// bar inside them). A one-row launcher therefore sat in a window 45u taller
// than its content.
test("R27 · a row count charges the action bar only when the bar is drawn", async () => {
  const budget = await import("../src/launcher/result-budget.ts");
  const {
    LAUNCHER_ACTION_BAR_UNITS,
    LAUNCHER_WINDOW_HEIGHT,
    launcherRowChrome,
    launcherRowHeight,
    launcherRowUnits,
    MAX_RESULTS,
  } = budget;

  // The full slab is unchanged while the bar is drawn — that is R25's constant,
  // and the settings-panel/launcher-height round trip depends on it.
  assert.equal(launcherRowUnits(MAX_RESULTS, true), 475);
  assert.equal(launcherRowHeight(MAX_RESULTS, 1, LAUNCHER_WINDOW_HEIGHT), LAUNCHER_WINDOW_HEIGHT);
  assert.equal(launcherRowHeight(MAX_RESULTS, 1, LAUNCHER_WINDOW_HEIGHT), 527);

  // Without the bar, exactly the bar's own segment comes off — nothing else.
  for (let rows = 1; rows <= MAX_RESULTS; rows += 1) {
    assert.equal(
      launcherRowUnits(rows, false),
      launcherRowUnits(rows, true) - LAUNCHER_ACTION_BAR_UNITS,
      `${rows} rows without its bar is exactly one bar shorter`,
    );
  }

  // The one-row launcher is the empty launcher: field 42 + breath 4 + panel top
  // 4 + one 42u row + tail 2 = 94u, plus the 1px frame top and bottom and the
  // scroller's 4px reservation — no bar, no section title, no row gaps.
  assert.equal(launcherRowUnits(1, false), 94);
  assert.equal(launcherRowChrome(1), 2 + 4);
  assert.equal(launcherRowHeight(1, 1, LAUNCHER_WINDOW_HEIGHT, false), 100);
  // …and the empty-query section title is chrome, not a row.
  assert.equal(launcherRowChrome(1, true), 2 + 4 + 26);

  // A short count's chrome counts the gaps it really has; the nine-row top
  // keeps the R25 ceiling so the full slab stays 527px (a ceiling, not a
  // measurement).
  assert.equal(launcherRowChrome(2), 2 + 4 + 1);
  assert.equal(launcherRowChrome(3), 2 + 4 + 2);
  assert.equal(launcherRowChrome(MAX_RESULTS), 52);
});
