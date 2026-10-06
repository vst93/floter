// R121 · the dead-event clean-up's guard.
//
// R119's event census compared the two ends of every event in the app. It found
// the extension-mutated event (see `DEAD_EVENT` below — this file may not spell
// it) with **five** emitters on the Rust side and **zero**
// subscribers anywhere on the frontend: the panel refreshes by awaiting the
// command's return value (`ExtensionsPanel.tsx`, `useExtensionActions.ts`), so
// every one of those emits was a no-op. R121 deletes all five:
//
//   src-tauri/src/commands/extensions.rs  :1228  :1343  :1893  :1934
//   src-tauri/src/deep_link.rs            :834
//
// The first test scans the whole `src-tauri/src` tree (recursively — a stray
// emit in a module the census did not name must still trip it). The second is
// the non-hollow check: the events that *do* have live senders and receivers
// are pinned so this guard cannot pass by emptying the tree. The last test
// proves this file assembles its banned token instead of spelling it.
import assert from "node:assert/strict";
import { readdir, readFile } from "node:fs/promises";
import test from "node:test";

const root = new URL("../", import.meta.url);
const read = (path: string) => readFile(new URL(path, root), "utf8");

// The banned literal, assembled: 5 emits died with their last (never-existing)
// listener. A revival anywhere in the Rust tree is a red guard.
const DEAD_EVENT = "extensions" + "-changed";

// Live wiring the guard leans on, assembled for symmetry with the ban.
const RUST_SRC = "src-tauri/src";
const TERMINAL_SESSION = `${RUST_SRC}/terminal/session.rs`;
const TERMINAL_EVENT = "term://" + "frame";
const LIB = `${RUST_SRC}/lib.rs`;
const SETTINGS_EVENT = "floter://" + "open-settings";

/** Every file under `dir`, recursively, as root-relative paths. */
async function tree(dir: string): Promise<string[]> {
  const entries = await readdir(new URL(`${dir}/`, root), { withFileTypes: true });
  const files: string[] = [];
  for (const entry of entries) {
    const rel = `${dir}/${entry.name}`;
    if (entry.isDirectory()) files.push(...(await tree(rel)));
    else files.push(rel);
  }
  return files;
}

test("no `DEAD_EVENT` emit survives anywhere in the Rust tree", async () => {
  const files = await tree(RUST_SRC);
  assert.ok(files.length > 50, "the scan must cover a live tree, not an empty path");
  const hits: string[] = [];
  for (const file of files) {
    const source = await read(file);
    if (source.includes(DEAD_EVENT)) hits.push(file);
  }
  assert.deepEqual(
    hits,
    [],
    "this event has no subscriber on the frontend: every emit is a no-op (R121)",
  );
});

test("the events with live senders and receivers still exist", async () => {
  const terminal = await read(TERMINAL_SESSION);
  assert.ok(terminal.includes(TERMINAL_EVENT), "term://frame has a real emitter and listener");
  const lib = await read(LIB);
  assert.ok(lib.includes(SETTINGS_EVENT), "floter://open-settings has a real emitter and listener");
});

test("this guard assembles its banned token, it does not spell it", async () => {
  const self = await read("tests/r121-dead-event.test.ts");
  assert.ok(
    !self.includes(DEAD_EVENT),
    "the guard must not write the literal it scans for",
  );
  assert.ok(self.includes("DEAD_EVENT"), "the guard must still use the assembled constant");
});
