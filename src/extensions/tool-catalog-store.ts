// R68 · the one reader of `extensions_tool_catalog` for the UI.
//
// Both surfaces that need the install catalog — the launcher's install rows and
// the extensions panel's install button — read the same report, and the report
// is expensive enough (a `stat` scan of the host search path on the Rust side)
// that neither should own its own copy. So the fetch lives here, once:
//
//   * a **module-level memo** holds the last report for the process, so
//     reopening the panel or typing another query costs no IPC;
//   * the **reveal listener in `App.tsx`** asks for one refresh per window
//     reveal (`refreshToolCatalog`), which is the only moment the answer can
//     have changed (a tool installed in the user's own shell while the window
//     was hidden). There is no polling and no timer;
//   * a failed read is a **soft landing**: the report becomes `null` and every
//     consumer renders nothing rather than an error. Install discovery is a
//     bonus, not a feature the user asked for, so a failed scan must never
//     interrupt an ordinary search.
//
// The hook subscribes to the memo so a refresh that lands while the launcher or
// the panel is mounted repaints both.

import { invoke } from "@tauri-apps/api/core";
import { useEffect, useState } from "react";
import type { ToolCatalogReport } from "./tool-install";

type Listener = () => void;

let report: ToolCatalogReport | null = null;
let loaded = false;
let inFlight: Promise<ToolCatalogReport | null> | null = null;
const listeners = new Set<Listener>();

const emit = () => {
  for (const listener of listeners) listener();
};

const fetchReport = async (): Promise<ToolCatalogReport | null> => {
  try {
    report = await invoke<ToolCatalogReport>("extensions_tool_catalog");
  } catch {
    report = null;
  }
  loaded = true;
  emit();
  return report;
};

/** The catalog, fetching it once per process on the first call. */
export const loadToolCatalog = (): Promise<ToolCatalogReport | null> => {
  if (loaded) return Promise.resolve(report);
  if (!inFlight) {
    inFlight = fetchReport().finally(() => {
      inFlight = null;
    });
  }
  return inFlight;
};

/** Re-read the catalog. Called from the window-reveal listener, once per
 *  reveal; a refresh already in flight is shared rather than duplicated. */
export const refreshToolCatalog = (): Promise<ToolCatalogReport | null> => {
  if (!inFlight) {
    inFlight = fetchReport().finally(() => {
      inFlight = null;
    });
  }
  return inFlight;
};

/** The last report, synchronously — `null` before the first answer and after a
 *  failed read. Exposed for tests and for the store's own hook. */
export const toolCatalogSnapshot = (): ToolCatalogReport | null => report;

/**
 * Subscribe a component to the catalog. The initial value is whatever the memo
 * already holds, so a panel opened after the launcher does not flash empty.
 */
export const useToolCatalog = (): ToolCatalogReport | null => {
  const [snapshot, setSnapshot] = useState<ToolCatalogReport | null>(report);
  useEffect(() => {
    const listener = () => setSnapshot(report);
    listeners.add(listener);
    // Catch a report that landed between the render and this effect.
    listener();
    void loadToolCatalog();
    return () => {
      listeners.delete(listener);
    };
  }, []);
  return snapshot;
};
