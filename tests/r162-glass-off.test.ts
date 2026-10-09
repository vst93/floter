// R162 · the fourth glass position: **off**.
//
// The user's macOS retest of v0.3.13 is still 「非常卡，设置页/列表滚动掉帧」.
// "Reduce motion" only shortens animation durations — it cannot touch the nine
// `backdrop-filter` shells, which re-sample the window on every scrolled frame,
// and `prefers-reduced-transparency` has never shipped in WebKit (R104/R161
// verified that twice, and R104 deleted the tombstone block rather than
// re-plant one). So the lever has to be the app's own, and this round adds it
// as a fourth position of the existing glass control rather than as a new
// settings field: `glass_step` gains the id `off`, `[data-glass="off"]`
// neutralises the effect tokens, and the three frame shells stop filtering
// altogether.
//
// The guards below lock the four things that make the switch honest:
//
//   1. the vocabulary — `off` is a shipped id, it normalizes to itself, it is
//      the thinnest position, and the legacy migration table is untouched;
//   2. the off block — every effect token neutralised, and the three shells
//      carrying *no* filter (both spellings) over `--surface-opaque`, with no
//      colour of their own;
//   3. the default path is byte-for-byte unchanged: the three stops keep
//      10 / 22 / 28px and each shell keeps its own blur, read from the shell's
//      *own* rule so the off block's comma group cannot satisfy the scan;
//   4. the wiring — a fourth segment, and its label in both dictionaries.
//
// The Rust premise stated in the task book ("Rust stores the string with no
// value domain") is false, and the report records it: `normalize_glass_step`
// in `src-tauri/src/commands/config.rs` maps an unknown id back to `regular`
// on both the read path and every save. This round is scoped to TS/CSS, so
// nothing here asserts the Rust domain; the guard would only have to change if
// a later round adds `off` to the loader's own list.
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

import { decl, ruleFor, rules, stripComments } from "./css.ts";

const root = new URL("../", import.meta.url);
const read = (path: string) => readFile(new URL(path, root), "utf8");

/** Strip JSX/JS comments so prose that merely names a symbol cannot satisfy a
 *  scan (the R79 convention). */
const stripJsComments = (source: string) =>
  source.replace(/\/\*[\s\S]*?\*\//g, "").replace(/^\s*\/\/.*$/gm, "");

/** The one rule whose selector group *is* `selector` — not a comma group that
 *  merely contains it. The off block ships both a token rule and a shell rule
 *  group, so "which rule did I read" has to be exact (the R161 `soleRule`
 *  convention). */
const soleRule = (css: string, selector: string) => {
  const rule = rules(css).find(({ selector: s }) => s === selector);
  assert.ok(rule, `${selector} must be its own rule in base.css`);
  return rule!.body;
};

/** The rule whose selector group contains `selector` as one of its parts. */
const groupFor = (css: string, selector: string) => {
  const rule = rules(css).find(({ selector: s }) =>
    s.split(",").map((part) => part.trim()).includes(selector),
  );
  assert.ok(rule, `${selector} must be declared`);
  return rule!;
};

const SHELLS = [
  ["launcher.css", ".collapsed-card", "--glass-blur"],
  ["settings.css", ".settings-card", "--glass-blur"],
  ["terminal.css", ".terminal-panel", "--glass-blur-terminal"],
] as const;

const baseCss = async () => stripComments(await read("src/styles/base.css"));

// ── 1 · the vocabulary ─────────────────────────────────────────────────────

test("`off` is a shipped step id, and it is the thinnest position", async () => {
  const { GLASS_STEPS, normalizeGlassStep } = await import("../src/glass-material.ts");
  // The control's display order, thinnest first: off is the first position, so
  // the segmented control paints it leftmost.
  assert.deepEqual([...GLASS_STEPS], ["off", "frosted", "regular", "liquid"]);
  for (const step of GLASS_STEPS) assert.equal(normalizeGlassStep(step), step);
  // It is normalized like every other id — case- and whitespace-insensitive.
  assert.equal(normalizeGlassStep("OFF"), "off");
  assert.equal(normalizeGlassStep(" off "), "off");
  // The pre-GLASS-3STOP migration table is deliberately untouched by this
  // round: `off` is a new id, not a re-pointed old one.
  for (const [old, next] of [
    ["low", "frosted"],
    ["mid", "regular"],
    ["high", "liquid"],
    ["deep", "liquid"],
    ["jelly", "liquid"],
  ] as const) {
    assert.equal(normalizeGlassStep(old), next, `${old} must still migrate to ${next}`);
  }
  // …and an unknown id still rests on the balanced stop, never on off: a
  // hand-edited file must not be able to silently remove the material.
  for (const unknown of [undefined, null, "", "clear", "offf", "offf ", 0, {}, "true"]) {
    assert.equal(
      normalizeGlassStep(unknown),
      "regular",
      `${JSON.stringify(unknown)} must normalize to regular, not to off`,
    );
  }
});

test("`off` is the control's own position and writes the same one field", async () => {
  const { GLASS_INTENSITIES, glassIntensityOf, glassIntensitySettings } =
    await import("../src/glass-material.ts");
  // The reverse lookup: a stored `off` lights the off segment, at any
  // transparency — the lookup stays opacity-independent.
  for (const transparency of [undefined, 0, 0.1, 0.47, 0.95, 1]) {
    assert.equal(
      glassIntensityOf("off", transparency),
      "off",
      `a stored off must display on the off position at ${transparency}`,
    );
  }
  // It writes `glass_step` and nothing else, exactly like a stop: the round
  // must not resurrect the GLASS-UNIFY tint coupling.
  const written = glassIntensitySettings("off");
  assert.deepEqual(Object.keys(written), ["glass_step"]);
  assert.equal(written.glass_step, "off");
  assert.equal(glassIntensityOf(written.glass_step), "off", "off must round-trip through the control");
  // The three effect stops are untouched: same levels, same round trip.
  assert.deepEqual([...GLASS_INTENSITIES], [1, 2, 3]);
  for (const level of GLASS_INTENSITIES) {
    const stop = glassIntensitySettings(level);
    assert.deepEqual(Object.keys(stop), ["glass_step"]);
    assert.equal(glassIntensityOf(stop.glass_step), level, `stop ${level} must round-trip`);
  }
  // A stop still never lands on off, and off never lands on a stop.
  for (const level of GLASS_INTENSITIES) {
    assert.notEqual(glassIntensitySettings(level).glass_step, "off");
  }
  assert.notEqual(glassIntensityOf("off"), 1);
});

// ── 2 · the off block ──────────────────────────────────────────────────────

test("the off block neutralises every effect token, haze included", async () => {
  const css = await baseCss();
  const block = soleRule(css, '[data-glass="off"]');
  assert.equal(decl(block, "--glass-step-blur"), "0px", "off must zero the blur token");
  assert.equal(decl(block, "--glass-step-saturate"), "100%", "off must drop the saturation lift");
  assert.equal(decl(block, "--glass-lens-scale"), "0", "off must drop the control lens");
  // The haze is 0 for the reason the `prefers-contrast: more` override already
  // documents: the veil stands in for a *thin* panel's readability, and off
  // mode's shells are painted opaque, so a veil on them would be mud.
  assert.equal(decl(block, "--glass-step-dim"), "0", "off must drop the haze layer");
  // The step never supplies alpha, on any position.
  assert.ok(!/--glass-step-fill/.test(block), "off must not declare a fill floor");

  // The terminal shell reads an alias, not the token: the alias chain has to
  // resolve to the same 0px or the terminal would keep a blur radius.
  const rootBlock = soleRule(css, ":root");
  for (const alias of ["--glass-blur", "--glass-blur-terminal"]) {
    assert.match(
      decl(rootBlock, alias)!,
      /var\(--glass-step-blur\)/,
      `${alias} must stay an alias of the step's blur, so off zeroes it too`,
    );
  }
  // The plugin-facing mirror carries off's haze as well (it is the hand-off's
  // source, asserted against the same block).
  const { GLASS_STEP_TOKENS } = await import("../src/glass-material.ts");
  assert.equal(
    GLASS_STEP_TOKENS.off.dim,
    Number(decl(block, "--glass-step-dim")),
    "GLASS_STEP_TOKENS.off.dim must equal [data-glass=\"off\"] --glass-step-dim",
  );
});

test("the three shells drop the filter entirely and take the near-solid stand-in", async () => {
  const css = await baseCss();
  const group = groupFor(css, '[data-glass="off"] .collapsed-card');
  const parts = group.selector.split(",").map((part) => part.trim());
  // One rule group naming exactly the three shells, so they cannot drift apart.
  for (const [file, shell] of SHELLS) {
    assert.ok(
      parts.includes(`[data-glass="off"] ${shell}`),
      `${shell} (${file}) must be in the off rule group, got ${group.selector}`,
    );
  }
  const body = group.body;
  // Both spellings, or a WebKitGTK build would keep blurring while the sheet
  // claims it does not. `none` — not `blur(0px)`: a zero-radius filter still
  // costs a composited layer per shell, which is the whole cost this round is
  // removing.
  assert.match(body, /(?:^|;)\s*-webkit-backdrop-filter:\s*none\s*;/, "the prefixed filter must be none");
  assert.match(body, /(?:^|;)\s*backdrop-filter:\s*none\s*;/, "the standard filter must be none");
  assert.ok(!/blur\(/.test(body), `off must not keep a blur radius, got ${body}`);
  assert.ok(!/saturate\(/.test(body), `off must not keep a saturation filter, got ${body}`);
  // The fill is the existing near-solid stand-in — the token the retired
  // reduced-transparency fallback already used for "stops being see-through" —
  // so the palette (and the light theme) follow it with no colour here.
  assert.equal(decl(body, "background"), "var(--surface-opaque)");
  assert.ok(
    !/#[0-9a-f]{3,8}\b|rgba?\(|hsla?\(/i.test(body),
    `the off fill must not carry a colour literal, got ${body}`,
  );

  // It lives in base.css as `[data-glass="off"] .shell` (0-2-0), which beats
  // each surface sheet's own 0-1-0 shell rule without depending on import
  // order. A surface sheet that branched on the step would break that.
  for (const [file] of SHELLS) {
    const sheet = stripComments(await read(`src/styles/${file}`));
    assert.ok(
      !sheet.includes("data-glass"),
      `${file} must not branch on the step — the off rule is written once in base.css`,
    );
  }
});

// ── 3 · the default path is unchanged ──────────────────────────────────────

test("the default path keeps its three stops and its three filters", async () => {
  const css = await baseCss();
  // The three stops' own rules, read standalone: the off block is a separate
  // rule and must not have moved the resting values.
  const stepBlurs: Record<string, string> = {};
  for (const step of ["frosted", "regular", "liquid"]) {
    stepBlurs[step] = decl(soleRule(css, `[data-glass="${step}"]`), "--glass-step-blur")!;
  }
  assert.deepEqual(stepBlurs, { frosted: "10px", regular: "22px", liquid: "28px" });
  assert.equal(
    decl(soleRule(css, "html:not([data-glass])"), "--glass-step-blur"),
    "22px",
    "the pre-hydration default is still regular",
  );
  // …and each shell still filters with its aliased blur in its own sheet. The
  // off block's comma group is deliberately *not* what satisfies this: the
  // shell's own rule is read, so "the default path lost its filter" stays red.
  for (const [file, selector, token] of SHELLS) {
    const sheet = stripComments(await read(`src/styles/${file}`));
    const body = ruleFor(sheet, selector);
    assert.match(
      body,
      new RegExp(`backdrop-filter:\\s*blur\\(var\\(${token.replace(/[-]/g, "\\-")}\\)\\)`),
      `${file}: ${selector} must keep its blur on the default path`,
    );
  }
  // No `!important` anywhere in the addition: the switch is specificity, not a
  // sledgehammer.
  const offSlices = [soleRule(css, '[data-glass="off"]'), groupFor(css, '[data-glass="off"] .collapsed-card').body];
  for (const slice of offSlices) {
    assert.ok(!slice.includes("!important"), "the off block must not need !important");
  }
});

// ── 4 · the wiring: a fourth segment, labelled in both dictionaries ────────

test("the glass control has a fourth segment, first in display order", async () => {
  const page = stripJsComments(await read("src/settings/GeneralPage.tsx"));
  // The option list: the off position, then the three stops off the shared
  // table — so the control's order is `off → frosted → regular → liquid`.
  const start = page.indexOf("const GLASS_INTENSITY_OPTIONS");
  assert.notEqual(start, -1, "the option list must exist");
  const options = page.slice(start, page.indexOf("];", start));
  assert.match(options, /value: "off",\s*labelKey: "settings\.glassIntensity\.off"/);
  assert.ok(
    options.indexOf('"off"') < options.indexOf("GLASS_INTENSITIES.map"),
    `off must be the control's first position, got ${options}`,
  );
  assert.match(page, /data-glass-intensity=\{option\.value\}/, "each segment carries its position");
  // The control's value vocabulary is the four-position one, not the
  // three-stop one — `off` is not a stop with a blur/saturate/lens triple.
  assert.match(page, /value: GlassSelection/, "the control's value type must admit off");
  assert.match(
    page,
    /onChangeGlassIntensity: \(level: GlassSelection\) => void/,
    "the page's callback must accept the off position",
  );
  // …and the mutator that persists it takes the same vocabulary and writes the
  // same single field.
  const hook = await read("src/hooks/useSettings.ts");
  assert.match(hook, /\(level: GlassSelection\)/, "the mutator must accept the off position");
  assert.match(hook, /glassIntensitySettings\(level\)/, "the mutator derives the step from the shared table");
});

test("the off label is declared in both dictionaries", async () => {
  const i18n = await read("src/i18n.ts");
  const occurrences = i18n.match(/"settings\.glassIntensity\.off":/g) ?? [];
  assert.equal(occurrences.length, 2, "settings.glassIntensity.off must exist in en and zh");
  // Paired, not merely present: the English source of truth says Off and the
  // Chinese one says 关闭.
  assert.match(i18n, /"settings\.glassIntensity\.off": "Off"/);
  assert.match(i18n, /"settings\.glassIntensity\.off": "关闭"/);
  // The existing keys are untouched by the addition.
  for (const key of ["settings.glassIntensity", "settings.glassIntensity.1", "settings.glassIntensity.2", "settings.glassIntensity.3"]) {
    assert.equal(
      (i18n.match(new RegExp(`"${key.replace(/\./g, "\\.")}":`, "g")) ?? []).length,
      2,
      `${key} must stay declared in both dictionaries`,
    );
  }
});

test("the off step survives the Rust-side whitelist (restart round-trip)", async () => {
  // The loader normalizes the stored id through the Rust whitelist; a step the
  // backend does not list is silently rewritten to `regular` on the first
  // restart. `off` must be a real stored value, so the Rust domain has to list
  // it — the same four-id array the node-side GlassStep union mirrors.
  const rust = await read("src-tauri/src/commands/config.rs");
  assert.match(
    rust,
    /const GLASS_STEPS: \[&str; 4\] = \["off", "frosted", "regular", "liquid"\]/,
    "the Rust whitelist must list `off` or a restart rewrites the user's choice",
  );
  assert.match(rust, /normalize_glass_step\("off"\), "off"/, "the normalizer test must pin the round-trip");
});
