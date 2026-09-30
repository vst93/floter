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
// R72 · the chips band: the 22u row (its underline is gone, so is the 2u the
// chip spent on it) plus the 2u breath below it. `LAUNCHER_FILTER_UNITS` is the
// module's twin of this number.
const CHIPS = 18 + 2;
const PANEL_TOP = 4;
const PANEL_TAIL = 2;
const SCROLL_EDGE = 4;
const GAP = 1;
const TITLE = 26;
// R69 · with no chips row the field's breath and the two insets below it all
// collapse, and the list draws this much of its own top padding instead.
const LIST_TOP = 6;

/** What the sheets draw for a card whose field is followed by an optional chip
 *  row, `heights` of result rows and an optional section heading. `open: false`
 *  is the collapsed clip: the field band and the card's frame, and nothing else
 *  — the breath is the panel's separator and goes with it (R69). */
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
  if (!open) return FRAME + FIELD;
  const rows = heights.length;
  return (
    FRAME +
    FIELD +
    (chips ? BREATH : LIST_TOP) +
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
  assert.equal(drawn({ open: true, heights }), 240, "the sheets draw a five-row compact list at 240px");
  assert.equal(after, 240, "…and the window is that card");
  assert.equal(before, 280, "the 42u-per-row billing was 40px taller");

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

test("R66/R69 · a collapsed panel is the field band, not a phantom row", async () => {
  const { launcherClosedHeight, launcherContentHeight, ROW_HEIGHT_COMPACT } = await import(
    "../src/launcher/result-budget.ts"
  );

  const collapsed = launcherClosedHeight(1);
  const drawnCard = drawn({ open: false });
  // R69 · the collapsed card is the 56u field and the 2px frame, and *nothing*
  // else: the breath under the field was the band the user kept reading on an
  // empty page (「输入框下方还是有个对应的空隙」), so it goes with the panel it
  // separated the field from, and the window is exactly that card.
  assert.equal(drawnCard, 58, "the collapsed card draws the field band and the frame");
  assert.equal(collapsed, drawnCard, "…and the window is that card");
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
  assert.equal(oldPhantom, 100, "the phantom-row window was 100px over a 58px card");
  assert.equal(oldWithHeading, 126, "…and the un-drawn heading made the empty field 126px");
});

// ── 3 · the App wiring ────────────────────────────────────────────────────

test("R66/R68 · the window is the content's height, and the change is a short walk", async () => {
  const app = stripJsComments(await read("src/App.tsx"));
  const budget = await import("../src/launcher/result-budget.ts");

  // The window is the exact total at every instant: no absorber, no settle, no
  // held number. (The walk below animates *toward* that total; it never parks
  // anywhere else.)
  assert.match(
    app,
    /const launcherHeldUnits = launcherHasContent \? launcherListUnitsRaw : 0;/,
    "the height is the content's own unit total",
  );
  assert.doesNotMatch(
    app,
    /launcherSettled|resolveLauncherUnits|launcherUnitsRef/,
    "no held-height state is left in the App",
  );
  assert.equal(budget.resolveLauncherUnits, undefined, "the absorber is retired");

  // R68 · the edge walks to the target over one short, named beat, with the
  // flicker sources dealt with: the walk is ordered (each `setSize` lands before
  // the next is painted), the observer yields to it, the card is marked so the
  // sheets freeze the material and hide the scrollers' bars, and `src-tauri`
  // debounces the macOS shadow.
  const hook = stripJsComments(await read("src/hooks/useLauncherHeight.ts"));
  assert.match(hook, /export const RESIZE_TWEEN_MS = \d+;/, "one named beat");
  assert.match(hook, /requestAnimationFrame\(step\)/, "painted per frame");
  assert.match(hook, /1 - \(1 - t\) \*\* 3/, "with an ease-out curve so it lands softly");
  assert.match(hook, /const RESIZING_CLASS = "launcher-resizing";/, "the card is marked for the walk");
  assert.match(hook, /if \(tweenFrame\) return;/, "the observer yields to the walk in flight");
  assert.match(hook, /resizeLauncherWindow\(card, height, SETTLE_PASSES\)/, "the bounded settle chain stays");
  assert.match(
    hook,
    /const from = Number\.isFinite\(tweenHeight\) \? tweenHeight : currentWindowHeight\(\);/,
    "a retarget starts from the edge's real height",
  );

  const css = stripJsComments(await read("src/styles/launcher.css"));
  assert.match(
    css,
    /\.launcher-resizing \.launcher-results,[\s\S]{0,120}\.launcher-resizing \.launcher-plugin-text \{[\s\S]{0,80}overflow-y: hidden;/,
    "the walk hides the scrollers' bars",
  );
  // …but it does *not* touch the material. An earlier pass swapped the card's
  // blur for `--surface-opaque` for the walk's duration, and the input row —
  // `--glass-field` over `--surface-opaque`, so a hair translucent — showed the
  // swap as a flicker on its own background (the user's 「输入框部分背景透明会闪烁」).
  // The glass is constant through the walk; the Rust shadow debounce and the
  // ordered resizes are what keep it still.
  assert.doesNotMatch(css, /backdrop-filter:\s*none/, "the walk does not drop the blur");
  assert.doesNotMatch(
    css,
    /\.collapsed-card\.launcher-resizing/,
    "the card's own material is not swapped for the walk",
  );
  // The shadow churn that made the first walk flicker is handled on the Rust
  // side now: one refresh after a resize burst, not one per frame.
  const rust = await read("src-tauri/src/lib.rs");
  assert.match(rust, /static SHADOW_GENERATION: AtomicU64/, "the shadow refresh is debounced");
  assert.match(rust, /refresh_macos_shadow_debounced\(&shadow_window\)/, "…and the resize handler uses it");
});

// ── 4 · the walk itself ─────────────────────────────────────────────────────

test("R68 · the resize walks the edge to the target instead of teleporting", async () => {
  const globals = globalThis as unknown as Record<string, unknown>;
  const saved = {
    window: globals.window,
    document: globals.document,
    getComputedStyle: globals.getComputedStyle,
    requestAnimationFrame: globals.requestAnimationFrame,
    cancelAnimationFrame: globals.cancelAnimationFrame,
    performance: globals.performance,
  };

  const sizes: number[] = [];
  const fakeWindow = {
    innerHeight: 300,
    __TAURI_INTERNALS__: {
      metadata: { currentWindow: { label: "main" } },
      invoke: async (cmd: string, args: unknown) => {
        if (cmd === "plugin:window|set_size") {
          const { height } = (args as { value: { toJSON(): { Logical: { height: number } } } })
            .value.toJSON().Logical;
          sizes.push(height);
          fakeWindow.innerHeight = height;
        }
        return undefined;
      },
      transformCallback: () => 1,
      unregisterCallback: () => undefined,
    },
  };
  let clock = 0;
  globals.window = fakeWindow;
  // The walk needs a document to paint in; without one (every other test in this
  // suite) the resize lands in one call, which is why those tests still see one
  // `setSize` per target.
  globals.document = {};
  globals.performance = { now: () => clock };
  globals.cancelAnimationFrame = () => undefined;
  globals.requestAnimationFrame = (cb: () => void) => {
    clock += 40; // four painted frames across the 150ms beat
    cb();
    return 1;
  };
  globals.getComputedStyle = () => ({
    display: "block",
    borderTopWidth: "0px",
    borderBottomWidth: "0px",
    paddingTop: "0px",
    paddingBottom: "0px",
  });

  try {
    const { syncLauncherHeight, RESIZE_TWEEN_MS } = await import(
      "../src/hooks/useLauncherHeight.ts"
    );
    assert.ok(RESIZE_TWEEN_MS > 0 && RESIZE_TWEEN_MS <= 300, "one short visual beat");
    syncLauncherHeight(
      {
        current: {
          children: [{ offsetTop: 0, offsetHeight: 20 }],
          parentElement: null,
          classList: { toggle: () => undefined },
        },
      } as unknown as { current: HTMLDivElement | null },
      100,
    );
    await new Promise((resolve) => setTimeout(resolve, 5));

    assert.ok(sizes.length >= 3, `a walk, not a jump: ${sizes.join(", ")}`);
    assert.ok(sizes[0] > 100 && sizes[0] < 300, "the first step is between the old and new heights");
    assert.equal(sizes[sizes.length - 1], 100, "…and the walk lands exactly on the target");
    for (let i = 1; i < sizes.length; i += 1) {
      assert.ok(sizes[i] <= sizes[i - 1], "a shrink never moves back up");
      assert.ok(sizes[i] >= 100, "and never overshoots the target");
    }
  } finally {
    Object.assign(globals, saved);
  }
});

test("R68 · the numbered slots are not renumbered from a mid-walk viewport", async () => {
  // The walk moves the scroller's box every frame. The numbered `⌘N` slots
  // follow the viewport report, so measuring mid-walk renumbered the list — and
  // the key map the handler reads — while the edge moved, and a numbered
  // shortcut could land on nothing (the user's 「列表项的快捷选择失效」). The
  // measure now waits the walk out and reads the still box.
  const row = stripJsComments(await read("src/launcher/LauncherResults.tsx"));
  assert.match(
    row,
    /if \(list\.closest\("\.launcher-resizing"\)\) \{[\s\S]{0,600}?return;/,
    "the visible-range measure waits the walk out",
  );
  assert.match(
    row,
    /if \(!visibleReportFrame\.current\) armMeasureBeat\(measureVisibleRows\);/,
    "…by retrying it once the walk has ended",
  );
});

test("R69 · an empty page with nothing below the field is the field band alone", async () => {
  // 56u of field + the card's 2px frame; no breath, no panel tail.
  const budget = await import("../src/launcher/result-budget.ts");
  assert.equal(budget.launcherClosedHeight(1), 58);
  assert.equal(budget.launcherClosedHeight(0.9), 53, "the unit part scales, the frame does not");

  const app = stripJsComments(await read("src/App.tsx"));
  assert.match(
    app,
    /!launcherHasContent\s*\?\s*launcherClosedHeight\(launcherScale\)/,
    "the App takes the closed branch when nothing is below the field, and charges the field band alone",
  );
  assert.match(
    app,
    /launcherHasContent \? "" : " collapsed-card--panel-closed"/,
    "…and tells the sheet the same thing",
  );

  const css = stripJsComments(await read("src/styles/launcher.css"));
  assert.match(
    css,
    /\.collapsed-card--panel-closed \.collapsed-card__input-row \{\s*margin-bottom: 0;/,
    "the field's breath goes with the panel it separated the field from",
  );
});
