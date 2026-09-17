// GLASS-REAXIS: the control-lens application census and the settings migration.
//
// The round's third hard requirement was 「当前的液态玻璃效果还是不够，特别是
// 各种控件按钮」. The fix is the `--glass-lens-*` family plus the upgraded
// control rungs, and this file is the census that keeps the *application* from
// drifting: the buttons, segmented controls, fields and switch tracks the user
// named must actually draw the lens, not a flat pane, and the stop control must
// grade them.
//
// It also pins the two migrations the user's file may arrive through:
//
//   * a pre-R8 file (`main_opacity` only) — the Rust split is untouched;
//   * a GLASS-UNIFY file (a `(step, tint)` pair) — the stop is read from the
//     step and the tint is preserved as the slider value.
import assert from "node:assert/strict";
import { readdir, readFile } from "node:fs/promises";
import test from "node:test";

const root = new URL("../", import.meta.url);
const read = (path: string) => readFile(new URL(path, root), "utf8");
const stripComments = (css: string) => css.replace(/\/\*[\s\S]*?\*\//g, "");

const rules = (css: string) => {
  const out: { selector: string; body: string }[] = [];
  for (const match of css.matchAll(/([^{}]+)\{([^{}]*)\}/g)) {
    out.push({ selector: match[1].trim(), body: match[2] });
  }
  return out;
};

const hostSheets = async () => {
  const out: { name: string; css: string }[] = [];
  for (const dir of ["src/styles/", "src/extensions/"]) {
    for (const name of (await readdir(new URL(dir, root))).filter((n) => n.endsWith(".css"))) {
      out.push({ name: `${dir}${name}`, css: stripComments(await read(`${dir}${name}`)) });
    }
  }
  return out;
};

/** The rule body for a selector, in the given sheet. */
const ruleBody = (css: string, selector: string) => {
  const rule = rules(stripComments(css)).find(({ selector: s }) =>
    s.split(",").map((p) => p.trim()).includes(selector),
  );
  assert.ok(rule, `${selector} must exist`);
  return rule!.body;
};

// ── A. the lens reaches the controls the user named ────────────────────────

test("every named control draws a lens rung, not a bare ring", async () => {
  // The user named buttons, segmented controls, fields and switch tracks. Each
  // must resolve its resting shadow to a rung the lens now owns — either
  // `--elev-0` (the raised lens) or `--elev-track` (the recessed lens) — so a
  // stop change grades all of them. A control that regressed to a hand-rolled
  // `inset 0 0 0 1px` would lose the lens silently; this is the census that
  // makes it loud.
  const controls: [string, string][] = [
    // Buttons (raised lens).
    ["src/styles/extensions.css", ".extensions-action-button"],
    ["src/styles/extensions.css", ".app-toast__action"],
    ["src/styles/terminal.css", ".plugin-page-host__button"],
    ["src/styles/settings.css", ".settings-save-alert button"],
    ["src/styles/settings.css", ".settings-copy-button"],
    ["src/styles/settings.css", ".session-manager__sort-toggle"],
    ["src/styles/settings.css", ".settings-option--active"],
    ["src/styles/settings.css", ".settings-option--static:hover"],
    ["src/styles/settings.css", ".settings-sidebar__item:hover"],
    ["src/styles/launcher.css", ".launcher-result__icon"],
    ["src/styles/launcher.css", ".launcher-action-bar__icon"],
    // Segmented controls (recessed track + raised chosen slot).
    ["src/styles/settings.css", ".settings-options--inline"],
    ["src/styles/extensions.css", ".extension-custom-mode"],
    ["src/styles/terminal.css", ".clipboard-panel__tabs"],
    // Switch track (recessed lens).
    ["src/styles/settings.css", ".settings-switch"],
  ];
  for (const [file, selector] of controls) {
    const body = ruleBody(await read(file), selector);
    assert.match(
      body,
      /box-shadow:\s*var\(--elev-(0|track)\)/,
      `${file}: ${selector} must draw a lens rung (--elev-0 / --elev-track), got "${body.match(/box-shadow:[^;]+/)?.[0] ?? "none"}"`,
    );
  }

  // Two controls are deliberately transparent at rest and light their lens
  // only under the pointer: the icon chip (a bare glyph on the surface) and the
  // clipboard row. Their *hover* rule must carry the lens rung, or the lens
  // never reaches them.
  for (const [file, selector] of [
    ["src/styles/extensions.css", ".extensions-icon-button:hover:not(:disabled):not(.extensions-icon-button--disabled)"],
    ["src/styles/terminal.css", ".clipboard-row:hover"],
    ["src/styles/terminal.css", ".toolbar-button:hover:not(:disabled)"],
    ["src/styles/launcher.css", ".launcher-result:not(.launcher-result--unavailable):not(.launcher-result--selected):hover"],
  ] as [string, string][]) {
    const body = ruleBody(await read(file), selector);
    assert.match(
      body,
      /box-shadow:\s*var\(--elev-(0|track)\)/,
      `${file}: ${selector} must light a lens rung under the pointer`,
    );
  }
});

test("the recessed track lens is the raised lens read backwards", async () => {
  const css = stripComments(await read("src/styles/base.css"));
  const rootBlock = css.slice(css.indexOf(":root {"), css.indexOf('[data-theme="light"]'));
  const inset = rootBlock.match(/--glass-lens-stack-inset:\s*([^;]+);/);
  assert.ok(inset, "--glass-lens-stack-inset must be declared");
  const value = inset![1];
  // The groove's light pools at the bottom (the rim is on the bottom edge) and
  // its top edge is the dark one — the opposite of the raised stack.
  assert.match(value, /inset 0 -1px 0 var\(--glass-lens-rim\)/, "the track's lit edge is its bottom");
  assert.match(value, /inset 0 1px 0 var\(--glass-lens-edge\)/, "the track's dark edge is its top");
  assert.match(value, /inset 0 1px 2px/, "the track keeps its inner slot shadow");
  assert.match(value, /var\(--glass-track-edge\)/, "the track keeps its ring");
  for (const layer of value.split(/,(?![^(]*\))/).map((p) => p.trim()).filter(Boolean)) {
    assert.match(layer, /^inset/, `a track lens layer must be inset, got "${layer}"`);
  }
  // `--elev-track` is that token, so a stop grades every groove at once.
  assert.equal(rootBlock.match(/--elev-track:\s*([^;]+);/)![1].trim(), "var(--glass-lens-stack-inset)");
});

test("the hover cue is the lens plus an edge glow, and only on the pointer", async () => {
  const css = stripComments(await read("src/styles/base.css"));
  const rootBlock = css.slice(css.indexOf(":root {"), css.indexOf('[data-theme="light"]'));
  const hover = rootBlock.match(/--glass-lens-stack-hover:\s*([^;]+);/);
  assert.ok(hover, "--glass-lens-stack-hover must be declared");
  assert.match(hover![1], /var\(--glass-lens-stack\)/, "the hover cue builds on the resting lens");
  assert.match(hover![1], /var\(--glass-lens-glow\)/, "the hover cue adds the edge glow");
  // The glow is a hover-only token: it must not appear in the resting stack, or
  // every control would glow at rest.
  assert.ok(
    !/glass-lens-glow/.test(rootBlock.match(/--glass-lens-stack:\s*([^;]+);/)![1]),
    "the resting lens must not carry the hover glow",
  );
});

test("the lens is never a filter and never an animated property", async () => {
  // The performance red line: the lens is drawn with insets, so no sheet may
  // pair a lens token with a filter, and no transition may name one.
  for (const { name, css } of await hostSheets()) {
    for (const { selector, body } of rules(css)) {
      if (!/glass-lens/.test(body)) continue;
      assert.ok(!/backdrop-filter/.test(body), `${name}: ${selector} pairs a lens with a backdrop-filter`);
      for (const value of [...body.matchAll(/(?:^|;)\s*(?:transition|animation|will-change)\s*:\s*([^;]+)/g)]) {
        assert.ok(!/glass-lens/.test(value[1]), `${name}: ${selector} animates a lens token (${value[1]})`);
      }
    }
  }
});

test("the lens is a keyline, not an accent fill, so the budget is untouched", async () => {
  // The lens family uses white/black insets and the pane edge — no accent. If a
  // future round tinted a lens with the accent it would silently join the
  // accent census; this makes that a red test.
  const css = stripComments(await read("src/styles/base.css"));
  const rootBlock = css.slice(css.indexOf(":root {"), css.indexOf('[data-theme="light"]'));
  const slice = rootBlock.slice(rootBlock.indexOf("--glass-lens-rim-base"));
  const lensDefs = slice.slice(0, slice.indexOf("--glass-raised-shadow"));
  assert.ok(!/accent/.test(lensDefs), "the lens family must not consume the accent");
});

// ── B. the settings migrations ─────────────────────────────────────────────

test("a pre-R8 file still splits, and the split's opacity lands in the sliders", async () => {
  // The Rust migration is untouched by this round; the assertion is that the
  // *frontend* now reads the migrated pair as (step → stop, opacity → slider).
  const { glassIntensityOf } = await import("../src/glass-material.ts");
  // 94 → 47% + high (the shipped migration).
  assert.equal(glassIntensityOf("high", 47 / 100), 3, "the migrated 94 shows stop 3");
  // 25 → 13% + low.
  assert.equal(glassIntensityOf("low", 13 / 100), 1, "the migrated 25 shows stop 1");
  // 50 → 25% + mid.
  assert.equal(glassIntensityOf("mid", 25 / 100), 2, "the migrated 50 shows stop 2");
  // The Rust side still declares the same split and defaults.
  const rust = await read("src-tauri/src/commands/config.rs");
  assert.match(rust, /settings\.main_opacity = legacy\.div_ceil\(2\) as u8/);
  assert.match(rust, /const DEFAULT_MAIN_OPACITY: u8 = 47;/);
  assert.match(rust, /const DEFAULT_TERMINAL_OPACITY: u8 = 46;/);
});

test("a GLASS-UNIFY (step, tint) pair loses no information", async () => {
  // GLASS-UNIFY wrote a (step, tint) pair into (glass_step, main_opacity). The
  // stop is recovered from the step; the tint is *kept* as the opacity, so an
  // upgrading user sees the same window solidity and can nudge it.
  const { glassIntensityOf, GLASS_INTENSITY } = await import("../src/glass-material.ts");
  const unified: [string, number, number][] = [
    // [step written by GLASS-UNIFY, tint it wrote, stop it displays on]
    ["low", 0.22, 1],
    ["mid", 0.4, 2],
    ["high", 0.58, 3],
    ["high", 0.78, 3],
    ["high", 0.98, 3],
  ];
  for (const [step, tint, stop] of unified) {
    assert.equal(
      glassIntensityOf(step as "low" | "mid" | "high", tint),
      stop,
      `GLASS-UNIFY (${step}, ${tint}) must display on stop ${stop}`,
    );
  }
  // The new stops are reachable only through the new ids — a GLASS-UNIFY file
  // can never have written them, which is what makes the migration lossless.
  for (const level of [4, 5] as const) {
    assert.ok(
      GLASS_INTENSITY[level].step === "deep" || GLASS_INTENSITY[level].step === "jelly",
      `stop ${level} must use a new step id`,
    );
  }
});

test("the stop value domain is five ids on both sides of the bridge", async () => {
  const { GLASS_STEPS } = await import("../src/glass-material.ts");
  assert.deepEqual([...GLASS_STEPS], ["low", "mid", "high", "deep", "jelly"]);
  // The Rust loader accepts all five.
  const rust = await read("src-tauri/src/commands/config.rs");
  for (const step of GLASS_STEPS) {
    assert.ok(rust.includes(`"${step}"`), `Rust must know the ${step} step`);
  }
  // The plugin bridge accepts all five, so a page on stop 4/5 gets its step.
  const bridge = await read("src/plugin-pages.ts");
  for (const step of GLASS_STEPS) {
    assert.ok(bridge.includes(`"${step}"`), `the bridge must accept the ${step} step`);
  }
});

// ── C. the stop and the sliders stay independent in the store ──────────────

test("the settings shape keeps the three fields separate", async () => {
  const app = await read("src/App.tsx");
  for (const field of ["main_opacity", "terminal_opacity", "glass_step"]) {
    assert.match(app, new RegExp(`${field}:`), `AppSettings must carry ${field}`);
  }
  // The two transparency custom properties are still written from the two
  // opacity fields, and the step from its own effect.
  assert.match(app, /setProperty\("--main-opacity"/);
  assert.match(app, /setProperty\("--terminal-opacity"/);
  assert.match(app, /setAttribute\("data-glass", settings\.glass_step\)/);
});

test("the effect stop does not change the shipped frame fill", async () => {
  // The stop is the effect axis, so the three heaviest stops share one fill
  // floor: changing stop 3 → 5 must not make the window more opaque. This is
  // the numerical form of "档位不是 tint".
  const css = stripComments(await read("src/styles/base.css"));
  const fill = (step: string) => {
    const start = css.indexOf(`[data-glass="${step}"]`);
    const open = css.indexOf("{", start);
    const close = css.indexOf("}", open);
    return Number(css.slice(open, close).match(/--glass-step-fill:\s*([\d.]+)/)![1]);
  };
  assert.equal(fill("high"), fill("deep"), "stop 3 and 4 share a fill floor");
  assert.equal(fill("deep"), fill("jelly"), "stop 4 and 5 share a fill floor");
  // …while the saturation — the effect — does climb.
  const sat = (step: string) => {
    const start = css.indexOf(`[data-glass="${step}"]`);
    const open = css.indexOf("{", start);
    const close = css.indexOf("}", open);
    return Number(css.slice(open, close).match(/--glass-step-saturate:\s*([\d.]+)/)![1]);
  };
  assert.ok(sat("jelly") > sat("deep") && sat("deep") > sat("high"), "saturation must climb");
});
