// R76/R77 · the retired clipboard page's behaviour suites are gone with the page.
//
// This file used to drive the clipboard page's pure decisions (type chips, the
// keyboard resolver, the row-action trio, the filter tabs, pin persistence)
// out of `src/clipboard-list.ts`, plus a Lucide-drift check on the page's
// inlined icon geometry. R33 retired the iframe page onto the launcher's
// generic configuration overlay; R76 deleted the page's source and its
// pure-logic module, and R77 deleted the inlined icon module that drift check
// read (its only importer was this suite). The live clipboard surface is the
// launcher mode, covered by `r38-clipboard-mode`, `plugin-mode` and
// `plugin-pagination`.
//
// What stays here is the freeze lock: the retired page layer's files must stay
// deleted.
import assert from "node:assert/strict";
import test from "node:test";

import { assertRetiredPageLayerIsGone } from "./retired-page-layer.ts";

const root = new URL("../", import.meta.url);

test("the retired clipboard page source stays deleted", async () => {
  await assertRetiredPageLayerIsGone(root);
});
