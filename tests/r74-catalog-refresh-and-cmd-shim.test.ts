// R74 · three small repairs, pinned where each can actually go red.
//
//   1. the catalog memo's *second* invalidation point. The install row's door
//      into the terminal page (`openTerminalSession`) is the one launch branch
//      that does not hide the window, so the native `floter://revealed` event
//      never fires on the way back and a tool installed in that session stayed
//      on offer as an install row. The explicit return to the launcher now asks
//      for one refresh — same store, same memo, one more moment.
//   2. the Windows `.cmd` / `.bat` shim. `CreateProcess` cannot start a batch
//      file, so the one launch entry point re-points such a program at
//      `cmd /c`. The rule is a pure, `cfg`-free function with a truth table in
//      Rust (it runs on Linux); only the call site is Windows-only, and that
//      call site is pinned here by source, since it cannot be compiled here.
//   3. the deleted `clear_history_copy_cache` test helper.
//
// Mutations that must turn this file (or the Rust truth table beside it) red:
//   * deleting the `refreshToolCatalog()` call from `returnToInputMode` -> the
//     memo test fails;
//   * making `refreshToolCatalog` always start a new read -> the idempotency
//     test fails;
//   * inverting the suffix rule in `windows_cmd_shim_wrap` -> the Rust truth
//     table fails (the source pin only guards the wiring);
//   * dropping the `#[cfg(windows)]` call site -> the source-pin fails.

import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

import { refreshToolCatalog, toolCatalogSnapshot } from "../src/extensions/tool-catalog-store.ts";

const root = new URL("../", import.meta.url);
const read = (path: string) => readFile(new URL(path, root), "utf8");
const stripJsComments = (source: string) =>
  source.replace(/\/\*[\s\S]*?\*\//g, "").replace(/\/\/[^\n]*/g, "");
/** Strip Rust comments so a prose mention cannot satisfy a code check. */
const stripRustComments = (source: string) =>
  source.replace(/\/\*[\s\S]*?\*\//g, "").replace(/^\s*\/\/.*$/gm, "");

// ── 1 · the second invalidation point ─────────────────────────────────────

test("returning to the launcher from the terminal page refreshes the tool catalog", async () => {
  const app = stripJsComments(await read("src/App.tsx"));
  const start = app.indexOf("const returnToInputMode = async () => {");
  const end = app.indexOf("const openSettings = (page?: SettingsPage) => {", start);
  assert.ok(start > -1 && end > start, "the explicit return path must exist");
  assert.match(
    app.slice(start, end),
    /refreshToolCatalog\(\)/,
    "the return asks for one catalog refresh",
  );
});

test("the memo and the reveal listener are still the mechanism", async () => {
  const store = stripJsComments(await read("src/extensions/tool-catalog-store.ts"));
  assert.match(store, /let report: ToolCatalogReport \| null = null/);
  assert.match(store, /export const refreshToolCatalog/);
  assert.ok(!/setInterval|setTimeout/.test(store), "no polling, no timer");
  const app = await read("src/App.tsx");
  const reveal = app.indexOf('listen<string>("floter://revealed"');
  assert.notEqual(reveal, -1, "the reveal listener exists");
  assert.ok(
    app.indexOf("refreshToolCatalog()", reveal) > reveal,
    "and still refreshes the catalog once per reveal",
  );
});

test("the catalog refresh is idempotent and a failed read is a soft landing", async () => {
  const first = refreshToolCatalog();
  const second = refreshToolCatalog();
  assert.equal(first, second, "a refresh already in flight is shared, not duplicated");
  await first;
  assert.equal(
    toolCatalogSnapshot(),
    null,
    "a read with no host lands as null, never as a throw",
  );
  const third = refreshToolCatalog();
  assert.notEqual(third, first, "once settled, the next refresh starts a new read");
  await third;
});

// ── 2 · the Windows shim, pinned by source ────────────────────────────────

test("the Windows launch branch re-points a .cmd/.bat program at the interpreter", async () => {
  const rust = stripRustComments(await read("src-tauri/src/process_launch.rs"));
  const rule = rust.indexOf("pub(crate) fn windows_cmd_shim_wrap");
  assert.notEqual(rule, -1, "the pure wrapper must exist");
  // The rule itself carries no `cfg`: the truth table beside it runs everywhere.
  assert.ok(
    rust.slice(0, rule).trimEnd().endsWith("#[cfg_attr(not(windows), allow(dead_code))]"),
    "the rule is compiled on every platform; only its call site is Windows-only",
  );
  assert.match(
    rust,
    /fn only_cmd_and_bat_programs_are_wrapped_for_the_command_interpreter/,
    "the truth table must stay beside the rule",
  );
  const spawn = rust.indexOf("pub(crate) fn spawn_application");
  assert.notEqual(spawn, -1, "the one launch entry point must exist");
  const body = rust.slice(spawn, spawn + 900);
  assert.match(body, /#\[cfg\(windows\)\]/, "the Windows branch must be cfg-gated");
  assert.match(
    body,
    /match windows_cmd_shim_wrap\(program, args\)/,
    "the Windows branch consults the wrapper",
  );
  assert.match(
    body,
    /Some\(\(shell, wrapped\)\)\s*=>\s*spawn_detached\(&shell, &wrapped\)/,
    "and spawns `cmd /c <shim> <args…>` instead of the shim itself",
  );
});

// ── 3 · the dead helper ───────────────────────────────────────────────────

test("the unused history cache reset is gone", async () => {
  const history = await read("src-tauri/src/browser_data/history.rs");
  assert.ok(
    !history.includes("clear_history_copy_cache"),
    "the never-called test helper is deleted, not merely unused",
  );
});
