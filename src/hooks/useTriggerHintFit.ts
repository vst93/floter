import { useLayoutEffect, useState } from "react";

import { triggerHintFits } from "../launcher/trigger-hint.ts";

/**
 * R48 · keep the inline trigger nudge from surviving as a stub.
 *
 * The hint rides the field row (see `.collapsed-card__trigger-hint`): the field
 * is the grower (`flex: 1 1 0`) and the nudge is the shrinkable piece, so a row
 * that cannot hold both takes the space out of the nudge. CSS ellipsises that,
 * but a hint with two glyphs left is no longer a nudge — the App hides it
 * instead, and the decision is a *measurement*, because a stylesheet cannot ask
 * how much room a flex item kept after the field took its share (see
 * `launcher/trigger-hint.ts` for the rule itself).
 *
 * The measurement runs in a layout effect, before the browser paints, so the
 * hide/show never flashes. When the element already carries `display: none`
 * from the previous pass the effect clears it for that one pass — React
 * re-applies the state before paint — so a hint that became readable again is
 * measured at its real width rather than at zero.
 *
 * `text` is the rendered hint (a string, so the effect re-runs when the nudge's
 * *contents* change and not on every render's fresh object); `scaleKey` is the
 * interface step, which moves both the caption size and the row's insets, so a
 * step change re-measures even when ref and text stand still.
 */
export function useTriggerHintFit(
  ref: React.RefObject<HTMLElement | null>,
  text: string | null,
  scaleKey: string | number,
): boolean {
  const [visible, setVisible] = useState(true);

  useLayoutEffect(() => {
    if (!text) return;
    const element = ref.current;
    if (!element) return;
    // One pre-paint layout pass at the hint's real width, even if the last
    // measurement left it `display: none`.
    element.style.display = "";
    const next = triggerHintFits(
      element.getBoundingClientRect().width,
      cellWidthOf(element),
    );
    if (next === visible) {
      // No state change means React will not re-render (and so will not
      // re-apply the style we just cleared), so restore it by hand.
      element.style.display = next ? "" : "none";
      return;
    }
    setVisible(next);
    // `visible` is deliberately not a dependency: the effect reads it as the
    // value this pass rendered with, and re-running it on its own write would
    // loop.
  }, [ref, text, scaleKey]);

  return text !== null && visible;
}

/** The advance width of one `ch` in the element's own font. `ch` is a CSS unit
 *  but the floor is compared against a measured box, so it has to be read as
 *  pixels; the canvas is the one measurer available without a probe in the DOM.
 *  The half-em fallback keeps the rule alive if the canvas is unavailable. */
const cellWidthOf = (element: Element): number => {
  const style = getComputedStyle(element);
  const size = Number.parseFloat(style.fontSize);
  const fallback = Number.isFinite(size) && size > 0 ? size * 0.5 : 0;
  const context = document.createElement("canvas").getContext("2d");
  if (!context) return fallback;
  context.font = `${style.fontStyle} ${style.fontWeight} ${style.fontSize} ${style.fontFamily}`;
  const measured = context.measureText("0").width;
  return measured > 0 ? measured : fallback;
};
