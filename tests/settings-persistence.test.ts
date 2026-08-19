import assert from "node:assert/strict";
import test from "node:test";
import { createSerialSettingsWriter } from "../src/settings-persistence.ts";

test("settings writes preserve the order they were requested", async () => {
  const writes: string[] = [];
  let releaseFirst: (() => void) | undefined;
  const firstGate = new Promise<void>((resolve) => {
    releaseFirst = resolve;
  });
  const write = createSerialSettingsWriter<string>(async (value) => {
    writes.push(value);
    if (value === "first") await firstGate;
  });

  const first = write("first");
  const second = write("second");
  await Promise.resolve();

  assert.deepEqual(writes, ["first"]);
  releaseFirst?.();
  await Promise.all([first, second]);
  assert.deepEqual(writes, ["first", "second"]);
});

test("a failed settings write does not block the latest snapshot", async () => {
  const writes: string[] = [];
  const write = createSerialSettingsWriter<string>(async (value) => {
    writes.push(value);
    if (value === "broken") throw new Error("disk unavailable");
  });

  await assert.rejects(write("broken"), /disk unavailable/);
  await write("latest");

  assert.deepEqual(writes, ["broken", "latest"]);
});
