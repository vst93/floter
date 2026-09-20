// R10-B · The collapsed launcher's pinned card loses its header bar, and the
// launcher gets one implicit drag handle in its place.
//
// The user's report, verbatim: 「现在简洁版头部还是有独立的头部状态栏，这个是不需要
// 的。之前就说过要做成隐藏式的，就跟搜索和终端页面一样，鼠标悬浮到顶部才微微显示
// 出来，然后支持拖动」. Two facts carry the round, and each is asserted from the
// source that owns it rather than from a restatement:
//
//   1. the pinned card's header is an implicit band on the *launcher* only —
//      the collapsed shell tags the instance `data-variant="launcher"` and the
//      sheet fades that header to zero until it is hovered or focused; the
//      terminal page's instance keeps its always-visible header;
//   2. the card is no longer a drag surface, and the one drag surface the
//      launcher has is a 28px band at the card's top that never covers the
//      field or the buttons.
//
// Both are checked positively (the band exists and calls `startDrag`, the
// variant is emitted, the header rule exists) *and* negatively (no
// `onMouseDown={startDrag}` on the card, no launcher variant on the terminal
// instance), so deleting the code is what turns the suite red — not deleting
// the prose.
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

// The collapsed shell's rendered branch: from the last `if (mode === "collapsed")`
// (the render branch; the mode effect above it uses the same test) to the
// terminal branch's own preamble. Sliced by structure, not by a fixed window,
// so the assertions below cannot go vacuous when App.tsx moves around them.
const collapsedBranch = (app: string) => {
  const start = app.lastIndexOf('if (mode === "collapsed") {');
  assert.notEqual(start, -1, "App.tsx must still branch on the collapsed mode");
  const end = app.indexOf("const identityTitle", start);
  assert.notEqual(end, -1, "the collapsed branch must still be followed by the terminal one");
  return app.slice(start, end);
};

// ── 1 · the drag handle ───────────────────────────────────────────────────

test("the launcher card is no longer a drag surface", async () => {
  const app = await read("src/App.tsx");
  const branch = collapsedBranch(app);
  // The card element itself carries no drag binding. The one `startDrag` the
  // branch keeps belongs to the hot zone (asserted next).
  assert.ok(
    !/ref=\{collapsedCardRef\}[\s\S]{0,600}?onMouseDown=\{startDrag\}/.test(branch),
    "the collapsed card must not be one big drag handle",
  );
  assert.equal(
    (branch.match(/onMouseDown=\{startDrag\}/g) ?? []).length,
    1,
    "the collapsed branch must bind the drag exactly once",
  );
});

test("one 28px hot zone at the card's top is the drag handle", async () => {
  const app = await read("src/App.tsx");
  const branch = collapsedBranch(app);
  const zone = branch.match(
    /<div\s+className="collapsed-card__drag-zone"[\s\S]{0,200}?onMouseDown=\{startDrag\}\s*\/>/,
  );
  assert.ok(zone, "the collapsed card must render the drag hot zone");
  assert.match(zone![0], /aria-hidden="true"/, "the band is invisible: it must be hidden from AT");
  // The field is the reason the band exists at all: it may not cover it.
  assert.ok(
    branch.indexOf('className="collapsed-card__drag-zone"') <
      branch.indexOf('className="collapsed-card__input"'),
    "the band must come before the field in the DOM, so the field paints over it",
  );
  // And it must not steal focus from the field on press: the zone only drags.
  assert.ok(
    !/collapsed-card__drag-zone[\s\S]{0,200}?onClick=/.test(branch),
    "the band is a drag handle, not a focus target",
  );
});

test("the hot zone is a 28px absolute band under the field, and the card is not grab-shaped", async () => {
  const css = stripComments(await read("src/styles/launcher.css"));
  const zone = rule(css, ".collapsed-card__drag-zone");
  assert.ok(zone, "launcher.css must define the drag hot zone");
  assert.equal(decl(zone!.body, "height"), "calc(var(--u) * 28)", "the hit region is 28px");
  assert.equal(decl(zone!.body, "position"), "absolute", "the band is taken out of layout");
  assert.equal(decl(zone!.body, "cursor"), "grab", "the band advertises the drag");
  // Below the field and the buttons, both of which are `z-index: 1`.
  assert.equal(decl(zone!.body, "z-index"), "0", "the band must paint under the controls");
  const active = rule(css, ".collapsed-card__drag-zone:active");
  assert.ok(active, "the band needs its grabbing state");
  assert.equal(decl(active!.body, "cursor"), "grabbing");

  // The card lost the whole-surface grab cursor with it.
  const card = rule(css, ".collapsed-card");
  assert.ok(card, "launcher.css must define .collapsed-card");
  assert.equal(decl(card!.body, "cursor"), null, "the card must not read as a grab surface");

  // The field still floats above the band — that is what keeps its clicks.
  const field = rule(css, ".collapsed-card__input");
  assert.ok(field, "launcher.css must define the field");
  assert.equal(decl(field!.body, "z-index"), "1", "the field must stay above the band");
});

// ── 2 · the header goes implicit on the launcher only ──────────────────────

test("the collapsed instance is tagged, the terminal instance is not", async () => {
  const app = await read("src/App.tsx");
  // The card element is built once, before the mode branches, and the tag is
  // derived from the mode right there — that is what lets the same element
  // serve the terminal page (visible header) and the launcher (implicit one).
  const element = app.slice(
    app.indexOf("const pinnedCardElement"),
    app.indexOf("const pluginLayer"),
  );
  assert.ok(element.length > 0, "App must still build the pinned card element once");
  assert.match(
    element,
    /variant=\{mode === "collapsed" \? "launcher" : "terminal"\}/,
    "the card element must tag the collapsed instance as the launcher variant",
  );
  // The terminal page renders the same element and keeps the default variant:
  // exactly one `variant=` exists in the whole file, and it is the one above.
  assert.equal(
    (app.match(/variant=\{/g) ?? []).length,
    1,
    "only the one card element may pass the variant",
  );
  // And the collapsed branch is where that element is rendered.
  assert.match(
    collapsedBranch(app),
    /\{pinnedCardElement\}/,
    "the collapsed shell must render the tagged card",
  );
});

test("PinnedTerminalCard emits the variant and defaults to the terminal one", async () => {
  const card = await read("src/terminal/PinnedTerminalCard.tsx");
  assert.match(
    card,
    /variant\?: "terminal" \| "launcher"/,
    "the prop must be optional and narrow",
  );
  assert.match(card, /variant = "terminal"/, "the default must be the always-visible header");
  assert.match(card, /data-variant=\{variant\}/, "the variant must reach the DOM for the sheet");
});

test("the launcher header fades in on hover/focus and is inert while hidden", async () => {
  const css = stripComments(await read("src/styles/pinned-card.css"));
  const hidden = rule(
    css,
    '.collapsed-shell [data-variant="launcher"] .pinned-card__header',
  );
  assert.ok(hidden, "the launcher variant's header rule must exist");
  assert.equal(decl(hidden!.body, "opacity"), "0", "the header starts invisible");
  assert.equal(
    decl(hidden!.body, "transition"),
    "opacity var(--dur-3) var(--ease-out)",
    "it uses the shared duration token, like the terminal bar's reveal",
  );

  const revealed = rule(
    css,
    '.collapsed-shell [data-variant="launcher"] .pinned-card__header:hover, .collapsed-shell [data-variant="launcher"] .pinned-card__header:focus-within',
  );
  assert.ok(revealed, "hover and keyboard focus must both reveal the header");
  assert.equal(decl(revealed!.body, "opacity"), "1");

  // Hidden means unclickable *and* untabbable: no invisible control may sit
  // over the terminal, and the close button must not be a keyboard trap on a
  // band nobody can see.
  const inert = rule(
    css,
    '.collapsed-shell [data-variant="launcher"] .pinned-card__header:not(:hover):not(:focus-within) > *',
  );
  assert.ok(inert, "the hidden header's children must be taken out of the hit-test and tab order");
  assert.equal(decl(inert!.body, "visibility"), "hidden");

  // The 28px is layout, not paint: the card must not jump when the band fades.
  const header = rule(css, ".pinned-card__header");
  assert.ok(header, "pinned-card.css must define the header");
  assert.equal(decl(header!.body, "height"), "28px", "the header keeps its height");
  assert.equal(decl(header!.body, "opacity"), null, "the base header stays visible for the terminal page");
});
