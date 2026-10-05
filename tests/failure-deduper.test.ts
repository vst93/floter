// Per-key failure deduplication (src/failure-deduper.ts).
//
// R101 · this policy used to be pinned beside the base-plugin registry in
// `tests/builtin-plugins.test.ts`, because the two subjects shared one module.
// The module was split into `src/builtin-plugins.ts` and
// `src/failure-deduper.ts`, so the cases that drive the deduper moved here.
//
// The policy is the app's answer to an automatic failure — a 2s poll against a
// backend that stays down, a `floter://` link a hostile page retries in a loop
// — which would otherwise raise one toast per attempt forever. It is scoped per
// key so a *user gesture* failure (a copy, a delete) still reports each time the
// user asks.
import assert from "node:assert/strict";
import test from "node:test";

import { FAILURE_NOTIFY_DEDUP_MS, createFailureDeduper } from "../src/failure-deduper.ts";

test("five consecutive automatic failures raise exactly one toast (30s dedupe per key)", () => {
  // Drive the shared dedupe policy directly: an automatic trigger calls this
  // once per failed attempt, and the burst below is 20s. Only the first may
  // paint; the others must be swallowed.
  assert.ok(
    FAILURE_NOTIFY_DEDUP_MS >= 30_000,
    "the window must dwarf a 2s poll — at 30s an automatic failure can earn at most one toast",
  );
  const deduper = createFailureDeduper();
  const raised: number[] = [];
  let now = 1_000_000;
  for (let attempt = 0; attempt < 5; attempt += 1) {
    if (deduper.allow("some.loadFailed", now)) raised.push(now);
    now += 2000; // the poll interval
  }
  assert.deepEqual(raised, [1_000_000], "the outage must be one toast, not five");
  // The window is per key: a *different* failure in the middle of the outage
  // is still news and is not swallowed by the first key.
  assert.equal(deduper.allow("some.copyFailed", now), true, "dedupe must be per message key");
  // Recovery re-arms: failure → success → failure is two toasts, not one.
  deduper.clear("some.loadFailed");
  assert.equal(deduper.allow("some.loadFailed", now), true, "a success in between makes the relapse news again");
  // And the window really elapses: the next failure after it is announced.
  const later = createFailureDeduper();
  assert.equal(later.allow("k", 0), true);
  assert.equal(later.allow("k", FAILURE_NOTIFY_DEDUP_MS - 1), false);
  assert.equal(later.allow("k", FAILURE_NOTIFY_DEDUP_MS), true);
});
