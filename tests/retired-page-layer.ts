// R76 · the retired iframe plugin-page layer.
//
// R33 retired the built-in pages onto the launcher's generic configuration
// overlay, but the layer's source stayed in the tree — reachable by nothing,
// still type-checked, and its ~600 host-side CSS rules still shipped. R76
// deleted it physically.
//
// The suites that used to read those files now assert they are gone, the way
// the R-FREEZE-2 retirement round did: physical deletion plus a negative
// guard, so reviving a dead file (or a dead symbol in a live one) turns the
// guard red instead of silently re-entering the tree.
import assert from "node:assert/strict";
import { stat } from "node:fs/promises";

/** The dead source and stylesheets R76 deleted. Nothing may recreate them. */
export const RETIRED_PAGE_PATHS = [
  "src/plugins/clipboard/main.ts",
  "src/plugins/browser/main.ts",
  "src/plugins/PluginPageHost.tsx",
  "src/plugins/clipboard/page.css",
  "src/plugins/browser/page.css",
  "src/clipboard-list.ts",
] as const;

/**
 * Assert every retired path is gone. `root` is the suite's `new URL("../",
 * import.meta.url)` so each caller resolves against the repo root.
 */
export const assertRetiredPageLayerIsGone = async (root: URL): Promise<void> => {
  for (const path of RETIRED_PAGE_PATHS) {
    await assert.rejects(
      stat(new URL(path, root)),
      { code: "ENOENT" },
      `${path} is retired (R33/R76) and must stay deleted`,
    );
  }
};
