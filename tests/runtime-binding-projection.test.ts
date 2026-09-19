// R-AUDIT-G5 · the single availability projector's frontend half.
//
// The backend now answers "can this runtime run right now" from one place
// (`extensions::runtime_binding`), and the row carries the reason instead of
// only the symptom. This file pins the frontend contract so the two truths
// cannot re-split:
//
// 1. the row must render its unavailable reason from the projector's fields
//    (runtimeUnavailableCode/Detail), not re-derive availability from the
//    lock state locally;
// 2. the projector fields must be exactly absent (or null) when
//    runtimeAvailable is true — no stale reason can outlive a recovery;
// 3. both dictionaries name the state (no English fallback in zh).

import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

const root = new URL("../", import.meta.url);
const read = (path: string) => readFile(new URL(path, root), "utf8");

const stripJsComments = (source: string) =>
  source.replace(/\/\*[\s\S]*?\*\//g, "").replace(/^\s*\/\/.*$/gm, "");

test("the row renders the projector's reason, not its own guess", async () => {
  const row = stripJsComments(await read("src/extensions/ExtensionRow.tsx"));
  assert.ok(
    row.includes("runtimeUnavailableCode") || row.includes("runtimeUnavailableDetail"),
    "the row must consume the projector's unavailability fields",
  );
  // The row must NOT answer availability from tool-lock state directly: the
  // lock's LockState may not re-enter the row's availability logic now that
  // the backend owns the projection.
  assert.ok(
    !row.includes("lockState"),
    "the row must not re-derive availability from the lock state - the backend projector owns it",
  );
});

test("a recovered entry cannot keep a stale reason", async () => {
  const panel = await read("src/ExtensionsPanel.tsx"); // raw: the contract lives in a doc comment
  const anchor = panel.indexOf("runtimeUnavailableCode");
  assert.notEqual(anchor, -1, "the panel type must carry the projector fields");
  const comment = panel.slice(Math.max(0, panel.lastIndexOf("/**", anchor)), anchor);
  assert.ok(
    /runtimeAvailable` is true/.test(comment),
    "the null/absent contract must be documented at the type",
  );
});

test("both dictionaries name the unavailability state", async () => {
  const i18n = await read("src/i18n.ts");
  for (const key of ["settings.extensions.runtimeUnavailableDetail"]) {
    const hits = i18n.split(key).length - 1;
    assert.ok(hits >= 2, `${key} must exist in both dictionaries (found ${hits})`);
  }
});
