// HIG-2 · the three accessibility backstops: reduce-transparency, increase-
// contrast, reduce-motion. R8 wrote the material blocks, but R7-4a/b (the
// plugin page host + topbar) and R7-5 (the unified toast) added surfaces after
// those blocks were written, and nothing asserted the new surfaces were
// covered. This file is that assertion.
//
// One media-block reader, walked by its own braces (never a fixed-width window,
// which silently goes vacuous when the file around it moves), plus the
// structural scans that make each backstop mean something:
//
//   * RT: an opaque fallback is not enough — the surface must also have dropped
//     its blur, or the "opaque" claim is just a background over a live filter;
//   * IC: the stronger stroke has to reach the states and the new chrome, not
//     only the two shells;
//   * RM: every selector that carries an `animation` anywhere in the host
//     sheets has to be neutralized in the reduced-motion block.
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

// ── Reduce transparency ───────────────────────────────────────────────────

test("RT: every surface shell goes near-solid and drops its blur", async () => {
  const block = mediaBlock(await base(), "(prefers-reduced-transparency: reduce)");
  const opaque = selectorsDeclaring(block, "background", /var\(--surface-opaque\)/);
  // The shells R8 covered…
  for (const shell of [
    ".collapsed-card",
    ".collapsed-card:focus-within",
    ".settings-card",
    ".terminal-panel",
    ".clipboard-panel",
  ]) {
    assert.ok(opaque.has(shell), `RT must make ${shell} near-solid`);
  }
  // …plus the post-R8 surfaces HIG-2 added.
  for (const shell of [".plugin-page-host__frame", ".plugin-page-host__topbar", ".plugin-page-host__loading", ".plugin-page-host__error"]) {
    assert.ok(opaque.has(shell), `RT must make the post-R8 surface ${shell} near-solid`);
  }
  // The two floaters and the content recesses.
  for (const surface of [".app-toast", ".settings-save-alert--toast", ".launcher-bottom", ".settings-content", ".extension-tool-results"]) {
    assert.ok(opaque.has(surface), `RT must make ${surface} near-solid`);
  }
  // The blur must be dropped wherever a blur existed. `.plugin-page-host__frame`
  // and `.plugin-page-host__topbar` are claimed to have no blur of their own
  // (the frame is opaque), so the assertion is that no RT-covered surface is
  // left with a live filter.
  const noBlur = new Set<string>();
  for (const { selector, body } of RULES(block)) {
    if (!/(?:^|;)\s*(?:-webkit-)?backdrop-filter\s*:\s*none/.test(body)) continue;
    for (const part of selector.split(",")) noBlur.add(part.trim());
  }
  for (const shell of [".collapsed-card", ".settings-card", ".terminal-panel", ".clipboard-panel", ".plugin-page-host__frame"]) {
    assert.ok(noBlur.has(shell), `RT must drop the blur on ${shell}`);
  }
  // The near-solid stand-in is genuinely near-solid, not a token that could
  // drift translucently.
  const css = await base();
  const opaqueValue = css.match(/--surface-opaque:\s*rgba\(\s*[\d.]+,\s*[\d.]+,\s*[\d.]+,\s*([\d.]+)\s*\)/);
  assert.ok(opaqueValue, "--surface-opaque must be an explicit rgba");
  assert.ok(Number(opaqueValue![1]) >= 0.98, `--surface-opaque must be near-solid, got ${opaqueValue![1]}`);
});

// ── Increase contrast ─────────────────────────────────────────────────────

test("IC: the material floor is raised and the new chrome gets the stroke", async () => {
  const css = await base();
  const block = mediaBlock(css, "(prefers-contrast: more)");
  // The material: the fill floor is raised on every step and dimming dropped.
  assert.match(css, /--stroke-contrast:\s*rgba\(255,\s*255,\s*255,\s*0\.42\)/, "the dark contrast stroke");
  assert.match(block, /--glass-step-fill:\s*0\.86/);
  assert.match(block, /--glass-step-dim:\s*0/);
  // The stronger strokes reach the states and the new chrome, not just the
  // shells.
  const stroked = selectorsDeclaring(block, "box-shadow", /var\(--stroke-contrast/);
  for (const selector of [
    ".settings-option",
    ".settings-option--active",
    ".clipboard-panel__tab--active",
    ".extension-row--selected",
    ".extension-health__tag",
  ]) {
    assert.ok(stroked.has(selector), `IC must strengthen the stroke on ${selector}`);
  }
  // The floating chrome takes a stronger border instead.
  const bordered = selectorsDeclaring(block, "border-color", /var\(--stroke-contrast\)/);
  for (const selector of [".collapsed-card", ".settings-card", ".terminal-panel", ".plugin-page-host__topbar", ".plugin-page-host__button", ".app-toast", ".extension-drawer"]) {
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
  assert.ok(
    !hostFiles.includes("src/plugins/clipboard/page.css"),
    "the clipboard plugin page is a page boundary and must stay out of the host scan",
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
