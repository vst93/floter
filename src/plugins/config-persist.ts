// R113 · the write path behind the plugin configuration overlay.
//
// R29 painted a change immediately and fired its write off without waiting; the
// write swallowed its own failure (`.catch(() => undefined)`), so a refused
// write left the control showing a value the backend never stored *and*
// advanced the committed snapshot as if it had (R111 finding C, "静默失效").
//
// This module gives the write a result. `writeConfigChange` turns a rejection
// into `false` — it catches on purpose, but it does not hide anything: every
// caller receives the outcome. `configChangeOutcome` then says what the overlay
// does with it, the same optimistic-rollback shape the settings surface uses
// (R94, `settings-persistence.ts`): accepted advances the committed snapshot,
// refused restores the painted values to it and names the failure line.

import type { MessageKey } from "../i18n";
import type { PluginConfigValue } from "./config-schema";

/** The schema-shaped value map the overlay paints and commits. */
export type ConfigValues = Record<string, PluginConfigValue>;

/** Run one write and report whether the backend accepted it.
 *
 *  The write is the caller's `invoke` thunk. The rejection is caught here so it
 *  becomes the boolean the caller acts on — not so it can be dropped. */
export const writeConfigChange = async (write: () => Promise<unknown>): Promise<boolean> => {
  try {
    await write();
    return true;
  } catch {
    return false;
  }
};

/** What one painted change resolves to once its write answers.
 *
 *  `commit` advances the committed snapshot to the painted values — the backend
 *  stored them. `rollback` restores the painted values to that snapshot and
 *  names the failure line, so a control can never keep showing a value that was
 *  never stored. */
export type ConfigChangeOutcome =
  | { kind: "commit"; committed: ConfigValues }
  | { kind: "rollback"; values: ConfigValues; feedback: MessageKey };

export const configChangeOutcome = (
  next: ConfigValues,
  committed: ConfigValues,
  accepted: boolean,
): ConfigChangeOutcome =>
  accepted
    ? { kind: "commit", committed: next }
    : { kind: "rollback", values: committed, feedback: "settings.saveFailed" };
