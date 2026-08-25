import assert from "node:assert/strict";
import test from "node:test";
import {
  CARD_DEFAULT_HEIGHT,
  CARD_DEFAULT_WIDTH,
  CARD_MIN_HEIGHT,
  CARD_MIN_WIDTH,
  clampCardGeometry,
  defaultCardGeometry,
  type CardGeometry,
  loadPinnedGeometry,
  pinReducer,
  PINNED_SESSION_ID,
  type PinState,
  savePinnedGeometry,
} from "../src/terminal/pinState.ts";

const PINNED = (
  brokerSessionId: string,
  generation: number,
): PinState => ({
  status: "pinned",
  session: { brokerSessionId, generation, label: null },
});

// ---- pin state reducer ----------------------------------------------------

test("pin reducer pins an idle state", () => {
  const next = pinReducer({ status: "idle" }, {
    type: "pin",
    brokerSessionId: "abc",
    generation: 7,
    label: "dev shell",
  });
  assert.deepEqual(next, {
    status: "pinned",
    session: { brokerSessionId: "abc", generation: 7, label: "dev shell" },
  });
});

test("pin reducer replaces the card when a second session is pinned", () => {
  const next = pinReducer(PINNED("aaa", 1), {
    type: "pin",
    brokerSessionId: "bbb",
    generation: 2,
  });
  assert.equal(next.status, "pinned");
  assert.ok(next.status === "pinned");
  assert.equal(next.session.brokerSessionId, "bbb");
  // The old card must be fully gone: a stale exit event for its generation
  // must not close the new one.
  assert.equal(pinReducer(next, { type: "sessionClosed", generation: 1 }), next);
});

test("pin reducer unpins", () => {
  assert.deepEqual(pinReducer(PINNED("abc", 3), { type: "unpin" }), { status: "idle" });
  assert.deepEqual(pinReducer({ status: "idle" }, { type: "unpin" }), { status: "idle" });
});

test("pin reducer cleans up when the pinned session exits", () => {
  const next = pinReducer(PINNED("abc", 9), { type: "sessionClosed", generation: 9 });
  assert.deepEqual(next, { status: "idle" });
});

test("pin reducer ignores exit events from other generations", () => {
  const state = PINNED("abc", 9);
  assert.equal(pinReducer(state, { type: "sessionClosed", generation: 8 }), state);
});

test("pin reducer ignores labels while idle or for other generations' cards", () => {
  const idle = pinReducer({ status: "idle" }, { type: "label", label: "x" });
  assert.deepEqual(idle, { status: "idle" });
  const state = PINNED("abc", 5);
  const labelled = pinReducer(state, { type: "label", label: "build" });
  assert.ok(labelled.status === "pinned");
  assert.equal(labelled.session.label, "build");
});

test("pinned session id is namespaced away from the main view", () => {
  assert.notEqual(PINNED_SESSION_ID, "main");
});

// ---- geometry clamp -------------------------------------------------------

const geom = (x: number, y: number, width: number, height: number): CardGeometry => ({
  x,
  y,
  width,
  height,
});

test("clamp keeps a dragged card fully inside the window", () => {
  const clamped = clampCardGeometry(geom(900, 700, 460, 280), 1000, 800);
  assert.deepEqual(clamped, { x: 540, y: 520, width: 460, height: 280 });

  const negative = clampCardGeometry(geom(-50, -10, 460, 280), 1000, 800);
  assert.deepEqual(negative, { x: 0, y: 0, width: 460, height: 280 });
});

test("clamp constrains resize beyond the window bounds", () => {
  const clamped = clampCardGeometry(geom(0, 0, 5000, 5000), 1000, 800);
  assert.deepEqual(clamped, { x: 0, y: 0, width: 1000, height: 800 });
});

test("clamp enforces the minimum card size", () => {
  const clamped = clampCardGeometry(geom(10, 10, 120, 90), 1000, 800);
  assert.equal(clamped.width, CARD_MIN_WIDTH);
  assert.equal(clamped.height, CARD_MIN_HEIGHT);
  assert.equal(clamped.x, 10);
  assert.equal(clamped.y, 10);
});

test("clamp re-fits the card after the window shrinks (window-resize case)", () => {
  // Card was placed in a 1600x900 window; the window is now 800x400.
  const clamped = clampCardGeometry(geom(1100, 600, 460, 280), 800, 400);
  assert.deepEqual(clamped, { x: 340, y: 120, width: 460, height: 280 });

  // Window smaller than the minimum: minimum wins, position pins to 0.
  const tiny = clampCardGeometry(geom(50, 50, 320, 180), 200, 100);
  assert.deepEqual(tiny, { x: 0, y: 0, width: CARD_MIN_WIDTH, height: CARD_MIN_HEIGHT });
});

test("clamp repairs non-finite geometry defensively", () => {
  const clamped = clampCardGeometry(
    geom(Number.NaN, Number.POSITIVE_INFINITY, Number.NaN, 280),
    1000,
    800,
  );
  assert.equal(Number.isFinite(clamped.x), true);
  assert.equal(Number.isFinite(clamped.y), true);
  assert.equal(clamped.width, CARD_DEFAULT_WIDTH);
  assert.equal(clamped.height, 280);
});

test("default geometry sits at the bottom-right inside the bounds", () => {
  const def = defaultCardGeometry(1280, 860);
  assert.deepEqual(def, {
    x: 1280 - CARD_DEFAULT_WIDTH - 24,
    y: 860 - CARD_DEFAULT_HEIGHT - 24,
    width: CARD_DEFAULT_WIDTH,
    height: CARD_DEFAULT_HEIGHT,
  });
  // And it is always a valid placement for its own bounds.
  assert.deepEqual(clampCardGeometry(def, 1280, 860), def);
});

// ---- persistence round-trip -----------------------------------------------

class MemoryStorage {
  private map = new Map<string, string>();
  getItem(key: string): string | null {
    return this.map.get(key) ?? null;
  }
  setItem(key: string, value: string): void {
    this.map.set(key, value);
  }
}

test("geometry survives a persistence round-trip", () => {
  const storage = new MemoryStorage();
  assert.equal(loadPinnedGeometry(storage), null);

  const saved = geom(123, 45, 480, 300);
  savePinnedGeometry(storage, saved);
  const loaded = loadPinnedGeometry(storage as unknown as Storage);
  assert.deepEqual(loaded, saved);
});

test("load rejects malformed persisted payloads", () => {
  const storage = new MemoryStorage();
  for (const raw of [
    "not json",
    "null",
    "42",
    "{}",
    JSON.stringify({ x: "a", y: 0, width: 100, height: 100 }),
    JSON.stringify({ x: Number.NaN, y: 0, width: 100, height: 100 }),
    JSON.stringify({ y: 0, width: 100, height: 100 }),
  ]) {
    storage.setItem("floter.pinned-terminal.geometry", raw);
    assert.equal(loadPinnedGeometry(storage as unknown as Storage), null, raw);
  }
});

test("save failures are swallowed instead of breaking the caller", () => {
  const throwing = {
    getItem: () => null,
    setItem: () => {
      throw new Error("quota exceeded");
    },
  };
  assert.doesNotThrow(() => savePinnedGeometry(throwing as unknown as Storage, geom(1, 2, 3, 4)));
  assert.equal(loadPinnedGeometry(null), null);
  assert.doesNotThrow(() => savePinnedGeometry(null, geom(1, 2, 3, 4)));
});
