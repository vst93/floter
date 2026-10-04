// R84 · the detached plugin window's React surface.
//
// One run per mount cycle: the backend parks a `DetachRequest` in a pull slot
// before this window even exists, the view pulls it on mount (and again on
// every `plugin-detach-request` emit while it is open), runs the command
// through the same `external_plugin_run` the launcher field uses, and renders
// the output through the same dual-form pipeline the launcher's plugin page
// uses (text verbatim under a header, or standard rows as a plain list). What
// it deliberately does NOT have: hide-on-blur, an Esc handler, a summon
// lifecycle — the user pinned this window so it would stay put while the
// launcher card comes and goes.
//
// The R65 「选中即复制」 gesture rides along: selection inside the output is
// reported to the copy chokepoint, exactly as `PluginTextView` does inside the
// launcher.

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { invoke } from "@tauri-apps/api/core";
import { getCurrentWindow } from "@tauri-apps/api/window";
import { X } from "lucide-react";

import { createTranslator, normalizeLanguage, type Translate } from "../i18n.ts";
import type { AppSettings } from "../App";
import type { ExternalRunOutput } from "../plugins/external.ts";
import { externalRunText } from "../plugins/external.ts";
import { resolvePluginView, type PluginView } from "../launcher/plugin-mode.ts";
import { PluginTextView } from "../launcher/PluginTextView.tsx";
import {
  validateDetachRequest,
  type DetachRequest,
} from "./detach.ts";

/** One window, one language decision: read the persisted language once at
 *  mount and build the translator. A language flip in the launcher's settings
 *  reaches this window on the next detach (new title) — the same settle time
 *  the tray labels take. */
const useDetachedTranslator = (): Translate => {
  const [t, setT] = useState<Translate>(() => createTranslator("en"));
  useEffect(() => {
    let alive = true;
    invoke<AppSettings>("get_settings")
      .then((settings) => {
        if (alive) setT(createTranslator(normalizeLanguage(settings.language)));
      })
      .catch(() => undefined);
    return () => {
      alive = false;
    };
  }, []);
  return t;
};

/** The detached view's whole state machine: the request being shown, the run
 *  attached to it, and the copy notice the R65 gesture reports into. */
type DetachedState = {
  request: DetachRequest | null;
  status: "idle" | "running" | "done" | "failed";
  output: ExternalRunOutput | null;
  failure: string | null;
};

/** Pull the pending request exactly once. The backend clears its slot on read,
 *  so a remount does not re-run an old command against the user's intent. */
const takePendingRequest = async (): Promise<DetachRequest | null> => {
  const raw = await invoke<unknown>("take_plugin_window_request");
  return validateDetachRequest(raw);
};

/** Run the request's command. Same command, same contract as the field:
 *  argv in, one `ExternalRunOutput` out, no shell. */
const runRequest = async (request: DetachRequest): Promise<ExternalRunOutput> =>
  invoke<ExternalRunOutput>("external_plugin_run", {
    extensionId: request.extensionId,
    commandId: request.commandId,
    args: [...request.args],
  });

export default function DetachedPluginApp() {
  const t = useDetachedTranslator();
  const [state, setState] = useState<DetachedState>({
    request: null,
    status: "idle",
    output: null,
    failure: null,
  });
  const outputRef = useRef<HTMLDivElement | null>(null);
  // A stale response from an earlier run must never paint over a newer one —
  // the same generation guard `runExternalCommand` uses in the launcher.
  const runGeneration = useRef(0);

  const run = useCallback((request: DetachRequest) => {
    const generation = ++runGeneration.current;
    setState({ request, status: "running", output: null, failure: null });
    // The window title follows the command, so the taskbar entry names what
    // is pinned (the Rust builder sets the same title for the first paint).
    void getCurrentWindow()
      .setTitle(`${request.commandLabel} · floter`)
      .catch(() => undefined);
    runRequest(request)
      .then((output) => {
        if (runGeneration.current !== generation) return;
        setState({ request, status: "done", output, failure: null });
      })
      .catch((error) => {
        if (runGeneration.current !== generation) return;
        setState({
          request,
          status: "failed",
          output: null,
          failure: String(error),
        });
      });
  }, []);

  // Mount: pull the slot the backend filled before the window existed.
  useEffect(() => {
    let alive = true;
    takePendingRequest()
      .then((request) => {
        if (alive && request) run(request);
      })
      .catch(() => undefined);
    return () => {
      alive = false;
    };
  }, [run]);

  // Open window, second detach: the backend emits on this label; replace the
  // run. Same slot contract — the emit is a nudge, the truth is the slot.
  useEffect(() => {
    let unlisten: (() => void) | undefined;
    let alive = true;
    void getCurrentWindow()
      .listen("plugin-detach-request", () => {
        takePendingRequest()
          .then((request) => {
            if (alive && request) run(request);
          })
          .catch(() => undefined);
      })
      .then((dispose) => {
        if (alive) unlisten = dispose;
        else dispose();
      });
    return () => {
      alive = false;
      unlisten?.();
    };
  }, [run]);

  // R65 · the copy gesture: selection inside the output copies. The chokepoint
  // and the notice are this view's own (the launcher's notice is inside the
  // card, which this window is not).
  const [copyNotice, setCopyNotice] = useState<string | null>(null);
  useEffect(() => {
    const onMouseUp = () => {
      const selection = window.getSelection();
      if (!selection || selection.isCollapsed) return;
      const container = outputRef.current;
      if (!container || !container.contains(selection.anchorNode)) return;
      const text = selection.toString();
      if (text.length === 0) return;
      void navigator.clipboard
        .writeText(text)
        .then(() => {
          setCopyNotice(t("pluginWindow.copied"));
        })
        .catch(() => undefined);
    };
    document.addEventListener("mouseup", onMouseUp);
    return () => document.removeEventListener("mouseup", onMouseUp);
  }, [t]);

  const rerun = useCallback(() => {
    if (state.request) run(state.request);
  }, [run, state.request]);

  const close = useCallback(() => {
    void invoke("close_plugin_window").catch(() => undefined);
  }, []);

  // The same dual-form resolve the launcher's catalog hook feeds. The text
  // form reuses `PluginTextView` verbatim (its metrics band the height); the
  // list form renders the standard rows as a plain, non-interactive list — a
  // pinned window shows an answer, not a second search page.
  const view: PluginView | null = useMemo(() => {
    if (state.status !== "done" || !state.output) return null;
    const text = externalRunText(state.output);
    if (text === null) return null;
    return resolvePluginView({ output: text });
  }, [state.status, state.output]);

  const headerLabel = state.request?.commandLabel ?? t("pluginWindow.fallbackTitle");

  return (
    <div className="plugin-window">
      <header className="plugin-window__bar">
        <span className="plugin-window__title" title={headerLabel}>
          {headerLabel}
        </span>
        {state.request && (
          <div className="plugin-window__bar-actions">
            <button
              type="button"
              className="plugin-window__bar-button"
              onClick={rerun}
              disabled={state.status === "running"}
              title={t("pluginWindow.rerun")}
              aria-label={t("pluginWindow.rerun")}
            >
              {t("pluginWindow.rerun")}
            </button>
            <button
              type="button"
              className="plugin-window__bar-button"
              onClick={close}
              title={t("pluginWindow.close")}
              aria-label={t("pluginWindow.close")}
            >
              <X size={16} strokeWidth={1.8} aria-hidden="true" />
            </button>
          </div>
        )}
      </header>
      <div className="plugin-window__body" ref={outputRef}>
        {state.status === "running" && (
          <div className="plugin-window__status">{t("launcher.externalRunning")}</div>
        )}
        {state.status === "failed" && (
          <div className="plugin-window__status plugin-window__status--failed">
            {state.failure || t("launcher.externalFailed")}
          </div>
        )}
        {state.status === "idle" && !state.request && (
          <div className="plugin-window__status">{t("pluginWindow.idle")}</div>
        )}
        {state.status === "done" &&
          (view === null ? (
            <div className="plugin-window__status">{t("launcher.externalEmpty")}</div>
          ) : view.form === "text" ? (
            <PluginTextView
              t={t}
              text={view.text}
              metrics={view.metrics}
              onCopySelection={() => setCopyNotice(t("pluginWindow.copied"))}
            />
          ) : (
            <ul className="plugin-window__list">
              {view.items.map((item) => (
                <li key={item.id} className="plugin-window__row">
                  <span className="plugin-window__row-title">{item.title}</span>
                  {"subtitle" in item && item.subtitle && (
                    <span className="plugin-window__row-subtitle">{item.subtitle}</span>
                  )}
                </li>
              ))}
            </ul>
          ))}
      </div>
      {copyNotice && (
        <div className="plugin-window__notice" role="status" aria-live="polite">
          {copyNotice}
        </div>
      )}
    </div>
  );
}
