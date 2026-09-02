import { useLayoutEffect } from "react";
import { getCurrentWindow, LogicalSize } from "@tauri-apps/api/window";

const INPUT_WINDOW_WIDTH = 720;

/**
 * Sync the launcher window's height to the collapsed card's measured content.
 *
 * Called on every result/feedback/actionBar change so the window stays exactly
 * as tall as the rows inside it — no prediction, just measurement. Offsets
 * rather than getBoundingClientRect because the shell plays a scale animation
 * on entry and a rect measured mid-animation is scaled by 0.986.
 */
export function useLauncherHeight(
  mode: string,
  collapsedCardRef: React.RefObject<HTMLDivElement | null>,
  dependencies: React.DependencyList,
) {
  useLayoutEffect(() => {
    if (mode !== "collapsed") return;
    syncLauncherHeight(collapsedCardRef);
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
  const last = card?.lastElementChild as HTMLElement | null;
  if (!card || !last) return;

  const style = getComputedStyle(card);
  const frame =
    (parseFloat(style.borderTopWidth) || 0) +
    (parseFloat(style.borderBottomWidth) || 0) +
    (parseFloat(style.paddingBottom) || 0);

  let height = Math.ceil(last.offsetTop + last.offsetHeight + frame);
  if (!height) return;

  // The shell wrapping the card may carry padding (Windows uses it to give
  // the CSS box-shadow room outside the card), and the window has to be
  // that much taller for the padding to actually show.
  const shell = card.parentElement;
  if (shell) {
    const shellStyle = getComputedStyle(shell);
    height +=
      (parseFloat(shellStyle.paddingTop) || 0) +
      (parseFloat(shellStyle.paddingBottom) || 0);
  }

  getCurrentWindow()
    .setSize(new LogicalSize(INPUT_WINDOW_WIDTH, height))
    .catch(() => undefined);
}
