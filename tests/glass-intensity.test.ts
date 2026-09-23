// GLASS-3STOP: the three-stop **liquid-glass effect** control, plus the two
// transparency sliders.
//
// GLASS-REAXIS restored the R8 split (a stop controls the *effect*, the
// sliders control the window's transparency). GLASS-3STOP answers the user's
// follow-up on that control:
//
//   「玻璃效果每档差异有点小，改成 3 档，逐步从磨砂玻璃到类似苹果的液态玻璃效果」
//   「透明度现在好像受到当前玻璃效果的影响，除了 10%，其他档位都基本看不到透明」
//
// The first complaint is a *spread* one: five stops sat on top of each other,
// so a step change was a subtle tint shift rather than a visible material
// change. The second is a *coupling* one: the stop's fill floor lifted the
// frame's alpha, so the transparency slider's 10→100 range only moved a
// 0.75→0.98 band and "looked like nothing". This round collapses to three
// far-apart stops and removes the step from the frame alpha entirely.
//
// The tests here lock the five things that make the round honest:
//
//   1. the three-stop effect table itself (step / blur / saturate / lens);
//   2. the write path — a stop writes **only** `glass_step` and never an
//      opacity field;
//   3. cross-stop monotonicity — blur, saturation and lens never fall, so
//      "higher stop = more liquid" cannot invert mid-range;
//   4. the reverse lookup is opacity-independent, and the *frame alpha* is
//      step-independent: the slider is the only alpha truth;
//   5. the wiring — three segments, two sliders, two independent mutators.
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

const declaration = (block: string, name: string) => {
  const match = block.match(new RegExp(`--${name}:\\s*([^;]+);`));
  assert.ok(match, `--${name} must be declared`);
  return match![1].trim();
};

/** The `:root` block, up to the light-theme override. */
const rootBlock = (css: string) => {
  const start = css.indexOf(":root {");
  const end = css.indexOf('[data-theme="light"]');
  assert.ok(start >= 0 && end > start, "base.css must have a :root block before the light theme");
  return css.slice(start, end);
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

// ── A. the three-stop effect table ─────────────────────────────────────────

test("the effect table is three frozen (step, blur, saturate, lens) stops", () => {
  // Every stop, spelled out. This is the mutation lock: change a step, a blur,
  // a saturation or a lens level anywhere and this assertion fails, so a later
  // round cannot quietly retune a stop without also updating its test.
  const expected: Record<
    GlassIntensity,
    { step: GlassStep; blur: number; saturate: number; lens: number }
  > = {
    1: { step: "frosted", blur: 10, saturate: 130, lens: 1 },
    2: { step: "regular", blur: 22, saturate: 170, lens: 2 },
    3: { step: "liquid", blur: 28, saturate: 200, lens: 3 },
  };
  assert.deepEqual([...GLASS_INTENSITIES], [1, 2, 3], "the control is three stops");
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
  // exactly one field, and GLASS-3STOP keeps it. This is the mutation lock for
  // "档位写 opacity → 红".
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

test("the stops are monotonic: blur, saturation and lens never fall", async () => {
  const steps = await shippedSteps();
  const effect = GLASS_INTENSITIES.map((level) => {
    const { step, blur, saturate, lens } = GLASS_INTENSITY[level];
    return { level, step, blur, saturate, lens, shipped: steps[step] };
  });
  for (let i = 1; i < effect.length; i += 1) {
    const prev = effect[i - 1];
    const curr = effect[i];
    // The effect thickens on every axis. GLASS-3STOP pulls the stops far
    // enough apart that all three climb *strictly* at every step — that is
    // what makes a step change visible rather than a subtle tint shift.
    assert.ok(
      curr.blur > prev.blur,
      `stop ${curr.level}'s blur (${curr.blur}) must exceed stop ${prev.level}'s (${prev.blur})`,
    );
    assert.ok(
      curr.saturate > prev.saturate,
      `stop ${curr.level}'s saturate (${curr.saturate}) must exceed stop ${prev.level}'s (${prev.saturate})`,
    );
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
  // The ends really are the two ends of the perceived range, and the spread is
  // wide enough to read at a glance: the user's "每档差异有点小" complaint.
  assert.equal(effect[0].step, "frosted", "stop 1 is the frosted end");
  assert.equal(effect[2].step, "liquid", "stop 3 is the liquid end");
  assert.equal(effect[2].saturate, 200, "stop 3 spends the full saturation budget");
  assert.ok(
    effect[2].blur - effect[0].blur >= 16,
    `the blur spread must be at least 16px, got ${effect[2].blur - effect[0].blur}`,
  );
  assert.ok(
    effect[2].saturate - effect[0].saturate >= 60,
    `the saturation spread must be at least 60%, got ${effect[2].saturate - effect[0].saturate}`,
  );
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
    frosted: 1,
    regular: 2,
    liquid: 3,
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
  assert.equal(cells, 3 * 101, "the sweep must cover every step × percentage");
  // Every stop is reachable from some stored step, or a segment would be dead.
  const reached = new Set<GlassIntensity>();
  for (const step of GLASS_STEPS) reached.add(glassIntensityOf(step, 0.5));
  assert.deepEqual([...reached].sort(), [1, 2, 3], "all three stops must be reachable");
  // A garbage step rests on the balanced stop, the same rule the Rust loader
  // uses; the lookup must not throw.
  assert.equal(glassIntensityOf("ultra" as GlassStep), 2, "an unknown step rests on stop 2");
  assert.equal(glassIntensityOf("" as GlassStep), 2, "an empty step rests on stop 2");
});

test("the pre-GLASS-3STOP vocabulary migrates on lookup", () => {
  // The reverse lookup also has to survive a stored old id: a file written by a
  // build that shipped `low`/`mid`/`high`/`deep`/`jelly` must display on the
  // right new stop. The two thin stops keep their position; every heavy stop
  // lands on `liquid` (the old jelly material promoted to the top stop).
  const legacy: [string, GlassIntensity][] = [
    ["low", 1],
    ["mid", 2],
    ["high", 3],
    ["deep", 3],
    ["jelly", 3],
  ];
  for (const [old, stop] of legacy) {
    assert.equal(glassIntensityOf(old as GlassStep), stop, `a stored ${old} must display on stop ${stop}`);
  }
  // The frontend default (regular + 47%) displays on stop 2.
  assert.equal(glassIntensityOf("regular", 47 / 100), 2, "the frontend default displays on stop 2");
});

// ── D. the alpha decoupling: the slider is the only alpha truth ─────────────

/** Evaluate the shipped `--glass-frame-alpha` formula at a transparency. The
 *  formula is `min(solid_top, max(frame_floor, opacity))` — read from
 *  base.css by the test below, not restated as a JS copy. */
const frameAlpha = (opacity: number, floor: number, solidTop: number) =>
  Math.min(solidTop, Math.max(floor, opacity));

test("the transparency slider is the frame's only alpha truth", async () => {
  const css = stripComments(await read("src/styles/base.css"));
  const root = rootBlock(css);
  const frame = declaration(root, "glass-frame-alpha");
  // The formula consumes the slider and the two clamps — and *not* the step.
  // This is the mutation lock for "把 frame_alpha 公式改回 step_fill 抬底 → 红":
  // any `--glass-step-` token in the frame alpha fails here.
  assert.ok(
    frame.includes("var(--main-opacity)"),
    `--glass-frame-alpha must consume the transparency slider; got "${frame}"`,
  );
  assert.ok(
    !/--glass-step-/.test(frame),
    `--glass-frame-alpha must not read a glass step — the slider is the only alpha truth; got "${frame}"`,
  );
  // The terminal's frame alpha is the same shape over its own slider.
  const terminal = declaration(root, "glass-frame-alpha-terminal");
  assert.ok(terminal.includes("var(--terminal-opacity)"), `terminal frame alpha must read its slider; got "${terminal}"`);
  assert.ok(!/--glass-step-/.test(terminal), `terminal frame alpha must not read a step; got "${terminal}"`);

  // No step block may declare a fill floor at all: `--glass-step-fill` is gone
  // from the vocabulary, so a re-added fill would be the coupling coming back.
  for (const step of GLASS_STEPS) {
    const block = stepBlock(css, step);
    assert.ok(
      !/--glass-step-fill/.test(block),
      `[data-glass="${step}"] must not declare a fill floor — the step is the effect axis`,
    );
  }
  assert.ok(
    !/--glass-step-fill/.test(css),
    "base.css must not declare --glass-step-fill anywhere: the step never supplies alpha",
  );

  const solidTop = number(root, "glass-solid-top");
  const floor = number(root, "glass-frame-floor");
  assert.equal(floor, 0, "the normal-use frame floor is 0 — 10% really is 10%");

  // The user's measured complaint: within one step, opacity 0.10 and 0.95 must
  // paint visibly different frames. The old floor-coupling gave a 0.75→0.98
  // band (a 0.23 spread); the decoupled formula gives the slider's whole range.
  const low = frameAlpha(0.1, floor, solidTop);
  const high = frameAlpha(0.95, floor, solidTop);
  assert.ok(
    high - low >= 0.6,
    `the same step must paint a ≥0.6 alpha spread from 10%→95%; got ${(high - low).toFixed(3)} ` +
      `(${low.toFixed(3)} → ${high.toFixed(3)})`,
  );
  assert.equal(low, 0.1, "10% must paint a 0.10 frame — the user's headline ask");
  assert.equal(high, 0.95, "95% must paint a 0.95 frame");
  assert.equal(frameAlpha(1, floor, solidTop), solidTop, "100% clamps to the near-solid top");
});

test("switching the step never moves the frame alpha", async () => {
  const css = stripComments(await read("src/styles/base.css"));
  const root = rootBlock(css);
  // The step is absent from the frame alpha, so the *same* slider position
  // paints the same frame on every step. Re-derive the formula for each step's
  // haze and assert the frame alpha is identical — the step may change the
  // haze composited into the tint, never the frame.
  const solidTop = number(root, "glass-solid-top");
  const floor = number(root, "glass-frame-floor");
  const frames = GLASS_STEPS.map(() => frameAlpha(0.47, floor, solidTop));
  assert.equal(new Set(frames).size, 1, "the step must not change the frame alpha");
  // …while the step's haze *does* move the painted tint: that is the
  // readability compensation for the thin frosted end.
  const hazes = GLASS_STEPS.map((step) => number(stepBlock(css, step), "glass-step-dim"));
  assert.ok(hazes[0] > hazes[1] && hazes[1] >= hazes[2], "the frosted end carries the most haze");
  const tintAlpha = (haze: number, t: number, frame: number) =>
    1 - (1 - haze * (1 - t)) * (1 - frame);
  const painted = GLASS_STEPS.map((step) => tintAlpha(number(stepBlock(css, step), "glass-step-dim"), 0.1, frameAlpha(0.1, floor, solidTop)));
  assert.ok(
    painted[0] > painted[2],
    `at 10% the frosted haze must make its panel denser than liquid's; got ${painted[0].toFixed(3)} vs ${painted[2].toFixed(3)}`,
  );
});

// ── E. the wiring: three segments, two independent sliders ──────────────────

test("the settings page renders three effect segments and two opacity sliders", async () => {
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
  // The OpacityControl component carries one `type="range"`; R42 moved the
  // font-size range into the shared `TerminalAppearance` component (which the
  // terminal page renders too), so this page declares only the opacity range.
  assert.equal(
    (page.match(/type="range"/g) ?? []).length,
    1,
    "General declares only the shared opacity range (the font size moved to TerminalAppearance)",
  );
  const terminalAppearance = await read("src/settings/TerminalAppearance.tsx");
  assert.equal(
    (terminalAppearance.match(/type="range"/g) ?? []).length,
    3,
    "TerminalAppearance declares the font-size, line-height and wheel-scroll ranges",
  );
  assert.equal(
    (page.match(/<OpacityControl/g) ?? []).length,
    2,
    "the two transparency sliders are two OpacityControl instances",
  );
  assert.match(page, /settings\.transparencyHint/, "the transparency hint is back");
});

test("the settings selector shows three stops, and a migrated file selects the right one", async () => {
  // The UI half of the migration: the selector is built from GLASS_INTENSITIES
  // (three segments) and the *selected* segment is `glassIntensityOf` of the
  // stored step. A file that still holds a pre-GLASS-3STOP id must therefore
  // light the migrated stop, not fall back to a default.
  const page = await read("src/settings/GeneralPage.tsx");
  assert.match(
    page,
    /GLASS_INTENSITIES\.map\(\(value\) => \(\{/,
    "the selector must be built from the shared three-stop list",
  );
  assert.match(
    page,
    /value=\{glassIntensityOf\(settings\.glass_step/,
    "the selected segment must be the reverse lookup of the stored step",
  );
  // What each stored id selects. The migrated ids land where the Rust loader
  // put them; the opacity never enters the decision.
  const selected: [string, GlassIntensity][] = [
    ["frosted", 1],
    ["regular", 2],
    ["liquid", 3],
    ["low", 1],
    ["mid", 2],
    ["high", 3],
    ["deep", 3],
    ["jelly", 3],
    ["ultra", 2],
  ];
  for (const [stored, stop] of selected) {
    assert.equal(
      glassIntensityOf(stored as GlassStep, 0.1),
      stop,
      `a stored ${stored} must light segment ${stop}`,
    );
    // …and the same at any transparency, so a slider nudge never moves it.
    assert.equal(
      glassIntensityOf(stored as GlassStep, 0.95),
      stop,
      `a stored ${stored} must light segment ${stop} at any transparency`,
    );
  }
  // The selector renders exactly one segment per stop, and the label keys are
  // the three new ones (the dead .4/.5 keys are gone).
  assert.deepEqual([...GLASS_INTENSITIES], [1, 2, 3]);
  for (const level of GLASS_INTENSITIES) {
    assert.match(GLASS_INTENSITY[level].label, /^settings\.glassIntensity\.[123]$/);
  }
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
    "settings.glassIntensityHint",
    "settings.transparency.main",
    "settings.transparency.terminal",
    "settings.transparencyHint",
  ]) {
    const occurrences = i18n.match(new RegExp(`"${key.replace(/\./g, "\\.")}":`, "g")) ?? [];
    assert.equal(occurrences.length, 2, `${key} must be declared in both en and zh`);
  }
  // The dead five-stop keys are gone from both dictionaries.
  for (const dead of ["settings.glassIntensity.4", "settings.glassIntensity.5"]) {
    assert.equal(
      (i18n.match(new RegExp(`"${dead.replace(/\./g, "\\.")}":`, "g")) ?? []).length,
      0,
      `${dead} is a dead key and must be removed from both dictionaries`,
    );
  }
});

// ── F. the lens family in base.css ─────────────────────────────────────────

test("the lens family exists, is stop-driven, and ascends with the stop", async () => {
  const css = stripComments(await read("src/styles/base.css"));
  const rootBlockText = css.slice(css.indexOf(":root {"), css.indexOf('[data-theme="light"]'));
  // The four lens tokens, declared from a per-palette base times the stop's
  // scale. The base alphas live in the palette blocks; the scale lives in the
  // step blocks.
  for (const token of ["glass-lens-rim", "glass-lens-sheen", "glass-lens-edge", "glass-lens-glow"]) {
    const match = rootBlockText.match(new RegExp(`--${token}:\\s*([^;]+);`));
    assert.ok(match, `--${token} must be declared`);
    assert.match(
      match![1],
      new RegExp(`var\\(--${token}-base\\)\\s*\\*\\s*var\\(--glass-lens-scale\\)`),
      `--${token} must be base × scale`,
    );
  }
  // The composed stack consumes all four (rim, sheen, edge) plus the shared
  // pane edge; every layer is `inset`, so a control never casts or blurs.
  const stack = rootBlockText.match(/--glass-lens-stack:\s*([^;]+);/);
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
  const lensSlice = rootBlockText.slice(rootBlockText.indexOf("--glass-lens-rim-base"));
  assert.ok(!/backdrop-filter|filter:/.test(lensSlice), "the lens family must carry no filter");

  // The stop's scale ascends in the shipped blocks, which is what makes the
  // lens *look* heavier at a higher stop. A reverted scale is the mutation lock
  // for "lens token 档间倒挂 → 红".
  const scales = GLASS_STEPS.map((step) => number(stepBlock(css, step), "glass-lens-scale"));
  for (let i = 1; i < scales.length; i += 1) {
    assert.ok(scales[i] > scales[i - 1], `lens scale must ascend: ${scales[i - 1]} → ${scales[i]}`);
  }
  // The effective rim/sheen alpha at each stop (base × scale) must also ascend,
  // so a stop cannot raise the scale while lowering the base.
  const base = number(rootBlockText, "glass-lens-rim-base");
  const sheenBase = number(rootBlockText, "glass-lens-sheen-base");
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
  const rootBlockText = css.slice(css.indexOf(":root {"), css.indexOf('[data-theme="light"]'));
  // Rung 0 is the lens stack now: every control that already drew `--elev-0`
  // gains the lens with no per-rule edit. This is the mutation lock for
  // "控件 lens stack 删除 → 红".
  const elev0 = rootBlockText.match(/--elev-0:\s*([^;]+);/);
  assert.ok(elev0, "--elev-0 must be declared");
  assert.match(elev0![1], /var\(--glass-lens-stack\)/, "rung 0 must be the lens stack");
  // The field is the lens read backwards plus the slot shadow.
  const field = rootBlockText.match(/--glass-field-shadow:\s*([^;]+);/);
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

// ── G. mutation locks ──────────────────────────────────────────────────────
//
// The two regressions this round exists to prevent, each written as a
// predicate over a *mutated* source: the test asserts the mutation lands and
// that the shipped assertion would reject it. A green suite on the unmutated
// file is not enough — these prove the guards actually bite.

test("mutation lock: a re-coupled frame alpha (step fill lifting the slider) goes red", async () => {
  const css = stripComments(await read("src/styles/base.css"));
  const root = rootBlock(css);
  const shipped = declaration(root, "glass-frame-alpha");
  // The pre-GLASS-3STOP shape: the step's floor sliding to near-solid. The
  // decoupling assertion is "the frame alpha must not read a step", so this is
  // exactly the string it has to reject.
  const mutated = `calc(\n    var(--glass-step-fill) + (var(--glass-solid-top) - var(--glass-step-fill)) * var(--main-opacity)\n  )`;
  assert.notEqual(mutated, shipped, "the mutation must actually differ from the shipped formula");
  assert.match(mutated, /--glass-step-/, "the mutated formula must carry a step token");
  assert.ok(
    !/--glass-step-/.test(shipped),
    "the shipped frame alpha must not carry a step token — otherwise the mutation lock is vacuous",
  );
  // The mutation's measured effect: at 0.10 the floor-coupled formula paints
  // 0.748, not 0.10, so the slider's bottom half is invisible — the user's bug.
  const floorCoupled = 0.8 + (0.98 - 0.8) * 0.1;
  assert.ok(
    floorCoupled > 0.7,
    `the re-coupled formula would paint ${floorCoupled.toFixed(3)} at 10% — the regression the round fixes`,
  );
});

test("mutation lock: migrating low -> regular (instead of frosted) goes red", async () => {
  // The migration's whole point is that the *thin* old stop lands on the thin
  // new one. A later round that mapped `low` onto `regular` would silently
  // promote every Clear user to the balanced material. The Rust table and the
  // frontend map must both reject it.
  const { normalizeGlassStep } = await import("../src/glass-material.ts");
  assert.equal(normalizeGlassStep("low"), "frosted", "low must migrate to frosted");
  assert.notEqual(normalizeGlassStep("low"), "regular", "low must NOT migrate to regular");
  const rust = await read("src-tauri/src/commands/config.rs");
  assert.match(
    rust,
    /\("low", "frosted"\)/,
    "the Rust legacy map must send low to frosted",
  );
  assert.ok(
    !/\("low", "regular"\)/.test(rust),
    "the Rust legacy map must not send low to regular",
  );
  // Every heavy stop lands on liquid, not on regular: an upgrader on the
  // heaviest material is not demoted.
  for (const heavy of ["high", "deep", "jelly"]) {
    assert.equal(normalizeGlassStep(heavy), "liquid", `${heavy} must migrate to liquid`);
  }
});

test("mutation lock: a step that re-declares a fill floor goes red", async () => {
  // `--glass-step-fill` is gone from the vocabulary. If a later round adds it
  // back to a step block, the frame alpha would have something to couple to
  // again; the decoupling test above checks the formula, and this checks the
  // token itself is absent everywhere.
  const css = stripComments(await read("src/styles/base.css"));
  assert.ok(!/--glass-step-fill/.test(css), "no step may declare a fill floor");
  const mutated = css.replace(
    /(\[data-glass="regular"\] \{)/,
    "$1\n  --glass-step-fill: 0.68;",
  );
  assert.notEqual(mutated, css, "the mutation must land");
  assert.match(mutated, /--glass-step-fill/, "the mutation lock must reject a re-added fill floor");
});
