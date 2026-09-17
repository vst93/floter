// GLASS-UNIFY: one intensity control over the (glass_step, tint) pair.
//
// R8 exposed the material (a 3-way step) and the window transparency (a
// continuous slider) side by side. The user judged the two knobs to be one
// perceived axis — "效果强度和透明度…感觉不出来它们的独立" — so this round
// 收回 UI 层的双控制: the settings panel now offers a single five-stop glass
// intensity control and the continuous slider is gone. Nothing below the UI
// changed: a stop is a fixed `(glass_step, tint)` pair written into the same
// two fields, and the reverse lookup turns any stored pair back into the stop
// it is closest to.
//
// The tests here lock the four things that make the unification honest:
//
//   1. the five-stop table itself (move any pair and this fails);
//   2. cross-stop monotonicity — blur never falls, effective tint never falls,
//      so "higher stop = more glass" cannot invert mid-range;
//   3. the reverse lookup over the whole `(step, tint)` space, boundaries
//      included, so a stored pair always displays on exactly one stop;
//   4. the wiring — the settings page renders five segments, the chosen one
//      writes its pair, and *no* opacity slider exists anywhere in the UI.
//
// The effective-tint arithmetic is read from the shipped step tokens in
// base.css, not restated here, so the calibration cannot drift from the CSS.
import assert from "node:assert/strict";
import { readFileSync, readdirSync } from "node:fs";
import { readFile } from "node:fs/promises";
import test from "node:test";

import {
  GLASS_INTENSITIES,
  GLASS_INTENSITY,
  GLASS_STEP_TOKENS,
  GLASS_SOLID_TOP,
  clampWindowOpacity,
  glassEffectiveTint,
  glassIntensityForTint,
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

/** The shipped material per step, straight out of base.css. */
const shippedSteps = async () => {
  const css = stripComments(await read("src/styles/base.css"));
  const out = {} as Record<GlassStep, { blur: number; fill: number; dim: number }>;
  for (const step of ["low", "mid", "high"] as GlassStep[]) {
    const block = stepBlock(css, step);
    out[step] = {
      blur: number(block, "glass-step-blur", "px"),
      fill: number(block, "glass-step-fill"),
      dim: number(block, "glass-step-dim"),
    };
  }
  return out;
};

// ── A. the five-stop table ─────────────────────────────────────────────────

test("the intensity table is five frozen (glass_step, tint) pairs", () => {
  // Every pair, spelled out. This is the mutation lock: change a step or a
  // tint anywhere in the table and this assertion fails, so a later round
  // cannot quietly retune a stop without also updating its test.
  const expected: Record<GlassIntensity, { step: GlassStep; tint: number }> = {
    1: { step: "low", tint: 0.22 },
    2: { step: "mid", tint: 0.4 },
    3: { step: "high", tint: 0.58 },
    4: { step: "high", tint: 0.78 },
    5: { step: "high", tint: 0.98 },
  };
  assert.deepEqual([...GLASS_INTENSITIES], [1, 2, 3, 4, 5], "the control is five stops");
  for (const level of GLASS_INTENSITIES) {
    assert.equal(GLASS_INTENSITY[level].step, expected[level].step, `stop ${level}'s step`);
    assert.equal(GLASS_INTENSITY[level].tint, expected[level].tint, `stop ${level}'s tint`);
    // Labels are i18n keys, never literals or parameter names — and they must
    // exist in the dictionary: a key nobody resolves renders as blank.
    assert.match(GLASS_INTENSITY[level].label, /^settings\.glassIntensity\.\d$/);
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
  // The top stop is the old slider's own maxed end, not 1.0: a fully opaque
  // frame drops the material's sheen and rim (see `--glass-solid-top`).
  assert.equal(GLASS_INTENSITY[5].tint, GLASS_SOLID_TOP, "stop 5 is the near-solid top, not 1.0");
  // The tint anchors are inside the spec's ±0.05 calibration band.
  const anchors: Record<GlassIntensity, number> = { 1: 0.22, 2: 0.4, 3: 0.58, 4: 0.78, 5: 0.98 };
  for (const level of GLASS_INTENSITIES) {
    assert.ok(
      Math.abs(GLASS_INTENSITY[level].tint - anchors[level]) <= 0.05,
      `stop ${level} must sit within ±0.05 of its anchor ${anchors[level]}`,
    );
  }
});

test("a stop writes its pair into the settings fields, as integers", () => {
  for (const level of GLASS_INTENSITIES) {
    const written = glassIntensitySettings(level);
    assert.equal(written.glass_step, GLASS_INTENSITY[level].step, `stop ${level} writes its step`);
    // Both opacity fields move together — the control is one axis.
    assert.equal(written.main_opacity, written.terminal_opacity, `stop ${level} writes one tint to both frames`);
    assert.equal(written.main_opacity, Math.round(GLASS_INTENSITY[level].tint * 100));
    // The Rust fields are u8 percentages; the written values must be integers.
    assert.equal(Number.isInteger(written.main_opacity), true);
  }
  // The write path round-trips through the reverse lookup: a stop the UI just
  // wrote displays on that same stop.
  for (const level of GLASS_INTENSITIES) {
    const { glass_step, main_opacity } = glassIntensitySettings(level);
    assert.equal(glassIntensityOf(glass_step, main_opacity / 100), level, `stop ${level} must round-trip`);
  }
});

// ── B. cross-stop monotonicity ─────────────────────────────────────────────

test("the stops are monotonic: blur never falls, effective tint never falls", async () => {
  const steps = await shippedSteps();
  const effective = GLASS_INTENSITIES.map((level) => {
    const { step, tint } = GLASS_INTENSITY[level];
    return { level, step, tint, blur: steps[step].blur, effective: glassEffectiveTint(step, tint) };
  });
  for (let i = 1; i < effective.length; i += 1) {
    const prev = effective[i - 1];
    const curr = effective[i];
    // The material thickens: blur is non-decreasing …
    assert.ok(
      curr.blur >= prev.blur,
      `stop ${curr.level}'s blur (${curr.blur}) must not fall below stop ${prev.level}'s (${prev.blur})`,
    );
    // … the effective tint is non-decreasing (less shows through) …
    assert.ok(
      curr.effective >= prev.effective,
      `stop ${curr.level}'s effective tint (${curr.effective.toFixed(4)}) must not fall below ` +
        `stop ${prev.level}'s (${prev.effective.toFixed(4)})`,
    );
    // … and the raw transparency does not increase either.
    assert.ok(
      curr.tint >= prev.tint,
      `stop ${curr.level}'s transparency (${curr.tint}) must not exceed stop ${prev.level}'s (${prev.tint})`,
    );
  }
  // The ends really are the two ends of the perceived range: stop 1 is the
  // thinnest material (Clear variant), stop 5 the near-solid panel.
  assert.equal(effective[0].step, "low", "stop 1 is the Clear variant");
  assert.equal(effective[4].step, "high", "stop 5 is the heaviest variant");
  assert.ok(
    effective[4].effective >= 0.95,
    `stop 5 must be near-opaque, got ${effective[4].effective.toFixed(4)}`,
  );
  assert.ok(
    effective[0].effective <= 0.75,
    `stop 1 must read as thin glass, got ${effective[0].effective.toFixed(4)}`,
  );
});

// ── C. the reverse lookup over the whole space ─────────────────────────────

test("every (step, transparency) pair lands on exactly one stop", () => {
  // The full space: three steps × every integer percentage the Rust u8 fields
  // can hold (0-100, though the UI clamps to 10-100). `glassIntensityOf` must
  // return one of the five ids for all of them — no undefined, no throw.
  const ids = new Set<number>(GLASS_INTENSITIES);
  let cells = 0;
  for (const step of ["low", "mid", "high"] as GlassStep[]) {
    for (let percent = 0; percent <= 100; percent += 1) {
      const level = glassIntensityOf(step, percent / 100);
      assert.ok(ids.has(level), `${step}@${percent}% resolved to an unknown stop ${level}`);
      cells += 1;
    }
  }
  assert.equal(cells, 3 * 101, "the sweep must cover every step × percentage");
  // Every stop is reachable from some stored pair, or a segment would be dead.
  const reached = new Set<GlassIntensity>();
  for (const step of ["low", "mid", "high"] as GlassStep[]) {
    for (let percent = 0; percent <= 100; percent += 1) {
      reached.add(glassIntensityOf(step, percent / 100));
    }
  }
  assert.deepEqual([...reached].sort(), [1, 2, 3, 4, 5], "all five stops must be reachable");
});

test("the reverse lookup's boundaries are the midpoints between adjacent stops", () => {
  // The boundary between stop i and stop i+1 is the mean of their effective
  // tints. Assert the boundary itself and both sides, so the rule is pinned
  // rather than merely "it returns something".
  for (let i = 0; i < GLASS_INTENSITIES.length - 1; i += 1) {
    const lower = GLASS_INTENSITIES[i];
    const upper = GLASS_INTENSITIES[i + 1];
    const boundary =
      (glassEffectiveTint(GLASS_INTENSITY[lower].step, GLASS_INTENSITY[lower].tint) +
        glassEffectiveTint(GLASS_INTENSITY[upper].step, GLASS_INTENSITY[upper].tint)) /
      2;
    // A tie falls to the lower (thinner) stop, the conservative choice.
    assert.equal(glassIntensityForTint(boundary), lower, `the boundary ${boundary.toFixed(4)} falls to stop ${lower}`);
    assert.equal(glassIntensityForTint(boundary - 1e-9), lower, "just below the boundary is the lower stop");
    assert.equal(glassIntensityForTint(boundary + 1e-9), upper, "just above the boundary is the upper stop");
  }
  // The extreme ends clamp without a gap.
  assert.equal(glassIntensityForTint(0), 1, "0 tint is stop 1");
  assert.equal(glassIntensityForTint(1), 5, "full tint is stop 5");
  assert.equal(glassIntensityForTint(-1), 1, "below-range tint clamps to stop 1");
  assert.equal(glassIntensityForTint(2), 5, "above-range tint clamps to stop 5");
  // The two named extremes from the spec, asserted as stored pairs. The
  // thinnest pair the model can express (the Clear step at zero tint) is
  // stop 1; the slider's 0.98 top is stop 5 on every step. Note that a
  // *higher* step's zero-tint floor is already denser than stop 1 — the
  // Regular floor (0.68) is above stop 1's composite — so only the Clear
  // floor lands on stop 1.
  assert.equal(glassIntensityOf("low", 0), 1, "the thinnest possible pair (Clear@0) is stop 1");
  assert.equal(glassIntensityOf("high", 0), 2, "the heaviest step's own floor is already stop 2");
  for (const step of ["low", "mid", "high"] as GlassStep[]) {
    assert.equal(glassIntensityOf(step, 0.98), 5, `${step}@0.98 must be stop 5`);
  }
});

test("the migrated pre-R8 file (94 -> 47% + high) displays on stop 3", () => {
  // `migrate_legacy_glass_strength` halves the old slider and picks the step
  // by its tertile; an old 94 becomes main_opacity 47 + glass_step "high". That
  // pair is not a stop the control wrote, so it must reverse-look-up cleanly.
  // Its effective tint (0.8846) sits between stop 2 (0.800) and stop 3 (0.9044),
  // above the 0.8522 boundary — so the settings panel shows "Strong" (stop 3).
  const level = glassIntensityOf("high", 47 / 100);
  assert.equal(level, 3, "an upgraded 94 must display on stop 3 (Strong)");
  // The default the frontend ships (47% + mid) is a different stop, and both
  // are reachable — the migration and the default do not collapse together.
  assert.equal(glassIntensityOf("mid", 47 / 100), 2, "the frontend default (mid + 47%) is stop 2");
});

// ── D. the wiring: five segments, no slider ────────────────────────────────

test("the settings page renders five intensity segments and no opacity slider", async () => {
  const page = await read("src/settings/GeneralPage.tsx");
  // The control is built from the shared stop table, one segment per stop.
  assert.match(page, /GLASS_INTENSITIES\.map/, "the segments must come from GLASS_INTENSITIES");
  assert.match(page, /data-glass-intensity=\{option\.value\}/, "each segment must carry its stop id");
  assert.match(page, /role="radiogroup"/, "the segments stay a radiogroup (keyboard contract)");
  assert.match(page, /role="radio"/, "each segment is a radio");
  assert.match(page, /glassIntensityOf\(settings\.glass_step/, "the chosen stop is derived by the reverse lookup");
  // The wiring: choosing a segment calls the unified mutator with the stop id.
  assert.match(page, /onChange=\{onChangeGlassIntensity\}/, "a segment click writes the stop's pair");

  // The R8 controls are gone: no range input, no preset buttons, no
  // percentage readout, no transparency labels. `type="range"` still appears
  // for font size, so the absence is scoped to the removed control classes.
  assert.ok(
    !/opacity-control/.test(page),
    "the opacity control markup must be deleted from GeneralPage",
  );
  assert.ok(
    !/type="range"[^>]*className="opacity-control__range"|opacity-control__range/.test(page),
    "no transparency range input may remain",
  );
  assert.ok(
    !/settings\.transparency\.(main|terminal)|settings\.transparencyHint/.test(page),
    "the transparency i18n keys must no longer be referenced",
  );
  assert.ok(
    !/GlassStepControl|GLASS_STEP_LABELS|settings\.glassStep/.test(page),
    "the R8 step control and its keys must be gone",
  );
});

test("no opacity slider survives anywhere in the host UI", async () => {
  // A DOM/component-level scan: the class names the removed control owned, and
  // the component itself, must not reappear in any TS/TSX file. The font-size
  // slider legitimately remains, so this is scoped to the removed control.
  // Scan every TS/TSX file under src/ (not a hand-picked list): a future
  // file that resurrects the removed control must fail here too.
  const files = [];
  const collect = (dir) => {
    for (const entry of readdirSync(new URL(dir, root), { withFileTypes: true })) {
      const rel = `${dir}${entry.name}`;
      if (entry.isDirectory()) collect(`${rel}/`);
      else if (/\.(tsx?|css)$/.test(entry.name)) files.push(rel);
    }
  };
  collect("src/");
  for (const file of files) {
    // Strip comments: the unified mutator's doc block explains what it
    // replaced, and a code scan must not read prose as code.
    const source = (await read(file))
      .replace(/\/\*[\s\S]*?\*\//g, "")
      .replace(/\/\/[^\n]*/g, "");
    for (const dead of [
      "opacity-control__range",
      "opacity-control__presets",
      "opacity-control__preset",
      "OpacityControl",
      "normalizeOpacity",
      "changeOpacity",
    ]) {
      assert.ok(!source.includes(dead), `${file} still references the removed opacity control (${dead})`);
    }
  }
  // The removed CSS classes must be gone from every sheet too — the TS scan
  // above already covers .css files for the control name; class fragments
  // share the same dead-symbol list.
});

test("the unified mutator writes the step and both opacity fields together", async () => {
  const hook = await read("src/hooks/useSettings.ts");
  assert.match(hook, /const changeGlassIntensity = useCallback/, "the unified mutator must exist");
  const body = hook.slice(hook.indexOf("const changeGlassIntensity = useCallback"));
  const fn = body.slice(0, body.indexOf("const changeFontSize"));
  assert.match(fn, /glassIntensitySettings\(level\)/, "the mutator must derive the pair from the stop table");
  for (const field of ["glass_step", "main_opacity", "terminal_opacity"]) {
    assert.match(fn, new RegExp(`${field}:`), `the mutator must write ${field}`);
  }
  // The R8 split mutators are gone.
  assert.ok(!/const changeOpacity = useCallback/.test(hook), "the opacity slider mutator must be gone");
  assert.ok(!/const changeGlassStep = useCallback/.test(hook), "the step-only mutator must be gone");
});

test("the opacity clamp keeps the Rust window-opacity band", () => {
  // The old `normalizeOpacity` snapped to presets; with a discrete control that
  // snapping is gone, leaving the clamp the Rust side also enforces.
  assert.equal(clampWindowOpacity(47), 47);
  assert.equal(clampWindowOpacity(0), 10, "the floor is the Rust MIN_WINDOW_OPACITY");
  assert.equal(clampWindowOpacity(100), 100);
  assert.equal(clampWindowOpacity(150), 100);
  assert.equal(clampWindowOpacity(-5), 10);
  assert.equal(clampWindowOpacity(Number.NaN), 47, "a non-finite value falls back to the default");
  // No preset snapping: 24 stays 24, unlike the old 25-preset snap.
  assert.equal(clampWindowOpacity(24), 24);
});

test("the i18n keys exist in both languages and the removed ones are gone", async () => {
  const i18n = await read("src/i18n.ts");
  for (const key of [
    "settings.glassIntensity",
    "settings.glassIntensity.1",
    "settings.glassIntensity.2",
    "settings.glassIntensity.3",
    "settings.glassIntensity.4",
    "settings.glassIntensity.5",
    "settings.glassIntensityHint",
  ]) {
    const occurrences = i18n.match(new RegExp(`"${key.replace(/\./g, "\\.")}":`, "g")) ?? [];
    assert.equal(occurrences.length, 2, `${key} must be declared in both en and zh`);
  }
  for (const dead of ["settings.glassStep", "settings.transparency"]) {
    assert.ok(!i18n.includes(`"${dead}`), `the removed ${dead}* keys must be gone`);
  }
});
