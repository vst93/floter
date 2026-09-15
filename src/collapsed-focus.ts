// The single collector that guarantees the launcher's text input owns the
// keyboard whenever the collapsed surface is showing.
//
// Historically every path back to the launcher scheduled its own timers
// (`focusCollapsedInput(0/90/140)`, `focusCollapsedInput(90)`, …). That
// scattered approach has a structural hole. Returning to collapsed runs a
// native window resize (`show_input` then `syncLauncherHeight`), and a resize
// commits *after* the last scheduled focus beat: the webview can move keyboard
// focus back to the document body while the geometry settles, so the input
// ends up unfocused with no further attempt. On top of that a kept-alive
// plugin iframe can take the keyboard back later still (its page schedules its
// own `focus()`s on reveal, and a hidden iframe document can still hold
// focus). Nothing in the old design observed either event.
//
// This module owns one rule — "while `mode === collapsed`, the text input
// holds the keyboard unless focus is deliberately inside the launcher card" —
// and enforces it on every signal that can break it:
//
//   * the multi-beat pattern scheduled right after a surface commit,
//   * every `focusout` that leaves the card,
//   * the window regaining OS focus (`focus`, e.g. a native reveal or a WM
//     re-activation), and
//   * the completion of every launcher resize (see `reassertCollapsedFocus`,
//     called by `syncLauncherHeight` once its `setSize` settles).
//
// The policy predicates are pure and the controller accepts its DOM seams, so
// the whole thing is testable without a browser (tests/collapsed-focus.test.ts).

/** Default beat pattern: the commit instant, then two later attempts that ride
 * out the platform's window-reveal / autoFocus races. */
export const COLLAPSED_FOCUS_BEATS_MS = [0, 90, 140] as const;

export type CollapsedFocusRefs = {
  /** Live mirror of the current view mode. */
  modeRef: { current: string };
  inputRef: { current: HTMLInputElement | null };
  cardRef: { current: HTMLElement | null };
};

/**
 * Whether the collapsed input should take the keyboard back from `active`.
 *
 * Yields to the card's own focusable controls (so Tab can still walk the
 * result list and the settings button) and to the input itself; reclaims from
 * everything else — the document body, a kept-alive plugin iframe, or any node
 * outside the card.
 */
export function shouldReclaimCollapsedFocus(
  active: Element | null,
  input: HTMLInputElement | null,
  card: HTMLElement | null,
): boolean {
  if (!input) return false;
  if (active === input) return false;
  if (active && card && card.contains(active)) return false;
  return true;
}

/**
 * Focus the collapsed input now and put the caret at the end. Returns whether
 * the focus actually landed (the surface may have changed underneath us).
 */
export function focusCollapsedInputNow(refs: CollapsedFocusRefs): boolean {
  if (refs.modeRef.current !== "collapsed") return false;
  const input = refs.inputRef.current;
  if (!input) return false;
  input.focus({ preventScroll: true });
  const length = input.value.length;
  input.setSelectionRange(length, length);
  return true;
}

export type CollapsedFocusController = {
  /** Schedule the beat pattern, or a single beat when `delay` is given. */
  focus: (delay?: number) => void;
  /** Re-assert synchronously; no-op unless the collapsed surface is showing. */
  reassert: () => void;
  /** Run the reclaim policy once (used by the focusout/window-focus watchers). */
  reclaim: () => void;
  /** Attach the event-driven watchers. Returns a disposer (idempotent). */
  attach: () => () => void;
};

/**
 * Build the controller. The DOM seams (`schedule`, `activeElement`,
 * `addEventListener`/`removeEventListener`) are injectable so the behaviour can
 * be driven by a test without a browser.
 */
export function createCollapsedFocusController(deps: {
  refs: CollapsedFocusRefs;
  schedule?: (fn: () => void, delay: number) => void;
  activeElement?: () => Element | null;
  listen?: {
    onFocusOut: (handler: () => void) => () => void;
    onWindowFocus: (handler: () => void) => () => void;
  };
}): CollapsedFocusController {
  const {
    refs,
    schedule = (fn, delay) => {
      window.setTimeout(fn, delay);
    },
    activeElement = () => document.activeElement,
  } = deps;

  const focus = (delay = 0) => {
    schedule(() => {
      focusCollapsedInputNow(refs);
    }, delay);
  };

  const reassert = () => {
    focusCollapsedInputNow(refs);
  };

  const reclaim = () => {
    if (refs.modeRef.current !== "collapsed") return;
    if (
      shouldReclaimCollapsedFocus(
        activeElement(),
        refs.inputRef.current,
        refs.cardRef.current,
      )
    ) {
      focusCollapsedInputNow(refs);
    }
  };

  let attachedDisposer: (() => void) | null = null;
  const attach = () => {
    if (attachedDisposer) return attachedDisposer;
    const scheduleReclaim = () => {
      // At `focusout` time the incoming element has not been focused yet, so
      // the check is deferred a tick to see where the keyboard settled.
      schedule(reclaim, 0);
    };
    let disposeFocusOut: () => void;
    let disposeWindowFocus: () => void;
    if (deps.listen) {
      disposeFocusOut = deps.listen.onFocusOut(scheduleReclaim);
      disposeWindowFocus = deps.listen.onWindowFocus(reclaim);
    } else {
      document.addEventListener("focusout", scheduleReclaim, true);
      window.addEventListener("focus", reclaim);
      disposeFocusOut = () => document.removeEventListener("focusout", scheduleReclaim, true);
      disposeWindowFocus = () => window.removeEventListener("focus", reclaim);
    }
    attachedDisposer = () => {
      disposeFocusOut();
      disposeWindowFocus();
      attachedDisposer = null;
    };
    return attachedDisposer;
  };

  return { focus, reassert, reclaim, attach };
}

// A single process-wide reassert hook, so `syncLauncherHeight` (a leaf helper
// that knows nothing about React) can ask the live controller to re-collect
// focus the moment a native launcher resize settles. Set once by `App`.
let reassertHook: (() => void) | null = null;

export function setCollapsedFocusReassert(hook: (() => void) | null): void {
  reassertHook = hook;
}

export function reassertCollapsedFocus(): void {
  reassertHook?.();
}
