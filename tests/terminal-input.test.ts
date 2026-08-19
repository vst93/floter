import assert from "node:assert/strict";
import test from "node:test";
import { encodeKey } from "../src/terminal/input.ts";

type KeyOptions = Partial<Pick<KeyboardEvent, "altKey" | "ctrlKey" | "metaKey" | "shiftKey">> & {
  altGraph?: boolean;
};

const keyEvent = (key: string, options: KeyOptions = {}): KeyboardEvent => ({
  key,
  altKey: options.altKey ?? false,
  ctrlKey: options.ctrlKey ?? false,
  metaKey: options.metaKey ?? false,
  shiftKey: options.shiftKey ?? false,
  getModifierState: (modifier: string) => modifier === "AltGraph" && Boolean(options.altGraph),
}) as KeyboardEvent;

const encoded = (key: string, options?: KeyOptions): number[] | null => {
  const result = encodeKey(keyEvent(key, options), 0);
  return result ? Array.from(result) : null;
};

test("modified function keys use xterm modifier parameters", () => {
  assert.deepEqual(encoded("F1", { shiftKey: true }), [27, 91, 49, 59, 50, 80]);
  assert.deepEqual(encoded("F5", { ctrlKey: true }), [27, 91, 49, 53, 59, 53, 126]);
  assert.deepEqual(encoded("F12", { altKey: true }), [27, 91, 50, 52, 59, 51, 126]);
});

test("unmodified function key sequences remain compatible", () => {
  assert.deepEqual(encoded("F1"), [27, 79, 80]);
  assert.deepEqual(encoded("F5"), [27, 91, 49, 53, 126]);
});

test("AltGraph sends the composed character instead of a Ctrl sequence", () => {
  assert.deepEqual(
    encoded("@", { ctrlKey: true, altKey: true, altGraph: true }),
    [64],
  );
  assert.deepEqual(
    encoded("€", { ctrlKey: true, altKey: true, altGraph: true }),
    [226, 130, 172],
  );
});

test("Ctrl+Alt keeps the Meta prefix for intentional shortcuts", () => {
  assert.deepEqual(encoded("c", { ctrlKey: true, altKey: true }), [27, 3]);
});
