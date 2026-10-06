// R123 · the startup-path hygiene guard.
//
// R122 surveyed the cold start and left a ranked candidate list. R123 lands the
// two low-risk points (semantics unchanged, no visible behaviour):
//
//   1. `ensure_launch_at_startup` reconciled the registration by writing it
//      again on every launch. On Linux that meant an unconditional
//      `create_dir_all` + `fs::write` on the steady-state path; on macOS a
//      `plist::to_file_xml`. Both now read the existing entry back and return
//      before touching anything when it already says what it would write.
//      (Windows goes through the registry API — no file to read — and is left
//      as it is; the R123 report registers that.)
//   2. The cold-start path read the settings file twice: once through
//      `saved_terminal_size()` in the `AppState` constructor and once in the
//      `setup` closure. The constructor now installs the same fallback the
//      getter uses, and the one read `setup` already performed fills the slot
//      — so the startup path reads the file once.
//
// The guard reads the source, not the behaviour (a Rust-side assertion would
// be a scope breach for this round). Every token it counts is assembled from
// fragments, and its last test proves the file does not spell the literal it
// scans for.
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

const root = new URL("../", import.meta.url);
const read = (path: string) => readFile(new URL(path, root), "utf8");
const count = (haystack: string, needle: string) => haystack.split(needle).length - 1;

/**
 * Rust line comments dropped before counting: a comment that names a call is
 * not a call, so the counts below describe code and not prose. Truncating at a
 * `//` inside a string literal can only *remove* text, and no counted token
 * lives in one, so the counts stay honest either way.
 */
const code = (source: string) => source.replace(/\/\/[^\n]*/g, "");

const AUTOSTART = "src-tauri/src/commands/autostart.rs";
const LIB = "src-tauri/src/lib.rs";
const CONFIG = "src-tauri/src/commands/config.rs";

// Counted tokens, assembled so this guard does not spell them.
const LOAD_SETTINGS = "load_" + "settings()";
const SAVED_TERMINAL_SIZE = "saved_" + "terminal_size";
const NORMALIZE_SIZE = "normalize_" + "terminal_size";
const READ_ENTRY = "fs::" + "read";
const WRITE_ENTRY = "fs::" + "write";
const READ_PLIST = "Value::" + "from_file";
const WRITE_PLIST = "to_file_" + "xml";
const CREATE_DIR = "create_dir_" + "all";
// The deferred constructor value and the setup-side fill (task 1).
const LAZY_DEFAULT = "terminal_height: Mutex::new(" + "TERMINAL_WINDOW_HEIGHT)";
const FILL_SLOT =
  NORMALIZE_SIZE + "(settings.terminal_width, settings.terminal_height)";

/**
 * One `apply_launch_at_startup` cfg branch: from its `#[cfg(...)]` attribute to
 * the next cfg attribute in the file. The platform bodies are the region
 * `ensure_launch_at_startup` delegates to, so this is where the read-compare-
 * write shape has to live.
 */
function branch(source: string, cfg: string): string {
  const marker = `#[cfg(${cfg})]\nfn apply_launch_at_startup`;
  const at = source.indexOf(marker);
  assert.ok(at >= 0, `the ${cfg} applier must exist`);
  const rest = source.slice(at);
  const next = rest.indexOf("#[cfg(", 1);
  return next === -1 ? rest : rest.slice(0, next);
}

test("every file-based autostart applier reads the entry back before it writes", async () => {
  const source = await read(AUTOSTART);

  // Linux · `floter.desktop`: byte-identical entry short-circuits before the
  // directory is even created, which is what makes the steady state zero-write.
  const linux = code(branch(source, 'target_os = "linux"'));
  assert.ok(linux.includes(READ_ENTRY), "the .desktop applier must read the entry back");
  assert.ok(
    linux.indexOf(READ_ENTRY) < linux.indexOf(WRITE_ENTRY),
    "the read must precede the write",
  );
  assert.ok(
    linux.indexOf(READ_ENTRY) < linux.indexOf(CREATE_DIR),
    "a byte-identical entry must not touch the autostart directory",
  );
  assert.ok(
    linux.includes("return Ok(())"),
    "an unchanged entry must short-circuit with the same Ok the write returns",
  );

  // macOS · `com.v.floter.plist`: the comparison is on the parsed value (a
  // plist's serializer whitespace is not part of the entry), same ordering.
  const macos = code(branch(source, 'target_os = "macos"'));
  assert.ok(macos.includes(READ_PLIST), "the plist applier must read the file back");
  assert.ok(
    macos.indexOf(READ_PLIST) < macos.indexOf(WRITE_PLIST),
    "the read must precede the write",
  );
  assert.ok(
    macos.indexOf(READ_PLIST) < macos.indexOf(CREATE_DIR),
    "an unchanged plist must not touch the LaunchAgents directory",
  );
  assert.ok(
    macos.includes("return Ok(())"),
    "an unchanged plist must short-circuit with the same Ok the write returns",
  );
});

test("the cold-start path reads the settings file exactly once", async () => {
  const source = await read(LIB);
  const start = source.indexOf("pub fn run()");
  assert.ok(start >= 0, "run() must exist");
  const end = source.indexOf(".invoke_handler(tauri::generate_handler![", start);
  assert.ok(end > start, "the builder wiring must still end at invoke_handler");
  const startup = code(source.slice(start, end));

  assert.equal(
    count(startup, LOAD_SETTINGS),
    1,
    "the startup path must read the settings file once, not once per caller",
  );
  assert.equal(
    count(startup, SAVED_TERMINAL_SIZE),
    0,
    "the AppState constructor may not read the terminal size back off disk",
  );
  assert.ok(startup.includes(LAZY_DEFAULT), "the constructor must install the lazy fallback");
  assert.ok(
    startup.includes(FILL_SLOT),
    "the setup closure must fill the deferred slot from the settings it read",
  );
});

test("the anchors this guard leans on are still live, not a hollow shell", async () => {
  // `saved_terminal_size` stays in config.rs and still normalizes defensively:
  // the constructor change moved the caller, it did not move the clamp.
  const config = await read(CONFIG);
  const anchor = `pub fn ${SAVED_TERMINAL_SIZE}()`;
  assert.ok(config.includes(anchor), `${anchor} must stay in config.rs`);
  const body = config.slice(config.indexOf(anchor), config.indexOf(anchor) + 400);
  assert.ok(body.includes(NORMALIZE_SIZE), "the saved size must still be normalized");

  // The delegation and the signatures are unchanged: task 2 is a body edit.
  const autostart = await read(AUTOSTART);
  assert.ok(
    autostart.includes(
      "pub fn ensure_launch_at_startup(enabled: bool) -> Result<(), String> {\n    apply_launch_at_startup(enabled)\n}",
    ),
    "ensure_launch_at_startup keeps its signature and its delegation",
  );
  assert.ok(
    autostart.includes("pub fn set_launch_at_startup(enabled: bool) -> Result<(), String> {"),
    "set_launch_at_startup keeps its signature",
  );
});

test("this guard assembles the token it counts, it does not spell it", async () => {
  const self = await read("tests/r123-startup-hygiene.test.ts");
  assert.ok(!self.includes(LOAD_SETTINGS), "the guard must not spell the call it counts");
  assert.ok(self.includes("LOAD_SETTINGS"), "the guard must still use the assembled constant");
});
