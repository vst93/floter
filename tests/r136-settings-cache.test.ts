// R136 · the settings snapshot cache guard.
//
// R134's command-layer census named the settings re-read as the command layer's
// hot-path tax: every settings read was a disk read plus a JSON parse, and the
// command layer performs one per launcher summon and ≥3 per browser search.
// R136 installs a process-wide snapshot keyed on (settings path, mtime) and
// dropped by every successful write.
//
// This guard reads the source, not the behaviour — the behavioural half is the
// two `commands::config::tests` cases (`a_write_invalidates_the_cached_snapshot`
// and `an_external_mtime_bump_rereads_the_file`). Every token it counts is
// assembled from fragments, and its last test proves the file does not spell
// the literals it scans for.
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

const root = new URL("../", import.meta.url);
const read = (path: string) => readFile(new URL(path, root), "utf8");
const count = (haystack: string, needle: string) => haystack.split(needle).length - 1;

/**
 * Rust line comments dropped before counting (the same convention r123 uses):
 * a comment that names a call is not a call, so the counts describe code and
 * not prose.
 */
const code = (source: string) => source.replace(/\/\/[^\n]*/g, "");

const CONFIG = "src-tauri/src/commands/config.rs";
const LIB = "src-tauri/src/lib.rs";
const R123 = "tests/r123-startup-hygiene.test.ts";

// Counted/anchored tokens, assembled so this guard does not spell them.
const SNAPSHOT = "SETTINGS_" + "SNAPSHOT";
const CACHE_DECL = "static " + SNAPSHOT + ": RwLock<Option<SettingsSnapshot>>";
const CACHED_FN = "fn cached_" + "settings(";
const CACHED_CALL = "cached_" + "settings(&";
const LOAD_FN = "pub fn load_" + "settings()";
const LOAD_FROM = "fn load_" + "settings_from(";
const LOAD_DELEGATION = "load_" + "settings_from(&config_dir)";
const READ_SETTINGS = "read_" + "settings(&";
const MTIME_FN = "fn settings_" + "mtime(";
const MTIME_CALL = "settings_" + "mtime(path)";
const HIT_RETURN = "Some(" + "settings)";
const INVALIDATE = "invalidate_" + "settings_snapshot();";
const WRITE_SETTINGS = "fn write_" + "settings(settings: &AppSettings)";
const WRITE_TO = "fn write_" + "settings_to(";
const LOAD_SETTINGS = "load_" + "settings()";
const R123_CONST = "LOAD_" + "SETTINGS";

/**
 * The invalidation points this round registers, by the function that carries
 * the call. The list is deliberately one entry long: every production writer
 * calls `write_settings`, which delegates to `write_settings_to`, and the
 * tests write through the same funnel — so the single point covers every path
 * that lands settings.json, and the count assertion below refuses a second
 * (drifted) invalidation site.
 */
const INVALIDATION_SITES: string[] = [WRITE_TO];

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

test("config.rs declares the snapshot slot and consults it before the disk", async () => {
  const source = code(await read(CONFIG));
  assert.ok(source.includes(CACHE_DECL), "the process-wide snapshot slot must be declared");
  assert.ok(source.includes(CACHED_FN), "the hit path must be one named function");

  // `load_settings` keeps its signature and reaches the cache through the
  // delegating helper — the chain the command layer actually walks.
  assert.ok(
    fnBody(source, LOAD_FN).includes(LOAD_DELEGATION),
    "load_settings must still delegate to the cache-aware helper",
  );

  const body = fnBody(source, LOAD_FROM);
  assert.ok(body.includes(CACHED_CALL), "the read path must consult the snapshot");
  assert.ok(body.includes(READ_SETTINGS), "a miss must still fall back to the disk read");
  assert.ok(
    body.indexOf(CACHED_CALL) < body.indexOf(READ_SETTINGS),
    "the snapshot must be consulted before the disk read",
  );
});

test("every settings write funnels through one invalidation point", async () => {
  const source = code(await read(CONFIG));

  for (const path of INVALIDATION_SITES) {
    assert.ok(fnBody(source, path).includes(INVALIDATE), `${path} must drop the snapshot`);
  }
  assert.equal(
    count(source, INVALIDATE),
    INVALIDATION_SITES.length,
    "the invalidation must live at exactly the registered points",
  );

  // `write_settings` is what every command calls; the funnel is only sound
  // while it keeps delegating to the registered point.
  assert.ok(
    fnBody(source, WRITE_SETTINGS).includes(WRITE_TO.slice("fn ".length)),
    "write_settings must still delegate to the registered funnel",
  );
});

test("the mtime check gates the cache hit", async () => {
  const source = code(await read(CONFIG));
  assert.ok(source.includes(MTIME_FN), "the mtime helper must exist");

  const body = fnBody(source, CACHED_FN);
  assert.ok(body.includes(MTIME_CALL), "the hit path must re-check the file's mtime");
  assert.ok(
    body.indexOf(MTIME_CALL) < body.lastIndexOf(HIT_RETURN),
    "the mtime check must precede the hit return",
  );
});

test("the r123 startup-read invariant still holds and is not hollow", async () => {
  // r123 pins the cold-start path to exactly one settings read; R136 must not
  // have added a second one (the snapshot serves the later callers instead).
  const source = code(await read(LIB));
  const start = source.indexOf("pub fn run()");
  assert.ok(start >= 0, "run() must exist");
  const end = source.indexOf(".invoke_handler(tauri::generate_handler![", start);
  assert.ok(end > start, "the builder wiring must still end at invoke_handler");
  const startup = source.slice(start, end);
  assert.equal(
    count(startup, LOAD_SETTINGS),
    1,
    "the cold-start path must still read the settings file exactly once",
  );

  // The r123 guard itself must still carry its own assertion — a deleted or
  // hollowed guard is not a passing one.
  const r123 = await read(R123);
  assert.ok(
    r123.includes(`count(startup, ${R123_CONST})`),
    "r123 must still count the startup reads through its assembled token",
  );
  assert.ok(
    r123.includes("the startup path must read the settings file once"),
    "r123 must still carry the once-only assertion",
  );
});

test("this guard assembles the tokens it counts, it does not spell them", async () => {
  const self = await read("tests/r136-settings-cache.test.ts");
  for (const token of [
    CACHE_DECL,
    CACHED_CALL,
    READ_SETTINGS,
    INVALIDATE,
    MTIME_CALL,
    LOAD_SETTINGS,
  ]) {
    assert.ok(!self.includes(token), `the guard must not spell ${token}`);
  }
  assert.ok(self.includes("CACHE_DECL"), "the guard must still use the assembled constant");
});
