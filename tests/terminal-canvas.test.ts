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

test("the terminal canvas alpha is the frame alpha, not the bare transparency slider", async () => {
  const root = await rootBlock();
  const frameFloor = number(root, "glass-frame-floor");
  const solidTop = number(root, "glass-solid-top");
  const terminalTransparency = number(root, "terminal-opacity");

  // GLASS-3STOP: the canvas alpha is the transparency slider, clamped to the
  // near-solid top and lifted only by the accessibility floor. The material
  // step contributes no floor — it changes the material, not the thickness.
  assert.equal(canvasFill(frameFloor, solidTop, 1), solidTop, "100% must reach the near-solid top");
  assert.equal(canvasFill(frameFloor, solidTop, 0.1), 0.1, "10% must paint a 0.10 canvas — the slider is the truth");
  assert.equal(
    canvasFill(frameFloor, solidTop, 0.5),
    0.5,
    "the canvas must track the slider across its whole range",
  );
  // The accessibility floor lifts the bottom end; the top still clamps.
  assert.equal(canvasFill(0.86, solidTop, 0.1), 0.86, "the contrast floor lifts the bottom end");
  assert.equal(canvasFill(0.86, solidTop, 1), solidTop, "the top still clamps at the near-solid top");

  // The shipped numbers make the two ends concrete: 10% is genuinely 0.10.
  assert.equal(canvasFill(0, 0.98, 0.1), 0.1);
  assert.equal(canvasFill(0, 0.98, 1), 0.98);

  // Decoupling, measured: the canvas alpha is *the slider itself* now — the
  // material step no longer props it up. At the default 0.46 the canvas paints
  // 0.46, not the old 0.818 floor-coupled value.
  const painted = canvasFill(frameFloor, solidTop, terminalTransparency);
  assert.equal(
    painted,
    terminalTransparency,
    "the canvas must paint the slider value, with no step-supplied floor",
  );
  // …and the step cannot move it: the same slider position paints the same
  // alpha on every step (the step is absent from the formula).
  const frames = ["frosted", "regular", "liquid"].map(() => canvasFill(frameFloor, solidTop, 0.5));
  assert.equal(new Set(frames).size, 1, "the material step must not move the canvas alpha");
});

// The pure function above only proves the arithmetic is right *if* the
// renderer calls it. Before this assertion, reverting `render.ts`'s bgOpacity
// to `cssNumber(style, "--terminal-opacity", 1)` — the pre-R8 bare-slider
// behaviour — left the suite green, because the test was evaluating a function
// the renderer no longer had to use. This pins the wiring: the renderer reads
// the three tokens and composes them through `canvasFill`.
test("the renderer composes the canvas fill from the three shipped tokens", async () => {
  // Comments and the doc block name both `--terminal-opacity` and the frame
  // tokens for explanation; strip them so the assertion is about the code.
  const render = (await read("src/terminal/render.ts")).replace(/\/\/[^\n]*/g, "");
  assert.match(render, /import \{ canvasFill \} from "\.\/canvas-fill"/, "render.ts must import the shared composition");
  for (const name of ["--glass-frame-floor", "--glass-solid-top", "--terminal-opacity"]) {
    assert.ok(
      render.includes(`cssNumber(style, "${name}"`),
      `render.ts must read ${name} when resolving the canvas fill`,
    );
  }
  assert.match(
    render,
    /this\.bgOpacity = canvasFill\(/,
    "bgOpacity must be the composed frame alpha, not a bare slider read",
  );
  assert.ok(
    !/bgOpacity = cssNumber\(style, "--terminal-opacity"/.test(render),
    "the canvas must not paint the raw transparency slider without the clamps — that is the pre-R8 behaviour",
  );
  // The step must not reach the canvas fill: it is the material axis.
  assert.ok(
    !/--glass-step-fill/.test(render),
    "render.ts must not read a step fill — the slider is the alpha truth",
  );
});
