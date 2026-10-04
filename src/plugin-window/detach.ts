// R84 · the detached plugin window's pure logic.
//
// The user asked for a plugin page that can be pinned out of the launcher:
// 「按相关的快捷键或者操作之后，插件页面可以独立固定在界面上而不自动消失。
//   独立之后的插件要脱离原来的整个软件主体，也就是不再跟随呼出和隐藏，
//   就相当于分割出来的一部分」— a real second window that stays put while the
// launcher card keeps its own summon/hide life.
//
// This module is the decisions a node test can pin, for the same reason
// `launcher/plugin-mode.ts` is pure: the window label, the request's shape and
// validation, and the one rule the whole feature stands on — **the detached
// window never hides on blur** — are exactly the lines a review could silently
// delete, so they live where a test imports them without dragging the app's
// runtime in. The React surface that consumes these values is
// `DetachedPluginApp.tsx`; the Rust half is `detach_plugin_window` et al. in
// `src-tauri/src/lib.rs`.

/** The label of the second window every detached plugin run lives in. One
 *  window, many runs: a second detach while it is open replaces the content
 *  (the backend hands the new request over on the same label). Spelled once so
 *  the Rust builder, the capability file and the frontend branch cannot drift
 *  apart — the tests read this constant against `capabilities/`. */
export const PLUGIN_WINDOW_LABEL = "plugin-detached";

/** The launcher card's own label — the only window hide-on-blur has ever
 *  governed. Read from here rather than inlined so the blur rule below can
 *  name its subject. */
export const MAIN_WINDOW_LABEL = "main";

/** What a detach hands across the process boundary. `extension_id` +
 *  `command_id` pick the command; `command_label` is the human name for the
 *  window title and the view's header; `args` is the field's text already
 *  split into argv items by the external protocol's own splitter. Plain data —
 *  the window shows one run, so the request is one value. */
export type DetachRequest = {
  readonly extensionId: string;
  readonly commandId: string;
  readonly commandLabel: string;
  readonly args: readonly string[];
};

/** Validate the raw payload the launcher assembles before it crosses into the
 *  backend. A command id is the routing truth — without one the window would
 *  run nothing — and ids are non-empty by the same convention the install
 *  path's `validate_id` enforces. Returns `null` for anything malformed; the
 *  caller treats that as "no request", never as an error surface. */
export const validateDetachRequest = (input: unknown): DetachRequest | null => {
  if (typeof input !== "object" || input === null) return null;
  const candidate = input as Record<string, unknown>;
  const extensionId = candidate.extensionId;
  const commandId = candidate.commandId;
  const commandLabel = candidate.commandLabel;
  const args = candidate.args;
  if (typeof extensionId !== "string" || extensionId.trim().length === 0) return null;
  if (typeof commandId !== "string" || commandId.trim().length === 0) return null;
  if (typeof commandLabel !== "string" || commandLabel.trim().length === 0) return null;
  if (!Array.isArray(args) || args.some((item) => typeof item !== "string")) return null;
  return { extensionId, commandId, commandLabel, args: args as readonly string[] };
};

/**
 * The rule the feature exists for, as one function.
 *
 * The launcher card hides when it loses focus (`hide_on_blur`, R2-era) — that
 * is its nature. The detached window is the thing the user pinned precisely so
 * it would NOT go away; a blur-hide there would make the pin a lie. So:
 *
 *   · main window + setting on  → hides on blur (unchanged behaviour);
 *   · the detached window       → never, whatever the setting says — the
 *     setting is the *launcher card's* preference, not a window system one;
 *   · any other window          → no (defensive default: a label this module
 *     has never heard of earns no hiding).
 *
 * App.tsx's focus effect calls this with `getCurrentWindow().label`, so the
 * exact same code path now answers for both windows.
 */
export const hideOnBlurApplies = (windowLabel: string, hideOnBlur: boolean): boolean =>
  windowLabel === MAIN_WINDOW_LABEL && hideOnBlur;

/** The one window title both the Rust builder and the detached view agree on
 *  when the request's own label is somehow absent. Not reachable through
 *  `validateDetachRequest` (it rejects empty labels) — this exists for the
 *  Rust side's fallback path where the title is set before validation. */
export const PLUGIN_WINDOW_FALLBACK_TITLE = "floter plugin";
