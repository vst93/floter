// Locks in the single "the collapsed input owns the keyboard" collector.
//
// The old design spread the guarantee across ad-hoc timers at each call site
// (`focusCollapsedInput(0/90/140)`), which has two failure modes this suite
// pins down: a focus drop that lands *after* the last beat (a native resize
// settling, an OS reveal) was never noticed, and a kept-alive plugin iframe
// taking the keyboard back later was never reclaimed. `src/collapsed-focus.ts`
// centralizes the policy; these tests drive it with injected DOM seams so the
// scheduling, the reclaim policy and the reassert hook are all falsifiable.
//
// Mutations that must turn this red:
//   * drop the `modeRef` gate in `focusCollapsedInputNow` -> the "stays out of
//     other surfaces" case fails;
//   * make `shouldReclaimCollapsedFocus` always return false -> the focusout /
//     window-focus reclaim cases fail;
//   * remove the `card.contains` guard -> the "yields to card controls" case
//     fails;
//   * never call the reassert hook from `syncLauncherHeight` -> the
//     resize-settled case in the integration assertion fails.
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";
import {
  COLLAPSED_FOCUS_BEATS_MS,
  createCollapsedFocusController,
  focusCollapsedInputNow,
  reassertCollapsedFocus,
  setCollapsedFocusReassert,
  shouldReclaimCollapsedFocus,
} from "../src/collapsed-focus.ts";

const root = new URL("../", import.meta.url);
const read = (path: string) => readFile(new URL(path, root), "utf8");
// Source-shape assertions below must look at *code*, not prose: the very
// comment explaining a seam often quotes the identifier being asserted (e.g.
// "See `refocus_webview` in lib.rs"), which would satisfy a naive regex even
// if the code were removed. Strip comments first.
const readCode = async (path: string) =>
  (await read(path))
    .replace(/\/\*[\s\S]*?\*\//g, "")
    .replace(/\/\/[^\n]*/g, "");

type FakeInput = {
  value: string;
  focused: boolean;
  selection: [number, number] | null;
  focusCalls: number;
};

/** A minimal stand-in for the pieces the collector touches on the input. */
function fakeInput(value = ""): HTMLInputElement & FakeInput {
  const input: FakeInput = {
    value,
    focused: false,
    selection: null,
    focusCalls: 0,
    focus() {
      this.focused = true;
      this.focusCalls += 1;
    },
    setSelectionRange(start: number, end: number) {
      this.selection = [start, end];
    },
  };
  return input as unknown as HTMLInputElement & FakeInput;
}

function fakeCard(children: Element[]): HTMLElement {
  const card = {
    contains(node: Element | null) {
      return node !== null && children.includes(node);
    },
  };
  return card as unknown as HTMLElement;
}

const flushMicrotasks = () => new Promise<void>((resolve) => setTimeout(resolve, 0));

test("shouldReclaimCollapsedFocus yields to the input and the card's own controls", () => {
  const input = fakeInput("hi");
  const button = { tagName: "BUTTON" } as unknown as Element;
  const card = fakeCard([button]);

  assert.equal(shouldReclaimCollapsedFocus(input, input, card), false, "input is already home");
  assert.equal(shouldReclaimCollapsedFocus(button, input, card), false, "card controls keep focus");
  assert.equal(shouldReclaimCollapsedFocus(null, input, card), true, "body (null active) is reclaimed");
  assert.equal(
    shouldReclaimCollapsedFocus({ tagName: "IFRAME" } as unknown as Element, input, card),
    true,
    "an outside element (e.g. the kept-alive iframe) is reclaimed",
  );
});

test("focusCollapsedInputNow only fires on the collapsed surface", () => {
  const input = fakeInput("abc");
  const modeRef = { current: "terminal" };
  const refs = { modeRef, inputRef: { current: input }, cardRef: { current: null } };

  assert.equal(focusCollapsedInputNow(refs), false);
  assert.equal(input.focused, false, "must not steal focus from the terminal");

  modeRef.current = "collapsed";
  assert.equal(focusCollapsedInputNow(refs), true);
  assert.equal(input.focused, true);
  assert.deepEqual(input.selection, [3, 3], "caret is placed at the end");
});

test("the controller schedules each beat and re-checks the mode at fire time", async () => {
  const scheduled: Array<{ delay: number }> = [];
  const input = fakeInput("");
  const modeRef = { current: "collapsed" };
  const controller = createCollapsedFocusController({
    refs: { modeRef, inputRef: { current: input }, cardRef: { current: null } },
    schedule: (fn, delay) => {
      scheduled.push({ delay });
      queueMicrotask(fn);
    },
  });

  controller.focus();
  controller.focus(90);
  controller.focus(140);
  assert.deepEqual(
    scheduled.map((entry) => entry.delay),
    [0, 90, 140],
    "the call sites drive the beat delays",
  );

  // A beat that fires after the surface changed must be inert.
  modeRef.current = "settings";
  input.focused = false;
  input.focusCalls = 0;
  await flushMicrotasks();
  assert.equal(input.focusCalls, 0, "a stale beat must not reclaim from another surface");
});

test("the collector's attach() reclaims on focusout and on window focus", () => {
  const input = fakeInput("");
  const card = fakeCard([]);
  const outside = { tagName: "DIV" } as unknown as Element;
  let active: Element | null = outside;
  const focusOutHandlers: Array<() => void> = [];
  const windowFocusHandlers: Array<() => void> = [];
  const microtasks: Array<() => void> = [];

  const controller = createCollapsedFocusController({
    refs: {
      modeRef: { current: "collapsed" },
      inputRef: { current: input },
      cardRef: { current: card },
    },
    schedule: (fn) => microtasks.push(fn),
    activeElement: () => active,
    listen: {
      onFocusOut: (handler) => {
        focusOutHandlers.push(handler);
        return () => undefined;
      },
      onWindowFocus: (handler) => {
        windowFocusHandlers.push(handler);
        return () => undefined;
      },
    },
  });

  const dispose = controller.attach();
  assert.equal(focusOutHandlers.length, 1, "focusout watcher installed once");
  assert.equal(windowFocusHandlers.length, 1, "window focus watcher installed once");

  // Leaving the card: focusout defers a tick, then reclaims.
  active = outside;
  focusOutHandlers[0]();
  for (const task of microtasks.splice(0)) task();
  assert.equal(input.focusCalls, 1, "focus that left the card is reclaimed");

  // A kept-alive iframe taking the keyboard while the input should be home.
  input.focused = false;
  active = { tagName: "IFRAME" } as unknown as Element;
  windowFocusHandlers[0]();
  assert.equal(input.focusCalls, 2, "a window-focus event also reclaims");

  // A card control holding focus must NOT be reclaimed from.
  input.focused = false;
  const cardButton = { tagName: "BUTTON" } as unknown as Element;
  (card as unknown as { contains: (n: Element | null) => boolean }).contains = (n) => n === cardButton;
  active = cardButton;
  windowFocusHandlers[0]();
  assert.equal(input.focusCalls, 2, "card controls keep the keyboard");

  dispose();
  controller.attach(); // idempotent re-attach is allowed
});

test("the collector re-arms the native first responder only after a DOM focus lands", () => {
  const input = fakeInput("");
  const outside = { tagName: "DIV" } as unknown as Element;
  let active: Element | null = { tagName: "IFRAME" } as unknown as Element;
  let nativeRefocuses = 0;
  const microtasks: Array<() => void> = [];
  const modeRef = { current: "collapsed" };

  const controller = createCollapsedFocusController({
    refs: { modeRef, inputRef: { current: input }, cardRef: { current: null } },
    schedule: (fn) => microtasks.push(fn),
    activeElement: () => active,
    nativeRefocus: () => {
      nativeRefocuses += 1;
    },
  });

  // A scheduled beat re-arms the native responder once it lands the DOM focus.
  controller.focus(0);
  for (const task of microtasks.splice(0)) task();
  assert.equal(input.focusCalls, 1, "the DOM focus landed");
  assert.equal(nativeRefocuses, 1, "the native responder is re-armed alongside it");

  // A reclaim from outside the card goes through the same seam…
  input.focused = false;
  active = outside;
  controller.reclaim();
  assert.equal(nativeRefocuses, 2, "reclaim also re-arms the native responder");

  // …but yields (native untouched) when a card control holds the keyboard.
  const cardButton = { tagName: "BUTTON" } as unknown as Element;
  const card = {
    contains: (n: Element | null) => n === cardButton,
  } as unknown as HTMLElement;
  const yielding = createCollapsedFocusController({
    refs: { modeRef, inputRef: { current: input }, cardRef: { current: card } },
    schedule: () => undefined,
    activeElement: () => cardButton,
    nativeRefocus: () => {
      nativeRefocuses += 1;
    },
  });
  const before = nativeRefocuses;
  yielding.reclaim();
  assert.equal(nativeRefocuses, before, "a card control keeps both DOM and native focus");

  // On another surface nothing is touched at all.
  modeRef.current = "terminal";
  const focusCallsBefore = input.focusCalls;
  const nativeBefore = nativeRefocuses;
  controller.reassert();
  assert.equal(input.focusCalls, focusCallsBefore, "reassert stays inert off the collapsed surface");
  assert.equal(nativeRefocuses, nativeBefore, "and never pulls the native responder there");
});

test("reassertCollapsedFocus routes through the process-wide hook", () => {
  const input = fakeInput("");
  const controller = createCollapsedFocusController({
    refs: {
      modeRef: { current: "collapsed" },
      inputRef: { current: input },
      cardRef: { current: null },
    },
    schedule: () => undefined,
  });
  setCollapsedFocusReassert(() => controller.reassert());
  reassertCollapsedFocus();
  assert.equal(input.focusCalls, 1, "the resize-settled hook re-collects focus");
  setCollapsedFocusReassert(null);
  reassertCollapsedFocus();
  assert.equal(input.focusCalls, 1, "clearing the hook is inert");
});

test("syncLauncherHeight re-asserts focus once its native resize settles", async () => {
  const source = await read("src/hooks/useLauncherHeight.ts");
  assert.match(
    source,
    // R15: the handler became a block — it re-asserts focus *and* starts the
    // settle re-measure — so the call is matched inside the resolved arm
    // rather than as a bare expression body.
    /\.then\(\(\)\s*=>\s*\{[\s\S]*?reassertCollapsedFocus\(\)/,
    "the resize completion must re-run the focus collector",
  );
  // And App wires the collector's reassert into that hook.
  const app = await readCode("src/App.tsx");
  assert.match(app, /setCollapsedFocusReassert\(/, "App must register the reassert hook");
  // R7-2 moved the declaration of the beat rhythm into `surface-policy.ts`;
  // App still schedules through it, and the policy row still references the
  // collector's beat list (so the rhythm has exactly one definition).
  assert.match(
    app,
    /surfaceFocusBeats\("collapsed"\)/,
    "App must schedule the collapsed beats through the declared surface policy",
  );
  const policy = await readCode("src/surface-policy.ts");
  assert.match(
    policy,
    /beats:\s*COLLAPSED_FOCUS_BEATS_MS/,
    "the collapsed policy must reference the collector's beat list, not copy it",
  );
});

test("syncLauncherHeight actually re-asserts focus once setSize resolves (runtime)", async () => {
  // The assertion above is a regex over the source; this one drives the real
  // helper with a stubbed Tauri IPC bridge and a resolved `setSize`, so a
  // refactor that keeps the `.then(...)` text but stops calling the hook (or
  // reorders the chain) still fails.
  const globalWindow = globalThis as unknown as {
    window: unknown;
    getComputedStyle: (el: unknown) => Record<string, string>;
    requestAnimationFrame: (cb: () => void) => number;
  };
  const previousWindow = globalWindow.window;
  const previousGetComputedStyle = globalWindow.getComputedStyle;
  const previousRequestAnimationFrame = globalWindow.requestAnimationFrame;
  const calls: Array<{ cmd: string; args: unknown }> = [];
  globalWindow.window = {
    __TAURI_INTERNALS__: {
      metadata: { currentWindow: { label: "main" } },
      invoke: async (cmd: string, args: unknown) => {
        calls.push({ cmd, args });
        return undefined;
      },
      transformCallback: () => 1,
      unregisterCallback: () => undefined,
    },
  };
  // Only the boxes `syncLauncherHeight` measures; the values are irrelevant to
  // the contract under test (a non-zero height must be produced).
  globalWindow.getComputedStyle = () => ({
    display: "block",
    borderTopWidth: "1px",
    borderBottomWidth: "1px",
    paddingTop: "0px",
    paddingBottom: "0px",
  });
  // R15: the helper re-measures after the resize settles, scheduled for the
  // next paint. Stub the frame out (and never run the callback) so the settle
  // pass is observable but cannot touch the stubs after the test has restored
  // them; the fallback timer would otherwise fire mid-teardown.
  let settleFrames = 0;
  globalWindow.requestAnimationFrame = () => {
    settleFrames += 1;
    return 0;
  };

  try {
    const { syncLauncherHeight } = await import("../src/hooks/useLauncherHeight.ts");
    const { setCollapsedFocusReassert: setHook } = await import("../src/collapsed-focus.ts");

    let reasserted = 0;
    setHook(() => {
      reasserted += 1;
    });
    const card = {
      children: [{ offsetTop: 8, offsetHeight: 50 }],
      parentElement: null,
    };
    syncLauncherHeight({ current: card } as unknown as {
      current: HTMLDivElement | null;
    });
    // `setSize` returns a resolved promise; give the `.then` a microtask turn.
    await new Promise((resolve) => setTimeout(resolve, 5));

    assert.equal(
      calls.filter((call) => call.cmd === "plugin:window|set_size").length,
      1,
      "the launcher resize goes through setSize",
    );
    assert.equal(reasserted, 1, "a resolved setSize must run the focus reassert");
    assert.equal(
      settleFrames,
      1,
      "R15: the settle re-measure must be scheduled for the next paint after the resize lands",
    );
    setHook(null);
  } finally {
    globalWindow.window = previousWindow;
    globalWindow.getComputedStyle = previousGetComputedStyle;
    globalWindow.requestAnimationFrame = previousRequestAnimationFrame;
  }
});

test("the macOS native first-responder re-arm is gated, wired and registered", async () => {
  // The DOM-only collector cannot draw a caret on WKWebView; the fix is a
  // native seam that must stay macOS-gated (Linux/Windows keep DOM-only
  // behaviour) and must actually exist on the Rust side.
  const app = await readCode("src/App.tsx");
  assert.match(
    app,
    /nativeRefocus:\s*IS_MAC\s*\?/,
    "the native refocus seam must be compiled in on macOS only",
  );
  assert.match(app, /invoke\("refocus_webview"\)/, "App must request the native refocus");

  const lib = await readCode("src-tauri/src/lib.rs");
  assert.match(lib, /fn refocus_webview\(/, "backend must define the command");
  assert.match(lib, /refocus_webview,/, "backend must register it in generate_handler!");
  assert.ok(
    lib.includes("webview.set_focus()"),
    "the command must drive Webview::set_focus (wry's makeFirstResponder on macOS)",
  );

  // Source-level hardening of the panel-reveal path: `show_and_make_key`
  // installs the *content view* as first responder, and its deferred retry can
  // land after the frontend has re-armed — so the native reveal path must
  // re-point at the web view both immediately and after the retry.
  assert.match(lib, /fn arm_macos_webview_responder\(/, "a reveal-path re-arm helper must exist");
  assert.match(
    lib,
    /arm_macos_webview_responder\(window\)/,
    "the reveal path re-arms right after show_and_make_key",
  );
  assert.match(
    lib,
    /arm_macos_webview_responder\(&retry_window\)/,
    "the deferred key retry re-arms after its own show_and_make_key",
  );
});

test("no call site schedules its own collapsed-focus delay literals", async () => {
  // The whole point is one collector: the individual 90/140/80/20 ms literals
  // that used to be repeated per path are gone, so no path can silently miss a
  // beat again.
  const app = await read("src/App.tsx");
  assert.equal(
    /focusCollapsedInput\(\s*(20|80|90|140)\s*\)/.test(app),
    false,
    "collapsed focus delays must come from COLLAPSED_FOCUS_BEATS_MS",
  );
  assert.ok(
    COLLAPSED_FOCUS_BEATS_MS.includes(0) && COLLAPSED_FOCUS_BEATS_MS.includes(90) && COLLAPSED_FOCUS_BEATS_MS.includes(140),
    "the shared beat list preserves the historical 0/90/140 rhythm",
  );
});
