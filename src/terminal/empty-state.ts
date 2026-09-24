// R60 · The terminal page's empty state.
//
// The terminal page can be revealed with nothing in it: the native "show
// terminal" path (`floter://revealed` with the `terminal` payload) enters the
// page without spawning anything, and the page's own ✕ / ⌘W closes the session
// it was showing. In both cases the canvas is a blank rectangle that says
// nothing — the user reported it as the one surface with no way out and no way
// in.
//
// The hint is inline: it is painted *inside* the canvas region as a quiet,
// centered block (never a floating card over it), and it exists only while the
// page is empty. One session is enough to make it disappear, and the canvas
// takes the whole region back the moment it does.
//
// Why not read `ptyReady` directly: that ref is the *input* gate, flipped inside
// async spawn paths and read by the once-registered IPC listeners without going
// through React. The empty state is a render decision, so it reads the two
// pieces of state the page already publishes:
//
//   * `mainSessionIdentity` — set by a successful spawn/attach (`describeMainSession`),
//     cleared when the session is closed. Non-null means "a session exists".
//   * `terminalResident` — set when the PTY child exits: the page is *held* with
//     the final frame on screen (R9-2 slice 5). The hint must not cover output
//     the user is still reading, so a retained frame counts as content.
//
// This is a pure module (no React, no Tauri) so the node suite can pin both the
// rule and its one timing decision.
import type { ViewMode } from "../App";

/**
 * How long the page must stay empty before the hint is shown.
 *
 * A fresh session takes a broker round trip (`term_spawn`) before
 * `describeMainSession` publishes its identity, so for a few dozen milliseconds
 * after *every* ordinary entry the page is technically empty. Painting the hint
 * into that window would make the terminal flash a "no session" sentence on
 * every command the user runs. One idle beat is long enough to cover the spawn
 * and short enough that a genuinely empty page still feels immediate — the user
 * is looking at a blank canvas with nothing else to read.
 */
export const TERMINAL_EMPTY_HINT_DELAY = 240;

/**
 * Whether the terminal page has nothing to show: it is the terminal page, no
 * session exists, and no exited session left a frame behind.
 */
export const terminalPageEmpty = (mode: ViewMode, hasSession: boolean): boolean =>
  mode === "terminal" && !hasSession;
