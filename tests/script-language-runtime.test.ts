// R9-1 · the script-language vocabulary, the derived integration ID, and the
// inline toolchain status line.
//
// Three things changed on the frontend this round, and each has a mutation
// that turns it back into the old behaviour:
//
//   1. the ID field became a derived caption — a user never authors it;
//   2. the language list grew to eight and moved into one module, so the
//      picker, the template and the export extension cannot disagree;
//   3. the language picker gained a status line that distinguishes "not
//      checked yet" from "checked and missing".
//
// The pure projection is driven directly; the JSX facts are read off the
// source, because the panel is a 2000-line component the node runner does not
// render.

import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";
import { createTranslator } from "../src/i18n.ts";
import {
  SCRIPT_LANGUAGES,
  scriptExtension,
  scriptLanguage,
  scriptRuntimeStatus,
  scriptTemplate,
  scriptTemplates,
  type ScriptRuntimeCheck,
} from "../src/extensions/script-languages.ts";

const root = new URL("../", import.meta.url);
const read = (path: string) => readFile(new URL(path, root), "utf8");
const stripJsComments = (source: string) =>
  source.replace(/\/\*[\s\S]*?\*\//g, "").replace(/^\s*\/\/.*$/gm, "");

// ── 1 · the language vocabulary is one module ─────────────────────────────

test("every language the backend accepts is offered, with an extension", () => {
  const ids = SCRIPT_LANGUAGES.map((language) => language.id);
  assert.deepEqual(ids, ["js", "shell", "powershell", "python", "ruby", "php", "go", "rust"]);
  for (const language of SCRIPT_LANGUAGES) {
    assert.ok(language.label.length > 0, `${language.id} has a label`);
    assert.match(language.extension, /^[a-z0-9]+$/, `${language.id} extension is bare`);
    assert.ok(language.template.trim().length > 0, `${language.id} has a runnable template`);
    assert.ok(language.toolchain.length > 0, `${language.id} names its toolchain`);
  }
  // The compiled pair is exactly Go and Rust — the same classifier the Rust
  // enum applies (`is_compiled`).
  assert.deepEqual(
    SCRIPT_LANGUAGES.filter((language) => language.compiled).map((language) => language.id),
    ["go", "rust"],
  );
});

test("the template and the export extension come from the same table", () => {
  // Mutation: reintroduce the nested ternary in ExtensionsPanel's export path
  // (`scriptLanguage === "shell" ? "sh" : ... : "js"`) and the python/php
  // exports silently get `.js`, which this asserts against.
  assert.equal(scriptExtension("python"), "py");
  assert.equal(scriptExtension("php"), "php");
  assert.equal(scriptExtension("go"), "go");
  assert.equal(scriptTemplate("go"), scriptLanguage("go").template);
  assert.equal(scriptTemplates().length, SCRIPT_LANGUAGES.length);
  // An unknown id falls back to a usable editor rather than throwing.
  assert.equal(scriptLanguage("cobol").id, "js");
});

test("the panel and the drawer both read the shared table", async () => {
  const panel = stripJsComments(await read("src/ExtensionsPanel.tsx"));
  const drawer = stripJsComments(await read("src/extensions/CustomIntegrationDrawer.tsx"));
  // The export path uses the shared helper…
  assert.match(panel, /scriptExtension\(customIntegration\.scriptLanguage\)/);
  // …and neither file re-declares the option list or a template record.
  for (const [name, source] of [["panel", panel], ["drawer", drawer]] as const) {
    assert.ok(
      !/"powershell":\s*"#!/.test(source),
      `${name} must not carry its own template record`,
    );
    assert.ok(
      !/<option value="powershell">/.test(source),
      `${name} must not carry an inline option list`,
    );
  }
  // The drawer renders every language from the table.
  assert.match(drawer, /SCRIPT_LANGUAGES\.map\(/);
});

// ── 2 · the ID is derived, not authored ───────────────────────────────────

test("the ID is a derived caption, not a form field", async () => {
  const drawer = stripJsComments(await read("src/extensions/CustomIntegrationDrawer.tsx"));
  // Mutation: put the `<input … value={integration.id} …>` back and this fails
  // — the ID would be a first-class field the user is asked to author again.
  assert.ok(
    !/value=\{integration\.id\}/.test(drawer),
    "the ID must not be an editable input",
  );
  assert.match(
    drawer,
    /extension-custom-form__derived/,
    "the ID is rendered as the derived caption",
  );
  // The caption carries the value and, on create, says it was generated.
  assert.match(drawer, /<code>\{integration\.id\}<\/code>/);
  assert.match(drawer, /customIdDerived/);
  // The command id remains the only thing the derivation reads.
  assert.match(drawer, /customCommand/);
});

test("the derivation still runs on create and is frozen on edit", async () => {
  const panel = stripJsComments(await read("src/ExtensionsPanel.tsx"));
  // The name → slug → `local.{slug}` derivation is unchanged…
  assert.match(panel, /`local\.\$\{slug\}`/, "the id is still derived from the slug");
  // …and the drawer shows the value read-only on edit (the backend refuses a
  // changed id: `Custom integration ID cannot be changed after creation`).
  const drawer = stripJsComments(await read("src/extensions/CustomIntegrationDrawer.tsx"));
  assert.match(drawer, /\{!editingId && <em>/, "the \"generated\" note is create-only");
});

// ── 3 · the inline toolchain status ───────────────────────────────────────

const check = (overrides: Partial<ScriptRuntimeCheck>): ScriptRuntimeCheck => ({
  available: false,
  path: null,
  version: null,
  versionOutput: null,
  compiled: false,
  candidates: ["node"],
  ...overrides,
});

test("an unchecked language is 'checking', never 'missing'", () => {
  // The whole reason the state is a three-way union: `null` means "we have not
  // looked", and painting it as missing would accuse the machine of a fact
  // nobody established.
  assert.deepEqual(scriptRuntimeStatus("python", null), { state: "checking" });
  assert.deepEqual(scriptRuntimeStatus("python", undefined), { state: "checking" });
});

test("a resolved toolchain reports its name, version and path", () => {
  const status = scriptRuntimeStatus(
    "python",
    check({
      available: true,
      path: "/usr/bin/python3.12",
      version: "3.12.4",
      versionOutput: "Python 3.12.4",
    }),
  );
  assert.equal(status.state, "available");
  assert.deepEqual(status, {
    state: "available",
    name: "python3.12",
    version: "3.12.4",
    path: "/usr/bin/python3.12",
    compiled: false,
  });
  // A toolchain that answers nothing is still available: no version is not the
  // same as no runtime.
  const bare = scriptRuntimeStatus("python", check({ available: true, path: "/usr/bin/python3" }));
  assert.equal(bare.state, "available");
  assert.equal(bare.state === "available" && bare.version, null);
});

test("a missing toolchain names what was searched", () => {
  const status = scriptRuntimeStatus(
    "python",
    check({ candidates: ["python3", "python", "python3.12"] }),
  );
  assert.equal(status.state, "missing");
  assert.deepEqual(status.state === "missing" && status.names, [
    "python3",
    "python",
    "python3.12",
  ]);
  // A probe that failed before the backend could name its candidates falls back
  // to the language's own toolchain name, so the line never renders blank.
  const fallback = scriptRuntimeStatus("ruby", check({ candidates: [] }));
  assert.deepEqual(fallback.state === "missing" && fallback.names, ["ruby"]);
});

test("the drawer renders all three states and the compiled note", async () => {
  const drawer = stripJsComments(await read("src/extensions/CustomIntegrationDrawer.tsx"));
  assert.match(drawer, /status\.state === "checking"/);
  assert.match(drawer, /status\.state === "missing"/);
  assert.match(drawer, /extension-custom-runtime--ready/);
  assert.match(drawer, /customScriptCompiled/);
  // The check is a prop from the panel — the drawer never invokes IPC itself.
  assert.ok(
    !/extensions_script_runtime_check/.test(drawer),
    "the drawer receives the check result; the panel owns the probe",
  );
  const panel = stripJsComments(await read("src/ExtensionsPanel.tsx"));
  assert.match(panel, /invoke<ScriptRuntimeCheck>\("extensions_script_runtime_check"/);
});

test("the probe is cached per language and reset per drawer session", async () => {
  const panel = stripJsComments(await read("src/ExtensionsPanel.tsx"));
  // The cache is a ref, not state, so a re-selection reads it synchronously
  // and cannot fire a second PATH scan (the mutation this guards: a per-render
  // probe, i.e. one per keystroke).
  assert.match(panel, /runtimeChecksRef = useRef\(new Map/);
  assert.match(panel, /runtimeChecksPending = useRef\(new Map/);
  assert.match(panel, /runtimeChecksRef\.current\.get\(language\)/);
  // A fresh drawer clears both, because the machine may have changed.
  assert.match(panel, /runtimeChecksRef\.current\.clear\(\)/);
  assert.match(panel, /runtimeChecksPending\.current\.clear\(\)/);
});

test("a missing toolchain warns once on save but never blocks it", async () => {
  const panel = stripJsComments(await read("src/ExtensionsPanel.tsx"));
  const body = panel.slice(panel.indexOf("const createCustomIntegration = async"));
  const upToBusy = body.slice(0, body.indexOf("setBusy({ id: customIntegration.id"));
  assert.match(upToBusy, /probeScriptRuntime\(customIntegration\.scriptLanguage\)/);
  assert.match(upToBusy, /onNotify\("warning"/);
  assert.ok(
    !/return;[\s\S]{0,40}check\.available/.test(upToBusy),
    "an unavailable toolchain must not return early — the save still runs",
  );
  // The warning kind exists on the stack, not just in the call.
  const toastState = await read("src/toast-state.ts");
  assert.match(toastState, /ToastKind = "error" \| "success" \| "warning"/);
  assert.match(toastState, /warning: \d+/);
  const stack = await read("src/components/ToastStack.tsx");
  assert.match(stack, /toast\.kind === "warning"/);
  const css = await read("src/styles/extensions.css");
  assert.match(css, /\.app-toast--warning\s*\{/);
});

// ── 4 · the copy is translated, and the ID label changed ──────────────────

test("the new strings exist in both dictionaries", async () => {
  const en = createTranslator("en");
  const zh = createTranslator("zh");
  for (const key of [
    "settings.extensions.customIdDerived",
    "settings.extensions.customScriptRuntimeChecking",
    "settings.extensions.customScriptRuntimeMissing",
    "settings.extensions.customScriptRuntimeSaveWarning",
    "settings.extensions.customScriptCompiled",
  ] as const) {
    assert.ok(en(key).length > 0, `${key} has an English string`);
    assert.notEqual(en(key), key, `${key} is not a raw key`);
    assert.notEqual(zh(key), key, `${key} is translated in zh`);
    assert.notEqual(zh(key), en(key), `${key} is not left in English`);
  }
  // The two interpolating strings keep their placeholders in both languages.
  assert.match(en("settings.extensions.customScriptRuntimeMissing"), /\{names\}/);
  assert.match(zh("settings.extensions.customScriptRuntimeMissing"), /\{names\}/);
  assert.match(en("settings.extensions.customScriptRuntimeSaveWarning"), /\{language\}/);
  assert.match(zh("settings.extensions.customScriptRuntimeSaveWarning"), /\{language\}/);
});

test("the ID label no longer claims to be a bare acronym", () => {
  // The old value was the byte-identical "ID", which is why the i18n sweep had
  // it on its allowed-identical list. It is now a real label in both languages.
  const en = createTranslator("en");
  const zh = createTranslator("zh");
  assert.notEqual(en("settings.extensions.customId"), "ID");
  assert.notEqual(zh("settings.extensions.customId"), "ID");
  assert.notEqual(en("settings.extensions.customId"), zh("settings.extensions.customId"));
});
