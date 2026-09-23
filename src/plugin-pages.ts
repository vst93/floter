// Pure logic behind floter's plugin surfaces: the base-plugin registry the
// settings panel renders, the generic postMessage bridge protocol between a
// plugin page and the host app, page URL building, and allowlist checks. Kept
// free of React and Tauri so the node test suite can exercise it directly.
//
// R33 · the built-in iframe pages are retired. Their configuration is now the
// launcher's `PluginConfigOverlay`, driven by the schema in
// `src/plugins/config-schema.ts`; nothing in the app opens an iframe any more.
// The bridge below is kept as the *published* external-page protocol (see
// docs/extensions/plugin-page-protocol.md) — a capability with no built-in
// consumer, not a second settings surface.
//
// The contract: a plugin page runs in a sandboxed iframe. External pages use
// an opaque origin (no same-origin, no Tauri APIs); the trusted built-in page
// may opt into same-origin only when WebKit requires it to load local assets.
// Every page still talks to the host through the postMessage bridge. Every
// message carries a `floter` type tag; unknown messages are ignored rather
// than erroring.

import type { MessageKey } from "./i18n";

/** Stable id of the built-in clipboard base plugin, mirroring the backend's
 * registry (src-tauri/src/plugin_pages.rs). */
export const CLIPBOARD_PLUGIN_ID = "builtin.clipboard";

/** Stable id of the built-in browser plugin, mirroring the same registry. */
export const BROWSER_PLUGIN_ID = "builtin.browser";

/** Marker property every bridge message carries. */
export const BRIDGE_TAG = "floter" as const;

/**
 * The plugin-page bridge protocol version this host speaks.
 *
 * A plugin page announces the version it was written against in its
 * `frame-ready` handshake (see [`BridgeFrameReady`]). The host compares that
 * number with this constant *before* it enters interactive mode: a page that
 * announces nothing — or a version this build does not know — gets a visible
 * error state that names the version the host supports, instead of a silent
 * white iframe that looks like a crash. Bump this constant whenever a message
 * shape changes in a way an older peer cannot simply ignore.
 */
export const PLUGIN_PAGE_PROTOCOL = 1;

/**
 * Page → host: the page's document is loaded and its message listener is
 * attached. This is the handshake, and it is the first message a page sends —
 * before any `invoke`, before the host pushes any settings.
 *
 * `protocol` is the [`PLUGIN_PAGE_PROTOCOL`] the page was written against. A
 * page that omits it is treated as incompatible rather than as an old page:
 * guessing compatibility is how a version negotiation silently rots.
 */
export type BridgeFrameReady = {
  [BRIDGE_TAG]: "frame-ready";
  /** The [`PLUGIN_PAGE_PROTOCOL`] the page was written against. */
  protocol: number;
};

/** Page → host: run one command through the host's `invoke()`. */
export type BridgeRequest = {
  [BRIDGE_TAG]: "invoke";
  /** Caller-chosen correlation id echoed back on the result. */
  id: number;
  session?: string;
  command: string;
  args?: Record<string, unknown> | null;
};

/** Page → host: dismiss the plugin page, returning to the remembered surface. */
export type BridgeClose = { [BRIDGE_TAG]: "close" };

/**
 * Page → host: begin a native window drag, exactly as a mousedown on the
 * terminal or launcher chrome would.
 *
 * Why a message type instead of `data-tauri-drag-region`: a plugin page runs
 * in a sandboxed iframe, so a mousedown inside it is invisible to the host — it
 * cannot reach the host's `startDrag`, and the attribute would only work for a
 * host-document element. The page therefore reports the *intent* ("the user
 * pressed on my blank header and wants to move the window") and the host
 * executes it through the very same `startDrag` path the three shells use,
 * keeping the Windows blur-grace and the interactive-element guard in one
 * place. The page does its own interactive-element exemption before sending —
 * a press on a button, field or row never reaches the wire — so the host has
 * nothing to second-guess.
 *
 * This is the one message in the protocol that asks the host to move the OS
 * window rather than touch app state; it is deliberately payload-free, so a
 * page cannot smuggle coordinates, a target window or a size into it.
 */
export type BridgeDrag = { [BRIDGE_TAG]: "drag" };

/**
 * Page → host: raise one feedback toast on the *app's* stack.
 *
 * Why a message type instead of a host command on the invoke allowlist: the
 * toast stack is frontend-only state (`toast-state.ts` + `ToastStack.tsx`),
 * not a Tauri command, so there is nothing for the backend to allowlist and
 * nothing that could be answered with a `result`. A page that wants feedback
 * is not asking the host to *do* something, it is asking the host to *show*
 * something — same-shaped request, different pipeline. The existing bridge
 * already carries that kind of one-way page → host traffic (`close`), so this
 * rides the same path and the same source-of-frame check.
 *
 * The wire carries a dictionary KEY, never the user-visible string: the toast
 * is host chrome (position, lifetime, visuals all live host-side), so the host
 * translates and drops keys it does not know rather than painting text a
 * sandbox chose. That is also what keeps the two documents' dictionaries from
 * having to stay in sync — the page's own `t()` may not even have the key.
 */
export type BridgeNotify = {
  [BRIDGE_TAG]: "host-notify";
  /** Page-generated correlation id, echoed back verbatim on a
   * [`BridgeNotifyRetry`]. It is what lets three coexisting toasts each keep
   * their own retry action: without it the page can only remember the newest
   * failure, and pressing an older toast's retry would silently re-run the
   * wrong action. */
  id: number;
  kind: "error" | "success";
  /** Dictionary key from the app's own `src/i18n.ts` tables. */
  messageKey: string;
  /** True when the page can re-run whatever failed; the toast then offers a
   * retry that asks the page to try again (see [`BridgeNotifyRetry`]). */
  retryable?: boolean;
};

/**
 * Host → page: the user pressed the retry action on the toast raised by a
 * previous [`BridgeNotify`]. The page re-runs the action that failed — it is
 * the only side that knows what that action was and can keep its own state
 * (entries, selection) consistent with the outcome.
 *
 * `id` is the notification's own id, not a fresh one: the host never mints
 * ids, it only echoes. A reply whose id no longer maps to a live failure is
 * dropped, so a toast the user dismissed long ago cannot fire the action that
 * happens to be newest.
 */
export type BridgeNotifyRetry = { [BRIDGE_TAG]: "notify-retry"; id: number };

/** Host → page: result of a bridge invocation, matched by correlation id. */
export type BridgeResult =
  | { [BRIDGE_TAG]: "result"; id: number; session?: string; ok: true; value: unknown }
  | { [BRIDGE_TAG]: "result"; id: number; session?: string; ok: false; error: string };

export type BridgeVisibility = { [BRIDGE_TAG]: "visibility"; visible: boolean };

/**
 * Host → page: live opacity update. Opacity is also a bootstrap query param,
 * but the sliders move mid-session; pushing the new values as a message lets
 * the page restyle in place instead of forcing an iframe remount (which would
 * throw away the page's filter text, selection and scroll position).
 */
export type BridgeOpacity = {
  [BRIDGE_TAG]: "opacity";
  mainOpacity: number;
  terminalOpacity: number;
};

/**
 * Host → page: live theme update. Theme is also a bootstrap query param, but
 * the app theme can change mid-session (user toggles dark/light in settings);
 * pushing the new value as a message lets the page update in place.
 */
export type BridgeTheme = {
  [BRIDGE_TAG]: "theme";
  theme: "dark" | "light";
};

/**
 * Host → page: live glass-step update. The step is also a bootstrap query
 * param, but the settings panel can change it mid-session; pushing the new
 * value as a message lets the page swap its material in place instead of
 * forcing an iframe remount.
 *
 * The value is the step *id*, not a resolved alpha: the page owns the same
 * `GLASS_STEP_TOKENS` table the host does, so the numbers have one source and
 * travel once. A page built before this field existed simply never receives
 * the message and keeps its own default (Regular) — see the `mid` fallback in
 * the clipboard page's handler.
 */
export type BridgeGlass = {
  [BRIDGE_TAG]: "glass";
  glassStep: "frosted" | "regular" | "liquid";
};

/**
 * Host → page: the page was just revealed after being hidden. Sent when the
 * plugin toggles from hidden (pluginId null) to shown. The page should reload
 * its data to show fresh content.
 */
export type BridgeReload = {
  [BRIDGE_TAG]: "reload";
};

/**
 * Dictionary keys a page may name on the wire. Deliberately narrow: dotted
 * lower-camel keys from `src/i18n.ts` (`clipboard.actionFailed`), nothing that
 * could smuggle markup, whitespace or a 10 MB string across the sandbox
 * boundary. The host still has to find the key in its own dictionary — this
 * only rules out shapes that cannot be keys at all.
 */
const MESSAGE_KEY_SHAPE = /^[A-Za-z][A-Za-z0-9.]{0,63}$/;

export type BridgeFromPage = BridgeRequest | BridgeClose | BridgeNotify | BridgeDrag | BridgeFrameReady;

/**
 * How many failure retries a page remembers at once. The host keeps at most
 * `MAX_TOASTS` (3) toasts on screen, so a retry older than the third-newest
 * failure can no longer be pressed; keeping the registry the same size bounds
 * it for a page that fails in a loop. The oldest entry is evicted first — the
 * same end the toast stack drops from.
 */
export const RETRY_REGISTRY_CAPACITY = 3;

export type RetryRegistry = {
  /** Remember the action that belongs to notification `id`. */
  add: (id: number, retry: () => void) => void;
  /**
   * Run and consume the retry for `id`. An id the page never issued, or one
   * already run or evicted, is dropped without running anything — this is the
   * stale drop that keeps an old toast from firing the newest action.
   */
  run: (id: number) => void;
};

/**
 * Page-side map from a [`BridgeNotify`] id to the action that raised it.
 *
 * Pure and DOM-free so the node suite can drive the exact multi-toast case the
 * single-slot version got wrong (see [`BridgeNotifyRetry`]). Insertion order
 * is the eviction order because `Map` iterates in insertion order.
 */
export const createRetryRegistry = (
  capacity: number = RETRY_REGISTRY_CAPACITY,
): RetryRegistry => {
  const retries = new Map<number, () => void>();
  return {
    add: (id, retry) => {
      retries.delete(id);
      retries.set(id, retry);
      while (retries.size > capacity) {
        const oldest = retries.keys().next().value;
        if (oldest === undefined) break;
        retries.delete(oldest);
      }
    },
    run: (id) => {
      const retry = retries.get(id);
      if (!retry) return;
      retries.delete(id);
      retry();
    },
  };
};

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
 * Page-side deduper for failures raised by an automatic trigger.
 *
 * Why this exists: the clipboard page polls every 2s, so a backend that stays
 * down would otherwise raise one toast per poll forever — three toasts
 * churning on screen, none readable. Why it is scoped per key rather than
 * wrapping every `notifyFailure`: a failure caused by a *user gesture* (a copy,
 * a delete) must still report each time the user asks, or the second click
 * fails silently. Only the poll needs coalescing, and it names one key.
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

// Arrays are objects but never valid bridge payloads: `args` travels into a
// named-args invoke call, so an array would silently become `{0: …}`.
const isRecord = (value: unknown): value is Record<string, unknown> =>
  typeof value === "object" && value !== null && !Array.isArray(value);

export const isBridgeRequest = (data: unknown): data is BridgeRequest =>
  isRecord(data) &&
  data[BRIDGE_TAG] === "invoke" &&
  typeof data.id === "number" &&
  Number.isFinite(data.id) &&
  (data.session === undefined || (typeof data.session === "string" && data.session.length <= 128)) &&
  typeof data.command === "string" &&
  data.command.length > 0 &&
  // A missing args field is fine; anything present must be an object (or null)
  // because it travels straight into the named-args invoke call.
  (data.args === undefined || data.args === null || isRecord(data.args));

export const isBridgeClose = (data: unknown): data is BridgeClose =>
  isRecord(data) && data[BRIDGE_TAG] === "close";

export const isBridgeNotify = (data: unknown): data is BridgeNotify =>
  isRecord(data) &&
  data[BRIDGE_TAG] === "host-notify" &&
  typeof data.id === "number" &&
  Number.isFinite(data.id) &&
  (data.kind === "error" || data.kind === "success") &&
  typeof data.messageKey === "string" &&
  MESSAGE_KEY_SHAPE.test(data.messageKey) &&
  (data.retryable === undefined || typeof data.retryable === "boolean");

export const isBridgeDrag = (data: unknown): data is BridgeDrag =>
  isRecord(data) && data[BRIDGE_TAG] === "drag";

export const isBridgeFrameReady = (data: unknown): data is BridgeFrameReady =>
  isRecord(data) &&
  data[BRIDGE_TAG] === "frame-ready" &&
  typeof data.protocol === "number" &&
  Number.isFinite(data.protocol);

/**
 * The outcome of the protocol handshake, decided once per loaded document.
 *
 * - `accepted` — the page announced this host's protocol; the bridge opens.
 * - `missing` — the page never announced a version (a stale page, or a page
 *   that is not a plugin page at all).
 * - `mismatch` — the page announced a version this build does not speak.
 *
 * `missing` and `mismatch` are kept apart on purpose: one is a page that
 * should be updated, the other is a page built for a *different* protocol,
 * and a page author debugging a load needs to tell those two apart from the
 * error text alone. Both are refused the same way.
 */
export type HandshakeVerdict =
  | { status: "accepted" }
  | { status: "missing" }
  | { status: "mismatch"; protocol: number };

/**
 * Decide whether a page may enter interactive mode, from the message the page
 * sent in its handshake.
 *
 * This is pure and DOM-free so the node suite can drive all three states
 * (`match` / `missing` / `mismatch`) without a browser. `expected` defaults to
 * this build's [`PLUGIN_PAGE_PROTOCOL`], and is a parameter only so a test can
 * play the part of a future host and prove the verdict tracks the host's own
 * number rather than a hardcoded 1.
 */
export const pluginPageHandshake = (
  data: unknown,
  expected: number = PLUGIN_PAGE_PROTOCOL,
): HandshakeVerdict => {
  if (!isBridgeFrameReady(data)) return { status: "missing" };
  if (data.protocol !== expected) return { status: "mismatch", protocol: data.protocol };
  return { status: "accepted" };
};

/**
 * The one-line reason a refused handshake shows above its retry button.
 *
 * The host owns the words, but the *facts* — which version the page asked for,
 * which version this build speaks — are what a page author needs to fix the
 * page, so both numbers are in the text rather than only in the console. The
 * strings themselves live in the i18n dictionary; this builder only picks the
 * variant and resolves the two numbers, so `mismatch` and `missing` cannot
 * silently collapse into the same sentence.
 */
export const handshakeErrorDetail = (
  verdict: HandshakeVerdict,
  expected: number = PLUGIN_PAGE_PROTOCOL,
): { key: "plugin.protocolMissing" | "plugin.protocolMismatch"; params: Record<string, number> } => {
  if (verdict.status === "mismatch") {
    return {
      key: "plugin.protocolMismatch",
      params: { page: verdict.protocol, host: expected },
    };
  }
  return { key: "plugin.protocolMissing", params: { host: expected } };
};

/**
 * Whether a page message should start a native window drag right now.
 *
 * Two conditions, and the second is not decoration: a kept-alive page that is
 * *hidden* behind another one must not move the window on a stale press, so the
 * host only honours the request while that page is the active surface. The
 * predicate lives here (not inline in the message listener) so the routing —
 * `drag` from the live page → the host's one drag body — is a unit the node
 * suite can drive, and a regression to a no-op is a red test rather than a
 * silently dead header.
 */
export const shouldStartWindowDrag = (data: unknown, pageActive: boolean): boolean =>
  isBridgeDrag(data) && pageActive;

export const isBridgeNotifyRetry = (data: unknown): data is BridgeNotifyRetry =>
  isRecord(data) &&
  data[BRIDGE_TAG] === "notify-retry" &&
  typeof data.id === "number" &&
  Number.isFinite(data.id);

export const isBridgeOpacity = (data: unknown): data is BridgeOpacity =>
  isRecord(data) &&
  data[BRIDGE_TAG] === "opacity" &&
  typeof data.mainOpacity === "number" &&
  Number.isFinite(data.mainOpacity) &&
  typeof data.terminalOpacity === "number" &&
  Number.isFinite(data.terminalOpacity);

export const isBridgeTheme = (data: unknown): data is BridgeTheme =>
  isRecord(data) &&
  data[BRIDGE_TAG] === "theme" &&
  (data.theme === "dark" || data.theme === "light");

export const isBridgeGlass = (data: unknown): data is BridgeGlass =>
  isRecord(data) &&
  data[BRIDGE_TAG] === "glass" &&
  (data.glassStep === "frosted" ||
    data.glassStep === "regular" ||
    data.glassStep === "liquid");

export const isBridgeReload = (data: unknown): data is BridgeReload =>
  isRecord(data) && data[BRIDGE_TAG] === "reload";

export const isBridgeVisibility = (data: unknown): data is BridgeVisibility =>
  isRecord(data) && data[BRIDGE_TAG] === "visibility" && typeof data.visible === "boolean";

export const isBridgeResult = (data: unknown): data is BridgeResult =>
  isRecord(data) &&
  data[BRIDGE_TAG] === "result" &&
  typeof data.id === "number" &&
  Number.isFinite(data.id) &&
  (data.session === undefined || (typeof data.session === "string" && data.session.length <= 128)) &&
  (data.ok === true
    ? "value" in data
    : data.ok === false && typeof data.error === "string");

export const isBridgeResultForSession = (data: unknown, session: string): data is BridgeResult =>
  isBridgeResult(data) && data.session === session;

/**
 * Whether a plugin page may call `command`. The host enforces this before
 * every invoke; the allowlist comes from the backend's page descriptor (the
 * existing permission model can gate it further later).
 */
export const commandAllowed = (
  allowed: readonly string[],
  command: string,
): boolean => allowed.includes(command);

/**
 * Absolute URL for a plugin page asset against the app document's own base,
 * plus bootstrap query params (language, theme, opacities) that a sandboxed
 * page cannot read any other way — its opaque origin blocks storage and most
 * document access, but its own location string is still visible.
 *
 * Pure so node tests can pin the shapes both a dev-server URL and a packaged
 * `tauri://` URL produce.
 *
 * Throws when `page` resolves off the app's own origin: a descriptor's `page`
 * comes from a plugin, so an absolute URL there would otherwise load a remote
 * document into a frame the host feeds bootstrap params and answers bridge
 * invocations for. Protocol and host are compared alongside `origin` because
 * non-special schemes (`tauri:`) report an opaque `"null"` origin that would
 * otherwise match anything.
 */
export const buildPluginPageUrl = (
  base: string,
  page: string,
  params?: Record<string, string | number>,
): string => {
  const baseUrl = new URL(base);
  const url = new URL(page, baseUrl);
  if (
    url.origin !== baseUrl.origin ||
    url.protocol !== baseUrl.protocol ||
    url.host !== baseUrl.host
  ) {
    throw new Error(`Plugin page must stay on the app origin: ${page}`);
  }
  for (const [key, value] of Object.entries(params ?? {})) {
    url.searchParams.set(key, String(value));
  }
  return url.toString();
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
  /** Whether the plugin has a persisted on/off switch. Both built-in plugins
   *  do: the clipboard switch lives in its long-standing
   *  `clipboard_history_enabled` field and the browser's in its own
   *  `browser_plugin.enabled`. A future always-on plugin would render no switch
   *  rather than a dead one. */
  toggleable: boolean;
  /** R33 · whether the plugin has a declarative configuration schema the row
   *  can open in the launcher's generic overlay. Both built-ins do; a plugin
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
];

/**
 * The built-in pages that must be sandboxed with `allow-same-origin`.
 *
 * Both built-ins ship as bundled assets on the app's own origin, and WebKit
 * refuses to load a bundled stylesheet (and, for an ES module entry, the module
 * itself) from an opaque-origin frame. R26-B gave the browser plugin a page but
 * only the clipboard id was special-cased in `PluginPageHost`, so on macOS the
 * browser page's module never ran — and a page whose script never runs never
 * sends its `frame-ready`, which is exactly the “此页面未声明插件页协议版本”
 * the user saw. This list is the one place that decides the exception; a page
 * not named here keeps the opaque-origin sandbox.
 */
export const SAME_ORIGIN_PLUGIN_PAGES: readonly string[] = [
  CLIPBOARD_PLUGIN_ID,
  BROWSER_PLUGIN_ID,
];

/** Whether a plugin page may load same-origin (bundled) assets. */
export const pluginPageNeedsSameOrigin = (id: string | null | undefined): boolean =>
  id !== null && id !== undefined && SAME_ORIGIN_PLUGIN_PAGES.includes(id);
