import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import type { MouseEvent as ReactMouseEvent } from "react";
import { invoke } from "@tauri-apps/api/core";
import {
  BRIDGE_TAG,
  PLUGIN_PAGE_PROTOCOL,
  buildPluginPageUrl,
  commandAllowed,
  handshakeErrorDetail,
  isBridgeClose,
  isBridgeDrag,
  isBridgeFrameReady,
  isBridgeNotify,
  isBridgeRequest,
  isBridgeResult,
  pluginPageHandshake,
  pluginPageNeedsSameOrigin,
  shouldStartWindowDrag,
} from "../plugin-pages";
import type { BridgeNotifyRetry, BridgeOpacity, BridgeTheme, BridgeReload, BridgeVisibility, BridgeGlass, HandshakeVerdict } from "../plugin-pages";
import { glassStepStyle, glassContentStyle, type GlassStep } from "../glass-material";
import { createTranslator, isMessageKey, type Language, type MessageKey } from "../i18n";
import type { ToastAction } from "../toast-state";

/**
 * Host side of the generic plugin-page mechanism.
 *
 * Renders the active plugin's HTML page inside a sandboxed iframe occupying
 * the whole terminal-panel canvas (the window geometry itself is owned by the
 * backend's `show_plugin_page`), and bridges postMessage traffic to
 * `invoke()` behind the page's per-plugin command allowlist.
 *
 * Why an iframe rather than injecting the HTML into the app document:
 * external plugin HTML is less trusted than our own. A sandboxed frame without
 * `allow-same-origin` gets an opaque origin — no DOM access to the host app, no
 * Tauri IPC surface at all. The built-in clipboard page is the one trusted
 * exception: WebKit refuses its same-origin stylesheet request from an opaque
 * sandbox, so that page opts into `allow-same-origin` while remaining sandboxed.
 * Its only host capability is still the allowlisted bridge.
 */

/** How long to wait for the backend's page descriptor before showing the retry
 * state. Mirrors the page-side bridge timeout in `clipboard/main.ts` so both
 * ends of the pipeline give up on a silent host after the same delay. */
const DESCRIPTOR_TIMEOUT_MS = 10_000;

/**
 * How long a loaded document has to announce its protocol version before the
 * host gives up on it. A page that fires `load` and then says nothing is not a
 * page this host can talk to; showing the refused-handshake error names the
 * version the host speaks instead of leaving the opaque loading placeholder up
 * forever. The same 10s window as the descriptor fetch, so the two stages of a
 * load fail on the same clock.
 */
const HANDSHAKE_TIMEOUT_MS = 10_000;

export type PluginPageDescriptorInfo = {
  id: string;
  titleKey: string;
  page: string;
  allowedCommands: string[];
};

type PluginPageHostProps = {
  pluginId: string | null;
  /** Bootstrap values handed to the page via query params. */
  language: Language;
  theme: "dark" | "light";
  mainOpacity: number;
  terminalOpacity: number;
  glassStep: GlassStep;
  onClose: () => void;
  /**
   * Raise one feedback toast on the app's own stack (see App.tsx's `notify`).
   * A plugin page cannot paint host chrome — its document is a sandboxed
   * iframe — so its failures travel here and land on the same `toast-state`
   * pipeline every other surface uses: one position, one lifetime, one
   * visual.
   *
   * `action` is the optional retry slot. The page names a dictionary key over
   * the bridge (never text), this host resolves it against the app's own
   * dictionary and drops unknown keys, and the retry closure is built *here*,
   * next to the iframe it has to message.
   */
  onNotify: (kind: "error" | "success", text: string, action?: ToastAction) => void;
  /**
   * Host-owned window drag, wired to the *same* `startDrag` handler the three
   * shells use (see App.tsx). Reusing it — rather than a new
   * `data-tauri-drag-region` attribute — is deliberate: `startDrag` carries the
   * Windows blur-grace logic and the "never drag from a button/input" guard, so
   * the plugin chrome and the terminal/settings chrome stay behaviourally
   * identical. In plugin mode the terminal bar is not rendered (the branch
   * paints only the rounded backdrop), so this is the sole drag region and the
   * two can never fight over a mousedown.
   */
  onDragStart: (event: ReactMouseEvent) => void;
  /**
   * The drag *action* on its own, with no event attached: what a plugin page
   * asks for over the bridge when the user presses its blank header. A page
   * runs in a sandboxed iframe and cannot start a native window drag, so the
   * press is reported as a `drag` message and the host runs the same move the
   * `onDragStart` handler would — same platform path, same Windows blur-grace.
   * The page has already applied the interactive-element guard, so there is no
   * event to inspect here.
   */
  onWindowDrag: () => void;
};

export function PluginPageHost({
  pluginId,
  language,
  theme,
  mainOpacity,
  terminalOpacity,
  glassStep,
  onClose,
  onDragStart,
  onWindowDrag,
  onNotify,
}: PluginPageHostProps) {
  const iframeRef = useRef<HTMLIFrameElement>(null);
  /** The pending handshake deadline for the current document (see
   * [`HANDSHAKE_TIMEOUT_MS`]); cleared the moment the page announces itself. */
  const handshakeTimerRef = useRef<number | undefined>(undefined);
  const [descriptor, setDescriptor] = useState<PluginPageDescriptorInfo | null>(null);
  const [loadFailed, setLoadFailed] = useState(false);
  /** Bumped by the error state's retry button to re-run the descriptor fetch. */
  const [reloadNonce, setReloadNonce] = useState(0);
  // Handshake state: the protocol verdict for the current document. `null`
  // means the document has loaded but has not announced a version yet (or no
  // document is loaded). Only `{ status: "accepted" }` opens the bridge and
  // lets the host push settings; `missing` / `mismatch` paint the error state
  // with the version numbers that explain themselves.
  const [handshake, setHandshake] = useState<HandshakeVerdict | null>(null);
  /** The `src` the current verdict belongs to. A new document invalidates it. */
  const [handshakeSrc, setHandshakeSrc] = useState<string | null>(null);
  /** True once the page has passed the protocol handshake and can be spoken to. */
  const ready = handshake?.status === "accepted";
  /** The handshake came back with a version the host does not speak (or none at
   * all): the page is refused and shown the error state instead of its frame. */
  const refused = handshake !== null && handshake.status !== "accepted";
  // Ref mirror of `handshake` for the once-registered message listener (same
  // reasoning as `allowedRef`): the listener must gate on the *current* verdict
  // without resubscribing on every handshake transition.
  const handshakeRef = useRef<HandshakeVerdict | null>(null);
  handshakeRef.current = handshake;
  // Ref mirror so the once-registered message listener always sees the current
  // allowlist without resubscribing on re-render.
  const allowedRef = useRef<readonly string[]>([]);
  const activeRef = useRef(pluginId);
  activeRef.current = pluginId;
  // Read at message time, never captured: the message listener registers once
  // and the App callbacks behind these are stable (`useCallback`), but keeping
  // them in a ref means a future non-stable callback cannot silently bind a
  // stale closure to the bridge for the life of the page.
  const notifyRef = useRef(onNotify);
  notifyRef.current = onNotify;
  // Same once-registered-listener reasoning as `notifyRef`: a future unstable
  // App callback must not bind a stale drag handler for the life of the page.
  const windowDragRef = useRef(onWindowDrag);
  windowDragRef.current = onWindowDrag;
  // Read (not depended on) when building the iframe src, so slider moves
  // reach a live page as a message instead of as a remount. The glass step is
  // read the same way for its message push, but its *container* style below
  // depends on it directly.
  const opacityRef = useRef({ mainOpacity, terminalOpacity });
  opacityRef.current = { mainOpacity, terminalOpacity };
  // The descriptor's human name arrives as an i18n KEY (`titleKey`); translate
  // it here so the iframe's accessible name reads as a label, never as a raw
  // dictionary key.
  const t = useMemo(() => createTranslator(language), [language]);
  // The message listener below registers once (it must not be resubscribed on
  // every render, or a message could be handled twice), so the translator it
  // uses has to be read through a ref: a language change mid-session must not
  // leave the bridge translating into the old dictionary.
  const tRef = useRef(t);
  tRef.current = t;

  useEffect(() => {
    if (!pluginId) {
      // pluginId went null: the page was closed. Keep the iframe alive but hidden
      // so reopening is instant. Don't clear descriptor/src — the page stays
      // mounted in the background.
      return;
    }
    let cancelled = false;
    setLoadFailed(false);
    // Only fetch descriptor if we don't have one yet, or pluginId changed to
    // a different plugin (switching pages, not toggling the same one).
    if (descriptor && descriptor.id === pluginId) {
      // Same page as before: already loaded, just show it and tell it to reload.
      if (ready && iframeRef.current?.contentWindow) {
        const reloadMessage: BridgeReload = { [BRIDGE_TAG]: "reload" };
        iframeRef.current.contentWindow.postMessage(reloadMessage, "*");
      }
      return;
    }
    setDescriptor(null);
    allowedRef.current = [];
    // A descriptor fetch that never settles would leave this host rendering
    // `null` forever (see the JSX below) — and because the window itself is
    // transparent, an empty host is not a blank panel but a see-through hole
    // onto the desktop, with no error state and no way back. Time it out so a
    // hung command surfaces the retry instead of an invisible surface.
    const timer = window.setTimeout(() => {
      if (!cancelled) setLoadFailed(true);
    }, DESCRIPTOR_TIMEOUT_MS);
    invoke<PluginPageDescriptorInfo>("plugin_page_descriptor", { id: pluginId })
      .then((info) => {
        if (cancelled) return;
        allowedRef.current = info.allowedCommands;
        setDescriptor(info);
      })
      .catch(() => {
        if (!cancelled) setLoadFailed(true);
      })
      .finally(() => {
        window.clearTimeout(timer);
      });
    return () => {
      cancelled = true;
      window.clearTimeout(timer);
    };
  }, [pluginId, reloadNonce, descriptor, ready]);

  useEffect(() => {
    const onMessage = (event: MessageEvent) => {
      // Read the ref at event time: this listener registers before the
      // descriptor resolves, so any value captured now would be stale for the
      // whole life of the page.
      const frame = iframeRef.current;
      if (!frame || event.source !== frame.contentWindow) return;
      const data: unknown = event.data;

      if (isBridgeFrameReady(data)) {
        // The handshake. A page only gets to speak the protocol once it has
        // said which version it speaks; a missing or unknown version is
        // refused here, and the error state below names the number the host
        // supports. Ignore a second handshake from an already-decided
        // document (a page that re-announces cannot re-open a refused bridge).
        window.clearTimeout(handshakeTimerRef.current);
        setHandshake((current) => current ?? pluginPageHandshake(data, PLUGIN_PAGE_PROTOCOL));
        return;
      }

      // Everything below is interactive traffic. A document that has not been
      // accepted on the protocol handshake is not part of the conversation:
      // its invoke, notify and drag messages are dropped rather than answered,
      // so a stale page cannot half-use a bridge whose version it does not
      // share.
      if (handshakeRef.current?.status !== "accepted") return;

      if (isBridgeRequest(data)) {
        if (!commandAllowed(allowedRef.current, data.command)) {
          frame.contentWindow?.postMessage(
            {
              [BRIDGE_TAG]: "result",
              id: data.id,
              session: data.session,
              ok: false,
              error: `Command not allowed for this plugin page: ${data.command}`,
            },
            "*",
          );
          return;
        }
        invoke(data.command, (data.args ?? {}) as Record<string, unknown>)
          .then((value) => {
            frame.contentWindow?.postMessage(
              { [BRIDGE_TAG]: "result", id: data.id, session: data.session, ok: true, value },
              "*",
            );
          })
          .catch((error) => {
            frame.contentWindow?.postMessage(
              {
                [BRIDGE_TAG]: "result",
                id: data.id,
                session: data.session,
                ok: false,
                error: error instanceof Error ? error.message : String(error),
              },
              "*",
            );
          });
        return;
      }

      if (isBridgeClose(data)) {
        if (activeRef.current) onClose();
        return;
      }

      if (isBridgeDrag(data)) {
        // The page asked for a native window drag (a press on its blank chrome).
        // The routing predicate is pure and unit-tested: it gates on the page
        // being the live surface, so a kept-alive page hidden behind another
        // one cannot move the window on a stale press.
        if (shouldStartWindowDrag(data, Boolean(activeRef.current))) windowDragRef.current();
        return;
      }

      if (isBridgeNotify(data)) {
        // Host-owned feedback: the page names a dictionary key, the host shows
        // it on the app's one toast stack. An unknown key is dropped rather
        // than painted — the words belong to the host. A retryable toast gets
        // an action that asks the page to run its own retry; when the page is
        // gone by the time it is pressed there is nobody to ask, so nothing
        // happens rather than a second toast about the first one.
        if (!isMessageKey(data.messageKey)) return;
        const frame = iframeRef.current?.contentWindow;
        const translate = tRef.current;
        // The page mints the id; the host only echoes it back. Whatever action
        // the page registered under it is the one that re-runs, so two or
        // three coexisting toasts each keep their own retry instead of the
        // oldest one firing the newest failure.
        const notifyId = data.id;
        notifyRef.current(
          data.kind,
          translate(data.messageKey),
          data.retryable === true && frame
            ? {
                label: translate("plugin.retry"),
                run: () => {
                  const retry: BridgeNotifyRetry = { [BRIDGE_TAG]: "notify-retry", id: notifyId };
                  frame.postMessage(retry, "*");
                },
              }
            : undefined,
        );
        return;
      }

      if (isBridgeResult(data)) {
        // Not produced by pages; kept out of the host switch above. No-op.
      }
    };
    window.addEventListener("message", onMessage);
    return () => window.removeEventListener("message", onMessage);
  }, [onClose]);

  const handleFrameLoad = useCallback(() => {
    // Do NOT clear the verdict here. A page's own script runs before the
    // iframe's `load` event, so its `frame-ready` is often *already processed*
    // by the time this fires — resetting here would wipe an accepted handshake
    // and then time out to a false `missing`. The fresh-document reset belongs
    // to the `[src]` effect above, which runs before the new document even
    // starts loading. This handler only arms (or re-arms) the deadline; the
    // timer callback can never overwrite a verdict that already exists.
    window.clearTimeout(handshakeTimerRef.current);
    handshakeTimerRef.current = window.setTimeout(() => {
      setHandshake((current) => current ?? { status: "missing" });
    }, HANDSHAKE_TIMEOUT_MS);
  }, []);

  // The bootstrap messages below all wait for the accepted handshake: pushing
  // settings into a page that has not agreed on the protocol would be exactly
  // the silent mismatch this round removes. This effect is the handshake's
  // success path — the page gets its initial settings the moment it is accepted.
  useEffect(() => {
    if (!ready) return;
    window.clearTimeout(handshakeTimerRef.current);
    if (iframeRef.current?.contentWindow) {
      const opacityMessage: BridgeOpacity = {
        [BRIDGE_TAG]: "opacity",
        mainOpacity: opacityRef.current.mainOpacity,
        terminalOpacity: opacityRef.current.terminalOpacity,
      };
      const themeMessage: BridgeTheme = {
        [BRIDGE_TAG]: "theme",
        theme,
      };
      iframeRef.current.contentWindow.postMessage(opacityMessage, "*");
      iframeRef.current.contentWindow.postMessage(themeMessage, "*");
      const glassMessage: BridgeGlass = {
        [BRIDGE_TAG]: "glass",
        glassStep,
      };
      iframeRef.current.contentWindow.postMessage(glassMessage, "*");
    }
    // Hand the keyboard to the page: its own Spotlight discipline (typing
    // routes to its filter input) takes over from here.
    if (activeRef.current) iframeRef.current?.focus();
  }, [ready, theme]);

  useEffect(() => {
    if (!ready) return;
    const glassMessage: BridgeGlass = {
      [BRIDGE_TAG]: "glass",
      glassStep,
    };
    iframeRef.current?.contentWindow?.postMessage(glassMessage, "*");
  }, [ready, glassStep]);

  // Bootstrap params double as a cache-buster-free way to pass settings the
  // sandboxed page cannot read itself. Re-keying the iframe when they change
  // is acceptable — plugin pages are transient overlays, and theme/language
  // changes mid-session are rare enough that a remount beats staleness.
  //
  // Opacity is deliberately absent from the deps: it is read from the ref for
  // the initial bootstrap, then pushed as a message (below) so dragging a
  // slider restyles the live page instead of remounting it mid-interaction.
  // The glass step is the same: it bootstraps here and then travels as a
  // message, so changing the material never remounts the page either.
  const src = useMemo(() => {
    if (!descriptor) return null;
    try {
      return buildPluginPageUrl(document.baseURI, descriptor.page, {
        lang: language,
        theme,
        "main-opacity": opacityRef.current.mainOpacity,
        "terminal-opacity": opacityRef.current.terminalOpacity,
        "glass-step": glassStep,
      });
    } catch {
      // A page that resolves off-origin is a registry bug, not something to
      // load: fall through to the error state's retry.
      return null;
    }
  }, [descriptor, language, theme, glassStep]);

  // A fresh document (new descriptor, language or theme) has no listener yet
  // and its predecessor's verdict does not transfer: reset during the *render*
  // that produces the new src, not in an effect. `useEffect` runs after paint
  // and a cached same-origin page can have run its own script — and sent its
  // `frame-ready` — before that, which would then be wiped by the reset; two
  // rounds of this bug look exactly like “the page times out”. Deriving the
  // reset in render means it lands in the same commit that changes the iframe's
  // `src`, i.e. strictly before the new document exists.
  if (handshakeSrc !== src) {
    setHandshakeSrc(src);
    setHandshake(null);
  }

  useEffect(() => {
    if (pluginId) return;
    // The page was closed while the iframe stays mounted (its home is now the
    // persistent plugin layer above the shells). "display:none" on the layer
    // hides the iframe element but does not blur the document inside it — a
    // hidden iframe document can still hold the keyboard and swallow the first
    // keystroke meant for the surface underneath, and it can re-claim focus
    // later still (a kept-alive page schedules its own `focus()`s on reveal).
    // Relinquish explicitly: blur the frame, then clear whatever the inner
    // document had focused (the same-origin built-in page; a cross-origin page
    // exposes no contentDocument and is left to the host's focus collector in
    // App.tsx, which reclaims on the resulting focusout).
    const frame = iframeRef.current;
    if (!frame) return;
    frame.blur();
    try {
      frame.contentWindow?.blur();
    } catch {
      // Some engines reject blur() on a hidden/removed window; the host
      // collector below is the guarantee, this is only belt-and-braces.
    }
    try {
      (frame.contentDocument?.activeElement as HTMLElement | null)?.blur();
    } catch {
      // Cross-origin (opaque) sandbox: no document access by design.
    }
  }, [pluginId]);

  useEffect(() => {
    if (!ready) return;
    const frame = iframeRef.current?.contentWindow;
    const message: BridgeVisibility = { [BRIDGE_TAG]: "visibility", visible: Boolean(pluginId) };
    frame?.postMessage(message, "*");
    if (pluginId) iframeRef.current?.focus();
    return () => frame?.postMessage({ [BRIDGE_TAG]: "visibility", visible: false }, "*");
  }, [ready, pluginId]);

  useEffect(() => {
    if (!ready) return;
    const opacityMessage: BridgeOpacity = {
      [BRIDGE_TAG]: "opacity",
      mainOpacity,
      terminalOpacity,
    };
    iframeRef.current?.contentWindow?.postMessage(opacityMessage, "*");
  }, [ready, mainOpacity, terminalOpacity]);

  useEffect(() => {
    if (!ready) return;
    const themeMessage: BridgeTheme = {
      [BRIDGE_TAG]: "theme",
      theme,
    };
    iframeRef.current?.contentWindow?.postMessage(themeMessage, "*");
  }, [ready, theme]);

  // GLASS-CLIP: the content recess the page's own sheet composes is injected
  // through the same two channels the step tokens use (this container style,
  // and the page's bootstrap + bridge messages). The container injection is the
  // host-side record of the hand-off; the page reads the value from its own
  // root because a sandboxed cross-document frame cannot inherit it.
  const glassTokens = { ...glassStepStyle(glassStep), ...glassContentStyle(terminalOpacity) };

  return (
    <div
      className="plugin-page-host"
      data-plugin-id={pluginId ?? ""}
      data-glass-step={glassStep}
      style={{ display: pluginId ? "flex" : "none", ...glassTokens }}
    >
      {pluginId ? (
        // Host-owned chrome, rendered *inside* this host so it is part of the
        // plugin surface and therefore inside `pluginLayer` — never a sibling
        // of it (the iframe's keep-alive depends on that stable position). It
        // is not focusable itself; the only control is the close button, which
        // keeps the existing Esc / Cmd+W meaning and returns to the remembered
        // surface through `onClose`. It is also the window's drag handle in
        // this mode; see `onDragStart`.
        <header className="plugin-page-host__topbar" onMouseDown={onDragStart}>
          <span className="plugin-page-host__topbar-title">
            {descriptor ? t(descriptor.titleKey as MessageKey) : ""}
          </span>
          <button
            type="button"
            className="toolbar-button toolbar-button--close"
            aria-label={t("plugin.close")}
            title={t("plugin.closeHint")}
            onClick={onClose}
          >
            ×
          </button>
        </header>
      ) : null}
      <div className="plugin-page-host__body">
        {src ? (
          <iframe
            ref={iframeRef}
            className="plugin-page-host__frame"
            src={src}
            title={descriptor ? t(descriptor.titleKey as MessageKey) : pluginId ?? ""}
            // WebKit needs same-origin for a built-in page to load its bundled
            // stylesheet and module entry; external plugin pages retain the
            // opaque-origin sandbox. The set is the one in `plugin-pages.ts`
            // (`pluginPageNeedsSameOrigin`), not a clipboard-only special case:
            // the browser page is a built-in too, and an opaque-origin frame on
            // WebKit never ran its module — so it never sent `frame-ready`.
            sandbox={pluginPageNeedsSameOrigin(descriptor?.id) ? "allow-scripts allow-same-origin" : "allow-scripts"}
            onLoad={handleFrameLoad}
            // A refused handshake hides the frame but keeps it mounted: the
            // error state below is what the user must see, and unmounting on a
            // verdict would churn the frame for no gain (the retry button
            // replaces the descriptor and thus the `src` anyway).
            style={refused ? { display: "none" } : undefined}
          />
        ) : null}
        {refused && src ? (
          // The page loaded but refused the protocol handshake: it announced a
          // version this build does not speak, or announced none at all. Show
          // which version — the page's and the host's — instead of a white
          // iframe. `handshakeErrorDetail` picks the variant; the dictionary
          // owns the words.
          <div className="plugin-page-host__error" role="alert">
            <span className="plugin-page-host__error-title">
              {t("plugin.pageError")}
            </span>
            <span className="plugin-page-host__error-detail">
              {(() => {
                const detail = handshakeErrorDetail(handshake!, PLUGIN_PAGE_PROTOCOL);
                return t(detail.key, detail.params);
              })()}
            </span>
            <button
              type="button"
              className="plugin-page-host__button"
              onClick={() => { setDescriptor(null); setReloadNonce((nonce) => nonce + 1); }}
            >
              {t("settings.retry")}
            </button>
          </div>
        ) : null}
        {!src && (loadFailed || descriptor) ? (
          // `descriptor && !src` means the page URL was rejected as off-origin.
          <div className="plugin-page-host__error" role="alert">
            <span className="plugin-page-host__error-title">
              {t("plugin.pageError")}
            </span>
            <button
              type="button"
              className="plugin-page-host__button"
              onClick={() => { setDescriptor(null); setReloadNonce((nonce) => nonce + 1); }}
            >
              {t("settings.retry")}
            </button>
          </div>
        ) : null}
        {!src && !loadFailed && !descriptor && pluginId ? (
          // Descriptor still in flight. Render an opaque placeholder rather than
          // nothing: this host fills a transparent window, so an empty subtree
          // shows the desktop through the panel for as long as the fetch takes.
          <div className="plugin-page-host__loading" aria-busy="true" />
        ) : null}
      </div>
    </div>
  );
}
