// R28 · the text form of the plugin capability.
//
// A plugin whose output is not the standard structure has its output printed
// verbatim under the search field — the first of the two display forms the user
// asked for (「纯文本，文本形式下 搜索框，下方直接展示工具输出的文本」). The
// block has a **minimum** height (never a one-pixel sliver) and a **maximum**
// height, and scrolls inside itself past the maximum, so a long command output
// can never grow the launcher window.
//
// The two numbers are not written here: they are the capability layer's
// `PLUGIN_TEXT_MIN_UNITS` / `PLUGIN_TEXT_MAX_UNITS`, carried on the view's
// metrics and handed to the sheet as inline `calc(var(--u) * N)` lengths. The
// window height that contains this block is the band table's, resolved from
// `pluginViewRows` exactly as the list's is.
//
// R63 · this is the surface the user calls 「直出」 (direct output): the text a
// summoned command prints, shown in the search interface itself. Selecting it
// copies it — the terminal's 「选中即复制」 gesture (R44) on the launcher's own
// text. The gesture only *reports* here; the one copy chokepoint and the notice
// live outside this view.

import { useEffect, useRef } from "react";
import type { CSSProperties } from "react";
import type { Translate } from "../i18n.ts";
import { selectionTextIn } from "./plugin-text-copy.ts";
import type { PluginTextMetrics } from "./plugin-mode.ts";

/** The two bounds, as unit *counts* the sheet multiplies by `--u`.
 *
 * They are custom properties rather than `calc()` strings written here on
 * purpose: this module must not spell the scale knob itself (`--u` has exactly
 * one owner, `src/ui-scale.ts` — see `ui-scale.test.ts`), so it hands the sheet
 * two numbers and `styles/launcher.css` does the `calc(var(--u) * …)`. */
const textStyle = (metrics: PluginTextMetrics): CSSProperties =>
  ({
    "--plugin-text-min": String(metrics.minUnits),
    "--plugin-text-max": String(metrics.maxUnits),
  }) as CSSProperties;

export function PluginTextView({
  t,
  text,
  metrics,
  onCopySelection,
}: {
  t: Translate;
  text: string;
  metrics: PluginTextMetrics;
  /** R63 · report the text a selection gesture produced inside the block. The
   *  caller owns the clipboard write and the notice (the single chokepoint is
   *  `hooks/useLauncherTextCopy.ts`); this view only answers the question
   *  "which selection is ours?" through the pure `selectionTextIn`. */
  onCopySelection: (text: string) => void;
}) {
  const blockRef = useRef<HTMLPreElement>(null);
  /** Whether the gesture in flight started in this block. A ref (not state):
   *  arming it must not re-render, and the mouseup that consumes it must see
   *  the same value the mousedown wrote. */
  const gestureStartRef = useRef(false);

  // R63 · copy on select, on the window's own mouse events.
  //
  // The gesture is defined by where it *started*: a mousedown inside the block
  // arms the gesture, and the matching mouseup anywhere copies the selection it
  // produced. Window-level (not the block's own `onMouseUp`) because a drag
  // that reaches the block's bottom edge auto-scrolls the inner viewport and is
  // then released *outside* the box — the browser keeps the selection's two
  // ends inside the block, so the gesture is still ours, but a handler bound to
  // the `<pre>` would never hear the release and the classic "drag to the end of
  // a long output" would silently copy nothing.
  //
  // Nothing is prevented and no state is written during the drag, so the
  // browser's own selection, its auto-scroll and the inner scrollbar all
  // behave natively: the release point is the selection end, the R44 rule.
  useEffect(() => {
    const onMouseDown = (event: MouseEvent) => {
      const block = blockRef.current;
      gestureStartRef.current = block !== null && block.contains(event.target as Node);
    };
    const onMouseUp = () => {
      if (!gestureStartRef.current) return;
      gestureStartRef.current = false;
      const selected = selectionTextIn(blockRef.current, window.getSelection());
      if (selected !== null) onCopySelection(selected);
    };
    window.addEventListener("mousedown", onMouseDown);
    window.addEventListener("mouseup", onMouseUp);
    return () => {
      window.removeEventListener("mousedown", onMouseDown);
      window.removeEventListener("mouseup", onMouseUp);
    };
  }, [onCopySelection]);

  return (
    <div className="launcher-options" role="region" aria-label={t("launcher.results")}>
      <pre
        ref={blockRef}
        className={`launcher-plugin-text${
          metrics.scrolls ? " launcher-plugin-text--scrollable" : ""
        }`}
        style={textStyle(metrics)}
        // The output is the plugin's own text; it is read, not tabbed into.
        tabIndex={-1}
      >
        {text}
      </pre>
    </div>
  );
}
