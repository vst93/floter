// R60 · The ⌘-held bare-terminal row.
//
// The user's ask, verbatim: 「方案 A 增加一个浮动终端的选项，同时在按住 cmd 时也
// 自动增加底部的终端执行项，一样的再按回车进入，只是这是没有任何命令执行」 —
// and, on the floating half: 「系统命令『浮动终端』这逻辑算了，不要」.
//
// So there is exactly one extra door, and it is a *row*, not a window: while the
// app modifier (⌘ on macOS, Ctrl elsewhere — the same normalization the rest of
// the app uses) is held down on the ordinary search page, one system row is
// appended below the query's own results. Enter on it opens a blank terminal
// session — the identical action the terminal system row carries — so nothing is
// executed and nothing is typed into the shell.
//
// Why a module of its own: the row's *visibility* and the window's *height* must
// be one decision (R52's rule for the chips row and the subline insets, R58's for
// the list chrome). The App reads the row through {@link bareTerminalPlacement}
// and the row then takes part in the ordinary list accounting —
// `displayedResults.length` for the chrome and `launcherRowHeightUnits` for the
// row's own height — so there is no second predicate to drift from this one. It
// is a pure module (no React, no Tauri) so the node suite can pin the rule.
//
// R61 · the user corrected their own first word (「不是追加」): the modifier has
// *two* placements. A typed query keeps the append above; the empty page lifts
// the row above the recents as the featured lead and takes the selection on the
// modifier's press edge. Both come from the one predicate, so the billing —
// `displayedResults` — stays one source.
import type { MessageKey, Translate } from "../i18n";
import type { LauncherItem } from "./LauncherResults";

/** The row's stable id. The renderer keys rows by it, so it must not depend on
 *  the query that happens to be typed when the modifier goes down. */
export const BARE_TERMINAL_ROW_ID = "system-terminal-bare";

/**
 * The app modifier state a `KeyboardEvent` reports, platform-normalized.
 *
 * The same rule `shortcuts.ts` applies to every binding: ⌘ on macOS, Ctrl
 * everywhere else. It lives here, next to the row it drives, because it is the
 * row's *premise* — and because this module is importable by the node suite
 * (type-only imports only) while the hook that owns the listeners is not.
 * The flag is a parameter rather than `IS_MAC` so the rule has one spelling and
 * two platforms' worth of tests.
 */
export const appModifierHeld = (
  event: { metaKey: boolean; ctrlKey: boolean },
  isMac: boolean,
): boolean => (isMac ? event.metaKey : event.ctrlKey);

/** The title / subtitle keys, named here so a test can pin that both
 *  dictionaries carry them (the row is bilingual like every other row). */
export const BARE_TERMINAL_TITLE_KEY: MessageKey = "launcher.terminalRow";
export const BARE_TERMINAL_SUBTITLE_KEY: MessageKey = "launcher.terminalRowSubtitle";

/** R61 · *How* the ⌘-held row joins the list, which is the one predicate the
 *  two forms share.
 *
 *  The user's ask, verbatim (after correcting their own earlier word once):
 *  「不是追加，当有输入内容时，当前是末尾本身就有 cmd 会自动选中，现在是要在
 *  没有任何输入时，按住 cmd 要特殊显示这项并选中」. So the modifier is one
 *  gesture with two placements, decided by whether a query is typed:
 *
 *   * `"appended"` — a typed query. This is R60's behaviour, unchanged: the row
 *     is the list's last line, below the query's own results (and any drop).
 *   * `"featured"` — the empty page. R61's addition: the row is lifted *above*
 *     the recents block (the 「最近启动」 heading included) as a highlighted
 *     featured row, and it takes the selection on the modifier's press edge.
 *   * `null` — not drawn at all: the modifier is up, or a plugin scope owns the
 *     list (R32/R38/R39/R50 — a terminal row inside bookmark or clipboard
 *     history would be a row the plugin never produced).
 *
 *  Both drawn forms remain one predicate and one billing source: the App puts
 *  the row into `displayedResults`, so `launcherListUnits` charges it and
 *  `displayedResults.length` counts it exactly as every other row. The placement
 *  only decides where in that list the row goes — never a second height term.
 */
export type BareTerminalPlacement = "appended" | "featured";

export const bareTerminalPlacement = (
  query: string,
  modifierHeld: boolean,
  scope: string | null,
): BareTerminalPlacement | null => {
  if (!modifierHeld || scope !== null) return null;
  return query.trim().length > 0 ? "appended" : "featured";
};

/**
 * Whether the ⌘-held row is drawn — the boolean every reader that does not care
 * about the two placements still asks (R60 called this the whole predicate; R61
 * keeps the name as the derived form, so there is one authority, not two).
 *
 * The terms and their exclusions are {@link bareTerminalPlacement}'s: the
 * modifier is held, the ordinary page owns the list, and the query chooses which
 * of the two rows is put on screen.
 */
export const bareTerminalRowVisible = (
  query: string,
  modifierHeld: boolean,
  scope: string | null,
): boolean => bareTerminalPlacement(query, modifierHeld, scope) !== null;

/**
 * The row itself: a `system` row whose action is `terminal`, which is the same
 * action the terminal system row carries (see `SYSTEM_COMMANDS` in
 * `useLauncherCatalog` and the `runSystemAction` branch in `useLauncherActions`).
 * Enter, the pointer and the numbered shortcut therefore reach it through the
 * existing row machinery — no new key path, no second runner.
 *
 * R61 · `featured` marks the empty page's lift: the renderer adds
 * `.launcher-result--featured` for it (an accent edge and an accent glyph; no
 * fill — R46's budget is untouched). The append path calls this with the flag
 * off, so R60's row is byte-for-byte what it was.
 */
export const bareTerminalRow = (t: Translate, featured = false): LauncherItem => ({
  type: "system",
  id: BARE_TERMINAL_ROW_ID,
  title: t(BARE_TERMINAL_TITLE_KEY),
  subtitle: t(BARE_TERMINAL_SUBTITLE_KEY),
  action: "terminal",
  ...(featured ? { featured: true } : {}),
});

/**
 * Whether a launcher row is that row.
 *
 * The id is what tells it apart from the terminal *system* row a query can
 * match (`system-terminal`): the two carry the same action, but only this one
 * exists because the modifier is down, and only this one is ever on screen
 * while the modifier is still held — which is the fact the key handler needs
 * when it decides whether ⌘⏎ belongs to the action bar or to the selected row.
 */
export const isBareTerminalRow = (item: { id?: string } | undefined): boolean =>
  item?.id === BARE_TERMINAL_ROW_ID;
