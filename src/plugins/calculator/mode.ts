// R50 · the calculator plugin's *output* — and nothing else.
//
// The clipboard's `plugins/clipboard/mode.ts` is the model: this module turns
// calculator history into standard `PluginRow`s and the capability layer
// (`launcher/plugin-mode.ts`) decides the form, the tier and the height. What
// stays here is the calculator's own row vocabulary — which entry a chip keeps,
// the one-line time word, and the empty states — because those are facts about
// a calculation, not about a launcher.

import {
  calculatorEntryMatchesFilter,
  calculatorTime,
  type CalculatorEntry,
  type CalculatorModeFilter,
} from "../../calculator.ts";
import type { MessageKey, Translate } from "../../i18n.ts";
import type { PluginRow } from "../../launcher/plugin-mode.ts";
import { MAX_RESULTS } from "../../launcher/result-budget.ts";
import { statusRowBase } from "../status.ts";

/** How many calculator rows the mode shows. The budget is the launcher's ten
 *  rows (`MAX_RESULTS`); the mode holds the whole (bounded) history and the
 *  launcher windows it client-side, exactly as the clipboard does. */
export const CALCULATOR_FETCH_LIMIT = MAX_RESULTS;

/** A status line for the calculator mode: no history yet, nothing starred, or
 *  a search that found nothing. Information, not a door. */
export const calculatorStatusRow = (
  id: string,
  key: MessageKey,
  t: Translate,
): PluginRow => ({
  family: "calculator",
  ...statusRowBase(id, t(key)),
});

/** The compact time word a row prints: a clock for today, the translated word
 *  for yesterday, a date beyond that. */
const rowTimeText = (entry: CalculatorEntry, t: Translate, now: number): string => {
  const time = calculatorTime(entry.created_at, now);
  if (time.kind === "yesterday") return t("calculator.yesterday");
  return time.text;
};

/** One calculator history row: the expression as the title, the compact time
 *  as the subtitle, and the whole entry for the renderer's right-aligned
 *  result. */
export const calculatorRow = (
  entry: CalculatorEntry,
  t: Translate,
  now: number,
): PluginRow => ({
  family: "calculator",
  id: entry.id,
  title: entry.expression,
  subtitle: rowTimeText(entry, t, now),
  entry,
});

/**
 * The mode's rows for one request: the history filtered by the active chip. The
 * **field's text is deliberately not a search needle** here, unlike the
 * clipboard and browser modes: the field is the pending *expression*, and
 * filtering the history by the expression as it is typed would hide every past
 * calculation exactly while the user is writing a new one. The bottom list is
 * the history — `全部` or `收藏` — and Enter's choice between evaluating and
 * copying is `calculatorEnterAction`'s, not a filter's.
 *
 * An empty result is a status line, and the two empty states are distinct — an
 * empty history is not an empty favorites view.
 */
export const calculatorModeRows = (
  entries: readonly CalculatorEntry[],
  mode: { needle: string; filter: CalculatorModeFilter },
  t: Translate,
  now: number,
  limit: number = CALCULATOR_FETCH_LIMIT,
): PluginRow[] => {
  const matched = entries.filter((entry) => calculatorEntryMatchesFilter(entry, mode.filter));
  const matches = matched.slice(0, limit);
  if (matches.length) return matches.map((entry) => calculatorRow(entry, t, now));
  return [
    calculatorStatusRow(
      "calculator-empty",
      mode.filter === "favorites" ? "calculator.emptyFavorites" : "calculator.empty",
      t,
    ),
  ];
};
