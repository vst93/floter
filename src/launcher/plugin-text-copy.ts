// R63 · the 直出 (direct output) text surface's selection rule.
//
// The user asked for the gesture the terminal has had since R44 — 「选中即复制」
// — on the launcher's own text surface: the block `PluginTextView.tsx` prints
// under the field when a plugin (or an external command summoned in the search
// box) answers with text rather than a list. There the selection lives in the
// DOM's own `Selection`, not in a renderer's cell model, but the *rule* is the
// same shape: the gesture must have produced a live, non-empty selection that
// belongs to the output block, and nothing else may be copied.
//
// This is the pure half — no React, no Tauri, no DOM globals — so the question
// "is this selection ours?" is a node test's subject rather than a claim in a
// comment. The caller owns the write and the notice; see
// `hooks/useLauncherTextCopy.ts` for the one chokepoint they go through.

/** The slice of `Element` the rule reads. Structural so the node suite can
 *  hand it a plain object (and so this module needs no DOM lib). */
export type SelectionContainer = { contains(node: Node | null): boolean };

/** The slice of `Selection` the rule reads. */
export type TextSelection = {
  isCollapsed: boolean;
  anchorNode: Node | null;
  focusNode: Node | null;
  toString(): string;
};

/**
 * The text a selection gesture should copy, or `null` when the gesture did not
 * produce one this surface owns.
 *
 * Three refusals, in order:
 *
 *   1. no selection, or a collapsed one (a plain click) — nothing was selected;
 *   2. either end outside the output block — a drag that began in the field and
 *      ended in the output (or the reverse) is not this surface's selection, and
 *      copying it would silently capture the query too;
 *   3. an empty/blank string — a drag across blank space makes a live selection
 *      with nothing in it, which must not report a copy that never happened.
 */
export const selectionTextIn = (
  container: SelectionContainer | null,
  selection: TextSelection | null,
): string | null => {
  if (!container || !selection || selection.isCollapsed) return null;
  if (
    !container.contains(selection.anchorNode) ||
    !container.contains(selection.focusNode)
  ) {
    return null;
  }
  const text = selection.toString();
  return text.trim().length > 0 ? text : null;
};
