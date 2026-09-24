// R55 · the history delete returns to ⌘⌫.
//
// The user's report (R53), verbatim: 「还有 cmd+空格删除这种逻辑不合适，换个快捷键」.
// R50 shipped `CmdOrCtrl+Backspace`; R53 moved it to a literal `Ctrl+Backspace`
// (⌃⌫ on macOS, Ctrl+⌫ elsewhere) to dodge macOS's ⌘⌫ "delete to line start".
//
// R55 reverses the key on the user's explicit request — 「之前改的删除快捷键改成
// cmd+删除」 — and mitigates the collision **structurally** instead of dodging it:
// the handler only claims the key while the field is empty (`historyDeleteCanClaim`),
// so a hand trimming typed text keeps the native editing gesture unchanged.
//
// Mutations that must turn this file red:
//   * leaving the R53 `Ctrl+Backspace` in place (the constant assertion);
//   * dropping the empty-field gate (the `historyDeleteCanClaim` unit block);
//   * a key handler that stops routing through `HISTORY_DELETE_SHORTCUT` or the
//     claim helper (the wiring assertions);
//   * the clipboard page keeping its own ⌘⌫ spelling instead of the shared
//     `isHistoryDeleteKey` predicate (the one-source assertion).

import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

import {
  CALCULATOR_FAVORITE_SHORTCUT,
  CLIPBOARD_FAVORITE_SHORTCUT,
  HISTORY_DELETE_SHORTCUT,
  isHistoryDeleteKey,
} from "../src/launcher.ts";
import {
  HISTORY_DELETE_CONFIRM_MS,
  historyDeleteCanClaim,
  reduceHistoryDelete,
} from "../src/plugins/history-actions.ts";
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

/** The resolved ⌫: `metaKey` on macOS (⌘), `ctrlKey` elsewhere (Ctrl). */
const appModifierBackspace = keyEvent({
  key: "Backspace",
  code: "Backspace",
  metaKey: IS_MAC,
  ctrlKey: !IS_MAC,
});
/** The R53 key, now the "other" modifier — it must no longer be the delete key. */
const ctrlBackspace = keyEvent({ key: "Backspace", ctrlKey: true, code: "Backspace" });

// ── 1 · the constant ─────────────────────────────────────────────────────

test("R55 · the delete key is CmdOrCtrl+⌫, declared once", async () => {
  assert.equal(HISTORY_DELETE_SHORTCUT, "CmdOrCtrl+Backspace");

  const launcher = await read("src/launcher.ts");
  assert.equal(launcher.match(/export const HISTORY_DELETE_SHORTCUT/g)?.length, 1);
  // No stray copy of the key outside the one declaration.
  const sources = await Promise.all([
    read("src/hooks/useLauncherActions.ts"),
    read("src/App.tsx"),
    read("src/plugins/history-actions.ts"),
    read("src/clipboard-list.ts"),
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

test("R55 · the app-modifier ⌫ matches; the R53 Control form no longer does", () => {
  assert.ok(matchesShortcut(appModifierBackspace, HISTORY_DELETE_SHORTCUT));
  // The negative control for the reverse: on macOS ⌃⌫ is now "some other key",
  // on Windows/Linux `ctrlKey` *is* the app modifier, so the same event still
  // matches — the platform split is exactly what `CmdOrCtrl` buys.
  assert.equal(
    matchesShortcut(ctrlBackspace, HISTORY_DELETE_SHORTCUT),
    IS_MAC ? false : true,
    "only macOS moved",
  );

  // The neighbours the key must not swallow (⌘/Ctrl is resolved above).
  const rejects: Partial<KeyboardEvent>[] = [
    { altKey: true },
    { shiftKey: true }, // ⇧⌫ selects backwards
    {}, // plain ⌫ edits the field
  ];
  for (const init of rejects) {
    assert.equal(
      matchesShortcut(
        keyEvent({ key: "Backspace", code: "Backspace", ...init }),
        HISTORY_DELETE_SHORTCUT,
      ),
      false,
      `Backspace with ${JSON.stringify(init)} is not the delete key`,
    );
  }
});

test("R55 · the key collides with no other binding the launcher owns", () => {
  const others = [
    CLIPBOARD_FAVORITE_SHORTCUT,
    CALCULATOR_FAVORITE_SHORTCUT,
    `${IS_MAC ? "Cmd" : "Ctrl"}+Shift+Backspace`,
    `${IS_MAC ? "Cmd" : "Ctrl"}+Enter`,
    ...Object.values(DEFAULT_SHORTCUTS).filter((value) => value !== ""),
  ];
  for (const binding of others) {
    assert.equal(
      matchesShortcut(appModifierBackspace, binding),
      false,
      `the delete key must not also be ${binding}`,
    );
  }
  assert.ok(matchesShortcut(appModifierBackspace, HISTORY_DELETE_SHORTCUT));
});

test("R55 · the key is drawn as the delete key it is", () => {
  assert.equal(formatShortcut(HISTORY_DELETE_SHORTCUT), IS_MAC ? "⌘⌫" : "Ctrl+⌫");
});

// ── 3 · the structural collision mitigation ──────────────────────────────

test("R55 · the field with text never arms: ⌘⌫ stays a text edit", () => {
  // The guard the key handler is built on, unit-tested on its own: only an
  // empty field (the mode's needle, trigger word excluded) can claim the key.
  assert.equal(historyDeleteCanClaim(true, ""), true);
  assert.equal(historyDeleteCanClaim(true, "   "), true, "whitespace is still empty");
  assert.equal(historyDeleteCanClaim(true, "docker"), false, "typed text keeps the edit");
  assert.equal(historyDeleteCanClaim(false, ""), false, "outside a history mode: never");
});

test("R55 · a second app-modifier ⌫ confirms, the R53 key disarms instead", () => {
  const isDeleteKey = (event: KeyboardEvent) => matchesShortcut(event, HISTORY_DELETE_SHORTCUT);

  const armed = reduceHistoryDelete(null, { type: "press", id: "a", now: 1000 });
  assert.deepEqual(armed.state, { id: "a", armedAt: 1000 });

  // On macOS the R53 key is now just "any other key": it cancels, never confirms.
  if (IS_MAC) {
    assert.equal(isDeleteKey(keyEvent({ key: "Backspace", ctrlKey: true, code: "Backspace" })), false);
  }
  assert.equal(reduceHistoryDelete(armed.state, { type: "cancel" }).state, null);

  assert.equal(isDeleteKey(appModifierBackspace), true);
  const confirmed = reduceHistoryDelete(armed.state, { type: "press", id: "a", now: 2000 });
  assert.equal(confirmed.confirm, "a");
  assert.equal(confirmed.state, null);

  const stale = reduceHistoryDelete(armed.state, {
    type: "expire",
    now: 1000 + HISTORY_DELETE_CONFIRM_MS + 1,
  });
  assert.equal(stale.state, null);
  assert.equal(stale.confirm, null);
});

// ── 4 · the wiring: one key, one source for the name ─────────────────────

test("R55 · the handler claims the key on both history modes, two-step, empty field", async () => {
  const actions = stripJsComments(await read("src/hooks/useLauncherActions.ts"));

  // The claim is gated on a history mode, the empty-field helper, and the key.
  assert.match(
    actions,
    /if \(clipboardScope \|\| calculatorScope\) \{\s*if \(historyDeleteCanClaim\(true, query\) && matchesShortcut\(event, HISTORY_DELETE_SHORTCUT\)\)/,
  );
  assert.match(actions, /outcome\.confirm/);
  assert.match(actions, /deleteClipboardEntry\(entry\.id\)/);
  assert.match(actions, /deleteCalculatorEntry\(entry\.id\)/);
});

test("R55 · the clipboard page recognises the same key through the shared predicate", async () => {
  const list = stripJsComments(await read("src/clipboard-list.ts"));
  assert.match(list, /isHistoryDeleteKey\(/);
  // No independent `key === "Backspace"` + modifier spelling survives.
  assert.equal(
    /modifier && key === "Backspace"/.test(list),
    false,
    "the clipboard page must not keep its own ⌘⌫",
  );
  // The predicate itself is the one in launcher.ts.
  assert.equal(isHistoryDeleteKey({ key: "Backspace", code: "Backspace", ctrlKey: !IS_MAC, metaKey: IS_MAC, altKey: false, shiftKey: false }), true);
});

test("R55 · the hint names the key through formatShortcut, not a literal", async () => {
  const app = stripJsComments(await read("src/App.tsx"));
  assert.match(app, /delete: formatShortcut\(HISTORY_DELETE_SHORTCUT\)/);
  assert.equal(/⌘⌫/.test(app), false);
  assert.equal(/⌃⌫/.test(app), false);
});
