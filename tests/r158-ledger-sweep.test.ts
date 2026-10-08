// R158 · the ledger sweep.
//
// Two entries the R153 report registered and this round closes, plus the
// comment debt R157 left behind:
//
//   1. R153-R1 — `extensions_run_output` was deleted but its write path stayed.
//      `remember_run_output` stored into a session map no production code ever
//      read: an orphan write. R158 removes the whole store (the AppState field,
//      the write call, the store and its two tests). The guard scans every Rust
//      source for a survivor.
//   2. R153-R2 — R153 rendered the description input unconditionally, but
//      `create_custom_integration_locked` hardcodes the script manifest's
//      description and never reads `request.description`. In script mode the
//      input was editable, silently dropped and prefilled with the internal
//      string. R158 gives it the rule R153 gave `versionArgs`: executable-only.
//   3. R157 §8.2-1 — base.css still said DWM rounding was off. R157 turns it on
//      (`DWMWCP_ROUND`), so the two comments are corrected while every
//      declaration is left byte-identical (the bundled CSS hash must not move).
//
// All three are source-scan: the gate has no Windows runtime, and the orphan is
// a call-site fact rather than a behaviour.
import assert from "node:assert/strict";
import { readFileSync, readdirSync } from "node:fs";
import { readFile } from "node:fs/promises";
import test from "node:test";

const root = new URL("../", import.meta.url);
const read = (path: string) => readFile(new URL(path, root), "utf8");
const readSync = (path: string) => readFileSync(new URL(path, root), "utf8");

/** Every `.rs` file under `src-tauri/src`, relative to the repo root. */
const rustSources = (): string[] =>
  (readdirSync(new URL("src-tauri/src", root), { recursive: true }) as string[])
    .filter((name) => name.endsWith(".rs"))
    .map((name) => `src-tauri/src/${name}`);

// ── 1 · the orphan run-output store is gone ───────────────────────────────

test("the orphan run-output store is gone from every Rust source", () => {
  const sources = rustSources();
  assert.ok(sources.length > 20, `the scan must see the tree (saw ${sources.length})`);
  const forbidden = [
    "remember_run_output",
    "RunOutputStore",
    "run_outputs",
    "MAX_RUN_OUTPUT_BYTES",
    "truncate_stream",
  ];
  for (const path of sources) {
    const source = readSync(path);
    for (const needle of forbidden) {
      assert.ok(
        !source.includes(needle),
        `${path} must not carry \`${needle}\`: R158 removed the orphan run-output store`,
      );
    }
    // The method, not the `RunOutput` wire type the IPC result still needs.
    assert.ok(
      !/fn run_output\s*\(/.test(source),
      `${path} must not define a \`run_output\` method`,
    );
  }
  // The AppState itself carries no such field: the write had nowhere to go.
  const state = readSync("src-tauri/src/extensions/mod.rs");
  assert.ok(
    !/run_outputs\s*:/.test(state),
    "ExtensionState must carry no `run_outputs` field",
  );
});

// ── 2 · the description control is executable-only ────────────────────────

test("the description control renders only for the executable runtime", async () => {
  const drawer = await read("src/extensions/CustomIntegrationDrawer.tsx");
  assert.match(
    drawer,
    /integration\.mode === "executable" && <label><span>\{t\("settings\.extensions\.customDescription"\)\}/,
    "the drawer must gate the description input on executable mode",
  );
  // Exactly one site: the gated one. A second, unconditional copy would revive
  // the dead control the guard exists to catch.
  assert.equal(
    (drawer.match(/customDescription/g) ?? []).length,
    1,
    "the description label must have exactly one site",
  );
});

test("the backend reads a description back for executable mode only", async () => {
  const install = await read("src-tauri/src/extensions/install.rs");
  assert.match(
    install,
    /description: \(mode == "executable"\)\.then_some\(manifest\.description\)/,
    "the read-back must project the manifest description for executable mode only",
  );
  assert.ok(
    !install.includes("description: Some(manifest.description)"),
    "the unconditional read-back must be gone",
  );
  // Script mode never reads `request.description`: the manifest gets its own
  // fixed sentence, which is why the drawer hides the control there.
  assert.match(
    install,
    /description: if script_mode \{\s*"Local script integration"\.to_string\(\)/,
    "script mode must hardcode its own description",
  );
});

// ── 3 · base.css describes R157's corner, and keeps both radii ────────────

test("base.css comments no longer claim DWM rounding is off", async () => {
  const css = await read("src/styles/base.css");
  for (const stale of ["rounding is disabled", "DWM rounding is off"]) {
    assert.ok(
      !css.includes(stale),
      `base.css must not still say "${stale}" — R157 turns the DWM corner on`,
    );
  }
  assert.match(css, /DWMWCP_ROUND/, "the comment must name the R157 preset");
  // The declarations are untouched: both radii are the same numbers as before.
  assert.match(css, /--window-radius: 14px;/, "the default radius stays 14px");
  assert.match(css, /--window-radius: 8px;/, "the Windows radius stays 8px");
});
