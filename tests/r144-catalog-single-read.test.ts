// R144 · one manifest read per catalog entry per rebuild, and no `--version`
// probe on a describe cache hit.
//
// Task 1 — `load_provider_commands_uncached` read and parsed the same installed
// manifest two or three times per entry: the interpreter language, the
// refreshed-binding validator, the static descriptor, `runtime_available` and
// the provider invocation each called a loader that read the file itself. R144
// reads the manifest once (`entry_manifest`) and threads that one parse through
// the manifest-taking family.
//
// Task 2 — `ProviderManager::describe` spawned the provider's `--version` probe
// (a subprocess with a 2 s timeout) *before* it consulted the cache, so even a
// fresh cache hit paid for it. R144 moves the probe after the cache check.
//
// This file is a source pin. A node test cannot spawn a provider, so the
// behavioural half lives in the Rust suite
// (`catalog::tests::a_cached_describe_does_not_spawn_*`, which counts the
// probes the fixture actually sees). What this file keeps is the shape: no bare
// loader in the loop, and the probe after the cache.
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

const root = new URL("../", import.meta.url);
const read = (path: string) => readFile(new URL(path, root), "utf8");

// Assembled from fragments so this guard never spells the anchors it scans for.
const CATALOG = "src-tauri/src/extensions/" + "catalog.rs";
const PROVIDER = "src-tauri/src/extensions/" + "provider.rs";
const LOOP = "load_provider_commands_" + "uncached";
const HELPER = "entry_" + "manifest";
const BARE_LOAD = "ExtensionManifest::" + "load";
const SINGLE_READ = HELPER + "(&entry)";
const WITH_MANIFEST = "_with_" + "manifest";
const PROBE = "provider_" + "version";
const CACHE_ANCHOR = "cache.modified_ms == " + "modified_ms";
// The loading forms that each re-read the manifest, and the manifest-taking
// forms that carry the one parse instead.
const LOADERS = [
  "static_description",
  "provider_invocation",
  "runtime_available",
  "entry_script_interpreter_language",
];
const THREADED = ["static_description", "provider_invocation", "runtime_available"].map(
  (name) => name + WITH_MANIFEST,
);

/** Remove Rust comments so an anchor can only be satisfied by real code. */
const stripComments = (source: string) =>
  source.replace(/\/\*[\s\S]*?\*\//g, "").replace(/^\s*\/\/.*$/gm, "");

/** The text of one item, from its signature through the matching `}`. */
const itemBody = (source: string, signature: string) => {
  const start = source.indexOf(signature);
  assert.notEqual(start, -1, `${signature} must exist`);
  const open = source.indexOf("{", start);
  assert.notEqual(open, -1, `${signature} must open a block`);
  let depth = 0;
  for (let index = open; index < source.length; index += 1) {
    if (source[index] === "{") depth += 1;
    else if (source[index] === "}") {
      depth -= 1;
      if (depth === 0) return source.slice(start, index);
    }
  }
  return assert.fail(`${signature} must close its block`);
};

test("R144 · the catalog rebuild reads each manifest exactly once", async () => {
  const source = stripComments(await read(CATALOG));
  const body = itemBody(source, "async fn " + LOOP + "(");
  assert.doesNotMatch(body, new RegExp(BARE_LOAD), "no bare manifest load in the loop");
  assert.equal(
    body.split(SINGLE_READ).length - 1,
    1,
    "the loop must call the single-read helper exactly once per entry",
  );
  // The bare loaders re-read the file; the loop must call none of them.
  for (const loader of LOADERS) {
    assert.doesNotMatch(
      body,
      new RegExp("registry::" + loader + "\\(\\s*&entry"),
      `the loop must not call the loading form of ${loader}`,
    );
  }
  // ...and it must call each manifest-taking form it needs.
  for (const threaded of THREADED) {
    assert.match(body, new RegExp(threaded + "\\("), `${threaded} must carry the one parse`);
  }
  // The one read is real, not merely relocated into a second loader.
  const helper = itemBody(source, "fn " + HELPER + "(");
  assert.match(helper, new RegExp(BARE_LOAD), "the helper owns the one read");
});

test("R144 · the version probe runs only after the describe cache check", async () => {
  const source = stripComments(await read(PROVIDER));
  const body = itemBody(source, "pub async fn describe(");
  const cache = body.indexOf(CACHE_ANCHOR);
  const probe = body.indexOf(PROBE);
  assert.notEqual(cache, -1, "the cache check must exist");
  assert.notEqual(probe, -1, "the version probe must exist");
  assert.ok(probe > cache, "the probe must sit after the cache check, not before it");
  // The probe must not be reachable from inside the cache-hit branch: slice
  // from the `!force` guard to the cache comparison and demand no probe there.
  const hit = body.slice(body.indexOf("if !force"), cache);
  assert.doesNotMatch(hit, new RegExp(PROBE), "a cache hit must not probe the version");
});

test("R144 · this guard assembles its anchors, it does not spell them", async () => {
  const self = await read("tests/r144-catalog-single-read.test.ts");
  for (const literal of [
    BARE_LOAD,
    SINGLE_READ,
    WITH_MANIFEST,
    PROBE,
    CACHE_ANCHOR,
    ...THREADED,
  ]) {
    assert.ok(!self.includes(literal), `the guard must not spell: ${literal}`);
  }
});
