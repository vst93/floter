// R50 · the two-step inline delete, shared by the calculator and clipboard
// history lists.
//
// The user added one gesture to both plugins' launcher lists:
//
//   「剪切板跟计算器插件的列表上也需要增加删除单条的快捷键」
//
// and set the hand-feel in the same breath: no confirmation *overlay* (the
// project's standing red line against floating chrome inside the launcher), so
// the confirmation is **inline and two-step**, exactly like R38's "clear
// history" button:
//
//   1. the first press of the delete key arms the selected row — the row (and
//      the feedback line) switches to a muted "press again to delete";
//   2. a second press of the same key while still armed confirms;
//   3. Esc, any other key, a focus change or the timeout disarms it.
//
// This module is that state machine, pure and free of React/Tauri/DOM, so the
// node suite can drive every edge (arming a *different* row replaces the arm
// rather than confirming it; the timeout window; the cancel paths). The App
// owns one `useState` and only executes what this returns.
//
// ## The key
//
// `CmdOrCtrl+Backspace` (⌘⌫ on macOS, Ctrl+⌫ elsewhere), declared beside the
// other mode-local shortcuts in `launcher.ts` (`HISTORY_DELETE_SHORTCUT`). A bare
// ⌫/Delete cannot be the list's gesture: the field is a live text input, and
// plain Backspace must keep editing it. The modifier makes the intent
// deliberate, and the key is only *claimed* while the field is empty and the
// selection sits on a history row — elsewhere the field's own handling runs
// untouched.
//
// ## The ⌘⌫ round trip (R50 → R53 → R55)
//
// R50 shipped this as `CmdOrCtrl+Backspace`, i.e. ⌘⌫ on macOS, which is macOS's
// own "delete to the beginning of the line" in a text field. The user's report
// was exact — 「还有 cmd+空格删除这种逻辑不合适，换个快捷键」 — because a hand that
// already has that gesture pressed ⌘⌫ to trim what it *typed* and instead armed a
// destructive row delete. R53 moved the key to a literal `Ctrl+Backspace` (⌃⌫ on
// macOS, unbound there; word-delete is ⌥⌫, the emacs edits are ⌃H/⌃D/⌃K).
//
// R55 reverses that on the user's explicit request — 「之前改的删除快捷键改成
// cmd+删除」 — and mitigates the collision **structurally** instead of avoiding
// the key:
//
//   · the handler (`useLauncherActions`) only claims ⌘⌫ when `query` is empty —
//     it holds the mode's needle, not the trigger word — so a field with any
//     text at all keeps ⌘⌫ as "delete to line start". A row can only be armed
//     while the list is being *browsed*, never while something is being typed;
//   · the claim is further gated on a runnable history row (a status line or a
//     disabled row is not armed);
//   · the two-step arm/confirm and Esc/timeout cancellation are unchanged, so
//     even a mis-arm costs one Escape, not a deletion.
//
// The residual cost is named and accepted: on macOS, with an *empty* field and a
// history row selected, ⌘⌫ no longer deletes text (there is none) — the field is
// empty by construction of the guard. On Windows/Linux ⌘⌫ resolves to the same
// Ctrl+⌫ the list always used; the field's Ctrl+⌫ delete-word is claimed only in
// the empty-field + selected-row state, and Esc clears the field when that is
// what the user wants.

/** How long an armed row waits for its confirming press. Matches the "a few
 *  seconds" the user asked for and the overlay's own confirm dwell. */
export const HISTORY_DELETE_CONFIRM_MS = 5000;

/**
 * Whether the history-delete key may be *claimed* right now.
 *
 * R55 · the structural mitigation for putting the key back on the app
 * modifier (⌘⌫ on macOS): the key only belongs to the list while one of the
 * history modes owns the field **and the field is empty**. `needle` is the
 * mode's own text (App keeps the trigger word out of `query`), so any typed
 * character stands for "the user is editing", and the field's native editing
 * gesture — macOS's ⌘⌫ delete-to-line-start — must win. With an empty field
 * there is no text to delete, so the row arm cannot swallow a real edit.
 */
export const historyDeleteCanClaim = (historyScope: boolean, needle: string): boolean =>
  historyScope && needle.trim() === "";

/** The armed row: its id plus when it was armed (for the timeout). */
export type ArmedHistoryDelete = { readonly id: string; readonly armedAt: number } | null;

export type HistoryDeleteEvent =
  | { readonly type: "press"; readonly id: string; readonly now: number }
  | { readonly type: "cancel" }
  | { readonly type: "expire"; readonly now: number };

export type HistoryDeleteOutcome = {
  /** The arm state after the event. */
  readonly state: ArmedHistoryDelete;
  /** The id whose delete this event confirms, or `null` for an arm/cancel. */
  readonly confirm: string | null;
};

/**
 * Fold one event into the arm state.
 *
 * Pressing the *same* row again inside the window confirms; pressing a
 * different row re-arms on the new row (the confirmation never transfers);
 * `cancel` always disarms; `expire` disarms only once the window has passed, so
 * a stray timer event cannot cancel a still-valid arm.
 */
export const reduceHistoryDelete = (
  state: ArmedHistoryDelete,
  event: HistoryDeleteEvent,
): HistoryDeleteOutcome => {
  switch (event.type) {
    case "cancel":
      return { state: null, confirm: null };
    case "expire":
      if (state && event.now - state.armedAt > HISTORY_DELETE_CONFIRM_MS) {
        return { state: null, confirm: null };
      }
      return { state, confirm: null };
    case "press": {
      const same = state !== null && state.id === event.id;
      if (same && event.now - state.armedAt <= HISTORY_DELETE_CONFIRM_MS) {
        return { state: null, confirm: event.id };
      }
      return { state: { id: event.id, armedAt: event.now }, confirm: null };
    }
  }
};

/** Whether the row with `id` is the armed one. */
export const historyDeleteArmed = (state: ArmedHistoryDelete, id: string): boolean =>
  state !== null && state.id === id;

/**
 * Where the selection lands after the row at `index` is removed from a list
 * that had `length` rows: the neighbour takes the slot, and removing the last
 * row hands the selection to the new last row. The list never jumps to the top
 * (the user's 「邻居接管选中，不跳顶」).
 */
export const selectionAfterRemoval = (index: number, length: number): number => {
  if (length <= 1) return 0;
  return Math.max(0, Math.min(index, length - 2));
};
