// R76 · the clipboard page's incremental row reconciler is gone with the page.
//
// This suite used to pin the retired page's in-place list reconciliation: rows
// keyed by `data-row-id`, patched instead of rebuilt, thumbnails gated on
// scrolling into view. That page (`src/plugins/clipboard/main.ts`) was an
// iframe document R33 retired; R76 deleted its source, so there is nothing left
// to reconcile. The live clipboard surface is the launcher mode
// (`src/plugins/clipboard/mode.ts` + the launcher's own row renderer), which
// has its own suites.
//
// The negative guard is the point: a revived page source turns this red.
import test from "node:test";

import { assertRetiredPageLayerIsGone } from "./retired-page-layer.ts";

const root = new URL("../", import.meta.url);

test("the retired clipboard page source stays deleted", async () => {
  await assertRetiredPageLayerIsGone(root);
});
