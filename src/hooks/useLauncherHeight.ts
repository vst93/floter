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
      // R68 · while the walk owns the edge, the observer must not join in. The
      // walk resizes the card every frame, so the observer fires every frame;
      // without this guard each of those fires starts a *second* walk to the
      // same target, cancelling the one in flight and restarting it from the
      // mid-way height. The walk's own landing brings the observer back (the
      // last frame clears `tweenFrame` before the resize lands), so a genuine
      // overflow is still caught.
      if (tweenFrame) return;
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
 * R68 · how long one window height change takes, in milliseconds.
 *
 * A native window resize teleports: the bottom edge is at the old height this
 * frame and at the new one the next. R67 removed a walk because the walk itself
 * was the flicker — every `setSize` repainted the WebView and made the window
 * server recompute the panel's shadow — but both of those have since been dealt
 * with (the shadow is debounced in `src-tauri`, and the walk marks the card so
 * the sheets freeze the material for its duration), and the list no longer
 * fights it (the ⌘-held row is never scrolled into view against a stale box). So
 * the edge walks again, and now the motion is the only thing left to see.
 *
 * It is a *visual* constant like the `--dur-*` steps (a duration), not a layout
 * one, and it is deliberately under the ~200ms the catalog search already spends
 * before a query becomes a list.
 */
export const RESIZE_TWEEN_MS = 150;

/** The tween's clock. `performance.now()` where it exists, `Date.now()` where it
 *  does not (the node suite), so the beat is drivable either way. */
const nowMs = (): number =>
  typeof performance !== "undefined" && typeof performance.now === "function"
    ? performance.now()
    : Date.now();

/** The height the tween last painted. A retarget starts from here rather than
 *  from `window.innerHeight`, which only catches up when the platform lands the
 *  previous frame's resize — starting from it would make a mid-walk change jump
 *  back to where the edge was two frames ago. */
let tweenHeight = Number.NaN;
let tweenFrame = 0;
let tweenGeneration = 0;
let tweenDone: (() => void) | null = null;

/** Stop the walk in flight, if any. The pending promise is resolved (not
 *  dropped) so its `.then` still runs the focus reassert; the caller that
 *  retargeted starts its own walk immediately after. */
const cancelTween = () => {
  // The generation makes a cancelled walk's callbacks inert: its pending
  // `setSize` may still resolve after the next walk has been armed.
  tweenGeneration += 1;
  if (tweenFrame) {
    if (typeof cancelAnimationFrame === "function") cancelAnimationFrame(tweenFrame);
    tweenFrame = 0;
  }
  const done = tweenDone;
  tweenDone = null;
  done?.();
};

/**
 * R68 · the card's "an edge walk is in flight" marker, and what it buys.
 *
 * A window mid-walk is smaller than the content it is growing into, so the
 * result scroller would overflow for a frame and paint its bar, and the card's
 * `backdrop-filter` would re-sample the desktop behind it every frame. So while
 * the walk owns the edge the card carries this class and the sheets (a) keep the
 * scrollers from painting a bar and (b) take the reduced-transparency block's own
 * swap — the blur goes, the card and its panel go near-solid — leaving nothing
 * behind the content to re-sample. It is a *visual* marker like the tween's
 * duration, never a layout one: nothing it does changes a height.
 */
const RESIZING_CLASS = "launcher-resizing";

const markResizing = (card: HTMLElement, on: boolean) => {
  card.classList.toggle(RESIZING_CLASS, on);
};

/**
 * Land a window height, walking the edge there over {@link RESIZE_TWEEN_MS} with
 * an ease-out curve (`1 - (1-t)³`: quick off the mark, soft at the landing).
 *
 * The walk needs a document to paint in; where there is none (the node suite) or
 * `requestAnimationFrame` is missing, the height lands in one call — the
 * function's contract is the same either way: the window ends at `height`.
 */
function tweenLauncherHeight(card: HTMLElement, height: number): Promise<void> {
  const platform = getCurrentWindow();
  const from = Number.isFinite(tweenHeight) ? tweenHeight : currentWindowHeight();
  const land = () => {
    tweenHeight = height;
    return platform.setSize(new LogicalSize(INPUT_WINDOW_WIDTH, height));
  };
  if (
    typeof document === "undefined" ||
    typeof requestAnimationFrame !== "function" ||
    !Number.isFinite(from) ||
    Math.abs(height - from) <= 1
  ) {
    cancelTween();
    return land();
  }
  cancelTween();
  const generation = tweenGeneration;
  markResizing(card, true);
  return new Promise<void>((resolve) => {
    const start = nowMs();
    tweenDone = resolve;
    const step = () => {
      const t = Math.min(1, (nowMs() - start) / RESIZE_TWEEN_MS);
      const eased = 1 - (1 - t) ** 3;
      const next = Math.round(from + (height - from) * eased);
      tweenHeight = next;
      // Wait for each resize to land before painting the next: the platform
      // posts `setContentSize` to the main queue, so two frames in flight at
      // once can land out of order and the edge bounces.
      void platform.setSize(new LogicalSize(INPUT_WINDOW_WIDTH, next)).then(() => {
        if (generation !== tweenGeneration) return;
        if (t < 1) {
          tweenFrame = requestAnimationFrame(step);
        } else {
          tweenFrame = 0;
          tweenHeight = height;
          tweenDone = null;
          resolve();
        }
      });
    };
    tweenFrame = requestAnimationFrame(step);
  }).finally(() => {
    // Only the walk that owns the edge drops the marker: a retarget resolves the
    // old promise on a microtask *after* the new walk is armed, so `tweenFrame`
    // tells the two apart.
    if (!tweenFrame) markResizing(card, false);
  });
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
  // R68 · a short walk, not a jump. R67 had this as one `setSize` because the
  // walk was the flicker; both flicker sources are gone now (the shadow is
  // debounced in `src-tauri`, the material is frozen for the walk, and the list
  // no longer scrolls against a stale box), so the motion is the only thing left
  // to see and a jump reads as a glitch rather than as the panel resizing.
  return tweenLauncherHeight(card, height)
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
