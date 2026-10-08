// R156 · the rendering engine's own version, read off the user agent so the
// About page can report it.
//
// The round that added this could not reproduce the Windows report on the
// Linux gate, and the one number that decides between the remaining mechanisms
// is the WebView2 runtime's version: the transparent-window regression tracked
// as WebView2Feedback#5481 / #5752 is a *runtime* behaviour (a theme-derived
// base painted over `DefaultBackgroundColor` alpha=0), it arrived in a stable
// channel and was fixed and broken again across channels, so "which runtime is
// this machine actually running" is not a question the source tree can answer.
// The About page is where a user already reads the app's version, so the
// engine's version belongs next to it: one line, no controls, and a retest can
// quote it instead of guessing.
//
// The parse is deliberately tiny and total. WebView2 identifies itself as Edge
// (`Edg/<version>`) and carries the same Chromium build in `Chrome/<version>`;
// anything else that reports `Chrome/<version>` is the Linux WebKitGTK shell
// (which reports `Safari/<version>` instead) or a plain browser, so the
// fallback is "Chromium". macOS's WKWebView and Linux's WebKitGTK match
// neither and return `null`, which is what keeps the row off every platform
// that has no WebView2 — the probe is cross-platform harmless by being absent.

/** The engine a window is running on, and its full version string. */
export type WebViewRuntime = {
  /** `WebView2` when the user agent identifies Edge, `Chromium` otherwise. */
  runtime: "WebView2" | "Chromium";
  /** The version as the engine reports it, e.g. `154.0.4258.62`. */
  version: string;
};

/**
 * The engine version in a user agent, or `null` when the engine is not one of
 * the Chromium family (WKWebView, WebKitGTK) and there is nothing worth
 * printing.
 *
 * Edge first: WebView2's user agent is Edge's, and `Chrome/` on the same
 * string is the Chromium build the Edge version is derived from — the runtime
 * number a WebView2 bug report needs is the Edge one.
 */
export const parseWebViewRuntime = (userAgent: string): WebViewRuntime | null => {
  const edge = /Edg(?:e|A|iOS)?\/([\d.]+)/.exec(userAgent);
  if (edge) return { runtime: "WebView2", version: edge[1] };

  const chromium = /(?:Chrome|CriOS)\/([\d.]+)/.exec(userAgent);
  if (chromium) return { runtime: "Chromium", version: chromium[1] };

  return null;
};

/** The label the About row prints: engine name and version, one string. */
export const formatWebViewRuntime = (runtime: WebViewRuntime): string =>
  `${runtime.runtime} ${runtime.version}`;
