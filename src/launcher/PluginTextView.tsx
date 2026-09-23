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

import type { CSSProperties } from "react";
import type { Translate } from "../i18n.ts";
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
}: {
  t: Translate;
  text: string;
  metrics: PluginTextMetrics;
}) {
  return (
    <div className="launcher-options" role="region" aria-label={t("launcher.results")}>
      <pre
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
