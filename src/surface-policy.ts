// R7-2 · Surface keyboard policy, declared once.
//
// Two tables, both pure (no React, no DOM construction) so the node suite can
// enumerate them and pin the pre-R7-2 semantics as golden:
//
//   * `SURFACE_FOCUS_POLICY` — who owns the keyboard on each surface. S1/S2/S4
//     already handed the keyboard over on entry, each in its own branch of
//     App's mode effect; S3 (settings) did not, and left focus on `<body>`.
//     The table names the owner for all four so a new surface cannot be added
//     without answering the question.
//
//   * `DISMISS_TABLE` — what Escape and Cmd/Ctrl+W do on each surface. Before
//     R7-2 the answer was spelled out three times in `useAppKeyboard` (plus a
//     fourth copy of the chord literal in the modal layer), so the fact that
//     the surfaces agreed was a coincidence of copy-paste rather than a rule.
//
// Neither table changes an existing behaviour: the golden assertions in
// `tests/surface-policy.test.ts` were written against the pre-R7-2 handler and
// must stay green.

import { IS_WINDOWS, matchesShortcut, type ShortcutMap } from "./shortcuts.ts";
import { COLLAPSED_FOCUS_BEATS_MS } from "./collapsed-focus.ts";
import type { SettingsPage } from "./settings-persistence.ts";
/** The three surfaces; mirrors `ViewMode` in `App.tsx`. The retired plugin
 *  page used to be a fourth; R33 folded its configuration into the collapsed
 *  surface, so there is no iframe to hand the keyboard to any more. */
export type AppSurface = "collapsed" | "terminal" | "settings";

// ---------------------------------------------------------------------------
// Focus on entry
// ---------------------------------------------------------------------------

/** Who holds the keyboard once a surface has committed. */
export type FocusOwner =
  | "collapsed-input"
  | "terminal-canvas"
  | "settings-sidebar";

export type SurfaceFocusPolicy = {
  /** The control the surface hands the keyboard to. */
  owner: FocusOwner;
  /** Delayed focus attempts after the commit instant, in ms. Empty when the
   *  keyboard is claimed synchronously (settings) or by a guest document
   *  (plugin). */
  beats: readonly number[];
  /** Extra attempt on Windows, where the window is still being shown and
   *  focused when the first beat lands. */
  windowsRetry?: number;
};

/** Second attempt at handing the terminal canvas the keyboard on Windows, where
 * the window is still being shown and focused when the first one lands. */
export const TERMINAL_FOCUS_RETRY = 180;

/**
 * The keyboard home of every surface.
 *
 * The collapsed entry reuses `COLLAPSED_FOCUS_BEATS_MS` by reference: the
 * collector in `collapsed-focus.ts` remains the thing that *enforces* the
 * policy (beats, focusout reclaim, resize reassert), and this table is where
 * the policy is *declared*. Settings claims its sidebar synchronously — see
 * `focusSettingsSidebar` — because a delayed beat could pull the keyboard back
 * out of a field the user had already reached.
 */
export const SURFACE_FOCUS_POLICY: Record<AppSurface, SurfaceFocusPolicy> = {
  collapsed: {
    owner: "collapsed-input",
    beats: COLLAPSED_FOCUS_BEATS_MS,
    windowsRetry: TERMINAL_FOCUS_RETRY,
  },
  terminal: {
    owner: "terminal-canvas",
    beats: [80],
    windowsRetry: TERMINAL_FOCUS_RETRY,
  },
  settings: {
    owner: "settings-sidebar",
    beats: [],
  },
};

/**
 * The delayed focus attempts for a surface on entry, including the Windows
 * retry where the platform needs one. Order is the order the beats fire in.
 */
export function surfaceFocusBeats(surface: AppSurface): number[] {
  const policy = SURFACE_FOCUS_POLICY[surface];
  const beats = [...policy.beats];
  if (IS_WINDOWS && policy.windowsRetry !== undefined) beats.push(policy.windowsRetry);
  return beats;
}

/**
 * Focus the sidebar item for `page` — the settings surface's keyboard home.
 * Returns whether the focus landed (the button may not be mounted yet).
 *
 * Pure DOM: this is deliberately not a `make-key`/reveal native command, so the
 * macOS first-responder re-arm chain (58c6d24) is untouched.
 */
export function focusSettingsSidebar(
  buttons: { get: (page: SettingsPage) => HTMLElement | undefined },
  page: SettingsPage,
): boolean {
  const target = buttons.get(page);
  if (!target) return false;
  target.focus({ preventScroll: true });
  return true;
}

/**
 * Roving tabindex for the sidebar: exactly one item — the active page — is in
 * the tab order, and it is the one ↑/↓ and the initial focus already land on.
 * Tab therefore leaves the sidebar for the content instead of walking all five
 * buttons.
 */
export function settingsSidebarTabIndex(page: SettingsPage, active: SettingsPage): 0 | -1 {
  return page === active ? 0 : -1;
}

/**
 * Run a surface's declared entry policy. The seams are the app's existing
 * focus entry points, so this adds a rule without adding a mechanism: every
 * beat still goes through the collector (`focusCollapsedInput`), the canvas
 * focus helper, or a plain DOM `focus()` on the sidebar button.
 *
 * Returns the owner that was asked, so callers (and tests) can assert the
 * decision without observing DOM state.
 */
export function applySurfaceFocusOnEntry(
  surface: AppSurface,
  seams: {
    focusCollapsedInput: (delay: number) => void;
    focusTerminalView: (delay: number) => void;
    focusSettingsSidebar: () => boolean;
  },
): FocusOwner {
  const policy = SURFACE_FOCUS_POLICY[surface];
  switch (policy.owner) {
    case "collapsed-input":
      for (const beat of surfaceFocusBeats("collapsed")) seams.focusCollapsedInput(beat);
      break;
    case "terminal-canvas":
      for (const beat of surfaceFocusBeats("terminal")) seams.focusTerminalView(beat);
      break;
    case "settings-sidebar":
      seams.focusSettingsSidebar();
      break;
  }
  return policy.owner;
}

// ---------------------------------------------------------------------------
// Escape / Cmd+W
// ---------------------------------------------------------------------------

/** The trigger order the resolver walks. `mod-w` first preserves the pre-R7-2
 *  order: settings and plugin tested the literal chord before Escape. */
export const DISMISS_TRIGGER_ORDER = ["mod-w", "escape", "new-command"] as const;

export type DismissTrigger = (typeof DISMISS_TRIGGER_ORDER)[number];

export type DismissAction =
  | "hide-window"
  | "close-settings"
  | "return-to-input";

export type DismissRule = {
  action: DismissAction;
  /** Also stop propagation, so the modal layer / other window listeners do not
   *  see the press a second time (Cmd+W on settings and plugin, today). */
  stopPropagation?: boolean;
  /** On macOS the terminal's new-command path re-asserts the launcher input
   *  after the modifiers are released. */
  reassertOnKeyUp?: boolean;
};

/** `true` for Cmd+W on macOS and Ctrl+W elsewhere — the literal close chord,
 *  independent of Shift/Alt and of the user's shortcut map. */
export function isModW(event: KeyboardEvent): boolean {
  return (event.metaKey || event.ctrlKey) && event.key.toLowerCase() === "w";
}

/** Whether the press is the surface-dismiss chord the modal layer also uses. */
export function isDismissKey(event: KeyboardEvent): boolean {
  return isModW(event) || event.key === "Escape";
}

export type DismissTable = Record<
  AppSurface,
  Record<DismissTrigger, DismissRule | null>
>;

/**
 * Esc / Cmd+W per surface. A `null` entry is an explicit "this surface does not
 * dismiss on this trigger" — not an omission; the `DismissTable` type makes
 * every cell mandatory.
 *
 * `terminal` is the interesting row: its Escape belongs to the running shell
 * (or to the pinned card's "return to main"), and its Cmd+W is the user's
 * configurable new-command binding, which returns to the launcher rather than
 * dismissing a window.
 */
export const DISMISS_TABLE: DismissTable = {
  collapsed: {
    "mod-w": null, // only the configurable new-command binding dismisses here
    escape: { action: "hide-window" },
    "new-command": { action: "hide-window" },
  },
  terminal: {
    "mod-w": null, // the shell's Ctrl+W; on macOS the Cmd chord is swallowed later
    escape: null, // the shell's, or the pinned card's "return to main"
    "new-command": { action: "return-to-input", reassertOnKeyUp: true },
  },
  settings: {
    "mod-w": { action: "close-settings", stopPropagation: true },
    escape: { action: "close-settings" },
    "new-command": { action: "close-settings" },
  },
};

/**
 * Surfaces whose window-level handler stands down completely while an
 * `[aria-modal="true"]` dialog owns the keyboard. Settings is the only one
 * today (the integrations panel opens dialogs inside it): the dialog closes
 * itself on Esc/Cmd+W, and the surface underneath — including its ↑/↓ page
 * navigation — must not react at all.
 */
export const MODAL_GUARDED_SURFACES: readonly AppSurface[] = ["settings"];

export function surfaceYieldsToModal(surface: AppSurface): boolean {
  return MODAL_GUARDED_SURFACES.includes(surface);
}

function triggerMatches(
  trigger: DismissTrigger,
  event: KeyboardEvent,
  shortcuts: Pick<ShortcutMap, "new_command">,
): boolean {
  switch (trigger) {
    case "mod-w":
      return isModW(event);
    case "escape":
      return event.key === "Escape";
    case "new-command":
      return matchesShortcut(event, shortcuts.new_command);
  }
}

/**
 * The dismiss rule for a press on a surface, or `null` when the surface does
 * not dismiss on it. Walks `DISMISS_TRIGGER_ORDER`, so the observable result is
 * identical to the pre-R7-2 hand-written branch order.
 */
export function resolveDismissRule(
  surface: AppSurface,
  event: KeyboardEvent,
  shortcuts: Pick<ShortcutMap, "new_command">,
): DismissRule | null {
  const rules = DISMISS_TABLE[surface];
  for (const trigger of DISMISS_TRIGGER_ORDER) {
    const rule = rules[trigger];
    if (!rule) continue;
    if (triggerMatches(trigger, event, shortcuts)) return rule;
  }
  return null;
}
