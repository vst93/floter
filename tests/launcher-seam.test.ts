// R13 · the seam between the launcher's field and its list.
//
// The user's report, verbatim: 「样式也不行啊，不够精致，输入框下方的阴影太草率了」.
// The screenshot was the filled list state, and the thing under the field was
// the aura: one horizontal stop that ran `--accent-wash` 42% across the row and
// ended on a hard vertical edge. A rectangle of colour with a straight right
// wall parked under the field reads as a stain, not as light — and the list
// itself began on a line nobody had drawn.
//
// R13 reshaped that aura; R15 removed it. The user's note never changed
// (「这个阴影还是太丑了，可以淡点，简洁点」), and a wash over the whole input row is
// a shadow however its stops are arranged. What is left is the seam:
//
//   1. the input row owns a 1px hairline on its own floor — a neutral one
//      while there is a list to divide and an accent one while the row is
//      focused, never both, so the seam is always exactly one pixel; and
//   2. nothing else is drawn under the field. No aura node, no aura rule, no
//      `--accent-wash` in this sheet, and no fill or shadow on the input row —
//      the caret and the seam are the whole focus story.
//
// Both are checked positively (the gradients exist with their stops, the gates
// exist, the class is emitted) and negatively (the wash is gone from the sheet
// *and* from the markup, no `color-mix`, no `90deg` accent gradient, the two
// lines cannot co-occur), so putting the smear back — in any shape — is what
// turns the suite red.
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

// Both seams share one geometry rule; the per-element rules only carry colour.
const SEAM = ".collapsed-card__input-row::before, .collapsed-card__input-row::after";

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
    "launcher.css must not reference --accent-wash any more: focus is the caret and the seam, not a wash",
  );
  // The removal is a subtraction, not a swap: the row gains nothing in its
  // place. No fill, no shadow, no new gradient on the input row itself.
  const row = rule(css, ".collapsed-card__input-row");
  assert.ok(row, "the input row rule must still exist");
  assert.equal(decl(row!.body, "background"), null, "the input row paints no fill of its own");
  assert.equal(decl(row!.body, "box-shadow"), null, "the input row paints no shadow of its own");
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
  // The one accent gradient that is allowed under the field is the focus seam,
  // and it is a 1px keyline — never a filled area.
  const accentSeam = rule(css, ".collapsed-card__input-row::before");
  assert.ok(accentSeam, "the focus seam must still exist");
  const background = decl(accentSeam!.body, "background") ?? "";
  assert.equal(angle(background), "90deg", "the only accent under the field is the horizontal hairline");
  assert.equal(
    stops(background).length,
    4,
    "four stops and no more: transparent at both ends, accent in the middle — an area fill would need a vertical angle",
  );
  assert.equal(
    decl(rule(css, SEAM)!.body, "height"),
    "1px",
    "and it is a line: an accent gradient taller than a pixel is the aura again",
  );
});

test("the input row owns a 1px seam on its floor, faded to nothing at both ends", async () => {
  const css = stripComments(await read(LAUNCHER));
  const seam = rule(css, ".collapsed-card__input-row::after");
  assert.ok(seam, "the neutral seam must be a pseudo-element on the input row — no node, nothing to measure");
  const geometry = rule(css, SEAM);
  assert.ok(geometry, "both seams share one geometry rule, so they can never drift apart");
  assert.equal(decl(geometry!.body, "content"), '""', "a pseudo-element needs content to exist at all");
  assert.equal(decl(geometry!.body, "height"), "1px", "the seam is a hairline");
  assert.equal(decl(geometry!.body, "bottom"), "0", "the seam sits on the row's floor, where the list begins");
  assert.equal(decl(geometry!.body, "opacity"), "0", "the seam is dark until the list lights it");
  assert.equal(decl(geometry!.body, "pointer-events"), "none", "a seam must never eat a click meant for the field or a row");
  assert.equal(
    decl(geometry!.body, "left"),
    "calc(var(--u) * 16)",
    "the seam is inset to the field's own padding: the mark belongs to the field, not to the window",
  );
  const background = decl(seam!.body, "background");
  assert.ok(background && /linear-gradient\(\s*90deg/.test(background), "the seam is drawn as a gradient so it can fade at both ends");
  assert.ok(/transparent 0%/.test(background!) && /transparent 100%/.test(background!), "both ends of the seam must reach transparent: a line with two soft ends separates without ruling");
  assert.ok(/var\(--input-stroke\)/.test(background!), "the resting seam is --input-stroke, the same hairline the card's own edge is drawn with");
});

test("the seam is gated: only while there is a list under it", async () => {
  const css = stripComments(await read(LAUNCHER));
  const gate = rule(css, ".collapsed-card--results-visible .collapsed-card__input-row::after");
  assert.ok(gate, "the neutral seam needs a gate — an empty launcher has no list to divide the field from");
  assert.equal(
    decl(gate!.body, "opacity"),
    "0.6",
    "R15: the neutral divider lands at 0.6, not at full strength — a divider is found, not seen",
  );
});

test("focus lights the accent hairline and puts the neutral one out — one pixel, never two", async () => {
  const css = stripComments(await read(LAUNCHER));
  const accent = rule(css, ".collapsed-card__input-row::before");
  assert.ok(accent, "the focused seam is ::before, so it and the neutral ::after can never share a pixel");
  const geometry = rule(css, SEAM);
  assert.equal(decl(geometry!.body, "height"), "1px", "the focused seam is a keyline, not a glow");
  assert.equal(decl(geometry!.body, "bottom"), "0", "the focused seam is the landing point of the aura: the row's floor");
  const background = decl(accent!.body, "background");
  assert.ok(background && /var\(--accent-edge\)/.test(background!), "the focused seam is --accent-edge, the app's keyline token");
  assert.ok(!/var\(--accent-edge-strong\)/.test(background!), "--accent-edge-strong is a filled control's edge; a 1px seam is a mark");
  const light = rule(css, ".collapsed-card:focus-within .collapsed-card__input-row::before");
  assert.ok(light && decl(light.body, "opacity") === "1", "focus-within must light the accent seam");
  const off = rule(css, ".collapsed-card--results-visible:focus-within .collapsed-card__input-row::after");
  assert.ok(off && decl(off.body, "opacity") === "0", "the neutral seam must drop to zero the moment the row is focused, or the seam doubles");
});

test("the aura's old gate is gone and focus is still unmistakable without it", async () => {
  const css = stripComments(await read(LAUNCHER));
  // No rule anywhere lights a wash on the input row.
  const lit = rules(css).filter((r) => /collapsed-card__aura/.test(r.selector));
  assert.equal(lit.length, 0, "nothing may light an aura: the gate and the rule go together");
  // Focus stays legible through the card's own two marks — the active border
  // and the focus seam — which is what makes the removal safe (WCAG 2.4.7).
  const focused = rule(css, ".collapsed-card:focus-within");
  assert.ok(focused, "the card's focus state must survive the aura");
  assert.equal(
    decl(focused!.body, "border-color"),
    "var(--input-stroke-active)",
    "focus still paints the card's own edge",
  );
  const seam = rule(css, ".collapsed-card:focus-within .collapsed-card__input-row::before");
  assert.ok(
    seam && decl(seam.body, "opacity") === "1",
    "and the accent seam at full strength: the seam, not a wash, is the focus story now",
  );
});

test("the class the sheet gates on is the class the launcher emits", async () => {
  const css = stripComments(await read(LAUNCHER));
  const app = await read(APP);
  const emitted = app.match(/className=\{`collapsed-card\$\{[^}]*\}[^`]*`\}/)?.[0];
  assert.ok(emitted, "the collapsed card's class list must be findable in App.tsx");
  assert.ok(emitted!.includes("collapsed-card--results-visible"), "the launcher must emit collapsed-card--results-visible");
  assert.ok(
    /displayedResults\.length > 0 \? " collapsed-card--results-visible" : ""/.test(emitted!),
    "the class must be gated on the rendered list, not on the query: the empty-query clipboard row is a list too",
  );
  assert.ok(css.includes(".collapsed-card--results-visible"), "and the sheet must be the one that reads it — no orphan class");
  // The card must not gate on the query instead: an empty query still shows
  // the clipboard row, and that list needs the same seam.
  assert.ok(!/hasQuery \? " collapsed-card--results-visible"/.test(emitted!), "gating the seam on hasQuery would hide it in the empty-query list");
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
  assert.match(
    hook,
    /\.then\(\(\) => \{[\s\S]*?reassertCollapsedFocus\(\);[\s\S]*?afterPaint\(\(\) => \{[\s\S]*?measureCardHeight\(card\)[\s\S]*?settled !== height[\s\S]*?resizeLauncherWindow\(card, settled/,
    "the settle pass must re-measure after the resize lands and re-apply only a real difference",
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
