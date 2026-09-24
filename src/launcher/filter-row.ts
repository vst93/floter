// R41 · the plugin filter row's visibility, declared once.
//
// The chips row (`.launcher-filter`: the browser's range chips and the
// clipboard's six) is the **plugin list's own filter**, not part of the search
// field. Until R41 its visibility was spelled out as `launcherScope ===
// "browser" || launcherScope === "clipboard"` at each render site, which said
// "whenever a plugin mode is active". That is one state too many: while the
// plugin's configuration overlay is open, the list is not on screen and the
// chips filtered nothing the user could see — yet the row still rendered above
// the overlay, a filter for an invisible list.
//
// The user's report, verbatim: 「现在两个内置插件在启动的时候进入设置界面，
// 中间的状态切换栏还是会显示出来。这个应该和下面列表一样作为一个整体，而不应该和
// 上面搜索框作为一个整体。」
//
// So the row's visibility is a function of the same fact the list's is: is the
// plugin's *list body* on screen? Three conditions, all necessary:
//
//   · the app is on the collapsed surface (the chips row does not exist on the
//     settings panel or the terminal — those render their own trees);
//   · a plugin mode owns the field (the chips are that plugin's filter); and
//   · the plugin's configuration overlay is closed (the overlay takes the
//     list's place, and a filter for a hidden list is chrome with no referent).
//
// It is deliberately **not** "always show": the row is the plugin list's
// filter, and a filter that outlives its list is the leak the report names.
//
// Pure (no React, no DOM) so the node suite can drive the whole matrix.

import type { ViewMode } from "../App.tsx";

/** The plugin scopes that draw a filter row. An external plugin command has no
 *  chip vocabulary of its own, so it is absent here by construction rather
 *  than by a runtime check at the render site. */
export const FILTER_ROW_SCOPES = ["browser", "clipboard", "calculator"] as const;

export type FilterRowScope = (typeof FILTER_ROW_SCOPES)[number];

export type FilterRowState = {
  /** The app's surface: the chips row lives only inside `collapsed`. */
  mode: ViewMode;
  /** The active plugin mode's scope, or `null` on the ordinary search page. */
  scope: string | null;
  /** Whether the plugin's configuration overlay is open. */
  configOpen: boolean;
};

/** Whether `scope` is one that draws a chips row. */
export function filterRowScope(scope: string | null): FilterRowScope | null {
  return FILTER_ROW_SCOPES.find((candidate) => candidate === scope) ?? null;
}

/** Whether the chips row for the active plugin mode is on screen. */
export function pluginFilterRowVisible(state: FilterRowState): boolean {
  if (state.mode !== "collapsed") return false;
  if (state.configOpen) return false;
  return filterRowScope(state.scope) !== null;
}
