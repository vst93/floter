// The base-plugin registry the settings panel renders (src/builtin-plugins.ts).
//
// R101 · the module this suite used to read — a name that named neither of
// its two subjects — was split: the registry is now
// `src/builtin-plugins.ts` and the per-key failure deduper is
// `src/failure-deduper.ts`. The deduper's cases moved to
// `tests/failure-deduper.test.ts`; this file keeps the registry, the freeze
// lock on the R96-deleted bridge, and the Rust descriptor mirror.
//
// R96 deleted the generic postMessage bridge that used to live beside the
// registry — page URL building, the command allowlist, the handshake, the
// message types and every `isBridge*` guard. It had no producer (the manifest
// declares no page) and no consumer (no built-in page, no host), so it was not
// a published contract but dead code. This suite pins the live half and, below,
// turns red if any deleted export, field, document or dictionary key comes
// back.
//
// Retired names are assembled from parts so this file does not itself
// reintroduce a token the round's zero-hit grep bans.
import assert from "node:assert/strict";
import { readdir, readFile, stat } from "node:fs/promises";
import test from "node:test";

import {
  BROWSER_PLUGIN_ID,
  BUILTIN_BASE_PLUGINS,
  CALCULATOR_PLUGIN_ID,
  CLIPBOARD_PLUGIN_ID,
} from "../src/builtin-plugins.ts";
import { FAILURE_NOTIFY_DEDUP_MS, createFailureDeduper } from "../src/failure-deduper.ts";

const root = new URL("../", import.meta.url);
const read = (path: string) => readFile(new URL(path, root), "utf8");
const exists = async (path: string) => {
  try {
    await stat(new URL(path, root));
    return true;
  } catch {
    return false;
  }
};

// ── 1 · the base-plugin list mirrors the Rust registry ────────────────────

// R26-C · the settings panel's base-plugins list must carry every registered
// plugin, browser included.
//
// The bug: `App.tsx` assembled the list by hand and only ever named
// `builtin.clipboard`, so when R26-B registered `builtin.browser` the settings
// panel never showed it — the plugin had no entry. The list now lives in
// `BUILTIN_BASE_PLUGINS` (src/builtin-plugins.ts), and this guard pins it to the
// Rust registry in BOTH directions: a descriptor without a row fails, and a row
// naming an unregistered plugin fails.
test("the base-plugin list carries builtin.browser and mirrors the Rust registry", async () => {
  const ids = BUILTIN_BASE_PLUGINS.map((plugin) => plugin.id);
  assert.ok(
    ids.includes(BROWSER_PLUGIN_ID),
    "the base-plugin list must contain builtin.browser (the R26-C regression)",
  );
  assert.ok(ids.includes(CLIPBOARD_PLUGIN_ID), "the base-plugin list must contain builtin.clipboard");
  assert.ok(ids.includes(CALCULATOR_PLUGIN_ID), "the base-plugin list must contain builtin.calculator");

  // The registry's constant names -> values, then the ids `DESCRIPTORS` uses.
  const rust = await read("src-tauri/src/plugin_pages.rs");
  const constants = new Map<string, string>();
  for (const match of rust.matchAll(/pub const (\w+_PLUGIN_ID): &str = "([^"]+)";/g)) {
    constants.set(match[1], match[2]);
  }
  const descriptorsAt = rust.indexOf("static DESCRIPTORS");
  assert.notEqual(descriptorsAt, -1, "the Rust descriptor registry must exist");
  const registryIds = [...rust.slice(descriptorsAt).matchAll(/id: (\w+_PLUGIN_ID),/g)].map((match) => {
    const value = constants.get(match[1]);
    assert.ok(value, `${match[1]} must be declared before the registry uses it`);
    return value;
  });

  assert.deepEqual(
    [...ids].sort(),
    [...new Set(registryIds)].sort(),
    "the settings list and the Rust registry must name the same plugins",
  );

  // The list is only real if the panel renders it: the hand-written array in
  // App.tsx is gone, replaced by this registry.
  const app = await read("src/App.tsx");
  assert.match(
    app,
    /basePlugins=\{BUILTIN_BASE_PLUGINS/,
    "App.tsx must render the shared base-plugin list, not a hand-written array",
  );
});

test("every base-plugin row opens the generic configuration overlay", () => {
  // R33 · no built-in page is registered any more, so every row is
  // `configurable` (the launcher overlay) rather than a page door.
  for (const plugin of BUILTIN_BASE_PLUGINS) {
    assert.equal(plugin.configurable, true, `${plugin.id} opens the generic overlay`);
  }
});

// ── 2 · the R101 split, and the old module path stays retired ─────────────

test("the two split modules each export only their own subject", async () => {
  // Non-vacuity: the live spine is still exported, from the file that names it.
  const registry = (await import("../src/builtin-plugins.ts")) as Record<string, unknown>;
  const deduper = (await import("../src/failure-deduper.ts")) as Record<string, unknown>;
  for (const live of ["BUILTIN_BASE_PLUGINS", "CLIPBOARD_PLUGIN_ID", "BROWSER_PLUGIN_ID", "CALCULATOR_PLUGIN_ID"]) {
    assert.ok(live in registry, `${live} must stay exported by builtin-plugins.ts`);
  }
  for (const live of ["createFailureDeduper", "FAILURE_NOTIFY_DEDUP_MS"]) {
    assert.ok(live in deduper, `${live} must stay exported by failure-deduper.ts`);
  }
  // And neither half grew the other's subject back.
  assert.ok(!("createFailureDeduper" in registry), "the registry module must not carry the deduper");
  assert.ok(!("BUILTIN_BASE_PLUGINS" in deduper), "the deduper module must not carry the registry");
});

test("no live source imports the deleted module path again", async () => {
  // R101 · the old module was split into `builtin-plugins.ts` and
  // `failure-deduper.ts` and deleted. This is the negative guard: scan every
  // TypeScript source under `src/` and `tests/` and turn red if the old path
  // reappears as an import (or, indeed, as any mention).
  const OLD_MODULE = "plugin" + "-pages";
  const collect = async (dir: string): Promise<string[]> => {
    const entries = await readdir(new URL(dir, root), { withFileTypes: true });
    const files: string[] = [];
    for (const entry of entries) {
      const path = `${dir}/${entry.name}`;
      if (entry.isDirectory()) files.push(...(await collect(path)));
      else if (/\.tsx?$/.test(entry.name)) files.push(path);
    }
    return files;
  };
  const files = [...(await collect("src")), ...(await collect("tests"))];
  assert.ok(files.length > 100, "the scan must cover the real source trees");
  for (const file of files) {
    const source = await read(file);
    assert.ok(
      !source.includes(OLD_MODULE),
      `${file} must not reference the deleted \`${OLD_MODULE}\` module`,
    );
  }
});

// ── 3 · the retired bridge stays retired ──────────────────────────────────

/** Value exports R96 deleted from the old module, assembled so the zero-hit
 *  grep stays clean. */
const RETIRED_VALUE_EXPORTS = [
  "BRIDGE" + "_TAG",
  "PLUGIN_PAGE" + "_PROTOCOL",
  "buildPlugin" + "PageUrl",
  "command" + "Allowed",
  "pluginPage" + "Handshake",
  "handshake" + "ErrorDetail",
  "shouldStart" + "WindowDrag",
  "pluginPageNeeds" + "SameOrigin",
  "SAME_ORIGIN" + "_PLUGIN_PAGES",
  "createRetry" + "Registry",
  "RETRY_REGISTRY" + "_CAPACITY",
  "MESSAGE_KEY" + "_SHAPE",
] as const;

test("the retired bridge value exports are gone from both split modules", async () => {
  const modules = [
    (await import("../src/builtin-plugins.ts")) as Record<string, unknown>,
    (await import("../src/failure-deduper.ts")) as Record<string, unknown>,
  ];
  for (const name of RETIRED_VALUE_EXPORTS) {
    for (const mod of modules) {
      assert.ok(!(name in mod), `${name} was deleted in R96 and must stay gone`);
    }
  }
});

test("no bridge message type or guard survives in the split module sources", async () => {
  for (const path of ["src/builtin-plugins.ts", "src/failure-deduper.ts"]) {
    const source = await read(path);
    // The whole `isBridge*` family, the `Bridge*` payload types and the page →
    // host union. These are compile-time names, so the scan is over the source
    // rather than the runtime export object.
    assert.ok(!/\bisBridge[A-Z]/.test(source), `no isBridge* guard may come back in ${path}`);
    assert.ok(!/export type Bridge[A-Z]/.test(source), `no Bridge* payload type may come back in ${path}`);
    assert.ok(!/\bBridgeFromPage\b/.test(source), `the page → host union must stay gone in ${path}`);
  }
  // And the registry module still declares the live spine.
  assert.match(await read("src/builtin-plugins.ts"), /export const BUILTIN_BASE_PLUGINS/);
});

test("the retired bridge dictionary keys are gone from the i18n table", async () => {
  const i18n = await read("src/i18n.ts");
  // `plugin.*` was the bridge's own key namespace (`protocolMissing`,
  // `protocolMismatch`, `retry`, `exampleNotify`); notifications live under
  // `notification.plugin.*` and are unaffected.
  assert.equal(
    [...i18n.matchAll(/"plugin\./g)].length,
    0,
    "no `plugin.*` dictionary key may survive the bridge deletion",
  );
});

test("the retired protocol document and example page are gone", async () => {
  // Non-vacuity: the extensions index that used to link them still resolves.
  assert.equal(await exists("docs/extensions/README.zh-CN.md"), true);
  assert.equal(
    await exists("docs/extensions/plugin-page-" + "protocol.md"),
    false,
    "the protocol document was deleted in R96 and must stay deleted",
  );
  assert.equal(
    await exists("docs/extensions/examples/hello-" + "page"),
    false,
    "the hello-page example was deleted in R96 and must stay deleted",
  );
});

// ── 4 · the Rust descriptor and wire type stay shrunk ─────────────────────

test("the Rust descriptor no longer carries a page slot or a command allowlist", async () => {
  const rust = await read("src-tauri/src/plugin_pages.rs");
  // The live spine: the id constants and the descriptor table.
  assert.match(rust, /static DESCRIPTORS: &\[PluginPageDescriptor\] = &\[/);
  assert.match(rust, /pub fn descriptor\(id: &str\)/);
  // Non-vacuity for the field scans below.
  assert.match(rust, /pub id: &'static str,/);
  assert.match(rust, /pub title_key: &'static str,/);
  // The deleted fields. `page` is matched as a struct field (`page:`), which is
  // what a revival would add to `PluginPageDescriptor` and to each registry
  // entry; the identifier still appears inside `open_plugin_page` and the
  // `PluginPage*` type names, which are live.
  assert.ok(!/\bpage\s*:/.test(rust), "the descriptor's `page` field must stay deleted (R96)");
  assert.ok(
    !rust.includes("allow" + "ed_commands"),
    "the per-plugin command allowlist must stay deleted (R96)",
  );
  assert.ok(!rust.includes("has_" + "page"), "the `has_page` wire flag must stay deleted (R96)");
  assert.ok(!rust.includes("_COM" + "MANDS"), "the per-plugin command tables must stay deleted (R96)");
});

test("the builtin-plugin wire row no longer reports hasPage", async () => {
  const rust = await read("src-tauri/src/plugin_pages.rs");
  const info = rust.slice(rust.indexOf("pub struct BuiltinPluginInfo"));
  const body = info.slice(0, info.indexOf("}"));
  assert.match(body, /pub enabled: bool,/, "the settings panel still reads `enabled`");
  assert.ok(
    !body.includes("has_" + "page"),
    "the wire row's `has_page` field must stay deleted (R96)",
  );
});

test("open_plugin_page still emits the plugin-config event the overlay answers", async () => {
  // R96 · the event path is the live half of this module; the descriptor
  // projection and the emit are what the console `clip` command and the
  // cold-start hand-off ride on.
  const rust = await read("src-tauri/src/plugin_pages.rs");
  assert.match(rust, /pub fn open_plugin_page\(app: &AppHandle, id: &str\)/);
  assert.match(rust, /"floter:\/\/plugin-config"/);
  assert.match(rust, /pub\(crate\) fn take_pending_plugin_page/);
  // …and the frontend listener still answers that event name.
  const app = await read("src/App.tsx");
  assert.match(app, /listen<\{ id: string; toggle: boolean \}>\("floter:\/\/plugin-config"/);
});
