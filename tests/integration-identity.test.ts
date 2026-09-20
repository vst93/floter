// R9-3 · the integration id is minted once at creation and never moves.
//
// The user report was direct: "集成 id 没有动态变化，不行就在创建的时间随机产生，
// 后面保持不变". Before this round the id was derived from the *command* id
// (`local.${slug}` in the frontend, `local.{command}` in the backend, plus a
// `.2`/`.3` suffix on collision). That made the identity track an editable
// field, so renaming the command left the id describing a command that no
// longer existed.
//
// The fix puts allocation in exactly one place — the backend create sink — and
// makes the update path ignore the request's id entirely. These tests pin the
// frontend half (no derivation, no authored id) and the Rust half (one
// allocator, an ignoring update) at the source level, because neither the React
// drawer nor the Rust command runs in this runner.

import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";
import { createTranslator } from "../src/i18n.ts";

const root = new URL("../", import.meta.url);
const read = (path: string) => readFile(new URL(path, root), "utf8");
const stripJsComments = (source: string) =>
  source.replace(/\/\*[\s\S]*?\*\//g, "").replace(/^\s*\/\/.*$/gm, "");

const panel = () => read("src/ExtensionsPanel.tsx").then(stripJsComments);
const drawer = () =>
  read("src/extensions/CustomIntegrationDrawer.tsx").then(stripJsComments);
const install = () => read("src-tauri/src/extensions/install.rs");

// ── 1 · the frontend never mints or derives an id ─────────────────────────

test("the panel derives no id from the command slug", async () => {
  const source = await panel();
  // Mutation: restore `id: current.id === DEFAULT ? `local.${slug}` : current.id`
  // in `chooseToolCandidate` and this fails — the id would follow the command.
  assert.ok(
    !/`local\.\$\{/.test(source),
    "no `local.${...}` derivation may exist in the panel",
  );
  assert.ok(
    !/id:\s*current\.id === DEFAULT_CUSTOM_INTEGRATION\.id/.test(source),
    "the id must not be conditionally derived from the draft",
  );
});

test("the create payload sends an empty placeholder, not a frontend id", async () => {
  const source = await panel();
  // Mutation: send `customIntegration.id` (or a derived value) and this fails.
  assert.match(
    source,
    /id: editingCustomId \?\? ""/,
    "the request id is backend-owned: empty on create, the addressed id on edit",
  );
});

test("the default draft carries no pre-baked id", async () => {
  const source = await panel();
  const at = source.indexOf("const DEFAULT_CUSTOM_INTEGRATION");
  const body = source.slice(at, source.indexOf("};", at));
  assert.match(body, /id: ""/, "the draft id is empty; the backend supplies it");
  assert.ok(
    !/id: "local\./.test(body),
    "the draft must not ship a local.* literal",
  );
});

test("the drawer shows the id only when editing, and never as a field", async () => {
  const source = await drawer();
  // Mutation: turn the caption back into `<input value={integration.id}>` and
  // this fails — the id would become an authored field again.
  assert.ok(
    !/value=\{integration\.id\}/.test(source),
    "the id is never an editable input",
  );
  assert.match(
    source,
    /\{editingId && <p className="extension-custom-form__derived">/,
    "the id caption is edit-only (there is no id on create)",
  );
  assert.match(source, /<code>\{integration\.id\}<\/code>/, "the caption shows the real id");
});

// ── 2 · the backend is the single allocator ───────────────────────────────

test("exactly one function mints a custom integration id", async () => {
  const source = await install();
  // The only `format!("local.…")` in the module is the allocator. A second one
  // anywhere would be the second source of truth this round removes.
  const mintSites = [...source.matchAll(/format!\("local\.\{/g)].length;
  assert.equal(mintSites, 1, "one allocator, no derived-id fallback");
  assert.match(
    source,
    /fn new_custom_integration_id\(\) -> String \{\n\s*let short = uuid::Uuid::new_v4\(\)\.simple\(\)\.to_string\(\);\n\s*format!\("local\.\{\}", &short\[\.\.8\]\)/,
    "the id is `local.<8 hex>` from a v4 uuid",
  );
});

test("the old derived-suffix collision scheme is gone", async () => {
  const source = await install();
  assert.ok(
    !/local\.dup/.test(source),
    "the `.2`/`.3` suffix allocator must be deleted",
  );
  assert.ok(
    !/format!\("\{base_id\}\.\{\}"/.test(source),
    "no base-id-plus-counter id may survive",
  );
  // Collisions regenerate instead.
  assert.match(source, /fn is_id_collision\(/, "collision detection is one predicate");
  assert.match(source, /create_custom_integration_with\(state, request, new_custom_integration_id\)/);
});

test("the update path ignores the id in the request", async () => {
  const source = await install();
  const at = source.indexOf("pub async fn update_custom_integration(");
  assert.notEqual(at, -1, "the update entry point exists");
  const body = source.slice(at, source.indexOf("\n}\n", at));
  // Mutation: restore the `if request.id != extension_id { return Err(...) }`
  // guard and this fails — the request id would be consulted again.
  assert.ok(
    !/request\.id\.trim\(\)/.test(body),
    "the request id must not be read on the update path",
  );
  assert.match(
    body,
    /create_custom_integration_locked\(state, extension_id, request\)/,
    "the addressed entry's id is the only id an update writes",
  );
});

// ── 3 · the copy is symmetric and no longer claims derivation ─────────────

test("the derived-id string is retired from both dictionaries", async () => {
  const source = await read("src/i18n.ts");
  assert.ok(
    !source.includes("customIdDerived"),
    "the \"generated from the command id\" copy must be deleted from both languages",
  );
  // The id label itself stays, and is translated in both languages.
  const en = createTranslator("en");
  const zh = createTranslator("zh");
  assert.notEqual(en("settings.extensions.customId"), "settings.extensions.customId");
  assert.notEqual(zh("settings.extensions.customId"), "settings.extensions.customId");
  assert.notEqual(en("settings.extensions.customId"), zh("settings.extensions.customId"));
});
