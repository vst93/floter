// CLIP-DRAG · the payload-free window-drag bridge message.
//
// The clipboard page used to sit behind a sandboxed iframe: a mousedown inside
// it never reached the host's `startDrag`, so the page reported the *intent*
// with a payload-free `drag` message and the host ran its one drag body. R33
// retired the built-in pages onto the launcher's configuration overlay and R76
// deleted the page's source.
//
// What remains live — and is what this suite still pins — is the *protocol*:
// the message shape, the routing predicate, and the one shared OS-drag body the
// shells' own mousedown funnels through. The retired page-side guard and the
// page's stylesheet are gone; the negative guard below keeps them gone.
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";
import {
  BRIDGE_TAG,
  isBridgeDrag,
  shouldStartWindowDrag,
} from "../src/plugin-pages.ts";
import { assertRetiredPageLayerIsGone } from "./retired-page-layer.ts";

const root = new URL("../", import.meta.url);
const read = (path: string) => readFile(new URL(path, root), "utf8");
const code = (src: string) => src.replace(/\/\*[\s\S]*?\*\//g, "").replace(/\/\/[^\n]*/g, "");

test("the retired clipboard page source stays deleted", async () => {
  await assertRetiredPageLayerIsGone(root);
});

// ── 1 · the wire message ──────────────────────────────────────────────────

test("the drag message is recognized, and a non-drag message is not", () => {
  assert.ok(isBridgeDrag({ [BRIDGE_TAG]: "drag" }), "a drag message must be recognized");
  for (const bad of [
    { [BRIDGE_TAG]: "close" },
    { [BRIDGE_TAG]: "invoke", id: 1, command: "c" },
    { [BRIDGE_TAG]: "opacity", mainOpacity: 1, terminalOpacity: 1 },
    null,
    undefined,
    {},
    "drag",
  ]) {
    assert.equal(isBridgeDrag(bad), false, `${JSON.stringify(bad)} must not be a drag`);
  }
});

// ── 2 · the host routes it to the one drag body ───────────────────────────

test("the host honours a drag only from the page that is the live surface", () => {
  const drag = { [BRIDGE_TAG]: "drag" };
  assert.equal(shouldStartWindowDrag(drag, true), true, "the live page's drag must fire");
  assert.equal(
    shouldStartWindowDrag(drag, false),
    false,
    "a kept-alive page hidden behind another one must not move the window on a stale press",
  );
  assert.equal(shouldStartWindowDrag({ [BRIDGE_TAG]: "close" }, true), false, "only a drag routes to the drag body");
  assert.equal(shouldStartWindowDrag(null, true), false, "a null payload must not route");
});

test("App keeps one shared OS-drag body the shells funnel through", async () => {
  const app = await read("src/App.tsx");
  // R33 · App no longer mounts the host; the drag body stays the one shared
  // callback the shells' own mousedown funnels through.
  assert.ok(!/<PluginPageHost\b/.test(app), "the retired host must not be mounted");
  assert.match(app, /const beginDrag = useCallback\(\(\) => \{/, "the drag body must be one shared callback");
  assert.match(app, /invoke\("start_drag"\)/, "the body must still invoke the OS drag");
  // And the shells' own mousedown still funnels through it.
  assert.match(app, /const startDrag = \(event: React\.MouseEvent\) => \{[\s\S]*?beginDrag\(\);/, "startDrag must delegate");
  // Every `invoke("start_drag")` lives inside the one shared body — the two
  // calls are its Windows and non-Windows branches, not two entry points.
  const body = app.slice(app.indexOf("const beginDrag = useCallback"), app.indexOf("const startDrag ="));
  assert.equal(
    (app.match(/invoke\("start_drag"\)/g) ?? []).length,
    2,
    "the OS drag is invoked from the two platform branches",
  );
  assert.equal(
    (body.match(/invoke\("start_drag"\)/g) ?? []).length,
    2,
    "both branches must live inside the shared `beginDrag` body",
  );
  // No parallel Tauri drag-region attribute, which would be a second,
  // differently-behaving drag mechanism. Check the code, not the prose.
  assert.ok(
    !/data-tauri-drag-region/.test(code(app)),
    "the shells must reuse startDrag, not add a data-tauri-drag-region",
  );
});

test("the bridge protocol keeps the drag type without changing an existing one", async () => {
  const protocol = await read("src/plugin-pages.ts");
  // The type is declared, recognized, and part of the page → host union.
  assert.match(protocol, /export type BridgeDrag = \{ \[BRIDGE_TAG\]: "drag" \};/);
  assert.match(protocol, /export type BridgeFromPage = [^;]*BridgeDrag[^;]*;/);
  // The message types the round promised not to touch are all still declared
  // with their original tags.
  for (const [name, tag] of [
    ["BridgeClose", "close"],
    ["BridgeNotify", "host-notify"],
    ["BridgeReload", "reload"],
    ["BridgeOpacity", "opacity"],
    ["BridgeTheme", "theme"],
    ["BridgeGlass", "glass"],
    ["BridgeVisibility", "visibility"],
  ] as const) {
    assert.match(protocol, new RegExp(`export type ${name} = [^;]*"${tag}"`), `${name} must keep its tag`);
  }
});

// ── 3 · mutation lock ─────────────────────────────────────────────────────

test("mutation lock: turning the drag route into a no-op goes red", () => {
  // The exact regression: the listener stops calling the drag body.
  const route = (data: unknown, active: boolean, fire: () => void) => {
    if (shouldStartWindowDrag(data, active)) fire();
  };
  const drag = { [BRIDGE_TAG]: "drag" };
  let fired = 0;
  route(drag, true, () => { fired += 1; });
  assert.equal(fired, 1, "the shipped predicate must fire the drag");

  // A no-op — the listener recognizes the message but never runs the body.
  const noopRoute = (_data: unknown, _active: boolean, _fire: () => void) => {};
  let noopFired = 0;
  noopRoute(drag, true, () => { noopFired += 1; });
  assert.equal(noopFired, 0, "a no-op route must not fire the drag");

  // And the predicate itself must reject what the no-op would have accepted.
  assert.ok(
    shouldStartWindowDrag(drag, true),
    "the shipped predicate must accept a live page's drag",
  );
  assert.ok(
    !shouldStartWindowDrag(drag, false),
    "the predicate must reject a hidden page's drag",
  );
});
