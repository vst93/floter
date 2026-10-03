// R80 · the command-row builder.
//
// Every `command`-family launcher row — the catalog completion row, the catalog
// command row, R68's install row and R69's invoke row — is the same nine
// required fields with a different producer:
//
//   { type, id, title, subtitle, warnings, sourceName, commandLine,
//     execution, completion }
//
// The four producers used to spell that skeleton out by hand, which is four
// places to update the day the family grows a required field (and four chances
// to update only three of them). This module states the skeleton once. The
// family-specific extras that ride on top — `installCommand`, `launchArgv`,
// `launchNeedsTerminal`, `directOutput` — stay with the producer that owns
// them, spread over the builder's result.
//
// It is a pure module (no React, no Tauri) so the node suite can pin the shape
// without a DOM.

import type { ExecutionPlan } from "../launcher";
import type { CommandWarning, LauncherItem } from "./LauncherResults";

/** One `command`-family launcher row: the shape the list draws, the numbered
 *  shortcuts slot, and the execution layer reads. */
export type CommandRow = Extract<LauncherItem, { type: "command" }>;

/** The nine fields every command row carries, whatever produced it. The
 *  producer supplies the values; {@link commandRow} supplies `type`. */
export type CommandRowFields = {
  id: string;
  title: string;
  subtitle: string;
  warnings: CommandWarning[];
  sourceName: string;
  commandLine: string;
  execution: ExecutionPlan | null;
  completion: boolean;
};

/**
 * Build a command row from its shared fields.
 *
 * The row's `type` is added here; a producer that carries an extra field
 * (`installCommand`, `launchArgv`, …) spreads it over the result, so the nine
 * shared fields can only be written in one place.
 */
export const commandRow = (fields: CommandRowFields): CommandRow => ({
  type: "command",
  ...fields,
});
