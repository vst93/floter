// R10-B · The collapsed launcher gets one implicit drag handle in its place.
//
// The user's report, verbatim: 「现在简洁版头部还是有独立的头部状态栏，这个是不需要
// 的。之前就说过要做成隐藏式的，就跟搜索和终端页面一样，鼠标悬浮到顶部才微微显示
// 出来，然后支持拖动」. The fact this round is asserted from the source that owns
// it rather than from a restatement:
//
//   · the card is no longer a drag surface, and the one drag surface the
//     launcher has is a 28px band at the card's top that never covers the
//     field or the buttons.
//
// It is checked positively (the band exists and calls `startDrag`) *and*
// negatively (no `onMouseDown={startDrag}` on the card), so deleting the code
// is what turns the suite red — not deleting the prose.
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
