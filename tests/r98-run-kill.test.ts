// R98 · a disable/uninstall kills an in-flight run, and the frontend says so.
//
// R92's survey left one item for the user to rule on: today a disable or
// uninstall does *not* stop a run that is already executing — the run keeps
// going until its 300 s timeout. The user's verdict was "kill it at once".
//
// The backend half (the abort registry, the two kill call sites, the keyed
// `run_killed` failure) is driven by the Rust suite. This file pins the
// frontend half: the new backend key is claimed by the one mapper that owns the
// wording, both dictionaries carry it, and the kill call sites really exist in
// the command/uninstall paths — a review that deleted one would otherwise leave
// a run alive with nothing to say about it.
//
// The mutation this file exists to catch: delete the `run_killed` branch from
// `run-errors.ts` and the run-killed contract test goes red.

import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

import {
  RUN_ERROR_MESSAGE_KEYS,
  parseRunError,
  runErrorMessage,
} from "../src/extensions/run-errors.ts";
import { createTranslator } from "../src/i18n.ts";

const root = new URL("../", import.meta.url);
const read = (path: string) => readFile(new URL(path, root), "utf8");
const stripJsComments = (source: string) =>
  source.replace(/\/\*[\s\S]*?\*\//g, "").replace(/^\s*\/\/.*$/gm, "");
const en = createTranslator("en");
const zh = createTranslator("zh");

const killed = 'run_killed:{"extension_id":"local.a1b2c3d4"}';

// ── 1 · the mapper turns the keyed payload into a sentence ─────────────────

test("R98 · the killed payload localises to a sentence that names the integration", () => {
  const message = runErrorMessage(killed, en);
  assert.ok(message, "the key must map to a message");
  assert.match(message, /local\.a1b2c3d4/);
  assert.match(message, /disabled or uninstalled/i);
  assert.ok(!message.includes("run_killed"), "the raw key must not survive");
});

test("R98 · the Chinese sentence is localised, not the English one", () => {
  const message = runErrorMessage(killed, zh);
  assert.ok(message);
  assert.match(message, /local\.a1b2c3d4/);
  assert.ok(!/disabled or uninstalled/i.test(message), "the English sentence leaked into zh");
});

test("R98 · a killed message that is not a keyed object is not swallowed", () => {
  // The caller keeps its ordinary toast — the same conservative split every
  // other run_error key relies on.
  for (const message of [
    "run_killed:not json",
    "run_killed",
    "run_killed:",
    "the run was stopped",
  ]) {
    assert.equal(runErrorMessage(message, en), null, `${message} must not map`);
  }
});

test("R98 · the killed key parses as a run error like every other key", () => {
  assert.deepEqual(parseRunError(killed), {
    key: "run_killed",
    payload: { extension_id: "local.a1b2c3d4" },
  });
});

// ── 2 · both dictionaries, same placeholder ────────────────────────────────

test("R98 · runErrorKilled is a symmetric en/zh pair", () => {
  const key = "settings.extensions.runErrorKilled";
  const placeholders = (value: string) =>
    [...value.matchAll(/\{(\w+)\}/g)].map((match) => match[1]).sort();
  assert.deepEqual(placeholders(en(key)), placeholders(zh(key)));
  assert.ok(en(key).length > 0 && zh(key).length > 0);
  assert.notEqual(en(key), zh(key), "the two dictionaries must not share one string");
  // It is part of the mapper's own key list, so the symmetry sweep covers it.
  assert.ok(RUN_ERROR_MESSAGE_KEYS.includes(key));
});

// ── 3 · the backend key set and the frontend mapper stay in step ───────────

test("R98 · every backend run_error key is claimed by the frontend mapper", async () => {
  // The single source of truth for the keys is the Rust module's `pub const`
  // table. Reading it here means a new backend key cannot reach the user as a
  // raw `run_…:{…}` string: either the mapper claims it, or this test fails.
  const rust = await read("src-tauri/src/extensions/run_error.rs");
  const keys = [...rust.matchAll(/pub const RUN_[A-Z_]+: &str = "([a-z_]+)";/g)].map(
    (match) => match[1],
  );
  // R128 grew the family from 7 to 10: the disabled/broken refusals and the
  // task-failure sentence are keyed the same way and must be claimed here too.
  assert.equal(keys.length, 10, `unexpected backend key set: ${keys.join(", ")}`);
  assert.ok(keys.includes("run_killed"), "the R98 key must exist in the backend");
  for (const key of keys) {
    assert.notEqual(parseRunError(`${key}:{}`), null, `${key} must parse on the frontend`);
  }
  // And the mapper claims exactly that set — no more, no fewer. The `known`
  // array and the `RunErrorKey` union both spell the keys, so the strings are
  // collected as a set.
  const frontend = stripJsComments(await read("src/extensions/run-errors.ts"));
  const claimed = new Set(
    [...frontend.matchAll(/"(run_[a-z_]+)"/g)].map((match) => match[1]),
  );
  assert.deepEqual([...claimed].sort(), [...keys].sort());
  // And each is a real dictionary key, so the mapper's list and the wording
  // table cannot drift apart.
  assert.equal(RUN_ERROR_MESSAGE_KEYS.length, keys.length);
});

// ── 4 · the kill call sites really exist ───────────────────────────────────

test("R98 · disabling an integration kills its runs in the shared handler", async () => {
  const commands = stripJsComments(await read("src-tauri/src/commands/extensions.rs"));
  // The disable branch, beside the completion cancel it already had.
  assert.match(
    commands,
    /if !enabled \{[\s\S]{0,300}state\.provider\.cancel_completions\(\);[\s\S]{0,300}state\.kill_extension_runs\(id\);/,
    "the disable branch must abort in-flight runs",
  );
});

test("R98 · uninstalling kills the runs before any file is touched", async () => {
  const uninstall = stripJsComments(await read("src-tauri/src/extensions/uninstall.rs"));
  const kill = uninstall.indexOf("state.kill_extension_runs(&request.extension_id);");
  assert.ok(kill >= 0, "the componentized uninstall must abort in-flight runs");
  // The kill precedes the first journal/staging step, so a removed program is
  // never still executing.
  const staging = uninstall.indexOf("write_removal_journal");
  assert.ok(staging > kill, "the kill must come before the removal is staged");
});

test("R98 · the run spawn is registered for abort, and the abort reports a key", async () => {
  const run = stripJsComments(await read("src-tauri/src/extensions/run.rs"));
  // The spawn's abort handle is registered through the state…
  assert.match(run, /state\.register_run_abort\(extension_id, task\.abort_handle\(\)\)/);
  // …and a cancelled task maps to the keyed failure the mapper localises.
  assert.match(run, /error\.is_cancelled\(\)\s*=>\s*Err\(run_error::killed\(extension_id\)\)/);
  // The registry's guard removes the handle on every exit path.
  assert.match(run, /impl Drop for RunAbortGuard/);
  assert.match(run, /handles\.retain\(\|handle\| handle\.id\(\) != self\.task_id\)/);
});
