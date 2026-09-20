// R9-2 slice 1 · the manual-run entry and its output route.
//
// Three things this round added to the frontend, each with a mutation that
// turns it back into the old behaviour:
//
//   1. `ExtensionRow` gained a Run control that dispatches on the *backend's*
//      returned route — terminal goes to `runCommand` (terminal page + PTY),
//      background completes inline with a toast;
//   2. the manifest's `output` mode has an inline switch per row and a radio
//      pair in the drawer, both writing through the ordinary update command;
//   3. a background run leaves a "last output" record the row can expand
//      inline (a fixed-height scroller, never an overlay).
//
// The pure projections are driven directly; the JSX facts are read off the
// source, because the panel is a 2000-line component the node runner does not
// render.

import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";
import { createTranslator } from "../src/i18n.ts";
import {
  DEFAULT_OUTPUT_MODE,
  formatRunDuration,
  outputModeLabel,
  runAvailability,
  type RunnableExtension,
} from "../src/extensions/run-routing.ts";

const root = new URL("../", import.meta.url);
const read = (path: string) => readFile(new URL(path, root), "utf8");
const stripJsComments = (source: string) =>
  source.replace(/\/\*[\s\S]*?\*\//g, "").replace(/^\s*\/\/.*$/gm, "");

const row: RunnableExtension = {
  connected: true,
  enabled: true,
  state: "enabled",
  runtimeAvailable: true,
};

// ── 1 · run availability mirrors the backend's runnable check ──────────────

test("a row can run only when it is connected, enabled, healthy and present", () => {
  assert.equal(runAvailability(row), true);
  // Each dimension on its own removes the affordance. Mutation: drop any one
  // of the four conjuncts from `runAvailability` and its case goes red.
  const cases: Array<[string, Partial<RunnableExtension>]> = [
    ["disconnected", { connected: false }],
    ["disabled", { enabled: false, state: "disabled" }],
    ["broken", { state: "broken" }],
    ["runtime missing", { runtimeAvailable: false }],
  ];
  for (const [label, patch] of cases) {
    assert.equal(
      runAvailability({ ...row, ...patch }),
      false,
      `${label} must not be runnable`,
    );
  }
});

// The backend re-checks the same four facts (`extensions::run::runnable_entry`)
// and refuses a disabled or broken integration. The projection exists so the
// control's state and the command's answer cannot disagree; this asserts the
// panel actually calls it rather than re-deriving the rule inline.
test("the panel projects run availability through the shared helper", async () => {
  const panel = stripJsComments(await read("src/ExtensionsPanel.tsx"));
  assert.match(
    panel,
    /runAvailable=\{runAvailability\(extension\)\}/,
    "the row's run availability must come from the shared projection",
  );
  // …and the helper is the only place the run rule is spelled out: the row's
  // run prop is fed from it, not from a second inline conjunction.
  const helper = stripJsComments(await read("src/extensions/run-routing.ts"));
  assert.match(helper, /extension\.connected[\s\S]{0,80}&& extension\.enabled/, "the rule lives in the helper");
  assert.ok(
    !/runAvailable=\{[^}]*&&/.test(panel),
    "the panel must not re-derive the availability rule inline",
  );
});

// ── 2 · the duration wording is one function ───────────────────────────────

test("run durations read in milliseconds below a second and seconds above", () => {
  assert.equal(formatRunDuration(0), "0 ms");
  assert.equal(formatRunDuration(420), "420 ms");
  assert.equal(formatRunDuration(999), "999 ms");
  assert.equal(formatRunDuration(1000), "1.0 s");
  assert.equal(formatRunDuration(3420), "3.4 s");
  // A clock that went backwards must not render "NaN ms" or a negative.
  assert.equal(formatRunDuration(Number.NaN), "0 ms");
  assert.equal(formatRunDuration(-5), "0 ms");
  assert.equal(formatRunDuration(Number.POSITIVE_INFINITY), "0 ms");
});

// ── 3 · the row dispatches on the route the backend returned ───────────────

test("a terminal run hands the protected plan to the terminal page", async () => {
  const panel = stripJsComments(await read("src/ExtensionsPanel.tsx"));
  const at = panel.indexOf('invoke<RunOutcome>("extensions_run"');
  assert.notEqual(at, -1, "the run handler must call extensions_run");
  // The route comes from the outcome, never from a frontend-side guess.
  const handler = panel.slice(at, panel.indexOf("const toggleOutputView", at));
  assert.match(
    handler,
    /if \(outcome\.route === "terminal"\)/,
    "the terminal branch must read the backend's route",
  );
  assert.match(
    handler,
    /onOpenCommand\(outcome\.plan, extension\.name\)/,
    "a terminal run must go through the existing runCommand channel",
  );
  // The frontend must never assemble argv: the only plan it forwards is the
  // one the backend returned (a token-bearing plan), and it never touches
  // `plan.args` / `plan.program`.
  assert.ok(
    !/outcome\.plan\.args/.test(handler) && !/outcome\.plan\.program/.test(handler),
    "the frontend must not read argv off the plan",
  );
  assert.ok(
    !/\.join\(/.test(handler),
    "the frontend must not join anything into a command string",
  );
});

test("a background run reports through the toast stack and keeps the output", async () => {
  const panel = stripJsComments(await read("src/ExtensionsPanel.tsx"));
  const at = panel.indexOf('invoke<RunOutcome>("extensions_run"');
  const handler = panel.slice(at, panel.indexOf("const toggleOutputView", at));
  // Success and failure are two different notices, and the failure names the
  // exit code — the fact a user needs from a script that did not succeed.
  assert.match(handler, /customRunSucceeded/, "a successful run must report completion");
  assert.match(handler, /customRunFailedToast/, "a failed run must report the exit code");
  assert.match(handler, /outcome\.exitCode/, "the failure notice must carry the real exit code");
  assert.match(handler, /formatRunDuration\(outcome\.durationMs\)/, "the notice must include the duration");
  // The captured output is retained for the row's inline scroller.
  assert.match(handler, /setRunOutputs/, "the background output must be retained");
  // The notification path is the panel's ordinary toast, not a new
  // CompletionAction (the closed enum in notifications.rs stays untouched).
  const notifications = await read("src/notifications.ts");
  assert.ok(
    !/RunScript/.test(notifications),
    "the run feedback must not add a CompletionAction variant",
  );
});

// ── 4 · the output mode is configured in the drawer only (R9-2 slice 5) ────

test("the row no longer carries an inline output-mode switch", async () => {
  // The user's feedback: "the output mode should just be set in the config, the
  // list doesn't need its own switch". The row keeps Run and View output, and
  // the mode lives in the drawer editor alone.
  const source = stripJsComments(await read("src/extensions/ExtensionRow.tsx"));
  assert.ok(
    !/extension-row__output-switch/.test(source),
    "the row must not render an output-mode switch",
  );
  assert.ok(
    !/onToggleOutputMode/.test(source),
    "the row must not accept an output-mode toggle handler",
  );
  // Run and the output reveal are the only two run-related affordances left.
  assert.match(source, /customRun\b|customRunning\b/, "the row must keep its Run control");
  assert.match(source, /customViewOutput/, "the row must keep its view-output affordance");
  // The panel no longer owns a toggle handler that flips the mode per row.
  const panel = stripJsComments(await read("src/ExtensionsPanel.tsx"));
  assert.ok(
    !/const toggleOutputMode/.test(panel),
    "the panel must not keep a per-row output-mode toggle",
  );
  assert.ok(
    !/outputModeBusy/.test(panel),
    "the panel must not pass an output-mode busy flag to the row",
  );
});

test("a new integration defaults to background output", async () => {
  // The drawer's form is seeded from `DEFAULT_CUSTOM_INTEGRATION.output`, which
  // is the shared `DEFAULT_OUTPUT_MODE`. Mutation: flip that constant to
  // "terminal" and both assertions below go red.
  assert.equal(DEFAULT_OUTPUT_MODE, "background", "the shared default must be background");
  const panel = stripJsComments(await read("src/ExtensionsPanel.tsx"));
  assert.match(panel, /output: DEFAULT_OUTPUT_MODE/, "the form must be seeded from the shared default");
  assert.ok(
    !/output: "terminal"/.test(panel),
    "no form default may hard-code the terminal mode",
  );
  // The drawer's radio reflects that draft value and nothing else.
  const drawer = stripJsComments(await read("src/extensions/CustomIntegrationDrawer.tsx"));
  assert.match(
    drawer,
    /aria-checked=\{integration\.output === mode\}/,
    "the drawer's radio must reflect the draft's mode",
  );
});

test("the drawer is the only surface that sets the output mode", async () => {
  const drawer = stripJsComments(await read("src/extensions/CustomIntegrationDrawer.tsx"));
  assert.match(drawer, /extension-custom-output/, "the drawer must render the output block");
  assert.match(
    drawer,
    /\["background", "terminal"\] as const\)\.map/,
    "the drawer must offer exactly the two declared modes",
  );
  // The selector writes the manifest through the ordinary update transaction.
  assert.match(drawer, /update\(\(current\) => \(\{ \.\.\.current, output: mode \}\)\)/, "the radio must write the draft");
  // Both mode names render from the drawer's own keys.
  for (const key of ["customOutputTerminal", "customOutputBackground"]) {
    assert.ok(drawer.includes(key), `${key} must be rendered in the drawer`);
  }
  assert.equal(outputModeLabel("terminal"), "terminal");
  assert.equal(outputModeLabel("background"), "background");
});

// ── 5 · the "last output" reveal is inline and bounded ─────────────────────

test("the last output expands inline as a fixed-height scroller, not an overlay", async () => {
  const row = stripJsComments(await read("src/extensions/ExtensionRow.tsx"));
  assert.match(row, /extension-row__output/, "the row must render the output block");
  assert.match(row, /<pre className="extension-row__output-body">/, "the output body must be a read-only <pre>");
  assert.match(row, /customViewOutput/, "there must be a view-output affordance");
  assert.ok(
    !/extension-permission-backdrop|role="dialog"|aria-modal/.test(row),
    "the output view must not be an overlay or dialog",
  );

  const css = stripJsComments(await read("src/styles/extensions.css"));
  const body = css.slice(css.indexOf(".extension-row__output-body {"));
  const rule = body.slice(0, body.indexOf("}"));
  assert.match(rule, /max-height:\s*180px/, "the scroller must be height-bounded");
  assert.match(rule, /overflow:\s*auto/, "the scroller must scroll rather than grow the row");
  // The block spans the row's full width on its own line.
  const block = css.slice(css.indexOf(".extension-row__output {"));
  assert.match(block.slice(0, block.indexOf("}")), /grid-column:\s*1 \/ -1/, "the output block must span the row");
});

// A terminal run streams into the PTY and leaves no captured record, so the
// view-output affordance must be gated on a record existing.
test("the output reveal appears only when a background run left a record", async () => {
  const row = stripJsComments(await read("src/extensions/ExtensionRow.tsx"));
  const gate = row.slice(row.indexOf("customViewOutput") - 400, row.indexOf("customViewOutput"));
  assert.match(gate, /lastOutput && !outputOpen/, "the affordance must be gated on a stored output");
});

// ── 6 · i18n symmetry for the new keys ─────────────────────────────────────

test("every new run/output key exists in both dictionaries with the same placeholders", async () => {
  const i18n = await read("src/i18n.ts");
  const en = i18n.slice(i18n.indexOf("const en = {"), i18n.indexOf("export type MessageKey"));
  const zh = i18n.slice(i18n.indexOf("const zh: Record<MessageKey, string> = {"), i18n.indexOf("const messages: Record<Language"));
  const keys = [
    "settings.extensions.customOutput",
    "settings.extensions.customOutputTerminal",
    "settings.extensions.customOutputBackground",
    "settings.extensions.customOutputHint",
    "settings.extensions.customRun",
    "settings.extensions.customRunning",
    "settings.extensions.customRunUnavailable",
    "settings.extensions.customRunSucceeded",
    "settings.extensions.customRunFailedToast",
    "settings.extensions.customLastOutput",
    "settings.extensions.customViewOutput",
    "settings.extensions.customHideOutput",
    "settings.extensions.customNoOutput",
    "settings.extensions.customOutputTruncated",
  ];
  for (const key of keys) {
    const occurrences = i18n.split(`"${key}"`).length - 1;
    assert.equal(occurrences, 2, `${key} must be declared once per language`);
    assert.ok(en.includes(`"${key}"`), `${key} must be in en`);
    assert.ok(zh.includes(`"${key}"`), `${key} must be in zh`);
  }
  // Placeholders must match across languages (the symmetry sweep also checks
  // this globally; pinned here so a rename fails in this file's context too).
  for (const key of ["customRunSucceeded", "customRunFailedToast"]) {
    const line = (body: string) => body.slice(body.indexOf(`"settings.extensions.${key}"`));
    const placeholders = (value: string) =>
      [...value.slice(0, value.indexOf("\n")).matchAll(/\{(\w+)\}/g)].map((m) => m[1]).sort();
    assert.deepEqual(placeholders(line(zh)), placeholders(line(en)), `${key} placeholders must match`);
  }
});

test("the run notices render with their real arguments", () => {
  const t = createTranslator("en");
  assert.match(t("settings.extensions.customRunSucceeded", { name: "Deploy", duration: "1.2 s" }), /Deploy.*1\.2 s/);
  assert.match(t("settings.extensions.customRunFailedToast", { name: "Deploy", code: 3 }), /Deploy.*3/);
});
