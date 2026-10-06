// R120 · the event-wiring clean-up's guard.
//
// R119 surveyed every frontend subscription and promise in the app and left a
// ranked candidate list. R120 lands five of those points, all in `src/App.tsx`:
//
//   F1  the `custom-shortcut` listener was re-registered on every render (its
//       effect depended on a callback rebuilt each render), so a keypress could
//       fall into the unlisten→re-listen gap and every render paid two IPC
//       invokes. It now reads the action through a ref and registers once.
//   F2  `onFocusChanged` stored its disposer in a local and only ran it from
//       the cleanup: a teardown *before* the promise resolved leaked the
//       listener forever. It now carries a `disposed` guard, like the three
//       existing precedents.
//   F5  the non-Windows `start_drag` branch dropped its rejection while the
//       Windows branch swallowed it; both are silent now.
//   F3  the `floter://<mode>` listener had no sender anywhere in the tree (or
//       in `src-tauri` history): a subscription that was unreachable from
//       birth.
//   F8  two TEMPORARY DEV <PROBE> blocks (one writing the system clipboard)
//       are gone; nothing ever set their trigger flag.
//
// Every banned token is assembled from fragments so this guard does not spell
// the very strings it scans for — its own last test proves that.
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

const root = new URL("../", import.meta.url);
const read = (path: string) => readFile(new URL(path, root), "utf8");
const count = (haystack: string, needle: string) => haystack.split(needle).length - 1;

const APP = "src/App.tsx";

// Banned literals, assembled.
const GHOST_EVENT = "floter://" + "mode";
const PROBE_FLAG = "__floter" + "Probe";
const PROBE_COMMENT = "TEMPORARY DEV " + "PROBE";

// Live wiring the guard leans on, assembled for symmetry with the bans.
const SHORTCUT_EVENT = "custom-shortcut://" + "trigger";
const SHORTCUT_REF = "customShortcutAction" + "Ref";
const FOCUS_CHANGED = "onFocus" + "Changed";
const DRAG_COMMAND = "start_" + "drag";

// `invoke("start_drag")` immediately chained to the silent catch. The Windows
// branch spreads the chain over three lines, so the pattern spans whitespace;
// a branch that lost its catch drops the count to one.
const DRAG_SILENT = new RegExp(
  `invoke\\("${DRAG_COMMAND}"\\)\\s*\\.catch\\(\\(\\) => undefined\\)`,
  "g",
);

test("the custom-shortcut listener registers once and reads its action through a ref", async () => {
  const source = await read(APP);
  const at = source.indexOf(SHORTCUT_EVENT);
  assert.ok(at >= 0, "the custom-shortcut listener must still be wired");
  // The dependency array is the next `}, [...]);` after the listen call.
  const tail = source.slice(at, at + 900);
  const deps = tail.match(/\}\s*,\s*(\[[^\]]*\])\s*\)\s*;/);
  assert.ok(deps, "the effect's dependency array must be present");
  assert.equal(
    deps[1],
    "[]",
    "the listener must depend on nothing: a per-render dep re-registers it every render",
  );
  assert.ok(
    !tail.includes("runCustomShortcutAction" + "]"),
    "the rebuilt callback may not sit in the dependency array",
  );
  // The indirection itself: the ref exists, is refreshed each render, and the
  // listener calls through it rather than closing over the callback.
  assert.ok(source.includes(`const ${SHORTCUT_REF} = useRef(`), "the ref must be declared");
  assert.ok(
    source.includes(`${SHORTCUT_REF}.current = runCustomShortcutAction`),
    "the ref must be refreshed every render",
  );
  assert.ok(
    tail.includes(`${SHORTCUT_REF}.current(event.payload)`),
    "the listener must dispatch through the ref",
  );
});

test("the onFocusChanged disposer is guarded against a late resolve", async () => {
  const source = await read(APP);
  const at = source.indexOf(FOCUS_CHANGED);
  assert.ok(at >= 0, "the focus listener must still be wired");
  const tail = source.slice(at, at + 3000);
  const thenAt = tail.indexOf(".then((dispose) => {");
  assert.ok(thenAt >= 0, "the listener promise must keep its `.then` chain");
  const block = tail.slice(thenAt, thenAt + 500);
  assert.ok(
    block.includes("if (disposed) dispose();"),
    "a promise resolving after teardown must dispose itself, not leak",
  );
  assert.ok(
    block.includes("else unlisten = dispose;"),
    "a live promise must still hand the disposer to the cleanup",
  );
  assert.ok(tail.includes("disposed = true;"), "the cleanup must raise the disposed flag");
});

test("both start_drag branches swallow the rejection", async () => {
  const source = await read(APP);
  const silent = source.match(DRAG_SILENT) ?? [];
  assert.equal(
    silent.length,
    2,
    "the non-Windows and Windows drag invokes must each carry the silent catch",
  );
});

test("the ghost mode subscription is gone", async () => {
  const source = await read(APP);
  assert.equal(count(source, GHOST_EVENT), 0, "no sender ever existed for this event");
});

test("the two dev probes are gone", async () => {
  const source = await read(APP);
  assert.equal(count(source, PROBE_FLAG), 0, "nothing sets the probe flag; the block is dead");
  assert.equal(count(source, PROBE_COMMENT), 0, "the probe comment marks code to remove");
});

test("the wiring this guard pins is still live, not a hollow shell", async () => {
  const source = await read(APP);
  assert.ok(source.includes(SHORTCUT_EVENT), "the custom-shortcut listener must survive");
  assert.ok(source.includes(FOCUS_CHANGED), "the focus listener must survive");
  assert.ok(source.includes(DRAG_COMMAND), "the drag command must survive");
});

test("this guard assembles its banned tokens, it does not spell them", async () => {
  const self = await read("tests/r120-event-wiring.test.ts");
  for (const token of [GHOST_EVENT, PROBE_FLAG, PROBE_COMMENT]) {
    assert.ok(!self.includes(token), `the guard must not spell ${token}`);
  }
});
