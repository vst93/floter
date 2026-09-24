// R64 · the ⌘-held bare-terminal row: one placement, at the list's end.
//
// The user's message, verbatim:
//   「刚做的输入框为空时 按住 cmd 选中终端打开这个逻辑和一般搜索情况一样出现在
//    末尾啊。同时前面加的按住在列表额外增加终端选项的逻辑要去掉」
//
// Two corrections in one breath, and this round is exactly them:
//
//   1. R60's input-state append retires: a typed query draws *no* ⌘ row at all.
//   2. R61's featured lift retires: on the *empty* field the held row sits at the
//      list's **last** line (below the recents, the 「最近启动」 block included),
//      as an ordinary system row, and it keeps R61's preselection on the
//      modifier's press edge. Its ⌘ slot is the position's own — the number after
//      every runnable recent, never a reserved one.
//
// This file pins the things a later round could quietly move: the placement rule's
// one answer, the row's ordinary shape, the App's append/preselect wiring, the
// position's slot, the geometry, and the *absence* of the R61 featured treatment.
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";
import {
  BARE_TERMINAL_ROW_ID,
  BARE_TERMINAL_SUBTITLE_KEY,
  BARE_TERMINAL_TITLE_KEY,
  bareTerminalPlacement,
  bareTerminalRow,
  bareTerminalRowVisible,
} from "../src/launcher/terminal-row.ts";
import {
  ROW_HEIGHT_TWO_LINE,
  launcherContentHeight,
  launcherListUnits,
  resolveLauncherUnits,
  resultIndexForSlot,
  resultShortcutSlots,
} from "../src/launcher/result-budget.ts";
import { createTranslator } from "../src/i18n.ts";

const root = new URL("../", import.meta.url);
const read = (path: string) => readFile(new URL(path, root), "utf8");
const stripJsComments = (src: string) =>
  src.replace(/\/\*[\s\S]*?\*\//g, "").replace(/\/\/[^\n]*/g, "");

const en = createTranslator("en");
const zh = createTranslator("zh");

// ── 1 · one predicate, one answer ─────────────────────────────────────────

test("the placement is the one predicate, and it has exactly one answer", () => {
  // The empty field under the held modifier: the row, appended.
  assert.equal(bareTerminalPlacement("", true, null), "appended");
  assert.equal(bareTerminalPlacement("   ", true, null), "appended", "whitespace is still empty");
  // R60's input-state append is retired (correction 1): a typed query draws no
  // row, whatever the modifier does.
  assert.equal(bareTerminalPlacement("git", true, null), null, "a typed query draws no row");
  assert.equal(bareTerminalPlacement("term", true, null), null);
  // The two exclusions, shared with R60.
  assert.equal(bareTerminalPlacement("", false, null), null, "released, no row at all");
  assert.equal(bareTerminalPlacement("git", false, null), null);
  for (const scope of ["browser", "clipboard", "calculator", "external"]) {
    assert.equal(bareTerminalPlacement("", true, scope), null, `${scope} owns the list`);
    assert.equal(bareTerminalPlacement("git", true, scope), null, `${scope} owns the list`);
  }
  // R60's boolean is the derived form of the same rule — one authority, and the
  // two can never disagree.
  const cases: Array<[string, boolean, string | null]> = [
    ["", true, null],
    ["   ", true, null],
    ["git", true, null],
    ["", false, null],
    ["git", true, "browser"],
  ];
  for (const [query, held, scope] of cases) {
    assert.equal(
      bareTerminalRowVisible(query, held, scope),
      bareTerminalPlacement(query, held, scope) !== null,
      `bareTerminalRowVisible must be bareTerminalPlacement !== null for (${query}, ${held}, ${scope})`,
    );
  }
});

// ── 2 · the row is an ordinary system row again ───────────────────────────

test("the row carries no featured marker — R61's treatment retired with its lift", () => {
  const row = bareTerminalRow(en);
  assert.equal(row.type, "system");
  assert.equal(row.id, BARE_TERMINAL_ROW_ID);
  assert.equal(row.action, "terminal");
  assert.equal(row.title, en(BARE_TERMINAL_TITLE_KEY));
  assert.equal(row.subtitle, en(BARE_TERMINAL_SUBTITLE_KEY));
  assert.equal(bareTerminalRow(zh).title, zh(BARE_TERMINAL_TITLE_KEY), "bilingual");
  // No marker key at all — a `featured: false` would still be a shape the R61
  // renderer could branch on, so the key must be gone, not merely false.
  assert.ok(!("featured" in row), "the row carries no featured marker");
  // The row's own shape is unchanged from R60: a `system` row with a subtitle.
  assert.equal(row.type, "system");
});

// ── 3 · the App appends it at the end and preselects the last index ───────

test("the App appends the row through the one predicate and bills it as a row", async () => {
  const app = stripJsComments(await read("src/App.tsx"));
  assert.match(
    app,
    /const bareTerminalMode = bareTerminalPlacement\(query, appModifierDown, launcherScope\);/,
    "one predicate, read from the field, the modifier and the scope",
  );
  assert.match(
    app,
    /const showBareTerminalRow = bareTerminalMode !== null;/,
    "the drawn form is derived, never a second predicate",
  );
  // R64's one position: after the base list — the recents (or a drop group) and
  // the query's own rows — so the row is the list's last line.
  assert.match(
    app,
    /return showBareTerminalRow \? \[\.\.\.base, bareTerminalRow\(t\)\] : base;/,
    "the row is appended at the list's end",
  );
  // R61's prepend branch is gone, and no `featured` marker survives anywhere.
  assert.ok(!/featured/.test(app), "no featured branch survives in the App");
  assert.equal(
    app.split(/return showBareTerminalRow/).length - 1,
    1,
    "exactly one append path in the composed list",
  );
  // Billing: still the one `displayedResults` list, so no new height term.
  assert.match(app, /launcherListUnits\(displayedResults\.map\(launcherRowHeightUnits\)\)/);
  assert.ok(
    !/bareTerminal/.test(app.split("const launcherListUnitsRaw")[1] ?? ""),
    "no placement-specific term inside the height formula",
  );
});

test("the row is preselected on the press edge at the last index, released on the release edge", async () => {
  const app = stripJsComments(await read("src/App.tsx"));
  // The press edge: the held row is the list's *last* entry, so the selection
  // goes to the last index — the position the predicate just appended to. The
  // action bar is explicitly left.
  assert.match(
    app,
    /const heldBareTerminal = bareTerminalMode !== null;/,
    "the held form is read from the placement",
  );
  assert.match(
    app,
    /if \(heldBareTerminal\) \{\s*setSelectedActionBar\(false\);\s*setSelectedResultIndex\(displayedResultsLength - 1\);/,
    "the press edge preselects the last row",
  );
  assert.match(app, /const displayedResultsLength = displayedResults\.length;/);
  // The release edge: the ordinary empty page's default selection comes back.
  assert.match(
    app,
    /setSelectedResultIndex\(firstRunnableResultIndex < 0 \? 0 : dropped \+ firstRunnableResultIndex\);\s*setSelectedActionBar\(defaultsToActionBar\);/,
    "the release edge restores the default selection",
  );
  // The edges are edges: the effect bails when the placement has not changed,
  // so arrow steps and re-renders never re-seat the selection.
  assert.match(app, /if \(heldBareTerminal === wasShown\) return;/);
  // The last index is read from this commit's length, so a genuine length change
  // while held re-runs (and the equality guard absorbs it) rather than leaving a
  // stale index.
  assert.match(app, /\[heldBareTerminal, displayedResultsLength, defaultsToActionBar/);
  // Enter is still the row's own `runLauncherItem` path — no new key path.
  const actions = stripJsComments(await read("src/hooks/useLauncherActions.ts"));
  assert.match(actions, /if \(item\.action === "terminal"\) \{\s*void openTerminalSession\(\);\s*return;/);
});

// ── 4 · the position's ⌘ number, and the geometry ─────────────────────────

test("the appended row takes the position's last ⌘ number, never a reserved one", () => {
  // R37/R64 · numbering is purely in order over the composed list: three
  // runnable recents then the held row gives `1, 2, 3, 4`. The row's key is the
  // position's own, exactly like an ordinary search row at the list's end.
  const rows = [ROW_HEIGHT_TWO_LINE, ROW_HEIGHT_TWO_LINE, ROW_HEIGHT_TWO_LINE, ROW_HEIGHT_TWO_LINE];
  const flags = [true, true, true, true];
  const slots = resultShortcutSlots(rows, flags);
  assert.deepEqual(slots, [1, 2, 3, 4], "the last row takes the next number in sequence");
  assert.equal(resultIndexForSlot(slots, 4), 3, "⌘4 reaches the appended row");
  // Past the ten-row viewport the badge simply stops: the row at index 10+ is
  // outside the numbering, exactly as any other row there would be.
  const wide = Array.from({ length: 12 }, () => ROW_HEIGHT_TWO_LINE);
  const wideSlots = resultShortcutSlots(wide, wide.map(() => true));
  assert.equal(wideSlots[0], 1);
  assert.equal(wideSlots[9], 0, "the tenth visible runnable row takes `0`");
  assert.equal(wideSlots[10], null, "no eleventh digit to hand out");
});

test("the appended row costs exactly one row plus the 1px grid gap", () => {
  // Three recents, then the same page with the held row at the end. The section
  // title is present in both states (the empty page always draws it), so the
  // delta is the row's 42u plus the one extra 1px grid gap.
  const rows = [ROW_HEIGHT_TWO_LINE, ROW_HEIGHT_TWO_LINE, ROW_HEIGHT_TWO_LINE];
  const held = [...rows, ROW_HEIGHT_TWO_LINE];
  const height = (units: number[], count: number) =>
    launcherContentHeight(launcherListUnits(units), count, 1, Number.POSITIVE_INFINITY, true, true, false);
  assert.equal(height(held, held.length) - height(rows, rows.length), ROW_HEIGHT_TWO_LINE + 1);
  // The release is a one-row shrink: R43's hysteresis absorbs it, so hold/release
  // cannot flap — position on screen does not change the billing.
  const heldUnits = launcherListUnits(held);
  assert.equal(
    resolveLauncherUnits(heldUnits, launcherListUnits(rows)),
    heldUnits,
    "one row back is absorbed",
  );
  assert.ok(
    resolveLauncherUnits(heldUnits, launcherListUnits(rows) - ROW_HEIGHT_TWO_LINE - 1) < heldUnits,
    "and a real collapse still takes the window down",
  );
});

// ── 5 · the featured treatment is gone ────────────────────────────────────

test("the renderer keeps the recents heading at the top and draws no featured class", async () => {
  const results = stripJsComments(await read("src/launcher/LauncherResults.tsx"));
  // The 「最近启动」 heading always sits at the top of the list, as before R61:
  // the held row is appended below it, so there is no lead to move it under.
  assert.match(results, /showRecentTitle && \(/, "the recents heading leads the list");
  assert.ok(!/featuredLead/.test(results), "the R61 lead detection is gone");
  assert.ok(
    !/launcher-result--featured/.test(results),
    "no row earns the R61 featured class",
  );
  assert.ok(!/featured\?: boolean;/.test(results), "the item no longer carries the marker");
});

test("the featured CSS is deleted, so the row is a plain system row", async () => {
  const css = await read("src/styles/launcher.css");
  assert.equal(
    css.indexOf(".launcher-result--featured"),
    -1,
    "R61's featured rules are gone (edge, selected composition and accent glyph)",
  );
  // The row's own surface is the ordinary result surface — no accent fill, no
  // accent edge bought for it.
  assert.equal(css.indexOf("launcher-result--featured"), -1);
});

test("no accent was bought for the row: the ordinary result surface is untouched", async () => {
  const css = await read("src/styles/launcher.css");
  // The selected rule is still the plain tint + keyline, with no featured
  // composition left dangling after the deletion.
  assert.match(
    css.slice(css.indexOf(".launcher-result--selected {")),
    /background: var\(--glass-raised\);\s*box-shadow: inset 0 0 0 1px var\(--accent-edge\);/,
    "the selected row keeps the ordinary R15 treatment",
  );
});
