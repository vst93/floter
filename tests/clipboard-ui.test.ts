// R76 · the retired clipboard page's behaviour suites are gone with the page.
//
// This file used to drive the clipboard page's pure decisions (type chips, the
// keyboard resolver, the row-action trio, the filter tabs, pin persistence)
// out of `src/clipboard-list.ts`, plus a Lucide-drift check on the page's
// inlined icon geometry. R33 retired the iframe page onto the launcher's
// generic configuration overlay; R76 deleted the page's source and its
// pure-logic module. The live clipboard surface is the launcher mode, covered
// by `r38-clipboard-mode`, `plugin-mode` and `plugin-pagination`.
//
// What stays here is the one check whose *subject* was retained: the inlined
// icon geometry in `src/clipboard-icons.ts` (not on R76's delete list) still
// has to match the installed Lucide package.
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

import { assertRetiredPageLayerIsGone } from "./retired-page-layer.ts";

const root = new URL("../", import.meta.url);

test("the retired clipboard page source stays deleted", async () => {
  await assertRetiredPageLayerIsGone(root);
});

test("every inline clipboard icon still matches the installed Lucide package", async () => {
  // `src/clipboard-icons.ts` is a copy of Lucide's geometry (the page could not
  // render `lucide-react`). It is retained, so this asserts the copy has not
  // drifted: each node's `d`/`cx`/`cy`/`r`/`width`/`height` must equal the
  // package's own node for the same icon.
  const icons = await import("../src/clipboard-icons.ts");
  const lucideFor: Record<string, string> = {
    type: "type",
    link: "link",
    palette: "palette",
    image: "image",
    file: "file",
    folder: "folder",
    copy: "copy",
    pin: "pin",
    trash: "trash-2",
    search: "search",
    close: "x",
    check: "check",
  };
  for (const [name, lucideName] of Object.entries(lucideFor)) {
    const source = await readFile(
      new URL(`node_modules/lucide-react/dist/esm/icons/${lucideName}.mjs`, root),
      "utf8",
    );
    // Compare the set of `tag:key=value` tuples, order-independent: the copy
    // reorders nothing but drops Lucide's `key` metadata, which is React's.
    const tuples = (text: string) =>
      [...text.matchAll(/\[\s*"(\w+)",\s*\{([^}]*)\}\s*\]/g)]
        .flatMap(([, tag, body]) =>
          [...body.matchAll(/(\w+):\s*"([^"]*)"/g)]
            .filter(([, k]) => k !== "key")
            .map(([, k, v]) => `${tag}:${k}=${v}`),
        )
        .sort();
    const ours = (icons.CLIPBOARD_ICON_NODES as Record<string, unknown>)[name] as [
      string,
      Record<string, string>,
    ][];
    const ourTuples = ours
      .flatMap(([tag, attrs]) => Object.entries(attrs).map(([k, v]) => `${tag}:${k}=${v}`))
      .sort();
    assert.deepEqual(ourTuples, tuples(source), `${name} has drifted from Lucide's ${lucideName}`);
  }
});
