// R141 · the export family's join failure is keyed, and nineteen dead keys stay
// deleted.
//
// Task 1 — R135 moved the three export commands' blocking writes off the async
// worker, but the join failure was spelled in English at three anchors and
// reached the toast through `errorMessage`, which passes a non-key through
// verbatim. The backend now sends a dictionary key, a colon and the runtime's
// own error; the panel's KEYED_VALUE_PARAMS table fills the dictionary's
// `{error}` slot — the same keyed-value shape R128 gave the form family and the
// open-URL refusal. No consumer changed: the table is data the existing
// `:`-split branch reads.
//
// Task 2 — the nineteen keys R140 proved dead are gone from both dictionaries.
// This guard pins the set so a rename cannot silently resurrect one.
//
// Every scanned literal is assembled from fragments, and the last test proves
// this guard does not spell the strings it looks for.
import assert from "node:assert/strict";
import { readdir, readFile } from "node:fs/promises";
import test from "node:test";

const root = new URL("../", import.meta.url);
const read = (path: string) => readFile(new URL(path, root), "utf8");
const count = (haystack: string, needle: string) => haystack.split(needle).length - 1;

const EXTENSIONS = "src-tauri/src/commands/" + "extensions.rs";
const I18N = "src/" + "i18n.ts";
const PANEL = "src/" + "ExtensionsPanel.tsx";

/** Remove JS/TS/Rust comments so an anchor can only be satisfied by real code. */
const stripComments = (source: string) =>
  source.replace(/\/\*[\s\S]*?\*\//g, "").replace(/^\s*\/\/.*$/gm, "");

// Assembled so this guard does not spell the strings it scans for.
const KEYED_PREFIX = "settings.extensions." + "exportTaskFailed.";
const exportKey = (what: string) => KEYED_PREFIX + what;
const WHATS = ["extension", "script", "config"];
const ERROR_SLOT = ":" + "{error}";
const OLD_ENGLISH = ["Extension export ", "Script export ", "Configuration export "].map(
  (head) => head + "task failed: ",
);

// The nineteen keys R140 proved dead. Built from a family prefix plus a stem so
// the guard itself never spells a full key.
const SETTINGS = "settings.";
const CLIPBOARD = "clipboard.";
const LAUNCHER = "launcher.";
const DEAD_KEYS = [
  "extensions." + "source",
  LAUNCHER + "browserTabsUnavailable",
  SETTINGS + "group.link",
  SETTINGS + "language.en",
  SETTINGS + "language.zh",
  SETTINGS + "browserTarget",
  SETTINGS + "browserTargetAuto",
  SETTINGS + "browserCustomDir",
  SETTINGS + "browserHistoryDays",
  SETTINGS + "browserSort",
  SETTINGS + "browserSortHint",
  CLIPBOARD + "title",
  CLIPBOARD + "filter",
  CLIPBOARD + "clear",
  CLIPBOARD + "loadFailed",
  CLIPBOARD + "typeColor",
  SETTINGS + "shortcuts",
  SETTINGS + "extensions.customRunning",
  SETTINGS + "extensions.customRunRememberHint",
];

const QUOTES = ['"', "'", "`"];
const quoted = (key: string) => QUOTES.map((quote) => quote + key + quote);

/** Every `.ts` / `.tsx` / `.rs` file under a directory, recursively. */
const sourcesUnder = async (dir: string): Promise<string[]> => {
  const entries = await readdir(new URL(dir + "/", root), { withFileTypes: true });
  const files: string[] = [];
  for (const entry of entries) {
    const child = dir + "/" + entry.name;
    if (entry.isDirectory()) files.push(...(await sourcesUnder(child)));
    else if (/\.(ts|tsx|rs)$/.test(entry.name)) files.push(child);
  }
  return files;
};

test("the three export task failures are keyed at the source", async () => {
  const source = await read(EXTENSIONS);
  assert.equal(count(source, KEYED_PREFIX), 3, "the export family must key all three joins");
  for (const what of WHATS) {
    const key = exportKey(what);
    assert.equal(
      count(source, key + ERROR_SLOT),
      1,
      `${key} must carry the runtime's error after ':'`,
    );
  }
  for (const sentence of OLD_ENGLISH) {
    assert.equal(count(source, sentence), 0, `the raw English sentence must be gone: ${sentence}`);
  }
});

test("the panel's keyed-value table and both dictionaries carry the three keys", async () => {
  const panel = await read(PANEL);
  const i18n = await read(I18N);
  for (const what of WHATS) {
    const key = exportKey(what);
    const line = panel.split("\n").find((candidate) => candidate.includes(`"${key}"`));
    assert.ok(line, `${key} must be listed in KEYED_VALUE_PARAMS`);
    assert.ok(line.includes(`"error"`), `${key} must name the error placeholder`);

    assert.equal(count(i18n, `"${key}":`), 2, `${key} must have an English and a Chinese entry`);
    const entries = i18n.split("\n").filter((candidate) => candidate.includes(`"${key}":`));
    assert.equal(entries.length, 2, `${key} must carry a value on both sides`);
    assert.ok(entries.every((entry) => entry.includes("{error}")), `${key} must interpolate {error}`);
    assert.notEqual(entries[0], entries[1], `${key} languages must not share one string`);
  }
});

test("the nineteen dead keys stay out of every frontend and backend source", async () => {
  const files = [...(await sourcesUnder("src")), ...(await sourcesUnder("src-tauri/src"))];
  assert.ok(files.length > 100, `the scan must cover the tree (${files.length} files)`);
  for (const file of files) {
    const text = stripComments(await read(file));
    for (const key of DEAD_KEYS) {
      for (const literal of quoted(key)) {
        assert.equal(count(text, literal), 0, `${key} must not be resurrected in ${file}`);
      }
    }
  }
});

test("this guard assembles its literals, it does not spell them", async () => {
  const self = await read("tests/r141-export-task-failed-keyed.test.ts");
  for (const literal of [KEYED_PREFIX, ...WHATS.map(exportKey), ...OLD_ENGLISH, ...DEAD_KEYS]) {
    assert.ok(!self.includes(literal), `the guard must not spell: ${literal}`);
  }
});
