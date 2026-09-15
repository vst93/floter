// R7-GLASS: the app has exactly one sheet of glass per surface. Before this
// round every surface carried its own blur and so did the clipboard panel's
// host material, and the "transparency" slider wrote a raw alpha into
// `--input-bg`; now the shell is the only element that filters the desktop,
// everything inside it floats on plain tint, and the slider drives a glass
// formula instead of an alpha. These assertions keep that shape from drifting:
// the element count stays at one per surface, the derived tint keeps coming
// from the untouched `--main-opacity`/`--terminal-opacity` chain, the values
// stay inside the performance budget, and no transition ever animates a filter.
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

const styleFiles = async () => {
  const dir = new URL("src/styles/", root);
  const names = (await readdir(dir)).filter((n) => n.endsWith(".css"));
  return Promise.all(
    names.map(async (name) => ({ name, css: stripComments(await read(`src/styles/${name}`)) })),
  );
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

test("the glass tint is derived from the untouched opacity chain", async () => {
  const base = stripComments(await read("src/styles/base.css"));
  const rootBlock = base.slice(base.indexOf(":root {"), base.indexOf("[data-theme="));
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

test("glass values stay inside the performance budget", async () => {
  const base = stripComments(await read("src/styles/base.css"));
  const rootBlock = base.slice(base.indexOf(":root {"), base.indexOf("[data-theme="));
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
  assert.ok(Number(saturate[1]) <= 160, `saturate ${saturate![1]}% exceeds the 160% budget`);
  // The quantized variant is what keeps a slider drag from re-rasterizing on
  // every pixel of travel; an engine without round() keeps the calc fallback.
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
  // The focused card's own rule is more specific than the plain selector, so
  // without this the launcher the user is typing into would keep its glass
  // while every other shell went opaque. The assertion is only meaningful if
  // that more-specific rule really exists.
  const launcher = (await styleFiles()).find((f) => f.name === "launcher.css")!;
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
