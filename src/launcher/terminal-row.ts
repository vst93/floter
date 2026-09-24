// R60 · The ⌘-held bare-terminal row.
//
// The user's ask, verbatim: 「方案 A 增加一个浮动终端的选项，同时在按住 cmd 时也
// 自动增加底部的终端执行项，一样的再按回车进入，只是这是没有任何命令执行」 —
// and, on the floating half: 「系统命令『浮动终端』这逻辑算了，不要」.
//
// R64 · the row's one remaining form. The user corrected R61, verbatim: 「刚做的
// 输入框为空时 按住 cmd 选中终端打开这个逻辑和一般搜索情况一样出现在末尾啊。
// 同时前面加的按住在列表额外增加终端选项的逻辑要去掉」 — so a typed query no
// longer draws the row at all (R60's input-state append retires), and the empty
// page puts it at the list's **last** line (below the recents), preselected,
// exactly the position semantics of an ordinary search row. R61's featured lift
// above the recents retires with it.
//
// Why a module of its own: the row's *visibility* and the window's *height* must
// be one decision (R52's rule for the chips row and the subline insets, R58's for
// the list chrome). The App reads the row through {@link bareTerminalPlacement}
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

/** R64 · *How* the ⌘-held row joins the list.
 *
 *  The user's ask, verbatim (after correcting their own earlier word once and
 *  then correcting R61's answer): an *empty* field held under the modifier puts
 *  the row at the list's **last line** — 「出现在末尾」, the same position
 *  semantics as the ordinary search case — and preselects it. So there is one
 *  placement, and one only:
 *
 *   * `"appended"` — the empty page under the held modifier: the row is the
 *     list's last line, below the recents block (the 「最近启动」 rows and their
 *     heading), and it takes the selection on the modifier's press edge.
 *   * `null` — not drawn at all: the modifier is up, a query is typed (R60's
 *     input-state append retires — 「前面加的按住在列表额外增加终端选项的逻辑要
 *     去掉」), or a plugin scope owns the list (R32/R38/R39/R50 — a terminal row
 *     inside bookmark or clipboard history would be a row the plugin never
 *     produced).
 *
 *  The row remains one predicate and one billing source: the App puts it into
 *  `displayedResults`, so `launcherListUnits` charges it and
 *  `displayedResults.length` counts it exactly as every other row. The placement
 *  only decides where in that list the row goes — never a second height term.
 *
 *  R64 keeps the name `"appended"` (rather than renaming it `"held"`) because it
 *  still names the one thing R64 pins: the row is the list's *last* line. The
 *  gesture's name is already `bareTerminal*`; the placement's job is position.
 */
export type BareTerminalPlacement = "appended";

export const bareTerminalPlacement = (
  query: string,
  modifierHeld: boolean,
  scope: string | null,
): BareTerminalPlacement | null => {
  if (!modifierHeld || scope !== null) return null;
  return query.trim().length > 0 ? null : "appended";
};

/**
 * Whether the ⌘-held row is drawn — the boolean every reader that does not care
 * about the placement still asks (R60 called this the whole predicate; R61 and
 * R64 keep the name as the derived form, so there is one authority, not two).
 *
 * The terms and their exclusions are {@link bareTerminalPlacement}'s: the
 * modifier is held, the ordinary page owns the list, and the field is empty.
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
 * R64 · the R61 `featured` marker retires with the featured treatment: the row is
 * an ordinary `system` row again, with no accent edge and no accent glyph, at the
 * list's end exactly as an ordinary search row would be.
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
