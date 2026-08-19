import assert from "node:assert/strict";
import test from "node:test";
import { beginRequest, isCurrentRequest } from "../src/request-generation.ts";

test("a newer request invalidates an older response", () => {
  const current = { current: 0 };
  const first = beginRequest(current);
  const second = beginRequest(current);

  assert.equal(isCurrentRequest(current, first), false);
  assert.equal(isCurrentRequest(current, second), true);
});

test("request generations are monotonic from an existing value", () => {
  const current = { current: 9 };
  assert.equal(beginRequest(current), 10);
  assert.equal(current.current, 10);
  assert.equal(isCurrentRequest(current, 10), true);
});
