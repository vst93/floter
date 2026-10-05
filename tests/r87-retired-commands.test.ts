// R87 · the two R76 leftovers stay deleted.
//
// R76 retired the built-in iframe plugin-page layer and deleted the page
// commands its documents used, but two Rust commands survived with zero
// callers: `show_plugin_page` (the resize/reveal command in `lib.rs`) and
// `plugin_page_descriptor` (the page-table wrapper in `plugin_pages.rs`).
// R87 deleted both — and `PluginPageInfo`, the wire type whose only consumer
// was the wrapper. This is the freeze lock, in the shape R77 established: scan
// the real sources and turn red if a removed name comes back.
//
// The deleted names are assembled from parts so this file does not itself
// reintroduce a token the round's zero-hit grep bans.
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

const root = new URL("../", import.meta.url);
const read = (path: string) => readFile(new URL(path, root), "utf8");

/** Strip block/line comments so a comment that merely names a retired symbol
 *  cannot satisfy — or trip — the scan. */
const stripComments = (source: string) =>
  source.replace(/\/\*[\s\S]*?\*\//g, "").replace(/^\s*\/\/.*$/gm, "");

/** The two commands R87 deleted, plus the wire type that went with them. */
const SHOW_COMMAND = "show_plugin" + "_page";
const DESCRIPTOR_COMMAND = "plugin_page" + "_descriptor";
const WIRE_TYPE = "PluginPage" + "Info";

test("the two retired page commands stay out of the command table", async () => {
  const lib = stripComments(await read("src-tauri/src/lib.rs"));
  const pluginPages = stripComments(await read("src-tauri/src/plugin_pages.rs"));

  // Non-vacuity: the live commands the two sat beside must still be there, so a
  // misspelled path or an emptied file cannot pass as "already gone".
  for (const command of [
    "show_terminal",
    "builtin_plugins_list",
    "take_pending_plugin_page",
    "open_plugin_page",
  ]) {
    assert.ok(
      lib.includes(command) || pluginPages.includes(command),
      `${command} must stay registered`,
    );
  }

  assert.ok(
    !lib.includes(SHOW_COMMAND),
    `${SHOW_COMMAND} must not be defined or registered`,
  );
  assert.ok(
    !lib.includes(DESCRIPTOR_COMMAND),
    `${DESCRIPTOR_COMMAND} must not be registered`,
  );
  assert.ok(
    !pluginPages.includes(DESCRIPTOR_COMMAND),
    `${DESCRIPTOR_COMMAND} must not be defined`,
  );

  // The page-table lookup the wrapper used stays: the registry suite resolves
  // descriptors through it (`descriptor(CLIPBOARD_PLUGIN_ID)` and friends), so
  // it is a live accessor, not an orphan.
  assert.ok(pluginPages.includes("pub fn descriptor"), "the registry lookup must stay");
  assert.ok(
    !pluginPages.includes(WIRE_TYPE),
    `${WIRE_TYPE} had no consumer once the wrapper went and must stay deleted`,
  );
});
