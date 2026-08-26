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
import type { Language } from "../i18n";

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

type PendingCall = {
  resolve: (value: unknown) => void;
  reject: (error: string) => void;
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
  /** Correlation ids → resolvers for in-flight bridge invocations. */
  const pendingCalls = useRef(new Map<number, PendingCall>());
  // Ref mirrors so the once-registered message listener always sees current
  // values without resubscribing (and dropping in-flight calls) on re-render.
  const allowedRef = useRef<readonly string[]>([]);
  const frameRef = useRef<HTMLIFrameElement | null>(null);

  useEffect(() => {
    let cancelled = false;
    setDescriptor(null);
    setLoadFailed(false);
    invoke<PluginPageDescriptorInfo>("plugin_page_descriptor", { id: pluginId })
      .then((info) => {
        if (cancelled) return;
        allowedRef.current = info.allowedCommands;
        setDescriptor(info);
      })
      .catch(() => {
        if (!cancelled) setLoadFailed(true);
      });
    return () => {
      cancelled = true;
      allowedRef.current = [];
      for (const call of pendingCalls.current.values()) {
        call.reject("Plugin page closed");
      }
      pendingCalls.current.clear();
    };
  }, [pluginId]);

  useEffect(() => {
    frameRef.current = iframeRef.current;
    const onMessage = (event: MessageEvent) => {
      const frame = frameRef.current;
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
    // Hand the keyboard to the page: its own Spotlight discipline (typing
    // routes to its filter input) takes over from here.
    iframeRef.current?.focus();
  }, []);

  // Bootstrap params double as a cache-buster-free way to pass settings the
  // sandboxed page cannot read itself. Re-keying the iframe when they change
  // is acceptable — plugin pages are transient overlays, and theme/language
  // changes mid-session are rare enough that a remount beats staleness.
  const src = useMemo(() => {
    if (!descriptor) return null;
    return buildPluginPageUrl(document.baseURI, descriptor.page, {
      lang: language,
      theme,
      "main-opacity": mainOpacity,
      "terminal-opacity": terminalOpacity,
    });
  }, [descriptor, language, theme, mainOpacity, terminalOpacity]);

  return (
    <div className="plugin-page-host" data-plugin-id={pluginId}>
      {src ? (
        <iframe
          ref={iframeRef}
          className="plugin-page-host__frame"
          src={src}
          title={descriptor?.titleKey ?? pluginId}
          // Opaque origin: scripts yes, same-origin access and Tauri APIs no.
          sandbox="allow-scripts"
          onLoad={handleFrameLoad}
        />
      ) : loadFailed ? (
        <div className="plugin-page-host__error" role="alert">
          {pluginId}
        </div>
      ) : null}
    </div>
  );
}
