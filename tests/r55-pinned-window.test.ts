// R55 · the pin is an independent native window, not a card inside the main one.
//
// The user's report, verbatim: 「当前终端页固定时逻辑不对，我想要的时独立出来并固定
// 住，不是只能在终端页面内小窗」.
//
// Mutations that must turn this file red:
//   * pinning that goes back to an in-window card (the App retirement check);
//   * a window that is not always-on-top / has a taskbar item;
//   * a close path that kills the PTY instead of handing the session back.

import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

const root = new URL("../", import.meta.url);
const read = (path: string) => readFile(new URL(path, root), "utf8");

test("R55 · the document routes the pinned window to its own root", async () => {
  const main = await read("src/main.tsx");
  assert.match(main, /pinnedSessionFromLocation\(window\.location\.search\)/);
  assert.match(main, /<PinnedTerminalWindowApp brokerSessionId=\{pinnedSession\} \/>/);

  const window = await read("src/pinned-window.tsx");
  assert.match(window, /export function PinnedTerminalWindowApp/);
  // It attaches a view to the same broker session under the `pinned` id.
  assert.match(window, /term_attach_existing/);
  assert.match(window, /id: PINNED_SESSION_ID/);
  // It renders and scrolls the session itself.
  assert.match(window, /new TerminalCanvas/);
  assert.match(window, /term_resize/);
  assert.match(window, /term_input/);
  assert.match(window, /term_wheel/);
  // Closing detaches the view (the PTY survives) then closes the window.
  assert.match(window, /invoke\("term_close", \{ id: PINNED_SESSION_ID \}\)/);
  assert.match(window, /invoke\("close_pinned_terminal"\)/);
});

test("R55 · Rust opens an always-on-top, taskbar-less window", async () => {
  const rust = await read("src-tauri/src/lib.rs");
  assert.match(rust, /async fn open_pinned_terminal_window/);
  assert.match(rust, /"pinned-terminal"/);
  assert.match(rust, /\.always_on_top\(true\)/);
  assert.match(rust, /\.skip_taskbar\(true\)/);
  // The URL carries the broker session id to the second document.
  assert.match(rust, /index\.html\?pinned=\{broker_session_id\}/);
  // Closing hands the view back: the Destroyed handler drops the view and
  // tells the main window.
  assert.match(rust, /manager\.close\("pinned"\)/);
  assert.match(rust, /emit_to\(\s*"main",\s*"pinned-window:\/\/closed"/);
});

test("R55 · the main window pins through the native window and takes the session back", async () => {
  const coordinator = await read("src/hooks/usePinCoordinator.ts");
  assert.match(coordinator, /invoke\("open_pinned_terminal_window", \{ brokerSessionId \}\)/);
  assert.match(coordinator, /invoke\("close_pinned_terminal"\)/);
  assert.match(coordinator, /handlePinnedWindowClosed/);

  const app = await read("src/App.tsx");
  assert.match(app, /pinned-window:\/\/closed/);
  // The in-window card is gone from the tree.
  assert.equal(app.includes("<PinnedTerminalCard"), false);
});

test("R55 · the second window is allowed by the capability", async () => {
  const capability = JSON.parse(await read("src-tauri/capabilities/default.json")) as {
    windows: string[];
  };
  assert.ok(capability.windows.includes("main"));
  assert.ok(capability.windows.includes("pinned-terminal"));
});
