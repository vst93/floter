// R76 · the clipboard page's top-band dissolve is moot: the page is gone.
//
// CLIP-DISSOLVE removed the clipboard page's 56px status-bar band, CLIP-DRAG
// took the last band-shaped thing on the header, and this suite pinned the
// result. All of it lived in the retired iframe page (`page.css` + `main.ts`),
// which R33 retired and R76 deleted. The launcher's own clipboard mode has no
// page-local band to dissolve.
//
// The negative guard is the point: a revived page source turns this red.
import test from "node:test";

import { assertRetiredPageLayerIsGone } from "./retired-page-layer.ts";

const root = new URL("../", import.meta.url);

test("the retired clipboard page source stays deleted", async () => {
  await assertRetiredPageLayerIsGone(root);
});
