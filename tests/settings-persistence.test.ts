import assert from "node:assert/strict";
import test from "node:test";
import {
  createSerialSettingsWriter,
  createSettingsHydration,
  rollbackRejectedSettings,
} from "../src/settings-persistence.ts";

test("a rejected switch restores the last confirmed backend value", () => {
  const confirmed = { enabled: true, theme: "dark" };
  const attempted = { ...confirmed, enabled: false };
  assert.deepEqual(rollbackRejectedSettings(attempted, attempted, confirmed), confirmed);
});

test("rollback preserves edits made while a save was in flight", () => {
  const confirmed = { enabled: true, opacity: 90 };
  const attempted = { enabled: false, opacity: 80 };
  assert.deepEqual(rollbackRejectedSettings({ ...attempted, opacity: 70 }, attempted, confirmed), {
    enabled: true, opacity: 70,
  });
});

test("settings hydration preserves edits made while the disk read is in flight", () => {
  type Settings = { theme: string; fontSize: number; language: string };
  const hydration = createSettingsHydration<Settings>();
  const current = { theme: "light", fontSize: 14, language: "en" };
  const loaded = { theme: "dark", fontSize: 18, language: "zh" };

  hydration.markChanged("theme");

  assert.deepEqual(hydration.mergeLoaded(current, loaded), {
    theme: "light",
    fontSize: 18,
    language: "zh",
  });
});

test("settings writes wait until hydration has completed", async () => {
  const hydration = createSettingsHydration<{ theme: string }>();
  let continued = false;
  const waiting = hydration.waitUntilReady().then(() => {
    continued = true;
  });

  await Promise.resolve();
  assert.equal(continued, false);

  hydration.finish();
  await waiting;
  assert.equal(continued, true);
  assert.equal(hydration.isReady(), true);
});

test("a failed settings read stays blocked until a retry succeeds", async () => {
  type Settings = { theme: string; fontSize: number };
  const hydration = createSettingsHydration<Settings>();
  let continued = false;
  const waiting = hydration.waitUntilReady().then(() => {
    continued = true;
  });

  hydration.markChanged("theme");
  hydration.markFailed();
  await Promise.resolve();

  assert.equal(hydration.hasFailed(), true);
  assert.equal(hydration.isReady(), false);
  assert.equal(continued, false);
  assert.deepEqual(
    hydration.mergeLoaded(
      { theme: "light", fontSize: 14 },
      { theme: "dark", fontSize: 18 },
    ),
    { theme: "light", fontSize: 18 },
  );

  hydration.finish();
  await waiting;
  assert.equal(hydration.hasFailed(), false);
  assert.equal(continued, true);
});

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
