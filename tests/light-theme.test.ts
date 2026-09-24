// HIG-2 · the light-theme start.
//
// The light theme is zero-investment at HEAD (the reviewer's finding). HIG-2
// does the *start*, not a full design: it audits which dark tokens are
// structurally wrong on a light backdrop, adds the minimal
// `prefers-color-scheme: light` block for the pre-hydration first paint, and
// pins that block to the real `[data-theme="light"]` palette so the two cannot
// drift.
//
// The audit below is the "must be covered" list. A token on it is a dark value
// that would be wrong-on-light by construction (near-white text, near-black
// tints, a hairline that assumes a dark backdrop); a token off it is
// palette-independent (a radius, a type step, a duration, an elevation rung
// built from other tokens).
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

const root = new URL("../", import.meta.url);
const read = (path: string) => readFile(new URL(path, root), "utf8");
const stripComments = (css: string) => css.replace(/\/\*[\s\S]*?\*\//g, "");

const base = async () => stripComments(await read("src/styles/base.css"));

// Slice from a block's opening brace to its matching close, so the assertions
// read the block's own content.
const blockAt = (css: string, start: number) => {
  const open = css.indexOf("{", start);
  assert.notEqual(open, -1, "block must have an opening brace");
  let depth = 0;
  for (let i = open; i < css.length; i += 1) {
    if (css[i] === "{") depth += 1;
    else if (css[i] === "}") {
      depth -= 1;
      if (depth === 0) return css.slice(open + 1, i);
    }
  }
  assert.fail("unterminated block");
};

const lightBlock = async () => {
  const css = await base();
  const start = css.indexOf('[data-theme="light"]');
  assert.notEqual(start, -1, 'missing [data-theme="light"]');
  return blockAt(css, start);
};

const cssLightBlock = async () => {
  const css = await base();
  const start = css.indexOf("@media (prefers-color-scheme: light)");
  assert.notEqual(start, -1, "missing @media (prefers-color-scheme: light)");
  return blockAt(css, start);
};

// ── The audit: which dark tokens must be re-stated for light ──────────────

// Every token on this list is a dark value that is wrong on a light backdrop
// for one of three structural reasons. It is the round's "light must cover"
// checklist; the test asserts each one really is covered.
const MUST_COVER: { token: string; why: string }[] = [
  // 1. Text: the dark palette's text is near-white.
  { token: "text-primary", why: "near-white text is invisible on white" },
  { token: "text-strong", why: "near-white text" },
  { token: "text-secondary", why: "light text" },
  { token: "text-muted", why: "light text" },
  { token: "text-tertiary", why: "light text" },
  { token: "text-placeholder", why: "light text" },
  { token: "text-warning", why: "a pale pink warning" },
  { token: "text-placeholder-contrast", why: "light text" },
  // 2. Surfaces and recesses: the dark tints are near-black.
  { token: "glass-tint", why: "near-black frame tint" },
  { token: "glass-tint-terminal", why: "near-black terminal tint" },
  { token: "glass-float", why: "near-black floater tint" },
  { token: "surface-sunken", why: "near-black content recess" },
  { token: "surface-sunken-soft", why: "near-black content recess" },
  { token: "surface-opaque", why: "a near-black opaque stand-in" },
  // 3. Strokes and shadows: white hairlines/rims vanish, black ones scratch.
  { token: "hairline", why: "a white hairline is invisible on white" },
  { token: "edge-shadow", why: "a dark bottom edge" },
  { token: "keycap-shadow", why: "a dark keycap shadow" },
  { token: "window-shadow-ambient", why: "a heavy black ambient shadow" },
  { token: "window-shadow-contact", why: "a heavy black contact shadow" },
  { token: "input-stroke", why: "a white field stroke" },
  { token: "input-stroke-active", why: "a pastel-blue field stroke" },
  { token: "glass-control", why: "a white overlay on white is invisible" },
  { token: "glass-control-hover", why: "a white overlay on white" },
  { token: "glass-control-press", why: "a white overlay on white" },
  { token: "glass-control-edge", why: "a white stroke" },
  { token: "glass-control-rim", why: "a white rim" },
  { token: "glass-track", why: "a white groove on white" },
  { token: "glass-track-edge", why: "a white groove stroke" },
  { token: "glass-field", why: "a white field on white" },
  { token: "glass-field-shadow", why: "a dark inset that needs to lighten" },
  { token: "glass-raised-rim", why: "a white rim on white" },
  { token: "sheen-top", why: "a white sheen" },
  { token: "sheen-bottom", why: "a white sheen" },
  { token: "edge-highlight", why: "a white highlight" },
  { token: "edge-highlight-x", why: "a white side highlight" },
  { token: "keycap-highlight", why: "a white keycap highlight" },
  // 4. Accent, status and terminal: the pastel accent is below AA on white,
  //    and the terminal canvas paints its colours directly.
  { token: "accent", why: "a pastel blue is below AA on white" },
  { token: "accent-contrast", why: "the dark ink on the accent" },
  { token: "accent-tint", why: "a pastel accent tint" },
  { token: "accent-tint-hover", why: "a pastel accent tint" },
  { token: "accent-edge", why: "a pastel accent edge" },
  { token: "accent-edge-strong", why: "a pastel accent edge" },
  { token: "accent-glow", why: "a pastel glow" },
  { token: "accent-wash", why: "a pastel wash" },
  { token: "accent-ring", why: "a pastel focus ring" },
  { token: "accent-halo", why: "a pastel halo" },
  { token: "system-icon-surface", why: "a warm tint tuned for dark" },
  { token: "system-icon-surface-hover", why: "a warm tint tuned for dark" },
  { token: "system-icon-text", why: "a pale warm text" },
  { token: "action-shell-tint", why: "a green tint tuned for dark" },
  { token: "terminal-bg", why: "a near-black canvas" },
  { token: "terminal-fg", why: "a light canvas foreground" },
  { token: "terminal-cursor", why: "a cursor colour tuned for dark" },
  { token: "terminal-selection", why: "a white selection on a dark canvas" },
  { token: "terminal-scrollbar", why: "a light scrollbar" },
  { token: "terminal-bar", why: "a dark bar tint" },
  { token: "terminal-bar-top", why: "a dark bar tint" },
  { token: "terminal-bar-bottom", why: "a dark bar tint" },
  { token: "scrollbar-thumb", why: "a light scrollbar thumb" },
  { token: "stroke-contrast", why: "a white contrast stroke" },
  { token: "stroke-contrast-soft", why: "a white contrast stroke" },
];

test("the light palette covers every structurally-wrong dark token", async () => {
  const light = await lightBlock();
  const covered = new Set(
    [...light.matchAll(/--([a-z0-9-]+)\s*:/g)].map((m) => m[1]),
  );
  const missing = MUST_COVER.filter(({ token }) => !covered.has(token));
  assert.deepEqual(
    missing.map((m) => `--${m.token} (${m.why})`),
    [],
    "these dark tokens must be re-stated in the light palette",
  );
});

test("the light palette does not restate palette-independent tokens", async () => {
  // The counterpart to the audit: a ladder token (radius, type, motion, the
  // elevation rungs, the material *steps*) must NOT be duplicated per palette,
  // or the palette blocks become a second, drifting source.
  const light = await lightBlock();
  const covered = new Set(
    [...light.matchAll(/--([a-z0-9-]+)\s*:/g)].map((m) => m[1]),
  );
  const PALETTE_INDEPENDENT = [
    "radius-xs", "radius-sm", "radius-md", "radius-lg", "window-radius",
    "text-caption", "text-body", "text-emphasis", "text-title", "text-display",
    "dur-1", "dur-2", "dur-3", "dur-4", "spring", "ease-out", "ease-in-out", "ease-out-back",
    "elev-0", "elev-1", "elev-2", "elev-3", "elev-shadow-scale", "accent-budget",
    "focus-ring-width", "focus-ring-offset", "scroll-edge", "scroll-edge-soft",
    "glass-step-blur", "glass-step-saturate", "glass-step-fill", "glass-step-dim",
    "glass-solid-top", "main-opacity", "terminal-opacity", "glass-lens-scale",
  ];
  const duplicated = PALETTE_INDEPENDENT.filter((token) => covered.has(token));
  assert.deepEqual(
    duplicated.map((t) => `--${t}`),
    [],
    "these tokens are palette-independent and must not be duplicated in the light block",
  );
});

// ── The pre-hydration block ───────────────────────────────────────────────

test("the pre-hydration light block covers the crash tokens and stands down after hydration", async () => {
  const css = await base();
  const media = css.indexOf("@media (prefers-color-scheme: light)");
  assert.notEqual(media, -1);
  // Whole-block read (the media block wraps `html:not([data-theme])`).
  const outer = blockAt(css, media);
  // The guard: once App.tsx has written an explicit theme, the attribute
  // palette wins and this block must not apply.
  assert.match(outer, /html:not\(\[data-theme\]\)/, "the pre-hydration block must stand down once data-theme is set");
  assert.match(outer, /color-scheme:\s*light/);
  // It has to cover the crash set: text, surfaces, strokes, accent, terminal.
  const pre = new Set([...outer.matchAll(/--([a-z0-9-]+)\s*:/g)].map((m) => m[1]));
  for (const token of [
    "text-primary", "text-secondary", "text-muted", "text-warning",
    "glass-tint", "glass-float", "surface-sunken", "surface-opaque",
    "hairline", "glass-control", "glass-control-edge", "glass-raised-rim",
    "accent", "accent-tint", "accent-ring",
    "terminal-bg", "terminal-fg", "terminal-bar",
  ]) {
    assert.ok(pre.has(token), `the pre-hydration block must cover --${token}`);
  }
});

test("the pre-hydration values are pinned to the real light palette", async () => {
  // The two light blocks are the same palette expressed twice (once for
  // `data-theme`, once for the media query). The values must agree, or the
  // first frame is a different theme from the second.
  const css = await base();
  const light = await lightBlock();
  const pre = blockAt(css, css.indexOf("@media (prefers-color-scheme: light)"));
  const valueOf = (block: string, token: string) => {
    const match = block.match(new RegExp(`--${token}:\\s*([^;]+);`));
    assert.ok(match, `--${token} must be defined`);
    return match![1].replace(/\s+/g, " ").trim();
  };
  // Every token the pre-hydration block declares has to equal the palette
  // block's value.
  const preTokens = [...pre.matchAll(/--([a-z0-9-]+)\s*:/g)].map((m) => m[1]);
  for (const token of preTokens) {
    assert.equal(
      valueOf(pre, token),
      valueOf(light, token),
      `--${token} differs between the media-query block and [data-theme="light"]`,
    );
  }
});

test("the light text and accent clear the readability line", async () => {
  const css = await base();
  const light = await lightBlock();
  const rgba = (block: string, token: string) => {
    const value = block.match(new RegExp(`--${token}:\\s*([^;]+);`))![1].trim();
    const hex = value.match(/^#([0-9a-f]{6})$/i);
    if (hex) {
      const n = parseInt(hex[1], 16);
      return [(n >> 16) & 255, (n >> 8) & 255, n & 255, 1] as const;
    }
    const m = value.match(/rgba?\(\s*(\d+),\s*(\d+),\s*(\d+)(?:,\s*([\d.]+))?\s*\)/);
    assert.ok(m, `--${token} must be a hex or rgba, got "${value}"`);
    return [Number(m![1]), Number(m![2]), Number(m![3]), m![4] === undefined ? 1 : Number(m![4])] as const;
  };
  // The composited light recess over a black desktop (the light theme's worst
  // case, per the glass-material suite).
  const luminance = ([r, g, b]: readonly number[]) => {
    const lin = [r, g, b].map((c) => {
      const s = c / 255;
      return s <= 0.03928 ? s / 12.92 : ((s + 0.055) / 1.055) ** 2.4;
    });
    return 0.2126 * lin[0] + 0.7152 * lin[1] + 0.0722 * lin[2];
  };
  const ratio = (a: readonly number[], b: readonly number[]) => {
    const [l1, l2] = [luminance(a), luminance(b)].sort((x, y) => y - x);
    return (l1 + 0.05) / (l2 + 0.05);
  };
  const alpha = (name: string) => {
    const value = css.match(new RegExp(`--glass-content-alpha[^;]*;`));
    void value;
    return Number(light.match(new RegExp(`--${name}:\\s*rgba\\([^)]*,\\s*([\\d.]+)\\)`))?.[1] ?? 1);
  };
  // Recess composite: the frame alpha (the transparency slider clamped to the
  // near-solid top and lifted only by the accessibility floor) plus the
  // regular step's haze, then the content band over it, over black.
  const content = Number(css.match(/--glass-content-alpha:\s*calc\(\s*([\d.]+)/)![1]);
  const main = Number(css.match(/--main-opacity:\s*([\d.]+);/)![1]);
  const solidTop = Number(css.match(/--glass-solid-top:\s*([\d.]+);/)![1]);
  const frameFloor = Number(css.match(/--glass-frame-floor:\s*([\d.]+);/)![1]);
  const haze = Number(css.match(/--glass-step-dim:\s*([\d.]+);/)![1]);
  const frameAlpha = Math.min(solidTop, Math.max(frameFloor, main));
  const frame = 1 - (1 - haze * (1 - main)) * (1 - frameAlpha);
  const recessAlpha = Math.min(1, content + 0.18 * main);
  void alpha;
  const flatten = (fg: readonly number[], bg: readonly number[]) => {
    const a = fg[3];
    return [0, 1, 2].map((i) => fg[i] * a + bg[i] * (1 - a));
  };
  const frameRgb = flatten([250, 250, 252, frame], [0, 0, 0]);
  const recess = flatten([243, 244, 247, recessAlpha], frameRgb);
  // The text steps body copy actually uses. HIG-2 raised the light `--text-muted`
  // so all four clear AA body (4.5:1), not just the two headline steps: the
  // description copy under a settings option is 11px body text, and the round's
  // red line is now one number instead of a documented exception.
  for (const [token, floor] of [
    ["text-primary", 4.5],
    ["text-secondary", 4.5],
    ["text-muted", 4.5],
  ] as const) {
    const fg = flatten(rgba(light, token), recess);
    const r = ratio(fg, recess);
    assert.ok(
      r >= floor,
      `light --${token} reaches only ${r.toFixed(2)}:1 on the recess (floor ${floor}:1)`,
    );
  }
  // The accent is a label colour in light mode: AA-body for the small copy.
  const accent = flatten(rgba(light, "accent"), recess);
  const accentRatio = ratio(accent, recess);
  assert.ok(accentRatio >= 4.5, `light --accent reaches only ${accentRatio.toFixed(2)}:1 on the recess`);
});
