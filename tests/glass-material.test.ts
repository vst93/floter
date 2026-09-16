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
// plugin page (its own document and sheet) is deliberately outside this round
// and keeps its own single blur, listed here so the exception is visible.
const GLASS_SHELLS = [".collapsed-card", ".settings-card", ".terminal-panel"];

// Host sheets. `src/plugins/clipboard/page.css` is the plugin page's own
// document and belongs to R7-4/R7-6; it is the one sheet the control scan
// below deliberately does not read (the per-surface test still counts its
// single blur as the fourth surface's shell).
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
const affine = (value: string, variable: string) => {
  const match = value.match(
    new RegExp(`^calc\\(\\s*([\\d.]+)\\s*\\+\\s*([\\d.]+)\\s*\\*\\s*var\\(--${variable}\\)\\s*\\)$`),
  );
  assert.ok(match, `"${value}" must be calc(a + b * var(--${variable}))`);
  return { a: Number(match![1]), b: Number(match![2]) };
};

// The alpha a `--x-alpha` token resolves to, evaluated with the affine map it
// is written against.
const alphaAt = (map: { a: number; b: number }, opacity: number) => map.a + map.b * opacity;

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

test("the glass tint is derived from the untouched opacity chain", async () => {
  const rootBlock = await rootTokens();
  // The chain itself is unchanged: both variables are still declared here and
  // still written by App.tsx.
  assert.match(rootBlock, /--main-opacity:\s*0\.94;/);
  assert.match(rootBlock, /--terminal-opacity:\s*0\.92;/);
  assert.match(await read("src/App.tsx"), /setProperty\("--main-opacity"/);
  assert.match(await read("src/App.tsx"), /setProperty\("--terminal-opacity"/);
  // The glass formula consumes them, and only with calc + rgba (no color-mix,
  // whose WebKitGTK support varies across the versions floter ships to).
  const glassTokens = rootBlock.slice(rootBlock.indexOf("--glass-tint-alpha"));
  assert.match(glassTokens, /--glass-tint-alpha:\s*calc\([^)]*var\(--main-opacity\)\)/);
  assert.match(glassTokens, /--glass-tint-alpha-terminal:\s*calc\([^)]*var\(--terminal-opacity\)\)/);
  assert.ok(
    !/color-mix/.test(glassTokens.slice(0, glassTokens.indexOf("--input-stroke"))),
    "the derived glass tint must not use color-mix",
  );
});

// The coefficients themselves. R7-GLASS shipped (0.25 + 0.72·op); R7-GLASS-DEEP
// replaced it with (0.18 + 0.55·op) because the old map put the default at
// 0.927 and the panel stopped reading as glass. A mutation that restores the
// old slope has to fail here, which is the only place the number lives.
test("the tint map is the R7-GLASS-DEEP curve, not the old one", async () => {
  const rootBlock = await rootTokens();
  const main = affine(token(rootBlock, "glass-tint-alpha"), "main-opacity");
  assert.deepEqual(main, { a: 0.18, b: 0.55 }, "the shell tint map must be 0.18 + 0.55·op");

  const terminal = affine(token(rootBlock, "glass-tint-alpha-terminal"), "terminal-opacity");
  assert.equal(terminal.b, 0.55, "the terminal shares the shell's slope");
  assert.ok(terminal.a > main.a, "the terminal keeps a higher floor than the launcher");

  // The two ends the user actually meets: the default 94 must let a third of
  // the desktop through (it was 0.927), and the thinnest slider setting must
  // still be a surface rather than a hole.
  const atDefault = alphaAt(main, 0.94);
  assert.ok(atDefault <= 0.72, `default strength must tint at most 0.72, got ${atDefault}`);
  assert.ok(atDefault >= 0.6, `default strength must still tint at least 0.6, got ${atDefault}`);
  const atFloor = alphaAt(main, 0.1);
  assert.ok(atFloor >= 0.2, `the thinnest setting must stay opaque enough to read, got ${atFloor}`);

  // blur still moves the other way, so a thinner tint keeps its readability.
  const blur = token(rootBlock, "glass-blur");
  assert.match(blur, /^calc\(22px \+ 6px \* \(1 - var\(--main-opacity\)\)\)$/);
});

// The material ladder. Every control token has to keep its place relative to
// the shell at *every* slider position, not just at the default, which is why
// the checks below sweep the range instead of spot-checking 0.94.
test("float, shell and sunken keep their order across the whole slider", async () => {
  const rootBlock = await rootTokens();
  const shell = affine(token(rootBlock, "glass-tint-alpha"), "main-opacity");

  // Float is written as shell + a constant, so the ordering is structural
  // rather than two formulas that happen to agree at one value.
  const float = token(rootBlock, "glass-float-alpha");
  const delta = float.match(/^calc\(var\(--glass-tint-alpha\)\s*\+\s*([\d.]+)\)$/);
  assert.ok(delta, `--glass-float-alpha must be shell + a constant, got "${float}"`);
  assert.ok(Number(delta![1]) >= 0.15, `float must clear the shell by ≥0.15, got ${delta![1]}`);

  const sunken = token(rootBlock, "surface-sunken").match(/var\(--glass-tint-alpha\)\s*\*\s*([\d.]+)\)/);
  const sunkenSoft = token(rootBlock, "surface-sunken-soft").match(/var\(--glass-tint-alpha\)\s*\*\s*([\d.]+)\)/);
  assert.ok(sunken, "--surface-sunken must be a fraction of the shell tint");
  assert.ok(sunkenSoft, "--surface-sunken-soft must be a fraction of the shell tint");
  assert.ok(Number(sunken![1]) < 1, `sunken must be thinner than the shell, got ${sunken![1]}`);
  assert.ok(
    Number(sunkenSoft![1]) < Number(sunken![1]),
    "the soft recess must be thinner than the hard one",
  );

  // Sweep: for every strength the panel can be set to, the four layers stay
  // distinct and correctly ordered.
  for (let opacity = 0.1; opacity <= 1.0001; opacity += 0.05) {
    const shellAlpha = alphaAt(shell, opacity);
    const floatAlpha = shellAlpha + Number(delta![1]);
    const sunkenAlpha = shellAlpha * Number(sunken![1]);
    assert.ok(
      floatAlpha > shellAlpha && shellAlpha > sunkenAlpha,
      `ladder inverted at opacity ${opacity.toFixed(2)}: float ${floatAlpha} / shell ${shellAlpha} / sunken ${sunkenAlpha}`,
    );
    assert.ok(floatAlpha <= 1, `float must stay within [0,1] at opacity ${opacity.toFixed(2)}`);
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
  // same rim: selection is a lit pane, not a flat block of accent colour.
  assert.equal(token(rootBlock, "glass-raised"), "var(--accent-tint)");
  assert.match(token(rootBlock, "glass-raised-shadow"), /^inset 0 1px 0 var\(--glass-raised-rim\)$/);
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
  const blurDefinitions = [...rootBlock.matchAll(/--glass-blur[\w-]*:\s*([^;]+);/g)].map((m) => m[1]);
  assert.ok(blurDefinitions.length >= 2, "both the smooth and the quantized blur must be defined");
  for (const definition of blurDefinitions) {
    for (const literal of definition.matchAll(/([\d.]+)px/g)) {
      assert.ok(
        Number(literal[1]) <= 28,
        `blur component ${literal[0]} exceeds the 28px budget in "${definition}"`,
      );
    }
  }
  const saturate = rootBlock.match(/--glass-saturate:\s*([\d.]+)%/);
  assert.ok(saturate, "--glass-saturate must be defined");
  assert.ok(Number(saturate[1]) <= 180, `saturate ${saturate![1]}% exceeds the 180% budget`);
  assert.ok(Number(saturate[1]) >= 160, "the round raised saturation on purpose; it must not drift back");
  // The quantized variant is what keeps a slider drag from re-rasterizing on
  // every pixel of travel; an engine without round() keeps the calc fallback.
  const base = stripComments(await read("src/styles/base.css"));
  assert.match(base, /@supports \(width: round\(nearest, 1px, 1px\)\)/);
  assert.match(rootBlock, /--glass-blur:\s*calc\(/);
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

test("the plugin page keeps its own single blur, and the host does not double it", async () => {
  const page = stripComments(await read("src/plugins/clipboard/page.css"));
  const carriers = rules(page).filter(({ body }) => /(?:^|;)\s*backdrop-filter\s*:/.test(body));
  assert.equal(carriers.length, 1, "the plugin page's sheet owns exactly one blur");
  assert.match(carriers[0].selector, /\.clipboard-panel/);
  // The host-side material for the same surface is a tint only — the blur lives
  // on the panel shell underneath, so the two never stack.
  const host = stripComments(await read("src/styles/terminal.css"));
  const hostPanel = rules(host).find(({ selector }) => selector === ".clipboard-panel");
  assert.ok(hostPanel, "the host still defines .clipboard-panel's material");
  assert.ok(
    !/backdrop-filter/.test(hostPanel!.body),
    "the host .clipboard-panel must not add a blur of its own",
  );
  assert.match(hostPanel!.body, /var\(--glass-tint-terminal\)/);
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

test("the muted text steps clear 3:1 on the default glass", async () => {
  const rootBlock = await rootTokens();
  const shellMap = affine(token(rootBlock, "glass-tint-alpha"), "main-opacity");
  const shellAlpha = alphaAt(shellMap, 0.94);
  const sunkenFactor = Number(
    token(rootBlock, "surface-sunken").match(/var\(--glass-tint-alpha\)\s*\*\s*([\d.]+)\)/)![1],
  );
  const softFactor = Number(
    token(rootBlock, "surface-sunken-soft").match(/var\(--glass-tint-alpha\)\s*\*\s*([\d.]+)\)/)![1],
  );

  // Paint order matters: each layer is a tint *over the one below it*, so the
  // sunken field is flattened onto the shell, and the shell onto the desktop.
  const shellOver = (backdrop: number[]) => flatten([18, 19, 22, shellAlpha], backdrop);
  const sunkenOver = (backdrop: number[]) =>
    flatten([17, 18, 20, shellAlpha * sunkenFactor], shellOver(backdrop));
  const softOver = (backdrop: number[]) =>
    flatten([17, 18, 20, shellAlpha * softFactor], shellOver(backdrop));
  const controlOver = (backdrop: number[]) => flatten([255, 255, 255, 0.055], sunkenOver(backdrop));
  const raisedOver = (backdrop: number[]) => flatten([143, 183, 255, 0.13], sunkenOver(backdrop));

  // The alphas themselves, straight from the token block.
  const alphaOf = (name: string) => {
    const value = token(rootBlock, name);
    const match = value.match(/rgba\([^)]*,\s*([\d.]+)\)/);
    return match ? Number(match![1]) : 1;
  };
  const alphas: Record<string, number> = {
    "text-strong": alphaOf("text-strong"),
    "text-secondary": alphaOf("text-secondary"),
    "text-muted": alphaOf("text-muted"),
    "text-tertiary": alphaOf("text-tertiary"),
    "text-warning": alphaOf("text-warning"),
    "text-primary": 1,
    accent: 1,
  };

  // The ladder of text weights has to stay a ladder: primary > strong >
  // secondary > muted > tertiary, so nothing collapses two steps into one.
  const ordered = ["text-strong", "text-secondary", "text-muted", "text-tertiary"];
  for (let i = 1; i < ordered.length; i += 1) {
    assert.ok(
      alphas[ordered[i - 1]] > alphas[ordered[i]],
      `${ordered[i - 1]} (${alphas[ordered[i - 1]]}) must stay above ${ordered[i]} (${alphas[ordered[i]]})`,
    );
  }

  const ratioOn = (name: string, surface: number[]) => {
    const fg = flatten([...TEXT_RGB[name as keyof typeof TEXT_RGB], alphas[name]], surface);
    return contrastRatio(fg, surface);
  };

  // Where body copy actually lives: the sunken result field and the settings
  // body, both at the bright-wallpaper case. 4.5:1 for the two leading steps
  // and the accent, 3:1 for the two trailing ones and the warning.
  const bodyFloors: [string, number][] = [
    ["text-primary", 4.5],
    ["text-strong", 4.5],
    ["text-secondary", 4.5],
    ["accent", 4.5],
    ["text-muted", 3],
    ["text-tertiary", 3],
    ["text-warning", 3],
  ];
  for (const [name, floor] of bodyFloors) {
    for (const [surfaceName, surface] of [
      ["sunken result field", sunkenOver(BRIGHT_WALLPAPER)],
      ["settings body", softOver(BRIGHT_WALLPAPER)],
      ["control pane", controlOver(BRIGHT_WALLPAPER)],
      ["raised pane", raisedOver(BRIGHT_WALLPAPER)],
    ] as [string, number[]][]) {
      const ratio = ratioOn(name, surface);
      assert.ok(
        ratio >= floor,
        `${name} only reaches ${ratio.toFixed(2)}:1 on the ${surfaceName} over a bright desktop ` +
          `(floor ${floor}:1); raise its alpha in base.css rather than lowering the glass`,
      );
    }
  }

  // The header band is the one place secondary/tertiary text sits directly on
  // the shell. It is asserted against the *mean* desktop, which is what the
  // header is for; the pure-white extreme is reported, not asserted, because
  // the user owns that tradeoff with the slider.
  for (const [name, floor] of [["text-strong", 4.5], ["text-secondary", 3], ["text-tertiary", 3]] as [string, number][]) {
    const ratio = ratioOn(name, shellOver(GRADIENT_MEAN));
    assert.ok(ratio >= floor, `${name} only reaches ${ratio.toFixed(2)}:1 on the shell over a typical desktop (floor ${floor}:1)`);
  }
});

test("prefers-contrast: more restores an opaque shell, not just a border", async () => {
  const base = stripComments(await read("src/styles/base.css"));
  const block = mediaBlock(base, "(prefers-contrast: more)");
  // The system's "increase contrast" signal has to do more than darken a
  // border: at the thinnest slider setting the shell is genuinely see-through,
  // and a user who asked for contrast must not have to also find the slider.
  assert.match(
    block,
    /--glass-tint-alpha:\s*calc\([^)]*\)|--glass-tint-alpha:\s*[\d.]+/,
    "prefers-contrast: more must raise the shell tint",
  );
  const raised = block.match(/--glass-tint-alpha:\s*calc\(\s*([\d.]+)\s*\+\s*([\d.]+)\s*\*\s*var\(--main-opacity\)\s*\)/);
  assert.ok(raised, "the contrast override must stay an affine map of the slider");
  assert.ok(Number(raised![1]) >= 0.7, `the contrast floor must be high, got ${raised![1]}`);
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
  assert.match(shadow, /inset 0 1px 0 var\(--glass-control-rim\)/, "a field keeps the shared rim");
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
  const shellAlpha = 0.18 + 0.55 * 0.94;
  const shell = flatten([250, 250, 252, shellAlpha], [0, 0, 0]);
  const sunken = flatten([243, 244, 247, shellAlpha * 0.9], shell);

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
