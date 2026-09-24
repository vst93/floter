// R62 · when a command-launched session ends, its terminal page ends with it.
//
// The user's report, verbatim: 「在搜索框内通过搜索启动的命令行工具的二级命令启动会
// 快捷启动这个命令，并在终端页展示，这时退出 tui 时应该同时退出终端页面」.
//
// A session started from the search box exists for exactly one purpose: to carry
// that command. When the process behind it ends — the TUI the user quit, a fast
// one-shot, a crash — the terminal page has nothing left to show and holding it
// open leaves the user on a dead surface. The page leaves, and the same is true
// for a non-zero exit: a session that never came up must not strand the user.
//
// The exception is the whole point, and it is why this is a predicate rather
// than an unconditional rule: a *bare* shell (the system "Terminal" row, the
// ⌘-held row, Enter on the empty page — all `initialCommand: null`) is the
// user's own terminal. Exiting that shell is the user closing their terminal,
// not the app closing a page, so it never triggers an automatic exit.
//
// This is a pure module (no React, no Tauri) so the node suite can pin the two
// decisions without a DOM: which spawns count as command-started, and whether a
// given exit may take the page.
import type { ViewMode } from "../App";
import type { ExecutionPlan } from "../launcher";

/**
 * Whether a spawn is a command-launched session.
 *
 * True when the spawn hands the PTY either a raw command line (`initialCommand`
 * — the shell-syntax path, typed verbatim) or a structured `execution` plan
 * (the catalog/plugin path, where the program itself is the child). Both are
 * "a command"; only a spawn with neither is the bare interactive shell.
 */
export const isCommandStartedSession = (
  initialCommand: string | null,
  execution: ExecutionPlan | null,
): boolean => initialCommand !== null || execution !== null;

/**
 * Whether the PTY exit may take the terminal page back to the launcher.
 *
 * Two conditions, both required:
 *   * the session is command-started (see above) — a bare shell never exits
 *     the page; and
 *   * the terminal page is the surface on screen — an exit that lands while the
 *     user has since moved to settings or a plugin must not yank them away from
 *     what they are doing.
 */
export const shouldExitWithCommandSession = (
  commandStarted: boolean,
  mode: ViewMode,
): boolean => commandStarted && mode === "terminal";
