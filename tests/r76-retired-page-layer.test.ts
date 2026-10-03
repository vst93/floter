// R76 · the retired iframe plugin-page layer stays retired.
//
// R33 retired the built-in pages onto the launcher's configuration overlay, but
// the layer's source, its two plugin stylesheets and its ~600 host-side CSS
// rules stayed in the tree — reachable by nothing, still type-checked, and
// still shipped in `dist/assets/index-*.css`. R76 deleted them. This is the
// freeze lock, in the shape the R-FREEZE-2 round established: scan the real
// sources and turn red if any removed name comes back.
//
// The guard (`tests/retired-page-layer.ts`) proves the *files* are gone; the
// scans below prove no *live source* reintroduces a symbol from them.
//
// R79 · the fourteen suites that used to each call the guard now leave the
// freeze lock to this one test — their narratives moved here with the call.
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

import { RETIRED_PAGE_PATHS, assertRetiredPageLayerIsGone } from "./retired-page-layer.ts";

const root = new URL("../", import.meta.url);
const read = (path: string) => readFile(new URL(path, root), "utf8");

/** Strip CSS/JS block comments and JS line comments so a comment that merely
 *  names a retired symbol cannot satisfy — or trip — the scan. */
const stripComments = (source: string) =>
  source
    .replace(/\/\*[\s\S]*?\*\//g, "")
    .replace(/^\s*\/\/.*$/gm, "");

/** Live source trees the retired layer must never reappear in. */
const LIVE_SOURCES = [
  "src/App.tsx",
  "src/main.tsx",
  "src/plugin-pages.ts",
  "src/browser-page.ts",
  "src/launcher.ts",
  "src/glass-material.ts",
  "src/clipboard-history.ts",
  "src/hooks/useLauncherCatalog.ts",
  "src/hooks/useLauncherActions.ts",
  "src/plugins/clipboard/mode.ts",
  "src/plugins/browser/mode.ts",
  "src/plugins/config-schema.ts",
  "src/launcher/LauncherResults.tsx",
  "src/styles/terminal.css",
  "src/styles/base.css",
  "src/styles/launcher.css",
  "src/styles/settings.css",
  "src/styles/extensions.css",
];

const FORBIDDEN: readonly [string, string][] = [
  ["PluginPageHost", "the retired iframe host component"],
  ["clipboard-panel", "the retired clipboard page's host-side class"],
  ["plugin-page-host", "the retired plugin-page host chrome class"],
  ["clipboard-row", "the retired clipboard page's row class"],
  ["clipboard_read_image", "the retired clipboard page's image read command"],
  ["clipboard_entry_statuses", "the retired clipboard page's status command"],
  ["clipboard_read_file_preview", "the retired clipboard page's preview command"],
  ["clipboard-list", "the retired clipboard page's pure-logic module"],
  ["plugins/clipboard/main", "the retired clipboard page entry"],
  ["plugins/browser/main", "the retired browser page entry"],
  ["plugins/clipboard/page.css", "the retired clipboard page stylesheet"],
  ["plugins/browser/page.css", "the retired browser page stylesheet"],
];

test("the retired page layer's files are physically gone", async () => {
  // R79 · the per-suite guards converged here. The narratives that used to sit
  // beside the call in each suite, verbatim:
  //
  // · the clipboard page's incremental row reconciler (rows keyed by
  //   `data-row-id`, patched instead of rebuilt, thumbnails gated on scrolling
  //   into view) lived in `src/plugins/clipboard/main.ts` — an iframe document
  //   R33 retired; R76 deleted its source, so there is nothing left to
  //   reconcile.
  // · the page's behaviour suites (type chips, the keyboard resolver, the
  //   row-action trio, the filter tabs, pin persistence) lived in
  //   `src/clipboard-list.ts`; R76 deleted the page's source and its
  //   pure-logic module, and R77 deleted the inlined icon module that drift
  //   check read.
  // · CLIP-DISSOLVE removed the page's 56px status-bar band and CLIP-DRAG took
  //   the last band-shaped thing on the header; both lived in the retired page
  //   (`page.css` + `main.ts`), so the launcher's own clipboard mode has no
  //   page-local band to dissolve.
  // · the page's prompt-label wrap fix and its single 置顶 scope toggle lived
  //   in `src/plugins/clipboard/page.css` and `main.ts`.
  // · the page's own stylesheet consumed the host-injected glass bands; what is
  //   gone is the document that consumed the band.
  // · the retired page-side drag guard and the page's stylesheet are gone.
  // · `PluginPageHost.tsx`, the topbar's rules in `terminal.css`, and the
  //   plugin page stylesheets are deleted — there is no host chrome left.
  // · the built-in clipboard page was the protocol's first consumer, sending
  //   the same handshake from the shared constant; the example page is now the
  //   protocol's only worked consumer.
  // · the retired page's card and its two page stylesheets must stay deleted.
  //
  // The negative guard is the point: a revived page source turns this red.
  await assertRetiredPageLayerIsGone(root);
  // The guard is not vacuous: at least one of the paths resolves today, so a
  // typo in the list would not silently pass.
  assert.ok(RETIRED_PAGE_PATHS.length >= 6);
});

test("no live source reintroduces a retired page-layer symbol", async () => {
  for (const path of LIVE_SOURCES) {
    const source = stripComments(await read(path));
    for (const [token, what] of FORBIDDEN) {
      assert.ok(
        !source.includes(token),
        `${path} must not reference \`${token}\` — ${what} was deleted in R76`,
      );
    }
  }
});

test("the Rust registry and command table dropped the dead page commands", async () => {
  const pluginPages = stripComments(await read("src-tauri/src/plugin_pages.rs"));
  const lib = stripComments(await read("src-tauri/src/lib.rs"));
  const clipboard = stripComments(await read("src-tauri/src/clipboard_history/mod.rs"));
  for (const [token, what] of [
    ["clipboard_read_image", "the retired image-read command"],
    ["clipboard_entry_statuses", "the retired status command"],
    ["clipboard_read_file_preview", "the retired preview command"],
  ] as const) {
    assert.ok(!lib.includes(token), `lib.rs must not register ${what}`);
    assert.ok(!clipboard.includes(token), `clipboard_history must not implement ${what}`);
    assert.ok(!pluginPages.includes(token), `plugin_pages.rs must not allowlist ${what}`);
  }
  // The live commands are still there: this is a shrink, not an amputation.
  for (const command of [
    "clipboard_get_entries",
    "clipboard_set_favorite",
    "clipboard_delete",
    "clipboard_copy_entry",
    "clipboard_clear_history",
    "clipboard_thumbnail",
    "clipboard_get_settings",
    "clipboard_set_settings",
  ]) {
    assert.ok(lib.includes(command), `${command} must stay registered`);
    assert.ok(pluginPages.includes(command), `${command} must stay allowlisted`);
  }
});

test("no built-in plugin page is registered", async () => {
  const rust = stripComments(await read("src-tauri/src/plugin_pages.rs"));
  const pages = [...rust.matchAll(/page: "([^"]*)"/g)].map((match) => match[1]);
  assert.ok(pages.length >= 3, "every built-in descriptor must still exist");
  for (const page of pages) {
    assert.equal(page, "", "no built-in page path may be registered");
  }
});
