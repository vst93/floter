// hello-page — the smallest complete plugin page.
//
// It does four things, which is everything a page has to do:
//
//   1. shake hands            (`protocol.js` → `frameReady`)
//   2. read the opacity band  (bootstrap query params + the `opacity` message)
//   3. register a drag region (blank chrome → the `drag` message)
//   4. send a message         (a `host-notify` toast, and `close`)
//
// Everything it prints about the host is either a bootstrap value it read at
// load or a value the host pushed afterwards; it never reaches into the host
// document, because in the sandbox there is no host document to reach into.
//
// Plain ES module loaded by index.html — no bundler, no dependencies.

import {
  FROM_HOST,
  PROTOCOL,
  TAG,
  close,
  drag,
  frameReady,
  hostNotify,
} from "./protocol.js";

// ── 2a · bootstrap: query params are the pre-first-paint channel ──────────
//
// The host appends these to our URL. Read them synchronously, at module load,
// so the first frame already has the right theme/opacity. Every one needs a
// fallback: a host that predates the param simply does not send it.
const params = new URLSearchParams(window.location.search);
const state = {
  protocol: PROTOCOL,
  lang: params.get("lang") ?? "en",
  theme: params.get("theme") === "light" ? "light" : "dark",
  mainOpacity: numberOr(params.get("main-opacity"), 0.47),
  terminalOpacity: numberOr(params.get("terminal-opacity"), 0.46),
  glassStep: params.get("glass-step") ?? "regular",
  visible: true,
  hostMessages: 0,
};

/** Parse a bootstrap number, falling back when the param is absent/garbage. */
function numberOr(raw, fallback) {
  const value = Number(raw);
  return raw !== null && Number.isFinite(value) ? value : fallback;
}

// ── 1 · the handshake, before anything else ──────────────────────────────
//
// The host refuses every other message from a page that has not announced its
// protocol version, and shows a version-numbered error if this number does not
// match. Sending it first is not a convention, it is the gate.
window.parent.postMessage(frameReady(), "*");

// ── 2b · reading the opacity band ────────────────────────────────────────
//
// Two channels carry transparency: the bootstrap params (above, for the first
// frame) and the `opacity` message (for every slider move after that). Both
// funnel into one function so the page can never serve one and ignore the
// other.
//
// The CSS-side rule this demonstrates is the WebKit one: a `var()` is fine in
// the alpha slot, fatal in the colour-channel slot. So we write the *alpha* as
// a custom property and keep the RGB triple a literal in the stylesheet.
function applyOpacity(mainOpacity, terminalOpacity) {
  state.mainOpacity = mainOpacity;
  state.terminalOpacity = terminalOpacity;
  const root = document.documentElement.style;
  // A page field that visibly follows the slider (0.25 → 0.95 across 0 → 1),
  // matching the shape the built-in clipboard page uses.
  const field = 0.25 + 0.7 * clamp01(terminalOpacity);
  const row = Math.min(1, field + 0.08);
  root.setProperty("--field-alpha", String(field));
  root.setProperty("--row-alpha", String(row));
  render();
}

const clamp01 = (value) => Math.min(1, Math.max(0, Number.isFinite(value) ? value : 0));

// ── 4a · sending a message: a host-owned toast ───────────────────────────
//
// A page cannot paint its own notice — the toast stack, its position and its
// lifetime all live in the host. It therefore names a *dictionary key* and the
// host translates it. `plugin.protocolMismatch` is one of the host's own keys,
// though any key in its dictionary works.
let nextMessageId = 1;

function notify() {
  window.parent.postMessage(
    // `notify-retry` would come back here if retryable were true; this demo has
    // nothing to re-run, so it sends a plain success notice.
    hostNotify(nextMessageId++, "success", "plugin.exampleNotify", false),
    "*",
  );
}

// ── 3 · the drag region ──────────────────────────────────────────────────
//
// A sandboxed page never sees the host's mousedown, and
// `data-tauri-drag-region` only works inside the host document. So a press on
// blank chrome is reported as intent and the host runs its own window move.
// Interactive elements are exempted *here*, before anything crosses the wire.
const DRAG_EXEMPT = "button, input, textarea, select, a, [data-no-drag]";
const isDragTarget = (target) => !!target && !target.closest(DRAG_EXEMPT);

document.addEventListener("mousedown", (event) => {
  if (event.button !== 0) return;              // primary button only
  if (!isDragTarget(event.target)) return;     // never from a control or the scroller
  event.preventDefault();                       // do not start a text selection
  window.parent.postMessage(drag(), "*");
});

// ── 4b · the host's messages ─────────────────────────────────────────────
//
// Only the parent window may talk to us, and only tagged objects are ours.
// Unknown tags are ignored — that is how a page survives a host that is newer
// than it is.
window.addEventListener("message", (event) => {
  if (event.source !== window.parent) return;
  const data = event.data;
  if (!data || typeof data !== "object" || data[TAG] === undefined) return;
  state.hostMessages += 1;

  switch (data[TAG]) {
    case FROM_HOST.opacity:
      applyOpacity(data.mainOpacity, data.terminalOpacity);
      return;
    case FROM_HOST.theme:
      state.theme = data.theme;
      render();
      return;
    case FROM_HOST.glass:
      state.glassStep = data.glassStep;
      render();
      return;
    case FROM_HOST.visibility:
      state.visible = data.visible;
      render();
      return;
    case FROM_HOST.reload:
      // The page was revealed again: re-read whatever data it shows.
      render();
      return;
    default:
      // `result`, `notify-retry` and anything a newer host invents: not part
      // of this demo, ignored rather than errored on.
      return;
  }
});

// ── paint + wire the two controls ────────────────────────────────────────
const output = document.getElementById("state");
const notifyButton = document.getElementById("notify");
const closeButton = document.getElementById("close");

function render() {
  output.textContent = JSON.stringify(
    {
      protocol: state.protocol,
      theme: state.theme,
      glassStep: state.glassStep,
      mainOpacity: round(state.mainOpacity),
      terminalOpacity: round(state.terminalOpacity),
      visible: state.visible,
      hostMessages: state.hostMessages,
    },
    null,
    2,
  );
  document.documentElement.dataset.theme = state.theme;
}

const round = (value) => Math.round(value * 1000) / 1000;

// `mousedown` is prevented so the click does not first blur/blur the page.
notifyButton.addEventListener("mousedown", (event) => event.preventDefault());
notifyButton.addEventListener("click", notify);

// 5 · close: the host returns to the surface it came from.
closeButton.addEventListener("mousedown", (event) => event.preventDefault());
closeButton.addEventListener("click", () => window.parent.postMessage(close(), "*"));

applyOpacity(state.mainOpacity, state.terminalOpacity);
