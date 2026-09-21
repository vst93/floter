// R13 · the seam between the launcher's field and its list.
//
// The user's report, verbatim: 「样式也不行啊，不够精致，输入框下方的阴影太草率了」.
// The screenshot was the filled list state, and the thing under the field was
// the aura: one horizontal stop that ran `--accent-wash` 42% across the row and
// ended on a hard vertical edge. A rectangle of colour with a straight right
// wall parked under the field reads as a stain, not as light — and the list
// itself began on a line nobody had drawn.
//
// Two facts carry the round, and each is asserted from the source that owns it:
//
//   1. the aura falls away in *both* directions — full width, its one strength
//      at the field's text line, zero at both the row's top and its floor, so
//      there is no boundary left anywhere inside the card; and
//   2. the input row owns a 1px seam on its own floor, a neutral hairline while
//      there is a list to divide and an accent hairline while the row is
//      focused — never both, so the seam is always exactly one pixel.
//
// Both are checked positively (the gradients exist with their stops, the gates
// exist, the class is emitted) and negatively (the horizontal wash is gone, no
// `color-mix`, no `90deg` accent gradient, the two lines cannot co-occur), so
// putting the smear back is what turns the suite red.
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

test("the aura falls away in both directions: full width, zero at the top, zero at the floor", async () => {
  const css = stripComments(await read(LAUNCHER));
  const aura = rule(css, ".collapsed-card__aura");
  assert.ok(aura, ".collapsed-card__aura must exist — it is the field's lit state");
  const background = decl(aura!.body, "background");
  assert.ok(background, "the aura must paint a background");
  assert.equal(angle(background!), "180deg", "the aura must fall vertically — a horizontal angle is the smear this round removed");
  assert.ok(!/90deg/.test(background!), "no horizontal accent gradient may remain on the aura");
  const parts = stops(background!);
  assert.ok(parts.length >= 4, `the aura needs at least four stops (zero → peak → peak → zero), got ${parts.length}`);
  assert.ok(parts[0].startsWith("transparent"), "the aura must start at nothing under the card's top rim");
  assert.ok(parts[parts.length - 1].startsWith("transparent"), "the aura must reach nothing at the row's floor");
  assert.ok(
    parts.some((stop) => stop.startsWith("var(--accent-wash)")),
    "the aura's one strength is --accent-wash itself: the wash is redistributed, not turned up",
  );
  // The peak is a token, never a literal: a hardcoded rgba would be wrong in
  // the other palette, and the sheet is color-mix-free on purpose.
  assert.ok(!/color-mix/.test(css), "launcher.css must stay color-mix-free (WebKitGTK without the feature renders an empty box)");
  assert.ok(!/rgba?\(/.test(background!), "the aura gradient must derive from a token, not a hardcoded colour");
});

test("the old smear is gone: no 42% horizontal cutoff anywhere on the aura", async () => {
  const css = stripComments(await read(LAUNCHER));
  const background = decl(rule(css, ".collapsed-card__aura")!.body, "background");
  assert.ok(!/transparent\s+42%/.test(background!), "the 42% cutoff is the report itself — it must not come back");
  assert.ok(!/^\s*linear-gradient\(\s*90deg/.test(background!), "the aura is no longer a horizontal band");
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
  assert.equal(decl(gate!.body, "opacity"), "1", "the gate must be what turns the seam on");
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

test("the aura keeps its gate: filled or focused, and nothing else", async () => {
  const css = stripComments(await read(LAUNCHER));
  const lit = rules(css).filter((r) => /\.collapsed-card__aura\b/.test(r.selector) && decl(r.body, "opacity") === "1");
  assert.equal(lit.length, 1, "one rule lights the aura, and it names both states");
  assert.ok(lit[0].selector.includes(".collapsed-card--filled"), "a filled field is lit");
  assert.ok(lit[0].selector.includes(":focus-within"), "a focused field is lit");
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
