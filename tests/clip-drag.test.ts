// The one shared OS-drag body every shell's mousedown funnels through.
//
// The clipboard page used to sit behind a sandboxed iframe: a mousedown inside
// it never reached the host's `startDrag`, so the page sent a payload-free
// `drag` bridge message and the host ran its one drag body. R33 retired the
// built-in pages onto the launcher's configuration overlay, R76 deleted the
// page's source, and R96 deleted the published bridge — the `drag` message and
// its routing predicate are gone.
//
// What remains live — and is what this suite pins — is the one shared OS-drag
// body the shells' own mousedown funnels through.
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

const root = new URL("../", import.meta.url);
const read = (path: string) => readFile(new URL(path, root), "utf8");
const code = (src: string) => src.replace(/\/\*[\s\S]*?\*\//g, "").replace(/\/\/[^\n]*/g, "");

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

test("the retired drag bridge message stays deleted", async () => {
  const pluginPages = [
    await read("src/builtin-plugins.ts"),
    await read("src/failure-deduper.ts"),
  ].join("\n");
  // The `drag` message, its recognizer and its routing predicate went with the
  // rest of the bridge in R96. Assembled from parts so the round's zero-hit
  // grep stays clean.
  assert.ok(!pluginPages.includes("isBridge" + "Drag"), "the drag recognizer must stay gone");
  assert.ok(
    !pluginPages.includes("shouldStart" + "WindowDrag"),
    "the drag routing predicate must stay gone",
  );
  assert.ok(!pluginPages.includes("Bridge" + "Drag"), "the drag message type must stay gone");
});
