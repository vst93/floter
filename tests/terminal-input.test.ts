import assert from "node:assert/strict";
import test from "node:test";
import {
  encodeKey,
  isTerminalCompositionKey,
  shouldUseTerminalTextInput,
} from "../src/terminal/input.ts";

type KeyOptions = Partial<Pick<KeyboardEvent, "altKey" | "ctrlKey" | "metaKey" | "shiftKey">> & {
  altGraph?: boolean;
  isComposing?: boolean;
  keyCode?: number;
};

const keyEvent = (key: string, options: KeyOptions = {}): KeyboardEvent => ({
  key,
  altKey: options.altKey ?? false,
  ctrlKey: options.ctrlKey ?? false,
  metaKey: options.metaKey ?? false,
  shiftKey: options.shiftKey ?? false,
  isComposing: options.isComposing ?? false,
  keyCode: options.keyCode ?? 0,
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

test("printable, dead, and AltGraph keys use the native text input path", () => {
  assert.equal(shouldUseTerminalTextInput(keyEvent("a")), true);
  assert.equal(shouldUseTerminalTextInput(keyEvent("Dead")), true);
  assert.equal(
    shouldUseTerminalTextInput(keyEvent("€", { ctrlKey: true, altKey: true, altGraph: true })),
    true,
  );
});

test("terminal control and Meta shortcuts stay on the key encoder path", () => {
  assert.equal(shouldUseTerminalTextInput(keyEvent("c", { ctrlKey: true })), false);
  assert.equal(shouldUseTerminalTextInput(keyEvent("x", { altKey: true })), false);
  assert.equal(shouldUseTerminalTextInput(keyEvent("v", { metaKey: true })), false);
  assert.equal(shouldUseTerminalTextInput(keyEvent("Enter")), false);
});

test("IME confirmation keys are recognized after WebKit clears isComposing", () => {
  assert.equal(isTerminalCompositionKey(keyEvent("Process", { isComposing: true })), true);
  assert.equal(isTerminalCompositionKey(keyEvent("Enter", { keyCode: 229 })), true);
  assert.equal(isTerminalCompositionKey(keyEvent("Enter")), false);
});
