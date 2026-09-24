// R30 · the launcher's readability floor, and the status line that stopped
// pretending to be a result.
//
// The user's report, verbatim: 「窗口半透明过度：背后的 IDE 文字…透过窗口可读——
// 玻璃失去「面」的作用，全是噪点」.
//
// base.css's material model is the HIG lock and this round does not move it: the
// frame's alpha is the transparency slider clamped by `--glass-solid-top` and
// `--glass-frame-floor`, and the haze layer follows it. What the launcher adds is
// a **floor of its own**, scoped to `.collapsed-card`, so the one surface with
// body copy directly on the frame can never be as thin as a decorative sheet.
// The tests below lock the four things that make that honest:
//
//   1. the global formula is untouched (base.css still declares the R8 shape);
//   2. the launcher's floor exists, consumes the accessibility floor as well as
//      its own, and is *local* — no other shell inherits it;
//   3. the resulting fill is deeper but still a material: never opaque, still
//      graded by the glass step's haze, and still following the slider above
//      the floor;
//   4. the light palette gets its own colour, in both of the forms base.css
//      declares it.
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

const root = new URL("../", import.meta.url);
const read = (path: string) => readFile(new URL(path, root), "utf8");
const stripComments = (css: string) => css.replace(/\/\*[\s\S]*?\*\//g, "");

/** Every `selector { body }` rule in a sheet, in order. */
const rules = (css: string): { selector: string; body: string }[] => {
  const out: { selector: string; body: string }[] = [];
  const re = /([^{}]+)\{([^{}]*)\}/g;
  let match: RegExpExecArray | null;
  while ((match = re.exec(css))) {
    out.push({ selector: match[1].trim(), body: match[2] });
  }
  return out;
};

const ruleFor = (css: string, selector: string): string => {
  const rule = rules(css).find(({ selector: s }) => s.split(",").map((p) => p.trim()).includes(selector));
  assert.ok(rule, `${selector} must exist`);
  return rule!.body;
};

/** One custom property's value, from a rule body. */
const token = (body: string, name: string): string => {
  const match = body.match(new RegExp(`--${name}:\\s*([^;]+);`));
  assert.ok(match, `--${name} must be declared`);
  return match![1].trim();
};

const number = (body: string, name: string): number => {
  const value = token(body, name);
  const parsed = Number(value);
  assert.ok(Number.isFinite(parsed), `--${name} must be a number, got "${value}"`);
  return parsed;
};

const launcherCss = async () => stripComments(await read("src/styles/launcher.css"));
const baseCss = async () => stripComments(await read("src/styles/base.css"));

/**
 * The shipped launcher tint formula, evaluated. The *shape* is pinned by the
 * string assertions in the first test; this is the arithmetic those strings
 * mean, so the curve can be asserted without restating a second copy of it.
 */
const launcherTintAlpha = (options: {
  opacity: number;
  floor: number;
  frameFloor: number;
  solidTop: number;
  dim: number;
}): number => {
  const frame = Math.min(options.solidTop, Math.max(options.floor, options.frameFloor, options.opacity));
  return 1 - (1 - options.dim * (1 - options.opacity)) * (1 - frame);
};

// ── A · base.css is untouched ─────────────────────────────────────────────

test("the global material formula is still the R8 one", async () => {
  const base = await baseCss();
  const rootBlock = base.slice(0, base.indexOf("[data-theme="));
  assert.match(
    rootBlock,
    /--glass-frame-alpha:\s*min\(\s*var\(--glass-solid-top\),\s*max\(var\(--glass-frame-floor\),\s*var\(--main-opacity\)\)\s*\)/,
    "the frame alpha must still be min(solid_top, max(frame_floor, transparency))",
  );
  assert.match(
    rootBlock,
    /--glass-tint-alpha:\s*calc\(\s*1 - \(1 - var\(--glass-step-dim\) \* \(1 - var\(--main-opacity\)\)\) \* \(1 - var\(--glass-frame-alpha\)\)\s*\)/,
    "the haze layer must still follow the frame alpha",
  );
  assert.match(rootBlock, /--glass-tint:\s*rgba\(18, 19, 22, var\(--glass-tint-alpha\)\)/);
});

// ── B · the launcher's own floor ──────────────────────────────────────────

test("the launcher declares a local fill floor and consumes the global one", async () => {
  const card = ruleFor(await launcherCss(), ".collapsed-card");
  const floor = number(card, "launcher-glass-floor");
  assert.ok(floor >= 0.7 && floor <= 0.8, `the floor must be a readability step, got ${floor}`);
  const frame = token(card, "launcher-frame-alpha");
  for (const consumed of [
    "var(--launcher-glass-floor)",
    "var(--glass-frame-floor)",
    "var(--main-opacity)",
    "var(--glass-solid-top)",
  ]) {
    assert.ok(
      frame.includes(consumed),
      `the launcher's frame alpha must consume ${consumed}; got "${frame}"`,
    );
  }
  // The haze layer is the shared one: the glass step must still grade this
  // surface, so the step's dimming token is what the local alpha is built on.
  assert.match(token(card, "launcher-tint-alpha"), /var\(--glass-step-dim\)/);
  assert.match(token(card, "launcher-tint"), /var\(--launcher-tint-alpha\)/);

  // The card paints it, and only it — the two rules that set the card's own
  // background (rest and focused) both moved off the global tint.
  for (const selector of [".collapsed-card", ".collapsed-card:focus-within"]) {
    const body = ruleFor(await launcherCss(), selector);
    assert.match(body, /background:[\s\S]*?var\(--launcher-tint\)/, `${selector} must paint the launcher's fill`);
    assert.ok(
      !/var\(--glass-tint\)/.test(body),
      `${selector} must not fall back to the global tint`,
    );
  }

  // Local means local: no other shell reads the launcher's tokens.
  const launcher = await launcherCss();
  const declaredIn = [...launcher.matchAll(/--launcher-(?:glass-floor|frame-alpha|tint-alpha|tint):/g)];
  assert.ok(declaredIn.length >= 4, "the launcher tokens are declared");
  for (const file of ["settings.css", "terminal.css", "extensions.css"]) {
    const css = stripComments(await read(`src/styles/${file}`));
    assert.ok(
      !/--launcher-(glass-floor|frame-alpha|tint-alpha|tint)/.test(css),
      `${file} must not inherit the launcher's floor`,
    );
  }
});

test("the floor deepens the fill without turning the card into a board", async () => {
  const card = ruleFor(await launcherCss(), ".collapsed-card");
  const floor = number(card, "launcher-glass-floor");
  const dim = 0.5; // the default step's haze, `--glass-step-dim` in base.css
  const solidTop = 0.98;

  const before = (opacity: number) => launcherTintAlpha({ opacity, floor: 0, frameFloor: 0, solidTop, dim });
  const after = (opacity: number) =>
    launcherTintAlpha({ opacity, floor, frameFloor: 0, solidTop, dim });

  // The default step: 61% of the desktop came through the tint; now 19% does.
  assert.ok(before(0.47) < 0.65, `sanity: the old fill was thin, got ${before(0.47)}`);
  assert.ok(after(0.47) >= 0.8, `the launcher's default fill must be a face, got ${after(0.47)}`);
  // The bottom of the slider — the state the screenshot was taken in — is
  // lifted the most, and is the point of the floor.
  assert.ok(after(0.1) >= 0.85, `the slider's floor must stay readable, got ${after(0.1)}`);
  assert.ok(after(0.1) - before(0.1) > 0.3, "the floor must be a visible change at the thin end");

  // Still a material, not paint: never opaque, and the slider still moves it
  // above the floor.
  assert.ok(after(0.47) < 0.95, `the launcher must keep some translucency, got ${after(0.47)}`);
  assert.ok(after(1) > after(0.47), "above the floor the slider still decides");
  assert.ok(after(1) <= solidTop, "and the near-solid top is still the ceiling");

  // The accessibility floor still wins over the launcher's own.
  assert.ok(
    launcherTintAlpha({ opacity: 0.1, floor, frameFloor: 0.86, solidTop, dim }) >
      after(0.1),
    "prefers-contrast's 0.86 floor must lift the launcher above its own",
  );

  // The frame itself is untouched: the card still filters, and still draws the
  // shared rim.
  const cardBody = ruleFor(await launcherCss(), ".collapsed-card");
  assert.match(cardBody, /backdrop-filter:\s*blur\(var\(--glass-blur\)\)\s*saturate\(var\(--glass-saturate\)\)/);
  assert.match(cardBody, /box-shadow:\s*var\(--glass-rim-shadow\)/);
});

test("the light palette keeps the launcher's fill light", async () => {
  const launcher = await launcherCss();
  // The explicit attribute and the pre-hydration media block, matching the two
  // forms base.css declares `--glass-tint` in (see `light-theme.test.ts`).
  const attribute = rules(launcher).find(({ selector }) => selector === '[data-theme="light"] .collapsed-card');
  assert.ok(attribute, "the light palette must restate the launcher's fill");
  assert.match(attribute!.body, /--launcher-tint:\s*rgba\(250, 250, 252, var\(--launcher-tint-alpha\)\)/);

  const media = launcher.slice(launcher.indexOf("@media (prefers-color-scheme: light)"));
  assert.match(
    media,
    /html:not\(\[data-theme\]\) \.collapsed-card\s*\{[^}]*--launcher-tint:\s*rgba\(250, 250, 252, var\(--launcher-tint-alpha\)\)/,
    "the pre-hydration light block must cover the launcher too",
  );
  // Only the colour differs between the two palettes — the alpha is shared.
  assert.match(attribute!.body, /var\(--launcher-tint-alpha\)/);
});

// ── C · the status line stopped being a result row ────────────────────────

test("a plugin status line is drawn as a note, not as a row", async () => {
  const launcher = await launcherCss();
  const status = ruleFor(launcher, ".launcher-status");
  // Weaker than a row: shorter, muted, not a control.
  assert.match(status, /min-height:\s*calc\(var\(--u\) \* 30\)/, "the note is shorter than a 42u row");
  assert.match(status, /color:\s*var\(--text-muted\)/, "and reads at the muted step");
  assert.match(status, /font-weight:\s*480/, "and at a weaker weight than a subtitle");
  assert.ok(!/background:/.test(status), "it paints no plate of its own");
  assert.ok(!/box-shadow/.test(status), "and no rim — it is not a control");
  assert.ok(!/cursor:\s*pointer/.test(status), "and it is not clickable");
  // The icon column is the row's own, so the sentence starts on the title's
  // optical left edge, but the glyph carries no plate.
  const icon = ruleFor(launcher, ".launcher-status__icon");
  assert.match(icon, /width:\s*calc\(var\(--u\) \* 28\)/, "the note aligns with the rows' icon column");
  assert.ok(!/background:/.test(icon), "the info glyph has no plate");

  // …and the renderer really branches: the note is a plain div, not the
  // `<button role="option">` a result is.
  const source = (await read("src/launcher/LauncherResults.tsx"))
    .replace(/\/\*[\s\S]*?\*\//g, "")
    .replace(/\/\/[^\n]*/g, "");
  assert.match(
    source,
    /item\.type === "status"[\s\S]{0,400}?className="launcher-status" role="presentation"/,
    "a status item must render as the note element",
  );
  assert.match(source, /InfoIcon/, "with the info glyph, not the row's globe");
  assert.match(
    source,
    /type: "status"; id: string; title: string/,
    "and the launcher item variant is the protocol's own shape",
  );
});

test("the browser row drops its icon plate but keeps the column", async () => {
  const launcher = await launcherCss();
  const icon = ruleFor(launcher, ".launcher-result__icon--browser");
  assert.match(icon, /background:\s*transparent/, "eight identical grey plates was the noise");
  assert.match(icon, /box-shadow:\s*none/, "and the plate's rim goes with it");
  assert.match(icon, /color:\s*var\(--text-muted\)/, "the bare glyph is muted");
  assert.ok(!/width:/.test(icon), "the 28u column is kept so titles stay on the launcher's own edge");
});
