// R53 · the history delete moves off ⌘⌫.
//
// The user's report, verbatim: 「还有 cmd+空格删除这种逻辑不合适，换个快捷键」.
//
// R50's shared single-row delete was `CmdOrCtrl+Backspace` (⌘⌫ on macOS), which
// is macOS's own "delete to the beginning of the line" in every Cocoa text
// field. A hand that already has that gesture presses ⌘⌫ to trim what it typed
// and instead *arms a destructive row delete*. R53 changes the constant to a
// literal `Ctrl+Backspace` — ⌃⌫ on macOS (unbound there: word-delete is ⌥⌫, the
// emacs edits are ⌃H/⌃D/⌃K), Ctrl+⌫ elsewhere, exactly the key non-macOS already
// had. The arm/confirm machine is untouched.
//
// Mutations that must turn this file red:
//   * putting `CmdOrCtrl+Backspace` (or any ⌘-modifier spelling) back
//     -> the constant and the ⌘⌫-does-not-match assertions;
//   * a `CmdOrCtrl`-free but wrong key (⌘E, F2, ⇧⌫, plain ⌫) -> the key grammar;
//   * dropping `formatShortcut` from the hint so the displayed key drifts
//     -> "one source for the key name";
//   * a key handler that stops asking `matchesShortcut(event, HISTORY_DELETE_SHORTCUT)`
//     -> the wiring assertions.

import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

import {
  CALCULATOR_FAVORITE_SHORTCUT,
  CLIPBOARD_FAVORITE_SHORTCUT,
  HISTORY_DELETE_SHORTCUT,
} from "../src/launcher.ts";
import { HISTORY_DELETE_CONFIRM_MS, reduceHistoryDelete } from "../src/plugins/history-actions.ts";
import {
  DEFAULT_SHORTCUTS,
  IS_MAC,
  formatShortcut,
  matchesShortcut,
} from "../src/shortcuts.ts";

const root = new URL("../", import.meta.url);
const read = (path: string) => readFile(new URL(path, root), "utf8");
const stripJsComments = (src: string) =>
  src.replace(/\/\*[\s\S]*?\*\//g, "").replace(/\/\/[^\n]*/g, "");

const keyEvent = (init: Partial<KeyboardEvent> & { key: string }) =>
  ({
    ctrlKey: false,
    metaKey: false,
    altKey: false,
    shiftKey: false,
    code: `Key${init.key.toUpperCase()}`,
    ...init,
  }) as KeyboardEvent;

/** A Backspace event with the platform's Control key held. `⌃` is `ctrlKey` on
 *  every platform — that is the point of a literal `Ctrl` in the constant. */
const ctrlBackspace = keyEvent({ key: "Backspace", ctrlKey: true, code: "Backspace" });
/** ⌘⌫: the gesture the user complained about. On macOS it is the app modifier
 *  (⌘), so this is exactly the event that used to arm a delete. */
const cmdBackspace = keyEvent({ key: "Backspace", metaKey: true, code: "Backspace" });

// ── 1 · the constant ─────────────────────────────────────────────────────

test("R53 · the delete key is a literal Ctrl+⌫, and it is one definition", async () => {
  assert.equal(HISTORY_DELETE_SHORTCUT, "Ctrl+Backspace");
  // No `CmdOrCtrl` and no `Cmd`/`Meta`: the whole point is that ⌘ is not in it.
  assert.equal(/cmd|meta|super/i.test(HISTORY_DELETE_SHORTCUT), false);

  // One definition, beside the other mode-local keys, so the handler, the hint
  // and the docs cannot drift apart.
  const launcher = await read("src/launcher.ts");
  assert.equal(launcher.match(/export const HISTORY_DELETE_SHORTCUT/g)?.length, 1);
  const sources = await Promise.all([
    read("src/hooks/useLauncherActions.ts"),
    read("src/App.tsx"),
    read("src/plugins/history-actions.ts"),
  ]);
  for (const src of sources) {
    assert.equal(
      /HISTORY_DELETE_SHORTCUT\s*=\s*"/.test(src),
      false,
      "the key must be declared once, in launcher.ts",
    );
  }
});

// ── 2 · the key grammar ──────────────────────────────────────────────────

test("R53 · ⌃⌫ matches on every platform; ⌘⌫ and its neighbours do not", () => {
  assert.ok(matchesShortcut(ctrlBackspace, HISTORY_DELETE_SHORTCUT));

  // The regression, exactly: ⌘⌫ must be free on every platform. On macOS it is
  // the binding R50 shipped; on Windows/Linux a `metaKey` event is the Super/Win
  // key, a different combination than this binding — free there too.
  assert.equal(
    matchesShortcut(cmdBackspace, HISTORY_DELETE_SHORTCUT),
    false,
    "⌘⌫ is a text edit (delete to line start), never the row delete",
  );

  // The migration is deliberate and one-sided: macOS moves off the app modifier,
  // while non-macOS resolves to the same Ctrl+⌫ R50 already used — no Windows or
  // Linux regression hid inside the fix.
  assert.equal(
    matchesShortcut(ctrlBackspace, "CmdOrCtrl+Backspace"),
    IS_MAC ? false : true,
    "only macOS changes",
  );

  // The neighbours the key must not swallow.
  const rejects: Partial<KeyboardEvent>[] = [
    { ctrlKey: true, shiftKey: true }, // R32-era Ctrl+Shift+⌫ (clear typed history)
    { ctrlKey: true, altKey: true },
    { shiftKey: true }, // ⇧⌫ selects backwards
    {}, // plain ⌫ edits the field
  ];
  for (const init of rejects) {
    assert.equal(
      matchesShortcut(keyEvent({ key: "Backspace", code: "Backspace", ...init }), HISTORY_DELETE_SHORTCUT),
      false,
      `Backspace with ${JSON.stringify(init)} is not the delete key`,
    );
  }
  // A Control held on some *other* key is not it either.
  assert.equal(
    matchesShortcut(keyEvent({ key: "e", ctrlKey: true }), HISTORY_DELETE_SHORTCUT),
    false,
  );
});

test("R53 · the key collides with no other binding the launcher owns", () => {
  const others = [
    CLIPBOARD_FAVORITE_SHORTCUT,
    CALCULATOR_FAVORITE_SHORTCUT,
    "Ctrl+Shift+Backspace",
    `${IS_MAC ? "Cmd" : "Ctrl"}+Enter`,
    ...Object.values(DEFAULT_SHORTCUTS),
  ];
  for (const binding of others) {
    assert.equal(
      matchesShortcut(ctrlBackspace, binding),
      false,
      `⌃⌫ must not also be ${binding}`,
    );
  }
  // Positive control: the sweep is testing a real event, not an empty one.
  assert.ok(matchesShortcut(ctrlBackspace, HISTORY_DELETE_SHORTCUT));
});

test("R53 · the key is drawn as the delete key it is", () => {
  assert.equal(formatShortcut(HISTORY_DELETE_SHORTCUT), IS_MAC ? "⌃⌫" : "Ctrl+⌫");
});

// ── 3 · the armed / confirm flow, driven through the real key ─────────────

test("R53 · a second ⌃⌫ confirms, ⌘⌫ disarms instead", () => {
  // The App's armed branch, reduced to the one decision that changed:
  // `deleteKey = matchesShortcut(event, HISTORY_DELETE_SHORTCUT)`.
  const isDeleteKey = (event: KeyboardEvent) => matchesShortcut(event, HISTORY_DELETE_SHORTCUT);

  const armed = reduceHistoryDelete(null, { type: "press", id: "a", now: 1000 });
  assert.deepEqual(armed.state, { id: "a", armedAt: 1000 });

  // The old binding is just "any other key" now: it cancels, it never confirms.
  assert.equal(isDeleteKey(cmdBackspace), false);
  assert.equal(reduceHistoryDelete(armed.state, { type: "cancel" }).state, null);

  // The new one confirms inside the window…
  assert.equal(isDeleteKey(ctrlBackspace), true);
  const confirmed = reduceHistoryDelete(armed.state, { type: "press", id: "a", now: 2000 });
  assert.equal(confirmed.confirm, "a");
  assert.equal(confirmed.state, null);

  // …and a stale arm still expires, so a held ⌃⌫ cannot delete twice.
  const stale = reduceHistoryDelete(armed.state, {
    type: "expire",
    now: 1000 + HISTORY_DELETE_CONFIRM_MS + 1,
  });
  assert.equal(stale.state, null);
  assert.equal(stale.confirm, null);
});

// ── 4 · the wiring: one key, one source for the name ─────────────────────

test("R53 · the handler claims the key on both history modes, two-step", async () => {
  const actions = stripJsComments(await read("src/hooks/useLauncherActions.ts"));

  // The armed branch: only this key keeps an arm alive; anything else disarms.
  assert.match(
    actions,
    /const deleteKey\s*=\s*\(clipboardScope \|\| calculatorScope\) && matchesShortcut\(event, HISTORY_DELETE_SHORTCUT\)/,
  );
  // The claim: gated on the two history modes, resolved through the constant,
  // and only on a runnable row.
  assert.match(
    actions,
    /if \(clipboardScope \|\| calculatorScope\) \{\s*if \(matchesShortcut\(event, HISTORY_DELETE_SHORTCUT\)\)/,
  );
  assert.match(actions, /outcome\.confirm/);
  assert.match(actions, /deleteClipboardEntry\(entry\.id\)/);
  assert.match(actions, /deleteCalculatorEntry\(entry\.id\)/);

  // No stale spelling survives anywhere in the handler.
  assert.equal(/CmdOrCtrl\+Backspace/.test(actions), false);
});

test("R53 · the hint names the key through formatShortcut, not a literal", async () => {
  const app = stripJsComments(await read("src/App.tsx"));
  assert.match(app, /delete: formatShortcut\(HISTORY_DELETE_SHORTCUT\)/);
  // The displayed name is derived, so a hard-coded ⌘⌫ cannot reappear in JSX.
  assert.equal(/⌘⌫/.test(app), false);
});
