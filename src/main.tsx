import ReactDOM from "react-dom/client";
import { lazy, Suspense } from "react";
import { getCurrentWindow } from "@tauri-apps/api/window";

import App from "./App";
import { isPluginWindowLabel } from "./plugin-window/detach";
import { ErrorBoundary } from "./components/ErrorBoundary";

// R84 · one bundle, two kinds of window. The launcher card (`main`) keeps the
// whole App; a detached plugin window (`plugin-detached`, `plugin-detached-2`,
// … built by `detach_plugin_window` in lib.rs) renders only the pinned run's
// view. The label is the branch the Rust side names when it builds the window,
// so the two sides cannot disagree without a test noticing (see
// plugin-window.test.ts and the capability file).
//
// R149 · the detached view is a second kind of window the launcher card never
// renders, so it is split out of the first-frame bundle and pulled only when
// the label says this process is one. The main card stops parsing
// `app-pluginwindow`; a detached window pays one local chunk fetch for its own
// view (R143 §3.2).
const DetachedPluginApp = lazy(() => import("./plugin-window/DetachedPluginApp"));
const label = getCurrentWindow().label;

// R113 · one boundary at the one mount point. A render-time throw in either
// window kind lands on the fallback row instead of a blank window; the retry
// re-renders the same subtree. See `components/ErrorBoundary.tsx`.
ReactDOM.createRoot(document.getElementById("root") as HTMLElement).render(
  <ErrorBoundary>
    {isPluginWindowLabel(label) ? (
      <Suspense fallback={null}>
        <DetachedPluginApp />
      </Suspense>
    ) : (
      <App />
    )}
  </ErrorBoundary>,
);

