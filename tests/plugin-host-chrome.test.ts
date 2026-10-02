// R76 · the plugin page's host chrome is gone with the host.
//
// R7-4a/b gave a plugin page a 28px host-owned topbar (title + close) and
// consolidated the page's filter budget onto the shell's one blur. R33 retired
// the built-in iframe pages; R76 deleted `PluginPageHost.tsx`, the topbar's
// rules in `terminal.css`, and the plugin page stylesheets, so there is no host
// chrome left to pin.
//
// The one fact that outlives it — the terminal shell owns the surface's single
// blur — is asserted here against the live sheet, and the negative guard keeps
// the retired layer deleted.
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

import { assertRetiredPageLayerIsGone } from "./retired-page-layer.ts";

const root = new URL("../", import.meta.url);
const read = (path: string) => readFile(new URL(path, root), "utf8");
const stripComments = (css: string) => css.replace(/\/\*[\s\S]*?\*\//g, "");
const rules = (css: string) => {
  const out: { selector: string; body: string }[] = [];
  for (const match of css.matchAll(/([^{}]+)\{([^{}]*)\}/g)) {
    out.push({ selector: match[1].trim(), body: match[2] });
  }
  return out;
};

test("the retired plugin host and its page sheets stay deleted", async () => {
  await assertRetiredPageLayerIsGone(root);
});

test("the terminal shell owns the surface's single blur, with no literal", async () => {
  const host = stripComments(await read("src/styles/terminal.css"));
  const shell = rules(host).find(({ selector }) => selector === ".terminal-panel");
  assert.ok(shell, "terminal.css must define .terminal-panel");
  assert.match(
    shell!.body,
    /backdrop-filter:\s*blur\(var\(--glass-blur-terminal\)\)\s*saturate\(var\(--glass-saturate\)\)/,
  );
  assert.ok(!/blur\(\s*[\d.]+px/.test(shell!.body), "the shell must not use a literal blur");
});
