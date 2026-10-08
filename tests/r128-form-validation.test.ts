// R128 · the last group of user-visible English sentences the R124 census
// found: the custom-integration / local-connection form's eleven validation
// sentences (`install.rs`), the launcher's three external-run refusals
// (`run.rs`) and the open-URL refusal (`actions.rs`).
//
// The shape is the one R127 established: the backend answers with a stable
// dictionary key, the panel translates it behind `isMessageKey`, and an unknown
// string stays raw. The two sentences that carry a value (`cannotDeriveCommand`,
// `openNonWebUrl`) arrive as `key:<value>` and the value fills a `{stem}`/`{url}`
// slot; the three run refusals ride the run-error family's `run_…:{json}` wire
// (`run-errors.ts`) and fill `{id}`/`{reason}`/`{error}`.
//
// Every scanned literal is assembled from fragments, and the last test proves
// this guard does not spell the strings it looks for.
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

const root = new URL("../", import.meta.url);
const read = (path: string) => readFile(new URL(path, root), "utf8");
const count = (haystack: string, needle: string) => haystack.split(needle).length - 1;

const INSTALL = "src-tauri/src/extensions/" + "install.rs";
const RUN = "src-tauri/src/extensions/" + "run.rs";
const ACTIONS = "src-tauri/src/commands/" + "actions.rs";
const RUN_ERROR_RS = "src-tauri/src/extensions/" + "run_error.rs";
const I18N = "src/" + "i18n.ts";
const PANEL = "src/" + "ExtensionsPanel.tsx";
const RUN_ERRORS = "src/extensions/" + "run-errors.ts";
const R127 = "tests/r127-form-" + "errors.test.ts";

// Assembled so this guard does not spell the strings it scans for.
const FORM = "settings.extensions." + "form.";
const OPEN_URL = "settings.extensions." + "openNonWebUrl";
const RUN_ERROR = "settings.extensions." + "runError";
const runKey = (name: string) => RUN_ERROR + name;
const runLiteral = (name: string) => "run_" + name;

/** The eleven form sentences, keyed by their short name. */
const OLD_FORM: Record<string, string> = {
  nameLength: "Custom integration name must " + "contain 1 to 80 characters",
  commandShape:
    "Command must start with a letter or number " +
    "and contain only lowercase letters, numbers, hyphens, or underscores",
  platformRequired: "Select at least one " + "supported platform",
  modeInvalid: "Custom integration mode must be " + "executable or script",
  scriptEmpty: "Custom provider script " + "cannot be empty",
  scriptLanguageInvalid: "Script language is only valid " + "for script integrations",
  discoveredNameLength: "Discovered tool name must " + "contain 1 to 80 characters",
  cannotDeriveCommand: "Cannot derive a Floter command " + "from",
  packageKeywordMissing: "package.json is missing " + "the floter-extension keyword",
  localDistributionRequired: "Local connections must declare " + "distribution.type = local",
  localRuntimeRequired: "Local connection requires a system " + "or script runtime manifest",
};

/**
 * The eleven sites, pinned by 1-based line. Every replacement is line-neutral
 * except the derive refusal, whose longer `format!` rustfmt wraps onto four
 * lines — so the sites after it sit two lines lower than the R124 census.
 * R153 added 7 lines to `CustomIntegrationDefinition` (the `description` field
 * and its doc) and 1 more line to the definition's read-back, so the six sites
 * before the read-back sit 7 lines lower and the five after it sit 8 lower
 * than the R128 census. The three run refusals moved 7 lines lower (R153's
 * comment on the number branch).
 */
const FORM_SITES: Array<[number, string]> = [
  [250, "nameLength"],
  [263, "commandShape"],
  [268, "platformRequired"],
  [273, "modeInvalid"],
  [307, "scriptEmpty"],
  [310, "scriptLanguageInvalid"],
  [1173, "discoveredNameLength"],
  [1182, "cannotDeriveCommand"],
  [2481, "packageKeywordMissing"],
  [2502, "localDistributionRequired"],
  [2508, "localRuntimeRequired"],
];

/** The three run refusals' English, keyed by their short name. */
const OLD_RUN: Record<string, string> = {
  integration_disabled: "Integration {id} " + "is disabled",
  integration_broken: "Integration {id} " + "is broken: {reason}",
  task_failed: "Run task " + "failed: {error}",
};

/** The three run refusals, pinned by 1-based line. */
const RUN_SITES: Array<[number, string]> = [
  [437, "integration_disabled"],
  [440, "integration_broken"],
  [716, "task_failed"],
];

test("the eleven form sentences are keyed, not spelled in English", async () => {
  const source = await read(INSTALL);
  const lines = source.split("\n");
  for (const [line, suffix] of FORM_SITES) {
    assert.ok(
      lines[line - 1].includes(FORM + suffix),
      `install.rs:${line} must carry \`${FORM + suffix}\``,
    );
    assert.equal(
      count(source, OLD_FORM[suffix]),
      0,
      `install.rs must no longer spell \`${OLD_FORM[suffix]}\``,
    );
  }
});

test("the launcher's three refusals and the open-URL refusal are keyed too", async () => {
  const runSource = await read(RUN);
  const run = runSource.split("\n");
  for (const [line, name] of RUN_SITES) {
    assert.ok(
      run[line - 1].includes("run_error::" + name),
      `run.rs:${line} must build its refusal through run_error::${name}`,
    );
    assert.equal(
      count(runSource, OLD_RUN[name]),
      0,
      `run.rs must no longer spell \`${OLD_RUN[name]}\``,
    );
  }
  const actionsSource = await read(ACTIONS);
  const actions = actionsSource.split("\n");
  assert.ok(
    actions[45].includes(OPEN_URL + ":{url}"),
    `actions.rs:46 must carry \`${OPEN_URL}:{url}\``,
  );
  assert.equal(
    count(actionsSource, "Refusing to open a non-web " + "URL"),
    0,
    "actions.rs must no longer spell the refusal sentence",
  );
});

test("the run refusals are built by the family module, not inline", async () => {
  const source = await read(RUN_ERROR_RS);
  for (const [, name] of RUN_SITES) {
    const literal = runLiteral(name);
    assert.ok(
      source.includes(`pub const ${literal.toUpperCase()}: &str = "${literal}";`),
      `run_error.rs must declare the ${literal} constant`,
    );
    assert.ok(
      source.includes(`fn ${name}(`),
      `run_error.rs must expose the ${name} builder`,
    );
  }
});

test("every new key sits in both dictionaries, exactly once each", async () => {
  const i18n = await read(I18N);
  const keys = [
    ...FORM_SITES.map(([, suffix]) => FORM + suffix),
    OPEN_URL,
    runKey("IntegrationDisabled"),
    runKey("IntegrationBroken"),
    runKey("TaskFailed"),
  ];
  assert.equal(keys.length, 15, "this round adds fifteen keys");
  for (const key of keys) {
    assert.equal(count(i18n, `"${key}":`), 2, `${key} must have an English and a Chinese entry`);
  }
  // The English values are the sentences that used to be hardcoded, kept
  // verbatim — so a translation can never quietly reword the backend's fact.
  for (const suffix of Object.keys(OLD_FORM)) {
    const entries = i18n.split("\n").filter((line) => line.includes(`"${FORM + suffix}":`));
    assert.equal(entries.length, 2, `${FORM + suffix} must carry a value on both sides`);
    assert.notEqual(entries[0], entries[1], `${FORM + suffix} languages must not share one string`);
  }
});

test("the value-carrying sentences splice their value instead of gluing it", async () => {
  const i18n = await read(I18N);
  const valueOf = (key: string) =>
    i18n.split("\n").find((line) => line.includes(`"${key}":`)) ?? "";
  assert.ok(
    valueOf(FORM + "cannotDeriveCommand").includes("{stem}"),
    "the derive refusal must interpolate the stem",
  );
  assert.ok(valueOf(runKey("IntegrationDisabled")).includes("{id}"), "the disabled refusal names the id");
  assert.ok(valueOf(runKey("IntegrationBroken")).includes("{reason}"), "the broken refusal names the reason");
  assert.ok(valueOf(runKey("TaskFailed")).includes("{error}"), "the task failure names the error");
  assert.ok(valueOf(OPEN_URL).includes("{url}"), "the open-URL refusal names the URL");
  // The backend really delivers those values: the two plain keyed sentences
  // splice after the colon, and the run family carries a JSON payload.
  assert.ok(
    (await read(INSTALL)).includes(FORM + "cannotDeriveCommand:{stem}"),
    "install.rs must send the stem with the key",
  );
  assert.ok(
    (await read(ACTIONS)).includes(OPEN_URL + ":{url}"),
    "actions.rs must send the URL with the key",
  );
  const runErrorRs = await read(RUN_ERROR_RS);
  for (const field of ["extension_id", "reason", "error"]) {
    assert.ok(runErrorRs.includes(field), `run_error.rs must carry the ${field} payload field`);
  }
});

test("the consumers translate the keyed sentences instead of painting the key", async () => {
  const panel = await read(PANEL);
  // The one-value branch is wired into the panel's error reader.
  assert.ok(panel.includes("KEYED_VALUE_PARAMS"), "the panel must map a keyed value to its slot");
  assert.ok(panel.includes(FORM + "cannotDeriveCommand"), "the derive refusal must be claimed");
  assert.ok(panel.includes(OPEN_URL), "the open-URL refusal must be claimed");
  // The run family's mapper claims the three new keys, and the launcher reads
  // it before falling back to the raw message.
  const runErrors = await read(RUN_ERRORS);
  for (const [, name] of RUN_SITES) {
    assert.ok(runErrors.includes(`"${runLiteral(name)}"`), `run-errors.ts must claim ${name}`);
  }
});

test("the R127 anchors this round builds on are still there", async () => {
  const guard = await read(R127);
  assert.ok(guard.includes("pickerClosed."), "the R127 picker key anchor must remain");
  assert.ok(guard.includes("isMessageKey(message)"), "the R127 gate anchor must remain");
  assert.ok(guard.includes("errorMessage(nextError, t)"), "the R127 wiring anchor must remain");
});

test("this guard assembles its literals, it does not spell them", async () => {
  const self = await read("tests/r128-form-" + "validation.test.ts");
  for (const literal of [
    ...Object.values(OLD_FORM),
    ...Object.values(OLD_RUN),
    ...FORM_SITES.map(([, suffix]) => FORM + suffix),
    OPEN_URL,
    runKey("IntegrationDisabled"),
    runKey("IntegrationBroken"),
    runKey("TaskFailed"),
    ...RUN_SITES.map(([, name]) => runLiteral(name)),
  ]) {
    assert.ok(!self.includes(literal), `the guard must not spell: ${literal}`);
  }
});
