// R7-13b · CSS dimensions go through one scale knob.
//
// The whole interface is drawn from `--ui-scale` (1 this round; R7-13c is the
// round that lets the user move it). Box dimensions are written as
// `calc(var(--u) * N)` where `--u: calc(1px * var(--ui-scale))`, so at scale 1
// every converted declaration must resolve to *exactly* the pixel value it
// carried before the round. That is the invariant this file pins — not a
// snapshot of the CSS, but the arithmetic that says the refactor changed
// nothing.
//
// Mutations that must turn this file red:
//   * `--ui-scale: 1.2` in base.css -> "the scale is 1 this round" fails, and
//     every resolved sample moves off its original value;
//   * deleting `--ui-scale` (or `--u`) from the `:root` block -> the token
//     assertions fail before any value is compared;
//   * rewriting a sample as `calc(var(--u) * 99)` -> that sample's resolved
//     value no longer equals its recorded original.
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

const root = new URL("../", import.meta.url);
const read = (path: string) => readFile(new URL(path, root), "utf8");
const stripComments = (css: string) => css.replace(/\/\*[\s\S]*?\*\//g, "");

// The `:root` block as written in the file (comments stripped), so a token that
// only appears inside a comment does not satisfy an assertion.
const rootBlock = async () => {
  const base = stripComments(await read("src/styles/base.css"));
  const start = base.indexOf(":root {");
  const end = base.indexOf('[data-theme="light"]');
  assert.ok(start >= 0 && end > start, "base.css must keep a :root block before the light theme");
  return base.slice(start, end);
};

const token = (block: string, name: string) => {
  const match = block.match(new RegExp(`--${name}:\\s*([^;]+);`));
  assert.ok(match, `--${name} must be defined in the dark :root block`);
  return match![1].trim();
};

const rules = (css: string) => {
  const out: { selector: string; body: string }[] = [];
  for (const match of stripComments(css).matchAll(/([^{}]+)\{([^{}]*)\}/g)) {
    out.push({ selector: match[1].trim(), body: match[2] });
  }
  return out;
};

const declarations = (body: string, property: string) =>
  [...body.matchAll(new RegExp(`(?:^|;)\\s*${property}\\s*:\\s*([^;]+)`, "g"))].map((m) => m[1].trim());

const ruleFor = (css: string, selector: string) => {
  // A grouped rule (`.a, .b { … }`) is one body shared by several selectors, so
  // an exact-string match would miss the second name. Match any comma-separated
  // selector in the group instead.
  const rule = rules(css).find(({ selector: s }) =>
    s.split(",").some((part) => part.trim() === selector),
  );
  assert.ok(rule, `the sheet must still define ${selector}`);
  return rule!;
};

// ── Resolver ──────────────────────────────────────────────────────────────
// Evaluate `calc(var(--u) * N)` the way the engine would, with `--ui-scale`
// set: substitute `--u`, then evaluate the one product. The expressions in the
// sheets are all of this one shape (the transform is mechanical), so a full
// calc parser would only hide a malformed expression. `evaluate` returns null
// for anything else, and the caller then reports the raw string.
const evaluate = (expression: string, scale: number): number | null => {
  // A literal that was deliberately deferred keeps its pixel value; it is
  // reported as resolving to itself rather than as an error, and the separate
  // DEFERRED list below is what forces a reason to exist for it.
  const literal = expression.match(/^(\d+(?:\.\d+)?)px$/);
  if (literal) return Number(literal[1]);
  const match = expression.match(/^calc\(var\(--u\)\s*\*\s*(\d+(?:\.\d+)?)\)$/);
  if (!match) return null;
  return Number(match[1]) * (1 * scale);
};

// Split a value into top-level tokens, respecting parentheses: a
// `calc(var(--u) * 9)` is one token even though it contains spaces.
const splitTokens = (value: string) => {
  const out: string[] = [];
  let depth = 0;
  let start = 0;
  for (let i = 0; i < value.length; i += 1) {
    const c = value[i];
    if (c === "(") depth += 1;
    else if (c === ")") depth -= 1;
    else if (/\s/.test(c) && depth === 0) {
      if (i > start) out.push(value.slice(start, i));
      start = i + 1;
    }
  }
  if (start < value.length) out.push(value.slice(start));
  return out;
};

// Resolve a whole value (`0 calc(var(--u) * 9) 0 calc(var(--u) * 11)`) the way
// the browser does: each token on its own, `0` staying `0`.
const resolveValue = (value: string, scale: number) => {
  const out: number[] = [];
  for (const token_ of splitTokens(value.trim())) {
    if (token_ === "0") {
      out.push(0);
      continue;
    }
    const px = evaluate(token_, scale);
    assert.notEqual(px, null, `unresolvable token "${token_}" — the round writes calc(var(--u) * N) and nothing else`);
    out.push(px!);
  }
  return out;
};

const originalPx = (value: string) => splitTokens(value.trim()).map((t) => (t === "0" ? 0 : Number(t.replace("px", ""))));

// ── 1 · the knob ──────────────────────────────────────────────────────────

test("the interface scale is a `--ui-scale` knob owned by the settings layer", async () => {
  const block = await rootBlock();
  // The single truth. R7-13b pinned it at 1 as a no-behaviour-change refactor;
  // R7-13c is the round that moves it, so the assertion is now that the knob is
  // *derivable and live*: the `:root` fallback is the shipped default step (1),
  // and the multiplier is written onto the document by `src/ui-scale.ts`.
  assert.equal(token(block, "ui-scale"), "1", "the `:root` fallback is the default step, and its factor is 1");
  // `--u` is the scaled pixel unit, and it must derive from the knob rather
  // than be a second literal — that is what makes one token scale every box.
  assert.equal(token(block, "u"), "calc(1px * var(--ui-scale))", "--u must be calc(1px * var(--ui-scale))");
  // The type basis. R7-13c put the whole `--text-*` ladder on the knob (see
  // the type-scale test below), so this stays an alias of the ladder's own
  // basis rather than a second, unconsumed number.
  assert.equal(token(block, "font-base"), "calc(13px * var(--ui-scale))", "--font-base must be calc(13px * var(--ui-scale))");
});

test("`--ui-scale` is spelled by the scale module and nothing else", async () => {
  // R7-13b asserted no TypeScript module read the knob, because there was no
  // scale owner yet. R7-13c makes `src/ui-scale.ts` that owner: it is the one
  // module allowed to spell `--ui-scale` (its `applyUiScale` writes the value),
  // and it is where the step -> multiplier table lives. Every *consumer* stays
  // CSS — no component may compute a second scaled copy of a measurement.
  const { readdir } = await import("node:fs/promises");
  const walk = async (dir: URL): Promise<URL[]> => {
    const out: URL[] = [];
    for (const entry of await readdir(dir, { withFileTypes: true })) {
      const child = new URL(entry.name + (entry.isDirectory() ? "/" : ""), dir);
      if (entry.isDirectory()) out.push(...(await walk(child)));
      else if (/\.tsx?$/.test(entry.name)) out.push(child);
    }
    return out;
  };
  const OWNER = "src/ui-scale.ts";
  // Comments may *name* the knob (they explain the round); the sweep is about
  // code, so prose is stripped first — the same "code, not prose" convention
  // `collapsed-focus.test.ts` uses for its source-shape assertions. A module
  // that only mentions `--ui-scale` in a comment is not a second owner.
  const stripJsComments = (source: string) =>
    source.replace(/\/\*[\s\S]*?\*\//g, "").replace(/\/\/[^\n]*/g, "");
  const offenders: string[] = [];
  for (const file of await walk(new URL("src/", root))) {
    const relative = file.href.slice(root.href.length);
    if (relative === OWNER) continue;
    if (/(?:--ui-scale|--u\b|--font-base)/.test(stripJsComments(await readFile(file, "utf8")))) {
      offenders.push(relative);
    }
  }
  assert.deepEqual(offenders, [], `these modules read the CSS scale knob directly:\n${offenders.join("\n")}`);
  // And the owner really does carry it: a comment that merely mentions the
  // name would satisfy the sweep above while leaving the knob unowned.
  assert.match(
    await read(OWNER),
    /UI_SCALE_CSS_VAR\s*=\s*"--ui-scale"/,
    "ui-scale.ts must declare the one constant that spells the knob",
  );
});

// ── 2 · the converted declarations resolve to their originals ─────────────
// Each entry is [file, selector, property, value-before-R7-13b]. The value is
// read straight out of `git show HEAD:` at review time, so the table is the
// history, not a restatement of what the file happens to say now.

const SAMPLES: [string, string, string, string][] = [
  // base.css
  ["src/styles/base.css", "::-webkit-scrollbar", "width", "6px"],
  ["src/styles/base.css", ".platform-windows .collapsed-shell", "padding", "4px 10px 12px 4px"],
  ["src/styles/base.css", ".platform-windows .terminal-shell", "padding", "10px"],
  ["src/styles/base.css", ".platform-linux .collapsed-shell", "padding", "4px"],
  // launcher.css
  // R22 · the field's row was 56px for eleven rounds and became 48px: the row's
  // only content is the field's 22px line box, so 56px left ~17px of dead height
  // under the text that read as part of the gap to the first result
  // (「我指的这中间的空白太宽了」). R23 · still too wide below the text, so the row
  // is 42px now: the line box keeps 10px a side (22u + 2 × 10u), the deliberate
  // floor below which the field starts to look pinched. The pinned metric moves
  // with the sheet — this entry is the current value, so it is updated rather
  // than dropped.
  // R37 · back to 56px, and deliberately: the user asked for the launcher's
  // field row and the settings card's header to be one height (「可以和设置页面
  // 头部一样高，这样切换时一体性更好」), so the pinned metric is the settings
  // band's own 56px again (see `launcher/search-field.ts`).
  ["src/styles/launcher.css", ".collapsed-card__input-row", "min-height", "56px"],
  ["src/styles/launcher.css", ".collapsed-card__input", "min-height", "22px"],
  ["src/styles/launcher.css", ".collapsed-card__settings", "width", "28px"],
  ["src/styles/launcher.css", ".launcher-result", "height", "42px"],
  ["src/styles/launcher.css", ".launcher-action-bar", "padding", "0 9px 0 11px"],
  ["src/styles/launcher.css", ".launcher-action-bar", "margin-top", "3px"],
  ["src/styles/launcher.css", ".launcher-tip", "padding", "4px 10px 4px 12px"],
  ["src/styles/launcher.css", ".launcher-action-bar__hint", "padding", "2px 6px"],
  // settings.css
  ["src/styles/settings.css", ".settings-card__header", "height", "56px"],
  ["src/styles/settings.css", ".settings-row", "min-height", "44px"],
  ["src/styles/settings.css", ".settings-empty", "min-height", "92px"],
  ["src/styles/settings.css", ".settings-select", "height", "28px"],
  ["src/styles/settings.css", ".settings-switch", "width", "34px"],
  ["src/styles/settings.css", ".settings-switch__thumb", "width", "14px"],
  ["src/styles/settings.css", ".settings-page", "gap", "22px"],
  ["src/styles/settings.css", ".settings-content", "padding", "16px 18px 22px"],
  ["src/styles/settings.css", ".shortcut-recorder", "min-width", "82px"],
  ["src/styles/settings.css", ".update-banner__progress", "min-width", "180px"],
];

// The dimensions this round deliberately leaves as literals, each with the
// reason. The list is the *complete* set: the exhaustive test below scans the
// three target sheets for every retained box literal and requires it to appear
// here, so a value the transform quietly failed to reach cannot hide as an
// "exception". The literals are unchanged this round, so each still resolves
// to the pixels it carried before.
const DEFERRED: [string, string, string, string, string][] = [
  // `pages-apply.test.ts` asserts the literal string for these floors, and the
  // assertion lives in another round's file — converting them is leg 3's edit.
  ["src/styles/settings.css", ".settings-row", "min-height", "44px", "pinned by settings-apple.test.ts (HIG row floor)"],
  ["src/styles/settings.css", ".settings-empty", "min-height", "92px", "pinned by pages-apply.test.ts (placeholder floor)"],
  ["src/styles/settings.css", ".session-manager__row", "min-height", "56px", "pinned by pages-apply.test.ts (session row floor)"],
  ["src/styles/settings.css", ".session-manager > .settings-empty", "min-height", "160px", "page-scoped placeholder floor, pair of the 92px above"],
  ["src/styles/settings.css", ".settings-page", "max-width", "640px", "pinned by settings-apple.test.ts (content column)"],
  ["src/styles/launcher.css", ".launcher-section-title", "padding", "6px 11px 4px", "pinned by pages-apply.test.ts (group-title padding)"],
  // A family of reading measures. They are content widths, not control
  // geometry; leg 3 converts them together so the 640/720/320/220 relation
  // stays one decision.
  ["src/styles/settings.css", ".settings-page--wide", "max-width", "720px", "wide-page reading measure"],
  ["src/styles/settings.css", ".settings-empty__hint", "max-width", "320px", "hint reading measure"],
  ["src/styles/settings.css", ".settings-select", "max-width", "220px", "select reading measure"],
  // Not layout: the ceiling of the open/closed clip animation.
  ["src/styles/launcher.css", ".launcher-bottom-clip--open", "max-height", "600px", "clip-animation ceiling, not a layout size"],
];

// The one partially converted declaration: `.settings-content`'s padding keeps
// its top value (the scroll-edge reservation) while the other two scale.
const DEFERRED_PARTIAL: [string, string, string, string][] = [
  ["src/styles/settings.css", ".settings-content", "padding", "16px 18px 22px"],
];

// Every box property this round's transform could reach. The scan below uses
// it to enumerate what was left behind.
const BOX_PROPERTIES = new Set([
  "height", "min-height", "max-height", "width", "min-width", "max-width",
  "padding", "padding-top", "padding-right", "padding-bottom", "padding-left",
  "margin", "margin-top", "margin-right", "margin-bottom", "margin-left",
  "gap", "row-gap", "column-gap", "top", "right", "bottom", "left", "inset",
]);

// A retained literal: a pure `0`/`Npx` value with no calc and nothing wider
// than 1px (a hairline is exempt by design, not by oversight).
const isRetainedLiteral = (property: string, value: string) => {
  if (!BOX_PROPERTIES.has(property)) return false;
  if (!/\d+px/.test(value)) return false;
  if (/calc\(var\(--u\)/.test(value)) return false;
  if (!value.split(/\s+/).every((t) => /^(0|\d+px)$/.test(t))) return false;
  return !value.split(/\s+/).every((t) => t === "0" || t === "1px");
};

test("the retained literals are exactly the documented deferral list", async () => {
  // The exhaustive half. Converted values and 1px hairlines are excluded by
  // `isRetainedLiteral`; whatever is left must be on DEFERRED. A new literal
  // (or one the transform missed) fails here by name.
  const documented = new Set(DEFERRED.map(([file, selector, property, value]) => `${file}|${selector}|${property}: ${value}`));
  const found: string[] = [];
  for (const file of ["src/styles/base.css", "src/styles/launcher.css", "src/styles/settings.css"]) {
    const css = await read(file);
    for (const { selector, body } of rules(css)) {
      for (const [property, value] of [...body.matchAll(/(?:^|;)\s*([a-z-]+)\s*:\s*([^;]+)/g)].map((m) => [m[1], m[2].trim()])) {
        if (!isRetainedLiteral(property, value)) continue;
        for (const part of selector.split(",")) {
          const key = `${file}|${part.trim()}|${property}: ${value}`;
          if (!found.includes(key)) found.push(key);
        }
      }
    }
  }
  const undocumented = found.filter((key) => !documented.has(key));
  assert.deepEqual(
    undocumented,
    [],
    `these box literals are neither converted nor on the deferral list:\n${undocumented.join("\n")}`,
  );
  // …and the other direction: a stale entry on the list is a reader trap.
  const stale = [...documented].filter((key) => !found.includes(key));
  assert.deepEqual(stale, [], `these deferral entries no longer match the sheets:\n${stale.join("\n")}`);
});

test("the deferred literals still resolve to their original pixels", async () => {
  for (const [file, selector, property, before] of DEFERRED) {
    const body = ruleFor(await read(file), selector).body;
    const value = declarations(body, property)[0];
    assert.ok(value, `${file}: ${selector} must still declare ${property}`);
    assert.deepEqual(resolveValue(value, 1), originalPx(before), `${file}: ${selector} ${property} must not move`);
  }
  for (const [file, selector, property, before] of DEFERRED_PARTIAL) {
    const body = ruleFor(await read(file), selector).body;
    const value = declarations(body, property)[0];
    assert.ok(value, `${file}: ${selector} must still declare ${property}`);
    // Only the leading (deferred) component is pinned; the rest are converted.
    assert.deepEqual(
      resolveValue(value, 1).slice(0, originalPx(before).length),
      originalPx(before),
      `${file}: ${selector} ${property} — the deferred leading value must not move`,
    );
  }
});

test("every sampled dimension resolves to exactly its pre-round pixels at scale 1", async () => {
  // Resolve at the scale the sheet actually declares, not a hardcoded 1: if a
  // later edit moves `--ui-scale`, these samples move with it and fail here as
  // well as in the guard above — the invariant is coupled to the file, not to
  // the test's own constant.
  const declared = Number(token(await rootBlock(), "ui-scale"));
  assert.equal(declared, 1, "the scale this round resolves at is the declared one, and it is 1");
  const cache = new Map<string, string>();
  for (const [file, selector, property, before] of SAMPLES) {
    if (!cache.has(file)) cache.set(file, await read(file));
    const body = ruleFor(cache.get(file)!, selector).body;
    const after = declarations(body, property)[0];
    assert.ok(after, `${file}: ${selector} must still declare ${property}`);
    const resolved = resolveValue(after!, declared);
    assert.deepEqual(
      resolved,
      originalPx(before),
      `${file}: ${selector} ${property} — "${after}" must resolve to "${before}" at --ui-scale: ${declared}`,
    );
  }
});

test("the knob is live: the same samples scale with `--ui-scale`", async () => {
  // The counterpart to the invariant above. If the calc were decorative (a
  // literal that merely looked scaled), this would not move. 1.2 is leg 3's
  // job to ship, not this round's — the point here is that the arithmetic is
  // real before the UI is allowed to choose it.
  const css = await read("src/styles/launcher.css");
  const value = declarations(ruleFor(css, ".collapsed-card__input-row").body, "min-height")[0];
  // R22 moved the row from 56u to 48u and R23 from 48u to 42u; R37 moves it
  // back to 56u (the settings band's height). The arithmetic is what this test
  // is about, so the pair moves with the sheet.
  assert.deepEqual(resolveValue(value, 1), [56]);
  // Float arithmetic: `calc(56px * 1.2)` lands a hair under 67.2, so compare
  // the product rather than requiring the decimal to round-trip.
  assert.deepEqual(resolveValue(value, 1.2), [56 * 1.2], "56 * 1.2");
});

// ── 3 · hairlines are not scaled ──────────────────────────────────────────

test("1px hairlines stay literal: ink, not layout", async () => {
  // A border/divider/ring is a physical line. `calc(var(--u) * 1)` at a 1.2
  // step is 1.2px, which the compositor spreads over two device rows and shows
  // as a grey smear. The transform therefore only touches box dimensions, and
  // every hairline must still read a bare 1px.
  const DIVIDER: [string, string][] = [
    ["src/styles/settings.css", ".settings-card__header::after"],
    ["src/styles/settings.css", ".settings-row__divider"],
  ];
  for (const [file, selector] of DIVIDER) {
    const body = ruleFor(await read(file), selector).body;
    const band = declarations(body, "height")[0];
    assert.equal(band, "1px", `${file}: ${selector} is a hairline and must stay a literal 1px, got "${band}"`);
  }
  // R18 · the launcher left this list, and it left it by drawing *no* divider
  // at all: its field and its list are separated by a brightness step and an
  // 8u transparent breath (R18: 12u; R24 halved it — see
  // `tests/launcher-seam.test.ts`). There is
  // therefore no 1px band in that sheet to scale or to pin — and these two
  // assertions are what keep the round from being read as "the launcher's
  // hairlines were quietly dropped from the census".
  const launcherCss = await read("src/styles/launcher.css");
  assert.ok(
    !/height:\s*1px/.test(launcherCss),
    "the launcher paints no 1px band: it has no dividers to scale",
  );
  assert.ok(
    !/--hairline-fade/.test(launcherCss),
    "and no gradient hairline either",
  );
  // No sheet may write a scaled hairline: `calc(var(--u) * 1)` is the shape
  // that would slip past the rule above.
  for (const file of ["src/styles/base.css", "src/styles/launcher.css", "src/styles/settings.css"]) {
    assert.ok(
      !/calc\(var\(--u\)\s*\*\s*1\)/.test(await read(file)),
      `${file} must not scale a 1px hairline through --u`,
    );
  }
});

// ── 4 · the deferred sheets ───────────────────────────────────────────────

test("extensions / terminal / pinned-card keep box sizes for leg 3", async () => {
  // The brief scopes this round: the three sheets leg 3's launcher/settings/
  // base benefit from are converted; the rest wait, so a wrong conversion
  // cannot hide in a surface nobody is looking at yet. Their type already
  // reads the ladder (a prior round did that), so there is nothing to do here.
  for (const file of ["src/styles/extensions.css", "src/styles/terminal.css", "src/styles/pinned-card.css"]) {
    assert.ok(
      !/calc\(var\(--u\)/.test(await read(file)),
      `${file} must not consume --u this round — its boxes are leg 3's`,
    );
  }
});

test("the terminal font-size is decoupled and marked for leg 3", async () => {
  // Terminal chrome type scales with the interface; the *canvas* text does not
  // (it is the user's `settings.font_size`, painted by render.ts). The marker
  // is what tells leg 3 where the two axes meet.
  const terminal = await read("src/styles/terminal.css");
  assert.match(
    terminal,
    /R7-13b terminal type decoupling/,
    "terminal.css must carry the decoupling marker leg 3 reads",
  );
  // Every font-size in the sheet still comes from the type ladder; none may be
  // a literal that could be mistaken for the canvas's own size.
  for (const { selector, body } of rules(terminal)) {
    for (const size of declarations(body, "font-size")) {
      assert.ok(
        /var\(--text-(caption|body|emphasis|title|display)\)/.test(size),
        `terminal.css: ${selector} font-size: ${size} — terminal chrome reads the type ladder`,
      );
    }
  }
});

// ── 5 · the height measurement is untouched ───────────────────────────────

test("the launcher height measurement reads pixels, not the scale knob", async () => {
  // `syncLauncherHeight` measures the laid-out card with getBoundingClientRect
  // arithmetic and asks the native window for that many logical pixels. It has
  // to keep working while `--ui-scale` moves, so it must not consult the knob:
  // the card already rendered at the scaled size, and the measurement reads
  // whatever came out. This is a guard against leg 3 "helpfully" multiplying.
  const hook = await read("src/hooks/useLauncherHeight.ts");
  assert.doesNotMatch(hook, /ui-scale|--u\b|font-base/, "the measurement must not read the CSS scale knob");
  assert.match(hook, /offsetTop\s*\+\s*last\.offsetHeight/, "the measurement stays the offset arithmetic");
  assert.match(hook, /setSize\(new LogicalSize\(INPUT_WINDOW_WIDTH, height\)\)/, "and still asks for the measured height");
});
