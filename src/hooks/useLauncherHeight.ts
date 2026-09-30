import { useLayoutEffect } from "react";
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
    // from being clipped in those states.
    //
    // R71 · the callback is one call into the *one* height policy — the same
    // call the layout effect above makes. It used to carry duplicate guards of
    // its own (`applied`, `launcherHeightNeedsResize`), and a second set of
    // rules about the same question is how the observer's memory and the
    // platform's number drift apart: after a resize the frontend never saw, the
    // observer could refuse to restore a height it *had* already asked for, and
    // the surface that needed it stayed clipped (「书签插件就有问题」). The
    // policy's own audit against the platform (see `confirmLauncherHeight`) is
    // the only judgement now.
    const observer = new ResizeObserver(() => {
      const current = collapsedCardRef.current;
      if (!current) return;
      // R68 · while the walk owns the edge, the observer must not join in: the
      // walk resizes the card every frame, and each fire would start a *second*
      // walk to the same target, cancelling the one in flight and restarting it
      // from the mid-way height. The walk's own landing brings the observer
      // back, and R71 guarantees it always lands (see `tweenLauncherHeight`).
      if (tweenFrame) return;
      applyLauncherHeight(current, launcherTargetHeight(current, windowHeight));
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
  // R71 · the overflow guard only speaks for a window that *is* the height it
  // was sized for. Mid-flight — a walk is moving the edge, the WebView is still
  // reporting the previous surface's height, the card is reflowing in between —
  // `measured` and `current` are both transient, and reading an overflow out of
  // two transient numbers is what the user reported as 「窗口高度没有能自动化」:
  // the settle pass below re-targeted whatever the card happened to measure
  // (86 while shrinking, then 90 while growing), the window came to rest at
  // that mid-flight number instead of the plugin list's 563, and *nothing asked
  // again* — the sample and the window now agreed. The band is the answer; the
  // measurement may only raise it once the window is really at the band.
  const atBand = Number.isFinite(current) && Math.abs(current - base) <= HEIGHT_TOLERANCE;
  if (atBand && measured > current + 1) {
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
 *
 * R71 · and it must *always* be released. Entering a plugin mode asks for the
 * list's height while a walk is often already in flight (the mode entry, the
 * reveal, the previous surface), and a walk whose `setSize` never came back — a
 * throttled frame in a panel that is not the active app, a rejected platform
 * call — used to leave this claim set forever. Every later ask for that same
 * height was then silently dropped (the guard below), which is one more way for
 * the window to end up at a surface's height instead of the content's. The walk
 * now always settles — see [`tweenLauncherHeight`] — and the claim is released
 * on the same path that satisfies it.
 */
let pendingLauncherHeight = 0;

/**
 * How close two heights count as the same height.
 *
 * A native resize can land a logical pixel away from the value it was asked for
 * (the platform rounds between physical and logical units), so equality is never
 * exact. Two pixels is under a tenth of a row: it can never hide a real
 * difference between two surfaces' heights (a plugin list against a bare field
 * is tens to hundreds of pixels), and it is enough that the same honest answer
 * never has to be re-derived.
 */
const HEIGHT_TOLERANCE = 2;

/**
 * Land one launcher height, or return without touching the window.
 *
 * This is the R25 guard, and it is where the shake dies: the window already
 * carries the height (the constant, and the card is drawn inside it), so a
 * keystroke's call returns here instead of calling `setSize`.
 *
 * R71 · the "already carries it" answer is *audited* rather than believed —
 * see [`confirmLauncherHeight`]. It is the one branch where a wrong answer costs
 * the user the height they asked for, and `window.innerHeight` is not the window
 * (it is the WebView's viewport, which lags a native resize and can miss one
 * altogether).
 */
function applyLauncherHeight(card: HTMLElement, height: number) {
  if (!Number.isFinite(height) || height <= 0) return;
  if (Math.abs(height - currentWindowHeight()) <= HEIGHT_TOLERANCE) {
    // R72 · the WebView says the window is already there, and that is where the
    // decision ends. R71 audited this branch against the platform (`innerSize()`
    // + `scaleFactor()`) and corrected a disagreement — two IPC round trips on
    // every check, and a correction that walks the window, which re-enters this
    // function through the card observer. On a fast typist that is a feedback
    // loop: the user reported it as 「搜索键入时非常卡…甚至都卡住了」. The audit is
    // out; the stale-report case it was written for is handled where it actually
    // bites — the settle pass (`resizeLauncherWindow`) and the viewport cap
    // (`pluginViewRows`) — without a platform round trip.
    return;
  }
  if (height === pendingLauncherHeight) return;
  pendingLauncherHeight = height;
  void resizeLauncherWindow(card, height, SETTLE_PASSES).finally(() => {
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
let tweenWatchdog = 0;

/**
 * R71 · how long a walk may stay in flight before it is declared stuck.
 *
 * The walk is `RESIZE_TWEEN_MS` long and each frame waits for its own
 * `setSize` to land, so a healthy walk finishes in about half this window and
 * the ceiling is only ever reached by a walk that is *stuck*: frames that stop
 * arriving (a WebView that throttles `requestAnimationFrame` because its panel
 * is not the active app) or a `setSize` that is refused. An unsettled walk is
 * not a cosmetic stall. It keeps `tweenFrame` set (which
 * mutes the card's ResizeObserver, the one thing that corrects a height the
 * window missed) and it keeps [`pendingLauncherHeight`] claimed, so every later
 * ask for that same height is dropped. That is the wedge behind the user's
 * report — a plugin list whose window never grew to hold it. The watchdog ends
 * the walk at its target and hands the edge back.
 */
const RESIZE_WATCHDOG_MS = RESIZE_TWEEN_MS * 2 + 100;

/** Stop the walk in flight, if any. The pending promise is resolved (not
 *  dropped) so its `.then` still runs the focus reassert; the caller that
 *  retargeted starts its own walk immediately after. */
const cancelTween = () => {
  // The generation makes a cancelled walk's callbacks inert: its pending
  // `setSize` may still resolve after the next walk has been armed.
  tweenGeneration += 1;
  if (tweenWatchdog) {
    clearTimeout(tweenWatchdog);
    tweenWatchdog = 0;
  }
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
    // R71 · the walk's one exit. Every path that ends it — the last frame, a
    // refused `setSize`, the watchdog — lands the height, releases the frame
    // slot and settles the promise, so `resizeLauncherWindow`'s `.then` (and
    // with it the `pendingLauncherHeight` release) always runs.
    const settle = () => {
      if (tweenWatchdog) {
        clearTimeout(tweenWatchdog);
        tweenWatchdog = 0;
      }
      tweenFrame = 0;
      tweenHeight = height;
      tweenDone = null;
      resolve();
    };
    tweenDone = resolve;
    // R71 · the watchdog does not merely *declare* the walk over: it lands the
    // height. A walk whose frames stop arriving — a WebView that throttles
    // `requestAnimationFrame` because its panel is not the active app, which is
    // the normal state of this accessory panel — used to leave the window at
    // whatever height the last painted frame reached, and nothing else in the
    // module retries a height that is already "asked for". The result is the
    // user's report: the plugin list rendered inside a card that was never
    // grown to hold it, invisible until some other surface asked for a
    // different height. So the stall resolves by asking the platform for the
    // *target* directly: one `setSize`, no animation, and the edge is where the
    // content needs it.
    tweenWatchdog = setTimeout(() => {
      void platform
        .setSize(new LogicalSize(INPUT_WINDOW_WIDTH, height))
        .catch(() => undefined);
      settle();
    }, RESIZE_WATCHDOG_MS);
    const step = () => {
      const t = Math.min(1, (nowMs() - start) / RESIZE_TWEEN_MS);
      const eased = 1 - (1 - t) ** 3;
      const next = Math.round(from + (height - from) * eased);
      tweenHeight = next;
      // Wait for each resize to land before painting the next: the platform
      // posts `setContentSize` to the main queue, so two frames in flight at
      // once can land out of order and the edge bounces.
      void platform.setSize(new LogicalSize(INPUT_WINDOW_WIDTH, next)).then(
        () => {
          if (generation !== tweenGeneration) return;
          if (t < 1) {
            tweenFrame = requestAnimationFrame(step);
          } else {
            settle();
          }
        },
        // R71 · a refused frame ends the walk instead of stranding it: the
        // window keeps whatever height it has, and the caller is told the walk
        // is over so the next ask is not mistaken for a duplicate of this one.
        () => {
          if (generation === tweenGeneration) settle();
        },
      );
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
  const walk = tweenLauncherHeight(card, height);
  // R71 · the walk's own generation, read while it is the newest one. The
  // corrective pass below belongs to *this* ask, and a pass that outlives it is
  // not a correction but a cancellation: the plugin mode resizes twice in a row
  // (the empty list's own height, then the rows' 563), and the first walk's
  // delayed pass re-measured the *empty* card and walked the window to that
  // number — cancelling the walk that was already carrying the list to 563 and
  // leaving the window at 90 with nothing left to ask, which is exactly the
  // user's report (「窗口高度没有能自动化」). A superseded walk's pass stands down.
  const generation = tweenGeneration;
  return walk
    // A native resize can move WebView keyboard focus to the document body as
    // it settles. The resize is the last thing to land when returning to the
    // launcher, so the focus collector is re-run the instant it completes —
    // this is what closes the "focused, then dropped by the layout that was
    // still in flight" hole (see `collapsed-focus.ts`).
    .then(() => {
      reassertCollapsedFocus();
      if (passes <= 0) return;
      afterPaint(() => {
        // R71 · and the pass is a re-*measure*, not a prediction: it may only
        // speak for a window that really is the height it was given. Mid-flight
        // the card measures whatever the transition happens to be at (86 while
        // the edge shrinks), and both numbers have to agree before an overflow
        // means anything. `height` is already a *window* height, so the measured
        // card — which carries the shell's padding too — is compared with it
        // directly.
        if (generation !== tweenGeneration) return;
        const current = currentWindowHeight();
        if (!Number.isFinite(current) || Math.abs(current - height) > HEIGHT_TOLERANCE) return;
        const measured = measureCardHeight(card);
        if (measured > height + 1) resizeLauncherWindow(card, measured, passes - 1);
      });
    })
    .catch(() => undefined);
}
