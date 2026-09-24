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
// the list chrome). The App appends the row through {@link bareTerminalRowVisible}
// and the row then takes part in the ordinary list accounting —
// `displayedResults.length` for the chrome and `launcherRowHeightUnits` for the
// row's own height — so there is no second predicate to drift from this one. It
// is a pure module (no React, no Tauri) so the node suite can pin the rule.
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

/**
 * Whether the ⌘-held row is drawn.
 *
 * Three terms, each of them a deliberate exclusion:
 *
 *   * `modifierHeld` — the row exists for as long as the key is down. Release
 *     ⌘ and it is gone on the next render; the window does not snap back (R43's
 *     hysteresis absorbs the one-row shrink, exactly as it does for any other
 *     row that leaves the list), so the appearance/release pair never flaps.
 *   * `query` — only a *typed* query opens the offer. The empty query is the
 *     launcher's front door: it already carries the recents and the first-run
 *     tip, and a fourth thing to read there would fight the onboarding copy.
 *   * `scope` — the ordinary search page only. Inside a plugin mode the list
 *     belongs to the plugin (R32/R38/R39/R50), and appending a terminal row to
 *     bookmark or clipboard history would be a row the plugin never produced.
 */
export const bareTerminalRowVisible = (
  query: string,
  modifierHeld: boolean,
  scope: string | null,
): boolean => modifierHeld && scope === null && query.trim().length > 0;

/**
 * The row itself: a `system` row whose action is `terminal`, which is the same
 * action the terminal system row carries (see `SYSTEM_COMMANDS` in
 * `useLauncherCatalog` and the `runSystemAction` branch in `useLauncherActions`).
 * Enter, the pointer and the numbered shortcut therefore reach it through the
 * existing row machinery — no new key path, no second runner.
 */
export const bareTerminalRow = (t: Translate): LauncherItem => ({
  type: "system",
  id: BARE_TERMINAL_ROW_ID,
  title: t(BARE_TERMINAL_TITLE_KEY),
  subtitle: t(BARE_TERMINAL_SUBTITLE_KEY),
  action: "terminal",
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
