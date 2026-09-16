// The terminal canvas's background alpha, as a pure function.
//
// The canvas is a bitmap, so it cannot carry a `rgba()` tint like the CSS
// surfaces do: the renderer has to paint a single translucent colour over the
// tinted glass frame underneath it. R8 made that colour the *frame fill* — the
// same readability control the launcher's frame uses, applied to the
// terminal's own transparency slider — rather than the raw slider value.
//
// Kept in its own module, free of DOM and canvas types, so the arithmetic can
// be unit-tested directly. `render.ts` reads the three inputs out of CSS
// (`--glass-step-fill`, `--glass-solid-top`, `--terminal-opacity`) and calls
// this; the numbers stay in base.css and only the composition lives here.

/**
 * The frame's composited fill: the material step's own floor, sliding to the
 * near-solid top as the transparency control goes from 0 to 1.
 *
 * At `transparency = 0` the canvas is as thin as the step allows (`fill`); at
 * `transparency = 1` it reaches `solidTop` (0.98). It is deliberately *not*
 * the slider value itself: the step decides the material's floor, so the same
 * slider position paints a different alpha on the Clear and Regular steps.
 */
export const canvasFill = (fill: number, solidTop: number, transparency: number): number =>
  fill + (solidTop - fill) * transparency;
