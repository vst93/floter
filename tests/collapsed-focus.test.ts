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
    /\.then\(\(\)\s*=>\s*reassertCollapsedFocus\(\)\)/,
    "the resize completion must re-run the focus collector",
  );
  // And App wires the collector's reassert into that hook.
  const app = await read("src/App.tsx");
  assert.match(app, /setCollapsedFocusReassert\(/, "App must register the reassert hook");
  assert.match(
    app,
    /COLLAPSED_FOCUS_BEATS_MS/,
    "App must schedule through the shared beat list, not ad-hoc constants",
  );
  assert.equal(
    (app.match(/COLLAPSED_FOCUS_BEATS_MS/g) ?? []).length >= 1,
    true,
    "the shared beat list is used",
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
