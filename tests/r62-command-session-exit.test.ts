// R62 · a command-launched session's page ends when its process does.
//
// The user's report, verbatim: 「在搜索框内通过搜索启动的命令行工具的二级命令启动会
// 快捷启动这个命令，并在终端页展示，这时退出 tui 时应该同时退出终端页面」.
//
// A session the search box started exists to carry one command. When that
// process ends the page has nothing left to hold, so it leaves for the launcher
// by the exact path the close button takes. The whole round turns on one
// exception, pinned first below: a *bare* shell (the Terminal row, the ⌘-held
// row, Enter on the empty page) is the user's own terminal and never triggers
// an automatic exit.
//
// The tests are two halves:
//   * the pure predicate in `src/terminal/command-session.ts` — the truth table
//     for "which spawns count" and "which exits may leave"; and
//   * the wiring — that the spawn records the truth, the attach path clears it,
//     and the exit listener reuses `returnToInputMode` (no second teardown, no
//     overlay, no notification).
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

import { isCommandStartedSession, shouldExitWithCommandSession } from "../src/terminal/command-session.ts";

const root = new URL("../", import.meta.url);
const read = (path: string) => readFile(new URL(path, root), "utf8");
const stripJsComments = (src: string) =>
  src.replace(/\/\*[\s\S]*?\*\//g, "").replace(/\/\/[^\n]*/g, "");

// ── 1 · the truth table ───────────────────────────────────────────────────

test("only a spawn that carries a command counts as command-started", () => {
  // The bare doors — the Terminal row, the ⌘-held row, the empty page's Enter —
  // all spawn with no command and no execution plan.
  assert.equal(isCommandStartedSession(null, null), false, "a bare shell is not a command");
  // The raw-line path hands the shell the typed line verbatim.
  assert.equal(isCommandStartedSession("go version", null), true);
  // The catalog/plugin path hands over a structured plan; the program itself is
  // the PTY child. `runCommand` passes `initialCommand: null` for it, so the
  // execution plan alone has to mark the session as command-started.
  assert.equal(
    isCommandStartedSession(null, {
      program: "lazygit",
      args: [],
      mode: "pty",
      cwd: null,
      environment: {},
      inheritEnvironment: true,
    }),
    true,
  );
  assert.equal(
    isCommandStartedSession("lazygit", {
      program: "lazygit",
      args: [],
      mode: "pty",
      cwd: null,
      environment: {},
      inheritEnvironment: true,
    }),
    true,
  );
});

test("an exit takes the page only for a command session on the terminal page", () => {
  assert.equal(shouldExitWithCommandSession(true, "terminal"), true);
  // A bare shell never leaves the page, whatever the surface.
  assert.equal(shouldExitWithCommandSession(false, "terminal"), false);
  assert.equal(shouldExitWithCommandSession(false, "collapsed"), false);
  // A command that ends while the user is elsewhere must not yank them away.
  assert.equal(shouldExitWithCommandSession(true, "settings"), false);
  assert.equal(shouldExitWithCommandSession(true, "collapsed"), false);
});

// ── 2 · the spawn records the truth ───────────────────────────────────────

test("the spawn path marks a command session and only a command session", async () => {
  const hook = stripJsComments(await read("src/hooks/useTerminalView.ts"));
  const ensure = hook.slice(
    hook.indexOf("const ensureTerminalSession"),
    hook.indexOf("const handleTerminalExit"),
  );
  assert.match(
    ensure,
    /mainSessionCommandStarted\.current = isCommandStartedSession\(initialCommand, execution\)/,
    "the one place a spawn settles decides command-vs-bare from its own args",
  );
  // Closing the session clears the flag so a later bare spawn cannot inherit it.
  const close = hook.slice(
    hook.indexOf("const closeTerminalSession"),
    hook.indexOf("const ensureTerminalSession"),
  );
  assert.match(close, /mainSessionCommandStarted\.current = false/);
  // The exit notice is session-derived, so it must not outlive the session:
  // an automatic exit that left it behind would paint a dead session's notice
  // on the next session-less page (the native toggle's empty reveal).
  assert.match(close, /setTerminalResident\(null\)/);
});

test("resuming a broker session is a resume, not a launch", async () => {
  const actions = stripJsComments(await read("src/hooks/useLauncherActions.ts"));
  const resume = actions.slice(
    actions.indexOf("const resumeTerminalSession"),
    actions.indexOf("const launchApplication"),
  );
  assert.match(
    resume,
    /mainSessionCommandStarted\.current = false/,
    "an explicitly resumed session must not auto-exit its page on process end",
  );
  // The bare door shares the spawn path, so it is covered by the truth table
  // rather than by a second flag: `openTerminalSession` passes `null`.
  const bare = actions.slice(
    actions.indexOf("const openTerminalSession"),
    actions.indexOf("const resumeTerminalSession"),
  );
  assert.match(bare, /ensureTerminalSession\(null\)/);
});

// ── 3 · the exit leaves by the manual path ────────────────────────────────

test("a command exit returns to the launcher through returnToInputMode", async () => {
  const hook = stripJsComments(await read("src/hooks/useTerminalView.ts"));
  const listener = hook.slice(
    hook.indexOf("const unlistenExitPromise"),
    hook.indexOf("return () => {\n      unlistenFramePromise"),
  );
  assert.ok(listener.length > 0, "the exit listener must exist");
  // The decision is the shared predicate, read through the once-registered
  // listener's refs (the surface and the mode at the moment of the exit).
  assert.match(
    listener,
    /shouldExitWithCommandSession\(mainSessionCommandStarted\.current, modeRef\.current\)/,
  );
  // The leave itself is the close button's own path, not a bespoke collapse.
  assert.match(listener, /void returnToInputModeRef\.current\(\)/);
  // Nothing floats: no overlay, no feedback row, no notification — the page is
  // simply gone.
  assert.doesNotMatch(listener, /notify\(|showTerminalFeedback\(|setTerminalFeedback\(|toast/i);
  // The once-registered listener cannot close over a stale function or mode.
  assert.match(hook, /const returnToInputModeRef = useRef\(returnToInputMode\)/);
  assert.match(hook, /const modeRef = useRef\(mode\)/);
});

test("the hold-on-exit behaviour still owns handleTerminalExit", async () => {
  // R9-2 slice 5 and R62 are separate: the handler still *holds* the page (the
  // last frame and the exit notice); the automatic exit is an extra branch in
  // the listener, and only for a command session. A mutation that moved the
  // auto-exit into the handler would break both this and the R9-2 guard.
  const hook = stripJsComments(await read("src/hooks/useTerminalView.ts"));
  const at = hook.indexOf("const handleTerminalExit");
  const handler = hook.slice(at, hook.indexOf("const describeMainSession", at));
  assert.match(handler, /setTerminalResident\(\{ code \}\)/);
  assert.doesNotMatch(handler, /shouldExitWithCommandSession|returnToInputMode/);
  assert.ok(!/setMode\("collapsed"\)/.test(handler), "the handler itself must not collapse");
  assert.ok(!/closeTerminalSession\(\)/.test(handler), "the handler itself must not tear down");
});

// ── 4 · the app threads the one flag ──────────────────────────────────────

test("the App owns the flag and hands it to both hooks", async () => {
  const app = stripJsComments(await read("src/App.tsx"));
  assert.match(app, /const mainSessionCommandStarted = useRef\(false\)/);
  // Written by the spawn hook, reset by the resume hook — one shared ref.
  const passed = app.split("mainSessionCommandStarted,").length - 1;
  assert.equal(passed, 2, "both useTerminalView and useLauncherActions receive it");
  assert.match(app, /returnToInputMode,\n\s*setMainSessionIdentity,/);
});

// ── 5 · the guard, as far as it can be honoured ───────────────────────────

test("input is gated off once the PTY exits, so 'typed after exit' cannot hold", async () => {
  // The report's guard — do not auto-exit if the user typed into the shell
  // after the process died — cannot change the outcome: an exited session's
  // input gate is closed, so there is no post-exit keystroke to see. This pins
  // the reason for that degeneracy (falling back to "exit means exit").
  const hook = stripJsComments(await read("src/hooks/useTerminalView.ts"));
  const handler = hook.slice(
    hook.indexOf("const handleTerminalExit"),
    hook.indexOf("const describeMainSession", hook.indexOf("const handleTerminalExit")),
  );
  assert.match(handler, /ptyReady\.current = false/, "the exit closes the input gate");
  const keyboard = stripJsComments(await read("src/hooks/useAppKeyboard.ts"));
  assert.match(
    keyboard,
    /document\.activeElement !== terminalTextInputRef\.current \|\|\s*!surfaceReady\(\)/,
    "keystrokes are dropped unless the surface says it is ready",
  );
});
