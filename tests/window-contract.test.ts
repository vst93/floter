// R7-13a · The window-size contract has one source per language, and the two
// are compared here.
//
// `INPUT_WINDOW_WIDTH = 720` used to be written out four times: `App.tsx`,
// `useLauncherHeight.ts`, and twice inside `src-tauri/src/lib.rs` (where the
// const is also read by the native activation paths). The Rust const cannot
// import the TypeScript one, so the round left both in place and pinned them
// against each other instead: the assertions below read the Rust declaration as
// text, extract its value, and require the frontend's to equal it. Change either
// side alone and this file goes red.
//
// Mutations that must turn this red:
//   * `INPUT_WINDOW_WIDTH = 721` in `src/window-contract.ts` -> the
//     cross-language equality assertion fails;
//   * `const INPUT_WINDOW_WIDTH: f64 = 721.0;` in `src-tauri/src/lib.rs` -> the
//     same assertion fails from the other direction;
//   * re-declaring `const INPUT_WINDOW_WIDTH = 720;` in `App.tsx` or
//     `useLauncherHeight.ts` -> the sweep over `src/` fails and names the file.
import assert from "node:assert/strict";
import { readFile, readdir } from "node:fs/promises";
import test from "node:test";
import { INPUT_WINDOW_WIDTH } from "../src/window-contract.ts";

const root = new URL("../", import.meta.url);
const read = (path: string) => readFile(new URL(path, root), "utf8");

// The modules that actually carried a copy before this round. Named explicitly
// so a regression there is reported as the historical offender rather than just
// "some file in src/"; the sweep below then covers whatever comes next.
const FORMERLY_LOCAL = ["src/App.tsx", "src/hooks/useLauncherHeight.ts"];

// Every .ts/.tsx file under src/, as absolute URLs.
async function sourceFiles(directory: URL): Promise<URL[]> {
  const found: URL[] = [];
  for (const entry of await readdir(directory, { withFileTypes: true })) {
    const child = new URL(entry.name + (entry.isDirectory() ? "/" : ""), directory);
    if (entry.isDirectory()) found.push(...(await sourceFiles(child)));
    else if (/\.tsx?$/.test(entry.name)) found.push(child);
  }
  return found;
}

test("the frontend width matches the Rust declaration", async () => {
  const rust = await read("src-tauri/src/lib.rs");
  const match = /const\s+INPUT_WINDOW_WIDTH\s*:\s*f64\s*=\s*([0-9.]+)\s*;/.exec(rust);
  assert.ok(
    match,
    "src-tauri/src/lib.rs no longer declares `const INPUT_WINDOW_WIDTH: f64 = …;` — " +
      "the native side is the other half of this contract and its shape is what this test reads",
  );
  const rustWidth = Number(match[1]);
  assert.equal(
    INPUT_WINDOW_WIDTH,
    rustWidth,
    `frontend INPUT_WINDOW_WIDTH (${INPUT_WINDOW_WIDTH}) != Rust INPUT_WINDOW_WIDTH (${rustWidth}); ` +
      "the two are one contract and must move together",
  );
});

test("INPUT_WINDOW_WIDTH is declared in exactly one frontend module", async () => {
  for (const path of FORMERLY_LOCAL) {
    assert.doesNotMatch(
      await read(path),
      /const\s+INPUT_WINDOW_WIDTH\s*=/,
      `${path} declares its own INPUT_WINDOW_WIDTH; import it from src/window-contract.ts instead`,
    );
  }

  // The named check covers the regressions we know about; the sweep covers the
  // one we do not (a new surface quietly adding a third copy).
  const declaring: string[] = [];
  for (const file of await sourceFiles(new URL("src/", root))) {
    if (file.pathname.endsWith("window-contract.ts")) continue;
    if (/const\s+INPUT_WINDOW_WIDTH\s*=/.test(await readFile(file, "utf8"))) {
      declaring.push(file.href.slice(root.href.length));
    }
  }
  assert.deepEqual(
    declaring,
    [],
    `INPUT_WINDOW_WIDTH is declared outside src/window-contract.ts: ${declaring.join(", ")}`,
  );
});

test("the declaration is the number the native resize paths expect", () => {
  assert.equal(typeof INPUT_WINDOW_WIDTH, "number");
  assert.ok(Number.isInteger(INPUT_WINDOW_WIDTH) && INPUT_WINDOW_WIDTH > 0);
});

test("no height is contracted, and the settings height stays frontend-owned", async () => {
  // The collapsed height is measured at runtime (`syncLauncherHeight`) and Rust
  // only needs a fallback for the first resize, so it has no frontend
  // counterpart. Adding one here would create the very duplication this module
  // exists to remove.
  assert.doesNotMatch(
    await read("src/window-contract.ts"),
    /INPUT_WINDOW_HEIGHT/,
    "do not add INPUT_WINDOW_HEIGHT to the contract module — there is no frontend height to contract",
  );

  assert.match(
    await read("src/App.tsx"),
    /const\s+SETTINGS_WINDOW_HEIGHT\s*=\s*580\s*;/,
    "settings height is a frontend layout constant with no native counterpart; it does not belong in the contract module",
  );
});
