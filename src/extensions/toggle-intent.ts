import type { Extension } from "../ExtensionsPanel";

/**
 * R100 · the extension switch's two directions are not the same act.
 *
 * The switch is one control with two meanings. Turning an integration *off*
 * is the reversible half of a lifecycle change: it marks the integration
 * disabled, stops whatever it is running right now (R98), and keeps every file
 * and every byte of its data — so it asks first. Turning it back *on* restores
 * something the user already asked for and destroys nothing, so it stays
 * instant and never grows a confirmation of its own.
 *
 * This projection is the one place that decision lives, so the panel, the
 * source guard and the negative guard all read the same rule. It is pure and
 * carries no React or Tauri dependency: the caller owns the side effect
 * (`setDisableTarget` for the confirm branch, `runMutation(… "enable")` for the
 * immediate one).
 */
export type ToggleIntent =
  | { kind: "confirm-disable"; extension: Extension }
  | { kind: "enable"; extension: Extension };

export function toggleIntent(extension: Extension): ToggleIntent {
  return extension.enabled
    ? { kind: "confirm-disable", extension }
    : { kind: "enable", extension };
}
