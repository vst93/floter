// R9-2 slice 4 · the output-routing polish pass.
//
// The last slice of the arc tightened the edges around the two run routes
// rather than adding a capability:
//
//   1. the completion toast now reports *how much* was captured (line count,
//      and whether the capture was capped) and offers a one-press jump to the
//      inline output block;
//   2. a second run of an integration that already has one in flight is
//      refused, in the panel (first line) and on the backend
//      (`run_already_in_flight:*`), with a warning toast instead of silence;
//   3. the inline block has a real empty state and a truncation line at the
//      *end* of the text, and the run-parameter form states which route the
//      output will take.
//
// The pure projections (`runOutputLineCount`, `runOutputSummary`,
// `hasRunOutput`) are driven directly; the JSX facts are read off the source,
// the same split the earlier slices established.

import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";
import { createTranslator } from "../src/i18n.ts";
import {
  hasRunOutput,
  runOutputLineCount,
  runOutputSummary,
  type CapturedRunOutput,
} from "../src/extensions/run-routing.ts";

const root = new URL("../", import.meta.url);
const read = (path: string) => readFile(new URL(path, root), "utf8");
const stripJsComments = (source: string) =>
  source.replace(/\/\*[\s\S]*?\*\//g, "").replace(/^\s*\/\/.*$/gm, "");

const output = (patch: Partial<CapturedRunOutput> = {}): CapturedRunOutput => ({
  stdout: "",
  stderr: "",
  truncated: false,
  ...patch,
});

// ── 1 · the line count is the summary's one source ─────────────────────────

test("the line count reads both streams the way the block renders them", () => {
  // Empty capture is zero lines — not one. A script that printed nothing must
  // report "0 lines" rather than claiming a line of blank output.
  assert.equal(runOutputLineCount(output()), 0);
  // A single trailing newline is a terminator, not an extra empty line.
  assert.equal(runOutputLineCount(output({ stdout: "one\n" })), 1);
  assert.equal(runOutputLineCount(output({ stdout: "one\ntwo\n" })), 2);
  // A stream that is exactly one newline still counts as one line.
  assert.equal(runOutputLineCount(output({ stdout: "\n" })), 1);
  // A final line without a newline is still a line.
  assert.equal(runOutputLineCount(output({ stdout: "one\ntwo" })), 2);
  // Both streams read in capture order (the same join the block renders).
  assert.equal(runOutputLineCount(output({ stdout: "a\n", stderr: "b\nc\n" })), 3);
  // Mutation: drop the trailing-newline trim and `"one\n"` becomes 2.
  assert.notEqual(runOutputLineCount(output({ stdout: "one\n" })), 2);
});

test("hasRunOutput distinguishes a silent run from one that printed", () => {
  assert.equal(hasRunOutput(output()), false);
  assert.equal(hasRunOutput(output({ stdout: "x" })), true);
  assert.equal(hasRunOutput(output({ stderr: "warn\n" })), true);
});

test("the toast summary names the line count and the cap", () => {
  const t = createTranslator("en");
  const plain = runOutputSummary(output({ stdout: "a\nb\n" }), t);
  assert.match(plain, /2 lines/);
  assert.ok(!/truncat/i.test(plain), "an uncapped capture must not claim truncation");
  const capped = runOutputSummary(output({ stdout: "a\n", truncated: true }), t);
  assert.match(capped, /1 lines/);
  assert.match(capped, /truncat/i);
  assert.match(capped, /64 ?KB/i, "the cap must name its size");
  // The Chinese dictionary carries the same placeholders.
  const zh = createTranslator("zh");
  assert.match(runOutputSummary(output({ stdout: "a\n", truncated: true }), zh), /1/);
});

// ── 2 · the completion toast branches on success, failure and output ───────

test("a successful run reports its line count and offers the output jump", async () => {
  const panel = stripJsComments(await read("src/ExtensionsPanel.tsx"));
  const at = panel.indexOf('invoke<RunOutcome>("extensions_run"');
  const handler = panel.slice(at, panel.indexOf("const toggleOutputMode", at));
  assert.match(handler, /runOutputSummary\(outcome\.output, t\)/, "the success toast must summarise the output");
  // The action rides the toast only when there is something to show, and it
  // opens the same inline block the row's own toggle opens.
  assert.match(handler, /customViewOutput/, "the success toast must offer a view-output action");
  assert.match(
    handler,
    /run: \(\) => setOutputOpen\(\(current\) => \(\{ \.\.\.current, \[extension\.id\]: true \}\)\)/,
    "the toast action must expand the row's inline output block",
  );
});

test("a failed run reports the exit code as a warning and opens the output", async () => {
  const panel = stripJsComments(await read("src/ExtensionsPanel.tsx"));
  const at = panel.indexOf('invoke<RunOutcome>("extensions_run"');
  const handler = panel.slice(at, panel.indexOf("const toggleOutputMode", at));
  assert.match(handler, /customRunFailedToast/, "the failure toast key must remain");
  assert.match(handler, /outcome\.exitCode/, "the failure toast must carry the real exit code");
  // Warning, not error: a script that exits non-zero is a completed run whose
  // result the user asked for, not a host failure.
  assert.match(handler, /onNotify\("warning"[^)]*customRunFailedToast/s, "the failure notice must be a warning");
  assert.match(handler, /setOutputOpen\(\(current\) => \(\{ \.\.\.current, \[extension\.id\]: true \}\)\)/, "the failed run's output must be opened");
});

// ── 3 · a second concurrent run is refused, and the hint says so ───────────

test("the panel refuses a second run while one is in flight", async () => {
  const panel = stripJsComments(await read("src/ExtensionsPanel.tsx"));
  // Both entry points guard: the run handler and the form opener.
  assert.equal(
    (panel.match(/customRunAlreadyRunning/g) ?? []).length >= 2,
    true,
    "both run entry points must announce the in-flight refusal",
  );
  const at = panel.indexOf("const runExtension");
  const handler = panel.slice(at, panel.indexOf("const cancelRunForm", at));
  assert.match(handler, /if \(runBusy\)[\s\S]{0,200}customRunAlreadyRunning/, "runExtension must refuse while runBusy");
  // The backend's keyed refusal maps to the same notice rather than a raw
  // error string (a second window racing the local guard).
  assert.match(handler, /run_already_in_flight:/, "the backend refusal must be recognised");
});

test("the backend refuses a concurrent run with a stable keyed error", async () => {
  const run = stripJsComments(await read("src-tauri/src/extensions/run.rs"));
  assert.match(run, /const RUN_ALREADY_IN_FLIGHT: &str = "run_already_in_flight"/, "the key must be a constant");
  // The registry is a per-id set handed out as an RAII guard, so every exit
  // path (including a panic) releases the slot.
  assert.match(run, /struct RunInFlight \{/, "the in-flight registry must exist");
  assert.match(run, /fn begin\(&self, id: &str\) -> Result<RunInFlightGuard<'_>, String>/, "begin must hand out a guard");
  assert.match(run, /impl Drop for RunInFlightGuard/, "the guard must release on drop");
  assert.match(run, /let _in_flight = state\.begin_run\(id\)\?;/, "run() must claim the slot");
  // The argv builder is untouched by this: the guard is the only addition.
  assert.match(run, /fn param_arguments\(/, "the argv builder must still exist");
});

// ── 4 · the inline block's empty state and truncation tail ─────────────────

test("the inline block shows a muted placeholder when the run printed nothing", async () => {
  const row = stripJsComments(await read("src/extensions/ExtensionRow.tsx"));
  assert.match(row, /extension-row__output-empty/, "the empty capture must render its own element");
  const empty = row.slice(row.indexOf("extension-row__output-empty"));
  assert.match(empty.slice(0, 120), /customNoOutput/, "the placeholder must use the no-output key");
  const css = stripJsComments(await read("src/styles/extensions.css"));
  const rule = css.slice(css.indexOf(".extension-row__output-empty {"));
  assert.match(rule.slice(0, rule.indexOf("}")), /color:\s*var\(--text-muted\)/, "the empty state must be muted");
});

test("a truncated capture says so at the end of the text", async () => {
  const row = stripJsComments(await read("src/extensions/ExtensionRow.tsx"));
  assert.match(row, /extension-row__output-tail/, "the truncation line must exist");
  const tail = row.slice(row.indexOf("extension-row__output-tail"));
  const gate = row.slice(Math.max(0, row.indexOf("extension-row__output-tail") - 300), row.indexOf("extension-row__output-tail"));
  assert.match(gate, /lastOutput\?\.truncated/, "the line must be gated on truncation");
  assert.match(tail.slice(0, 200), /customOutputTruncatedTail/, "the line must use the tail key");
});

test("the output block reuses the terminal page's existing tokens, not new colours", async () => {
  const css = stripJsComments(await read("src/styles/extensions.css"));
  const block = css.slice(css.indexOf(".extension-row__output {"));
  const rule = block.slice(0, block.indexOf("}"));
  // Surface and text tokens already used across the panel; no raw colour.
  assert.match(rule, /background:\s*var\(--surface-sunken\)/, "the block must use the shared sunken surface");
  assert.match(rule, /border:\s*1px solid var\(--glass-control-edge\)/, "the edge must be the shared glass token");
  assert.ok(
    !/#[0-9a-fA-F]{3,8}\b/.test(rule) && !/rgba?\(/.test(rule),
    "the output block must not invent a colour",
  );
  // Mobile safety: the scroller is bounded and cannot widen the row.
  const body = css.slice(css.indexOf(".extension-row__output-body {"));
  const bodyRule = body.slice(0, body.indexOf("}"));
  assert.match(bodyRule, /max-height:\s*180px/, "the scroller must stay height-bounded");
  assert.match(bodyRule, /overflow-wrap:\s*anywhere/, "long tokens must wrap instead of widening the row");
});

// ── 5 · the parameter form states the output route ─────────────────────────

test("the run-parameter form names the route the run will take", async () => {
  const form = stripJsComments(await read("src/extensions/RunParamForm.tsx"));
  assert.match(form, /outputMode: "background" \| "terminal"/, "the form must take the manifest's mode");
  assert.match(form, /customRunOutputHintTerminal/, "the terminal route must have its own line");
  assert.match(form, /customRunOutputHintBackground/, "the background route must have its own line");
  assert.match(form, /extension-run-params__output-hint/, "the hint must render as its own element");
  // The mode is passed through from the row, which reads the extension.
  const row = stripJsComments(await read("src/extensions/ExtensionRow.tsx"));
  assert.match(row, /outputMode=\{extension\.output\}/, "the row must pass the manifest's declared mode");
  assert.match(form, /className="extension-run-params__output-hint"/, "the hint must render in the form body");
});

test("the route hints are distinct and translate", () => {
  const en = createTranslator("en");
  const zh = createTranslator("zh");
  const terminal = "settings.extensions.customRunOutputHintTerminal";
  const background = "settings.extensions.customRunOutputHintBackground";
  assert.notEqual(
    en(terminal),
    en(background),
    "the two routes must not share one sentence",
  );
  assert.match(en(terminal), /terminal/i);
  assert.match(en(background), /background/i);
  assert.notEqual(zh(terminal), zh(background), "the zh hints must differ too");
});

// ── 6 · i18n symmetry for the new keys ─────────────────────────────────────

test("every new key exists in both dictionaries with matching placeholders", async () => {
  const i18n = await read("src/i18n.ts");
  const en = i18n.slice(i18n.indexOf("const en = {"), i18n.indexOf("export type MessageKey"));
  const zh = i18n.slice(i18n.indexOf("const zh: Record<MessageKey, string> = {"), i18n.indexOf("const messages: Record<Language"));
  const keys = [
    "settings.extensions.customRunOutputLines",
    "settings.extensions.customRunOutputLinesTruncated",
    "settings.extensions.customRunAlreadyRunning",
    "settings.extensions.customRunOutputHintTerminal",
    "settings.extensions.customRunOutputHintBackground",
    "settings.extensions.customOutputTruncatedTail",
  ];
  for (const key of keys) {
    assert.ok(en.includes(`"${key}"`), `${key} must be in en`);
    assert.ok(zh.includes(`"${key}"`), `${key} must be in zh`);
  }
  for (const key of ["customRunOutputLines", "customRunOutputLinesTruncated"]) {
    const line = (body: string) => body.slice(body.indexOf(`"settings.extensions.${key}"`));
    const placeholders = (value: string) =>
      [...value.slice(0, value.indexOf("\n")).matchAll(/\{(\w+)\}/g)].map((m) => m[1]).sort();
    assert.deepEqual(placeholders(line(zh)), placeholders(line(en)), `${key} placeholders must match`);
  }
});
