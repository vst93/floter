// R11 · the readable half of an unavailable runtime binding.
//
// The report: a custom `php` script integration read
// `不可用: binding-changed Executable fingerprint changed at /opt/homebrew/bin/php`
// — the raw error code glued to an English sentence, in the Chinese UI, about a
// toolchain upgrade that had not touched the script at all.
//
// The backend now records the fact as a stable key plus a JSON payload
// (`binding-changed:{"path":…}`); this module is the only place the wording
// lives, so both dictionaries stay symmetric and neither the code nor the detail
// reaches the user untranslated.
//
// The pure projection is driven directly; the JSX facts are read off the
// source, because the panel is a component the node runner does not render.

import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";
import { createTranslator } from "../src/i18n.ts";
import {
  BINDING_DETAIL_MESSAGE_KEYS,
  bindingDetailMessage,
  failureReason,
  parseBindingDetail,
} from "../src/extensions/binding-errors.ts";

const root = new URL("../", import.meta.url);
const read = (path: string) => readFile(new URL(path, root), "utf8");
const en = createTranslator("en");
const zh = createTranslator("zh");

// ── 1 · the key/payload split ──────────────────────────────────────────────

test("a keyed backend detail splits into its code and path", () => {
  assert.deepEqual(parseBindingDetail('binding-changed:{"path":"/opt/homebrew/bin/php"}'), {
    code: "binding-changed",
    path: "/opt/homebrew/bin/php",
  });
  assert.deepEqual(parseBindingDetail('binding-missing:{"path":"/nope/tool"}'), {
    code: "binding-missing",
    path: "/nope/tool",
  });
});

test("the split is conservative so other error paths keep their own handling", () => {
  for (const detail of [
    "run_program_missing:{\"path\":\"/x\"}",
    "binding-changed:not json",
    "binding-changed:{}",
    "binding-unknown:{\"path\":\"/x\"}",
    "Executable fingerprint changed at /opt/homebrew/bin/php",
    "Provider describe exited with 7",
    "",
    null,
    undefined,
  ]) {
    assert.equal(parseBindingDetail(detail), null, `${detail} must not parse as a binding detail`);
  }
});

// ── 2 · every message names the file ───────────────────────────────────────

test("a changed executable names the file and the remedy, in both languages", () => {
  const detail = 'binding-changed:{"path":"/opt/homebrew/bin/php"}';
  const english = bindingDetailMessage(detail, en);
  assert.ok(english?.includes("/opt/homebrew/bin/php"), `${english}`);
  assert.match(english ?? "", /Re-check/);
  const chinese = bindingDetailMessage(detail, zh);
  assert.ok(chinese?.includes("/opt/homebrew/bin/php"), `${chinese}`);
  assert.match(chinese ?? "", /重新检测/);
  // The raw code never appears: the whole point of the round.
  assert.ok(!(chinese ?? "").includes("binding-changed"), `${chinese}`);
});

test("a missing executable names the file", () => {
  const detail = 'binding-missing:{"path":"/nope/tool"}';
  assert.match(bindingDetailMessage(detail, en) ?? "", /\/nope\/tool/);
  assert.match(bindingDetailMessage(detail, zh) ?? "", /\/nope\/tool/);
});

test("a detail from another path is left alone", () => {
  assert.equal(bindingDetailMessage("Provider describe exited with 7", zh), null);
  assert.equal(bindingDetailMessage(null, zh), null);
});

// ── 3 · the migration window ───────────────────────────────────────────────

test("the legacy English prose a pre-R11 build persisted is still localised", () => {
  const changed = bindingDetailMessage(
    "Executable fingerprint changed at /opt/homebrew/bin/php",
    zh,
  );
  assert.match(changed ?? "", /可执行文件已变更/);
  assert.match(changed ?? "", /\/opt\/homebrew\/bin\/php/);
  const missing = bindingDetailMessage("Executable is no longer available at /nope/tool", zh);
  assert.match(missing ?? "", /可执行文件不存在/);
});

// ── 4 · the one-line reason the UI shows ───────────────────────────────────

test("failureReason localises a binding detail instead of gluing the raw code to it", () => {
  const reason = failureReason("binding-changed", 'binding-changed:{"path":"/opt/homebrew/bin/php"}', zh);
  assert.ok(!reason.includes("binding-changed"), `${reason}`);
  assert.match(reason, /可执行文件已变更/);
});

test("failureReason still shows a translated code for a non-binding failure", () => {
  const reason = failureReason("describe-parse-failed", "Provider describe returned invalid JSON", en);
  assert.match(reason, /Provider output parsing failed/);
  assert.match(reason, /invalid JSON/);
});

// ── 5 · the dictionaries and the render sites ──────────────────────────────

test("both dictionaries name every binding detail key", async () => {
  const i18n = await read("src/i18n.ts");
  for (const key of BINDING_DETAIL_MESSAGE_KEYS) {
    const hits = i18n.split(key).length - 1;
    assert.ok(hits >= 2, `${key} must exist in both dictionaries (found ${hits})`);
  }
});

test("the drawer and the row route their failure text through the projector", async () => {
  const panel = await read("src/ExtensionsPanel.tsx");
  assert.ok(
    panel.includes("failureReason(selected.runtimeUnavailableCode") &&
      panel.includes("failureReason(selected.lastErrorCode"),
    "the drawer must localise both the unavailable and the broken reason",
  );
  // The old assembly printed the raw code in a <code> element next to the
  // backend's English sentence; that shape must not come back.
  assert.ok(
    !panel.includes("<code>{selected.runtimeUnavailableCode}</code>"),
    "the unavailable box must not print the raw error code",
  );
  const row = await read("src/extensions/ExtensionRow.tsx");
  assert.ok(
    row.includes("failureReason(extension.runtimeUnavailableCode") &&
      row.includes("failureReason(extension.lastErrorCode"),
    "the row must localise both of its failure titles",
  );
});
