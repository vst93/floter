import { useLayoutEffect, useRef } from "react";
import { getCurrentWindow, LogicalSize } from "@tauri-apps/api/window";
import { reassertCollapsedFocus } from "../collapsed-focus.ts";
import { INPUT_WINDOW_WIDTH } from "../window-contract.ts";

/**
 * Sync the launcher window's height to the collapsed card's measured content.
 *
 * Called on every result/feedback/actionBar change so the window stays exactly
 * as tall as the rows inside it — no prediction, just measurement. Offsets
 * rather than getBoundingClientRect because the shell plays a scale animation
 * on entry and a rect measured mid-animation is scaled by 0.986.
 *
 * R20 · the same "no prediction" rule now covers the case the dependency list
 * could not: a row that changes height while the row count stands still. The
 * hook watches the card itself, so the window follows the card's measured
 * content rather than the events that are supposed to imply it.
 */
export function useLauncherHeight(
  mode: string,
  collapsedCardRef: React.RefObject<HTMLDivElement | null>,
  dependencies: React.DependencyList,
) {
  // The height this hook last asked the window for. Only the observer reads it
  // (see below); the effect's own sync always runs.
  const applied = useRef(0);

  useLayoutEffect(() => {
    if (mode !== "collapsed") return;
    const card = collapsedCardRef.current;
    syncLauncherHeight(collapsedCardRef);
    if (!card || typeof ResizeObserver === "undefined") return;

    // R20 · the dependency list above is the row *count*; the window's height is
    // the rows' *height*, and the two do not move together. Every state that
    // changes a row's box without changing how many rows there are — a compact
    // row gaining a subtitle because the query reached a command, the
    // onboarding tip arriving or being dismissed, a font landing late and
    // reflowing the list — leaves the window at the height of the *previous*
    // list. The card is `min-height: 100%` inside a centred shell, so a card
    // taller than its window overflows at *both* ends: the field loses its top
    // edge (the user's 「输入框高度异常」 — the field's 56u (48u since R22) row
    // rendered as ~52px of
    // visible band) and the action bar is cut by the window's bottom edge
    // (「界面边框又变形了」). No dependency list can enumerate "a row got
    // taller", so the trigger stops being a prediction and becomes a
    // measurement: watch the card's own box and re-apply the measured height
    // whenever it moves.
    //
    // Two guards, both about not resizing the window against itself:
    //   * the target is compared with the window's *current* height rather than
    //     with a remembered number, so the very first callback after a sync is a
    //     no-op and a genuine external resize (the native reveal path sets a
    //     baseline of its own) is still corrected; and
    //   * a target we have already asked for is never asked for twice, so an
    //     observer that fires on the resize it caused terminates instead of
    //     oscillating against a platform that lands a pixel or two away.
    const observer = new ResizeObserver(() => {
      const current = collapsedCardRef.current;
      if (!current) return;
      const target = measureCardHeight(current);
      if (!target) return;
      if (Math.abs(target - window.innerHeight) <= 1) return;
      if (target === applied.current) return;
      applied.current = target;
      resizeLauncherWindow(current, target, SETTLE_PASSES);
    });
    observer.observe(card);
    return () => observer.disconnect();
  }, [mode, collapsedCardRef, ...dependencies]);
}

/**
 * Measure the card's laid-out content and resize the window to match.
 *
 * Extracted from App.tsx so the sizing logic lives in one place and the
 * measurement can be called imperatively (after show_input completes) or
 * declaratively (useLayoutEffect on result count changes).
 */
export function syncLauncherHeight(
  collapsedCardRef: React.RefObject<HTMLDivElement | null>,
) {
  const card = collapsedCardRef.current;
  if (!card) return;

  const height = measureCardHeight(card);
  if (!height) return;

  resizeLauncherWindow(card, height, SETTLE_PASSES);
}

/**
 * How many extra resizes may follow a settled one. Two is enough to absorb a
 * late layout (a font landing, a native resize re-entering the WebView) and
 * bounded, so a card that keeps changing size can never loop forever.
 */
const SETTLE_PASSES = 2;

/** Run after the next paint. The fallback keeps the helper drivable outside a
 *  WebView (the node suite has no `requestAnimationFrame`), and the beat is one
 *  frame either way. */
const afterPaint = (run: () => void) => {
  if (typeof requestAnimationFrame === "function") requestAnimationFrame(run);
  else setTimeout(run, 16);
};

/**
 * Measure the card's laid-out content, in window pixels.
 *
 * The card keeps a display:none plugin host mounted after the launcher content.
 * Walk backwards to the last child that participates in layout so the hidden
 * host is ignored while the collapsed result clip can still contribute its real
 * offset (zero when there are no results).
 */
function measureCardHeight(card: HTMLElement): number {
  const last = Array.from(card.children)
    .reverse()
    .find((child) => getComputedStyle(child).display !== "none") as HTMLElement | undefined;
  if (!last) return 0;

  const style = getComputedStyle(card);
  const frame =
    (parseFloat(style.borderTopWidth) || 0) +
    (parseFloat(style.borderBottomWidth) || 0) +
    (parseFloat(style.paddingBottom) || 0);

  const height = Math.ceil(last.offsetTop + last.offsetHeight + frame);
  if (!height) return 0;

  // The shell wrapping the card may carry padding (Windows uses it to give
  // the CSS box-shadow room outside the card), and the window has to be
  // that much taller for the padding to actually show.
  const shell = card.parentElement;
  if (!shell) return height;
  const shellStyle = getComputedStyle(shell);
  return (
    height +
    (parseFloat(shellStyle.paddingTop) || 0) +
    (parseFloat(shellStyle.paddingBottom) || 0)
  );
}

/**
 * Ask the native window for `height` and, once that has landed, re-measure.
 *
 * R15 · the resize is asynchronous and the card is `overflow: hidden`, so
 * between the DOM change that grew the card (a keystroke adding a row, the
 * action bar appearing) and the native resize landing, the card is clipped by
 * the window's *old* bottom edge — the action bar cut through the middle of
 * its own text, which is the user's report (「动不动页面布局就崩了」). One
 * corrective pass after the resize settles closes that window, and it is a
 * re-measure rather than a prediction: if the content really did change again,
 * the second measurement is the right one; if it did not, the pass is a no-op
 * and nothing is resized twice.
 */
function resizeLauncherWindow(card: HTMLElement, height: number, passes: number) {
  getCurrentWindow()
    .setSize(new LogicalSize(INPUT_WINDOW_WIDTH, height))
    // A native resize can move WebView keyboard focus to the document body as
    // it settles. The resize is the last thing to land when returning to the
    // launcher, so the focus collector is re-run the instant it completes —
    // this is what closes the "focused, then dropped by the layout that was
    // still in flight" hole (see `collapsed-focus.ts`).
    .then(() => {
      reassertCollapsedFocus();
      if (passes <= 0) return;
      afterPaint(() => {
        const settled = measureCardHeight(card);
        if (settled && settled !== height) resizeLauncherWindow(card, settled, passes - 1);
      });
    })
    .catch(() => undefined);
}
