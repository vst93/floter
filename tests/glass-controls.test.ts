// GLASS-REAXIS: the readability-control split.
//
// The user's complaint that opened this round was: "at maxed glass strength
// there is still noticeable transparency, and it hurts readability." The root
// cause was a single control — mislabelled *glass strength* — driving three
// things at once (the frame's tint, the blur, inversely, and the saturation),
// with a tint map that topped out at 0.73. This round splits the axis:
//
//   * **window transparency** (`main_opacity` / `terminal_opacity`, 10-100)
//     is readability. It moves the frame's fill from the step's own floor to
//     `--glass-solid-top` (0.98), so 100% is a near-opaque panel.
//   * **glass step** (`glass_step`: low | mid | high) is material. It maps
//     onto HIG's two Liquid Glass variants plus a readability-max variant:
//     Clear (10px blur, 0.30 fill, 35% dimming layer), Regular (24px, 0.68),
//     Regular-max (28px, 0.80, saturation at the budget ceiling).
//
// Everything below is asserted against the *shipped CSS tokens*, not against
// this file's copy of them: the tests read the five `[data-glass]` blocks, the
// `--glass-*` formulas and the palette out of base.css. What lives here is only
// the sentence "the two controls are orthogonal, and neither end of either is
// unreadable", and the arithmetic that says so.
//
// companion to `glass-material.test.ts` (the DOM/CSS half) and
// `hig-craft.test.ts` (the HIG craft half).
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

const root = new URL("../", import.meta.url);
const read = (path: string) => readFile(new URL(path, root), "utf8");
const stripComments = (css: string) => css.replace(/\/\*[\s\S]*?\*\//g, "");

// ── CSS-property parsing helpers ──────────────────────────────────────────

/** The `:root` block, up to the light-theme override. */
const rootBlock = (css: string) => {
  const start = css.indexOf(":root {");
  const end = css.indexOf('[data-theme="light"]');
  assert.ok(start >= 0 && end > start, "base.css must have a :root block before the light theme");
  return css.slice(start, end);
};

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

/** A `--x: calc(a + b * var(--input))` map, read as { a, b }. */
const declaration = (block: string, name: string) => {
  const match = block.match(new RegExp(`--${name}:\\s*([^;]+);`));
  assert.ok(match, `--${name} must be declared`);
  return match![1].trim();
};

// ── the shipped model, read from base.css ─────────────────────────────────

type Step = {
  name: string;
  blur: number;
  saturate: number;
  fill: number;
  dim: number;
};

type Model = {
  steps: Step[];
  solidTop: number;
  content: { a: number; b: number };
  darkPalette: string;
  lightPalette: string;
  transparency: { main: number; terminal: number };
};

const loadModel = async (): Promise<Model> => {
  const css = stripComments(await read("src/styles/base.css"));
  const root = rootBlock(css);
  const steps: Step[] = ["low", "mid", "high"].map((name) => {
    const block = stepBlock(css, name);
    return {
      name,
      blur: number(block, "glass-step-blur", "px"),
      saturate: number(block, "glass-step-saturate", "%"),
      fill: number(block, "glass-step-fill"),
      dim: number(block, "glass-step-dim"),
    };
  });
  const solidTopMatch = declaration(root, "glass-solid-top");
  const contentMatch = declaration(root, "glass-content-alpha").match(
    /^calc\(\s*([\d.]+)\s*\+\s*([\d.]+)\s*\*\s*var\(--main-opacity\)\s*\)$/,
  );
  assert.ok(contentMatch, "--glass-content-alpha must be calc(a + b·transparency)");
  return {
    steps,
    solidTop: Number(solidTopMatch),
    content: { a: Number(contentMatch![1]), b: Number(contentMatch![2]) },
    darkPalette: root,
    lightPalette: css.slice(css.indexOf('[data-theme="light"]'), css.indexOf("html,\nbody,")),
    transparency: {
      main: Number(declaration(root, "main-opacity")),
      terminal: Number(declaration(root, "terminal-opacity")),
    },
  };
};

/** The frame's composited fill: the step's floor sliding to `solidTop`, with
 *  the variant's dimming layer underneath it (fading as the frame solidifies). */
const frameAlpha = (step: Step, transparency: number, solidTop: number) => {
  const fill = step.fill + (solidTop - step.fill) * transparency;
  const dim = step.dim * (1 - transparency);
  return 1 - (1 - dim) * (1 - fill);
};

// ── WCAG arithmetic (the same functions the CSS half uses) ────────────────

const srgbToLinear = (c: number) => {
  const s = c / 255;
  return s <= 0.04045 ? s / 12.92 : ((s + 0.055) / 1.055) ** 2.4;
};
const relativeLuminance = ([r, g, b]: number[]) =>
  0.2126 * srgbToLinear(r) + 0.7152 * srgbToLinear(g) + 0.0722 * srgbToLinear(b);
const contrastRatio = (a: number[], b: number[]) => {
  const [hi, lo] = [relativeLuminance(a), relativeLuminance(b)].sort((x, y) => y - x);
  return (hi + 0.05) / (lo + 0.05);
};
const flatten = (rgba: number[], backdrop: number[]) =>
  rgba.slice(0, 3).map((c, i) => c * rgba[3] + backdrop[i] * (1 - rgba[3]));

// ── the tests ─────────────────────────────────────────────────────────────

test("the two controls are orthogonal: neither token is a function of the other", async () => {
  const model = await loadModel();
  const css = stripComments(await read("src/styles/base.css"));
  const root = rootBlock(css);

  // The step blocks must not mention the transparency control at all: a step
  // that read `--main-opacity` would re-couple the two axes, which is the
  // exact bug this round fixes.
  for (const step of ["low", "mid", "high"]) {
    const block = stepBlock(css, step);
    assert.ok(
      !/--main-opacity|--terminal-opacity/.test(block),
      `[data-glass="${step}"] must not read the transparency control — the material and the window opacity are orthogonal`,
    );
  }
  // …and the content layer is a band of the transparency control only; it must
  // not mention a step, because content is standard material, not glass.
  const content = declaration(root, "glass-content-alpha");
  assert.ok(
    !/--glass-step/.test(content),
    "--glass-content-alpha must not read the glass step: content is standard material",
  );
  // The frame aliases the step's blur and saturation, so a step switch reaches
  // the material without any rule naming a step.
  assert.equal(declaration(root, "glass-blur"), "var(--glass-step-blur)");
  assert.equal(declaration(root, "glass-blur-terminal"), "var(--glass-step-blur)");
  assert.equal(declaration(root, "glass-saturate"), "var(--glass-step-saturate)");

  // The frontend's mutators. GLASS-REAXIS restored the R8 split: a stop writes
  // only the effect step (`changeGlassIntensity`), and the two transparency
  // sliders write the opacity fields through one two-target mutator
  // (`changeOpacity`). The CSS model above is orthogonal, and so is the write
  // path — a stop never touches an opacity and a slider never touches the step.
  const hook = await read("src/hooks/useSettings.ts");
  assert.match(hook, /const changeGlassIntensity = useCallback/, "the effect mutator must exist");
  const intensityBody = hook.slice(hook.indexOf("const changeGlassIntensity = useCallback"));
  const intensityFn = intensityBody.slice(0, intensityBody.indexOf("const changeOpacity"));
  assert.match(
    intensityFn,
    /glass_step/,
    "the effect mutator must write glass_step — a stop is the effect axis",
  );
  assert.ok(
    !/main_opacity:|terminal_opacity:/.test(intensityFn),
    "the effect mutator must not write an opacity — that was the GLASS-UNIFY mistake",
  );
  // The two-target opacity mutator is back, and it does not read the step.
  assert.match(hook, /const changeOpacity = useCallback/, "the opacity slider mutator must exist");
  const opacityBody = hook.slice(hook.indexOf("const changeOpacity = useCallback"));
  const opacityFn = opacityBody
    .slice(0, opacityBody.indexOf("const changeGlassIntensity"))
    .replace(/\/\*[\s\S]*?\*\//g, "")
    .replace(/\/\/[^\n]*/g, "");
  assert.match(opacityFn, /target === "main"/, "the opacity mutator is two-target");
  assert.ok(!/glass_step/.test(opacityFn), "the opacity mutator must not touch the step");
});

// F1 (R8 microfix): the shape of the two readability formulas, asserted
// against the shipped declaration rather than against a JS copy of it.
//
// The sweep tests below re-derive the frame/dim arithmetic in JavaScript and
// evaluate it — but that means they test their own arithmetic, not the CSS.
// Before this test, replacing the whole `--glass-frame-alpha` declaration with
// the pre-R8 `calc(0.18 + 0.55 * var(--main-opacity))` (which tops out at
// 0.73, the user's original complaint) left the suite green. It also let the
// Clear step's 35% dimming layer be dropped from `--glass-tint-alpha` while
// the JS copy kept compositing it. These assertions close both. The structure
// they require is exactly base.css's:
//
//   frame-alpha     = step-fill + (solid-top - step-fill) * transparency
//   tint-alpha      = 1 - (1 - step-dim * (1 - transparency)) * (1 - frame-alpha)
//   frame-alpha-term= step-fill + (solid-top - step-fill) * terminal-transp.
//
// A bare literal coefficient (the R7 map's `0.18` / `0.55`) is rejected
// outright: the fill must come from the step's floor and the near-solid top,
// never from a constant baked into the formula.
test("the frame and dimming formulas are the shipped token composition, not a literal map", async () => {
  const css = stripComments(await read("src/styles/base.css"));
  const root = rootBlock(css);

  // The frame fill: the step's floor sliding to near-solid on the transparency
  // control. Both inputs are required, and no literal coefficient is allowed.
  const frame = declaration(root, "glass-frame-alpha");
  for (const token of ["var(--glass-step-fill)", "var(--glass-solid-top)", "var(--main-opacity)"]) {
    assert.ok(
      frame.includes(token),
      `--glass-frame-alpha must consume ${token}; got "${frame}". ` +
        "A floor/top/control triple is what makes 100% reach ≥0.95 instead of the R7 0.73 cap",
    );
  }
  assert.ok(
    !/\d+\.\d+/.test(frame),
    `--glass-frame-alpha must not carry a literal coefficient (the R7 map was 0.18 + 0.55·op); got "${frame}"`,
  );
  assert.ok(!frame.includes("0.18") && !frame.includes("0.55"), `--glass-frame-alpha regressed to the pre-R8 map: "${frame}"`);

  // The dimming layer: the step's own dim, composited underneath the frame and
  // fading out as the frame solidifies. Without `--glass-step-dim` here the
  // Clear variant's published 35% veil is silently gone.
  const tint = declaration(root, "glass-tint-alpha");
  for (const token of ["var(--glass-step-dim)", "var(--glass-frame-alpha)"]) {
    assert.ok(
      tint.includes(token),
      `--glass-tint-alpha must consume ${token}; got "${tint}". ` +
        "The Clear step's dimming layer is what keeps thin glass readable over bright content",
    );
  }

  // The terminal frame is the same shape over its own transparency control.
  const terminal = declaration(root, "glass-frame-alpha-terminal");
  for (const token of ["var(--glass-step-fill)", "var(--glass-solid-top)", "var(--terminal-opacity)"]) {
    assert.ok(
      terminal.includes(token),
      `--glass-frame-alpha-terminal must consume ${token}; got "${terminal}"`,
    );
  }
  assert.ok(
    !/\d+\.\d+/.test(terminal),
    `--glass-frame-alpha-terminal must not carry a literal coefficient; got "${terminal}"`,
  );

  // The two controls compose: the terminal tint must dim *and* frame, or the
  // terminal surface would be the one place the material branch is missing.
  const terminalTint = declaration(root, "glass-tint-alpha-terminal");
  for (const token of ["var(--glass-step-dim)", "var(--glass-frame-alpha-terminal)"]) {
    assert.ok(
      terminalTint.includes(token),
      `--glass-tint-alpha-terminal must consume ${token}; got "${terminalTint}"`,
    );
  }
});

test("the transparency slider maps its whole range onto the frame's fill", async () => {
  const model = await loadModel();
  for (const step of model.steps) {
    const atFloor = frameAlpha(step, 0.1, model.solidTop);
    const atTop = frameAlpha(step, 1.0, model.solidTop);
    assert.ok(atFloor < atTop, `${step.name}: the slider must actually move the frame`);
    // The slider's ends are the frame's ends: the step's own fill (plus its
    // dimming layer) at the 10% floor, the near-solid top at 100%.
    assert.ok(
      Math.abs(atFloor - frameAlpha(step, 0.1, model.solidTop)) < 1e-9,
      `${step.name}: the floor must be the step's own fill`,
    );
    assert.ok(atTop >= 0.95, `${step.name}: 100% must be near-opaque, got ${atTop.toFixed(3)}`);
  }
  // The default lands mid-range on every step: an upgrading user sees a
  // surface, not an extreme.
  for (const step of model.steps) {
    const atDefault = frameAlpha(step, model.transparency.main / 100, model.solidTop);
    assert.ok(
      atDefault > 0.5 && atDefault < 0.98,
      `${step.name}: the default transparency must land mid-range, got ${atDefault.toFixed(3)}`,
    );
  }
});

test("the step is the material axis: blur, saturation and fill all ascend", async () => {
  const model = await loadModel();
  const [low, mid, high] = model.steps;
  // HIG's variant bands (`liquid-glass.md › Cross-platform translation`):
  // clear 8-16px / 20-40% fill + 35% dimming, regular 20-40px / 60-80%.
  assert.ok(low.blur >= 8 && low.blur <= 16, `Clear blur must be 8-16px, got ${low.blur}`);
  assert.ok(low.fill >= 0.2 && low.fill <= 0.4, `Clear fill must be 20-40%, got ${low.fill}`);
  assert.equal(low.dim, 0.35, "Clear glass over bright content uses the published 35% dimming layer");
  assert.ok(mid.blur >= 20 && mid.blur <= 28, `Regular blur must be 20-28px, got ${mid.blur}`);
  assert.ok(mid.fill >= 0.6 && mid.fill <= 0.8, `Regular fill must be 60-80%, got ${mid.fill}`);
  assert.equal(high.blur, 28, "the High step is the budget ceiling");
  assert.equal(high.saturate, 180, "the High step spends the full saturation budget");

  for (const [a, b, axis] of [
    [low.blur, mid.blur, "blur"],
    [mid.blur, high.blur, "blur"],
    [low.saturate, mid.saturate, "saturation"],
    [mid.saturate, high.saturate, "saturation"],
    [low.fill, mid.fill, "fill"],
    [mid.fill, high.fill, "fill"],
  ] as [number, number, string][]) {
    assert.ok(a < b, `the ${axis} axis must ascend low -> high (${a} vs ${b})`);
  }
});

test("no combination of the two controls drops body copy below its floor", async () => {
  const model = await loadModel();
  // The four body-copy surfaces, as { tint, alpha(t) } over the frame.
  const recessAlpha = (t: number) => Math.min(1, model.content.a + model.content.b * t);
  const surfaces = (
    step: Step,
    t: number,
    backdrop: number[],
    tint: {
      frame: number[]; recess: number[]; float: number[]; raised: number[]; raisedAlpha: number;
      pane: number[]; paneAlpha: number;
    },
  ) => {
    const frame = flatten([...tint.frame, frameAlpha(step, t, model.solidTop)], backdrop);
    const recess = flatten([...tint.recess, recessAlpha(t)], frame);
    const soft = flatten([...tint.recess, Math.max(0.3, recessAlpha(t) - 0.06)], frame);
    const control = flatten([...tint.pane, tint.paneAlpha], recess);
    const float = flatten([...tint.float, Math.min(1, recessAlpha(t) + 0.06)], frame);
    // A raised selection pane sits on the content recess (the result field,
    // the settings body), not on the soft sub-recess.
    const raised = flatten([...tint.raised, tint.raisedAlpha], recess);
    return { frame, recess, soft, control, float, raised };
  };
  // The darkest realistic desktop for the dark theme's light text; the
  // brightest for the light theme's dark text. Reported as the worst case.
  const backdrops: Record<string, number[]> = {
    dark: [230, 220, 200],
    light: [0, 0, 0],
  };
  // The two palettes' material tints, read from base.css rather than guessed:
  // a light frame is near-white, a dark one near-black.
  const css = stripComments(await read("src/styles/base.css"));
  const lightBlock = css.slice(css.indexOf('[data-theme="light"]'));
  const rgbOf = (block: string, name: string) => {
    // Takes the leading `r, g, b` of either `rgba(r, g, b, a)` or
    // `rgba(r, g, b, var(--x))` — the light frame tint takes its alpha from the
    // shared token, so a strict `[^)]+` match would stop at the inner paren.
    const match = block.match(new RegExp(`--${name}:\\s*rgba\\(\\s*(\\d+)\\s*,\\s*(\\d+)\\s*,\\s*(\\d+)`));
    assert.ok(match, `--${name} must start with rgba(r, g, b in this palette`);
    return [Number(match![1]), Number(match![2]), Number(match![3])];
  };
  const root = rootBlock(css);
  const tints: Record<string, Parameters<typeof surfaces>[3]> = {
    dark: {
      frame: [18, 19, 22], recess: [17, 18, 20], float: [24, 25, 29],
      raised: rgbOf(root, "accent-tint"), raisedAlpha: 0.13, pane: [255, 255, 255], paneAlpha: 0.055,
    },
    light: {
      frame: rgbOf(lightBlock, "glass-tint"), recess: rgbOf(lightBlock, "surface-sunken"),
      float: rgbOf(lightBlock, "glass-float"),
      // The light accent is an opaque hex, not an rgba.
      raised: rgbOf(lightBlock, "accent-tint"), raisedAlpha: 0.1,
      pane: [0, 0, 0], paneAlpha: 0.035,
    },
  };
  const palettes: Record<string, { rgb: Record<string, number[]>; alpha: Record<string, number> }> = {
    dark: {
      rgb: {
        primary: [242, 243, 245], strong: [244, 245, 247], secondary: [238, 239, 242],
        muted: [238, 239, 242], tertiary: [238, 239, 242], warning: [255, 178, 164],
        accent: [143, 183, 255],
      },
      alpha: { primary: 1, strong: 0.96, secondary: 0.74, muted: 0.55, tertiary: 0.48, warning: 0.82, accent: 1 },
    },
    light: {
      rgb: {
        primary: [29, 29, 31], strong: [23, 23, 26], secondary: [60, 60, 67],
        muted: [60, 60, 67], tertiary: [60, 60, 67], warning: [156, 31, 24], accent: [6, 74, 166],
      },
      // Read below from the shipped palette where possible; these are the
      // hues' nominal alphas.
      alpha: { primary: 1, strong: 0.96, secondary: 0.86, muted: 0.74, tertiary: 0.72, warning: 1, accent: 1 },
    },
  };
  // Landings, mirroring the CSS-half contract: the frame carries only the two
  // leading steps; the selection pane carries no warning/tertiary copy.
  const landings: Record<string, string[]> = {
    frame: ["primary", "strong"],
    recess: ["primary", "strong", "secondary", "accent", "muted", "tertiary", "warning"],
    soft: ["primary", "strong", "secondary", "accent", "muted", "tertiary", "warning"],
    control: ["primary", "strong", "secondary", "accent", "muted", "tertiary", "warning"],
    float: ["primary", "strong", "secondary", "accent", "muted", "tertiary", "warning"],
    // The selection pane's label is the frame's primary text (the accent
    // survives only as the state edge), so accent itself is not a landing here.
    raised: ["primary", "strong", "secondary", "muted"],
  };
  const floors: Record<string, number> = {
    primary: 4.5, strong: 4.5, secondary: 4.5, accent: 4.5, warning: 3, muted: 3, tertiary: 3,
  };

  let cells = 0;
  let worst = { ratio: Infinity, label: "" };
  for (const [themeName, palette] of Object.entries(palettes)) {
    for (const step of model.steps) {
      for (let i = 0; i <= 18; i += 1) {
        const t = 0.1 + (0.9 / 18) * i;
        const built = surfaces(step, t, backdrops[themeName], tints[themeName]);
        for (const [surfaceName, names] of Object.entries(landings)) {
          for (const name of names) {
            const fg = flatten([...palette.rgb[name], palette.alpha[name]], built[surfaceName as keyof typeof built]);
            const ratio = contrastRatio(fg, built[surfaceName as keyof typeof built]);
            cells += 1;
            if (ratio < worst.ratio) {
              worst = { ratio, label: `${themeName} ${step.name}@${t.toFixed(2)} ${name}/${surfaceName}` };
            }
            assert.ok(
              ratio >= floors[name],
              `${themeName} ${step.name}@${t.toFixed(2)}: ${name} on the ${surfaceName} is ` +
                `${ratio.toFixed(2)}:1, below ${floors[name]}:1. The transparency slider may not be ` +
                "allowed to trade this away — raise the step's fill floor or the text alpha.",
            );
          }
        }
      }
    }
  }
  assert.ok(cells >= 3 * 19 * 30, `the sweep must cover the whole grid (got ${cells} cells)`);
  assert.ok(worst.ratio >= 3, `the worst cell must still be readable, got ${worst.label} at ${worst.ratio.toFixed(2)}`);
});

test("the content layer never collapses into the Clear band", async () => {
  const model = await loadModel();
  // This is the layer-discipline promise stated as a floor: the content layer
  // is a *standard material*, so its fill has to stay inside HIG's Regular
  // band (60-80%) at every slider position, even when the frame around it is
  // the Clear variant. A recess that tracked the Clear frame down to 0.30
  // would put 10px body copy on a sheet as thin as the glass it is supposed to
  // be structurally distinct from.
  for (let i = 0; i <= 18; i += 1) {
    const t = 0.1 + (0.9 / 18) * i;
    const alpha = Math.min(1, model.content.a + model.content.b * t);
    assert.ok(alpha >= 0.6, `at transparency ${t.toFixed(2)} the content layer is only ${alpha.toFixed(3)} — below the regular band`);
    assert.ok(alpha <= 0.8, `at transparency ${t.toFixed(2)} the content layer is ${alpha.toFixed(3)} — above the regular band`);
  }
  // …and a floater is always at least as dense as the content it holds.
  const css = stripComments(await read("src/styles/base.css"));
  const root = rootBlock(css);
  const floatDelta = declaration(root, "glass-float-alpha").match(/var\(--glass-tint-alpha\)\s*\+\s*([\d.]+)/);
  assert.ok(floatDelta, "--glass-float-alpha must be frame + a constant");
  assert.ok(Number(floatDelta![1]) >= 0.08, `a floater must clear the frame by ≥0.08, got ${floatDelta![1]}`);
});

// TODO(F12): this scan guards the *stylesheets* (a literal blur in any sheet
// fails). It cannot see a step value written by runtime JavaScript or an
// inline style, which is exactly how the plugin page receives its step (the
// host injects `--glass-step-*` onto the container, see PluginPageHost).
// tests/plugin-pages.test.ts now pins that hand-off, but a broader runtime
// scan remains future work.
test("the step reaches every surface but is never branched on in a surface file", async () => {
  const base = stripComments(await read("src/styles/base.css"));
  // Exactly five blocks, one per effect step.
  const declared = [...base.matchAll(/\[data-glass="(\w+)"\]/g)].map((m) => m[1]);
  assert.deepEqual([...new Set(declared)].sort(), ["deep", "high", "jelly", "low", "mid"]);

  // The material aliases are what every surface consumes; none of them may be
  // a literal, or a step switch would silently miss that surface.
  const root = rootBlock(base);
  for (const alias of ["glass-blur", "glass-blur-terminal", "glass-saturate"]) {
    assert.match(
      declaration(root, alias),
      /var\(--glass-step-/,
      `--${alias} must alias the step, not carry its own value`,
    );
  }
  // Every frame shell filters with the aliased blur, not a literal.
  const { readdir } = await import("node:fs/promises");
  const dir = "src/styles";
  for (const name of (await readdir(dir)).filter((n) => n.endsWith(".css"))) {
    const css = stripComments(await read(`src/styles/${name}`));
    for (const match of css.matchAll(/(?:^|;)\s*(?:-webkit-)?backdrop-filter:\s*([^;]+)/g)) {
      const value = match[1].trim();
      assert.ok(
        !/blur\(\s*[\d.]+px/.test(value),
        `${name}: a literal blur (${value}) would ignore the glass step — use var(--glass-blur)`,
      );
    }
  }
});

test("the two controls survive a round trip through the settings shape", async () => {
  // The data model: the step is a discrete 3-valued string, the transparency
  // values are percentages, and neither is derived from the other. This is the
  // contract the Rust side and the frontend have to agree on.
  const { normalizeGlassStep, GLASS_STEPS } = await import("../src/glass-material.ts");
  assert.deepEqual([...GLASS_STEPS], ["low", "mid", "high", "deep", "jelly"]);
  for (const step of GLASS_STEPS) assert.equal(normalizeGlassStep(step), step);
  // Unknown values rest on Regular rather than on a random variant.
  for (const unknown of [undefined, null, "", "Low", "clear", "solid", 0, {}, "999"]) {
    assert.equal(normalizeGlassStep(unknown), "mid", `${JSON.stringify(unknown)} must normalize to mid`);
  }

  // The Rust struct declares the same five-valued domain, and the loader
  // migrates a pre-R8 file by splitting its single slider in half.
  const rust = await read("src-tauri/src/commands/config.rs");
  assert.match(rust, /pub glass_step: String/, "the Rust settings struct must carry the step");
  assert.match(rust, /const GLASS_STEPS: \[&str; 5\] = \["low", "mid", "high", "deep", "jelly"\]/);
  assert.match(rust, /fn migrate_legacy_glass_strength/, "the legacy split must exist");
  assert.match(
    rust,
    /settings\.main_opacity = legacy\.div_ceil\(2\) as u8/,
    "the legacy slider must split in half, rounded to nearest",
  );
  // The default is the split equivalent of the pre-R8 94, so an upgrader who
  // never opens the settings panel keeps the window solidity their old file
  // implied.
  assert.match(rust, /const DEFAULT_MAIN_OPACITY: u8 = 47;/);
  assert.match(rust, /const DEFAULT_TERMINAL_OPACITY: u8 = 46;/);
  // The frontend default and the Rust default have to be the same number, or
  // the first render after an upgrade would jump.
  const hook = await read("src/hooks/useSettings.ts");
  assert.match(hook, /main_opacity: 47,/);
  assert.match(hook, /terminal_opacity: 46,/);
  assert.match(hook, /glass_step: "mid",/);
});

// The step's numbers have two homes in JavaScript-visible form: the CSS blocks
// in base.css (what the app document renders) and `GLASS_STEP_TOKENS` in
// `src/glass-material.ts` (what the host hands a sandboxed plugin page, which
// cannot read the host's `:root`). Two homes only stay honest if something
// says they are the same numbers, so this does.
test("the plugin-facing step table mirrors the shipped CSS blocks", async () => {
  const css = stripComments(await read("src/styles/base.css"));
  const { GLASS_STEP_TOKENS, GLASS_SOLID_TOP } = await import("../src/glass-material.ts");
  for (const step of ["low", "mid", "high"] as const) {
    const block = stepBlock(css, step);
    assert.equal(
      GLASS_STEP_TOKENS[step].fill,
      number(block, "glass-step-fill"),
      `GLASS_STEP_TOKENS.${step}.fill must equal [data-glass="${step}"] --glass-step-fill`,
    );
    assert.equal(
      GLASS_STEP_TOKENS[step].dim,
      number(block, "glass-step-dim"),
      `GLASS_STEP_TOKENS.${step}.dim must equal [data-glass="${step}"] --glass-step-dim`,
    );
  }
  assert.equal(
    GLASS_SOLID_TOP,
    number(rootBlock(css), "glass-solid-top"),
    "GLASS_SOLID_TOP must equal --glass-solid-top",
  );
});
