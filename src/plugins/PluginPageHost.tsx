import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { invoke } from "@tauri-apps/api/core";
import {
  BRIDGE_TAG,
  buildPluginPageUrl,
  commandAllowed,
  isBridgeClose,
  isBridgeRequest,
  isBridgeResult,
} from "../plugin-pages";
import type { BridgeOpacity } from "../plugin-pages";
import { createTranslator, type Language, type MessageKey } from "../i18n";

/**
 * Host side of the generic plugin-page mechanism.
 *
 * Renders the active plugin's HTML page inside a sandboxed iframe occupying
 * the whole terminal-panel canvas (the window geometry itself is owned by the
 * backend's `show_plugin_page`), and bridges postMessage traffic to
 * `invoke()` behind the page's per-plugin command allowlist.
 *
 * Why an iframe, sandboxed without allow-same-origin: plugin HTML — external
 * integrations especially — is less trusted than our own app code. The opaque
 * origin means the page cannot touch the host DOM or any Tauri surface; its
 * only capability is what the allowlisted bridge permits. Our own built-in
 * clipboard page deliberately rides this same pipeline so the mechanism is
 * exercised for real.
 */

/** How long to wait for the backend's page descriptor before showing the retry
 * state. Mirrors the page-side bridge timeout in `clipboard/main.ts` so both
 * ends of the pipeline give up on a silent host after the same delay. */
const DESCRIPTOR_TIMEOUT_MS = 10_000;

export type PluginPageDescriptorInfo = {
  id: string;
  titleKey: string;
  page: string;
  allowedCommands: string[];
};

type PluginPageHostProps = {
  pluginId: string;
  /** Bootstrap values handed to the page via query params. */
  language: Language;
  theme: "dark" | "light";
  mainOpacity: number;
  terminalOpacity: number;
  onClose: () => void;
};

export function PluginPageHost({
  pluginId,
  language,
  theme,
  mainOpacity,
  terminalOpacity,
  onClose,
}: PluginPageHostProps) {
  const iframeRef = useRef<HTMLIFrameElement>(null);
  const [descriptor, setDescriptor] = useState<PluginPageDescriptorInfo | null>(null);
  const [loadFailed, setLoadFailed] = useState(false);
  /** Bumped by the error state's retry button to re-run the descriptor fetch. */
  const [reloadNonce, setReloadNonce] = useState(0);
  /** True once the current document has loaded and can receive messages. */
  const [frameLoaded, setFrameLoaded] = useState(false);
  // Ref mirror so the once-registered message listener always sees the current
  // allowlist without resubscribing on re-render.
  const allowedRef = useRef<readonly string[]>([]);
  // Read (not depended on) when building the iframe src, so slider moves
  // reach a live page as a message instead of as a remount.
  const opacityRef = useRef({ mainOpacity, terminalOpacity });
  opacityRef.current = { mainOpacity, terminalOpacity };
  // The descriptor's human name arrives as an i18n KEY (`titleKey`); translate
  // it here so the iframe's accessible name reads as a label, never as a raw
  // dictionary key.
  const t = useMemo(() => createTranslator(language), [language]);

  useEffect(() => {
    let cancelled = false;
    setDescriptor(null);
    setLoadFailed(false);
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
      allowedRef.current = [];
    };
  }, [pluginId, reloadNonce]);

  useEffect(() => {
    const onMessage = (event: MessageEvent) => {
      // Read the ref at event time: this listener registers before the
      // descriptor resolves, so any value captured now would be stale for the
      // whole life of the page.
      const frame = iframeRef.current;
      if (!frame || event.source !== frame.contentWindow) return;
      const data: unknown = event.data;

      if (isBridgeRequest(data)) {
        if (!commandAllowed(allowedRef.current, data.command)) {
          frame.contentWindow?.postMessage(
            {
              [BRIDGE_TAG]: "result",
              id: data.id,
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
              { [BRIDGE_TAG]: "result", id: data.id, ok: true, value },
              "*",
            );
          })
          .catch((error) => {
            frame.contentWindow?.postMessage(
              {
                [BRIDGE_TAG]: "result",
                id: data.id,
                ok: false,
                error: error instanceof Error ? error.message : String(error),
              },
              "*",
            );
          });
        return;
      }

      if (isBridgeClose(data)) {
        onClose();
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
    setFrameLoaded(true);
    // Hand the keyboard to the page: its own Spotlight discipline (typing
    // routes to its filter input) takes over from here.
    iframeRef.current?.focus();
  }, []);

  // Bootstrap params double as a cache-buster-free way to pass settings the
  // sandboxed page cannot read itself. Re-keying the iframe when they change
  // is acceptable — plugin pages are transient overlays, and theme/language
  // changes mid-session are rare enough that a remount beats staleness.
  //
  // Opacity is deliberately absent from the deps: it is read from the ref for
  // the initial bootstrap, then pushed as a message (below) so dragging a
  // slider restyles the live page instead of remounting it mid-interaction.
  const src = useMemo(() => {
    if (!descriptor) return null;
    try {
      return buildPluginPageUrl(document.baseURI, descriptor.page, {
        lang: language,
        theme,
        "main-opacity": opacityRef.current.mainOpacity,
        "terminal-opacity": opacityRef.current.terminalOpacity,
      });
    } catch {
      // A page that resolves off-origin is a registry bug, not something to
      // load: fall through to the error state's retry.
      return null;
    }
  }, [descriptor, language, theme]);

  // A fresh document (new descriptor, language or theme) has no listener yet.
  useEffect(() => {
    setFrameLoaded(false);
  }, [src]);

  useEffect(() => {
    if (!frameLoaded) return;
    const message: BridgeOpacity = {
      [BRIDGE_TAG]: "opacity",
      mainOpacity,
      terminalOpacity,
    };
    iframeRef.current?.contentWindow?.postMessage(message, "*");
  }, [frameLoaded, mainOpacity, terminalOpacity]);

  return (
    <div className="plugin-page-host" data-plugin-id={pluginId}>
      {src ? (
        <iframe
          ref={iframeRef}
          className="plugin-page-host__frame"
          src={src}
          title={descriptor ? t(descriptor.titleKey as MessageKey) : pluginId}
          // Opaque origin: scripts yes, same-origin access and Tauri APIs no.
          sandbox="allow-scripts allow-same-origin"
          onLoad={handleFrameLoad}
        />
      ) : loadFailed || descriptor ? (
        // `descriptor && !src` means the page URL was rejected as off-origin.
        <div className="plugin-page-host__error" role="alert">
          <span className="plugin-page-host__error-title">
            {t("plugin.pageError")}
          </span>
          <button
            type="button"
            className="update-banner__button"
            onClick={() => setReloadNonce((nonce) => nonce + 1)}
          >
            {t("settings.retry")}
          </button>
        </div>
      ) : (
        // Descriptor still in flight. Render an opaque placeholder rather than
        // nothing: this host fills a transparent window, so an empty subtree
        // shows the desktop through the panel for as long as the fetch takes.
        <div className="plugin-page-host__loading" aria-busy="true" />
      )}
    </div>
  );
}
