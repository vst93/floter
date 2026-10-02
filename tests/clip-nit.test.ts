// R76 · the clipboard page's two UI nits are gone with the page.
//
// This suite used to pin the retired page's prompt-label wrap fix and its
// single 置顶 scope toggle. Both lived in `src/plugins/clipboard/page.css` and
// `src/plugins/clipboard/main.ts` — an iframe document R33 retired and R76
// deleted. The live clipboard surface is the launcher mode, whose own suites
// (`r38-clipboard-mode`, `plugin-mode`, `plugin-fusion`) cover its chips and
// filter vocabulary.
//
// The negative guard is the point: a revived page source turns this red.
import test from "node:test";

import { assertRetiredPageLayerIsGone } from "./retired-page-layer.ts";

const root = new URL("../", import.meta.url);

test("the retired clipboard page source stays deleted", async () => {
  await assertRetiredPageLayerIsGone(root);
});
