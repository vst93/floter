// R22 · the gap between the query text and the first result row; re-derived in
// R23 for the field's second trim, in R24 for the two segments outside it, and
// in R37 for the field row's return to the settings band's height.
//
// The user drew a box around it (「我指的这中间的空白太宽了」) in R22, after that
// round still read the space below the query as too wide
// (「输入框下内边距还是太宽」) — so the field's row lost another 6u in R23 — and
// after *that* still read the gap as too tall (「现在还是太高」), which is what R24
// answers. R23 left the field at its floor (10u a side: a 22u line box in a 42u
// row), so R24 moves the two segments around it instead.
//
// R37 · the user asked for the opposite of R23's trim on the *row*: 「头部的输入框
// 整体高度小了些，可以和设置页面头部一样高」. The row is 56u again (the settings
// band's height), so the per-side dead height is back to 17u. The *breath*
// (R18/R24) and the scroller's reservation (R22-R24) are untouched — the row's
// height and the gap under it are two decisions, and only the first one moved.
//
// Measured from the sheet, the space between the *bottom of the field's row* and
// the *top of the first result row* is three stacked decisions, all of them
// deliberate and none of them wrong on its own:
//
//   * the field row's dead height below its 22u line box — `(56u − 22u) / 2`
//     = 17u in R37, back to the pre-R22 value (10u in R23/R24, 13u in R22);
//   * R18's breath below the block — the row's `margin-bottom` plus
//     `.launcher-bottom`'s 4u top padding = 8u after R24 (4u + 4u), 12u in R18
//     through R23; and
//   * the scroll-edge reservation the scroller keeps as top padding — 14px
//     from base.css before, 8px while R22–R23 held the launcher's local
//     override, 4px now.
//
// Stacked, that is `17 + 12 + 14 = 43px` before, `13 + 12 + 8 = 33px` after R22,
// `10 + 12 + 8 = 30px` after R23, `10 + 8 + 4 = 22px` after R24 and
// `17 + 8 + 4 = 29px` after R37: R22 gave back
// 4u of field dead height (one side only — the row is flex-centred, so an 8u row
// cut is a 4u cut per side) plus 6px of reservation; R23 gave back 3u more of
// that same per-side dead height (a 6u row cut is 3u per side); R24 gave back
// the remaining 4u of breath and the remaining 4px of reservation; R37 gives
// back the 7u the row gained below its line box, at the user's request, and
// nothing else. The assertions below are the arithmetic, so a later edit to any
// one of
// the three has to face the total.
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

const read = (path: string) => readFile(new URL(`../${path}`, import.meta.url), "utf8");

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

const px = (value: string, property: string) => {
  const match = /^(\d+(?:\.\d+)?)px$/.exec(value.trim());
  assert.ok(match, `${property} must be a pixel literal, got "${value}"`);
  return Number(match![1]);
};

test("the query-to-first-row gap is the sum of three pinned decisions", async () => {
  const launcher = stripComments(await read("src/styles/launcher.css"));
  const base = stripComments(await read("src/styles/base.css"));

  // 1 · the field row and the line box it centres.
  const row = rule(launcher, ".collapsed-card__input-row");
  const rowHeight = units(/min-height:\s*([^;]+);/.exec(row)![1], "input row min-height");
  const fieldHeight = units(
    /min-height:\s*([^;]+);/.exec(rule(launcher, ".collapsed-card__input"))![1],
    "field min-height",
  );
  assert.equal(rowHeight, 56, "R37's 56u row — the settings band's height");
  assert.equal(fieldHeight, 22, "the field's own box is unchanged at 22u");
  const deadBelowTheText = (rowHeight - fieldHeight) / 2;
  assert.equal(deadBelowTheText, 17, "the line box keeps 17u above and below it (R37)");

  // 2 · R18's breath, halved in R24: the row's margin plus the panel's top padding.
  const breath = units(/margin-bottom:\s*([^;]+);/.exec(row)![1], "input row margin-bottom");
  const bottomPadding = /padding:\s*(calc\(var\(--u\)\s*\*\s*\d+\))[^;]*;/.exec(
    rule(launcher, ".launcher-bottom"),
  );
  assert.ok(bottomPadding, ".launcher-bottom must declare a unit padding");
  const topPadding = units(bottomPadding![1], ".launcher-bottom padding-top");
  assert.equal(breath, 4, "R24 halves R18's margin below the field: 8u → 4u");
  assert.equal(topPadding, 4, "…and its 4u top padding stays (it is also the panel's row inset)");
  assert.equal(breath + topPadding, 8, "so the breath under the block is 8u, not R18's 12u");

  // 3 · the scroll-edge reservation: 14px in base.css, 4px on the launcher.
  assert.match(
    base,
    /--scroll-edge:\s*14px;/,
    "base.css keeps the shared 14px token — this round does not touch that file",
  );
  const override = /--scroll-edge:\s*([^;]+);/.exec(rule(launcher, ".collapsed-card"))![1];
  assert.equal(px(override, "launcher --scroll-edge"), 4, "R24 reserves 4px in the launcher scope");
  // …and the scroller actually reads the variable, so the band, its
  // `background-size` and the reservation shrink together rather than forking.
  const results = rule(launcher, ".launcher-results");
  assert.match(results, /padding:\s*var\(--scroll-edge\)\s+0\s+0;/, "the reservation is the variable");

  // The total, and the fact that it only moved by what the decisions gave.
  const before = (56 - 22) / 2 + 12 + 14;
  const r22 = (48 - 22) / 2 + 12 + 8;
  const r23 = (42 - 22) / 2 + 12 + 8;
  const r24 = (42 - 22) / 2 + (breath + topPadding) + px(override, "launcher --scroll-edge");
  const after = deadBelowTheText + (breath + topPadding) + px(override, "launcher --scroll-edge");
  assert.equal(before, 43);
  assert.equal(r22, 33, "R22 gave back 4u of field dead height plus 6px of reservation");
  assert.equal(r23, 30, "R23 gave back 3u more of the same per-side dead height");
  assert.equal(r24, 22, "R24 gives back the last 4u of breath and 4px of reservation");
  assert.equal(after, 29, "R37's row is 56u again, so the gap is back to the pre-R22 17u a side");
  assert.equal(
    r24 - after,
    -7,
    "R37 restores exactly the 7u of per-side dead height R23 trimmed, and nothing else",
  );
});

test("the launcher override is local: no other surface inherits it", async () => {
  const launcher = stripComments(await read("src/styles/launcher.css"));
  const settings = stripComments(await read("src/styles/settings.css"));
  const terminal = stripComments(await read("src/styles/terminal.css"));
  // The override hangs off `.collapsed-card`, the launcher's own root, not off
  // `:root` and not off a shared class — so the settings page and the clipboard
  // panel keep base.css's 14px band.
  assert.equal(
    (launcher.match(/--scroll-edge:\s*\d+px/g) ?? []).length,
    1,
    "exactly one local override, on the launcher's own scope",
  );
  for (const [name, css] of [["settings.css", settings], ["terminal.css", terminal]] as const) {
    assert.ok(!/--scroll-edge:\s*\d+px/.test(css), `${name} must not override the shared token`);
  }
  // The launcher's own declaration sits inside `.collapsed-card`, the ancestor
  // of both the field and the results scroller — so it is scoped to this surface
  // and to nothing else.
  assert.match(launcher, /\.collapsed-card\s*\{[^}]*--scroll-edge:\s*4px/s);

  // …and the DOM agrees that `.collapsed-card` is the only launcher surface that
  // could inherit it. The plugin layer (the clipboard panel, whose list also
  // paints a scroll-edge band) is a *sibling* of `.collapsed-shell`, not a
  // child of the card, and the settings page is a separate return branch with
  // its own `.settings-shell`. So the override reaches the field row and the
  // results list, and stops there.
  const app = await read("src/App.tsx");
  // R33 · the plugin layer is gone; the toast host is the only sibling before
  // the collapsed shell, and it never nests in the card either.
  assert.match(
    app,
    /\{toastHost\}\s*<div className="collapsed-shell">/,
    "the toast host is a sibling rendered before the collapsed shell, not nested in the card",
  );
  assert.match(app, /<div className="settings-shell">/, "the settings page has its own shell");
});
