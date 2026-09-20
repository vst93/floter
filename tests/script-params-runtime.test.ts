// R9-2 slice 3 · the run-time parameter form and its argv hand-off.
//
// Slice 2 declared inputs; this round is the user filling them in. Three things
// the frontend owns, each with a mutation that turns it back into the old
// behaviour:
//
//   1. the inputs are *seeded* (session memory, then default, then blank) and
//      validated before a run is allowed — one pure projection in
//      `run-params.ts`, mirrored from the backend's required/number/select
//      rules;
//   2. the row gained an inline form driven by the declared kind (text /
//      number / boolean / select / path), reusing the house switch and the
//      existing action-button and error languages — never a dialog;
//   3. the collected answers go to `extensions_run` as `values`; the frontend
//      never builds argv. A backend `run_param_*` refusal becomes an inline
//      message, not a toast.
//
// The pure projection is driven directly; the JSX facts are read off the
// source, because the panel and the row are components the node runner does
// not render.

import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";
import { createTranslator } from "../src/i18n.ts";
import { emptyParam, type ScriptParam } from "../src/extensions/script-params.ts";
import {
  collectParamValues,
  paramRunErrorMessage,
  paramValueIssues,
  seedParamValue,
  seedParamValues,
} from "../src/extensions/run-params.ts";

const root = new URL("../", import.meta.url);
const read = (path: string) => readFile(new URL(path, root), "utf8");
const stripJsComments = (source: string) =>
  source.replace(/\/\*[\s\S]*?\*\//g, "").replace(/^\s*\/\/.*$/gm, "");

const param = (patch: Partial<ScriptParam> = {}): ScriptParam => ({ ...emptyParam(), ...patch });

// ── 1 · seeding: memory, then default, then blank ──────────────────────────

test("a value is seeded from session memory, then the default, then blank", () => {
  const withDefault = param({ id: "target", default: "example.com" });
  assert.equal(seedParamValue(withDefault), "example.com");
  // Memory wins over the default: the last answer is the one to repeat.
  assert.equal(seedParamValue(withDefault, "other.host"), "other.host");
  // A remembered empty string is a real answer, not "no memory".
  assert.equal(seedParamValue(withDefault, ""), "");
  // A boolean with no default seeds "false" — the state its switch shows.
  assert.equal(seedParamValue(param({ id: "force", kind: "boolean" })), "false");
  // A blank text param seeds an empty string.
  assert.equal(seedParamValue(param({ id: "note" })), "");
});

test("seeding a whole form keys every value by parameter id", () => {
  const params = [
    param({ id: "target", default: "example.com" }),
    param({ id: "count", kind: "number" }),
  ];
  assert.deepEqual(seedParamValues(params), { target: "example.com", count: "" });
  assert.deepEqual(seedParamValues(params, { count: "3" }), { target: "example.com", count: "3" });
});

// ── 2 · validation mirrors the backend's required/number/select rules ──────

test("a required parameter with no value and no default blocks the run", () => {
  const required = param({ id: "target", label: "Target host", required: true });
  // Mutation: drop the `param.required` branch and this goes green while the
  // backend would refuse the run.
  const issues = paramValueIssues([required], { target: "" });
  assert.deepEqual(issues, [
    { index: 0, key: "settings.extensions.customParamRequiredMissing", label: "Target host" },
  ]);
  // A blank label falls back to the id so the message still names something.
  assert.equal(paramValueIssues([param({ id: "target", required: true })], {})[0].label, "target");
  // A default satisfies `required` (it is a pre-fill, not an exemption).
  assert.deepEqual(
    paramValueIssues([param({ id: "target", required: true, default: "x" })], { target: "" }),
    [],
  );
  // An optional blank is fine.
  assert.deepEqual(paramValueIssues([param({ id: "note" })], { note: "" }), []);
});

test("kind rules reject a bad number or an out-of-range select", () => {
  assert.deepEqual(
    paramValueIssues([param({ id: "count", kind: "number" })], { count: "12" })[0]?.key,
    undefined,
  );
  assert.equal(
    paramValueIssues([param({ id: "count", kind: "number" })], { count: "three" })[0].key,
    "settings.extensions.customParamInvalidValue",
  );
  const mode = param({ id: "mode", kind: "select", options: ["fast", "slow"] });
  assert.deepEqual(paramValueIssues([mode], { mode: "fast" }), []);
  assert.equal(
    paramValueIssues([mode], { mode: "turbo" })[0].key,
    "settings.extensions.customParamInvalidValue",
  );
  // Boolean answers are the two literal strings its switch writes.
  assert.deepEqual(paramValueIssues([param({ id: "force", kind: "boolean" })], { force: "true" }), []);
  assert.equal(
    paramValueIssues([param({ id: "force", kind: "boolean" })], { force: "yes" })[0].key,
    "settings.extensions.customParamInvalidValue",
  );
});

// ── 3 · collection: declared ids only, blanks absent ───────────────────────

test("collected values drop blanks and undeclared keys", () => {
  const params = [param({ id: "target" }), param({ id: "note" })];
  assert.deepEqual(collectParamValues(params, { target: "example.com", note: "" }), {
    target: "example.com",
  });
  // A key the form does not know is never forwarded — the backend would refuse
  // it, and the frontend must not manufacture argv from an unknown id.
  assert.deepEqual(collectParamValues(params, { target: "x", sneaky: "--rm" }), { target: "x" });
});

// ── 4 · a backend refusal maps to a localised inline message ───────────────

test("a run_param_* refusal becomes a message, anything else stays null", () => {
  const t = createTranslator("en");
  const translate = (key: Parameters<typeof t>[0], values: { label: string }) => t(key, values);
  const params = [param({ id: "target", label: "Target host", required: true })];
  assert.match(
    paramRunErrorMessage("run_param_required:target", params, translate)!,
    /Target host/,
  );
  assert.match(paramRunErrorMessage("run_param_invalid:target", params, translate)!, /Target host/);
  assert.match(
    paramRunErrorMessage("run_param_windows_unsafe:target", params, translate)!,
    /Target host/,
  );
  assert.match(paramRunErrorMessage("run_param_unknown:ghost", params, translate)!, /ghost/);
  // Mutation: broaden the regex and a genuine failure is swallowed into a
  // parameter message.
  assert.equal(paramRunErrorMessage("Run timed out after 300 seconds", params, translate), null);
});

// ── 5 · the row renders the form inline, driven by kind ────────────────────

test("the run form is an inline row block, not a dialog", async () => {
  const form = stripJsComments(await read("src/extensions/RunParamForm.tsx"));
  assert.match(form, /export function RunParamForm\(/, "the form must be a component");
  // Kind-driven controls: a boolean is the house switch, a select a native
  // select, everything else a text input with a decimal inputMode for numbers.
  assert.match(form, /param\.kind === "boolean" \?/, "boolean renders the switch");
  assert.match(form, /role="switch"/, "the boolean control is a switch");
  assert.match(form, /param\.kind === "select" \?/, "select renders a select");
  assert.match(form, /inputMode=\{param\.kind === "number" \? "decimal" : undefined\}/, "number uses a decimal inputMode");
  assert.match(form, /settings-switch/, "the boolean reuses the settings switch");
  // No dialog, no overlay, no toast for a value error.
  assert.ok(!/role="dialog"|aria-modal|showModal|<dialog|backdrop/.test(form), "the form must not be a dialog");
  assert.ok(!/onNotify|showError/.test(form), "a value error must be inline, not a toast");
  // The row renders it and gates it on the declaration.
  const row = stripJsComments(await read("src/extensions/ExtensionRow.tsx"));
  assert.match(row, /<RunParamForm/, "the row must render the form");
  assert.match(row, /hasParams && runFormOpen/, "the form must be gated on declared params + open state");
  assert.equal((row.match(/const hasParams = \(runParams\?\.length \?\? 0\) > 0/) ?? []).length, 1);
  // The confirm passes the result of `collectParamValues`, which strips blanks
  // and undeclared keys. Mutation: pass the raw form state and an undeclared
  // key (or a blank) reaches the backend, which the backend would refuse.
  assert.match(form, /onRun\(collectParamValues\(params, values\)\)/, "the confirm must collect, not forward raw");
});

test("the row's Run control opens the form only when params are declared", async () => {
  const panel = stripJsComments(await read("src/ExtensionsPanel.tsx"));
  const at = panel.indexOf("const runExtension = async");
  assert.notEqual(at, -1, "the run handler must exist");
  const handler = panel.slice(at, panel.indexOf("const cancelRunForm", at));
  // An integration with no params runs straight away (values stays undefined).
  assert.match(handler, /if \(values === undefined && params\.length > 0\) \{[\s\S]{0,80}openRunForm/, "declared params open the form first");
  // The invoke carries the collected values, and `null` when there are none.
  assert.match(handler, /invoke<RunOutcome>\("extensions_run"/, "the run must call extensions_run");
  assert.match(handler, /values: values \?\? null/, "the invoke must carry the values");
  // The frontend never assembles argv: no join, no plan.args/program touch.
  assert.ok(!/\.join\(/.test(handler), "the frontend must not join a command string");
  assert.ok(!/outcome\.plan\.(args|program)/.test(handler), "the frontend must not read argv");
  // An integration with no params never opens a form: the run proceeds at once
  // with `values` undefined, which is the pre-slice behavior.
  assert.match(handler, /if \(values === undefined && params\.length > 0\)/, "no params means no form");
  assert.match(handler, /values: values \?\? null/, "a paramless run sends no values");
});

test("a successful or failed run banks the answers; a refusal keeps the form open", async () => {
  const panel = stripJsComments(await read("src/ExtensionsPanel.tsx"));
  const at = panel.indexOf("const runExtension = async");
  const handler = panel.slice(at, panel.indexOf("const cancelRunForm", at));
  assert.match(handler, /setRunParamMemory/, "the answers must be remembered for the session");
  assert.ok(!/localStorage|sessionStorage/.test(handler), "the memory is in-memory only, never persisted");
  // A refusal maps to an inline error and leaves the form open.
  assert.match(handler, /paramRunErrorMessage\(/, "a refusal must go through the mapper");
  assert.match(handler, /setRunParamError\(mapped\)/, "a refusal must render inline");
  // Only a completed run clears the form; both success and failure branches do.
  assert.equal((handler.match(/setRunFormId\(null\)/g) ?? []).length, 3, "terminal/success/failure all close the form");
  // Memory pre-fills the *next* form for the same integration.
  assert.match(
    panel,
    /seedParamValues\(declaredRunParams\(extension\), runParamMemory\[extension\.id\]\)/,
    "opening the form must seed from this session's last answers",
  );
});

// ── 6 · i18n symmetry for the new keys ─────────────────────────────────────

test("every new run-parameter key exists in both dictionaries with matching placeholders", async () => {
  const i18n = await read("src/i18n.ts");
  const en = i18n.slice(i18n.indexOf("const en = {"), i18n.indexOf("export type MessageKey"));
  const zh = i18n.slice(i18n.indexOf("const zh: Record<MessageKey, string> = {"), i18n.indexOf("const messages: Record<Language"));
  const keys = [
    "settings.extensions.customParamRequiredMissing",
    "settings.extensions.customParamInvalidValue",
    "settings.extensions.customParamUnknown",
    "settings.extensions.customParamWindowsUnsafe",
    "settings.extensions.customParamInjectionNote",
    "settings.extensions.customRunValues",
    "settings.extensions.customRunValuesHint",
    "settings.extensions.customRunConfirm",
    "settings.extensions.customRunCancel",
    "settings.extensions.customRunRememberHint",
  ];
  for (const key of keys) {
    assert.equal(i18n.split(`"${key}"`).length - 1, 2, `${key} must be declared once per language`);
    assert.ok(en.includes(`"${key}"`), `${key} must be in en`);
    assert.ok(zh.includes(`"${key}"`), `${key} must be in zh`);
    const placeholders = (body: string) =>
      [...body.slice(0, body.indexOf("\n")).matchAll(/\{(\w+)\}/g)].map((m) => m[1]).sort();
    const line = (body: string) => body.slice(body.indexOf(`"${key}"`));
    assert.deepEqual(placeholders(line(zh)), placeholders(line(en)), `${key} placeholders must match`);
  }
  // The messages render with their real arguments.
  const enT = createTranslator("en");
  assert.match(
    enT("settings.extensions.customParamRequiredMissing", { label: "Target" }),
    /Target/,
  );
  assert.match(
    enT("settings.extensions.customParamWindowsUnsafe", { label: "Target" }),
    /Target/,
  );
});
