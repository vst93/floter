// R7-2 · `runDismissAction` — the side-effect layer of the dismiss table.
//
// `surface-policy.test.ts` pins the *data* (`DISMISS_TABLE` cells) and the
// *call sites* (the handler asks `resolveDismissRule`). Neither can see what
// `runDismissAction` then does with the rule. That gap let the R7-2 review
// delete any of these four side effects with the whole 185-test suite still
// green:
//
//   1. the macOS keyup re-arm block (`if (rule.reassertOnKeyUp) { ... }`) —
//      the 58c6d24 first-responder fix silently gone;
//   2. `event.preventDefault()` — the press stops being consumed;
//   3. the `case "close-settings"` body calling `closePluginPage()`;
//   4. `if (rule.stopPropagation) event.stopPropagation()`.
//
// These tests close the gap by *executing* the handler. The hook itself is
// React-bound, so the source of `runDismissAction` + `onKeyDown` is lifted out
// of `useAppKeyboard.ts` (comment-stripped, anchored on the two `const`
// declarations) and compiled with injected seams: a fake `window` that records
// listeners/timers, a fake `document`, and a call-recording dependency bag.
// The assertions are therefore behavioural — they fail if the side effect
// stops happening, regardless of how the line is spelled.
//
// Each review mutation maps to a red assertion:
//   * mutation 1 -> "terminal new-command re-arms the input on key release"
//                   + "... only the reassertOnKeyUp rule installs a keyup
//                   re-arm" + "runDismissAction keeps the 1500ms / modifier
//                   release guard" (source-shape backstop);
//   * mutation 2 -> every "consumes the press" assertion (preventDefault);
//   * mutation 3 -> "settings ⌘W dispatches close-settings" (closePluginPage
//                   would fire instead);
//   * mutation 4 -> the "stopPropagation" assertions on the settings/plugin
//                   ⌘W presses.
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";
import { DISMISS_TABLE } from "../src/surface-policy.ts";

const ROOT = new URL("../", import.meta.url);
const readText = (path: string) => readFileSync(new URL(path, ROOT), "utf8");
const stripComments = (source: string) =>
  source.replace(/\/\*[\s\S]*?\*\//g, "").replace(/\/\/[^\n]*/g, "");

/**
 * Lift `runDismissAction` + `onKeyDown` out of the hook's effect. Anchored on
 * the two structural declarations (not a fixed window), so the slice follows
 * the code as it moves.
 */
function extractHandlerSource(code: string): string {
  const start = code.indexOf("const runDismissAction");
  const endMarker = 'window.addEventListener("keydown", onKeyDown);';
  const end = code.indexOf(endMarker, start) + endMarker.length;
  assert.ok(start > -1, "runDismissAction must be declared in useAppKeyboard.ts");
  assert.ok(end > start, "the window keydown registration must follow it");
  // TypeScript-only syntax the node parser cannot read: parameter type
  // annotations (restricted to the types this handler actually uses, so a
  // future object literal is not mistaken for one) and one generic call.
  const TYPE = "(?:KeyboardEvent|HTMLElement|DismissRule|number|string|boolean|void)";
  return code
    .slice(start, end)
    .replace(new RegExp(`(\\b\\w+)\\s*:\\s*${TYPE}(?:\\s*\\|\\s*${TYPE})*?(?=\\s*[,)])`, "g"), "$1")
    .replace(/<HTMLButtonElement>\(/g, "(");
}

/** Names the compiled handler closes over; every one is an injectable seam. */
const DEP_NAMES = [
  "window",
  "document",
  "HTMLElement",
  "invoke",
  "encodeKey",
  "isTerminalCompositionKey",
  "shouldUseTerminalTextInput",
  "IS_MAC",
  "matchesResultShortcut",
  "matchesShortcut",
  "resolveDismissRule",
  "surfaceYieldsToModal",
  "isArrowKeyEditableTarget",
  "nextSettingsPage",
  "mode",
  "shortcuts",
  "recordingAction",
  "launcherResults",
  "query",
  "inputRef",
  "selectionRef",
  "dimsRef",
  "surfaceReady",
  "terminalTextInputRef",
  "terminalInputTarget",
  "activeRenderer",
  "activeSurfaceRef",
  "setActiveSurface",
  "focusCollapsedInput",
  "returnToInputMode",
  "openInTerminal",
  "togglePinnedTerminal",
  "copySelection",
  "pasteClipboard",
  "closeSettings",
  "openSettings",
  "settingsPage",
  "changeSettingsPage",
  "settingsSidebarButtons",
  "refreshTerminalSessions",
  "closePluginPage",
  "runLauncherItem",
  "handleLauncherKey",
  "onLauncherDismiss",
  "resultShortcutSlots",
  "setQuery",
  "setHistory",
  "showLauncherFeedback",
  "setHistoryIndex",
  "collapsedCardRef",
] as const;

type Deps = Record<(typeof DEP_NAMES)[number], unknown>;

let compiled: Function | null = null;
function compileHandler(): Function {
  if (compiled) return compiled;
  const code = stripComments(readText("src/hooks/useAppKeyboard.ts"));
  const source = extractHandlerSource(code);
  compiled = new Function(...DEP_NAMES, `${source}\nreturn { runDismissAction, onKeyDown };`);
  return compiled;
}

// ---------------------------------------------------------------------------
// Harness
// ---------------------------------------------------------------------------

type Recorded = {
  invoke: Array<{ cmd: string; args: unknown }>;
  setActiveSurface: string[];
  focusCollapsedInput: unknown[];
  returnToInputMode: number[];
  openInTerminal: number[];
  togglePinnedTerminal: number[];
  copySelection: number[];
  pasteClipboard: number[];
  closeSettings: number[];
  openSettings: unknown[];
  changeSettingsPage: string[];
  refreshTerminalSessions: number[];
  closePluginPage: number[];
  runLauncherItem: unknown[];
  handleLauncherKey: unknown[];
  setQuery: Array<(current: string) => string>;
  setHistory: unknown[];
  showLauncherFeedback: string[];
  setHistoryIndex: unknown[];
};

function fakeWindow() {
  const listeners: Record<string, Array<(event: unknown) => void>> = {};
  const timers: Array<{ fn: () => void; ms: number }> = [];
  return {
    listeners,
    timers,
    addEventListener(type: string, fn: (event: unknown) => void) {
      (listeners[type] ||= []).push(fn);
    },
    removeEventListener(type: string, fn: (event: unknown) => void) {
      const bucket = listeners[type] || [];
      const index = bucket.indexOf(fn);
      if (index >= 0) bucket.splice(index, 1);
    },
    setTimeout(fn: () => void, ms: number) {
      timers.push({ fn, ms });
      return timers.length;
    },
    fire(type: string, event: unknown) {
      for (const fn of [...(listeners[type] || [])]) fn(event);
    },
  };
}

type KeyOverrides = Partial<{
  key: string;
  code: string;
  ctrlKey: boolean;
  metaKey: boolean;
  altKey: boolean;
  shiftKey: boolean;
  target: unknown;
  defaultPrevented: boolean;
}>;

/**
 * Minimal stand-in for `HTMLElement`: the handler only ever asks it for
 * `closest(selector)`. Instances are created with the answers they should give,
 * so a test can put the event target inside (or outside) a modal without a DOM.
 */
class FakeHTMLElement {
  private readonly closestMap: Record<string, unknown>;
  constructor(closestMap: Record<string, unknown> = {}) {
    this.closestMap = closestMap;
  }
  closest(selector: string): unknown {
    return this.closestMap[selector] ?? null;
  }
}

function keyEvent(overrides: KeyOverrides = {}) {
  return {
    key: "",
    code: "",
    ctrlKey: false,
    metaKey: false,
    altKey: false,
    shiftKey: false,
    target: null,
    defaultPrevented: false,
    preventCalls: 0,
    stopCalls: 0,
    preventDefault() {
      this.preventCalls += 1;
      this.defaultPrevented = true;
    },
    stopPropagation() {
      this.stopCalls += 1;
    },
    ...overrides,
  };
}

function makeHarness(options: {
  mode: string;
  dismiss: { action: string; stopPropagation?: boolean; reassertOnKeyUp?: boolean } | null;
  /** Extra dependency seams (modal guard, page navigator, sidebar buttons). */
  overrides?: Partial<Deps>;
}) {
  const calls: Recorded = {
    invoke: [],
    setActiveSurface: [],
    focusCollapsedInput: [],
    returnToInputMode: [],
    openInTerminal: [],
    togglePinnedTerminal: [],
    copySelection: [],
    pasteClipboard: [],
    closeSettings: [],
    openSettings: [],
    changeSettingsPage: [],
    refreshTerminalSessions: [],
    closePluginPage: [],
    runLauncherItem: [],
    handleLauncherKey: [],
    setQuery: [],
    setHistory: [],
    showLauncherFeedback: [],
    setHistoryIndex: [],
  };
  const win = fakeWindow();
  const deps: Deps = {
    window: win,
    document: { activeElement: null },
    // The handler only uses this in an `instanceof` check; a local class is
    // enough and keeps the node process free of a DOM shim.
    HTMLElement: FakeHTMLElement,
    invoke: (cmd: string, args: unknown) => calls.invoke.push({ cmd, args }),
    encodeKey: () => null,
    isTerminalCompositionKey: () => false,
    shouldUseTerminalTextInput: () => false,
    IS_MAC: false,
    matchesResultShortcut: () => null,
    matchesShortcut: () => false,
    resolveDismissRule: () => options.dismiss,
    surfaceYieldsToModal: () => false,
    isArrowKeyEditableTarget: () => false,
    nextSettingsPage: (page: string) => page,
    mode: options.mode,
    shortcuts: {},
    recordingAction: null,
    launcherResults: [],
    query: "",
    inputRef: { current: null },
    selectionRef: { current: null },
    dimsRef: { current: { cols: 80, rows: 24 } },
    surfaceReady: () => true,
    terminalTextInputRef: { current: null },
    terminalInputTarget: () => "main",
    activeRenderer: () => null,
    activeSurfaceRef: { current: "main" },
    setActiveSurface: (surface: string) => calls.setActiveSurface.push(surface),
    focusCollapsedInput: (delay: number) => calls.focusCollapsedInput.push(delay),
    returnToInputMode: () => calls.returnToInputMode.push(1),
    openInTerminal: () => calls.openInTerminal.push(1),
    togglePinnedTerminal: () => calls.togglePinnedTerminal.push(1),
    copySelection: () => calls.copySelection.push(1),
    pasteClipboard: () => calls.pasteClipboard.push(1),
    closeSettings: () => calls.closeSettings.push(1),
    openSettings: (page?: unknown) => calls.openSettings.push(page),
    settingsPage: "general",
    changeSettingsPage: (page: string) => calls.changeSettingsPage.push(page),
    settingsSidebarButtons: { current: new Map() },
    refreshTerminalSessions: () => calls.refreshTerminalSessions.push(1),
    closePluginPage: () => calls.closePluginPage.push(1),
    runLauncherItem: (item: unknown) => calls.runLauncherItem.push(item),
    handleLauncherKey: (event: unknown) => calls.handleLauncherKey.push(event),
    // R31 · the collapsed surface's mode-aware Esc/Cmd+W rule. Default: it
    // claims nothing, so the dismiss table behaves exactly as it did before the
    // rule existed; the rule's own behaviour is pinned in `plugin-fusion.test.ts`.
    onLauncherDismiss: () => false,
    resultShortcutSlots: [],
    setQuery: (updater: (current: string) => string) => calls.setQuery.push(updater),
    setHistory: (value: unknown) => calls.setHistory.push(value),
    showLauncherFeedback: (key: string) => calls.showLauncherFeedback.push(key),
    setHistoryIndex: (value: unknown) => calls.setHistoryIndex.push(value),
    collapsedCardRef: { current: null },
    ...options.overrides,
  };
  const api = compileHandler()(...DEP_NAMES.map((name) => deps[name])) as {
    runDismissAction: (
      rule: { action: string; stopPropagation?: boolean; reassertOnKeyUp?: boolean },
      event: unknown,
    ) => void;
    onKeyDown: (event: unknown) => void;
  };
  return { api, calls, win };
}

// ---------------------------------------------------------------------------
// preventDefault — the press is consumed
// ---------------------------------------------------------------------------

test("every dismiss action consumes the press (preventDefault)", () => {
  const cases: Array<[string, string]> = [
    ["settings", "close-settings"],
    ["collapsed", "hide-window"],
    ["terminal", "return-to-input"],
  ];
  for (const [mode, action] of cases) {
    const h = makeHarness({ mode, dismiss: { action } });
    const event = keyEvent({ key: "Escape", code: "Escape" });
    h.api.onKeyDown(event);
    assert.equal(
      event.preventCalls,
      1,
      `${mode}/${action} must call preventDefault exactly once`,
    );
    assert.equal(event.defaultPrevented, true, `${mode}/${action} must mark the press consumed`);
  }
});

// ---------------------------------------------------------------------------
// stopPropagation — consumed only where the rule asks for it
// ---------------------------------------------------------------------------

test("settings ⌘W dispatches close-settings and stops propagation", () => {
  const h = makeHarness({
    mode: "settings",
    dismiss: { action: "close-settings", stopPropagation: true },
  });
  const event = keyEvent({ key: "w", code: "KeyW", metaKey: true });
  h.api.onKeyDown(event);

  assert.equal(event.preventCalls, 1, "the ⌘W press must be consumed");
  assert.equal(event.stopCalls, 1, "the settings ⌘W rule must stop propagation");
  assert.deepEqual(h.calls.closeSettings, [1], "close-settings must run");
  assert.deepEqual(h.calls.closePluginPage, [], "close-plugin must NOT run for settings");
});

test("the retired plugin surface has no dismiss rule left in the table", () => {
  // R33 · the plugin page is gone; its configuration is an overlay inside the
  // collapsed surface, and Esc there is the launcher's own three-level rule
  // (`onLauncherDismiss`), not a window-level `close-plugin` action. A stale
  // table row would be a key handler for a surface that no longer exists.
  assert.equal(
    (DISMISS_TABLE as Record<string, unknown>).plugin,
    undefined,
    "the plugin row must be removed, not merely unreachable",
  );
});

test("a rule without stopPropagation leaves propagation alone", () => {
  const h = makeHarness({ mode: "settings", dismiss: { action: "close-settings" } });
  const event = keyEvent({ key: "Escape", code: "Escape" });
  h.api.onKeyDown(event);

  assert.equal(event.preventCalls, 1, "Escape still consumes the press");
  assert.equal(event.stopCalls, 0, "plain Escape must not stop propagation");
  assert.deepEqual(h.calls.closeSettings, [1]);
});

// ---------------------------------------------------------------------------
// action -> callback dispatch
// ---------------------------------------------------------------------------

test("hide-window invokes the native command", () => {
  const h = makeHarness({ mode: "collapsed", dismiss: { action: "hide-window" } });
  const event = keyEvent({ key: "Escape", code: "Escape" });
  h.api.onKeyDown(event);

  assert.deepEqual(h.calls.invoke, [{ cmd: "hide_window", args: undefined }]);
  assert.deepEqual(h.calls.closeSettings, []);
  assert.deepEqual(h.calls.closePluginPage, []);
});

test("return-to-input calls the mode-return seam, not a close", () => {
  const h = makeHarness({ mode: "terminal", dismiss: { action: "return-to-input" } });
  const event = keyEvent({ key: "w", code: "KeyW", ctrlKey: true });
  h.api.onKeyDown(event);

  assert.deepEqual(h.calls.returnToInputMode, [1]);
  assert.deepEqual(h.calls.closeSettings, []);
  assert.deepEqual(h.calls.closePluginPage, []);
  assert.deepEqual(h.calls.invoke, []);
});

// ---------------------------------------------------------------------------
// macOS keyup re-arm (58c6d24) — the fix must stay wired
// ---------------------------------------------------------------------------

const releaseEvent = (modifiers: KeyOverrides = {}) =>
  keyEvent({ key: "Meta", code: "MetaLeft", ...modifiers });

test("terminal new-command re-arms the input on key release", () => {
  const h = makeHarness({
    mode: "terminal",
    dismiss: { action: "return-to-input", reassertOnKeyUp: true },
  });
  const event = keyEvent({ key: "w", code: "KeyW", metaKey: true });
  h.api.onKeyDown(event);

  assert.deepEqual(h.calls.returnToInputMode, [1], "the action still runs");
  assert.equal(
    (h.win.listeners.keyup || []).length,
    1,
    "a keyup re-arm listener must be installed",
  );
  assert.deepEqual(
    h.win.timers.map((timer) => timer.ms),
    [1500],
    "the re-arm must self-disarm after 1500ms",
  );

  // A modifier still held is not the release we wait for.
  h.win.fire("keyup", releaseEvent({ metaKey: true }));
  assert.equal(h.calls.focusCollapsedInput.length, 0, "a held modifier is not a release");
  assert.equal((h.win.listeners.keyup || []).length, 1, "the listener stays armed");

  // The real release re-focuses the launcher input and disarms.
  h.win.fire("keyup", releaseEvent());
  assert.equal(h.calls.focusCollapsedInput.length, 1, "the clean release re-asserts the input");
  assert.equal((h.win.listeners.keyup || []).length, 0, "and removes the listener");
});

test("the 1500ms timeout disarms the keyup re-arm if no release arrives", () => {
  const h = makeHarness({
    mode: "terminal",
    dismiss: { action: "return-to-input", reassertOnKeyUp: true },
  });
  h.api.onKeyDown(keyEvent({ key: "w", code: "KeyW", metaKey: true }));
  assert.equal((h.win.listeners.keyup || []).length, 1);

  h.win.timers[0].fn();
  assert.equal((h.win.listeners.keyup || []).length, 0, "the timeout must remove the listener");
  assert.equal(h.calls.focusCollapsedInput.length, 0, "and must not itself re-focus");
});

test("only the reassertOnKeyUp rule installs a keyup re-arm", () => {
  for (const action of ["close-settings", "hide-window", "return-to-input"]) {
    const h = makeHarness({ mode: "collapsed", dismiss: { action } });
    h.api.onKeyDown(keyEvent({ key: "Escape", code: "Escape" }));
    assert.equal(
      (h.win.listeners.keyup || []).length,
      0,
      `${action} without reassertOnKeyUp must not arm a keyup listener`,
    );
    assert.deepEqual(h.win.timers, [], `${action} without reassertOnKeyUp must not schedule a timer`);
  }
});

// ---------------------------------------------------------------------------
// Source-shape backstop: the re-arm block stays bound to the terminal rule.
// ---------------------------------------------------------------------------

test("runDismissAction keeps the 1500ms / modifier release guard", () => {
  const code = stripComments(readText("src/hooks/useAppKeyboard.ts"));
  const source = extractHandlerSource(code);
  // Anchor on the reassert branch; the assertions below must live inside it.
  const branchStart = source.indexOf("if (rule.reassertOnKeyUp) {");
  assert.ok(branchStart > -1, "the reassertOnKeyUp branch must exist");
  const branchEnd = source.indexOf("switch (rule.action)", branchStart);
  assert.ok(branchEnd > branchStart, "the branch must sit before the action dispatch");
  const branch = source.slice(branchStart, branchEnd);

  assert.match(
    branch,
    /release\.metaKey \|\| release\.ctrlKey \|\| release\.altKey \|\| release\.shiftKey/,
    "the release handler must ignore a keyup that still holds a modifier",
  );
  assert.match(branch, /focusCollapsedInput\(\)/, "the release must re-assert the launcher input");
  assert.match(branch, /\}, 1500\)/, "the re-arm must self-disarm after 1500ms");
});

test("the reassertOnKeyUp flag is bound to the terminal new-command cell", () => {
  const policy = stripComments(readText("src/surface-policy.ts"));
  const tableStart = policy.indexOf("export const DISMISS_TABLE");
  assert.ok(tableStart > -1, "DISMISS_TABLE must exist");
  const terminalStart = policy.indexOf("terminal: {", tableStart);
  const settingsStart = policy.indexOf("settings: {", terminalStart);
  assert.ok(terminalStart > -1 && settingsStart > terminalStart, "the terminal row must exist");
  const terminalRow = policy.slice(terminalStart, settingsStart);

  assert.match(
    terminalRow,
    /"new-command":\s*\{\s*action:\s*"return-to-input",\s*reassertOnKeyUp:\s*true\s*\}/,
    "only terminal's new-command may ask for the keyup re-arm",
  );
  assert.equal(
    (terminalRow.match(/reassertOnKeyUp/g) || []).length,
    1,
    "exactly one terminal cell re-arms",
  );
});

// ---------------------------------------------------------------------------
// Settings modal guard — the surface must stand down while a dialog is open.
//
// The acceptance criterion is "modal open => the frame's focus never regresses
// and the surface rules stay put". `surfaceYieldsToModal` + the `[aria-modal]`
// test is what makes that true; before this block, deleting the guard left the
// whole suite green.
// ---------------------------------------------------------------------------

test("settings yields every key to an open modal (no dismiss, no page switch)", () => {
  // Pretend the press happened inside a dialog. The guard must return before
  // the sidebar arrows AND before the dismiss table resolves.
  const target = new FakeHTMLElement({ '[aria-modal="true"]': {} });
  const h = makeHarness({
    mode: "settings",
    dismiss: { action: "close-settings" },
    overrides: {
      surfaceYieldsToModal: () => true,
      isArrowKeyEditableTarget: () => false,
      nextSettingsPage: () => "sessions",
      settingsSidebarButtons: { current: new Map() },
    },
  });
  for (const key of ["Escape", "w", "ArrowDown"]) {
    const event = keyEvent({ key, code: key, metaKey: key === "w", target });
    h.api.onKeyDown(event);
    assert.equal(event.preventCalls, 0, `${key} inside a modal must not be consumed by the surface`);
  }
  assert.deepEqual(h.calls.closeSettings, [], "the modal closes itself; the surface must not also close");
  assert.deepEqual(h.calls.changeSettingsPage, [], "↑/↓ must not switch pages under the dialog");
});

test("the modal guard is gated by surfaceYieldsToModal, not by `mode`", () => {
  // Same aria-modal target, but the surface is not one that yields: the guard
  // must not fire, so the dismiss rule still runs.
  const target = new FakeHTMLElement({ '[aria-modal="true"]': {} });
  const h = makeHarness({
    mode: "settings",
    dismiss: { action: "close-settings" },
    overrides: { surfaceYieldsToModal: () => false },
  });
  const event = keyEvent({ key: "Escape", code: "Escape", target });
  h.api.onKeyDown(event);
  assert.deepEqual(h.calls.closeSettings, [1], "without the declaration the surface keeps handling keys");
});

// ---------------------------------------------------------------------------
// Settings sidebar ↑/↓ — the entry focus only pays off if the arrows drive it.
// ---------------------------------------------------------------------------

test("settings ↑/↓ moves focus to the next sidebar button and switches page", () => {
  const focused: string[] = [];
  const buttons = new Map<string, { focus: () => void }>([
    ["sessions", { focus: () => focused.push("sessions") }],
    ["about", { focus: () => focused.push("about") }],
  ]);
  const h = makeHarness({
    mode: "settings",
    dismiss: null,
    overrides: {
      settingsPage: "general",
      nextSettingsPage: (_page: string, direction: string) =>
        direction === "down" ? "sessions" : "about",
      settingsSidebarButtons: { current: buttons },
    },
  });

  h.api.onKeyDown(keyEvent({ key: "ArrowDown", code: "ArrowDown" }));
  assert.deepEqual(focused, ["sessions"], "↓ must land the keyboard on the next item");
  assert.deepEqual(h.calls.changeSettingsPage, ["sessions"], "↓ must commit the page switch");

  h.api.onKeyDown(keyEvent({ key: "ArrowUp", code: "ArrowUp" }));
  assert.deepEqual(focused, ["sessions", "about"], "↑ must land the keyboard on the previous item");
  assert.deepEqual(h.calls.changeSettingsPage, ["sessions", "about"]);
});

test("settings ↑/↓ leaves editable and modal targets alone", () => {
  const h = makeHarness({
    mode: "settings",
    dismiss: null,
    overrides: {
      isArrowKeyEditableTarget: () => true,
      nextSettingsPage: () => "sessions",
      settingsSidebarButtons: { current: new Map() },
    },
  });
  const event = keyEvent({ key: "ArrowDown", code: "ArrowDown" });
  h.api.onKeyDown(event);
  assert.equal(event.preventCalls, 0, "an input keeps its caret movement");
  assert.deepEqual(h.calls.changeSettingsPage, [], "no page switch from an editable target");
});

test("the arrow handler refreshes sessions when the switch lands on that page", () => {
  const h = makeHarness({
    mode: "settings",
    dismiss: null,
    overrides: {
      nextSettingsPage: () => "sessions",
      settingsSidebarButtons: { current: new Map() },
    },
  });
  h.api.onKeyDown(keyEvent({ key: "ArrowDown", code: "ArrowDown" }));
  assert.deepEqual(h.calls.refreshTerminalSessions, [1], "landing on sessions must refresh it");
});

// ---------------------------------------------------------------------------
// Dialog focus (58c6d24 + the inert walk) must survive the new entry focus.
//
// R7-2 lands the keyboard on the sidebar. Opening a dialog inside settings must
// still make everything outside it inert up to the `.settings-card` boundary,
// and closing must restore the element that had the keyboard (now the sidebar
// item) unless it has itself gone inert. `useDialogFocus` is React-bound, so
// this is the source-shape backstop the file already uses for side effects.
// ---------------------------------------------------------------------------

test("the dialog inert walk still stops at the settings-card boundary", async () => {
  const panel = stripComments(readText("src/ExtensionsPanel.tsx"));
  const walkStart = panel.indexOf("const inertElements");
  assert.ok(walkStart > -1, "useDialogFocus must keep its inert walk");
  const walkEnd = panel.indexOf("const handleKeyDown", walkStart);
  assert.ok(walkEnd > walkStart, "the walk must precede the key handler");
  const walk = panel.slice(walkStart, walkEnd);

  assert.match(
    walk,
    /sibling\.inert\s*=\s*true/,
    "siblings outside the dialog must be made inert",
  );
  assert.match(
    walk,
    /classList\.contains\("settings-card"\)\s*\)?\s*break/,
    "the walk must stop at the settings-card boundary",
  );
});

test("closing a dialog restores focus without reviving an inert target", async () => {
  const panel = stripComments(readText("src/ExtensionsPanel.tsx"));
  const restoreStart = panel.indexOf("previouslyFocused?.isConnected");
  assert.ok(restoreStart > -1, "the restore block must exist");
  const restore = panel.slice(restoreStart, restoreStart + 200);
  assert.match(
    restore,
    /!previouslyFocused\.closest\("\[inert\]"\)/,
    "a target that is inert (or inside an inert subtree) must not be refocused",
  );
});
