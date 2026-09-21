// R22 · the gap between the query text and the first result row; re-derived in
// R23 for the field's second trim.
//
// The user drew a box around it (「我指的这中间的空白太宽了」) in R22, and after
// that round still read the space below the query as too wide
// (「输入框下内边距还是太宽」) — so the field's row lost another 6u in R23.
// Measured from the sheet, the space between the *bottom of the field's row* and
// the *top of the first result row* is three stacked decisions, all of them
// deliberate and none of them wrong on its own:
//
//   * the field row's dead height below its 22u line box — `(42u − 22u) / 2`
//     = 10u after R23, 13u in R22, `(56u − 22u) / 2` = 17u before;
//   * R18's breath below the block — the row's 8u `margin-bottom` plus
//     `.launcher-bottom`'s 4u top padding = 12u (untouched in either round); and
//   * the scroll-edge reservation the scroller keeps as top padding — 14px
//     from base.css before, 8px now that the launcher overrides the token.
//
// Stacked, that is `17 + 12 + 14 = 43px` before, `13 + 12 + 8 = 33px` after R22
// and `10 + 12 + 8 = 30px` after R23: R22 gave back 4u of field dead height (one
// side only — the row is flex-centred, so an 8u row cut is a 4u cut per side)
// plus 6px of reservation; R23 gave back 3u more of that same per-side dead
// height (a 6u row cut is 3u per side). Nothing else moves. The assertions below
// are the arithmetic, so a later edit to any one of the three has to face the
// total.
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
  assert.equal(rowHeight, 42, "R23 takes the field's row from 48u to 42u");
  assert.equal(fieldHeight, 22, "the field's own box is unchanged at 22u");
  const deadBelowTheText = (rowHeight - fieldHeight) / 2;
  assert.equal(deadBelowTheText, 10, "the line box keeps 10u above and below it — the breathing floor");

  // 2 · R18's breath, untouched: the row's margin plus the panel's top padding.
  const breath = units(/margin-bottom:\s*([^;]+);/.exec(row)![1], "input row margin-bottom");
  const bottomPadding = /padding:\s*(calc\(var\(--u\)\s*\*\s*\d+\))[^;]*;/.exec(
    rule(launcher, ".launcher-bottom"),
  );
  assert.ok(bottomPadding, ".launcher-bottom must declare a unit padding");
  const topPadding = units(bottomPadding![1], ".launcher-bottom padding-top");
  assert.equal(breath, 8, "R18's 8u margin below the field stays");
  assert.equal(topPadding, 4, "…and its 4u top padding stays");

  // 3 · the scroll-edge reservation: 14px in base.css, 8px on the launcher.
  assert.match(
    base,
    /--scroll-edge:\s*14px;/,
    "base.css keeps the shared 14px token — this round does not touch that file",
  );
  const override = /--scroll-edge:\s*([^;]+);/.exec(rule(launcher, ".collapsed-card"))![1];
  assert.equal(px(override, "launcher --scroll-edge"), 8, "the launcher scope reserves 8px");
  // …and the scroller actually reads the variable, so the band, its
  // `background-size` and the reservation shrink together rather than forking.
  const results = rule(launcher, ".launcher-results");
  assert.match(results, /padding:\s*var\(--scroll-edge\)\s+0\s+0;/, "the reservation is the variable");

  // The total, and the fact that it only shrank by what the decisions gave.
  const before = (56 - 22) / 2 + 12 + 14;
  const after = deadBelowTheText + 12 + 8;
  assert.equal(before, 43);
  assert.equal(after, 30);
  assert.equal(
    before - after,
    13,
    "4u (R22) + 3u (R23) of field dead height, one side each, plus 6px of reservation, and nothing else",
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
  assert.match(launcher, /\.collapsed-card\s*\{[^}]*--scroll-edge:\s*8px/s);

  // …and the DOM agrees that `.collapsed-card` is the only launcher surface that
  // could inherit it. The plugin layer (the clipboard panel, whose list also
  // paints a scroll-edge band) is a *sibling* of `.collapsed-shell`, not a
  // child of the card, and the settings page is a separate return branch with
  // its own `.settings-shell`. So the override reaches the field row and the
  // results list, and stops there.
  const app = await read("src/App.tsx");
  assert.match(
    app,
    /\{pluginLayer\}\s*\{toastHost\}\s*<div className="collapsed-shell">/,
    "the plugin layer is a sibling rendered before the collapsed shell, not nested in the card",
  );
  assert.match(app, /<div className="settings-shell">/, "the settings page has its own shell");
});
