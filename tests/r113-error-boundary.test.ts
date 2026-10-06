// R113 · the root render backstop's guard.
//
// R111's survey (finding E) found `main.tsx` mounting the app with no boundary,
// so a render-time throw blanked the window. The fix wraps the one mount point
// in `ErrorBoundary`. This suite pins the three things that keep the wrapper
// real:
//
//   * the mount point still wraps its render in the boundary tag;
//   * the component actually catches (its derived-state hook is present), so
//     the tag cannot be a hollow shell; and
//   * the mount point still mounts the React root, so the wrapper did not
//     replace the app.
//
// The tokens are assembled rather than spelled: a guard that writes the string
// it greps for can pass by matching its own source.
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

const root = new URL("../", import.meta.url);
const read = (path: string) => readFile(new URL(path, root), "utf8");

const BOUNDARY_OPEN = "<" + "ErrorBoundary";
const CATCHER = "getDerivedStateFrom" + "Error";
const MOUNT = "create" + "Root";

test("the one mount point wraps its render in the boundary", async () => {
  const main = await read("src/main.tsx");
  assert.ok(main.includes(BOUNDARY_OPEN), "main.tsx must wrap the render in the boundary tag");
});

test("the boundary actually catches, it is not a hollow shell", async () => {
  const boundary = await read("src/components/ErrorBoundary.tsx");
  assert.ok(
    boundary.includes(CATCHER),
    "ErrorBoundary.tsx must implement the derived-state catcher",
  );
});

test("the mount point still mounts the app", async () => {
  const main = await read("src/main.tsx");
  assert.ok(main.includes(MOUNT), "main.tsx must still create the React root");
});

test("this guard assembles its tokens, it does not spell them", async () => {
  const self = await read("tests/r113-error-boundary.test.ts");
  for (const token of [BOUNDARY_OPEN, CATCHER, MOUNT]) {
    assert.ok(!self.includes(token), `the guard must not spell ${token} in its own source`);
  }
});
