// Pure logic behind floter's generic plugin HTML pages: the postMessage
// bridge protocol between a plugin page and the host app, page URL building,
// and allowlist checks. Kept free of React and Tauri so the node test suite
// can exercise it directly (see tests/plugin-pages.test.ts).
//
// The contract: a plugin page runs in a sandboxed iframe (no same-origin, no
// Tauri APIs). It asks the host to invoke commands on its behalf and to close
// it; the host replies with results. Every message carries a `floter` type
// tag; unknown messages are ignored rather than erroring.

/** Stable id of the built-in clipboard base plugin, mirroring the backend's
 * registry (src-tauri/src/plugin_pages.rs). */
export const CLIPBOARD_PLUGIN_ID = "builtin.clipboard";

/** Marker property every bridge message carries. */
export const BRIDGE_TAG = "floter" as const;

/** Page → host: run one command through the host's `invoke()`. */
export type BridgeRequest = {
  [BRIDGE_TAG]: "invoke";
  /** Caller-chosen correlation id echoed back on the result. */
  id: number;
  command: string;
  args?: Record<string, unknown> | null;
};

/** Page → host: dismiss the plugin page, returning to the remembered surface. */
export type BridgeClose = { [BRIDGE_TAG]: "close" };

/** Host → page: result of a bridge invocation, matched by correlation id. */
export type BridgeResult =
  | { [BRIDGE_TAG]: "result"; id: number; ok: true; value: unknown }
  | { [BRIDGE_TAG]: "result"; id: number; ok: false; error: string };

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
 * Host → page: the page was just revealed after being hidden. Sent when the
 * plugin toggles from hidden (pluginId null) to shown. The page should reload
 * its data to show fresh content.
 */
export type BridgeReload = {
  [BRIDGE_TAG]: "reload";
};

export type BridgeFromPage = BridgeRequest | BridgeClose;

// Arrays are objects but never valid bridge payloads: `args` travels into a
// named-args invoke call, so an array would silently become `{0: …}`.
const isRecord = (value: unknown): value is Record<string, unknown> =>
  typeof value === "object" && value !== null && !Array.isArray(value);

export const isBridgeRequest = (data: unknown): data is BridgeRequest =>
  isRecord(data) &&
  data[BRIDGE_TAG] === "invoke" &&
  typeof data.id === "number" &&
  Number.isFinite(data.id) &&
  typeof data.command === "string" &&
  data.command.length > 0 &&
  // A missing args field is fine; anything present must be an object (or null)
  // because it travels straight into the named-args invoke call.
  (data.args === undefined || data.args === null || isRecord(data.args));

export const isBridgeClose = (data: unknown): data is BridgeClose =>
  isRecord(data) && data[BRIDGE_TAG] === "close";

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

export const isBridgeReload = (data: unknown): data is BridgeReload =>
  isRecord(data) && data[BRIDGE_TAG] === "reload";

export const isBridgeResult = (data: unknown): data is BridgeResult =>
  isRecord(data) &&
  data[BRIDGE_TAG] === "result" &&
  typeof data.id === "number" &&
  Number.isFinite(data.id) &&
  (data.ok === true
    ? "value" in data
    : data.ok === false && typeof data.error === "string");

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
