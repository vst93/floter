// R9-2 slice 2 · the script-parameter *definition* editor.
//
// This round adds the configuration half of "declare the inputs a script
// accepts". The runtime half (rendering inputs, turning answers into argv) is
// a later slice and is deliberately absent here.
//
// Three things the frontend now owns, each with a mutation that turns it back
// into the old behaviour:
//
//   1. the parameter shape and its validation live in one module, mirrored
//      from `manifest::validate_param_definitions` — including the flag
//      whitelist that is the front half of the argv injection defence;
//   2. the drawer gained an inline definition editor (add row, remove row,
//      kind-driven field changes) — never a dialog;
//   3. saving sends the normalized definitions, and an invalid row blocks the
//      submit with an inline message instead of a toast.
//
// The pure projection is driven directly; the JSX facts are read off the
// source, because the drawer is a component the node runner does not render.

import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";
import { createTranslator } from "../src/i18n.ts";
import {
  emptyParam,
  formatParamOptions,
  fromWireParams,
  PARAM_FLAG_PATTERN,
  PARAM_ID_PATTERN,
  PARAM_KINDS,
  paramIssues,
  parseParamOptions,
  toWireParams,
  type ScriptParam,
} from "../src/extensions/script-params.ts";

const root = new URL("../", import.meta.url);
const read = (path: string) => readFile(new URL(path, root), "utf8");
const stripJsComments = (source: string) =>
  source.replace(/\/\*[\s\S]*?\*\//g, "").replace(/^\s*\/\/.*$/gm, "");

const param = (patch: Partial<ScriptParam> = {}): ScriptParam => ({ ...emptyParam(), ...patch });

// ── 1 · the shape is one module and mirrors the backend ────────────────────

test("the kind vocabulary matches the Rust ParamKind enum", () => {
  assert.deepEqual([...PARAM_KINDS], ["text", "number", "boolean", "select", "path"]);
  // The id and flag whitelists are the exact regexes the backend applies
  // (`manifest.rs`): letters/numbers/./-/_ for the id, a leading `-` and only
  // token-safe characters for the flag.
  assert.equal(PARAM_ID_PATTERN.source, "^[A-Za-z0-9._-]+$");
  assert.equal(PARAM_FLAG_PATTERN.source, "^-[A-Za-z0-9_.-]+$");
});

test("a fresh row is blank and needs a name before it validates", () => {
  const fresh = emptyParam();
  assert.equal(fresh.id, "");
  assert.equal(fresh.kind, "text");
  assert.equal(fresh.required, false);
  assert.deepEqual(fresh.options, []);
  // Mutation: let an empty id pass and this goes green while the backend would
  // reject the save.
  assert.equal(paramIssues([fresh]).length, 1);
});

// ── 2 · validation is the backend rule, projected ──────────────────────────

test("ids are whitelisted and must be unique", () => {
  assert.deepEqual(paramIssues([param({ id: "target" })]), []);
  for (const id of ["", "Target Host", "target;rm", "a$b", "../etc", "a/b"]) {
    const issues = paramIssues([param({ id })]);
    assert.equal(issues.length, 1, `${id} must be rejected`);
    assert.equal(issues[0].key, "settings.extensions.customParamInvalidId");
  }
  const duplicate = paramIssues([param({ id: "target" }), param({ id: "target" })]);
  assert.deepEqual(duplicate, [
    { index: 1, key: "settings.extensions.customParamDuplicateId" },
  ]);
  // The duplicate check is scoped to valid ids: two blank ids report the id
  // problem twice rather than a misleading "already used".
  assert.deepEqual(
    paramIssues([param(), param()]).map((issue) => issue.key),
    [
      "settings.extensions.customParamInvalidId",
      "settings.extensions.customParamInvalidId",
    ],
  );
});

test("a flag must be one token — the injection defence's front half", () => {
  for (const flag of ["--target", "-t", "--dry-run", "--host.name", "--a_b"]) {
    assert.deepEqual(paramIssues([param({ id: "target", flag })]), [], `${flag} should be legal`);
  }
  // Mutation: allow `$` in the flag and the `$(whoami)` case below goes green,
  // taking the injection defence with it.
  for (const flag of [
    "target",
    "-",
    "--target value",
    "--target;rm -rf /",
    "--target$(whoami)",
    // A lone `$` is the minimal case: only the `$` branch rejects it, so
    // removing that branch turns this red.
    "--target$X",
    '--target"quoted"',
    "--target'q'",
    "--target|pipe",
    "--target&bg",
    "--target>out",
    "--target<in",
    "--target`cmd`",
    "--target%VAR%",
  ]) {
    const issues = paramIssues([param({ id: "target", flag })]);
    assert.equal(issues.length, 1, `${flag} must be rejected`);
    assert.equal(issues[0].key, "settings.extensions.customParamInvalidFlag");
  }
  // An empty flag string is "no flag", not an invalid one: `toWireParams`
  // normalizes it away on save.
  assert.deepEqual(paramIssues([param({ id: "target", flag: "" })]), []);
});

test("select needs options, and a default must be legal for its kind", () => {
  assert.deepEqual(
    paramIssues([param({ id: "mode", kind: "select" })])[0].key,
    "settings.extensions.customParamSelectNoOptions",
  );
  assert.deepEqual(
    paramIssues([param({ id: "mode", kind: "select", options: ["fast"] })]),
    [],
  );
  // Number defaults must parse; boolean defaults must be true/false; select
  // defaults must be one of the options.
  assert.equal(
    paramIssues([param({ id: "count", kind: "number", default: "three" })])[0].key,
    "settings.extensions.customParamInvalidDefault",
  );
  assert.deepEqual(paramIssues([param({ id: "count", kind: "number", default: "3" })]), []);
  assert.equal(
    paramIssues([param({ id: "force", kind: "boolean", default: "yes" })])[0].key,
    "settings.extensions.customParamInvalidDefault",
  );
  assert.deepEqual(paramIssues([param({ id: "force", kind: "boolean", default: "true" })]), []);
  assert.equal(
    paramIssues([param({ id: "mode", kind: "select", options: ["fast"], default: "slow" })])[0].key,
    "settings.extensions.customParamInvalidDefault",
  );
  // `required` + `default` is valid: the default is a pre-fill, not an
  // exemption. Mutation: treat a default as satisfying `required` and this
  // still passes — the rule is the *absence* of a check, asserted by the
  // combination not being reported.
  assert.deepEqual(
    paramIssues([param({ id: "target", required: true, default: "example.com" })]),
    [],
  );
});

test("issues carry the offending row index so the drawer can place them", () => {
  const issues = paramIssues([
    param({ id: "ok" }),
    param({ id: "bad id" }),
    param({ id: "flag", flag: "--x y" }),
  ]);
  assert.deepEqual(issues, [
    { index: 1, key: "settings.extensions.customParamInvalidId" },
    { index: 2, key: "settings.extensions.customParamInvalidFlag" },
  ]);
});

// ── 3 · the wire shape round-trips, with blanks normalized away ────────────

test("an empty optional is absent on the wire, not an empty string", () => {
  const editor = [param({ id: "target", label: " Target ", flag: "", default: "", placeholder: "" })];
  const wire = toWireParams(editor);
  assert.equal(wire[0].flag, null);
  assert.equal(wire[0].default, null);
  assert.equal(wire[0].placeholder, null);
  assert.equal(wire[0].label, "Target", "the label is trimmed");
  // Mutation: send the raw editor row and the backend's `deny_unknown_fields`
  // manifest gets a `""` flag, which the flag pattern rejects.
  assert.deepEqual(fromWireParams(wire), [
    { ...editor[0], label: "Target", flag: null, default: null, placeholder: null },
  ]);
});

test("a definition read back from a legacy manifest yields an empty list", () => {
  assert.deepEqual(fromWireParams(undefined), []);
  assert.deepEqual(fromWireParams(null), []);
  assert.deepEqual(fromWireParams([]), []);
  // An unknown kind degrades to text rather than throwing.
  const read_back = fromWireParams([{ ...emptyParam(), kind: "color" as never }]);
  assert.equal(read_back[0].kind, "text");
});

test("options parse from and format back to a comma-separated field", () => {
  assert.deepEqual(parseParamOptions("fast, slow ,, "), ["fast", "slow"]);
  assert.deepEqual(parseParamOptions(""), []);
  assert.equal(formatParamOptions(["fast", "slow"]), "fast, slow");
  assert.equal(formatParamOptions([]), "");
});

// ── 4 · the drawer renders the editor inline ───────────────────────────────

test("the drawer owns an inline parameter editor, not a dialog", async () => {
  const drawer = stripJsComments(await read("src/extensions/CustomIntegrationDrawer.tsx"));
  assert.match(drawer, /function ScriptParamEditor\(/, "the editor must be a component");
  assert.match(drawer, /<ScriptParamEditor params=\{integration\.params\}/, "the drawer must render it from the form state");
  // The three interactions the round is about: add, remove, kind change.
  assert.match(drawer, /onChange\(\[\.\.\.params, emptyParam\(\)\]\)/, "add a row");
  assert.match(drawer, /params\.filter\(\(_, i\) => i !== index\)/, "remove a row");
  assert.match(drawer, /kind: event\.target\.value as ScriptParamKind/, "the kind select writes the row");
  // Kind-driven field changes: a select row reveals its options, a boolean row
  // hides the placeholder.
  assert.match(drawer, /param\.kind === "select" && <label className="extension-param-editor__options"/, "select reveals options");
  assert.match(drawer, /param\.kind !== "boolean" && <label>[\s\S]{0,120}customParamPlaceholder/, "boolean hides the placeholder");
  // The empty state is a muted hint, not an error banner.
  assert.match(drawer, /customParamEmpty/, "the empty state must render");
  // The editor is inline: it must not open a dialog or a popover.
  const editor = drawer.slice(drawer.indexOf("function ScriptParamEditor("), drawer.indexOf("function ScriptRuntimeLine("));
  assert.ok(!/role="dialog"|aria-modal|showModal|<dialog/.test(editor), "the editor must not be a dialog");
  assert.ok(!/toast|onNotify/.test(editor), "the editor must not toast on a validation error");
});

test("a validation error renders under the offending row", async () => {
  const drawer = stripJsComments(await read("src/ExtensionsPanel.tsx"));
  // The submit guard reads the same projection the editor renders…
  assert.match(drawer, /paramIssues\(customIntegration\.params\)\[0\]/, "the submit guard must use the shared projection");
  assert.match(drawer, /setCustomIntegrationError\(t\(paramProblem\.key/, "the error must render inline");
  assert.ok(!/showError\(t\(paramProblem/.test(drawer), "a validation error must not become a toast");
});

// ── 5 · saving carries the definitions ─────────────────────────────────────

test("the save request sends normalized params and the form holds them", async () => {
  const panel = stripJsComments(await read("src/ExtensionsPanel.tsx"));
  assert.match(panel, /params: toWireParams\(customIntegration\.params\)/, "the request must carry wire params");
  assert.match(panel, /params: fromWireParams\(definition\.params\)/, "reading a definition must project the wire params");
  assert.match(panel, /params: \[\]/, "a fresh form starts with no params");
  const form = await read("src/ExtensionsPanel.tsx");
  assert.match(form, /type CustomIntegrationForm = \{[\s\S]*?params: ScriptParam\[\];/, "the form type must carry params");
});

// ── 6 · the template explains how a script reads argv ──────────────────────

test("every script template documents argv, in both languages", async () => {
  const { SCRIPT_LANGUAGES } = await import("../src/extensions/script-languages.ts");
  for (const language of SCRIPT_LANGUAGES) {
    assert.match(language.template, /argv/, `${language.id} must mention argv`);
    // The bilingual note: an English line and a Chinese line, each naming the
    // read mechanism. Mutation: drop the Chinese line and this goes red.
    assert.match(language.template, /编辑器里声明的参数/, `${language.id} must carry the Chinese note`);
  }
});

// ── 7 · i18n symmetry for the new keys ─────────────────────────────────────

test("every parameter key is translated in both languages", async () => {
  const keys = [
    "settings.extensions.customParams",
    "settings.extensions.customParamsHint",
    "settings.extensions.customParamAdd",
    "settings.extensions.customParamRemove",
    "settings.extensions.customParamEmpty",
    "settings.extensions.customParamLabel",
    "settings.extensions.customParamId",
    "settings.extensions.customParamType",
    "settings.extensions.customParamDefault",
    "settings.extensions.customParamPlaceholder",
    "settings.extensions.customParamRequired",
    "settings.extensions.customParamFlag",
    "settings.extensions.customParamType.text",
    "settings.extensions.customParamType.number",
    "settings.extensions.customParamType.boolean",
    "settings.extensions.customParamType.select",
    "settings.extensions.customParamType.path",
    "settings.extensions.customParamOptions",
    "settings.extensions.customParamInvalidId",
    "settings.extensions.customParamDuplicateId",
    "settings.extensions.customParamInvalidFlag",
    "settings.extensions.customParamSelectNoOptions",
    "settings.extensions.customParamInvalidDefault",
  ] as const;
  const en = createTranslator("en");
  const zh = createTranslator("zh");
  for (const key of keys) {
    const english = en(key);
    const chinese = zh(key);
    assert.ok(english.length > 0 && english !== key, `${key} missing in en`);
    assert.ok(chinese.length > 0 && chinese !== key, `${key} missing in zh`);
    assert.notEqual(english, chinese, `${key} must be translated, not copied`);
  }
});
