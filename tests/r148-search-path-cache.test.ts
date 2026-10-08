// R148 · the search-path cache and the detach rollback.
//
// Task 1 — `search` read `local-commands.json` and `catalog-usage.json` from
// disk on every keystroke, and `list_applications` recomputed the application
// source signature (a walk of every `.desktop` root) on every summon. R148
// memoizes both catalog files on `(path, mtime)` and reuses the signature for
// the same 30 s window `check_applications` already uses.
//
// Task 2 — `detach_plugin_window` parked a slot/key pair before the window
// builder ran; a failed build left the pair behind. R148 rolls it back.
//
// This file is a source pin. A node test cannot touch a Rust private function,
// so the behavioural half lives in the Rust suite
// (`catalog::tests::cached_local_entries_reuses_until_mtime_changes`,
// `catalog::tests::repeated_searches_share_the_cached_catalog_files`, and
// `detach_plugin_window_tests::detach_failure_rolls_back_pending_slot`). What
// this file keeps is the shape: the memo before the read, the cooldown before
// the walk, and the rollback inside the failure branch.
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

const root = new URL("../", import.meta.url);
const read = (path: string) => readFile(new URL(path, root), "utf8");

// Assembled from fragments so this guard never spells the anchors it scans for.
const CATALOG = "src-tauri/src/extensions/" + "catalog.rs";
const APPS = "src-tauri/src/commands/apps/" + "mod.rs";
const LIB = "src-tauri/src/" + "lib.rs";

const LOCAL_FN = "fn " + "local_entries(";
const USAGE_FN = "fn " + "load_usage(";
const LOCAL_SLOT = "static " + "LOCAL_ENTRIES_CACHE";
const USAGE_SLOT = "static " + "USAGE_CACHE";
const CACHE_LOOKUP = "cached_" + "catalog_file(&";
const LOOKUP_FN = "fn " + "cached_catalog_file" + "<T>(";
const MTIME_FN = "fn " + "catalog_" + "file_mtime(";
const MTIME_CALL = "catalog_file_" + "mtime(";
const STORE = "store_" + "catalog_file(";
const MODIFIED = ".modified()";

const SIG_SLOT = "static " + "SOURCE_SIGNATURE_CACHE";
const SIG_HELPER = "fn " + "cached_source_signature(";
const SIG_CALL = "cached_" + "source_signature(&";
const COOLDOWN = "signature_" + "check_interval()";
const DURATION_SINCE = "duration_since(";
const LIST_FN = "pub async fn " + "list_applications(";
const WALK = "platform::" + "source_signature(";

const BUILD = "builder." + "build()";
const MAP_ERR = ".map_" + "err(";
const ROLLBACK = "rollback_" + "pending_plugin_window(";
const DETACH_FN = "fn " + "detach_plugin_window(";

/** Remove Rust comments so an anchor can only be satisfied by real code. */
const stripComments = (source: string) =>
  source.replace(/\/\*[\s\S]*?\*\//g, "").replace(/^\s*\/\/.*$/gm, "");

/** The body of `marker`'s function, up to the next top-level `fn`. */
function fnBody(source: string, marker: string): string {
  const at = source.indexOf(marker);
  assert.ok(at >= 0, `${marker} must exist`);
  const rest = source.slice(at);
  const next = rest.indexOf("\nfn ", 1);
  const nextPub = rest.indexOf("\npub fn ", 1);
  const cuts = [next, nextPub].filter((index) => index !== -1);
  return cuts.length === 0 ? rest : rest.slice(0, Math.min(...cuts));
}

test("R148 · both catalog files are memoized on (path, mtime)", async () => {
  const source = stripComments(await read(CATALOG));
  assert.ok(source.includes(LOCAL_SLOT), "local-commands.json needs its own slot");
  assert.ok(source.includes(USAGE_SLOT), "catalog-usage.json needs its own slot");

  for (const [marker, label] of [
    [LOCAL_FN, "local_entries"],
    [USAGE_FN, "load_usage"],
  ] as const) {
    const body = fnBody(source, marker);
    assert.ok(body.includes(CACHE_LOOKUP), `${label} must consult the memo before reading`);
    assert.ok(body.includes(STORE), `${label} must fill the memo on a miss`);
    assert.ok(
      body.indexOf(CACHE_LOOKUP) < body.indexOf(STORE),
      `${label} must consult the memo before filling it`,
    );
  }

  // The stat that validates a hit lives in its own helper, and the read path
  // captures it before the read so a mid-parse write cannot be cached under
  // the new file's timestamp.
  assert.ok(source.includes(MTIME_FN), "the mtime helper must exist");
  const helper = fnBody(source, MTIME_FN);
  assert.ok(helper.includes("metadata("), "the mtime helper must stat the file");
  assert.ok(helper.includes(MODIFIED), "the mtime helper must read the mtime");

  const lookup = fnBody(source, LOOKUP_FN);
  assert.ok(lookup.includes(MTIME_CALL), "the hit path must re-check the file's mtime");
  for (const [marker, label] of [
    [LOCAL_FN, "local_entries"],
    [USAGE_FN, "load_usage"],
  ] as const) {
    const body = fnBody(source, marker);
    assert.ok(body.includes(MTIME_CALL), `${label} must capture the mtime before the read`);
    assert.ok(
      body.indexOf(MTIME_CALL) < body.indexOf("std::fs::read("),
      `${label} must capture the mtime before it reads`,
    );
  }
});

test("R148 · the source signature is reused inside a 30 s window", async () => {
  const source = stripComments(await read(APPS));
  assert.ok(source.includes(SIG_SLOT), "the signature slot must be declared");

  const helper = fnBody(source, SIG_HELPER);
  assert.ok(
    helper.includes(COOLDOWN),
    "the cooldown must reuse check_applications' interval constant",
  );
  assert.ok(helper.includes(DURATION_SINCE), "the hit path must measure the stored instant");
  const walk = helper.indexOf(WALK);
  assert.ok(walk !== -1, "the helper must still perform the walk");
  assert.ok(
    helper.indexOf(COOLDOWN) < walk,
    "the cooldown check must precede the walk, not follow it",
  );

  // The list path reaches the cache-aware helper instead of the bare walk.
  const list = fnBody(source, LIST_FN);
  assert.ok(list.includes(SIG_CALL), "list_applications must call the cached helper");
});

test("R148 · a failed detach build rolls back the parked pair", async () => {
  const source = stripComments(await read(LIB));
  const detach = fnBody(source, DETACH_FN);

  const build = detach.indexOf(BUILD);
  assert.notEqual(build, -1, "the window build must exist");
  const mapErr = detach.indexOf(MAP_ERR, build);
  assert.notEqual(mapErr, -1, "the build must keep its map_err chain");

  const rollback = detach.indexOf(ROLLBACK, mapErr);
  const message = detach.indexOf("error.to_string()", mapErr);
  assert.ok(rollback > mapErr, "the failure branch must call the rollback");
  assert.ok(message > rollback, "the rollback must run before the error is returned");
  assert.ok(detach.indexOf("?", message) > message, "the failure must still propagate");
  assert.equal(
    detach.split(ROLLBACK).length - 1,
    1,
    "the rollback must live in the failure branch alone",
  );
});

test("R148 · this guard assembles its anchors, it does not spell them", async () => {
  const self = await read("tests/r148-search-path-cache.test.ts");
  for (const literal of [
    LOCAL_SLOT,
    USAGE_SLOT,
    CACHE_LOOKUP,
    LOOKUP_FN,
    MTIME_CALL,
    STORE,
    SIG_SLOT,
    SIG_CALL,
    COOLDOWN,
    WALK,
    BUILD,
    ROLLBACK,
  ]) {
    assert.ok(!self.includes(literal), `the guard must not spell: ${literal}`);
  }
  assert.ok(self.includes("LOCAL_SLOT"), "the guard must still use the assembled constants");
});
