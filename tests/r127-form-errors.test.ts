// R127 · form errors speak the session's language, and bare failures name their cause.
//
// Three closures, one guard.
//
// Task 1 — the five picker-closed sentences in
// `commands/extensions.rs` (extension export/import, local manifest, script
// export, configuration export) were hardcoded English, and four of them are
// painted straight into the panel's error line. They now arrive as dictionary
// keys (the `pickerClosed` family under `settings.extensions.`) and the panel
// translates them, the same contract `floter://` refusals already use. This
// suite slices each of the five command bodies and pins the keyed form while
// forbidding the old English sentence from coming back.
//
// Task 2 — six bare-word failures (a verb with no object, no OS error) gained
// their cause: the two scripted-terminal mocks, the two empty launcher inputs,
// the missing application path and the poisoned settings lock. This suite pins
// each rewritten line by file:line and proves the old literal is gone.
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
const TERMINAL = "src-tauri/src/extensions/" + "terminal_capability.rs";
const ACTIONS = "src-tauri/src/commands/" + "actions.rs";
const APPS = "src-tauri/src/commands/apps/" + "mod.rs";
const CONFIG = "src-tauri/src/commands/" + "config.rs";
const I18N = "src/" + "i18n.ts";
const PANEL = "src/" + "ExtensionsPanel.tsx";
const R126 = "tests/r126-dialog-and-" + "config.test.ts";

// Assembled so this guard does not spell the strings it scans for.
const PREFIX = "settings.extensions." + "pickerClosed.";
const CLOSED_EN = "picker closed " + "unexpectedly";
const pickerKey = (context: string) => PREFIX + context;
const signature = (name: string) => "pub async fn " + name + "(";
const bare = (text: string) => '"' + text + '".to_string()';

/** The P1-D messages, assembled from fragments for the self-proof below. */
const N_WRITE = "write failed: " + "{bytes} bytes rejected";
const N_READ = "read failed: " + "{bytes} bytes unavailable";
const N_COMMAND = "Empty command: " + "{command:?}";
const N_PROGRAM = "Empty program: " + "{program:?}";
const N_APPLICATION = "Application not found: " + "{path}";
const N_LOCK = "Settings lock is poisoned: " + "{error}";

/** The slice of a Rust file between two signatures (end marker exclusive). */
const between = (source: string, start: string, end: string): string => {
  const from = source.indexOf(start);
  assert.ok(from >= 0, `start signature not found: ${start}`);
  const to = source.indexOf(end, from + start.length);
  assert.ok(to > from, `end signature not found after start: ${end}`);
  return source.slice(from, to);
};

/** The five picker commands: [key context, own signature, next signature]. */
const PICKER_REGIONS: Array<[string, string, string]> = [
  ["export", signature("extensions_export"), signature("extensions_import")],
  ["import", signature("extensions_import"), signature("extensions_install")],
  [
    "localManifest",
    signature("extensions_pick_local_manifest"),
    signature("extensions_pick_local_package"),
  ],
  [
    "scriptExport",
    signature("extensions_custom_export_script"),
    signature("extensions_script_runtime_check"),
  ],
  [
    "configurationExport",
    signature("extensions_config_export"),
    "fn reject_bundled_static_configuration(",
  ],
];

/**
 * The six P1-D rewrites, pinned by file, 1-based line and expected content.
 *
 * R136 added the process-wide settings snapshot above `load_settings` in
 * config.rs, so the CONFIG pin moved from 1194 to 1296. The other five files
 * are untouched by that round and keep their R127 lines.
 */
const P1D_SITES: Array<[string, number, string, string]> = [
  [TERMINAL, 476, N_WRITE, "write failed"],
  [TERMINAL, 490, N_READ, "read failed"],
  [ACTIONS, 139, N_COMMAND, "Empty command"],
  [ACTIONS, 173, N_PROGRAM, "Empty program"],
  [APPS, 408, N_APPLICATION, "Application not found"],
  [CONFIG, 1322, N_LOCK, "Settings lock is poisoned"],
];

test("the five picker messages are keyed, not spelled in English", async () => {
  const source = await read(EXTENSIONS);
  for (const [context, start, end] of PICKER_REGIONS) {
    const region = between(source, start, end);
    assert.ok(
      region.includes(pickerKey(context)),
      `the ${context} picker must carry its dictionary key`,
    );
    assert.ok(
      !region.includes(CLOSED_EN),
      `the ${context} picker must not spell the English sentence any more`,
    );
    // Non-vacuous: the region really is the picker's error channel.
    assert.ok(region.includes(".map_err(|_|"), `the ${context} picker must map its channel`);
  }
});

test("every P1-D failure names its cause at the pinned line", async () => {
  const sources = new Map<string, string>();
  for (const [path] of P1D_SITES) {
    if (!sources.has(path)) sources.set(path, await read(path));
  }
  for (const [path, line, needle, old] of P1D_SITES) {
    const source = sources.get(path)!;
    assert.ok(
      source.split("\n")[line - 1].includes(needle),
      `${path}:${line} must read \`${needle}\``,
    );
    // The interpolation is load-bearing, not decoration.
    assert.ok(needle.includes("{") && needle.includes("}"), `${needle} must interpolate`);
    // The bare word it replaces is gone.
    assert.equal(count(source, bare(old)), 0, `${path} must no longer return \`${old}\` bare`);
  }
});

test("the two OS-rooted failures splice {path} and {error}, not just a verb", async () => {
  const apps = await read(APPS);
  const config = await read(CONFIG);
  assert.ok(apps.includes(N_APPLICATION), "the missing application must name its path");
  assert.ok(config.includes(N_LOCK), "the poisoned settings lock must name its error");
});

test("the picker-closed keys exist on both sides of the dictionary", async () => {
  const i18n = await read(I18N);
  for (const [context] of PICKER_REGIONS) {
    const key = pickerKey(context);
    assert.equal(count(i18n, `"${key}":`), 2, `${key} must have an English and a Chinese entry`);
  }
  // The English values are the sentences that used to be hardcoded, and the two
  // languages never collapse to one string.
  const exportEntries = i18n
    .split("\n")
    .filter((line) => line.includes(`"${pickerKey("export")}":`));
  assert.equal(exportEntries.length, 2, "the export key must carry a value on both sides");
  assert.notEqual(exportEntries[0], exportEntries[1], "the two languages must not share one string");
  assert.ok(
    exportEntries.every((line) => line.length > pickerKey("export").length + 4),
    "neither entry may be blank",
  );
});

test("the panel translates the keyed family instead of painting the key", async () => {
  const panel = await read(PANEL);
  assert.ok(panel.includes("isMessageKey(message)"), "the panel must gate on a real key");
  assert.ok(panel.includes("t(message)"), "the panel must translate the key");
  // Every display point hands the translator to the error reader. R127 wired
  // four; R128 added the custom-integration / local-connection form paths
  // (L1-L12), so the count is re-registered here rather than left stale. R135
  // keyed the connect-package picker (`pickerClosed.localPackage`) whose sole
  // consumer is `connectLocal`, so the count moved to nine.
  assert.equal(
    count(panel, "errorMessage(nextError, t)"),
    9,
    "each wired display point must pass the translator",
  );
});

test("the R126 anchors this round builds on are still there", async () => {
  const guard = await read(R126);
  assert.ok(
    guard.includes("extensions_pick_") && guard.includes("local_package("),
    "the R126 picker anchor must remain",
  );
  assert.ok(guard.includes("settings.") && guard.includes("pluginConfigLoadFailed"), "the R126 key must remain");
  assert.ok(guard.includes("loadFailed"), "the R126 state anchor must remain");
});

test("this guard assembles its literals, it does not spell them", async () => {
  const self = await read("tests/r127-form-errors.test.ts");
  for (const literal of [
    PREFIX,
    CLOSED_EN,
    pickerKey("export"),
    pickerKey("configurationExport"),
    N_WRITE,
    N_READ,
    N_COMMAND,
    N_PROGRAM,
    N_APPLICATION,
    N_LOCK,
  ]) {
    assert.ok(!self.includes(literal), `the guard must not spell: ${literal}`);
  }
});
