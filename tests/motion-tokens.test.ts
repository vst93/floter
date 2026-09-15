// R7-1: motion and duration are a global system, not a per-rule choice. Before
// this round the stylesheet carried 15 raw `cubic-bezier` literals in 5 distinct
// curves and 8 distinct millisecond durations, and the reduced-motion block
// missed six running animations. These assertions keep the convergence from
// drifting back: every finite duration has to resolve to a `--dur-*` token, the
// only literal curves are the token definitions themselves, and each animation
// that can run is neutralized under `prefers-reduced-motion`.
//
// Slices are taken by structure (the media block's own braces, the token block's
// own braces), never by a fixed-width window around a match — a window silently
// becomes vacuous when the file around it moves.
import assert from "node:assert/strict";
import { readdir, readFile } from "node:fs/promises";
import test from "node:test";

const root = new URL("../", import.meta.url);
const read = (path: string) => readFile(new URL(path, root), "utf8");
const stylesDir = new URL("src/styles/", root);

// Every CSS file the app ships. R7-1's convergence covers `src/styles/*.css`
// (the four surfaces' shared sheet). Two files are deliberately outside this
// round's file boundary and are listed here so the exception is visible rather
// than a silent gap: the uninstall dialog's own sheet, and the clipboard
// plugin page, whose token pass belongs to R7-4/R7-6.
const OUT_OF_SCOPE = [
  "src/extensions/ComponentizedUninstallDialog.css",
  "src/plugins/clipboard/page.css",
];

// Animations that must not run under `prefers-reduced-motion` (G-12). The
// spinner's `@keyframes` is a dead reference today, but it is listed because the
// declaration exists and would animate the moment the keyframes land.
const REDUCED_MOTION_ANIMATIONS = [
  "app-toast-in",
  "feedback-enter",
  "settings-page-enter",
  "extensions-spin",
  "shortcut-recording",
  "terminal-bar-dot",
];

// A `transition` value is a comma-separated list, and each item is judged on
// its own. `visibility 0s` is an immediate, non-animated swap of a discrete
// property — the one legitimate zero — but a finite duration sitting next to it
// in the same declaration is still a raw value that has to be tokenized.
// Matching the whole value for `0s` would skip exactly that mix, which is why
// the split happens before the test, not after it. Infinite loops are periodic,
// not entrances, and their periods (0.8s / 1.4s / 2.4s) are deliberately not on
// the token scale.
const motionItems = (value: string) => value.split(",").map((item) => item.trim()).filter(Boolean);

const itemIsImmediate = (item: string) =>
  /\binfinite\b/.test(item) || timeLiterals(item).every((literal) => literal === "0s");

// Every bare finite duration in a declaration, item by item. Exported to the
// test below that pins the mixed-declaration case.
const rawFiniteDurations = (value: string) =>
  motionItems(value)
    .filter((item) => !itemIsImmediate(item))
    .flatMap((item) => timeLiterals(item));

const stripComments = (css: string) => css.replace(/\/\*[\s\S]*?\*\//g, "");

const timeLiterals = (value: string) =>
  [...value.matchAll(/(?<![\w.-])(\d*\.?\d+)(ms|s)\b/g)].map((m) => m[0]);

const styleFiles = async () => {
  const names = (await readdir(stylesDir)).filter((n) => n.endsWith(".css"));
  return Promise.all(
    names.map(async (name) => ({ name, css: stripComments(await read(`src/styles/${name}`)) })),
  );
};

// The out-of-scope files are the whole exception, and the exception is the
// whole list: any other CSS file in `src/` is either a surface sheet (checked
// by the tests below) or an unnoticed new one. `ComponentizedUninstallDialog`
// is a dialog's own sheet and `clipboard/page.css` is a plugin page, whose
// token pass belongs to R7-4/R7-6 — both keep raw values this round on purpose.
test("the token convergence's file boundary is exactly the declared set", async () => {
  const all: string[] = [];
  const walk = async (dir: URL, prefix: string) => {
    for (const entry of await readdir(dir, { withFileTypes: true })) {
      if (entry.isDirectory()) await walk(new URL(`${entry.name}/`, dir), `${prefix}${entry.name}/`);
      else if (entry.name.endsWith(".css")) all.push(`${prefix}${entry.name}`);
    }
  };
  await walk(new URL("src/", root), "src/");
  const styles = (await readdir(stylesDir)).map((n) => `src/styles/${n}`);
  const outside = all.filter((p) => !styles.includes(p)).sort();
  assert.deepEqual(
    outside,
    [...OUT_OF_SCOPE].sort(),
    "a CSS file outside src/styles must be added to OUT_OF_SCOPE with its round, or converged",
  );
  // Both exceptions are genuinely unconverged, which is why they are listed.
  for (const path of OUT_OF_SCOPE) {
    const css = stripComments(await read(path));
    assert.ok(
      timeLiterals(css).length > 0 || css.includes("cubic-bezier"),
      `${path} is listed as out of scope but carries no raw motion value — converge it and drop the exception`,
    );
  }
});

// Returns every `transition` / `animation` declaration value with its file and
// the property it came from. `[^;}]+` deliberately spans newlines so a wrapped
// multi-property transition is read as one declaration.
const motionDeclarations = (name: string, css: string) => {
  const out: { file: string; property: string; value: string }[] = [];
  for (const match of css.matchAll(/(transition|animation)\s*:\s*([^;}]+)/g)) {
    out.push({ file: name, property: match[1], value: match[2].trim() });
  }
  return out;
};

test("the motion token scale is defined exactly once, in base.css", async () => {
  const base = stripComments(await read("src/styles/base.css"));
  const rootBlock = base.slice(base.indexOf(":root {"), base.indexOf("[data-theme="));
  for (const [token, value] of [
    ["--dur-1", "80ms"],
    ["--dur-2", "120ms"],
    ["--dur-3", "160ms"],
    ["--dur-4", "220ms"],
    ["--spring", "cubic-bezier(0.34, 1.56, 0.64, 1)"],
    ["--ease-out-back", "cubic-bezier(0.34, 1.56, 0.64, 1)"],
    ["--ease-in-out", "cubic-bezier(0.65, 0, 0.35, 1)"],
    ["--ease-out", "cubic-bezier(0.23, 1, 0.32, 1)"],
  ]) {
    assert.match(
      rootBlock,
      new RegExp(`${token}:\\s*${value.replace(/[.()]/g, "\\$&")}\\s*;`),
      `${token} must be defined in base.css :root as ${value}`,
    );
  }
});

test("every finite transition/animation duration resolves to a --dur-* token", async () => {
  for (const { name, css } of await styleFiles()) {
    for (const { property, value } of motionDeclarations(name, css)) {
      const literals = rawFiniteDurations(value);
      assert.deepEqual(
        literals,
        [],
        `${name}: ${property}: ${value} — a finite duration must use var(--dur-*), ` +
          `found raw ${literals.join(", ")}`,
      );
    }
  }
});

// The mutation this pins: reverting the item-level judgement to a whole-value
// `0s` match makes the second assertion pass instead of fail, because the raw
// `120ms` in the first item would never be looked at.
test("a declaration mixing a token duration with a 0s item still reports the raw one", () => {
  assert.deepEqual(
    rawFiniteDurations("transform var(--dur-2) var(--ease-out), visibility 0s"),
    [],
    "a fully tokenized declaration reports nothing",
  );
  assert.deepEqual(
    rawFiniteDurations("transform 120ms var(--ease-out), visibility 0s"),
    ["120ms"],
    "the finite item must be judged even though a 0s item shares the declaration",
  );
});

test("cubic-bezier literals exist only as base.css token definitions", async () => {
  for (const { name, css } of await styleFiles()) {
    for (const match of css.matchAll(/cubic-bezier\([^)]*\)/g)) {
      if (name !== "base.css") {
        assert.fail(`${name} carries a raw curve ${match[0]}; use var(--ease-*)`);
      }
      const lineStart = css.lastIndexOf("\n", match.index) + 1;
      const line = css.slice(lineStart, css.indexOf("\n", match.index));
      assert.match(line, /^\s*--[\w-]+:\s*cubic-bezier/, `${match[0]} must be a token definition`);
    }
  }
});

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

// Every selector that starts one of the listed animations, from every style
// file. The reduced-motion block has to name each one with `animation: none`.
const selectorsFor = (files: { name: string; css: string }[], animation: string) => {
  const found = new Set<string>();
  for (const { css } of files) {
    for (const rule of css.matchAll(/([^{}]+)\{([^{}]*)\}/g)) {
      const body = rule[2];
      const declaration = body.match(/(?:^|;)\s*animation\s*:\s*([^;}]+)/);
      if (!declaration) continue;
      const value = declaration[1].trim();
      if (value === "none") continue;
      if (!new RegExp(`(?:^|\\s)${animation}(?:\\s|$)`).test(value)) continue;
      for (const selector of rule[1].split(",")) found.add(selector.trim());
    }
  }
  return found;
};

// Selectors inside the reduced-motion block whose own declaration is
// `animation: none` (a `transition: none` list does not stop an animation).
const neutralizedSelectors = (block: string) => {
  const found = new Set<string>();
  for (const rule of block.matchAll(/([^{}]+)\{([^{}]*)\}/g)) {
    if (!/(?:^|;)\s*animation\s*:\s*none/.test(rule[2])) continue;
    for (const selector of rule[1].split(",")) found.add(selector.trim());
  }
  return found;
};

test("prefers-reduced-motion neutralizes every running animation (G-12)", async () => {
  const files = await styleFiles();
  const block = mediaBlock(
    files.find((f) => f.name === "base.css")!.css,
    "(prefers-reduced-motion: reduce)",
  );
  const neutralized = neutralizedSelectors(block);
  for (const animation of REDUCED_MOTION_ANIMATIONS) {
    const selectors = selectorsFor(files, animation);
    assert.ok(selectors.size > 0, `no selector starts ${animation}`);
    for (const selector of selectors) {
      assert.ok(
        neutralized.has(selector),
        `prefers-reduced-motion must set animation: none on ${selector} (${animation})`,
      );
    }
  }
});

test("all four surfaces share one shell enter, at --dur-4", async () => {
  const base = stripComments(await read("src/styles/base.css"));
  // `.collapsed-shell` (S1), `.terminal-shell` (S2 + S4 plugin page) and
  // `.settings-shell` (S3) share this one declaration; the plugin page rides
  // the terminal shell, so the four surfaces move as one.
  const rule = base.match(
    /\.collapsed-shell,\s*\.terminal-shell,\s*\.settings-shell\s*\{([^}]*)\}/,
  );
  assert.ok(rule, "the three shell selectors must share one rule");
  assert.match(rule[1], /animation:\s*shell-enter var\(--dur-4\) var\(--spring\)/);
  // The WebKitGTK jitter guard: these two lines are why the shell rule exists
  // as written and must survive any motion refactor.
  assert.match(rule[1], /will-change:\s*opacity, transform/);
  assert.match(rule[1], /transform-origin:\s*center top/);
});
