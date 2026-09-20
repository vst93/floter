// R9-5 · the readable half of a failed script run.
//
// The device report was "the script run also errors out" with no error text to
// go on: the backend refused with prose the user could not act on (or, on the
// terminal route, with the launcher's generic sentence), and the message never
// named *what* was missing or *where* the host looked.
//
// The backend now answers with a stable key plus a JSON payload
// (`run_interpreter_missing:{"language":"js","names":["node"],"searched":[…]}`).
// This module is the only place the wording lives, so both dictionaries stay
// symmetric and no key can reach the user as a raw `run_…:` string.
//
// The pure projection is driven directly; the JSX facts are read off the
// source, because the panel is a component the node runner does not render.

import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";
import { createTranslator } from "../src/i18n.ts";
import {
  RUN_ERROR_MESSAGE_KEYS,
  parseRunError,
  runErrorMessage,
} from "../src/extensions/run-errors.ts";

const root = new URL("../", import.meta.url);
const read = (path: string) => readFile(new URL(path, root), "utf8");
const en = createTranslator("en");
const zh = createTranslator("zh");

// ── 1 · the key/payload split ──────────────────────────────────────────────

test("a keyed backend message splits into its key and payload", () => {
  const parsed = parseRunError('run_program_missing:{"path":"/opt/homebrew/bin/node"}');
  assert.deepEqual(parsed, {
    key: "run_program_missing",
    payload: { path: "/opt/homebrew/bin/node" },
  });
});

test("the split is conservative so other error paths keep their own handling", () => {
  // Mutation: loosen the guard to "any `key:rest`" and these become run errors,
  // stealing the parameter form's inline message and the in-flight warning.
  for (const message of [
    "run_param_required:note",
    "run_already_in_flight:local.a",
    "Script toolchain is not available: ruby not found on PATH",
    "run_program_missing:not json",
    "run_unknown_key:{}",
    "",
  ]) {
    assert.equal(parseRunError(message), null, `${message} must not parse as a run error`);
  }
});

// ── 2 · every key maps to a message that names the fact ────────────────────

test("a missing script names the file it could not find", () => {
  const message = runErrorMessage(
    'run_script_missing:{"path":"/Users/me/.config/floter/extension-data/local.ab/integration/provider.sh"}',
    en,
  );
  assert.ok(message, "the key must map to a message");
  assert.match(message, /provider\.sh/);
  assert.match(message, /missing/i);
});

test("a missing program names the path that does not exist", () => {
  const message = runErrorMessage('run_program_missing:{"path":"/nope/tool"}', en);
  assert.ok(message);
  assert.match(message, /\/nope\/tool/);
});

test("a non-executable program names the path and says it is a permission problem", () => {
  const message = runErrorMessage('run_program_not_executable:{"path":"/nope/tool"}', en);
  assert.ok(message);
  assert.match(message, /\/nope\/tool/);
  assert.match(message, /executable/i);
});

test("a missing interpreter names the language, the binaries and the directories", () => {
  // The device-report scenario: the toolchain is installed somewhere the app
  // never looked. "Not found" alone is not actionable; the directories are.
  const message = runErrorMessage(
    'run_interpreter_missing:{"language":"js","names":["node"],"searched":["/usr/bin","/opt/homebrew/bin"]}',
    en,
  );
  assert.ok(message);
  assert.match(message, /js/);
  assert.match(message, /node/);
  assert.match(message, /\/opt\/homebrew\/bin/);
  assert.match(message, /\/usr\/bin/);
});

test("a spawn refusal keeps the OS's own words for anything unnamed", () => {
  const message = runErrorMessage(
    'run_spawn_failed:{"path":"/nope/tool","detail":"Exec format error (os error 8)"}',
    en,
  );
  assert.ok(message);
  assert.match(message, /\/nope\/tool/);
  assert.match(message, /Exec format error/);
});

test("a timeout states the budget it exceeded", () => {
  const message = runErrorMessage('run_timeout:{"seconds":300}', en);
  assert.ok(message);
  assert.match(message, /300/);
});

test("an unrecognised or malformed message is not swallowed", () => {
  // The caller keeps its ordinary toast — strictly better than losing the
  // failure behind a generic sentence.
  assert.equal(runErrorMessage("something else went wrong", en), null);
  assert.equal(runErrorMessage("run_program_missing:not json", en), null);
});

// ── 3 · both languages, same placeholders ──────────────────────────────────

test("every run-error key is translated in both dictionaries", () => {
  assert.equal(RUN_ERROR_MESSAGE_KEYS.length, 6);
  for (const key of RUN_ERROR_MESSAGE_KEYS) {
    const english = en(key);
    const chinese = zh(key);
    assert.ok(english.length > 0, `${key} has an English string`);
    assert.ok(chinese.length > 0, `${key} has a Chinese string`);
    assert.notEqual(english, chinese, `${key} must actually be translated`);
    // The placeholders have to match or one language interpolates a name the
    // other never declares.
    const placeholders = (value: string) =>
      [...value.matchAll(/\{(\w+)\}/g)].map((match) => match[1]).sort();
    assert.deepEqual(
      placeholders(english),
      placeholders(chinese),
      `${key} placeholder sets differ`,
    );
  }
});

test("the Chinese message for a missing interpreter is localized, not English", () => {
  const message = runErrorMessage(
    'run_interpreter_missing:{"language":"python","names":["python3"],"searched":["/usr/bin"]}',
    zh,
  );
  assert.ok(message);
  assert.match(message, /python/);
  assert.ok(!/Looked for/.test(message), "the English sentence leaked into zh");
});

// ── 4 · the panel wires the mapping into the run failure path ──────────────

test("the panel translates a keyed run failure instead of showing it raw", async () => {
  const panel = (await read("src/ExtensionsPanel.tsx"))
    .replace(/\/\*[\s\S]*?\*\//g, "")
    .replace(/^\s*\/\/.*$/gm, "");
  // The mapping is imported…
  assert.match(panel, /import \{ runErrorMessage \} from "\.\/extensions\/run-errors"/);
  // …and consulted in the run catch block, before the raw message fallback.
  assert.match(panel, /const runError = runErrorMessage\(message, t\)/);
  assert.match(panel, /if \(runError\) \{[\s\S]{0,200}showError\(runError\)/);
  // The parameter refusal keeps its inline home: it is checked first and
  // returns, so a `run_param_*` message can never become a toast.
  const paramIndex = panel.indexOf("if (mapped) {");
  const runIndex = panel.indexOf("const runError = runErrorMessage");
  assert.ok(paramIndex >= 0 && runIndex > paramIndex, "param refusal must be checked first");
});
