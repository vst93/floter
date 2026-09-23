// R44 · the copy notice's state machine, as a pure module.
//
// Every successful or failed copy reports itself in the terminal's own status
// row. The row is always in the layout (see `.terminal-copy-notice`), so what
// moves is only the text's opacity — and that opacity is a function of one of
// three phases:
//
//   idle ──show──▶ visible ──advance──▶ fade ──advance──▶ idle
//
// `visible` is the ~1.5s the message is readable; `fade` is the short window
// in which it is painted out (the CSS transition runs there) before the text
// is dropped. A second copy while the first is still on screen restarts the
// visible phase — the same message must not blink off and on, and the timer
// must not stack.
//
// The clock deliberately lives outside this module: the hook owns the timers
// (`useCopyNotice`) and calls `copyNoticeAdvance` on the phase's own delay, so
// the machine is deterministic and testable without fake timers.

import type { MessageKey } from "../i18n";

export const COPY_NOTICE_PHASES = ["idle", "visible", "fade"] as const;
export type CopyNoticePhase = (typeof COPY_NOTICE_PHASES)[number];

export interface CopyNoticeState {
  phase: CopyNoticePhase;
  /** The message being reported, or `null` while idle. */
  message: MessageKey | null;
}

export const COPY_NOTICE_IDLE: CopyNoticeState = { phase: "idle", message: null };

/** How long the message stays fully readable. */
export const COPY_NOTICE_VISIBLE_MS = 1500;
/** How long the fade-out transition runs before the text is dropped. Kept
 *  above the CSS transition's own duration so the node never unmounts the
 *  text mid-transition. */
export const COPY_NOTICE_FADE_MS = 260;

/** Report a message. Always lands in `visible`, so a copy that arrives during
 *  the fade pulls the message back to full opacity instead of letting it blink
 *  out and in. It deliberately returns a *fresh* object even when the phase did
 *  not change: that identity change is what re-runs the hook's effect and so
 *  restarts the delay, which is the throttle that keeps consecutive copies from
 *  flickering (and keeps the timers from stacking). */
export function copyNoticeShow(_state: CopyNoticeState, message: MessageKey): CopyNoticeState {
  return { phase: "visible", message };
}

/** Move one step along `visible → fade → idle`; `idle` is absorbing. */
export function copyNoticeAdvance(state: CopyNoticeState): CopyNoticeState {
  if (state.phase === "visible") return { phase: "fade", message: state.message };
  if (state.phase === "fade") return COPY_NOTICE_IDLE;
  return state;
}

/** How long to wait before calling {@link copyNoticeAdvance}, or `null` when
 *  the machine is idle and no timer should be armed. */
export function copyNoticeDelay(state: CopyNoticeState): number | null {
  if (state.phase === "visible") return COPY_NOTICE_VISIBLE_MS;
  if (state.phase === "fade") return COPY_NOTICE_FADE_MS;
  return null;
}
