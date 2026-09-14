// Sidebar ↑/↓ navigation rules for the settings panel, kept pure (no React, no
// DOM construction) so the node test suite can exercise them directly. The
// window-level key handler in `useAppKeyboard` is the only caller.

import { SETTINGS_PAGES, type SettingsPage } from "./settings-persistence.ts";

/**
 * Whether an event target owns its arrow keys and must not have them hijacked.
 *
 * Text entry (input/textarea, or anything contenteditable) and native
 * select/range controls move a caret or change a value on ↑/↓, so the sidebar
 * must leave them alone. Everything else — `<body>`, a plain button, a div —
 * is free to drive page navigation.
 *
 * Typed as `unknown` on purpose: keyboard events carry `EventTarget | null`,
 * which the DOM lib declares as `EventTarget`, and the node test harness has no
 * DOM types. The checks are structural, so a real element works and anything
 * else answers `false`.
 */
export function isArrowKeyEditableTarget(target: unknown): boolean {
  if (!target || typeof target !== "object") return false;
  const element = target as Partial<HTMLElement>;
  const tag = element.tagName;
  if (tag === "INPUT" || tag === "TEXTAREA" || tag === "SELECT") return true;
  if (element.isContentEditable === true) return true;
  return typeof element.closest === "function"
    ? Boolean(element.closest('[contenteditable="true"]'))
    : false;
}

/** The page ↑/↓ lands on, wrapping around the sidebar order. */
export function nextSettingsPage(
  current: SettingsPage,
  direction: "up" | "down",
): SettingsPage {
  const delta = direction === "down" ? 1 : SETTINGS_PAGES.length - 1;
  const index = SETTINGS_PAGES.indexOf(current);
  return SETTINGS_PAGES[(index + delta) % SETTINGS_PAGES.length];
}
