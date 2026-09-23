// R29 · the generic plugin configuration overlay.
//
// The user's second requirement: 「配置页面通用化：点击设置之后，弹出一个基于通用
// 规则的配置页面，而不是一个新的完全独立的页面。」 A plugin's settings are no
// longer a document of its own. They are a `PluginConfigSchema` — an ordered
// list of fields — rendered here by the generic controls, inside the launcher
// window, over whatever surface was showing.
//
// The overlay owns the three things a schema cannot: reading the plugin's
// current block from the backend, writing a change back through the plugin's
// narrow `*_get_settings`/`*_set_settings` pair (never the whole settings
// object), and resolving the one dynamic field — the browser target dropdown's
// discovered browsers. Everything else is the schema's.

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { invoke } from "@tauri-apps/api/core";
import type { Translate } from "../i18n";
import { browserTargets, normalizeBrowserSettings, normalizeProfiles } from "../browser-page";
import { normalizeClipboardSettings } from "../clipboard-history";
import { BROWSER_PLUGIN_ID, CLIPBOARD_PLUGIN_ID } from "../plugin-pages";
import {
  applyConfigChange,
  configDefaults,
  configValues,
  pluginConfigSchema,
  type PluginConfigContext,
  type PluginConfigValue,
} from "./config-schema";
import { PluginConfigRow } from "./controls";

export type PluginConfigOverlayProps = {
  pluginId: string;
  t: Translate;
  /** The launcher's own settings, for the clipboard switch (the long-standing
   *  `clipboard_history_enabled` field) before the plugin block answers. */
  clipboardEnabled: boolean;
  /** Write a field that lives in the general settings object. Only the
   *  clipboard switch does; the browser's `enabled` rides its own block. */
  onChangeGeneralSetting: (key: "clipboard_history_enabled", value: boolean) => void;
  /** R32 · the browser block the overlay just wrote, so the launcher's own
   *  settings snapshot can follow a change (the search-field radio takes effect
   *  on the next search, not on the next restart). Only the browser plugin
   *  calls it. */
  onBrowserSettingsChange: (settings: ReturnType<typeof normalizeBrowserSettings>) => void;
  onClose: () => void;
};

/** The schema-shaped values for one plugin, read from the backend. */
const loadPluginValues = async (
  pluginId: string,
  clipboardEnabled: boolean,
): Promise<Record<string, PluginConfigValue>> => {
  if (pluginId === CLIPBOARD_PLUGIN_ID) {
    const block = await invoke<unknown>("clipboard_get_settings").catch(() => null);
    const schema = pluginConfigSchema(CLIPBOARD_PLUGIN_ID)!;
    return configValues(schema, {
      enabled: clipboardEnabled,
      max_items: normalizeClipboardSettings(block).max_items,
    });
  }
  const block = await invoke<unknown>("browser_get_settings").catch(() => null);
  const schema = pluginConfigSchema(BROWSER_PLUGIN_ID)!;
  return configValues(schema, normalizeBrowserSettings(block) as unknown as Record<string, unknown>);
};

export function PluginConfigOverlay({
  pluginId,
  t,
  clipboardEnabled,
  onChangeGeneralSetting,
  onBrowserSettingsChange,
  onClose,
}: PluginConfigOverlayProps) {
  const [context, setContext] = useState<PluginConfigContext>({});
  const schema = useMemo(() => pluginConfigSchema(pluginId, context), [pluginId, context]);

  const [values, setValues] = useState<Record<string, PluginConfigValue>>(() =>
    schema ? configDefaults(schema) : {},
  );
  // The values as the backend last confirmed them; a change that fails to write
  // rolls the control back to this snapshot.
  const committed = useRef<Record<string, PluginConfigValue>>(values);
  // The latest painted values. A slider drag fires many changes before any
  // write resolves; each must build on the previous *painted* value, not on the
  // last committed one, or the intermediate steps would be dropped.
  const valuesRef = useRef(values);
  valuesRef.current = values;

  // R29 · the one dynamic field: the target dropdown's discovered browsers.
  // Fetched only for the browser plugin, and only once per open.
  useEffect(() => {
    if (pluginId !== BROWSER_PLUGIN_ID) return;
    let cancelled = false;
    invoke<unknown>("browser_discover")
      .then((rows) => {
        if (!cancelled) setContext({ browserTargets: browserTargets(normalizeProfiles(rows)) });
      })
      .catch(() => {
        if (!cancelled) setContext({});
      });
    return () => {
      cancelled = true;
    };
  }, [pluginId]);

  // The plugin's stored block, once per open (and once more if the clipboard
  // switch changed underneath us, so the two agree).
  useEffect(() => {
    if (!schema) return;
    let cancelled = false;
    loadPluginValues(pluginId, clipboardEnabled)
      .then((loaded) => {
        if (cancelled) return;
        // The discovered-target fetch may have landed first; keep the schema
        // it produced by loading values through the same schema the render uses.
        setValues(configValues(schema, loaded));
        committed.current = configValues(schema, loaded);
      })
      .catch(() => undefined);
    return () => {
      cancelled = true;
    };
    // `schema` identity changes when the target list arrives; reloading then is
    // a cheap no-op read and keeps the two in step.
  }, [pluginId, schema, clipboardEnabled]);

  const persist = useCallback(
    async (next: Record<string, PluginConfigValue>) => {
      if (pluginId === CLIPBOARD_PLUGIN_ID) {
        onChangeGeneralSetting("clipboard_history_enabled", next.enabled === true);
        await invoke("clipboard_set_settings", {
          settings: { max_items: Number(next.max_items) },
        }).catch(() => undefined);
        return;
      }
      await invoke("browser_set_settings", {
        settings: {
          enabled: next.enabled === true,
          target: String(next.target ?? "auto"),
          custom_base_dir: typeof next.custom_base_dir === "string" ? next.custom_base_dir : null,
          history_days: Number(next.history_days),
          cdp_enabled: next.cdp_enabled === true,
          cdp_port: Number(next.cdp_port),
          sort_order: String(next.sort_order ?? "relevance"),
          search_fields: String(next.search_fields ?? "all"),
        },
      })
        .then((stored) => {
          // The backend has the last word on normalization; hand the launcher
          // the block it actually stored so its next search uses it.
          onBrowserSettingsChange(normalizeBrowserSettings(stored));
        })
        .catch(() => undefined);
    },
    [pluginId, onChangeGeneralSetting, onBrowserSettingsChange],
  );

  const handleChange = useCallback(
    (key: string, raw: unknown) => {
      if (!schema) return;
      const next = applyConfigChange(schema, valuesRef.current, key, raw);
      // The control paints immediately; the write is fire-and-forget and the
      // committed snapshot advances only once the backend has accepted it.
      valuesRef.current = next;
      setValues(next);
      void persist(next).then(() => {
        committed.current = next;
      });
    },
    [schema, persist],
  );

  if (!schema) return null;

  return (
    <div
      className="plugin-config"
      role="dialog"
      aria-modal="false"
      aria-label={t(schema.titleKey)}
    >
      <div className="plugin-config__header">
        <span className="plugin-config__title">{t(schema.titleKey)}</span>
        <button
          type="button"
          className="plugin-config__close"
          aria-label={t("plugins.config.close")}
          title={t("plugins.config.close")}
          onClick={onClose}
        >
          <svg
            viewBox="0 0 24 24"
            width="14"
            height="14"
            fill="none"
            stroke="currentColor"
            strokeWidth="2"
            strokeLinecap="round"
            aria-hidden="true"
          >
            <path d="M6 6l12 12M18 6L6 18" />
          </svg>
        </button>
      </div>
      <div className="plugin-config__fields">
        {schema.fields.map((field) => (
          <PluginConfigRow
            key={field.key}
            t={t}
            field={field}
            value={values[field.key] ?? null}
            onChange={handleChange}
          />
        ))}
      </div>
    </div>
  );
}
