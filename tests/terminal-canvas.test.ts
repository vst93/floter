// R8-GLASS-UX: the terminal canvas's background alpha.
//
// The canvas is the one surface that cannot carry a CSS `rgba()` tint — it is
// a bitmap the renderer paints over the surface's glass frame — so its alpha
// is computed in JavaScript instead of by the stylesheet. That makes it the
// easiest place for the two controls to be re-coupled by a later edit (it was
// a plain `--terminal-opacity` read before R8), and the composition had no
// test at all.
//
// These assertions pin the composition to the shipped tokens: the canvas alpha
// is the material step's floor sliding to the near-solid top on the terminal
// transparency slider — *not* the slider value, which is what "decoupled"
// means numerically. A revert to the bare slider fails the second assertion.
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

import { canvasFill } from "../src/terminal/canvas-fill.ts";

const root = new URL("../", import.meta.url);
const read = (path: string) => readFile(new URL(path, root), "utf8");
const stripComments = (css: string) => css.replace(/\/\*[\s\S]*?\*\//g, "");

/** The `:root` token block of base.css, up to the light-theme override. */
const rootBlock = async () => {
  const base = stripComments(await read("src/styles/base.css"));
  return base.slice(base.indexOf(":root {"), base.indexOf('[data-theme="light"]'));
};

const number = (block: string, name: string) => {
  const match = block.match(new RegExp(`--${name}:\\s*([\\d.]+);`));
  assert.ok(match, `--${name} must be defined in base.css`);
  return Number(match![1]);
};

test("the terminal canvas alpha is the frame fill, not the bare transparency slider", async () => {
  const root = await rootBlock();
  const midFill = number(root, "glass-step-fill");
  const solidTop = number(root, "glass-solid-top");
  const terminalTransparency = number(root, "terminal-opacity");

  // The composition itself: fill + (solidTop − fill) × transparency.
  assert.equal(canvasFill(midFill, solidTop, 1), solidTop, "100% must reach the near-solid top");
  assert.equal(canvasFill(midFill, solidTop, 0), midFill, "0% must sit on the step's own floor");
  assert.equal(
    canvasFill(midFill, solidTop, 0.5),
    midFill + (solidTop - midFill) * 0.5,
    "the slider must interpolate linearly between floor and top",
  );

  // The shipped mid step (fill 0.68, top 0.98) makes the two ends concrete.
  assert.equal(canvasFill(0.68, 0.98, 1), 0.98);
  assert.equal(canvasFill(0.68, 0.98, 0), 0.68);

  // Decoupling, measured: the canvas alpha is not the slider value. Before R8
  // the renderer read `--terminal-opacity` straight into the pixel; that is
  // the mutation this assertion kills. At the default 0.46 the frame fill is
  // 0.818, which is nowhere near 0.46.
  const painted = canvasFill(midFill, solidTop, terminalTransparency);
  assert.notEqual(
    painted,
    terminalTransparency,
    "the canvas must paint the frame fill, not the raw transparency slider",
  );
  assert.ok(
    painted > terminalTransparency,
    `the frame fill (${painted.toFixed(3)}) must be more solid than the slider (${terminalTransparency}) — ` +
      "the step supplies the material, the slider only moves within it",
  );

  // The step moves the floor: Clear and Regular paint different alphas at the
  // same slider position. A canvas that read only the slider could not.
  const lowFill = canvasFill(0.3, solidTop, terminalTransparency);
  assert.notEqual(lowFill, painted, "the material step must reach the canvas's fill");
  assert.ok(lowFill < painted, "the Clear step's canvas must be thinner than Regular's");
});

// The pure function above only proves the arithmetic is right *if* the
// renderer calls it. Before this assertion, reverting `render.ts`'s bgOpacity
// to `cssNumber(style, "--terminal-opacity", 1)` — the pre-R8 bare-slider
// behaviour — left the suite green, because the test was evaluating a function
// the renderer no longer had to use. This pins the wiring: the renderer reads
// the three tokens and composes them through `canvasFill`.
test("the renderer composes the canvas fill from the three shipped tokens", async () => {
  // Comments and the doc block name both `--terminal-opacity` and the step
  // tokens for explanation; strip them so the assertion is about the code.
  const render = (await read("src/terminal/render.ts")).replace(/\/\/[^\n]*/g, "");
  assert.match(render, /import \{ canvasFill \} from "\.\/canvas-fill"/, "render.ts must import the shared composition");
  for (const name of ["--glass-step-fill", "--glass-solid-top", "--terminal-opacity"]) {
    assert.ok(
      render.includes(`cssNumber(style, "${name}"`),
      `render.ts must read ${name} when resolving the canvas fill`,
    );
  }
  assert.match(
    render,
    /this\.bgOpacity = canvasFill\(/,
    "bgOpacity must be the composed frame fill, not a bare slider read",
  );
  assert.ok(
    !/bgOpacity = cssNumber\(style, "--terminal-opacity"/.test(render),
    "the canvas must not paint the raw transparency slider — that is the pre-R8 behaviour",
  );
});
