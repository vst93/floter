// R8-2 · the zero-form connect path.
//
// The research report's P7: `extensions_connect_tool` existed, was fully
// implemented and fully tested on the Rust side, and had **no frontend caller**
// — the Detected row still opened the create-custom drawer and made the user
// confirm `name` / `id` / `command` / `version`, all four of which the backend
// already derives. P2 compounded it: with no ranking signal, the twelve rows on
// offer were the alphabetically first twelve PATH executables.
//
// These tests pin the frontend half of the fix. Each is written against the
// positive fact (the call that must happen), so deleting the wiring fails, and
// each names the mutation that turns it red.

import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";
import { createTranslator } from "../src/i18n.ts";

const root = new URL("../", import.meta.url);
const read = (path: string) => readFile(new URL(path, root), "utf8");
const stripJsComments = (source: string) =>
  source.replace(/\/\*[\s\S]*?\*\//g, "").replace(/^\s*\/\/.*$/gm, "");

/** One declaration body by brace matching (same helper the three-zone suite
 *  uses), so a slice can never be silently shortened by a later deletion. */
const functionBody = (source: string, signature: string) => {
  const at = source.indexOf(signature);
  assert.notEqual(at, -1, `missing declaration: ${signature}`);
  const open = source.indexOf("{", at);
  assert.notEqual(open, -1, `declaration without a body: ${signature}`);
  let depth = 0;
  for (let i = open; i < source.length; i += 1) {
    if (source[i] === "{") depth += 1;
    else if (source[i] === "}") {
      depth -= 1;
      if (depth === 0) return source.slice(open + 1, i);
    }
  }
  assert.fail(`unterminated declaration: ${signature}`);
};

// ── 1. One click, no form ──────────────────────────────────────────────────

// A bare PATH discovery must reach the backend's one-click command with the
// row's own candidate. The candidate matters: the backend's ranking, its
// version probe and its id allocation all read fields (`sources`, `quality`,
// `fingerprint`) that the old frontend reconstruction dropped.
//
// Mutation: call `openCreateCustomIntegration()` + `chooseToolCandidate({...})`
// again (the R7-3a path) and this fails.
test("a detected row connects through extensions_connect_tool with the row's own candidate", async () => {
  const panel = stripJsComments(await read("src/ExtensionsPanel.tsx"));
  const entry = functionBody(panel, "const connectDetected = async (extension: Extension)");

  assert.match(
    entry,
    /invoke\("extensions_connect_tool", \{ candidate \}\)/,
    "the one-click command must be invoked",
  );
  assert.match(
    entry,
    /extension\.toolCandidates/,
    "the candidate must come from the row, not be rebuilt here",
  );
  assert.ok(
    !/openCreateCustomIntegration/.test(entry),
    "the Detected path must not open the create form",
  );
  assert.ok(
    !/chooseToolCandidate/.test(entry),
    "the Detected path must not re-derive id/command/version in the frontend",
  );
});

// The permission set is the backend's, and only the backend's. Omitting
// `approvedPermissions` is what makes `tool_binding_permissions()` the single
// source of the disclosure; a literal list here would be a second one.
//
// Mutation: pass `approvedPermissions: ["environment", "process-spawn", ...]`
// and this fails.
test("the one-click connect does not hard-code a permission set", async () => {
  const panel = stripJsComments(await read("src/ExtensionsPanel.tsx"));
  const entry = functionBody(panel, "const connectDetected = async (extension: Extension)");
  assert.ok(
    !/approvedPermissions/.test(entry),
    "the connect call must omit approvedPermissions (backend default disclosure set)",
  );
  assert.ok(
    !/filesystem-write|network-fetch|clipboard/.test(entry),
    "no write/network/clipboard permission may appear on the automatic path",
  );
});

// Success must refresh through the existing chain, not patch local state.
test("a successful connect refreshes the list and announces it", async () => {
  const panel = stripJsComments(await read("src/ExtensionsPanel.tsx"));
  const entry = functionBody(panel, "const connectDetected = async (extension: Extension)");
  assert.match(entry, /await refreshAfterMutation\(\)/, "the list must be re-read after the write");
  assert.match(
    entry,
    /connectedNotice/,
    "the same success notice as every other connect path must be shown",
  );
});

// ── 2. Failure has a visible, in-place home ────────────────────────────────

// A detected row has no drawer, so a failed connect must leave a reason next to
// the row. The toast alone disappears.
//
// Mutation: drop the `setDetectedError` call (or the notice element below) and
// this fails.
test("a failed connect records an inline error and renders it in the section", async () => {
  const panel = stripJsComments(await read("src/ExtensionsPanel.tsx"));
  const entry = functionBody(panel, "const connectDetected = async (extension: Extension)");
  assert.match(entry, /setDetectedError\(\{ id: extension\.id, message: errorMessage\(nextError\) \}\)/);

  const sectionAt = panel.indexOf("extensions-section--detected");
  assert.notEqual(sectionAt, -1, "the Detected section must exist");
  const section = panel.slice(sectionAt, panel.indexOf("</section>", sectionAt));
  assert.match(section, /\{detectedError && \(/, "the error must be rendered in the Detected section");
  assert.match(section, /role="alert"/, "an inline error must be announced");
  assert.match(section, /extensions-notice--error/, "the inline error reuses the existing notice style");
  assert.match(section, /detectedConnectFailed/, "the inline error must name the tool that failed");
});

// ── 3. The disclosure is permanent ─────────────────────────────────────────

// One click with no form means the section itself carries the explanation of
// what connecting grants. It must be a always-on element in the Detected
// section — not a dialog (which a one-click path never opens) and not a
// tooltip (which a keyboard user may never see).
//
// Mutation: delete the `<p>` (or move it behind a conditional) and this fails.
test("the Detected section carries a permanent disclosure line", async () => {
  const panel = stripJsComments(await read("src/ExtensionsPanel.tsx"));
  const sectionAt = panel.indexOf("extensions-section--detected");
  const section = panel.slice(sectionAt, panel.indexOf("</section>", sectionAt));
  assert.match(section, /settings\.extensions\.detectedDisclosure/, "the disclosure copy must be rendered");
  assert.ok(
    !/\{[^}]*&&\s*\(\s*<p className="extensions-section-hint/.test(section),
    "the disclosure must not be gated behind a condition",
  );

  const en = createTranslator("en")("settings.extensions.detectedDisclosure");
  const zh = createTranslator("zh")("settings.extensions.detectedDisclosure");
  assert.match(en, /environment/i, "the English disclosure must name the environment permission");
  assert.match(en, /process/i, "the English disclosure must name the process-spawn permission");
  assert.match(zh, /环境/, "the Chinese disclosure must name the environment permission");
  assert.match(zh, /进程/, "the Chinese disclosure must name the process-spawn permission");
  for (const forbidden of ["filesystem-write", "network-fetch", "clipboard"]) {
    assert.ok(
      !en.includes(forbidden) && !zh.includes(forbidden),
      `${forbidden} is never part of a tool binding and must not appear in the disclosure`,
    );
  }
});

// The disclosure must have a style rule (an unstyled paragraph inside the
// section grid would inherit the row typography and read as a row).
test("the section hint has its own CSS rule", async () => {
  const css = (await read("src/styles/extensions.css")).replace(/\/\*[\s\S]*?\*\//g, "");
  assert.match(css, /\.extensions-section-hint\s*\{/, "the hint needs a rule");
  const rule = css.slice(css.indexOf(".extensions-section-hint {"));
  const body = rule.slice(0, rule.indexOf("}"));
  assert.match(body, /color:\s*var\(--text-tertiary\)/, "the hint is quieter than a row");
  assert.match(body, /font-size:\s*var\(--text-caption\)/, "the hint is caption-sized");
});

// ── 4. The retired path is really gone ─────────────────────────────────────

// The old flow must not survive anywhere as a second route to the same result:
// the panel's Detected section is the only caller of the one-click command, and
// the drawer is reached only from the overflow menu's blank-create item.
test("the one-click command is wired exactly once", async () => {
  const panel = stripJsComments(await read("src/ExtensionsPanel.tsx"));
  const calls = panel.split('invoke("extensions_connect_tool"').length - 1;
  assert.equal(calls, 1, "exactly one call site keeps the zero-form path the only one");
});
