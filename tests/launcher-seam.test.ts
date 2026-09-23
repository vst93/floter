// R18 · the field and the list are divided by brightness, not by a line.
//
// The history, in one paragraph. R13 replaced the aura (「输入框下方的阴影太草率
// 了」) with a hairline on the input row's floor. R15 removed the aura and left
// the hairline; R16 stretched it to the content column (「阴影线条拉齐」); R17
// made it solid (「不要渐隐」). The verdict after all four was the same as before
// them — 「还是很丑」 — and this round the user named a reference instead of a
// complaint: tinycast, which draws *no* rule between its field and its list.
// Its search block is simply one step brighter than the panel, its list sits on
// the panel's own material, and the boundary is read from that difference.
//
// So R18 is a subtraction, and a structural one:
//
//   1. the two seam pseudo-elements, their shared geometry rule and both gates
//      are deleted — not hidden, not made transparent, deleted;
//   2. the two gradient hairlines the panel still drew between its surfaces
//      (`.launcher-bottom::before`, `.launcher-action-bar::before`) go with
//      them, because a panel that has no seam between its field and its list
//      has no business drawing one between its list and its action bar either;
//   3. in their place the search row paints `--glass-field` — the token that
//      means "the surface a text field sits on", a white lift over the card's
//      tint in both palettes — and carries `margin-bottom: calc(var(--u) * 4)`,
//      which with `.launcher-bottom`'s own 4u of padding leaves an 8u
//      transparent breath between the two faces (R18 drew the margin at 8u for
//      a 12u breath; R24 halved it, see `tests/launcher-input-gap.test.ts`); and
//   4. the accent budget shrinks to the two marks the reference's palette has:
//      the selected row's tint + keyline, and the caret. The onboarding tip's
//      keyline and glyph leave the accent.
//
// The tests below are the same shape as the round's: the removal is checked
// negatively (no pseudo-element, no band, no `--hairline-fade`, no `height:
// 1px` anywhere in the sheet) and the replacement positively (the two faces,
// their two materials, the 8u of nothing between them), so putting any of it
// back — in any shape — turns the suite red.
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

const root = new URL("../", import.meta.url);
const read = (path: string) => readFile(new URL(path, root), "utf8");
const stripComments = (css: string) => css.replace(/\/\*[\s\S]*?\*\//g, "");

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
// The arguments of a gradient, one stop per element.
const angle = (value: string) => value.match(/^\s*(?:repeating-)?linear-gradient\(\s*([^,]+)/)?.[1].trim() ?? null;
const stops = (value: string) => {
  const body = value.slice(value.indexOf("(") + 1, value.lastIndexOf(")"));
  const parts = body.split(",").map((s) => s.trim());
  // The angle is not a stop.
  return angle(value) ? parts.slice(1) : parts;
};

const LAUNCHER = "src/styles/launcher.css";
const APP = "src/App.tsx";

test("the aura is gone: no wash node, no wash rule, no wash token in the sheet", async () => {
  const css = stripComments(await read(LAUNCHER));
  const app = await read(APP);
  assert.equal(
    rule(css, ".collapsed-card__aura"),
    undefined,
    "the aura rule must be gone — R15 retires the field rather than tuning it again",
  );
  assert.ok(
    !/\.collapsed-card__aura\b/.test(css),
    "no selector may survive the rule: a leftover gate would resurrect the wash",
  );
  assert.ok(
    !/collapsed-card__aura/.test(app),
    "the aura node must be gone from App.tsx too — a rule with no node is a comment, a node with no rule is a blank",
  );
  assert.ok(
    !/--accent-wash/.test(css),
    "launcher.css must not reference --accent-wash any more: focus is the caret and the card's own edge",
  );
  // The removal is a subtraction, not a swap: the row gained a *material*, and
  // nothing else. No shadow, no gradient, no accent.
  const row = rule(css, ".collapsed-card__input-row");
  assert.ok(row, "the input row rule must still exist");
  // R31 · the row paints exactly one thing now: the 1px hairline on its floor
  // (「输入框下方阴影可以只保留 1px」). No aura, no wash, no gradient — a single
  // inset line in the shared `--hairline` token. An inset shadow rather than a
  // border, so the row's box (a pinned metric — 42u in R23, 56u since R37) does
  // not grow.
  assert.equal(
    decl(row!.body, "box-shadow"),
    "inset 0 -1px 0 var(--hairline)",
    "R31: the row's only mark is the 1px field/list hairline",
  );
  assert.deepEqual(
    decl(row!.body, "background")!.split(",").map((layer) => layer.trim()),
    ["var(--glass-field)", "var(--surface-opaque)"],
    "R18: the row is the search *surface* — the field token, one step brighter than the card face",
  );
  assert.ok(!/gradient/.test(row!.body), "and it is a flat fill, not a wash in a new shape");
  assert.ok(!/color-mix/.test(css), "launcher.css must stay color-mix-free (WebKitGTK without the feature renders an empty box)");
});

test("the old smear cannot come back in a new shape: no accent wash gradient anywhere", async () => {
  const css = stripComments(await read(LAUNCHER));
  assert.ok(
    !/transparent\s+42%/.test(css),
    "the 42% cutoff is the original report — it must not come back",
  );
  assert.ok(
    !/--accent-wash/.test(css),
    "a vertical bloom is still a wash: the token is retired from this sheet, not reshaped",
  );
  // R18: and there is no seam left to reshape either. The search surface is
  // neutral — the only accent the row owns is the caret — and the panel draws
  // no accent mark between its surfaces at all.
  const row = rule(css, ".collapsed-card__input-row");
  assert.ok(row, "the input row must still exist");
  assert.ok(!/accent/.test(row!.body), "the search surface carries no accent: the caret is the row's whole colour story");
  // R31 · and no *second* mark: the one inset hairline is allowed, anything else
  // (a keyline, a bloom, a gradient) is the seam by another name.
  const shadows = decl(row!.body, "box-shadow");
  assert.equal(
    shadows,
    "inset 0 -1px 0 var(--hairline)",
    "exactly one inset mark, the R31 hairline — no accent, no second inset",
  );
  assert.ok(!/gradient/.test(row!.body), "and it is still a flat fill, not a wash in a new shape");
});

test("the divider is gone: no seam pseudo-elements and no hairline band left in the sheet", async () => {
  const css = stripComments(await read(LAUNCHER));
  // The R13/R16/R17 seam: both pseudo-elements, their shared geometry rule and
  // the two gates are deleted, not hidden behind an opacity.
  assert.equal(
    rule(css, ".collapsed-card__input-row::before"),
    undefined,
    "the focused seam is gone: focus is the caret and the card's own edge now",
  );
  assert.equal(
    rule(css, ".collapsed-card__input-row::after"),
    undefined,
    "and the neutral seam with it — a divider the user has rejected four times does not get a fifth shape",
  );
  assert.ok(
    !/collapsed-card__input-row::/.test(css),
    "no pseudo-element may survive on the input row at all",
  );
  assert.equal(
    rule(css, ".collapsed-card--results-visible .collapsed-card__input-row::after"),
    undefined,
    "the gate that lit the neutral seam goes with the seam",
  );
  assert.equal(
    rule(css, ".collapsed-card:focus-within .collapsed-card__input-row::before"),
    undefined,
    "and the gate that lit the accent one",
  );
  // The two other rules the panel drew between its surfaces: the gradient
  // hairline above the list field and the one above the action bar. A panel
  // with no divider between its field and its list may not keep one between its
  // list and its action bar.
  assert.equal(
    rule(css, ".launcher-bottom::before"),
    undefined,
    "the field/list boundary is a brightness step now, not a band",
  );
  assert.equal(
    rule(css, ".launcher-action-bar::before"),
    undefined,
    "the list/action-bar boundary likewise — the action bar welds to the list it belongs to",
  );
  assert.ok(!/--hairline-fade/.test(css), "no gradient hairline is left in the sheet");
  assert.ok(!/height:\s*1px/.test(css), "and no 1px band anywhere: the panel draws no dividers at all");
});

test("the two surfaces are told apart by brightness, with a breath of card between them", async () => {
  const css = stripComments(await read(LAUNCHER));
  const row = rule(css, ".collapsed-card__input-row");
  const bottom = rule(css, ".launcher-bottom");
  assert.ok(row, "the search surface must exist");
  assert.ok(bottom, "the list panel must exist");
  // The brighter face: the field token, which base.css defines as a white
  // overlay in *both* palettes, so it is a lift off the card's tint either way.
  // R21: and it is laid over the card's own near-solid face, so the lift lands
  // on a surface instead of on whatever the window is showing through.
  assert.deepEqual(
    decl(row!.body, "background")!.split(",").map((layer) => layer.trim()),
    ["var(--glass-field)", "var(--surface-opaque)"],
    "the search block is --glass-field over --surface-opaque: a white lift on a solid face",
  );
  // The darker face, unchanged: the list keeps the content recess it has always
  // had, so the two surfaces differ in brightness alone.
  assert.equal(
    decl(bottom!.body, "background"),
    "var(--surface-sunken)",
    "the list keeps the darker recess — the difference between the faces is the divider now",
  );
});

test("the breath between the two faces is transparent card, not a painted line", async () => {
  const css = stripComments(await read(LAUNCHER));
  const row = rule(css, ".collapsed-card__input-row");
  const bottom = rule(css, ".launcher-bottom");
  assert.ok(row && bottom, "both faces must exist");
  // The breath: 4u of the row's own margin plus the panel's 4u of padding is
  // 8u of the card's own material between the two faces (R18: 8u + 4u = 12u;
  // R24 halved the margin).
  assert.equal(
    decl(row!.body, "margin-bottom"),
    "calc(var(--u) * 4)",
    "the row carries the gap below the search surface",
  );
  assert.equal(
    decl(bottom!.body, "padding"),
    "calc(var(--u) * 4) calc(var(--u) * 4) calc(var(--u) * 2)",
    "and the panel's own 4u completes the 8u breath — the two faces never touch",
  );
  // A transparent gap, not a painted one: no border and no pseudo-element may
  // turn the breath back into a line.
  assert.equal(
    decl(row!.body, "border-bottom"),
    null,
    "the gap is not a border — a border would paint the line straight back in",
  );
  assert.ok(
    !/hairline-fade|::(before|after)/.test(css),
    "and nothing in the sheet draws on either boundary",
  );
});

test("focus is still unmistakable without the seam", async () => {
  const css = stripComments(await read(LAUNCHER));
  // No rule anywhere lights a wash or a seam on the input row.
  assert.equal(
    rules(css).filter((r) => /collapsed-card__input-row::/.test(r.selector)).length,
    0,
    "nothing may light a mark on the search surface: the removal is complete or it is not a removal",
  );
  // Focus stays legible through the card's own edge and the caret, which is
  // what makes the removal safe (WCAG 2.4.7).
  const focused = rule(css, ".collapsed-card:focus-within");
  assert.ok(focused, "the card's focus state must survive the seams");
  assert.equal(
    decl(focused!.body, "border-color"),
    "var(--input-stroke-active)",
    "focus still paints the card's own edge",
  );
  const field = rule(css, ".collapsed-card__input");
  assert.ok(field, "the field must still exist");
  assert.equal(
    decl(field!.body, "caret-color"),
    "var(--accent)",
    "and the caret is the accent — the one mark inside the surface",
  );
});

test("the seam's gate class is gone from the markup and the sheet", async () => {
  const css = stripComments(await read(LAUNCHER));
  const app = await read(APP);
  // R13 added `collapsed-card--results-visible` for one job: lighting the
  // neutral seam while there was a list under the field. The seam is gone, so
  // the class is dead state — and it must not be resurrected to gate the new
  // breath, because a *layout* metric that appears and disappears with the
  // result count is the 8px jump R15 spent a round removing from the action
  // bar.
  assert.ok(
    !/collapsed-card--results-visible/.test(app),
    "the markup must not emit the seam's gate class any more",
  );
  assert.ok(
    !/collapsed-card--results-visible/.test(css),
    "and no rule may read it — a gate with nothing behind it is a comment",
  );
  // The card's own class list is untouched: the launcher still names itself and
  // still carries the query-filled state.
  const emitted = app.match(/className=\{`collapsed-card\$\{[^`]*`\}/)?.[0];
  assert.ok(emitted, "the collapsed card's class list must still be findable in App.tsx");
  assert.ok(emitted!.includes("collapsed-card--filled"), "the filled state survives the round");
  assert.ok(css.includes(".collapsed-card {"), "and the sheet still styles the card itself");
});

// ── R15 · the clip expands as a layout snap, and the window catches up ────
//
// The report, verbatim: 「动不动页面布局就崩了」. The screenshot was the query `v`
// — nine rows and the action bar — with the bar's text cut through the middle
// by the card's own bottom edge. The card is `overflow: hidden` and the window
// is sized from a measurement of the card (see `syncLauncherHeight`), so any
// gap between "the content grew" and "the window grew" is visible as a cut.
//
// Two locks, one on each side of that gap:
//
//   1. the clip's expansion may never be *animated*: a transitioned
//      `grid-template-rows`/`max-height` would put the clip at a fraction of
//      its final size at the moment the measuring layout effect runs, and the
//      window would be set to that fraction; and
//   2. the window is re-measured once the native resize has settled, so a
//      resize that lands late (or a second content change while the first
//      resize is in flight) is corrected instead of leaving the card clipped.

const HOOK = "src/hooks/useLauncherHeight.ts";

// The layout properties that must never appear in a transition on the clip.
const LAYOUT_PROPS = ["grid-template-rows", "max-height", "height", "padding", "margin"];

test("the clip's expansion is a layout snap: only paint is transitioned", async () => {
  const css = stripComments(await read(LAUNCHER));
  for (const selector of [".launcher-bottom-clip", ".launcher-bottom-clip--open"]) {
    const clip = rule(css, selector);
    assert.ok(clip, `${selector} must exist — the clip is the action bar's reveal`);
    const property = decl(clip!.body, "transition-property");
    assert.equal(
      property,
      "opacity, visibility",
      `${selector} may transition paint only: a transitioned layout property makes the measured height a fraction of the real one`,
    );
    // The shorthand must not be used either: a later `transition:` would
    // silently override the longhands above (and vice versa), and the lock
    // would be checking a declaration the browser ignores.
    assert.equal(
      decl(clip!.body, "transition"),
      null,
      `${selector} must declare the longhands, not the shorthand — a shorthand next to them is a second, unread source of truth`,
    );
    for (const prop of LAYOUT_PROPS) {
      assert.ok(
        !new RegExp(`transition-property[^;]*\\b${prop}\\b`).test(clip!.body),
        `${prop} must never be transitioned on ${selector}`,
      );
    }
  }
  // …and the layout states themselves are unchanged: 0fr/0 collapsed, 1fr/600px
  // open. The fix is the transition list, not the geometry.
  assert.equal(decl(rule(css, ".launcher-bottom-clip")!.body, "grid-template-rows"), "0fr");
  assert.equal(decl(rule(css, ".launcher-bottom-clip")!.body, "max-height"), "0");
  assert.equal(decl(rule(css, ".launcher-bottom-clip--open")!.body, "grid-template-rows"), "1fr");
  assert.equal(decl(rule(css, ".launcher-bottom-clip--open")!.body, "max-height"), "600px");
});

test("the window is re-measured after the resize settles, so the card is never left clipped", async () => {
  const hook = await read(HOOK);
  // The measurement itself is untouched: the last laid-out child, offsets (not
  // rects — the entry scale animation would scale a rect), plus the card's
  // frame and the shell's padding.
  assert.match(
    hook,
    /last\.offsetTop \+ last\.offsetHeight \+ frame/,
    "the measurement stays an offset measurement of the last laid-out child",
  );
  assert.match(
    hook,
    /parseFloat\(shellStyle\.paddingTop\)[\s\S]{0,200}parseFloat\(shellStyle\.paddingBottom\)/,
    "the shell's padding is still added back: the window has to be that much taller for it to show",
  );
  // The correction: after the resize resolves, re-measure on the next frame and
  // resize again only if the content really did move.
  // R26-D · the re-measure feeds the same target rule the first resize used,
  // so the ordinary card settles back onto the height that was asked for and
  // the pass ends without a second `setSize`.
  assert.match(
    hook,
    /\.then\(\(\) => \{[\s\S]*?reassertCollapsedFocus\(\);[\s\S]*?afterPaint\(\(\) => \{[\s\S]*?launcherTargetHeight\(card, height\)[\s\S]*?settled !== height[\s\S]*?resizeLauncherWindow\(card, settled/,
    "the settle pass must re-measure after the resize lands and re-apply only a real difference",
  );
  assert.match(
    hook,
    /const base = windowHeight \+ shellPaddingHeight\(card\);/,
    "R26-D: the target is the caller's height (the band), plus the shell reservation",
  );
  assert.match(
    hook,
    /measured > current \+ 1/,
    "…and only a card that overflows the current window may raise it",
  );
  // …and it waits for the next paint, with a timer fallback so the helper stays
  // drivable where `requestAnimationFrame` does not exist (the node suite).
  assert.match(
    hook,
    /const afterPaint = \(run: \(\) => void\) => \{[\s\S]*?typeof requestAnimationFrame === "function"[\s\S]*?setTimeout\(run, 16\)/,
    "the settle pass must run after a paint, with a non-WebView fallback",
  );
  // …and it is bounded: a card that keeps changing size cannot spin.
  assert.match(hook, /const SETTLE_PASSES = \d+;/, "the settle passes must be a named, bounded constant");
  assert.match(hook, /passes <= 0/, "the settle pass must terminate");
  assert.match(
    hook,
    /resizeLauncherWindow\(card, height, SETTLE_PASSES\)/,
    "the initial resize must start the bounded chain",
  );
});
