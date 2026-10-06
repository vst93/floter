// HIG-2 · the accessibility backstops: increase-contrast and reduce-motion —
// plus the R104 lock that keeps the retired reduce-transparency block retired.
// R8 wrote the material blocks, but R7-4a/b (the plugin page host + topbar) and
// R7-5 (the unified toast) added surfaces after those blocks were written, and
// nothing asserted the new surfaces were covered. This file is that assertion.
//
// One media-block reader, walked by its own braces (never a fixed-width window,
// which silently goes vacuous when the file around it moves), plus the
// structural scans that make each backstop mean something:
//
//   * IC: the stronger stroke has to reach the states and the new chrome, not
//     only the two shells;
//   * RM: every selector that carries an `animation` anywhere in the host
//     sheets has to be neutralized in the reduced-motion block;
//   * RT: retired, not re-planted. The OS reduce-transparency media feature is
//     a dead hook on WebKit, so the block R8 wrote was deleted in R104 and the
//     scan below turns red if any live source reintroduces it.
import assert from "node:assert/strict";
import { readdir, readFile } from "node:fs/promises";
import test from "node:test";

const root = new URL("../", import.meta.url);
const read = (path: string) => readFile(new URL(path, root), "utf8");
const stripComments = (css: string) => css.replace(/\/\*[\s\S]*?\*\//g, "");

const RULES = (css: string) => {
  const out: { selector: string; body: string }[] = [];
  for (const match of css.matchAll(/([^{}]+)\{([^{}]*)\}/g)) {
    out.push({ selector: match[1].trim().replace(/\s+/g, " "), body: match[2] });
  }
  return out;
};

const base = async () => stripComments(await read("src/styles/base.css"));

// Every directory that holds host CSS. `src/extensions/` holds the uninstall
// dialog's private sheet, which consumes the host tokens and is covered by the
// RM scan too.
const HOST_DIRS = ["src/styles/", "src/extensions/"];

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

// Every selector a block declares with a given declaration, split on commas.
const selectorsDeclaring = (block: string, property: string, value: RegExp) => {
  const found = new Set<string>();
  for (const { selector, body } of RULES(block)) {
    const declaration = body.match(new RegExp(`(?:^|;)\\s*${property}\\s*:\\s*([^;]+)`, "g")) ?? [];
    if (!declaration.some((d) => value.test(d))) continue;
    for (const part of selector.split(",")) found.add(part.trim());
  }
  return found;
};

// ── Reduce transparency: retired, not re-planted ──────────────────────────
//
// R104 · the OS reduce-transparency media feature is a dead hook. WebKit has
// never implemented it (bugzilla 175497, opened 2007, still NEW), so on every
// platform Floter ships the query never matched and R8's block was a
// tombstone, not a backstop — the kind of thing that makes every later reader
// believe a system-level switch exists. R104 deleted it; reduce-transparency
// is owned by the app's own opacity sliders (`--main-opacity` /
// `--terminal-opacity`).
//
// This is the lock that keeps a live block from being re-planted, in the shape
// R76/R87/R96 established. The feature name is assembled from parts, and the
// scan reads every live source *including this file*, so a literal here would
// make the guard fail itself — which is exactly the property being asserted.
const RETIRED_RT_FEATURE = "prefers-reduced" + "-transparency";

/** Strip block and line comments so a comment that merely names the retired
 *  feature cannot trip — or satisfy — the scan. `terminal.css` names it in a
 *  comment on purpose: the note there is why re-planting it would be wrong. */
const stripAllComments = (source: string) =>
  source.replace(/\/\*[\s\S]*?\*\//g, "").replace(/^\s*\/\/.*$/gm, "");

test("RT: no live source re-plants the retired reduce-transparency block", async () => {
  // Non-vacuity: the two sibling backstops this suite still asserts are live,
  // so a wrong path or an emptied read cannot pass as "already gone".
  const css = await base();
  assert.ok(css.includes("@media (prefers-contrast: more)"), "the increase-contrast block must stay");
  assert.ok(
    css.includes("@media (prefers-reduced-motion: reduce)"),
    "the reduce-motion block must stay",
  );
  const collect = async (dir: string): Promise<string[]> => {
    const entries = await readdir(new URL(dir, root), { withFileTypes: true });
    const files: string[] = [];
    for (const entry of entries) {
      const path = `${dir}/${entry.name}`;
      if (entry.isDirectory()) files.push(...(await collect(path)));
      else if (/\.(?:css|tsx?)$/.test(entry.name)) files.push(path);
    }
    return files;
  };
  const files = [...(await collect("src")), ...(await collect("tests"))];
  assert.ok(files.length > 100, "the scan must cover the real source trees");
  assert.ok(files.includes("src/styles/base.css"), "the file the block lived in must be scanned");
  assert.ok(files.includes("tests/a11y-backstops.test.ts"), "the guard scans itself too");
  for (const file of files) {
    const source = stripAllComments(await read(file));
    assert.ok(
      !source.includes(RETIRED_RT_FEATURE),
      `${file} carries a live \`${RETIRED_RT_FEATURE}\` reference; R104 deleted the block because WebKit never implements that feature`,
    );
  }
});

// ── Increase contrast ─────────────────────────────────────────────────────

test("IC: the material floor is raised and the new chrome gets the stroke", async () => {
  const css = await base();
  const block = mediaBlock(css, "(prefers-contrast: more)");
  // The material: the frame's floor is raised on every step and haze dropped.
  assert.match(css, /--stroke-contrast:\s*rgba\(255,\s*255,\s*255,\s*0\.42\)/, "the dark contrast stroke");
  assert.match(block, /--glass-frame-floor:\s*0\.86/);
  assert.match(block, /--glass-step-dim:\s*0/);
  // The stronger strokes reach the states and the new chrome, not just the
  // shells.
  const stroked = selectorsDeclaring(block, "box-shadow", /var\(--stroke-contrast/);
  for (const selector of [
    ".settings-option",
    ".settings-option--active",
    ".extension-row--selected",
    ".extension-health__tag",
  ]) {
    assert.ok(stroked.has(selector), `IC must strengthen the stroke on ${selector}`);
  }
  // The floating chrome takes a stronger border instead.
  const bordered = selectorsDeclaring(block, "border-color", /var\(--stroke-contrast\)/);
  for (const selector of [".collapsed-card", ".settings-card", ".terminal-panel", ".app-toast", ".extension-drawer"]) {
    assert.ok(bordered.has(selector), `IC must strengthen the border on ${selector}`);
  }
});

// ── Reduce motion ─────────────────────────────────────────────────────────

test("RM: every animated selector in the host sheets is neutralized", async () => {
  const block = mediaBlock(await base(), "(prefers-reduced-motion: reduce)");
  const neutralized = selectorsDeclaring(block, "animation", /none/);
  // Scan every host sheet for a live animation declaration. `src/extensions/`
  // is a host directory too: the uninstall dialog's private sheet consumes the
  // host tokens and must obey the same motion rules (the reviewer injected an
  // animation there and the old `src/styles/`-only scan stayed green).
  const animated = new Set<string>();
  for (const dir of HOST_DIRS) {
    for (const name of (await readdir(new URL(dir, root))).filter((n) => n.endsWith(".css"))) {
      const file = stripComments(await read(`${dir}${name}`));
      for (const { selector, body } of RULES(file)) {
        const value = body.match(/(?:^|;)\s*animation\s*:\s*([^;}]+)/)?.[1]?.trim();
        if (!value || value === "none") continue;
        for (const part of selector.split(",")) animated.add(part.trim());
      }
    }
  }
  assert.ok(animated.size > 0, "the scan must find at least one animation");
  for (const selector of animated) {
    assert.ok(
      neutralized.has(selector),
      `prefers-reduced-motion must set animation: none on ${selector}`,
    );
  }
  // The R7-4/R7-5 additions specifically.
  for (const selector of [".app-toast", ".extensions-overflow__items", ".settings-save-alert--toast"]) {
    assert.ok(neutralized.has(selector), `RM must neutralize ${selector}`);
  }
});

test("RM: the host scan reaches the extension's private sheet", async () => {
  // Minor 3 / the reviewer's finding: the scan used to read `src/styles/` only,
  // so an animation injected into `src/extensions/ComponentizedUninstallDialog.css`
  // was invisible. `hostSheets()` and the RM scan must both read both host
  // directories. This asserts the range explicitly, so a future refactor that
  // narrows it fails here as well as in the behavioural test.
  const css = stripComments(await read("src/extensions/ComponentizedUninstallDialog.css"));
  assert.ok(css.includes(".extensions-uninstall-component"), "the private sheet must be read");
  const hostFiles: string[] = [];
  for (const dir of HOST_DIRS) {
    for (const name of (await readdir(new URL(dir, root))).filter((n) => n.endsWith(".css"))) {
      hostFiles.push(`${dir}${name}`);
    }
  }
  assert.ok(
    hostFiles.includes("src/extensions/ComponentizedUninstallDialog.css"),
    `the host scan must include the extension sheet, got:\n  ${hostFiles.join("\n  ")}`,
  );
});

test("RM: every finite animation duration is a motion token", async () => {
  for (const dir of HOST_DIRS) {
    for (const name of (await readdir(new URL(dir, root))).filter((n) => n.endsWith(".css"))) {
      const file = stripComments(await read(`${dir}${name}`));
      for (const { selector, body } of RULES(file)) {
        const value = body.match(/(?:^|;)\s*animation\s*:\s*([^;}]+)/)?.[1]?.trim();
        if (!value || value === "none") continue;
        // An ambient loop is periodic, and its period is deliberately not on the
        // entrance scale; every finite animation must name a duration token.
        if (/\binfinite\b/.test(value)) continue;
        assert.match(
          value,
          /var\(--dur-\d\)/,
          `${dir}${name}: ${selector} animation: ${value} — a finite animation must use a --dur-* token`,
        );
      }
    }
  }
  // The new spring-driven entrances are tokenized too: no raw cubic-bezier
  // and no raw duration on the toast / overflow menu / settings toast.
  for (const [name, selector] of [
    ["extensions.css", ".app-toast"],
    ["extensions.css", ".extensions-overflow__items"],
    ["settings.css", ".settings-save-alert--toast"],
  ] as const) {
    const css = stripComments(await read(`src/styles/${name}`));
    const rule = RULES(css).find((r) => r.selector === selector);
    assert.ok(rule, `${name}: ${selector} must exist`);
    assert.match(rule!.body, /animation:\s*\S+\s+var\(--dur-\d\)\s+var\(--spring\)/, `${name}: ${selector} must use the spring token`);
  }
});
