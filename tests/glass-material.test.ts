// R7-GLASS + R7-GLASS-DEEP: the app has exactly one sheet of glass per
// surface, and every control on top of it is the *same* glass, one step
// thinner. Before R7-GLASS every surface carried its own blur and so did the
// clipboard panel's host material, and the "transparency" slider wrote a raw
// alpha into `--input-bg`; now the shell is the only element that filters the
// desktop, everything inside it floats on plain tint, and the slider drives a
// glass formula instead of an alpha.
//
// R7-GLASS-DEEP then had to make the glass *visible*: the original affine map
// (0.25 + 0.72·op) put the default at 0.927, a 93%-opaque panel that read as a
// solid sheet, and the controls on top of it were still pre-glass flat fills.
// This round lowered the map to (0.18 + 0.55·op), re-derived every dependent
// token as a *ladder* (float > shell > sunken, control < shell), raised the rim
// highlight to a legible 15%, and moved every control onto shared ladder
// tokens.
//
// These assertions keep all of that from drifting: the element count stays at
// one per surface, no control ever grows a blur of its own, the derived tint
// keeps coming from the untouched `--main-opacity`/`--terminal-opacity` chain,
// the ladder keeps its ordering at every slider position, and no transition
// ever animates a filter.
//
// Slices are taken by structure (a rule's own braces, the media block's own
// braces), never by a fixed-width window around a match.
import assert from "node:assert/strict";
import { readdir, readFile } from "node:fs/promises";
import test from "node:test";

const root = new URL("../", import.meta.url);
const read = (path: string) => readFile(new URL(path, root), "utf8");
const stripComments = (css: string) => css.replace(/\/\*[\s\S]*?\*\//g, "");

// The one element per surface allowed to filter the backdrop. The clipboard
// plugin page declares no blur of its own since R7-4b: its in-page
// backdrop-filter was dead code (a sandboxed iframe samples its own
// document's backdrop, never the host's pixels) and was removed.
const GLASS_SHELLS = [".collapsed-card", ".settings-card", ".terminal-panel"];

// Host sheets. `src/plugins/clipboard/page.css` is the plugin page's own
// document; it is the one sheet the control scan below deliberately does not
// read (its material comes from host-injected --page-fill/--panel-bg, not
// from any in-page filter).
const styleFiles = async () => {
  const dir = new URL("src/styles/", root);
  const names = (await readdir(dir)).filter((n) => n.endsWith(".css"));
  const surfaces = await Promise.all(
    names.map(async (name) => ({ name: `src/styles/${name}`, css: stripComments(await read(`src/styles/${name}`)) })),
  );
  const dialog = {
    name: "src/extensions/ComponentizedUninstallDialog.css",
    css: stripComments(await read("src/extensions/ComponentizedUninstallDialog.css")),
  };
  return [...surfaces, dialog];
};

// Every rule in a sheet, as { selector, body }. Rule bodies never contain a
// nested brace in this codebase, so the pairing is exact.
const rules = (css: string) => {
  const out: { selector: string; body: string }[] = [];
  for (const match of css.matchAll(/([^{}]+)\{([^{}]*)\}/g)) {
    out.push({ selector: match[1].trim(), body: match[2] });
  }
  return out;
};

const declarations = (body: string, property: string) =>
  [...body.matchAll(new RegExp(`(?:^|;)\\s*${property}\\s*:\\s*([^;]+)`, "g"))].map((m) => m[1].trim());

// Walk a media block by its own braces so the assertions below read exactly the
// block's content, not whatever happens to follow it.
const mediaBlock = (css: string, query: string) => {
  const start = css.indexOf(`@media ${query}`);
  assert.notEqual(start, -1, `missing @media ${query}`);
  const open = css.indexOf("{", start);
  let depth = 0;
  for (let i = open; i < css.length; i += 1) {
    if (css[i] === "{") depth += 1;
    else if (css[i] === "}") {
      depth -= 1;
      if (depth === 0) return css.slice(open + 1, i);
    }
  }
  assert.fail(`unterminated @media ${query}`);
};

// The `:root` token block of base.css, up to the light-theme override. Every
// glass assertion reads this slice so a token can never be found in a nested
// block by accident.
const rootTokens = async () => {
  const base = stripComments(await read("src/styles/base.css"));
  return base.slice(base.indexOf(":root {"), base.indexOf('[data-theme="light"]'));
};

const token = (block: string, name: string) => {
  const match = block.match(new RegExp(`--${name}:\\s*([^;]+);`));
  assert.ok(match, `--${name} must be defined in the dark token block`);
  return match![1].trim();
};

// A `--x: calc(a + b * var(--y))` map, read as { a, b }. Both coefficients are
// pinned by the tests below; the map is what the whole material system hangs
// off, so "it is a calc that mentions the variable" is not enough.

// The selectors that actually declare a backdrop-filter (either spelling), from
// every sheet in src/styles. `none` is an opt-out, not a material.
const filteringSelectors = async () => {
  const found = new Map<string, string>();
  for (const { name, css } of await styleFiles()) {
    for (const { selector, body } of rules(css)) {
      const values = [
        ...declarations(body, "backdrop-filter"),
        ...declarations(body, "-webkit-backdrop-filter"),
      ];
      if (values.some((value) => value !== "none")) {
        for (const part of selector.split(",")) found.set(part.trim(), name);
      }
    }
  }
  return found;
};

test("exactly one element per surface filters the backdrop", async () => {
  const found = await filteringSelectors();
  assert.deepEqual(
    [...found.keys()].sort(),
    [...GLASS_SHELLS].sort(),
    "only the surface shells may carry a backdrop-filter; cards, rows, drawers and " +
      "dialogs float on the shell's glass instead of blurring it again",
  );
  // Each shell declares the prefixed and the standard property exactly once.
  for (const { name, css } of await styleFiles()) {
    for (const { selector, body } of rules(css)) {
      const prefixed = declarations(body, "-webkit-backdrop-filter").filter((v) => v !== "none").length;
      const standard = declarations(body, "backdrop-filter").filter((v) => v !== "none").length;
      if (prefixed + standard === 0) continue;
      assert.equal(prefixed, 1, `${name}: ${selector} must pair -webkit-backdrop-filter once`);
      assert.equal(standard, 1, `${name}: ${selector} must pair backdrop-filter once`);
      assert.equal(
        declarations(body, "-webkit-backdrop-filter")[0],
        declarations(body, "backdrop-filter")[0],
        `${name}: ${selector} — the prefixed and standard values must be identical`,
      );
    }
  }
});

test("no card, row, drawer or dialog re-blurs the shell", async () => {
  // The selectors that would betray a second material. Naming them makes the
  // regression loud: if a later round gives the drawer its own blur, the test
  // says which component broke the one-sheet rule.
  const contentSelectors = [
    ".launcher-result",
    ".launcher-bottom",
    ".launcher-action-bar",
    ".launcher-feedback",
    ".settings-sidebar",
    ".settings-content",
    ".extension-row",
    ".extension-drawer",
    ".extension-permission-dialog",
    ".extension-custom-dialog",
    ".extension-removal-dialog",
    ".extension-menu__items",
    ".app-toast",
    ".clipboard-row",
    ".pinned-card",
  ];
  const found = await filteringSelectors();
  for (const selector of contentSelectors) {
    for (const [carrier, file] of found) {
      assert.ok(
        !carrier.startsWith(selector),
        `${file}: ${carrier} carries a backdrop-filter — ${selector} content must not`,
      );
    }
  }
});

// R7-GLASS-DEEP's hard performance rule: the control layer is plain rgba over
// the shell, never a second blur. The per-surface test above proves the *live*
// shells are the only filters, but it is written against a fixed allow-list;
// this test is the general form of the same law, and it is the one a mutation
// that adds `backdrop-filter` to, say, `.extensions-action-button` has to trip.
test("no control rule anywhere in the host sheets declares a filter", async () => {
  const control = /(button|switch|tab|input|select|textarea|option|row|result|field|control|menu|toast|notice|chip|badge|track|thumb|drawer|dialog|card)/i;
  for (const { name, css } of await styleFiles()) {
    for (const { selector, body } of rules(css)) {
      if (!control.test(selector)) continue;
      // The three surface shells are the material itself, not a control on top
      // of it; every *other* selector that looks like a control has to be a
      // plain translucent pane.
      if (selector.split(",").some((part) => GLASS_SHELLS.includes(part.trim()))) continue;
      for (const property of ["backdrop-filter", "-webkit-backdrop-filter"]) {
        const values = declarations(body, property).filter((value) => value !== "none");
        assert.deepEqual(
          values,
          [],
          `${name}: ${selector} carries ${property} — controls are translucent rgba over the ` +
            "shell's single sheet, never a blur of their own (the performance red line)",
        );
      }
    }
  }
});

test("the window transparency chain drives the frame and nothing else", async () => {
  const rootBlock = await rootTokens();
  baseCache ??= await read("src/styles/base.css");
  // The chain itself is unchanged: both variables are still declared here and
  // still written by App.tsx, and they are now the *transparency* control.
  assert.match(rootBlock, /--main-opacity:\s*0\.47;/);
  assert.match(rootBlock, /--terminal-opacity:\s*0\.46;/);
  assert.match(await read("src/App.tsx"), /setProperty\("--main-opacity"/);
  assert.match(await read("src/App.tsx"), /setProperty\("--terminal-opacity"/);
  // The frame fill consumes them, and only with calc + rgba (no color-mix,
  // whose WebKitGTK support varies across the versions floter ships to).
  const glassTokens = rootBlock.slice(rootBlock.indexOf("--glass-frame-alpha"), rootBlock.indexOf("--input-stroke"));
  assert.match(glassTokens, /--glass-frame-alpha:\s*calc\(\s*[\s\S]*?var\(--main-opacity\)\s*\);/);
  assert.match(glassTokens, /--glass-frame-alpha-terminal:\s*calc\(\s*[\s\S]*?var\(--terminal-opacity\)\s*\);/);
  assert.ok(!/color-mix/.test(glassTokens), "the derived frame fill must not use color-mix");

  // R8's headline: the transparency slider is the readability control, so its
  // maxed end has to be a near-opaque frame — the R7 map topped out at 0.73
  // and the user's complaint ("still see-through at 100%") is exactly that.
  assert.match(token(rootBlock, "glass-solid-top"), /^0\.9[5-9]$/);
  for (const step of ["low", "mid", "high"]) {
    const floor = stepTokens(step).fill;
    // frame(t=1) = floor + (solidTop - floor) * 1 = solidTop on every step.
    assert.ok(
      floor + (SOLID_TOP - floor) * 1 >= 0.95,
      `${step}: the maxed transparency slider must reach ≥0.95, got ${floor + (SOLID_TOP - floor)}`,
    );
  }
});

// ── The glass step (R8) ────────────────────────────────────────────────────
//
// The material is three discrete variants, and they are HIG's own: `low` is
// the Clear variant (thin blur, low fill, and Apple's published 35% dimming
// layer for bright content), `mid` is Regular, and `high` is Regular pushed to
// this round's budget ceiling. The numbers live in `[data-glass]` blocks in
// base.css; these assertions keep them from drifting into each other, which is
// the whole failure mode of a three-step control (low and mid converging until
// the control does nothing).

/** Parse one `[data-glass="x"]` block into its four step tokens. */
const stepTokens = (step: string) => {
  const base = stripComments(baseCache ?? "");
  const start = base.indexOf(`[data-glass="${step}"]`);
  assert.notEqual(start, -1, `base.css must define [data-glass="${step}"]`);
  // A block's own braces, so the slice is structural rather than a fixed
  // window around the match.
  const open = base.indexOf("{", start);
  let depth = 0;
  let end = open;
  for (let i = open; i < base.length; i += 1) {
    if (base[i] === "{") depth += 1;
    else if (base[i] === "}") {
      depth -= 1;
      if (depth === 0) { end = i; break; }
    }
  }
  const body = base.slice(open + 1, end);
  const read = (name: string) => {
    const match = body.match(new RegExp(`--glass-step-${name}:\\s*([^;]+);`));
    assert.ok(match, `[data-glass="${step}"] must define --glass-step-${name}`);
    return match![1].trim();
  };
  return {
    blur: Number(read("blur").replace("px", "")),
    saturate: Number(read("saturate").replace("%", "")),
    fill: Number(read("fill")),
    dim: Number(read("dim")),
  };
};

const SOLID_TOP = 0.98;

// The three steps, read once so the band assertions below can share them.
let baseCache: string | null = null;
test("the glass step is three HIG variants, monotonic low < mid < high", async () => {
  baseCache = await read("src/styles/base.css");
  const steps = { low: stepTokens("low"), mid: stepTokens("mid"), high: stepTokens("high") };

  // The variant bands from `liquid-glass.md › Cross-platform translation`:
  // clear 8-16px / 20-40% fill / a 35% dimming layer, regular 20-40px /
  // 60-80%. `high` sits at the round's 28px budget ceiling.
  assert.ok(steps.low.blur >= 8 && steps.low.blur <= 16, `Clear blur must sit in 8-16px, got ${steps.low.blur}`);
  assert.ok(steps.low.fill >= 0.2 && steps.low.fill <= 0.4, `Clear fill must sit in 20-40%, got ${steps.low.fill}`);
  assert.equal(steps.low.dim, 0.35, "Clear glass over bright content carries the published 35% dimming layer");
  assert.ok(steps.mid.blur >= 20 && steps.mid.blur <= 28, `Regular blur must sit in 20-28px, got ${steps.mid.blur}`);
  assert.ok(steps.mid.fill >= 0.6 && steps.mid.fill <= 0.8, `Regular fill must sit in 60-80%, got ${steps.mid.fill}`);
  assert.equal(steps.mid.dim, 0, "the Regular variant needs no dimming layer");
  assert.equal(steps.high.blur, 28, "the High step sits at the 28px budget ceiling");
  assert.equal(steps.high.saturate, 180, "the High step sits at the 180% saturation budget");
  assert.equal(steps.high.dim, 0, "the High step needs no dimming layer");

  // Direction monotonicity: a higher step is more material — more blur, more
  // saturation, more fill. This is the "档位映射错位" mutation's target: swap
  // any two steps' values and one of these fails.
  assert.ok(steps.low.blur < steps.mid.blur && steps.mid.blur < steps.high.blur, "blur must ascend low < mid < high");
  assert.ok(steps.low.saturate < steps.mid.saturate && steps.mid.saturate < steps.high.saturate, "saturation must ascend");
  assert.ok(steps.low.fill < steps.mid.fill && steps.mid.fill < steps.high.fill, "fill must ascend");
  assert.ok(steps.low.dim > steps.mid.dim && steps.mid.dim === steps.high.dim, "only Clear carries a dimming layer");
  for (const [name, step] of Object.entries(steps)) {
    assert.ok(step.blur <= 28, `${name}: blur exceeds the 28px budget`);
    assert.ok(step.saturate <= 180, `${name}: saturate exceeds the 180% budget`);
  }

  // The frame at a given transparency must ascend with the step at *every*
  // slider position — otherwise the control would invert mid-range. At the
  // top the three converge (all near-solid), which is correct: the user asked
  // for an opaque window.
  for (let t = 0.1; t <= 0.9501; t += 0.05) {
    const frame = (s: { fill: number; dim: number }) => {
      const fill = s.fill + (SOLID_TOP - s.fill) * t;
      const dim = s.dim * (1 - t);
      return 1 - (1 - dim) * (1 - fill);
    };
    const [lo, mi, hi] = [frame(steps.low), frame(steps.mid), frame(steps.high)];
    assert.ok(lo < mi && mi < hi, `frame must ascend at transparency ${t.toFixed(2)}: ${lo} / ${mi} / ${hi}`);
  }
  // …and at 100% every step is the near-solid frame the user asked for.
  for (const [name, step] of Object.entries(steps)) {
    const fill = step.fill + (SOLID_TOP - step.fill) * 1;
    const dim = step.dim * (1 - 1);
    const frame = 1 - (1 - dim) * (1 - fill);
    assert.ok(frame >= 0.95, `${name}: at 100% transparency the frame must be near-solid, got ${frame.toFixed(3)}`);
  }
});

// The step is selected by an attribute on <html> rather than by a class on
// each surface, which is what makes it one mechanism instead of a branch per
// file. A later round that switched it in JavaScript would silently lose the
// a11y override keying, so the mechanism itself is pinned.
test("the step is an html attribute, and no surface file names a step", async () => {
  assert.match(
    await read("src/App.tsx"),
    /setAttribute\("data-glass",\s*settings\.glass_step\)/,
    "App.tsx must write the step onto <html> as data-glass",
  );
  // No surface sheet may branch on a step: the tokens are the only interface.
  for (const { name, css } of await styleFiles()) {
    if (name === "src/styles/base.css") continue; // the definition site
    assert.ok(
      !/data-glass/.test(css),
      `${name}: a surface file must not branch on data-glass — it consumes --glass-step-* tokens`,
    );
  }
  // base.css is the only file that defines the blocks, and it defines exactly
  // five: a sixth would be an undocumented effect step.
  const base = stripComments(await read("src/styles/base.css"));
  const blocks = [...base.matchAll(/\[data-glass="(\w+)"\]/g)].map((m) => m[1]);
  assert.deepEqual([...new Set(blocks)].sort(), ["deep", "high", "jelly", "low", "mid"]);
});


// The material ladder. R7-HIG's version of this test asserted
// `float > shell > sunken` — a frame-only ladder that R8 deliberately broke:
// the frames now slide from a Clear floor to near-solid while the content
// recess is a fixed standard-material band, so the two cross. What has to hold
// at every combination is the *composite*: the recess must read as denser than
// the frame it sits on, and a floater must read as denser than both.
test("the material ladder holds as composites across both controls", async () => {
  const rootBlock = await rootTokens();
  baseCache ??= await read("src/styles/base.css");

  // Float is written as frame + a constant, so "float is denser than the
  // frame" is structural rather than two formulas that agree by accident.
  const float = token(rootBlock, "glass-float-alpha");
  const delta = float.match(/^calc\(var\(--glass-tint-alpha\)\s*\+\s*([\d.]+)\)$/);
  assert.ok(delta, `--glass-float-alpha must be frame + a constant, got "${float}"`);
  assert.ok(Number(delta![1]) >= 0.08, `float must clear the frame by ≥0.08, got ${delta![1]}`);

  // The content layer is a fixed band of the *transparency* control, not a
  // fraction of the frame — that is the layer-discipline change of this round.
  const content = token(rootBlock, "glass-content-alpha");
  const contentMap = content.match(/^calc\(\s*([\d.]+)\s*\+\s*([\d.]+)\s*\*\s*var\(--main-opacity\)\s*\)$/);
  assert.ok(contentMap, `--glass-content-alpha must be calc(a + b·transparency), got "${content}"`);
  const [cA, cB] = [Number(contentMap![1]), Number(contentMap![2])];
  // The HIG regular band (60-80%) over the whole slider. The bottom of the
  // band is what keeps body copy legible at the thinnest frame; the top is
  // what keeps the recess reading as a recess rather than as paint.
  assert.ok(cA >= 0.6 && cA <= 0.7, `the content band must start in the regular range, got ${cA}`);
  assert.ok(cA + cB <= 0.81, `the content band must stay within 80%, got ${cA + cB}`);
  // The soft recess is the same band, one step thinner.
  const soft = token(rootBlock, "surface-sunken-soft");
  assert.match(soft, /calc\(var\(--glass-content-alpha\)\s*-\s*[\d.]+\)/);

  // The frame maps each step's fill floor to the near-solid top.
  const solidTop = Number(token(rootBlock, "glass-solid-top"));
  assert.ok(solidTop >= 0.95, `--glass-solid-top must be near-opaque, got ${solidTop}`);

  // Sweep every step × transparency and assert the composites stay ordered.
  const steps = { low: stepTokens("low"), mid: stepTokens("mid"), high: stepTokens("high") };
  for (const [name, step] of Object.entries(steps)) {
    for (let t = 0.1; t <= 1.0001; t += 0.05) {
      const fill = step.fill + (solidTop - step.fill) * t;
      const dim = step.dim * (1 - t);
      const frame = 1 - (1 - dim) * (1 - fill);
      const floatAlpha = Math.min(1, frame + Number(delta![1]));
      const contentAlpha = Math.min(1, cA + cB * t);
      // Composite opacity of the recess over the frame vs the frame alone.
      const recessComposite = 1 - (1 - frame) * (1 - contentAlpha);
      assert.ok(
        recessComposite >= frame,
        `${name}@${t.toFixed(2)}: the content recess must composite at least as dense as the frame ` +
          `(${recessComposite.toFixed(3)} vs ${frame.toFixed(3)})`,
      );
      assert.ok(
        floatAlpha >= frame,
        `${name}@${t.toFixed(2)}: a floater must never be thinner than the frame it floats over ` +
          `(${floatAlpha.toFixed(3)} vs ${frame.toFixed(3)})`,
      );
      assert.ok(floatAlpha <= 1, `${name}@${t.toFixed(2)}: float must stay within [0,1]`);
    }
  }
});

// The control ladder is its own ordering: a pane brightens under the pointer
// and gets *thinner* under the press, which is what makes a control read as
// sinking into the glass instead of popping out of it.
test("the control material ladder keeps rest < hover and press < rest", async () => {
  const rootBlock = await rootTokens();
  const alpha = (name: string) => {
    const value = token(rootBlock, name);
    const match = value.match(/rgba\(255,\s*255,\s*255,\s*([\d.]+)\)/);
    assert.ok(match, `--${name} must be a plain white rgba, got "${value}"`);
    return Number(match![1]);
  };
  const rest = alpha("glass-control");
  const hover = alpha("glass-control-hover");
  const press = alpha("glass-control-press");
  assert.ok(rest >= 0.04 && rest <= 0.07, `a resting pane must sit in the 4-7% band, got ${rest}`);
  assert.ok(hover >= 0.06 && hover <= 0.1, `a hovered pane must sit in the 6-10% band, got ${hover}`);
  assert.ok(press >= 0.03 && press <= 0.05, `a pressed pane must sit in the 3-5% band, got ${press}`);
  assert.ok(press < rest, `pressing must thin the pane (${press} < ${rest})`);
  assert.ok(hover > rest, `hovering must thicken the pane (${hover} > ${rest})`);
  assert.ok(alpha("glass-control-edge") <= 0.08, "the pane's inner stroke stays a hairline");
  assert.ok(alpha("glass-control-rim") >= 0.08, "the pane's top rim must be visible");

  // The historical aliases the rest of the stylesheet consumes have to *be*
  // the ladder, or the round's whole point (one material, no orphan colours)
  // would only be true of the tokens nobody uses.
  assert.equal(token(rootBlock, "surface-control"), "var(--glass-control)");
  assert.equal(token(rootBlock, "surface-control-hover"), "var(--glass-control-hover)");
  assert.equal(token(rootBlock, "stroke-control"), "var(--glass-control-edge)");
  assert.equal(token(rootBlock, "icon-surface"), "var(--glass-control)");
  assert.equal(token(rootBlock, "icon-edge"), "var(--glass-control-edge)");

  // Raised panes are the accent branch of the same ladder, and they carry the
  // same rim plus the elevation ladder's contact rung: selection is a lit pane
  // floating off the surface, not a flat block of accent colour.
  assert.equal(token(rootBlock, "glass-raised"), "var(--accent-tint)");
  assert.match(token(rootBlock, "glass-raised-shadow"), /var\(--elev-2\)/);
  assert.match(token(rootBlock, "glass-raised-shadow"), /inset 0 1px 0 var\(--glass-raised-rim\)/);
});

// The rim highlight is the single loudest liquid-glass cue. R7-GLASS shipped
// 11% / 22% and it was too shy to read as a lit edge; this round raised it and
// added the faint side stroke that closes the border into a full 1px ring.
test("the shell border is a lit top, a dark bottom and a full ring of glass", async () => {
  const rootBlock = await rootTokens();
  const rim = token(rootBlock, "glass-rim-shadow");
  const top = rootBlock.match(/--edge-highlight:\s*rgba\(255,\s*255,\s*255,\s*([\d.]+)\)/);
  const bottom = rootBlock.match(/--edge-shadow:\s*rgba\(0,\s*0,\s*0,\s*([\d.]+)\)/);
  const side = rootBlock.match(/--edge-highlight-x:\s*rgba\(255,\s*255,\s*255,\s*([\d.]+)\)/);
  assert.ok(top, "--edge-highlight must be defined");
  assert.ok(bottom, "--edge-shadow must be defined");
  assert.ok(side, "--edge-highlight-x must be defined");
  assert.ok(Number(top![1]) >= 0.15, `the top rim must be at least 15%, got ${top![1]}`);
  assert.ok(Number(bottom![1]) >= 0.1, `the bottom inner edge must be at least 10%, got ${bottom![1]}`);
  assert.ok(Number(side![1]) > 0 && Number(side![1]) <= 0.06, "the side stroke is a whisper, not a border");
  for (const [edge, variable] of [
    ["top", "--edge-highlight"],
    ["bottom", "--edge-shadow"],
    ["left", "--edge-highlight-x"],
    ["right", "--edge-highlight-x"],
  ] as const) {
    assert.match(
      rim,
      new RegExp(`inset [^,]*var\\(${variable}\\)`),
      `the rim must draw its ${edge} edge from ${variable}`,
    );
  }
  // The shells consume the token instead of restating the strokes, which is
  // what makes "the rim" one thing to tune.
  for (const file of ["launcher.css", "settings.css", "terminal.css"]) {
    const css = stripComments(await read(`src/styles/${file}`));
    const shell = rules(css).find(({ selector }) =>
      selector === ".collapsed-card" || selector === ".settings-card" || selector === ".terminal-panel",
    );
    assert.ok(shell, `${file}: the surface shell rule must exist`);
    assert.match(shell!.body, /box-shadow:\s*var\(--glass-rim-shadow\)/, `${file}: the shell must use --glass-rim-shadow`);
  }
});

test("glass values stay inside the performance budget", async () => {
  const rootBlock = await rootTokens();
  baseCache ??= await read("src/styles/base.css");

  // Every blur the material can render, wherever it is declared: the three
  // step tokens, and the two aliases the frame consumes. R7 derived the blur
  // from the slider and needed a `round()` quantizer; R8's blur is a literal
  // per step, so there is no calc to quantize and no `@supports` block — the
  // absence is asserted so a future round cannot quietly reintroduce a
  // slider-driven blur.
  const base = stripComments(baseCache!);
  const blurDeclarations = [...base.matchAll(/--glass-(?:step-)?blur[\w-]*:\s*([^;]+);/g)].map((m) => m[1]);
  assert.ok(blurDeclarations.length >= 4, "the three steps and the frame aliases must all declare a blur");
  for (const declaration of blurDeclarations) {
    for (const literal of declaration.matchAll(/([\d.]+)px/g)) {
      assert.ok(Number(literal[1]) <= 28, `blur component ${literal[0]} exceeds the 28px budget in "${declaration}"`);
    }
  }
  // The frame's blur is an alias of the step, not a formula over the slider.
  assert.match(rootBlock, /--glass-blur:\s*var\(--glass-step-blur\)/);
  assert.match(rootBlock, /--glass-blur-terminal:\s*var\(--glass-step-blur\)/);
  assert.ok(
    !/--glass-blur:\s*calc\(/.test(base),
    "the blur must be a per-step literal again — a calc here means the slider is driving the material",
  );
  assert.ok(
    !/@supports \(width: round\(/.test(base),
    "the round() quantizer existed to smooth a slider-driven blur; with discrete steps it is gone",
  );

  // Saturation is a per-step literal too, inside the round's 180% ceiling.
  const saturations = ["low", "mid", "high"].map((step) => stepTokens(step).saturate);
  for (const value of saturations) {
    assert.ok(value <= 180, `saturate ${value}% exceeds the 180% budget`);
    assert.ok(value >= 130, `saturate ${value}% is below the material's floor`);
  }
  assert.equal(Math.max(...saturations), 180, "the High step must spend the full saturation budget");
  assert.match(rootBlock, /--glass-saturate:\s*var\(--glass-step-saturate\)/);
});

test("no transition or animation ever names a filter", async () => {
  for (const { name, css } of await styleFiles()) {
    for (const { selector, body } of rules(css)) {
      for (const property of ["transition", "transition-property", "animation", "will-change"]) {
        for (const value of declarations(body, property)) {
          assert.ok(
            !/(?:^|[\s,])-?(?:-webkit-)?(?:backdrop-)?filter(?:[\s,]|$)/.test(value),
            `${name}: ${selector} ${property}: ${value} — filters are never animated`,
          );
        }
      }
    }
  }
});

test("reduced transparency turns the shells into near-solid panels", async () => {
  const base = stripComments(await read("src/styles/base.css"));
  const block = mediaBlock(base, "(prefers-reduced-transparency: reduce)");
  const covered = new Set<string>();
  const opaque = new Set<string>();
  for (const { selector, body } of rules(block)) {
    if (!/(?:^|;)\s*backdrop-filter\s*:\s*none/.test(body)) continue;
    for (const part of selector.split(",")) covered.add(part.trim());
    if (/(?:^|;)\s*background\s*:\s*var\(--surface-opaque\)/.test(body)) {
      for (const part of selector.split(",")) opaque.add(part.trim());
    }
  }
  for (const shell of [...GLASS_SHELLS, ".clipboard-panel"]) {
    assert.ok(
      covered.has(shell),
      `prefers-reduced-transparency must drop the blur on ${shell}`,
    );
    // The fill has to be swapped too, or a shell with the blur removed would
    // show the desktop through a barely-there tint.
    assert.ok(
      opaque.has(shell),
      `prefers-reduced-transparency must fall back to --surface-opaque on ${shell}`,
    );
  }
  // The fallback has to actually be near-solid. Now that the glass tint is much
  // lighter, a fallback that tracked it would leave a translucent panel with no
  // blur behind it — the worst of both.
  const opaqueToken = token(await rootTokens(), "surface-opaque");
  const alpha = opaqueToken.match(/rgba\([^)]*,\s*([\d.]+)\)/);
  assert.ok(alpha, "--surface-opaque must be an rgba");
  assert.ok(Number(alpha![1]) >= 0.97, `--surface-opaque must be near-solid, got ${alpha![1]}`);
  // The focused card's own rule is more specific than the plain selector, so
  // without this the launcher the user is typing into would keep its glass
  // while every other shell went opaque. The assertion is only meaningful if
  // that more-specific rule really exists.
  const launcher = (await styleFiles()).find((f) => f.name.endsWith("launcher.css"))!;
  const focusedRule = rules(launcher.css).find((r) => r.selector === ".collapsed-card:focus-within");
  assert.ok(focusedRule, "sanity: launcher.css still has a .collapsed-card:focus-within rule");
  assert.match(
    focusedRule!.body,
    /background:/,
    "sanity: the focused rule still sets a background, which is why the fallback must name it",
  );
  assert.ok(
    opaque.has(".collapsed-card:focus-within"),
    "the focused collapsed card must fall back too — its rule outranks the plain selector",
  );
});

test("R7-4b: the plugin page declares no blur, and the host shell is the sole provider", async () => {
  // The clipboard page runs in a sandboxed iframe. Its backdrop is its OWN
  // document's background, not the host's desktop — WebKitGTK will not let a
  // frame filter content across the frame boundary. That makes a
  // `backdrop-filter` on `.clipboard-panel` inert: it spends a slot of the
  // same-screen filter budget and blurs a flat fill. R7-4b removed it, so the
  // page's sheet owns zero filters and the one real blur is the shell's.
  const page = stripComments(await read("src/plugins/clipboard/page.css"));
  const carriers = rules(page).filter(({ body }) => /(?:^|;)\s*(?:-webkit-)?backdrop-filter\s*:/.test(body));
  assert.deepEqual(carriers, [], "clipboard/page.css must not declare a backdrop-filter (R7-4b)");
  // The material the page paints is still host-supplied: the page consumes the
  // injected `--page-fill` / `--panel-bg`, which main.ts derives from the
  // step tokens the host hands across the bridge. The subtraction must not
  // turn the page into a flat hole, so the tint has to still be there.
  const panel = rules(page).find(({ selector }) => selector === ".clipboard-panel");
  assert.ok(panel, "clipboard/page.css must still define .clipboard-panel's material");
  assert.match(panel!.body, /var\(--panel-bg\)/, "the page still paints the host-injected fill");
  // The shell it sits in keeps exactly one blur — that is the filter the page
  // consumes. If this ever disappears, the page has no glass at all.
  const host = stripComments(await read("src/styles/terminal.css"));
  const shell = rules(host).find(({ selector }) => selector === ".terminal-panel");
  assert.ok(shell, "terminal.css must define .terminal-panel");
  assert.match(shell!.body, /backdrop-filter:\s*blur\(var\(--glass-blur-terminal\)\)/);
});

// ── Text legibility on the thinner glass ──────────────────────────────────
//
// R7-GLASS-DEEP lets roughly a third of the desktop through the shell, so the
// composited surface under every glyph is lighter than it used to be and the
// old muted text alphas stopped clearing 3:1 over a bright backdrop. The three
// muted steps and the warning colour were re-tuned for it. These assertions
// recompute WCAG contrast against the composited material stack (backdrop →
// shell tint → sunken field → control/raised pane) at the default strength, so
// a later round that thins the glass again, or drops a text alpha back, fails
// here rather than in a screenshot review.
//
// The one combination that is *deliberately* left below the bar is the shell's
// own surface over a pure-white desktop — see the report. The slider is a
// legibility/transparency tradeoff the user owns, and the surfaces where text
// actually lives (the sunken result field, the settings body, the control
// panes, the floating layer) are asserted at the bright-wallpaper case, which
// is the brightest realistic desktop.

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
// `rgba(r, g, b, a)` flattened over an opaque backdrop.
const flatten = (rgba: number[], backdrop: number[]) =>
  rgba.slice(0, 3).map((c, i) => c * rgba[3] + backdrop[i] * (1 - rgba[3]));

// The text ladder, as { rgb, alpha }. Every value is read from base.css by the
// test below, so this table is the *hue* and the order, not the shipped alpha.
const TEXT_RGB = {
  "text-primary": [242, 243, 245],
  "text-strong": [244, 245, 247],
  "text-secondary": [238, 239, 242],
  "text-muted": [238, 239, 242],
  "text-tertiary": [238, 239, 242],
  accent: [143, 183, 255],
  "text-warning": [255, 178, 164],
};

// Backdrops, from the darkest realistic desktop to a blown-out bright one.
// "bright wallpaper" is the worst case asserted against; pure white is the
// measurement-only extreme reported in the round's report.
const BRIGHT_WALLPAPER = [230, 220, 200];
const GRADIENT_MEAN = [128, 110, 140];

// ── The R8 readability contract ────────────────────────────────────────────
//
// The user's complaint that opened this round was a readability one: at 100%
// the old single slider still left a quarter of the desktop showing through.
// R8 answers it three ways — the transparency control now reaches a near-solid
// frame, the material is a discrete HIG variant, and the content layer is a
// fixed standard-material band — and this test is the contract that keeps all
// three honest at *every* combination of the two controls, not just the
// default. It is the round's headline assertion.
//
// The model it evaluates is the shipped one: it reads the three steps out of
// base.css, the content band out of `--glass-content-alpha`, the near-solid
// top out of `--glass-solid-top`, and every text alpha out of the palette,
// then sweeps a 3 x 19 grid. Nothing here restates a number the stylesheet
// owns, so a mutation to any of them fails here.

/** The composited material stack, painted in order: backdrop → dimming layer →
 *  frame → content-layer standard material → control pane. */
const buildModel = (
  steps: Record<string, { fill: number; dim: number; blur: number }>,
  contentBand: { a: number; b: number },
  solidTop: number,
  paneAlpha: number,
) => {
  const frame = (step: { fill: number; dim: number }, t: number) => {
    const fill = step.fill + (solidTop - step.fill) * t;
    const dim = step.dim * (1 - t);
    return 1 - (1 - dim) * (1 - fill);
  };
  const content = (t: number) => Math.min(1, contentBand.a + contentBand.b * t);
  const over = (backdrop: number[], tint: number[], alpha: number) => flatten([...tint, alpha], backdrop);
  // The backdrop a thin blur cannot average: the brightest realistic desktop
  // for the dark theme, the darkest for the light one. A thicker blur pulls
  // it toward the mean, which is why HIG says thicker materials read better.
  const backdrop = (blur: number, extreme: number[]) => {
    const mean = [128, 110, 140];
    const f = Math.min(blur, 28) / 28;
    return extreme.map((c, i) => c + (mean[i] - c) * f);
  };
  return { frame, content, over, backdrop };
};

test("every step x transparency keeps the body-text floor (the R8 contract)", async () => {
  const rootBlock = await rootTokens();
  baseCache ??= await read("src/styles/base.css");
  const stepValues = { low: stepTokens("low"), mid: stepTokens("mid"), high: stepTokens("high") };
  const solidTop = Number(token(rootBlock, "glass-solid-top"));
  const content = token(rootBlock, "glass-content-alpha").match(
    /^calc\(\s*([\d.]+)\s*\+\s*([\d.]+)\s*\*\s*var\(--main-opacity\)\s*\)$/,
  );
  assert.ok(content, `--glass-content-alpha must be calc(a + b·transparency), got "${token(rootBlock, "glass-content-alpha")}"`);
  const band = { a: Number(content![1]), b: Number(content![2]) };
  const model = buildModel(stepValues, band, solidTop, 0.055);

  // A step in the ladder may be an opaque hex (the light warning colour is),
  // in which case its alpha is exactly 1.
  const alphaOf = (palette: string, name: string) => {
    const declaration = palette.match(new RegExp(`--${name}:\\s*([^;]+);`));
    assert.ok(declaration, `--${name} must be declared in the palette`);
    const rgba = declaration![1].match(/rgba\([^)]*,\s*([\d.]+)\)/);
    return rgba ? Number(rgba[1]) : 1;
  };
  const base = stripComments(baseCache!);
  const dark = base.slice(base.indexOf(":root {"), base.indexOf('[data-theme="light"]'));
  const light = base.slice(base.indexOf('[data-theme="light"]'), base.indexOf("html,\nbody,"));

  // Where each token is allowed to land. This is the honest map rather than
  // "every token on every surface": the frame carries only the two leading
  // steps (it is the functional layer, and its copy is titles and the query
  // field), the selection pane carries no tertiary/warning text in this app,
  // and everything else is a full body-copy surface.
  const LANDINGS: Record<string, string[]> = {
    frame: ["primary", "strong"],
    recess: ["primary", "strong", "secondary", "accent", "muted", "tertiary", "warning"],
    soft: ["primary", "strong", "secondary", "accent", "muted", "tertiary", "warning"],
    pane: ["primary", "strong", "secondary", "accent", "muted", "tertiary", "warning"],
    float: ["primary", "strong", "secondary", "accent", "muted", "tertiary", "warning"],
    track: ["primary", "strong", "secondary", "accent", "muted", "tertiary", "warning"],
    raised: ["primary", "strong", "secondary", "accent", "muted"],
  };
  const floors: Record<string, number> = {
    primary: 4.5, strong: 4.5, secondary: 4.5, accent: 4.5, warning: 3, muted: 3, tertiary: 3,
  };

  for (const [themeName, palette, extreme, frameTint, recessTint, paneTint, floatTint] of [
    ["dark", dark, [230, 220, 200], [18, 19, 22], [17, 18, 20], [255, 255, 255], [24, 25, 29]],
    ["light", light, [0, 0, 0], [250, 250, 252], [243, 244, 247], [0, 0, 0], [252, 252, 254]],
  ] as [string, string, number[], number[], number[], number[], number[]][]) {
    // The pane's polarity flips with the palette; read it from that palette's
    // own token block rather than from the dark one.
    const paneAlpha = themeName === "dark"
      ? Number(token(rootBlock, "glass-control").match(/rgba\(255,\s*255,\s*255,\s*([\d.]+)\)/)![1])
      : Number(palette.match(/--glass-control:\s*rgba\(0,\s*0,\s*0,\s*([\d.]+)\)/)![1]);
    const paneTintResolved = themeName === "dark" ? [255, 255, 255] : [0, 0, 0];
    const textAlphas: Record<string, number> = {
      primary: 1,
      strong: alphaOf(palette, "text-strong"),
      secondary: alphaOf(palette, "text-secondary"),
      muted: alphaOf(palette, "text-muted"),
      tertiary: alphaOf(palette, "text-tertiary"),
      warning: alphaOf(palette, "text-warning"),
      accent: 1,
    };
    const textRgb: Record<string, number[]> = {
      primary: themeName === "dark" ? [242, 243, 245] : [29, 29, 31],
      strong: themeName === "dark" ? [244, 245, 247] : [23, 23, 26],
      secondary: themeName === "dark" ? [238, 239, 242] : [60, 60, 67],
      muted: themeName === "dark" ? [238, 239, 242] : [60, 60, 67],
      tertiary: themeName === "dark" ? [238, 239, 242] : [60, 60, 67],
      warning: themeName === "dark" ? [255, 178, 164] : [156, 31, 24],
      accent: themeName === "dark"
        ? [143, 183, 255]
        : (() => {
            const hex = palette.match(/--accent:\s*#([0-9a-f]{6})/)![1];
            return [0, 2, 4].map((i) => parseInt(hex.slice(i, i + 2), 16));
          })(),
    };

    // The text ladder still has to *be* a ladder in this palette.
    assert.ok(
      textAlphas.strong > textAlphas.secondary &&
        textAlphas.secondary > textAlphas.muted &&
        textAlphas.muted >= textAlphas.tertiary,
      `${themeName}: the text ladder must stay ordered (strong > secondary > muted ≥ tertiary)`,
    );

    let cells = 0;
    for (const [stepName, step] of Object.entries(stepValues)) {
      for (let t = 0.1; t <= 1.0001; t += 0.05) {
        const frameAlpha = model.frame(step, t);
        const base = model.backdrop(step.blur, extreme);
        const frameSurface = model.over(base, frameTint, frameAlpha);
        const contentAlpha = model.content(t);
        const recessSurface = model.over(frameSurface, recessTint, contentAlpha);
        const softSurface = model.over(frameSurface, recessTint, Math.max(0.3, contentAlpha - 0.06));
        const paneSurface = model.over(recessSurface, paneTintResolved, paneAlpha);
        const trackSurface = model.over(recessSurface, paneTintResolved, themeName === "dark" ? 0.07 : 0.05);
        const floatSurface = model.over(frameSurface, floatTint, Math.min(1, contentAlpha + 0.06));
        const raisedTint = themeName === "dark" ? [143, 183, 255] : textRgb.accent;
        const raisedSurface = model.over(softSurface, raisedTint, themeName === "dark" ? 0.13 : 0.08);
        const surfaces: Record<string, number[]> = {
          frame: frameSurface, recess: recessSurface, soft: softSurface, pane: paneSurface,
          track: trackSurface, float: floatSurface, raised: raisedSurface,
        };
        for (const [surfaceName, names] of Object.entries(LANDINGS)) {
          for (const name of names) {
            const fg = flatten([...textRgb[name], textAlphas[name]], surfaces[surfaceName]);
            const ratio = contrastRatio(fg, surfaces[surfaceName]);
            cells += 1;
            assert.ok(
              ratio >= floors[name],
              `${themeName} ${stepName}@${t.toFixed(2)}: ${name} on the ${surfaceName} only reaches ` +
                `${ratio.toFixed(2)}:1 (floor ${floors[name]}:1). Do not lower the frame's fill floor, the ` +
                "content band or a text alpha without re-deriving this contract.",
            );
          }
        }
        // The recess must composite at least as densely as the frame it sits
        // on, or body copy would be *less* protected than the frame around it.
        const frameSolidity = 1 - (1 - frameAlpha) * (1 - contentAlpha);
        assert.ok(
          frameSolidity >= frameAlpha,
          `${themeName} ${stepName}@${t.toFixed(2)}: the recess must composite denser than the frame` +
            ` (${frameSolidity.toFixed(3)} vs ${frameAlpha.toFixed(3)})`,
        );
        // Layer discipline, stated as the thing it actually promises: the
        // content layer is a *standard material*, so its own fill never drops
        // into the Clear variant's band. A recess that tracked the frame down
        // to a 0.30 fill would put 10px body copy on a sheet as thin as the
        // Clear glass it is supposed to be structurally distinct from.
        assert.ok(
          contentAlpha >= 0.6,
          `${themeName} ${stepName}@${t.toFixed(2)}: the content layer must stay in the regular band ` +
            `(≥0.60), got ${contentAlpha.toFixed(3)} — content is standard material, not glass`,
        );
      }
    }
    assert.ok(cells >= 3 * 19 * 7, `${themeName}: the sweep must cover the full 3x19 grid`);
  }
});

// The user's headline number, asserted as a number: at 100% transparency the
// frame is near-opaque on every step, and the recess composites to essentially
// opaque. R7's map topped out at 0.73 and this is the regression that would
// reopen the complaint.
test("the maxed transparency slider is a near-opaque surface", async () => {
  const rootBlock = await rootTokens();
  baseCache ??= await read("src/styles/base.css");
  const solidTop = Number(token(rootBlock, "glass-solid-top"));
  const content = token(rootBlock, "glass-content-alpha").match(
    /^calc\(\s*([\d.]+)\s*\+\s*([\d.]+)\s*\*\s*var\(--main-opacity\)\s*\)$/,
  )!;
  const contentAtTop = Math.min(1, Number(content[1]) + Number(content[2]) * 1);
  for (const step of ["low", "mid", "high"]) {
    const values = stepTokens(step);
    const fill = values.fill + (solidTop - values.fill) * 1;
    const dim = values.dim * (1 - 1);
    const frame = 1 - (1 - dim) * (1 - fill);
    assert.ok(frame >= 0.95, `${step}: at 100% the frame must be ≥0.95 opaque, got ${frame.toFixed(3)}`);
    const composite = 1 - (1 - frame) * (1 - contentAtTop);
    assert.ok(
      composite >= 0.99,
      `${step}: at 100% the frame + content recess must composite to ≥0.99, got ${composite.toFixed(4)}`,
    );
  }
});

test("prefers-contrast: more restores an opaque shell, not just a border", async () => {
  const base = stripComments(await read("src/styles/base.css"));
  const block = mediaBlock(base, "(prefers-contrast: more)");
  // The system's "increase contrast" signal has to do more than darken a
  // border: at the low end of the transparency slider the frame is genuinely
  // see-through, and a user who asked for contrast must not have to also find
  // the slider. R8 expresses it against the same two tokens the steps and the
  // slider use — the step's fill floor and dimming layer — so it is testable
  // rather than a parallel formula.
  assert.match(block, /--glass-step-fill:\s*[\d.]+/, "prefers-contrast: more must raise the step's fill floor");
  assert.match(block, /--glass-step-dim:\s*0\b/, "prefers-contrast: more must drop the Clear dimming layer");
  const raised = [...block.matchAll(/--glass-step-fill:\s*([\d.]+)/g)].map((m) => Number(m[1]));
  assert.ok(raised.length >= 2, "the override must cover both the :root default and the step attribute selectors");
  for (const value of raised) {
    assert.ok(value >= 0.8, `the contrast fill floor must be high, got ${value}`);
  }
  // …and it has to land on every step, or the Clear variant would stay thin
  // under an accessibility setting that exists to stop exactly that.
  for (const step of ["low", "mid", "high"]) {
    assert.match(block, new RegExp(`\\[data-glass="${step}"\\]`), `the contrast override must cover the ${step} step`);
  }
});

// The field is the one control that sits *below* the surface rather than on
// it: a text input has to read as a slot cut into the glass, which is what the
// thinner fill plus a 1px inner shadow buys. The rule is asserted here because
// the shape is easy to lose — a later round that "simplifies" fields back onto
// the resting-pane token would make every input look like a button.
test("text fields are recessed, not raised", async () => {
  const rootBlock = await rootTokens();
  const field = Number(token(rootBlock, "glass-field").match(/rgba\(255,\s*255,\s*255,\s*([\d.]+)\)/)![1]);
  const pane = Number(token(rootBlock, "glass-control").match(/rgba\(255,\s*255,\s*255,\s*([\d.]+)\)/)![1]);
  assert.ok(field < pane, `a field must be thinner than a resting pane (${field} < ${pane})`);
  const shadow = token(rootBlock, "glass-field-shadow");
  assert.match(shadow, /inset 0 1px 2px/, "a field must carry a 1px inner shadow (the slot)");
  assert.match(shadow, /inset 0 1px 0 var\(--glass-lens-rim\)/, "a field keeps the lens rim");
  for (const [file, selector] of [
    ["extensions.css", ".extension-config-field input"],
    ["extensions.css", ".extension-custom-form textarea"],
  ] as const) {
    const css = stripComments(await read(`src/styles/${file}`));
    const rule = rules(css).find((r) => r.selector.split(",").map((s) => s.trim()).includes(selector));
    assert.ok(rule, `${file}: ${selector} must exist`);
    assert.match(rule!.body, /background:\s*var\(--glass-field\)/, `${file}: ${selector} must use the field token`);
    assert.match(rule!.body, /box-shadow:\s*var\(--glass-field-shadow\)/, `${file}: ${selector} must carry the slot shadow`);
  }
});

// Every interactive control answers the pointer *down* with the thinner press
// fill. That inversion (press < rest) is the whole "sinks into the glass" cue,
// and it is the kind of thing a later round drops one rule at a time — so the
// scan is structural: any `:active` rule that paints a background has to use
// the press token (or explicitly clear the fill), and any `:hover` rule that
// paints one has to come from the ladder.
test("every pressed control uses the press token, never a brighter fill", async () => {
  const pressUsers = new Set<string>();
  for (const { name, css } of await styleFiles()) {
    for (const { selector, body } of rules(css)) {
      if (!/:active/.test(selector)) continue;
      const backgrounds = declarations(body, "background").concat(declarations(body, "background-color"));
      if (backgrounds.length === 0) continue;
      for (const value of backgrounds) {
        assert.ok(
          /var\(--glass-control-press\)|transparent|none|inherit/.test(value),
          `${name}: ${selector} paints "${value}" while pressed — a pressed control thins into the ` +
            "glass (var(--glass-control-press)) instead of brightening",
        );
      }
      if (backgrounds.some((v) => /var\(--glass-control-press\)/.test(v))) pressUsers.add(name);
    }
  }
  // The cue is only real if the surfaces actually implement it.
  for (const file of ["launcher.css", "settings.css", "terminal.css", "extensions.css"]) {
    assert.ok(
      [...pressUsers].some((name) => name.endsWith(file)),
      `${file} has no pressed control using the press token`,
    );
  }
});

test("no hover rule paints a colour outside the material ladder", async () => {
  // The ladder tokens, plus the handful of names that legitimately stand for a
  // *branch* of it: the solid accent a primary button is filled with, the
  // scrollbar thumb, and the extensions panel's two aliases (asserted to point
  // into the ladder further down). `--accent-tint` and friends are deliberately
  // absent: they are the old flat selection fill, and a hover that reaches for
  // one is exactly the regression this test is for.
  const allowed =
    /var\(--(glass-control|glass-control-hover|glass-control-press|glass-raised|glass-raised-hover|glass-raised-warm|glass-raised-warm-hover|accent|scrollbar-thumb|ext-bg-hover|ext-bg-selected)\)/;
  for (const { name, css } of await styleFiles()) {
    for (const { selector, body } of rules(css)) {
      if (!/:hover/.test(selector)) continue;
      for (const value of declarations(body, "background").concat(declarations(body, "background-color"))) {
        if (/^(transparent|none|inherit|currentColor)$/.test(value)) continue;
        assert.match(
          value,
          allowed,
          `${name}: ${selector} hover paints "${value}" — hover states come from the glass ladder, ` +
            "not from a one-off colour",
        );
      }
    }
  }
  // The two aliases really do point into the ladder, which is what makes
  // allowing them above honest rather than a loophole.
  const ext = stripComments(await read("src/styles/extensions.css"));
  const panel = rules(ext).find(({ selector }) => selector === ".extensions-panel")!;
  assert.match(panel.body, /--ext-bg-hover:\s*var\(--surface-control-hover\)/);
  assert.match(panel.body, /--ext-bg-selected:\s*var\(--accent-tint-hover\)/);
});

// The light palette has to clear the same bars as the dark one. It is easy to
// forget: the dark theme's numbers were the ones in the report, and a light
// surface over a dark desktop is the mirror image of the dark theme over a
// bright one. The steps are asserted against the *sunken* field (where body
// copy lives) over a pure-black desktop — the light theme's worst case.
test("the light palette's muted steps clear 3:1 over a dark desktop", async () => {
  const base = stripComments(await read("src/styles/base.css"));
  const light = base.slice(base.indexOf('[data-theme="light"]'), base.indexOf("html,\nbody,"));
  const alphaOf = (name: string) => {
    const match = light.match(new RegExp(`--${name}:\\s*rgba\\([^)]*,\\s*([\\d.]+)\\)`));
    assert.ok(match, `--${name} must be a translucent rgba in the light palette`);
    return Number(match![1]);
  };
  // The material stack under the light body copy, read from the shipped tokens
  // rather than copied from the pre-R8 map: the frame is the mid step's floor
  // sliding to `--glass-solid-top` on the default transparency, and the sunken
  // field is the content band (`--glass-content-alpha`). The old
  // `0.18 + 0.55·0.94` / `·0.9` pair no longer exists in base.css, so keeping
  // it here would have tested a model the app no longer renders.
  const tokens = await rootTokens();
  const main = Number(token(tokens, "main-opacity"));
  const shellAlpha = stepTokens("mid").fill + (SOLID_TOP - stepTokens("mid").fill) * main;
  const contentMap = token(tokens, "glass-content-alpha").match(
    /^calc\(\s*([\d.]+)\s*\+\s*([\d.]+)\s*\*\s*var\(--main-opacity\)\s*\)$/,
  );
  assert.ok(contentMap, "--glass-content-alpha must be calc(a + b·transparency)");
  const contentAlpha = Number(contentMap![1]) + Number(contentMap![2]) * main;
  const shell = flatten([250, 250, 252, shellAlpha], [0, 0, 0]);
  const sunken = flatten([243, 244, 247, contentAlpha], shell);

  const floors: [string, number][] = [
    ["text-secondary", 4.5],
    ["text-muted", 3],
    ["text-tertiary", 3],
  ];
  for (const [name, floor] of floors) {
    const fg = flatten([60, 60, 67, alphaOf(name)], sunken);
    const ratio = contrastRatio(fg, sunken);
    assert.ok(
      ratio >= floor,
      `light ${name} only reaches ${ratio.toFixed(2)}:1 on the sunken field over a black desktop ` +
        `(floor ${floor}:1)`,
    );
  }
  // The light ladder keeps the same ordering as the dark one.
  assert.ok(alphaOf("text-secondary") > alphaOf("text-muted"), "light: secondary above muted");
  assert.ok(alphaOf("text-muted") > alphaOf("text-tertiary"), "light: muted above tertiary");
});
