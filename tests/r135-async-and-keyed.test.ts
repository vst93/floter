// R135 · two closures, one guard.
//
// Task 1 — the three export-family commands in `commands/extensions.rs` did
// their blocking filesystem work on the async worker: `extensions_export` built
// and wrote the sync document while holding the mutation lock, and
// `extensions_custom_export_script` / `extensions_config_export` wrote their
// payloads synchronously. Each blocking step now runs inside the runtime's
// `spawn_blocking`, joined before the command answers.
// The export's mutation lock still brackets the whole read+write: the guard is
// taken before the blocking call and never moves into the closure — a lock
// guard must not cross threads, and the critical section must not shrink.
//
// Task 2 — the last three hardcoded dialog-closed sentences join the R127
// family: the permission-review dialog behind `extensions_import`, and the
// connect-package picker's two channel drops (`extensions_pick_local_package`)
// now send the `.permissionReview` and `.localPackage` keys. The panel's
// `errorMessage(error, t)` translates them behind `isMessageKey`; the
// connect-package consumer was the one display point still passing no
// translator, so it was wired (re-registered in the R127 guard's count).
//
// Every scanned literal is assembled from fragments, and the last test proves
// this guard does not spell the strings it looks for.
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

const root = new URL("../", import.meta.url);
const read = (path: string) => readFile(new URL(path, root), "utf8");
const count = (haystack: string, needle: string) => haystack.split(needle).length - 1;

const EXTENSIONS = "src-tauri/src/commands/" + "extensions.rs";
const I18N = "src/" + "i18n.ts";
const PANEL = "src/" + "ExtensionsPanel.tsx";
const RUN_ERROR_RS = "src-tauri/src/extensions/" + "run_error.rs";
const R127 = "tests/r127-form-" + "errors.test.ts";

// Assembled so this guard does not spell the strings it scans for.
const BLOCKING = "tauri::async_runtime::" + "spawn_blocking";
const LOCK = "state.mutation_lock." + "lock().await";
const PREFIX = "settings.extensions." + "pickerClosed.";
const pickerKey = (context: string) => PREFIX + context;
const OLD_PERMISSION = "Permission review dialog " + "closed unexpectedly";
const OLD_LOCAL = "Local package picker " + "closed unexpectedly";
const signature = (name: string) => "pub async fn " + name + "(";

/** The slice of a Rust file between two signatures (end marker exclusive). */
const between = (source: string, start: string, end: string): string => {
  const from = source.indexOf(start);
  assert.ok(from >= 0, `start signature not found: ${start}`);
  const to = source.indexOf(end, from + start.length);
  assert.ok(to > from, `end signature not found after start: ${end}`);
  return source.slice(from, to);
};

/** The three export-family bodies: [name, own signature, next signature]. */
const REGIONS: Array<[string, string, string]> = [
  ["extensions_export", signature("extensions_export"), signature("extensions_import")],
  [
    "extensions_custom_export_script",
    signature("extensions_custom_export_script"),
    signature("extensions_script_runtime_check"),
  ],
  [
    "extensions_config_export",
    signature("extensions_config_export"),
    "fn reject_bundled_static_configuration(",
  ],
];

/** The five R127 picker contexts that must survive this round. */
const R127_CONTEXTS = ["export", "import", "localManifest", "scriptExport", "configurationExport"];

test("the three export-family commands run their blocking IO off the async worker", async () => {
  const source = await read(EXTENSIONS);
  for (const [name, start, end] of REGIONS) {
    const region = between(source, start, end);
    assert.ok(region.includes(BLOCKING), `${name} must move its blocking IO into ${BLOCKING}`);
    // Non-vacuous: the region really is the command body.
    assert.ok(region.includes(".map_err(|_|"), `${name} must map its error channel`);
    // The join is awaited before the command returns.
    const blockingAt = region.indexOf(BLOCKING);
    assert.ok(
      region.indexOf(".await", blockingAt) > blockingAt,
      `${name} must await the blocking join`,
    );
  }
});

test("the export's mutation lock still brackets the whole read+write", async () => {
  const source = await read(EXTENSIONS);
  const region = between(source, REGIONS[0][1], REGIONS[0][2]);
  const lockAt = region.indexOf(LOCK);
  const blockingAt = region.indexOf(BLOCKING);
  assert.ok(lockAt >= 0, "the export must still take the mutation lock");
  assert.ok(blockingAt > lockAt, "the lock must be taken before the blocking call");
  // The guard must not cross into the blocking thread: `mutation_lock` (and the
  // `.lock()` that yields the guard) appear exactly once — the guard line — and
  // the closure re-enters the state through the owned `AppHandle` instead.
  assert.equal(count(region, "mutation_lock"), 1, "the lock guard must not move into the closure");
  assert.equal(count(region, ".lock()"), 1, "the lock must be acquired once, on the async side");
  assert.ok(region.includes("app.state::<ExtensionState>()"), "the closure must re-enter the state");
});

test("the three dialog-closed sentences are keyed, not spelled in English", async () => {
  const source = await read(EXTENSIONS);
  assert.equal(count(source, OLD_PERMISSION), 0, "the permission-review sentence must be keyed");
  assert.equal(count(source, OLD_LOCAL), 0, "the local-package sentence must be keyed");

  // Each key is carried by the body that owns the dialog it describes.
  const importRegion = between(
    source,
    signature("extensions_import"),
    signature("extensions_install"),
  );
  assert.ok(
    importRegion.includes(pickerKey("permissionReview")),
    "the import review must carry its dictionary key",
  );

  const pickRegion = between(
    source,
    signature("extensions_pick_local_package"),
    "pub fn extensions_local_manifest_review(",
  );
  // Both channel drops (the folder prompt and the file picker) share one key —
  // same sentence, same shape.
  assert.equal(
    count(pickRegion, pickerKey("localPackage")),
    2,
    "the picker's two drops must share the one key",
  );
});

test("the two new keys sit on both dictionary sides, exactly once each", async () => {
  const i18n = await read(I18N);
  for (const context of ["permissionReview", "localPackage"]) {
    const key = pickerKey(context);
    assert.equal(count(i18n, `"${key}":`), 2, `${key} must have an English and a Chinese entry`);
    const entries = i18n.split("\n").filter((line) => line.includes(`"${key}":`));
    assert.equal(entries.length, 2, `${key} must carry a value on both sides`);
    assert.notEqual(entries[0], entries[1], `${key} languages must not share one string`);
    assert.ok(
      entries.every((line) => line.length > key.length + 4),
      `neither ${key} entry may be blank`,
    );
  }
});

test("the connect-package consumer hands the translator to the error reader", async () => {
  const panel = await read(PANEL);
  assert.ok(panel.includes("isMessageKey(message)"), "the panel must gate on a real key");
  assert.ok(
    panel.includes("extensions_pick_local_package"),
    "the consumer must still invoke the picker",
  );
  const region = between(
    panel,
    "const connectLocal = async () => {",
    "const reviewLocalManifest = async",
  );
  assert.ok(
    region.includes("errorMessage(nextError, t)"),
    "the picker's catch must translate its keyed failure, not paint the raw key",
  );
});

test("the R127 picker family is still complete", async () => {
  const i18n = await read(I18N);
  for (const context of R127_CONTEXTS) {
    assert.equal(
      count(i18n, `"${pickerKey(context)}":`),
      2,
      `${pickerKey(context)} must survive this round`,
    );
  }
  // The R127 anchors this round leans on remain non-hollow.
  const guard = await read(R127);
  assert.ok(guard.includes("pickerClosed."), "the R127 key anchor must remain");
  assert.ok(guard.includes("isMessageKey(message)"), "the R127 gate anchor must remain");
  assert.ok(
    guard.includes("errorMessage(nextError, t)"),
    "the R127 wiring anchor must remain",
  );
});

test("the run-error family's keyed() and its constants are still there", async () => {
  const source = await read(RUN_ERROR_RS);
  assert.ok(source.includes("fn keyed<"), "run_error.rs must keep the keyed() builder");
  assert.ok(source.includes("pub fn message_key("), "the mirror reader must remain");
  for (const name of [
    "RUN_SCRIPT_MISSING",
    "RUN_PROGRAM_MISSING",
    "RUN_PROGRAM_NOT_EXECUTABLE",
    "RUN_INTERPRETER_MISSING",
    "RUN_SPAWN_FAILED",
    "RUN_TIMEOUT",
    "RUN_KILLED",
    "RUN_INTEGRATION_DISABLED",
    "RUN_INTEGRATION_BROKEN",
    "RUN_TASK_FAILED",
  ]) {
    assert.ok(
      source.includes(`pub const ${name}: &str = "run_`),
      `run_error.rs must declare the ${name} constant`,
    );
  }
});

test("this guard assembles its literals, it does not spell them", async () => {
  const self = await read("tests/r135-async-and-keyed.test.ts");
  for (const literal of [
    BLOCKING,
    LOCK,
    PREFIX,
    pickerKey("permissionReview"),
    pickerKey("localPackage"),
    OLD_PERMISSION,
    OLD_LOCAL,
  ]) {
    assert.ok(!self.includes(literal), `the guard must not spell: ${literal}`);
  }
});
