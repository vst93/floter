import assert from "node:assert/strict";
import test from "node:test";
import {
  MAX_TOASTS,
  TOAST_DISMISS_MS,
  appendToast,
  removeToast,
  type AppToast,
} from "../src/toast-state.ts";

const toast = (id: number, kind: AppToast["kind"] = "success"): AppToast => ({
  id,
  kind,
  text: `toast ${id}`,
});

test("appending keeps only the newest toasts", () => {
  let toasts: AppToast[] = [];
  for (let id = 1; id <= MAX_TOASTS + 2; id += 1) {
    toasts = appendToast(toasts, toast(id));
  }
  assert.equal(toasts.length, MAX_TOASTS);
  // The oldest fell off the front; the newest is last.
  assert.deepEqual(toasts.map((t) => t.id), [3, 4, 5]);
});

test("removing drops exactly one toast and ignores unknown ids", () => {
  const toasts = [toast(1), toast(2), toast(3)];
  assert.deepEqual(removeToast(toasts, 2).map((t) => t.id), [1, 3]);
  assert.deepEqual(removeToast(toasts, 99).map((t) => t.id), [1, 2, 3]);
});

test("errors linger longer than successes", () => {
  assert.ok(TOAST_DISMISS_MS.error > TOAST_DISMISS_MS.success);
});
