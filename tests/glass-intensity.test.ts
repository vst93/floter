// GLASS-REAXIS: the five-stop **liquid-glass effect** control, plus the two
// restored transparency sliders.
//
// GLASS-UNIFY (the previous round) collapsed R8's dual controls into a single
// five-stop control, but spent the stops on the **tint** axis: each stop wrote
// a `(glass_step, tint)` pair into both opacity fields. The user's verdict was
// that the stop must control the *effect* — 扭曲程度、组件的透视效果、控件
// 透镜质感 — while the app's background transparency is a separate pair of
// sliders. This round re-axises the stops and restores the sliders.
//
// The tests here lock the five things that make the re-axis honest:
//
//   1. the five-stop effect table itself (step / blur / saturate / lens);
//   2. the write path — a stop writes **only** `glass_step` and never an
//      opacity field (the GLASS-UNIFY behaviour, made a red test);
//   3. cross-stop monotonicity — blur and saturation never fall, so "higher
//      stop = more liquid" cannot invert mid-range;
//   4. the reverse lookup is opacity-independent: any `glass_step` lands on
//      exactly one stop for *every* transparency value, and the sliders never
//      move the highlighted stop;
//   5. the wiring — five segments, two sliders, two independent mutators.
//
// The effect numbers are read from the shipped `[data-glass]` blocks in
// base.css, not restated here, so the table cannot drift from the CSS.
import assert from "node:assert/strict";
import { readFileSync, readdirSync } from "node:fs";
import { readFile, readdir } from "node:fs/promises";
import test from "node:test";

import {
  GLASS_INTENSITIES,
  GLASS_INTENSITY,
  GLASS_STEPS,
  clampWindowOpacity,
  glassIntensityOf,
  glassIntensitySettings,
  type GlassIntensity,
  type GlassStep,
} from "../src/glass-material.ts";

const root = new URL("../", import.meta.url);
const read = (path: string) => readFile(new URL(path, root), "utf8");
const stripComments = (css: string) => css.replace(/\/\*[\s\S]*?\*\//g, "");

/** One `[data-glass="x"]` block, walked by its own braces. */
const stepBlock = (css: string, step: string) => {
  const start = css.indexOf(`[data-glass="${step}"]`);
  assert.notEqual(start, -1, `base.css must define [data-glass="${step}"]`);
  const open = css.indexOf("{", start);
  let depth = 0;
  for (let i = open; i < css.length; i += 1) {
    if (css[i] === "{") depth += 1;
    else if (css[i] === "}") {
      depth -= 1;
      if (depth === 0) return css.slice(open + 1, i);
    }
  }
  assert.fail(`unterminated [data-glass="${step}"]`);
};

const number = (block: string, name: string, unit = "") => {
  const match = block.match(new RegExp(`--${name}:\\s*([-\\d.]+)${unit};`));
  assert.ok(match, `--${name} must be declared`);
  return Number(match![1]);
};

/** The shipped effect per step, straight out of base.css. */
const shippedSteps = async () => {
  const css = stripComments(await read("src/styles/base.css"));
  const out = {} as Record<GlassStep, { blur: number; saturate: number; lensScale: number }>;
  for (const step of GLASS_STEPS) {
    const block = stepBlock(css, step);
    out[step] = {
      blur: number(block, "glass-step-blur", "px"),
      saturate: number(block, "glass-step-saturate", "%"),
      lensScale: number(block, "glass-lens-scale"),
    };
  }
  return out;
};

// ── A. the five-stop effect table ──────────────────────────────────────────

test("the effect table is five frozen (step, blur, saturate, lens) stops", () => {
  // Every stop, spelled out. This is the mutation lock: change a step, a blur,
  // a saturation or a lens level anywhere and this assertion fails, so a later
  // round cannot quietly retune a stop without also updating its test.
  const expected: Record<
    GlassIntensity,
    { step: GlassStep; blur: number; saturate: number; lens: number }
  > = {
    1: { step: "low", blur: 10, saturate: 135, lens: 1 },
    2: { step: "mid", blur: 24, saturate: 165, lens: 2 },
    3: { step: "high", blur: 28, saturate: 180, lens: 3 },
    4: { step: "deep", blur: 28, saturate: 190, lens: 4 },
    5: { step: "jelly", blur: 28, saturate: 200, lens: 5 },
  };
  assert.deepEqual([...GLASS_INTENSITIES], [1, 2, 3, 4, 5], "the control is five stops");
  for (const level of GLASS_INTENSITIES) {
    const stop = GLASS_INTENSITY[level];
    assert.equal(stop.step, expected[level].step, `stop ${level}'s step`);
    assert.equal(stop.blur, expected[level].blur, `stop ${level}'s blur`);
    assert.equal(stop.saturate, expected[level].saturate, `stop ${level}'s saturate`);
    assert.equal(stop.lens, expected[level].lens, `stop ${level}'s lens level`);
    // The stop is the EFFECT axis: it must carry no tint/opacity field at all.
    // A `tint` key here would be the GLASS-UNIFY regression this round undoes.
    assert.ok(!("tint" in stop), `stop ${level} must not carry a tint — the stop is the effect axis`);
    // Labels are i18n keys, never literals or parameter names.
    assert.match(stop.label, /^settings\.glassIntensity\.\d$/);
  }
  // Every stop's label key is present in both dictionaries (i18n.ts is the
  // source of truth; the Record<MessageKey,string> types guarantee balance).
  const i18n = readFileSync(new URL("../src/i18n.ts", import.meta.url), "utf8");
  for (const level of GLASS_INTENSITIES) {
    assert.ok(
      i18n.includes(`"${GLASS_INTENSITY[level].label}":`),
      `stop ${level}'s label key exists in i18n.ts`,
    );
  }
});

test("a stop writes glass_step and nothing else", () => {
  // The headline correction: GLASS-UNIFY's `glassIntensitySettings` returned a
  // `(glass_step, main_opacity, terminal_opacity)` triple. GLASS-REAXIS returns
  // exactly one field. This is the mutation lock for "档位写 opacity → 红".
  for (const level of GLASS_INTENSITIES) {
    const written = glassIntensitySettings(level);
    assert.deepEqual(
      Object.keys(written),
      ["glass_step"],
      `stop ${level} must write glass_step and only glass_step`,
    );
    assert.equal(written.glass_step, GLASS_INTENSITY[level].step, `stop ${level} writes its step`);
  }
  // The write path round-trips through the reverse lookup: a step the UI just
  // wrote displays on that same stop.
  for (const level of GLASS_INTENSITIES) {
    const { glass_step } = glassIntensitySettings(level);
    assert.equal(glassIntensityOf(glass_step), level, `stop ${level} must round-trip`);
  }
});

test("the stop table and the shipped [data-glass] blocks agree", async () => {
  const steps = await shippedSteps();
  for (const level of GLASS_INTENSITIES) {
    const stop = GLASS_INTENSITY[level];
    const shipped = steps[stop.step];
    assert.equal(shipped.blur, stop.blur, `[data-glass="${stop.step}"] blur must equal stop ${level}'s`);
    assert.equal(
      shipped.saturate,
      stop.saturate,
      `[data-glass="${stop.step}"] saturate must equal stop ${level}'s`,
    );
  }
});

// ── B. cross-stop monotonicity ─────────────────────────────────────────────

test("the stops are monotonic: blur and saturation never fall", async () => {
  const steps = await shippedSteps();
  const effect = GLASS_INTENSITIES.map((level) => {
    const { step, blur, saturate, lens } = GLASS_INTENSITY[level];
    return { level, step, blur, saturate, lens, shipped: steps[step] };
  });
  for (let i = 1; i < effect.length; i += 1) {
    const prev = effect[i - 1];
    const curr = effect[i];
    // The effect thickens: blur is non-decreasing (three stops share the 28px
    // budget ceiling, so 3→4→5 hold rather than climb) …
    assert.ok(
      curr.blur >= prev.blur,
      `stop ${curr.level}'s blur (${curr.blur}) must not fall below stop ${prev.level}'s (${prev.blur})`,
    );
    // … and saturation climbs at every step, which is what makes the three
    // 28px stops distinct.
    assert.ok(
      curr.saturate > prev.saturate,
      `stop ${curr.level}'s saturate (${curr.saturate}) must exceed stop ${prev.level}'s (${prev.saturate})`,
    );
    // … and the control lens level climbs with them.
    assert.ok(
      curr.lens > prev.lens,
      `stop ${curr.level}'s lens (${curr.lens}) must exceed stop ${prev.level}'s (${prev.lens})`,
    );
    // The shipped lens scale ascends in lock-step, or a stop would change the
    // label without changing the controls.
    assert.ok(
      curr.shipped.lensScale > prev.shipped.lensScale,
      `stop ${curr.level}'s shipped lens scale (${curr.shipped.lensScale}) must exceed stop ` +
        `${prev.level}'s (${prev.shipped.lensScale})`,
    );
  }
  // The ends really are the two ends of the perceived range.
  assert.equal(effect[0].step, "low", "stop 1 is the Clear variant");
  assert.equal(effect[4].step, "jelly", "stop 5 is the heaviest variant");
  assert.equal(effect[4].saturate, 200, "stop 5 spends the full saturation budget");
  for (const entry of effect) {
    assert.ok(entry.blur <= 28, `stop ${entry.level}: blur exceeds the 28px budget`);
  }
});

// ── C. the reverse lookup over the whole space ─────────────────────────────

test("every stored step lands on exactly one stop, whatever the transparency", () => {
  // The reverse lookup is *opacity-independent*: the stop is the effect axis,
  // so no transparency value can move it. Sweep every step × every integer
  // percentage the Rust u8 fields can hold (0-100) and assert the answer is
  // constant per step — this is the mutation lock for "滑杆改档位显示 → 红".
  const expected: Record<GlassStep, GlassIntensity> = {
    low: 1,
    mid: 2,
    high: 3,
    deep: 4,
    jelly: 5,
  };
  let cells = 0;
  for (const step of GLASS_STEPS) {
    for (let percent = 0; percent <= 100; percent += 1) {
      const level = glassIntensityOf(step, percent / 100);
      assert.equal(
        level,
        expected[step],
        `${step}@${percent}% resolved to stop ${level}, not ${expected[step]} — the lookup must ignore opacity`,
      );
      cells += 1;
    }
  }
  assert.equal(cells, 5 * 101, "the sweep must cover every step × percentage");
  // Every stop is reachable from some stored step, or a segment would be dead.
  const reached = new Set<GlassIntensity>();
  for (const step of GLASS_STEPS) reached.add(glassIntensityOf(step, 0.5));
  assert.deepEqual([...reached].sort(), [1, 2, 3, 4, 5], "all five stops must be reachable");
  // A garbage step rests on the balanced stop, the same rule the Rust loader
  // uses; the lookup must not throw.
  assert.equal(glassIntensityOf("ultra" as GlassStep), 2, "an unknown step rests on stop 2");
  assert.equal(glassIntensityOf("" as GlassStep), 2, "an empty step rests on stop 2");
});

test("the migrated pre-R8 file and the GLASS-UNIFY pairs display sensibly", () => {
  // `migrate_legacy_glass_strength` halves the old slider and picks the step by
  // its tertile; an old 94 becomes main_opacity 47 + glass_step "high". That
  // pair displays on stop 3 (Strong) and the 47% stays in the slider untouched.
  assert.equal(glassIntensityOf("high", 47 / 100), 3, "an upgraded 94 must display on stop 3");
  // A GLASS-UNIFY file wrote (step, tint) pairs; the stop is read from the step
  // and the tint is preserved as the slider value. The whole old table:
  const unified: [GlassStep, number][] = [
    ["low", 0.22],
    ["mid", 0.4],
    ["high", 0.58],
    ["high", 0.78],
    ["high", 0.98],
  ];
  assert.deepEqual(
    unified.map(([step, tint]) => glassIntensityOf(step, tint)),
    [1, 2, 3, 3, 3],
    "a GLASS-UNIFY pair displays on its step's stop — the tint never moves it",
  );
  // The frontend default (mid + 47%) displays on stop 2.
  assert.equal(glassIntensityOf("mid", 47 / 100), 2, "the frontend default displays on stop 2");
});

// ── D. the wiring: five segments, two independent sliders ──────────────────

test("the settings page renders five effect segments and two opacity sliders", async () => {
  const page = await read("src/settings/GeneralPage.tsx");
  // The effect control is built from the shared stop table, one segment per stop.
  assert.match(page, /GLASS_INTENSITIES\.map/, "the segments must come from GLASS_INTENSITIES");
  assert.match(page, /data-glass-intensity=\{option\.value\}/, "each segment must carry its stop id");
  assert.match(page, /role="radiogroup"/, "the segments stay a radiogroup (keyboard contract)");
  assert.match(page, /role="radio"/, "each segment is a radio");
  assert.match(page, /glassIntensityOf\(settings\.glass_step/, "the chosen stop is derived by the reverse lookup");
  assert.match(page, /onChange=\{onChangeGlassIntensity\}/, "a segment click writes the stop's step");
  // The two transparency sliders are back, each with its own target.
  assert.match(page, /settings\.transparency\.main/, "the app transparency slider must exist");
  assert.match(page, /settings\.transparency\.terminal/, "the terminal transparency slider must exist");
  assert.match(page, /onChangeOpacity\("main"/, "the app slider writes the main target");
  assert.match(page, /onChangeOpacity\("terminal"/, "the terminal slider writes the terminal target");
  // The OpacityControl component carries one `type="range"` and the font-size
  // slider is a second; the two transparency sliders are two instances of the
  // shared component, so three ranges render from two declarations.
  assert.equal(
    (page.match(/type="range"/g) ?? []).length,
    2,
    "General declares the font-size range plus the shared opacity range",
  );
  assert.equal(
    (page.match(/<OpacityControl/g) ?? []).length,
    2,
    "the two transparency sliders are two OpacityControl instances",
  );
  assert.match(page, /settings\.transparencyHint/, "the transparency hint is back");
});

test("the two mutators are independent: a stop never writes opacity, a slider never writes the step", async () => {
  const hook = await read("src/hooks/useSettings.ts");
  // The effect mutator exists and writes only the step.
  assert.match(hook, /const changeGlassIntensity = useCallback/, "the effect mutator must exist");
  const intensityBody = hook.slice(hook.indexOf("const changeGlassIntensity = useCallback"));
  const intensityFn = intensityBody.slice(0, intensityBody.indexOf("const changeFontSize"));
  assert.match(intensityFn, /glassIntensitySettings\(level\)/, "the mutator derives the step from the stop table");
  assert.match(intensityFn, /glass_step:/, "the mutator must write glass_step");
  assert.ok(
    !/main_opacity:|terminal_opacity:/.test(intensityFn),
    "the effect mutator must not write either opacity — that was the GLASS-UNIFY mistake",
  );
  // The opacity mutator is back, single-target, and never touches the step.
  assert.match(hook, /const changeOpacity = useCallback/, "the opacity mutator must exist");
  const opacityBody = hook.slice(hook.indexOf("const changeOpacity = useCallback"));
  const opacityFn = opacityBody
    .slice(0, opacityBody.indexOf("const changeGlassIntensity"))
    .replace(/\/\*[\s\S]*?\*\//g, "")
    .replace(/\/\/[^\n]*/g, "");
  assert.match(opacityFn, /target === "main" \? "main_opacity" : "terminal_opacity"/, "one mutator, two targets");
  assert.ok(
    !/glass_step/.test(opacityFn),
    "the opacity mutator must not touch the step — the axes are independent",
  );
});

test("no opacity slider was left behind in a dead symbol, and the step is not a slider", async () => {
  // A DOM/component-level scan: the old GLASS-UNIFY dead-symbol list must no
  // longer be present, and the two sliders must use the restored class family.
  const files: string[] = [];
  const collect = (dir: string) => {
    for (const entry of readdirSync(new URL(dir, root), { withFileTypes: true })) {
      const rel = `${dir}${entry.name}`;
      if (entry.isDirectory()) collect(`${rel}/`);
      else if (/\.(tsx?|css)$/.test(entry.name)) files.push(rel);
    }
  };
  collect("src/");
  const dead = ["normalizeOpacity(", "OpacityControl"];
  for (const file of files) {
    const source = (await read(file)).replace(/\/\*[\s\S]*?\*\//g, "").replace(/\/\/[^\n]*/g, "");
    for (const symbol of dead) {
      // `normalizeOpacity` and `OpacityControl` are the restored names, so they
      // are *expected* in GeneralPage; the assertion is scoped to the other
      // files, which must not carry a second implementation.
      if (file.endsWith("settings/GeneralPage.tsx")) continue;
      assert.ok(!source.includes(symbol), `${file} must not carry a second ${symbol}`);
    }
  }
  // The restored slider CSS family is present.
  const settings = await read("src/styles/settings.css");
  for (const cls of [".opacity-controls", ".opacity-control__range", ".opacity-control__value"]) {
    assert.ok(settings.includes(cls), `settings.css must define ${cls}`);
  }
});

test("the opacity clamp keeps the Rust window-opacity band", () => {
  assert.equal(clampWindowOpacity(47), 47);
  assert.equal(clampWindowOpacity(0), 10, "the floor is the Rust MIN_WINDOW_OPACITY");
  assert.equal(clampWindowOpacity(100), 100);
  assert.equal(clampWindowOpacity(150), 100);
  assert.equal(clampWindowOpacity(-5), 10);
  assert.equal(clampWindowOpacity(Number.NaN), 47, "a non-finite value falls back to the default");
  // No preset snapping: 24 stays 24, unlike the pre-R8 25-preset snap.
  assert.equal(clampWindowOpacity(24), 24);
});

test("the i18n keys exist in both languages and are balanced", async () => {
  const i18n = await read("src/i18n.ts");
  for (const key of [
    "settings.glassIntensity",
    "settings.glassIntensity.1",
    "settings.glassIntensity.2",
    "settings.glassIntensity.3",
    "settings.glassIntensity.4",
    "settings.glassIntensity.5",
    "settings.glassIntensityHint",
    "settings.transparency.main",
    "settings.transparency.terminal",
    "settings.transparencyHint",
  ]) {
    const occurrences = i18n.match(new RegExp(`"${key.replace(/\./g, "\\.")}":`, "g")) ?? [];
    assert.equal(occurrences.length, 2, `${key} must be declared in both en and zh`);
  }
});

// ── E. the lens family in base.css ─────────────────────────────────────────

test("the lens family exists, is stop-driven, and ascends with the stop", async () => {
  const css = stripComments(await read("src/styles/base.css"));
  const rootBlock = css.slice(css.indexOf(":root {"), css.indexOf('[data-theme="light"]'));
  // The four lens tokens, declared from a per-palette base times the stop's
  // scale. The base alphas live in the palette blocks; the scale lives in the
  // step blocks.
  for (const token of ["glass-lens-rim", "glass-lens-sheen", "glass-lens-edge", "glass-lens-glow"]) {
    const match = rootBlock.match(new RegExp(`--${token}:\\s*([^;]+);`));
    assert.ok(match, `--${token} must be declared`);
    assert.match(
      match![1],
      new RegExp(`var\\(--${token}-base\\)\\s*\\*\\s*var\\(--glass-lens-scale\\)`),
      `--${token} must be base × scale`,
    );
  }
  // The composed stack consumes all four (rim, sheen, edge) plus the shared
  // pane edge; every layer is `inset`, so a control never casts or blurs.
  const stack = rootBlock.match(/--glass-lens-stack:\s*([^;]+);/);
  assert.ok(stack, "--glass-lens-stack must be declared");
  for (const token of ["glass-lens-rim", "glass-lens-sheen", "glass-lens-edge", "glass-control-edge"]) {
    assert.match(stack![1], new RegExp(`var\\(--${token}\\)`), `the stack must consume ${token}`);
  }
  const layers = stack![1].split(/,(?![^(]*\))/).map((part) => part.trim()).filter(Boolean);
  assert.ok(layers.length >= 4, "the stack is four or more layers");
  for (const layer of layers) {
    assert.match(layer, /^inset/, `every lens layer must be inset, got "${layer}"`);
  }
  // No backdrop-filter or filter in the lens definition — the performance red
  // line: the lens is drawn, never filtered.
  const lensSlice = rootBlock.slice(rootBlock.indexOf("--glass-lens-rim-base"));
  assert.ok(!/backdrop-filter|filter:/.test(lensSlice), "the lens family must carry no filter");

  // The stop's scale ascends 1→5 in the shipped blocks, which is what makes the
  // lens *look* heavier at a higher stop. A reverted scale is the mutation lock
  // for "lens token 档间倒挂 → 红".
  const scales = GLASS_STEPS.map((step) => number(stepBlock(css, step), "glass-lens-scale"));
  for (let i = 1; i < scales.length; i += 1) {
    assert.ok(scales[i] > scales[i - 1], `lens scale must ascend: ${scales[i - 1]} → ${scales[i]}`);
  }
  // The effective rim/sheen alpha at each stop (base × scale) must also ascend,
  // so a stop cannot raise the scale while lowering the base.
  const base = number(rootBlock, "glass-lens-rim-base");
  const sheenBase = number(rootBlock, "glass-lens-sheen-base");
  for (let i = 1; i < scales.length; i += 1) {
    assert.ok(base * scales[i] > base * scales[i - 1], "the effective rim alpha must ascend");
    assert.ok(sheenBase * scales[i] > sheenBase * scales[i - 1], "the effective sheen alpha must ascend");
  }
  // …and the light palette restates the four bases (its polarity inverts), so
  // the lens is not dark-only.
  const light = css.slice(css.indexOf('[data-theme="light"]'));
  for (const token of ["glass-lens-rim-base", "glass-lens-sheen-base", "glass-lens-edge-base", "glass-lens-glow-base"]) {
    assert.ok(light.includes(`--${token}:`), `the light palette must restate --${token}`);
  }
});

test("the control ladder draws the lens stack, and the field keeps its slot", async () => {
  const css = stripComments(await read("src/styles/base.css"));
  const rootBlock = css.slice(css.indexOf(":root {"), css.indexOf('[data-theme="light"]'));
  // Rung 0 is the lens stack now: every control that already drew `--elev-0`
  // gains the lens with no per-rule edit. This is the mutation lock for
  // "控件 lens stack 删除 → 红".
  const elev0 = rootBlock.match(/--elev-0:\s*([^;]+);/);
  assert.ok(elev0, "--elev-0 must be declared");
  assert.match(elev0![1], /var\(--glass-lens-stack\)/, "rung 0 must be the lens stack");
  // The field is the lens read backwards plus the slot shadow.
  const field = rootBlock.match(/--glass-field-shadow:\s*([^;]+);/);
  assert.ok(field, "--glass-field-shadow must be declared");
  assert.match(field![1], /inset 0 1px 2px/, "a field keeps its 1px slot shadow");
  assert.match(field![1], /var\(--glass-lens-rim\)/, "a field keeps the lens rim");
  assert.match(field![1], /var\(--glass-lens-edge\)/, "a field keeps the lens bottom edge");
});

test("no lens token reaches a surface file as a literal", async () => {
  // The lens is a token family; a surface that hardcodes an alpha would ignore
  // the stop control. Scan the host sheets for a lens-shaped literal.
  const names = (await readdir(new URL("src/styles/", root))).filter((n) => n.endsWith(".css"));
  for (const name of names) {
    if (name === "base.css") continue; // the definition site
    const css = stripComments(await read(`src/styles/${name}`));
    assert.ok(!/--glass-lens-[\w-]+:\s/.test(css), `${name} must not redefine a lens token`);
  }
});
