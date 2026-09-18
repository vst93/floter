// The terminal canvas's background alpha, as a pure function.
//
// The canvas is a bitmap, so it cannot carry a `rgba()` tint like the CSS
// surfaces do: the renderer has to paint a single translucent colour over the
// tinted glass frame underneath it. GLASS-3STOP made that colour the
// *transparency slider itself*, clamped to the near-solid top — the same
// readability control the launcher's frame uses. The glass step no longer
// contributes a floor, because a step changes the material, never the frame's
// thickness.
//
// Kept in its own module, free of DOM and canvas types, so the arithmetic can
// be unit-tested directly. `render.ts` reads the three inputs out of CSS
// (`--glass-frame-floor`, `--glass-solid-top`, `--terminal-opacity`) and calls
// this; the numbers stay in base.css and only the composition lives here.

/**
 * The frame's alpha: the transparency slider, clamped to the near-solid top
 * and lifted only by the accessibility floor.
 *
 * `floor` is 0 in normal use — at `transparency = 0.1` the canvas is 0.10,
 * exactly what the slider says — and the `prefers-contrast: more` override
 * raises it to 0.86. It is deliberately *not* the step's fill: the step is the
 * effect axis (blur / saturation / lens / haze), so the slider alone decides
 * how solid the frame is. This mirrors base.css's
 * `min(solid_top, max(frame_floor, opacity))`.
 */
export const canvasFill = (floor: number, solidTop: number, transparency: number): number =>
  Math.min(solidTop, Math.max(floor, transparency));
