// R21 · the launcher's pointer and keyboard share one selection, and the field
// they are typed into is a surface.
//
// Three user reports, two of them the same root cause:
//
//   1. 「输入框部分高度还是不正常」 — the field's box was 56u all along (R20 held);
//      what was wrong was that `--glass-field` alone is a *wash* (50% white in
//      the light palette, 3% in the dark one), so the window behind the launcher
//      read through the field as a blurred second line of text under the query.
//      A surface that shows a ghost under the caret reads as a taller field.
//   2. 「底部空白太大」 — the panel's bottom inset counted the last row's own
//      half-leading twice: 4u of padding *plus* up to 9.5u of a 34u row's
//      centred single line.
//   3. 「现在鼠标悬浮选中和输入时自动选中项之前存在交互冲突」 — the pointer
//      selected on every `mousemove` *and* painted a second, hover-only pane, so
//      a keyboard step left two rows lit and Enter ran the one the pointer was
//      not on.
//
// The tests below are the round's three claims, each checked against the sheet
// or the source rather than against a rendered frame: the field's stack is
// solid, the tail is asymmetric by design, and the pointer has exactly one
// selection path — `onPointerEnter` into the same state the keyboard writes.
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

const root = new URL("../", import.meta.url);
const read = (path: string) => readFile(new URL(path, root), "utf8");
const stripComments = (css: string) => css.replace(/\/\*[\s\S]*?\*\//g, "");
/** TSX/TS has two comment shapes; the round's own notes name the events it
 *  removed, so the scan has to read code and not prose. */
const stripJsComments = (source: string) =>
  source.replace(/\/\*[\s\S]*?\*\//g, "").replace(/(^|[^:])\/\/[^\n]*/g, "$1");

const rules = (css: string) => {
  const out: { selector: string; body: string }[] = [];
  for (const match of css.matchAll(/([^{}]+)\{([^{}]*)\}/g)) {
    out.push({ selector: match[1].trim().replace(/\s+/g, " "), body: match[2] });
  }
  return out;
};
const rule = (css: string, selector: string) =>
  rules(css).find((r) => r.selector === selector);
const decl = (body: string, prop: string) =>
  body.match(new RegExp(`(?:^|;)\\s*${prop}\\s*:\\s*([^;]+)`))?.[1].trim() ?? null;

const LAUNCHER = "src/styles/launcher.css";
const RESULTS = "src/launcher/LauncherResults.tsx";
const APP = "src/App.tsx";

test("the field is a surface: the lift lands on a near-solid face", async () => {
  const css = stripComments(await read(LAUNCHER));
  const row = rule(css, ".collapsed-card__input-row");
  assert.ok(row, "the search surface must exist");

  // Layer order is top-first, so the last layer is what the lift lands on.
  const layers = decl(row!.body, "background")!
    .split(",")
    .map((layer) => layer.trim());
  assert.equal(layers[0], "var(--glass-field)", "the lift stays the top layer");
  assert.equal(
    layers[layers.length - 1],
    "var(--surface-opaque)",
    "and it lands on the palette's near-solid face, not on the window",
  );

  // The claim only holds if the stand-in really is near-solid — in *both*
  // palettes, since the ghost is worse on the dark one.
  const base = await read("src/styles/base.css");
  const alphas = [...base.matchAll(/--surface-opaque:\s*rgba\([^)]*?,\s*([\d.]+)\s*\)/g)].map(
    (m) => Number(m[1]),
  );
  assert.ok(alphas.length >= 2, "both palettes declare --surface-opaque");
  for (const alpha of alphas) {
    assert.ok(alpha >= 0.97, `--surface-opaque must be near-solid, got ${alpha}`);
  }
});

test("the tail is shorter than the top, because the last row brings its own leading", async () => {
  const css = stripComments(await read(LAUNCHER));
  const bottom = rule(css, ".launcher-bottom");
  assert.ok(bottom, "the list panel must exist");
  const padding = decl(bottom!.body, "padding")!;
  const [top, side, tail] = [...padding.matchAll(/calc\(var\(--u\) \* (\d+)\)/g)].map(
    (m) => m[1],
  );
  assert.equal(top, "4", "the top stays 4u: it completes the 12u breath");
  assert.equal(tail, "2", "the tail is 2u: the row's leading is the rest");
  assert.equal(side, "4", "the sides keep the 4u that holds a selected row's tint off the card's edge");
});

test("the pointer selects on entry — one event, one state, no per-move writes", async () => {
  const source = stripJsComments(await read(RESULTS));
  assert.ok(
    !/onMouseMove/.test(source),
    "no mousemove selection: re-asserting the hovered row on every pixel is what fought the keyboard",
  );
  // Both row kinds take the same event: the result row and the action bar.
  const enters = source.match(/onPointerEnter=/g) ?? [];
  assert.equal(enters.length, 2, "the result row and the action bar both select on entry");
  assert.match(
    source,
    /onPointerEnter=\{\(\) => \{\s*if \(unavailable\) return;\s*onSelectResult\(index\);/,
    "a result row moves the shared selection to itself",
  );
  assert.match(
    source,
    /onPointerEnter=\{\(\) => onSelectActionBar\(\)\}/,
    "and the action bar selects itself through the same state",
  );
});

test("one highlight: no pointer-only pane survives on a row", async () => {
  const css = stripComments(await read(LAUNCHER));
  const painted = rules(css).filter(
    ({ selector, body }) =>
      /\.launcher-(result|action-bar)[^,{]*:hover/.test(selector) &&
      decl(body, "background") !== null,
  );
  assert.deepEqual(
    painted.map(({ selector }) => selector),
    [],
    "a hover-only background is a second highlight: the hovered row is the selected row now",
  );
  // …and the selected row is still the one thing that says "this runs on Enter".
  const selected = rule(css, ".launcher-result--selected");
  assert.match(selected!.body, /background:\s*var\(--glass-raised\)/);
});

test("the keyboard and the pointer write the same selection, and nothing else holds one", async () => {
  const app = await read(APP);
  assert.match(
    app,
    /onSelectResult=\{\(index\) => \{\s*setSelectedActionBar\(false\);\s*setSelectedResultIndex\(index\);\s*\}\}/,
    "the row handler clears the action bar and sets the one index both devices read",
  );
  assert.match(
    app,
    /onSelectActionBar=\{\(\) => setSelectedActionBar\(true\)\}/,
    "the action bar's selection is the other half of the same pair",
  );
  // No second index: a `hoverIndex`/`hoveredIndex` would be a second highlight
  // waiting to disagree with the first.
  assert.ok(
    !/hover(ed)?Index/i.test(app) && !/hover(ed)?Index/i.test(await read(RESULTS)),
    "there is no hover-only selection state to disagree with the real one",
  );
});
