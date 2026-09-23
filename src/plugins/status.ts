// R40 · a plugin's *status line*, one shape.
//
// R30 taught the capability layer to draw a plugin row whose `kind` is
// `"status"` as the launcher's muted note rather than a result: no icon plate,
// no `⌘N`, no pointer state, no Enter. Both built-ins then grew their own
// builder for one — `browserStatusRow` (R26-D) and `clipboardStatusRow` (R27) —
// and R39 added a third, the external command mode's idle / running / failed /
// empty lines, which the catalog hook builds inline. All three say the same
// thing in the same shape:
//
//   { id, title, subtitle: "", disabled: true, kind: "status" }
//
// The family-specific fields differ (a browser status row still carries an empty
// `url` and a `profileKey` so it validates as a browser row), so this module
// owns the shared *core* and each plugin spreads it with its own family. The
// benefit is the contract, not the line count: a status line is information, not
// a door, and that one fact is now written once — including for an external
// plugin, which can build its own with {@link pluginStatusRow}.
//
// Pure: no React, no Tauri, no DOM.

import type { PluginRow } from "../launcher/plugin-mode.ts";

/**
 * The fields every status line shares: the capability layer's `status` kind (so
 * the renderer draws the muted note) and `disabled` (so the tier rule treats a
 * list of only these as display-only). The title is already translated — the
 * caller owns its own `Translate`.
 */
export const statusRowBase = (id: string, title: string) => ({
  id,
  title,
  subtitle: "",
  disabled: true as const,
  kind: "status" as const,
});

/**
 * R39/R40 · the generic *external* plugin's status line (family `"plugin"`).
 * The four non-output states of a command mode — not run yet, running, the run
 * failed, no output — are all this row, built through one helper rather than
 * spelled inline in the catalog hook.
 */
export const pluginStatusRow = (id: string, title: string): PluginRow => ({
  family: "plugin",
  ...statusRowBase(id, title),
});
