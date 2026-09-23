// R40 · a plugin list mode's *filter axis*, declared once.
//
// R32 gave the browser mode a range filter and R38 gave the clipboard mode six
// kind chips; both arrived as the same three things written in three places:
//
//   · an ordered array of values (`BROWSER_FILTERS` / `CLIPBOARD_FILTERS`) —
//     the chips row and the Tab cycle both walk it;
//   · a `cycle…Filter` function that wraps at the ends; and
//   · a `…_FILTER_KEYS` map from each value to the i18n key its chip prints,
//     which lived in `App.tsx` beside the chips row.
//
// The three are one fact — "this is the dimension the mode filters by" — and
// splitting it is how the values and their words drift apart. This module is the
// one shape that fact takes, so a third plugin (a built-in or, one day, an
// external command that declares its own chips) declares an axis rather than
// restating the trio.
//
// It is a pure module — no React, no Tauri, no DOM — for the same reason
// `plugins/search.ts` is: the axis *is* the contract, and a node test can only
// pin it if it can import it without the app runtime.

import type { MessageKey } from "../i18n.ts";

/**
 * One dimension a plugin's list mode can be filtered by: the values the chips
 * row and the Tab cycle walk, in display order, and the i18n key each value
 * prints on its chip.
 *
 * The axis deliberately does **not** carry a predicate. What a value *selects*
 * is a fact about the plugin's own rows — a browser range narrows which of
 * three fetched lists contributed a row, a clipboard chip classifies an entry's
 * kind — and those live in the plugin's mode module
 * (`plugins/browser/mode.ts` / `plugins/clipboard/mode.ts`). The axis is the
 * *chrome's* half of the filter: which choices exist and what they are called.
 */
export type PluginFilterAxis<F extends string> = {
  readonly values: readonly F[];
  readonly labelKey: (value: F) => MessageKey;
};

/**
 * Build an axis from its two halves. The `Record<F, MessageKey>` argument is
 * exhaustive by type, so a value added to `values` without a label (or a label
 * for a value that does not exist) is a compile error rather than a chip that
 * prints an empty string.
 */
export const pluginFilterAxis = <F extends string>(
  values: readonly F[],
  labelKeys: Record<F, MessageKey>,
): PluginFilterAxis<F> => ({
  values,
  labelKey: (value) => labelKeys[value],
});

/**
 * Move one step through an axis, wrapping at both ends. `direction` is `1` for
 * Tab and `-1` for Shift+Tab — the contract the browser and clipboard cycles
 * have shared since R38. Callers always pass a `current` taken from the axis
 * (the mode owns it), so the modulo never has to answer for an off-axis value.
 */
export const cyclePluginFilter = <F extends string>(
  axis: PluginFilterAxis<F>,
  current: F,
  direction: 1 | -1,
): F => {
  const index = axis.values.indexOf(current);
  const length = axis.values.length;
  return axis.values[(index + direction + length) % length];
};
