// The surviving spine of floter's plugin surfaces (src/plugin-pages.ts): the
// base-plugin registry the settings panel renders and the per-key failure
// deduper the app's automatic triggers share.
//
// R96 deleted the generic postMessage bridge that used to live beside them —
// page URL building, the command allowlist, the handshake, the message types
// and every `isBridge*` guard. It had no producer (the manifest declares no
// page) and no consumer (no built-in page, no host), so it was not a published
// contract but dead code. This suite pins the live half and, at the bottom,
// turns red if any deleted export, field, document or dictionary key comes
// back.
//
// Retired names are assembled from parts so this file does not itself
// reintroduce a token the round's zero-hit grep bans.
import assert from "node:assert/strict";
import { readFile, stat } from "node:fs/promises";
import test from "node:test";

import {
  BROWSER_PLUGIN_ID,
  BUILTIN_BASE_PLUGINS,
  CALCULATOR_PLUGIN_ID,
  CLIPBOARD_PLUGIN_ID,
  FAILURE_NOTIFY_DEDUP_MS,
  createFailureDeduper,
} from "../src/plugin-pages.ts";

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
// `BUILTIN_BASE_PLUGINS` (src/plugin-pages.ts), and this guard pins it to the
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

// ── 2 · the failure deduper ───────────────────────────────────────────────

test("five consecutive automatic failures raise exactly one toast (30s dedupe per key)", () => {
  // Drive the shared dedupe policy directly: an automatic trigger calls this
  // once per failed attempt, and the burst below is 20s. Only the first may
  // paint; the others must be swallowed.
  assert.ok(
    FAILURE_NOTIFY_DEDUP_MS >= 30_000,
    "the window must dwarf a 2s poll — at 30s an automatic failure can earn at most one toast",
  );
  const deduper = createFailureDeduper();
  const raised: number[] = [];
  let now = 1_000_000;
  for (let attempt = 0; attempt < 5; attempt += 1) {
    if (deduper.allow("some.loadFailed", now)) raised.push(now);
    now += 2000; // the poll interval
  }
  assert.deepEqual(raised, [1_000_000], "the outage must be one toast, not five");
  // The window is per key: a *different* failure in the middle of the outage
  // is still news and is not swallowed by the first key.
  assert.equal(deduper.allow("some.copyFailed", now), true, "dedupe must be per message key");
  // Recovery re-arms: failure → success → failure is two toasts, not one.
  deduper.clear("some.loadFailed");
  assert.equal(deduper.allow("some.loadFailed", now), true, "a success in between makes the relapse news again");
  // And the window really elapses: the next failure after it is announced.
  const later = createFailureDeduper();
  assert.equal(later.allow("k", 0), true);
  assert.equal(later.allow("k", FAILURE_NOTIFY_DEDUP_MS - 1), false);
  assert.equal(later.allow("k", FAILURE_NOTIFY_DEDUP_MS), true);
});

// ── 3 · the retired bridge stays retired ──────────────────────────────────

/** Value exports R96 deleted from `src/plugin-pages.ts`, assembled so the
 *  zero-hit grep stays clean. */
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

test("the retired bridge value exports are gone from the module", async () => {
  const mod = (await import("../src/plugin-pages.ts")) as Record<string, unknown>;
  // Non-vacuity: the live spine is still exported, so an emptied module could
  // not pass as "already gone".
  for (const live of ["BUILTIN_BASE_PLUGINS", "createFailureDeduper", "FAILURE_NOTIFY_DEDUP_MS"]) {
    assert.ok(live in mod, `${live} must stay exported`);
  }
  for (const name of RETIRED_VALUE_EXPORTS) {
    assert.ok(!(name in mod), `${name} was deleted in R96 and must stay gone`);
  }
});

test("no bridge message type or guard survives in the module source", async () => {
  const source = await read("src/plugin-pages.ts");
  // The whole `isBridge*` family, the `Bridge*` payload types and the page →
  // host union. These are compile-time names, so the scan is over the source
  // rather than the runtime export object.
  assert.ok(!/\bisBridge[A-Z]/.test(source), "no isBridge* guard may come back");
  assert.ok(!/export type Bridge[A-Z]/.test(source), "no Bridge* payload type may come back");
  assert.ok(!/\bBridgeFromPage\b/.test(source), "the page → host union must stay gone");
  // And the module still declares the live spine.
  assert.match(source, /export const BUILTIN_BASE_PLUGINS/);
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
