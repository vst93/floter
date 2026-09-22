import { useLayoutEffect, useRef } from "react";
import { getCurrentWindow, LogicalSize } from "@tauri-apps/api/window";
import { reassertCollapsedFocus } from "../collapsed-focus.ts";
import { INPUT_WINDOW_WIDTH } from "../window-contract.ts";
import { LAUNCHER_WINDOW_HEIGHT } from "../launcher/result-budget.ts";

/**
 * Hold the launcher window at the collapsed card's height.
 *
 * R25 · the height is a **constant**, not a measurement. Until this round the
 * hook re-measured the card on every dependency change (the result count, the
 * action bar, the feedback row) and called `setSize` with whatever came out —
 * so every keystroke that changed how many rows matched resized the native
 * window, and the ResizeObserver's R20 settle pass could fire two or three
 * times inside one keystroke. The user's report is that shake: 「现在搜索页面
 * 输入进行过滤时页面整体有抖动的情况，不像原生应用」. The window now holds a
 * budget height (`LAUNCHER_WINDOW_HEIGHT`, or a shorter R26-D band) while the
 * launcher is open: typing, clearing the query, a tip appearing, a feedback row
 * arriving — none of them may move the window *within a band*. What moves is
 * the list inside it, which scrolls.
 *
 * R26-D · the height is a **band**, not one fixed slab. The caller hands the
 * band's height (App.tsx resolves it from the row count, with hysteresis), so
 * this hook still never measures to *decide*: a keystroke that keeps the same
 * band hands back the same number and the guards below turn it into a no-op. A
 * band crossing is one deliberate `setSize`.
 *
 * The measurement survives as the one thing a band cannot express: a card whose
 * content genuinely outgrew the window it is drawn in (a first-run tip above a
 * full list, a font landing late) must not be clipped. The target is therefore
 * the band's height, raised only when the measurement exceeds the *current*
 * window — see {@link launcherTargetHeight}.
 *
 * `windowHeight` is the caller's band at its interface step, already clamped to
 * the display: it is a *height*, not a step, so this hook never multiplies
 * anything — a measurement is read back in the pixels the browser laid out (see
 * the step tests in the node suite).
 */
export function useLauncherHeight(
  mode: string,
  collapsedCardRef: React.RefObject<HTMLDivElement | null>,
  windowHeight: number,
  dependencies: React.DependencyList,
) {
  // The height this hook last asked the window for. Only the observer reads it
  // (see below); the effect's own sync goes through the shared pending guard.
  const applied = useRef(0);

  useLayoutEffect(() => {
    if (mode !== "collapsed") return;
    const card = collapsedCardRef.current;
    if (!card) return;
    syncLauncherHeight(collapsedCardRef, windowHeight);
    if (typeof ResizeObserver === "undefined") return;

    // R20 · the dependency list above is the row *count*; a row can change
    // height while the count stands still (a compact row gaining a subtitle
    // because the query reached a command, a font landing late and reflowing
    // the list). Watching the card's own box is what keeps the window's content
    // from being clipped in those states — and it costs nothing now: the target
    // it computes is the same constant the window already carries, so the
    // callback returns before it reaches `setSize` (R25).
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
      const target = launcherTargetHeight(current, windowHeight);
      if (!target) return;
      if (Math.abs(target - window.innerHeight) <= 1) return;
      if (target === applied.current) return;
      applied.current = target;
      resizeLauncherWindow(current, target, SETTLE_PASSES);
    });
    observer.observe(card);
    return () => observer.disconnect();
  }, [mode, collapsedCardRef, windowHeight, ...dependencies]);
}

/**
 * Ask the window for the launcher's height.
 *
 * Extracted from App.tsx so the sizing logic lives in one place and the
 * measurement can be called imperatively (after `show_input` completes, on a
 * reveal) or declaratively (the layout effect above). Every path ends in the
 * same guarded call, so a second ask inside one keystroke cannot reach the
 * platform twice.
 *
 * `windowHeight` defaults to the constant at the default step: the only caller
 * that omits it is a node test, and a launcher-height window is a better
 * fallback than no resize at all.
 */
export function syncLauncherHeight(
  collapsedCardRef: React.RefObject<HTMLDivElement | null>,
  windowHeight: number = LAUNCHER_WINDOW_HEIGHT,
) {
  const card = collapsedCardRef.current;
  if (!card) return;

  const target = launcherTargetHeight(card, windowHeight);
  if (!target) return;

  applyLauncherHeight(card, target);
}

/**
 * The launcher's window height: the budget constant, and never less than the
 * card's own content.
 *
 * R25 · the constant is the answer in every state the sheets can draw; the
 * measurement is the guard for the states they cannot (see the note on the
 * hook).
 *
 * R26-D · the measurement is now an *overflow* guard only. The card fills the
 * window (`height: 100%`), so `measureCardHeight` returns the current window
 * height in every ordinary state; comparing that with the budget (R25's
 * `Math.max(windowHeight, measured)`) meant the window could never shrink — the
 * card always “measured” as tall as the slab it was drawn in. What genuinely
 * means “the content outgrew the window” is the measurement exceeding the
 * window that is *currently* showing, so the guard raises only then. Otherwise
 * the caller's height — the R26-D band, or the full slab — is the answer, and a
 * shorter band may step the window down.
 *
 * The shell's reservation is added to the caller's height rather than being part
 * of it: Windows and Linux pad `.collapsed-shell` so the card's box-shadow has
 * somewhere to land (macOS does not), and that padding comes out of the
 * window's height *before* the card sees any of it. It is a constant of the
 * platform's sheet — the same number every frame — which is why it can sit on
 * the target side without the window moving.
 */
const launcherTargetHeight = (card: HTMLElement, windowHeight: number): number => {
  const base = windowHeight + shellPaddingHeight(card);
  const measured = measureCardHeight(card);
  const current = currentWindowHeight();
  if (Number.isFinite(current) && measured > current + 1) {
    return Math.max(base, measured);
  }
  return base;
};

/**
 * The height a launcher resize has asked the platform for, while that request
 * is still in flight. Two asks inside one keystroke (the mode effect's sync and
 * the layout effect's, the show_input completion and the reveal's) must not
 * both reach `setSize`: the first one is on its way, and the window it will
 * leave behind is the height the second one is asking for. Cleared when the
 * resize lands, so a *later* ask — a display change, a settings round trip that
 * resized the window behind the launcher — is never mistaken for a duplicate.
 */
let pendingLauncherHeight = 0;

/**
 * Land one launcher height, or return without touching the window.
 *
 * This is the R25 guard, and it is where the shake dies: the window already
 * carries the height (the constant, and the card is drawn inside it), so a
 * keystroke's call returns here instead of calling `setSize`. A native window
 * height cannot be read synchronously from the WebView without a round trip,
 * so the check is against `window.innerHeight`, which is updated by the
 * platform when a resize lands.
 */
function applyLauncherHeight(card: HTMLElement, height: number) {
  if (Math.abs(height - currentWindowHeight()) <= 1) return;
  if (height === pendingLauncherHeight) return;
  pendingLauncherHeight = height;
  void resizeLauncherWindow(card, height, SETTLE_PASSES).then(() => {
    if (pendingLauncherHeight === height) pendingLauncherHeight = 0;
  });
}

/** The window's height in logical pixels, or `NaN` outside a WebView (the node
 *  suite drives this module directly). A `NaN` never satisfies the "already
 *  there" guard, which is the behaviour a caller with no window should get. */
const currentWindowHeight = (): number =>
  typeof window === "undefined" ? NaN : window.innerHeight;

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
  return height + shellPaddingHeight(card);
}

/**
 * The vertical room the shell around the card reserves for the card's shadow,
 * in window pixels.
 *
 * Read off the element rather than spelled in TypeScript, because it is the
 * sheet's number per platform (`base.css`: `.platform-windows .collapsed-shell`
 * and `.platform-linux .collapsed-shell`; macOS reserves nothing). It is a
 * constant of the platform, not a measurement of the card — which is what lets
 * {@link launcherTargetHeight} add it to the budget without the window moving.
 */
function shellPaddingHeight(card: HTMLElement): number {
  const shell = card.parentElement;
  if (!shell) return 0;
  const shellStyle = getComputedStyle(shell);
  return (
    (parseFloat(shellStyle.paddingTop) || 0) +
    (parseFloat(shellStyle.paddingBottom) || 0)
  );
}

/**
 * Ask the native window for `height` and, once that has landed, re-check it.
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
 *
 * R25 · the pass re-uses {@link launcherTargetHeight}, so it can only *raise*
 * the window (to a card that outgrew the budget). The ordinary case — the card
 * the window was just sized for — settles back to the height that was asked
 * for and the pass ends without a second `setSize`. This is the R15 logic kept
 * intact on the one path where a non-constant height is still possible.
 */
function resizeLauncherWindow(card: HTMLElement, height: number, passes: number): Promise<void> {
  return getCurrentWindow()
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
        const settled = launcherTargetHeight(card, height);
        if (settled && settled !== height) resizeLauncherWindow(card, settled, passes - 1);
      });
    })
    .catch(() => undefined);
}
