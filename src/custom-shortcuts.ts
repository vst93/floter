// R55 · user-defined global shortcuts, as a pure module.
//
// A custom shortcut binds one OS key to one launcher-addressable action. The
// action is a string with a small vocabulary:
//
//   · `plugin:<id>`   — open a built-in plugin's launcher mode *normally* (the
//                       window is shown and the mode is entered; a plugin is a
//                       visible thing, never a silent one);
//   · `action:<id>`   — run one of the app's own launcher actions (the same
//                       ids the built-in shortcut map uses);
//   · anything else   — a shell command line, run silently by the backend
//                       (`run_silent_command`): no window, no terminal, no UI.
//
// Keeping the classification here (pure, DOM-free) lets the node suite pin the
// "plugins open, commands run silently" rule without a webview, and lets the
// settings page and the trigger handler agree on one vocabulary.

import { parseShortcut } from "./shortcuts.ts";
import type { MessageKey } from "./i18n";

export type CustomShortcut = { key: string; action: string };

export type CustomShortcutKind = "plugin" | "action" | "command";

export type CustomShortcutBinding = {
  kind: CustomShortcutKind;
  value: string;
};

/** How an action string is executed. */
export const classifyCustomShortcutAction = (action: string): CustomShortcutBinding => {
  const trimmed = action.trim();
  if (trimmed.startsWith("plugin:")) {
    return { kind: "plugin", value: trimmed.slice("plugin:".length).trim() };
  }
  if (trimmed.startsWith("action:")) {
    return { kind: "action", value: trimmed.slice("action:".length).trim() };
  }
  return { kind: "command", value: trimmed };
};

/** The plugin actions the picker offers. `value` is the plugin id the App maps
 *  to an `ActivePluginMode`. */
export const CUSTOM_SHORTCUT_PLUGINS: { id: string; action: string; labelKey: MessageKey }[] = [
  { id: "clipboard", action: "plugin:clipboard", labelKey: "shortcut.clipboard_panel" },
  { id: "browser", action: "plugin:browser", labelKey: "customShortcut.plugin.browser" },
  { id: "calculator", action: "plugin:calculator", labelKey: "customShortcut.plugin.calculator" },
];

/** The app actions the picker offers, reusing the shortcut labels. */
export const CUSTOM_SHORTCUT_ACTIONS: { id: string; action: string; labelKey: MessageKey }[] = [
  { id: "toggle_window", action: "action:toggle_window", labelKey: "shortcut.toggle_window" },
  { id: "new_command", action: "action:new_command", labelKey: "shortcut.new_command" },
  { id: "open_settings", action: "action:open_settings", labelKey: "shortcut.open_settings" },
  {
    id: "open_external_terminal",
    action: "action:open_external_terminal",
    labelKey: "shortcut.open_external_terminal",
  },
];

export const CUSTOM_SHORTCUT_PICKER: { action: string; labelKey: MessageKey }[] = [
  ...CUSTOM_SHORTCUT_PLUGINS.map((entry) => ({ action: entry.action, labelKey: entry.labelKey })),
  ...CUSTOM_SHORTCUT_ACTIONS.map((entry) => ({ action: entry.action, labelKey: entry.labelKey })),
];

/** The two keys are the same binding. `parseShortcut` resolves `CmdOrCtrl`, so
 *  `Cmd+Shift+P` and `CmdOrCtrl+Shift+P` compare equal on macOS. */
export const customShortcutKeysEqual = (a: string, b: string): boolean => {
  const left = parseShortcut(a);
  const right = parseShortcut(b);
  if (!left || !right) return a.trim().toLowerCase() === b.trim().toLowerCase();
  return (
    left.ctrl === right.ctrl &&
    left.alt === right.alt &&
    left.shift === right.shift &&
    left.meta === right.meta &&
    left.key.toLowerCase() === right.key.toLowerCase()
  );
};

/**
 * The first key in `keys` that duplicates `key`, or `null`.
 *
 * Used twice: the settings page refuses to add a duplicate before persisting,
 * and the backend re-checks against the built-in map (a custom key may not
 * shadow `toggle_window` etc.). `ignoreIndex` skips the row being edited.
 */
export const duplicateCustomShortcutKey = (
  key: string,
  keys: readonly string[],
  ignoreIndex = -1,
): string | null => {
  if (!key.trim()) return null;
  const found = keys.findIndex(
    (existing, index) => index !== ignoreIndex && customShortcutKeysEqual(existing, key),
  );
  return found === -1 ? null : keys[found];
};

/** Drop empty/invalid entries and collapse duplicate keys (first wins). */
export const normalizeCustomShortcuts = (list: readonly CustomShortcut[]): CustomShortcut[] => {
  const out: CustomShortcut[] = [];
  for (const entry of list) {
    const action = entry.action.trim();
    if (!action) continue;
    const key = entry.key.trim();
    if (!key) continue;
    if (out.some((existing) => customShortcutKeysEqual(existing.key, key))) continue;
    out.push({ key, action });
  }
  return out;
};
