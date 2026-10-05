// Pure logic behind floter's plugin surfaces: the base-plugin registry the
// settings panel renders, and the per-key failure deduper the app's automatic
// triggers coalesce through. Kept free of React and Tauri so the node test
// suite can exercise it directly.
//
// R33 · the built-in iframe pages were retired; their configuration is now the
// launcher's `PluginConfigOverlay`, driven by the schema in
// `src/plugins/config-schema.ts`, and nothing in the app opens an iframe any
// more. R96 · the generic postMessage bridge that used to sit here — page URL
// building, the command allowlist, the handshake, the message types and their
// guards — was deleted with the rest of the page layer: it had no producer
// (the manifest declares no page) and no consumer (no built-in page, no host).
// What remains is the registry the settings panel renders and the deduper the
// deep-link refusals share.

import type { MessageKey } from "./i18n";

/** Stable id of the built-in clipboard base plugin, mirroring the backend's
 * registry (src-tauri/src/plugin_pages.rs). */
export const CLIPBOARD_PLUGIN_ID = "builtin.clipboard";

/** Stable id of the built-in browser plugin, mirroring the same registry. */
export const BROWSER_PLUGIN_ID = "builtin.browser";

/** R50 · stable id of the built-in calculator plugin.
 *
 *  R51 · like the other two built-ins it is registered on both sides of the
 *  plugin registry — a row in `BUILTIN_BASE_PLUGINS` and an entry in the
 *  backend's `DESCRIPTORS` — so the settings panel's "Base plugins" list and
 *  the launcher's search can *reach* it. The id keeps naming its configuration
 *  schema and the overlay's plugin identity; the registry row is what makes it
 *  discoverable. It still has no plugin page and no on/off switch: its whole
 *  surface is the launcher mode and its schema-driven configuration overlay. */
export const CALCULATOR_PLUGIN_ID = "builtin.calculator";

/**
 * How long a key stays quiet after it has been raised once. Long enough that a
 * 2s background poll cannot fill the stack (fifteen failures per toast), short
 * enough that the same failure coming back later is announced again. */
export const FAILURE_NOTIFY_DEDUP_MS = 30_000;

export type FailureDeduper = {
  /**
   * Whether `key` may be raised now, and if so, mark it as raised. The second
   * and later calls inside the window return false; the first call after it
   * elapses returns true again. `now` is injectable so the node suite can
   * drive a 5-failure burst without waiting wall-clock time.
   */
  allow: (key: string, now?: number) => boolean;
  /** Re-arm a key: the failure that comes after this is news again. */
  clear: (key: string) => void;
};

/**
 * Deduper for failures raised by an automatic trigger.
 *
 * Why this exists: an automatic failure — a 2s poll against a backend that
 * stays down, a `floter://` link a hostile page retries in a loop — would
 * otherwise raise one toast per attempt forever, three toasts churning on
 * screen, none readable. Why it is scoped per key rather than wrapping every
 * failure report: a failure caused by a *user gesture* (a copy, a delete) must
 * still report each time the user asks, or the second click fails silently.
 * Only the automatic triggers need coalescing, and each names one key.
 */
export const createFailureDeduper = (
  windowMs: number = FAILURE_NOTIFY_DEDUP_MS,
): FailureDeduper => {
  const raisedAt = new Map<string, number>();
  return {
    allow: (key, now = Date.now()) => {
      const last = raisedAt.get(key);
      if (last !== undefined && now - last < windowMs) return false;
      raisedAt.set(key, now);
      return true;
    },
    clear: (key) => {
      raisedAt.delete(key);
    },
  };
};

/**
 * One row in the settings panel's base-plugins list (the extensions ecosystem's
 * "Base plugins" section).
 */
export type BuiltinBasePlugin = {
  id: string;
  /** i18n key for the row's name. */
  titleKey: MessageKey;
  /** i18n key for the row's one-line description. */
  descriptionKey: MessageKey;
  /** Whether the plugin has a persisted on/off switch. The clipboard's switch
   *  lives in its long-standing `clipboard_history_enabled` field and the
   *  browser's in its own `browser_plugin.enabled`; the calculator (R51) has
   *  no persisted field, so it renders no switch rather than a dead one. */
  toggleable: boolean;
  /** R33 · whether the plugin has a declarative configuration schema the row
   *  can open in the launcher's generic overlay. All three do; a plugin
   *  without one renders no Configure button. */
  configurable: boolean;
  /** Optional extra note rendered under the row (e.g. the clipboard privacy
   *  line). A plugin without one renders nothing extra. */
  privacyKey?: MessageKey;
};

/**
 * The base plugins the settings panel lists, mirroring the backend registry
 * `src-tauri/src/plugin_pages.rs` (`DESCRIPTORS`).
 *
 * This is the list the UI renders — it is deliberately NOT assembled by hand
 * in `App.tsx` any more. R26-B registered `builtin.browser` in Rust and gave it
 * a settings page, but the panel still rendered a clipboard-only array, so the
 * new plugin never appeared (and there was no way to open its page).
 * `tests/plugin-pages.test.ts` asserts this list and the Rust descriptor table
 * contain the *same* ids in both directions, so a new descriptor cannot ship
 * without a row, and a row cannot name an unregistered plugin.
 */
export const BUILTIN_BASE_PLUGINS: readonly BuiltinBasePlugin[] = [
  {
    id: CLIPBOARD_PLUGIN_ID,
    titleKey: "settings.clipboardHistory",
    descriptionKey: "settings.clipboardHistoryHint",
    toggleable: true,
    configurable: true,
    privacyKey: "settings.clipboardPrivacy",
  },
  {
    id: BROWSER_PLUGIN_ID,
    titleKey: "settings.browser",
    descriptionKey: "settings.browserHint",
    toggleable: true,
    configurable: true,
  },
  {
    // R51 · the calculator joins the list. It has a configuration schema (the
    // R50 overlay: history capacity, retention, copy mode) so it is
    // `configurable`; it has no persisted on/off field, so it is deliberately
    // *not* `toggleable` — the row renders no dead switch, exactly as
    // `BuiltinBasePlugin.toggleable` documents. Registering it here and in
    // `DESCRIPTORS` together is what keeps the descriptor-equality guard
    // honest rather than deleting it.
    id: CALCULATOR_PLUGIN_ID,
    titleKey: "settings.calculator",
    descriptionKey: "settings.calculatorHint",
    toggleable: false,
    configurable: true,
  },
];
