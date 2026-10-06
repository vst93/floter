// R113 · the plugin configuration write path (R111 finding C).
//
// R29's overlay painted a change and fired its write off; the write swallowed
// its own failure, so a refused write left the control showing a value the
// backend never stored *and* advanced the committed snapshot as if it had.
// R113 gives the write a result and the overlay a rollback.
//
// This suite pins both halves: the write reports a rejection instead of hiding
// it, a refusal rolls the painted values back to the last committed snapshot,
// and a success advances that snapshot instead.
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";
import {
  configChangeOutcome,
  writeConfigChange,
  type ConfigValues,
} from "../src/plugins/config-persist.ts";

const root = new URL("../", import.meta.url);
const read = (path: string) => readFile(new URL(path, root), "utf8");
const count = (haystack: string, needle: string) => haystack.split(needle).length - 1;

// Assembled, not spelled (see r113-error-boundary.test.ts).
const SWALLOW = ".catch(() => " + "undefined)";

test("a refused write reports the rejection instead of swallowing it", async () => {
  const invoke = () => Promise.reject(new Error("disk full"));
  assert.equal(await writeConfigChange(invoke), false);
});

test("an accepted write reports acceptance and really runs the write", async () => {
  let ran = false;
  const invoke = () => {
    ran = true;
    return Promise.resolve({ max_items: 200 });
  };
  assert.equal(await writeConfigChange(invoke), true);
  assert.equal(ran, true, "the write must actually run");
});

test("a refused write rolls the painted values back to the committed snapshot", async () => {
  const committed: ConfigValues = { enabled: true, max_items: 100 };
  const next: ConfigValues = { enabled: true, max_items: 200 };
  const invoke = () => Promise.reject(new Error("disk full"));
  const outcome = configChangeOutcome(next, committed, await writeConfigChange(invoke));
  assert.deepEqual(outcome, {
    kind: "rollback",
    values: committed,
    feedback: "settings.saveFailed",
  });
  // Identity, not a copy: the rollback hands back the snapshot the overlay
  // already holds, so the control cannot drift from it.
  assert.ok(outcome.kind === "rollback" && outcome.values === committed);
});

test("an accepted write advances the committed snapshot to the painted values", async () => {
  const committed: ConfigValues = { enabled: true, max_items: 100 };
  const next: ConfigValues = { enabled: true, max_items: 200 };
  const outcome = configChangeOutcome(
    next,
    committed,
    await writeConfigChange(() => Promise.resolve()),
  );
  assert.deepEqual(outcome, { kind: "commit", committed: next });
  assert.ok(outcome.kind === "commit" && outcome.committed === next);
});

test("the overlay wires the outcome: refusal restores, acceptance commits", async () => {
  const overlay = await read("src/plugins/PluginConfigOverlay.tsx");
  const persistStart = overlay.indexOf("const persist = useCallback");
  assert.ok(persistStart > -1, "persist must exist");
  const persistEnd = overlay.indexOf("const handleChange = useCallback", persistStart);
  assert.ok(persistEnd > persistStart, "handleChange must follow persist");
  const persistBody = overlay.slice(persistStart, persistEnd);
  const handleEnd = overlay.indexOf("const runAction", persistEnd);
  const handleBody = overlay.slice(persistEnd, handleEnd);

  // The write path no longer swallows its own failure.
  assert.equal(count(persistBody, SWALLOW), 0, "persist must not swallow the write failure");
  assert.ok(persistBody.includes("writeConfigChange"), "persist must route through the reporting write");
  // The refusal branch restores the painted values and reports; the acceptance
  // branch advances the committed snapshot.
  assert.ok(handleBody.includes("configChangeOutcome"), "handleChange must consult the outcome");
  assert.ok(handleBody.includes("setValues(outcome.values)"), "a refusal must restore the committed values");
  assert.ok(handleBody.includes("setPersistFailed(true)"), "a refusal must report");
  assert.ok(
    handleBody.includes("committed.current = outcome.committed"),
    "an accepted write advances the committed snapshot",
  );
  // Only the read fallback may swallow, and the write path is not it.
  assert.ok(
    count(overlay, SWALLOW) <= 1,
    "no swallow may re-enter the write path (one read fallback is allowed)",
  );
});

test("the action failure line keeps its OK shape (asserted, not changed)", async () => {
  // `controls.tsx`'s action control already turns a refused run into its own
  // failure line and `runAction` already resolves a boolean — R113 only pins
  // that shape, it does not change it.
  const controls = await read("src/plugins/controls.tsx");
  assert.ok(
    controls.includes("void onRun?.(field.key).then((ok) => setFailed(ok === false))"),
    "the action control still turns a refused run into its failure line",
  );
  const overlay = await read("src/plugins/PluginConfigOverlay.tsx");
  const runStart = overlay.indexOf("const runAction = useCallback");
  assert.ok(runStart > -1, "runAction must exist");
  const runBody = overlay.slice(runStart, overlay.indexOf("if (!schema) return null;", runStart));
  assert.ok(runBody.includes("try {"), "runAction still try/catches the invoke");
  assert.ok(runBody.includes("return false"), "and resolves a boolean instead of rejecting");
});

test("this guard assembles its swallow token, it does not spell it", async () => {
  const self = await read("tests/r113-config-persist.test.ts");
  assert.ok(!self.includes(SWALLOW), "the guard must not spell the swallow form in its own source");
});
