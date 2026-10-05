// R84 · the detached plugin window's React surface.
//
// One request per mount cycle: the backend parks a `DetachRequest` in a pull
// slot before this window even exists, the view pulls it on mount (and again on
// every `plugin-detach-request` emit while it is open), and renders it. R90
// split the request into two arms: an `external` one runs the command through
// the same `external_plugin_run` the launcher field uses and renders the output
// through the same dual-form pipeline the launcher's plugin page uses (text
// verbatim under a header, or standard rows as a plain list); a `text` one is a
// frozen snapshot — a history line, a clipboard entry, a plugin row's text —
// with no command to run, drawn through the same `PluginTextView`. What the
// window deliberately does NOT have: hide-on-blur, an Esc handler, a summon
// lifecycle — the user pinned this window so it would stay put while the
// launcher card comes and goes.
//
// The R65 「选中即复制」 gesture rides along: selection inside the output is
// reported to the copy chokepoint, exactly as `PluginTextView` does inside the
// launcher.

import { useCallback, useEffect, useMemo, useRef, useState, type UIEvent } from "react";
import { invoke } from "@tauri-apps/api/core";
import { getCurrentWindow } from "@tauri-apps/api/window";
import { X } from "lucide-react";

import { createTranslator, normalizeLanguage, type Translate } from "../i18n.ts";
import type { AppSettings } from "../App";
import type { ExternalRunOutput } from "../plugins/external.ts";
import { externalRunText } from "../plugins/external.ts";
import {
  PLUGIN_INITIAL_PAGES,
  PLUGIN_LOAD_MORE_THRESHOLD,
  pagePluginEmission,
  resolvePluginView,
  pluginTextMetrics,
  type PluginView,
} from "../launcher/plugin-mode.ts";
import { PluginTextView } from "../launcher/PluginTextView.tsx";
import { selectionTextIn } from "../launcher/plugin-text-copy.ts";
import { runErrorMessage } from "../extensions/run-errors.ts";
import { useCopyNotice } from "../hooks/useCopyNotice.ts";
import { useLauncherTextCopy } from "../hooks/useLauncherTextCopy.ts";
import {
  validateDetachRequest,
  type DetachRequest,
  type ExternalDetachRequest,
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

/** Pull the pending request exactly once. The backend keys the slot by the
 *  window's own label and marks it delivered on read, so a remount does not
 *  re-run an old command against the user's intent. */
const takePendingRequest = async (): Promise<DetachRequest | null> => {
  const label = getCurrentWindow().label;
  const raw = await invoke<unknown>("take_plugin_window_request", { label });
  return validateDetachRequest(raw);
};

/** Run the request's command. Same command, same contract as the field:
 *  argv in, one `ExternalRunOutput` out, no shell. Only the `external` arm has
 *  a command to run; a `text` request is a snapshot and never reaches here. */
const runRequest = async (request: ExternalDetachRequest): Promise<ExternalRunOutput> =>
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

  // R95 · the list arm pages exactly as the launcher's does: the shared protocol
  // rule (`pagePluginEmission`), the shared page size and the shared first
  // window (`PLUGIN_INITIAL_PAGES`). A new request is a new result set, so the
  // window resets to the first pages in `run` below.
  const [pluginPages, setPluginPages] = useState(PLUGIN_INITIAL_PAGES);
  // One append in flight at a time. The append is synchronous — there is no
  // fetch to wait for — so the guard is released by the render the increment
  // causes, never by a frame (R72 recorded that an inactive WebView stops
  // firing `requestAnimationFrame`; a list that appends from memory has nothing
  // to defer).
  const pluginLoadingRef = useRef(false);
  useEffect(() => {
    pluginLoadingRef.current = false;
  }, [pluginPages]);

  const run = useCallback((request: DetachRequest) => {
    const generation = ++runGeneration.current;
    setPluginPages(PLUGIN_INITIAL_PAGES);
    // The window title follows the request, so the taskbar entry names what
    // is pinned (the Rust builder sets the same title for the first paint).
    // Both arms have a title: the external arm's command label, the text
    // arm's own row title.
    const windowTitle = request.kind === "text" ? request.title : request.commandLabel;
    void getCurrentWindow()
      .setTitle(`${windowTitle} · floter`)
      .catch(() => undefined);
    // R90 · the `text` arm has no run state machine: the content *is* the
    // request, so it goes straight to `done` and the view below renders it
    // through `PluginTextView`. Nothing is invoked; a remount cannot re-run
    // anything because there is nothing to run.
    if (request.kind === "text") {
      setState({ request, status: "done", output: null, failure: null });
      return;
    }
    setState({ request, status: "running", output: null, failure: null });
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

  // R65 · the copy gesture: selection inside the output copies. R93 · this view
  // reuses the launcher's own pieces instead of a second implementation — the
  // pure `selectionTextIn` rule for "is this selection ours?", the shared
  // `useCopyNotice` phase machine, and the one copy chokepoint
  // `useLauncherTextCopy` (the backend write the launcher's other copies use,
  // and the only one that works where `navigator.clipboard` is absent). The
  // notice is the detached window's own status line; only the vocabulary is
  // shared with the launcher.
  const { copyNotice, showCopyNotice } = useCopyNotice();
  const { copySelection } = useLauncherTextCopy(showCopyNotice);

  const rerun = useCallback(() => {
    // R90 · only the external arm has a command to re-run; a text snapshot is
    // static by definition (the button is not even drawn for it).
    if (state.request?.kind === "external") run(state.request);
  }, [run, state.request]);

  const close = useCallback(() => {
    void invoke("close_plugin_window", { label: getCurrentWindow().label }).catch(
      () => undefined,
    );
  }, []);

  // The same dual-form resolve the launcher's catalog hook feeds. R90 · the
  // text arm bypasses it: the request's own string is the content, and it is
  // measured with the same `pluginTextMetrics` the resolve would use — an empty
  // snapshot included (it is still a window with a body, not an empty state).
  // The external arm keeps its run pipeline untouched.
  const view: PluginView | null = useMemo(() => {
    if (state.request?.kind === "text" && state.status === "done") {
      return {
        form: "text",
        tier: "display",
        text: state.request.text,
        metrics: pluginTextMetrics(state.request.text),
      };
    }
    if (state.status !== "done" || !state.output) return null;
    const text = externalRunText(state.output);
    if (text === null) return null;
    // R95 · the list arm windows the same way the launcher's catalog hook does.
    // `pagePluginEmission` returns a list that fits the window untouched — a
    // non-list body (text) and a list of at most one page are byte-identical to
    // the pre-R95 resolve, which is the regression the ≤-page-size guard pins.
    return resolvePluginView(pagePluginEmission({ output: text }, pluginPages));
  }, [state.status, state.output, state.request, pluginPages]);

  // R95 · the scroll-to-bottom trigger, the launcher's own rule: only a list
  // that actually has another page reacts, and the trigger reads the scroller's
  // geometry against the shared threshold, so a window resize needs no second
  // budget. The ref lets the once-created handler see the current bit without
  // being rebuilt.
  const pluginHasMore =
    view !== null &&
    view.form === "list" &&
    view.page !== null &&
    view.page.hasMore;
  const pluginHasMoreRef = useRef(pluginHasMore);
  pluginHasMoreRef.current = pluginHasMore;

  const loadMorePluginPage = useCallback(() => {
    if (pluginLoadingRef.current || !pluginHasMoreRef.current) return;
    pluginLoadingRef.current = true;
    setPluginPages((pages) => pages + 1);
  }, []);

  const onBodyScroll = useCallback(
    (event: UIEvent<HTMLDivElement>) => {
      if (!pluginHasMoreRef.current) return;
      const node = event.currentTarget;
      const remaining = node.scrollHeight - node.scrollTop - node.clientHeight;
      if (remaining <= PLUGIN_LOAD_MORE_THRESHOLD) loadMorePluginPage();
    },
    [loadMorePluginPage],
  );

  // R65 · the list arm's copy-on-select. The text arm renders `PluginTextView`,
  // which owns the identical gesture for its own block (armed by a mousedown
  // inside it and consumed through `selectionTextIn`), so a body-wide listener
  // there too would write the clipboard twice; this one is therefore scoped to
  // the list arm. Its container is the body itself, the same one the old
  // hand-written check used.
  useEffect(() => {
    if (view?.form === "text") return;
    const onMouseUp = () => {
      const selected = selectionTextIn(outputRef.current, window.getSelection());
      if (selected !== null) void copySelection(selected);
    };
    document.addEventListener("mouseup", onMouseUp);
    return () => document.removeEventListener("mouseup", onMouseUp);
  }, [copySelection, view?.form]);

  const headerLabel =
    state.request === null
      ? t("pluginWindow.fallbackTitle")
      : state.request.kind === "text"
        ? state.request.title
        : state.request.commandLabel;

  return (
    <div className="plugin-window">
      <header className="plugin-window__bar">
        <span className="plugin-window__title" title={headerLabel}>
          {headerLabel}
        </span>
        {state.request && (
          <div className="plugin-window__bar-actions">
            {/* R90 · a text snapshot has no command to re-run, so the bar shows
                only Close. The external arm keeps its Rerun button. */}
            {state.request.kind === "external" && (
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
            )}
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
      <div className="plugin-window__body" ref={outputRef} onScroll={onBodyScroll}>
        {state.status === "running" && (
          <div className="plugin-window__status">{t("launcher.externalRunning")}</div>
        )}
        {state.status === "failed" && (
          <div className="plugin-window__status plugin-window__status--failed">
            {/* R93 · the backend answers a failed run with a stable key plus a
                JSON payload, never a sentence; `runErrorMessage` localises it
                exactly as the settings panel does. A message it does not
                recognise keeps the old `failure || generic` fallback. */}
            {runErrorMessage(state.failure ?? "", t) ||
              state.failure ||
              t("launcher.externalFailed")}
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
              onCopySelection={copySelection}
            />
          ) : (
            <ul className="plugin-window__list">
              {view.items.map((item) =>
                // R95 · a status row is information *about* the list, not an
                // entry in it, so it wears the muted note the launcher draws
                // for the same row (`launcher-status`) instead of the row plate.
                // Everything else in the detached list is read-only by
                // construction, so the launcher's other row chrome — selection,
                // `⌘N`, the source word — has no counterpart here (see the R95
                // report's row-by-row evaluation).
                item.type === "status" ? (
                  <li key={item.id} className="plugin-window__status" title={item.title}>
                    {item.title}
                  </li>
                ) : (
                  <li key={item.id} className="plugin-window__row">
                    <span className="plugin-window__row-title">{item.title}</span>
                    {"subtitle" in item && item.subtitle && (
                      <span className="plugin-window__row-subtitle">{item.subtitle}</span>
                    )}
                  </li>
                ),
              )}
            </ul>
          ))}
        {/* R93 · the notice is the detached window's own status line (not a
            floating layer), driven by the shared phase machine — it clears
            itself after the visible/fade window. */}
        {copyNotice.message && (
          <div className="plugin-window__status" role="status" aria-live="polite">
            {t(copyNotice.message)}
          </div>
        )}
      </div>
    </div>
  );
}
