// The message vocabulary of the hello-page example, in one importable module.
//
// Why a module and not a snippet inside `index.html`: the protocol's message
// names have to match the host's exactly, and the host's are declared in
// `src/plugin-pages.ts`. Keeping them here means the example, its README and
// `tests/plugin-page-protocol.test.ts` all read the same strings — a typo in
// one place cannot silently make the example wrong.
//
// Plain ES module, no build step: a browser loads it directly next to
// `index.html`.

/** The plugin-page protocol this page was written against. Must equal the
 * host's `PLUGIN_PAGE_PROTOCOL` (`src/plugin-pages.ts`); the host refuses the
 * handshake otherwise. */
export const PROTOCOL = 1;

/** The property every bridge message carries. */
export const TAG = "floter";

/** Page → host message names. */
export const TO_HOST = {
  frameReady: "frame-ready",
  invoke: "invoke",
  close: "close",
  drag: "drag",
  hostNotify: "host-notify",
};

/** Host → page message names. */
export const FROM_HOST = {
  result: "result",
  opacity: "opacity",
  theme: "theme",
  glass: "glass",
  visibility: "visibility",
  reload: "reload",
  notifyRetry: "notify-retry",
};

/** Handshake. Send this before anything else — the host drops every other
 * message from a page that has not announced its protocol version. */
export const frameReady = (protocol = PROTOCOL) => ({
  [TAG]: TO_HOST.frameReady,
  protocol,
});

/** Run one allowlisted host command. `id` is ours; the host echoes it back. */
export const invoke = (id, command, args = {}, session) => ({
  [TAG]: TO_HOST.invoke,
  id,
  ...(session === undefined ? {} : { session }),
  command,
  args,
});

/** Raise one host toast. `messageKey` names a key in the *host's* dictionary. */
export const hostNotify = (id, kind, messageKey, retryable = false) => ({
  [TAG]: TO_HOST.hostNotify,
  id,
  kind,
  messageKey,
  ...(retryable ? { retryable: true } : {}),
});

/** Ask the user's escape hatch to close this page. */
export const close = () => ({ [TAG]: TO_HOST.close });

/** Report a press on blank chrome as a window-drag intent. Payload-free by
 * design: the host executes its own drag path and takes nothing from us. */
export const drag = () => ({ [TAG]: TO_HOST.drag });
