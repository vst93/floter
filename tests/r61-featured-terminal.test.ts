// R61 · the featured bare-terminal row on the empty page.
//
// The user's two messages, the second overriding the first's word:
//   「⌘ 按住浮现行 这个之前就有了，现在要加的是输入框为空时也要追加」
//   「不是追加，当有输入内容时，当前是末尾本身就有 cmd 会自动选中，现在是要在
//    没有任何输入时，按住 cmd 要特殊显示这项并选中」
//
// So the round is one gesture with two placements and no second row:
//
//   1. a typed query keeps R60's form untouched — the row is appended below the
//      results and it is an ordinary `system` row;
//   2. the empty page *features* it — the same row, lifted above the recents
//      (heading included), drawn with an accent edge and an accent glyph, and
//      preselected on the modifier's press edge;
//   3. both forms share one predicate (`bareTerminalPlacement`) and one billing
//      source (`displayedResults`), so the appearance is one row plus one grid
//      gap and the release cannot flap the window.
//
// This file pins the things a later round could quietly move: the placement
// rule's two answers, the appended form's byte-for-byte shape, the App's
// prepend/preselect wiring, the geometry, and the "mark, never a fill" visual.
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
import { ROW_HEIGHT_TWO_LINE, launcherContentHeight, launcherListUnits, resolveLauncherUnits } from "../src/launcher/result-budget.ts";
import { createTranslator } from "../src/i18n.ts";

const root = new URL("../", import.meta.url);
const read = (path: string) => readFile(new URL(path, root), "utf8");
const stripJsComments = (src: string) =>
  src.replace(/\/\*[\s\S]*?\*\//g, "").replace(/\/\/[^\n]*/g, "");

const en = createTranslator("en");
const zh = createTranslator("zh");

// ── 1 · one predicate, two answers ────────────────────────────────────────

test("the placement is the one predicate, and it names both forms", () => {
  // A typed query is R60's append; the empty page is R61's featured lead.
  assert.equal(bareTerminalPlacement("git", true, null), "appended");
  assert.equal(bareTerminalPlacement("term", true, null), "appended");
  assert.equal(bareTerminalPlacement("", true, null), "featured");
  assert.equal(bareTerminalPlacement("   ", true, null), "featured", "whitespace is still empty");
  // The two exclusions, shared with R60.
  assert.equal(bareTerminalPlacement("git", false, null), null, "released, no row at all");
  assert.equal(bareTerminalPlacement("", false, null), null);
  for (const scope of ["browser", "clipboard", "calculator", "external"]) {
    assert.equal(bareTerminalPlacement("git", true, scope), null, `${scope} owns the list`);
    assert.equal(bareTerminalPlacement("", true, scope), null, `${scope} owns the list`);
  }
  // R60's boolean is the derived form of the same rule — one authority, and the
  // two can never disagree.
  const cases: Array<[string, boolean, string | null]> = [
    ["git", true, null],
    ["", true, null],
    ["   ", true, null],
    ["git", false, null],
    ["", true, "browser"],
  ];
  for (const [query, held, scope] of cases) {
    assert.equal(
      bareTerminalRowVisible(query, held, scope),
      bareTerminalPlacement(query, held, scope) !== null,
      `bareTerminalRowVisible must be bareTerminalPlacement !== null for (${query}, ${held}, ${scope})`,
    );
  }
});

// ── 2 · the two row shapes ────────────────────────────────────────────────

test("the featured row is the same row plus the featured marker", () => {
  const appended = bareTerminalRow(en);
  const featured = bareTerminalRow(en, true);
  // Identical in every field the row machinery reads: one id, one action, one
  // runner. The placement changes where it sits, never what it is.
  assert.equal(featured.type, "system");
  assert.equal(featured.id, BARE_TERMINAL_ROW_ID);
  assert.equal(featured.action, "terminal");
  assert.equal(featured.title, en(BARE_TERMINAL_TITLE_KEY));
  assert.equal(featured.subtitle, en(BARE_TERMINAL_SUBTITLE_KEY));
  assert.equal(featured.featured, true);
  assert.equal(bareTerminalRow(zh, true).featured, true, "bilingual");
  // The append path is byte-for-byte R60: no `featured` key at all (a `false`
  // key would still be a shape change for every equality-sensitive reader).
  assert.ok(!("featured" in appended), "the appended row carries no featured marker");
  assert.equal(appended.featured, undefined);
});

// ── 3 · the App wires both forms through the one predicate ────────────────

test("the App prepends for the empty page and appends for a query", async () => {
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
  // R61's featured form: above everything, the recents heading included.
  assert.match(
    app,
    /if \(bareTerminalMode === "featured"\) return \[bareTerminalRow\(t, true\), \.\.\.base\];/,
    "the featured row is prepended",
  );
  // R60's append form, verbatim.
  assert.match(
    app,
    /return showBareTerminalRow \? \[\.\.\.base, bareTerminalRow\(t\)\] : base;/,
    "the appended row is unchanged",
  );
  // Billing: still the one `displayedResults` list, so no new height term.
  assert.match(app, /launcherListUnits\(displayedResults\.map\(launcherRowHeightUnits\)\)/);
  assert.ok(
    !/featured|bareTerminal/.test(app.split("const launcherListUnitsRaw")[1] ?? ""),
    "no placement-specific term inside the height formula",
  );
});

test("the featured row is preselected on the press edge and released on the release edge", async () => {
  const app = stripJsComments(await read("src/App.tsx"));
  // The press edge: the featured row is the list's first entry, so index 0 is
  // the selection; the action bar is explicitly left.
  assert.match(
    app,
    /const featuredBareTerminal = bareTerminalMode === "featured";/,
    "the empty-page form is read from the placement",
  );
  assert.match(
    app,
    /if \(featuredBareTerminal\) \{\s*setSelectedActionBar\(false\);\s*setSelectedResultIndex\(0\);/,
    "the press edge preselects the featured row",
  );
  // The release edge: the ordinary empty page's default selection comes back.
  assert.match(
    app,
    /setSelectedResultIndex\(firstRunnableResultIndex < 0 \? 0 : dropped \+ firstRunnableResultIndex\);\s*setSelectedActionBar\(defaultsToActionBar\);/,
    "the release edge restores the default selection",
  );
  // The edges are edges: the effect bails when the placement has not changed,
  // so arrow steps and re-renders never re-seat the selection.
  assert.match(app, /if \(featuredBareTerminal === wasShown\) return;/);
  // Enter is still the row's own `runLauncherItem` path — no new key path.
  const actions = stripJsComments(await read("src/hooks/useLauncherActions.ts"));
  assert.match(actions, /if \(item\.action === "terminal"\) \{\s*void openTerminalSession\(\);\s*return;/);
});

// ── 4 · geometry: one row plus one grid gap ───────────────────────────────

test("the featured row costs exactly one row plus the 1px grid gap", () => {
  // Three recents, then the same page with the featured row on top. The section
  // title is charged in both states (the empty page draws it either way — the
  // renderer only moves it below the featured row), so the delta is the row's
  // 42u plus the one extra 1px grid gap.
  const rows = [ROW_HEIGHT_TWO_LINE, ROW_HEIGHT_TWO_LINE, ROW_HEIGHT_TWO_LINE];
  const featured = [ROW_HEIGHT_TWO_LINE, ...rows];
  const height = (units: number[], count: number) =>
    launcherContentHeight(launcherListUnits(units), count, 1, Number.POSITIVE_INFINITY, true, true, false);
  assert.equal(height(featured, featured.length) - height(rows, rows.length), ROW_HEIGHT_TWO_LINE + 1);
  // The release is a one-row shrink: R43's hysteresis absorbs it, exactly as it
  // absorbs any other row leaving the list, so hold/release cannot flap.
  const heldUnits = launcherListUnits(featured);
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

// ── 5 · the featured visual: a mark, never a fill ─────────────────────────

test("the renderer draws the marker and moves the recents heading under the row", async () => {
  const results = stripJsComments(await read("src/launcher/LauncherResults.tsx"));
  // The marker is read off the item, so only the system row can carry it.
  assert.match(
    results,
    /item\.type === "system" && item\.featured \? " launcher-result--featured" : ""/,
    "the row earns its class from the `featured` marker",
  );
  // The 「最近启动」 heading belongs to the recents block under the row.
  assert.match(
    results,
    /const featuredLead =\s*results\.length > 0 && results\[0\]\.type === "system" && results\[0\]\.featured === true;/,
    "the lead row is recognised",
  );
  assert.match(results, /showRecentTitle && !featuredLead && \(/, "no heading above the lead row");
  assert.match(
    results,
    /showRecentTitle && featuredLead && index === 1 && \(/,
    "the heading moves under the lead row, above the first recent",
  );
  // The `featured` marker is on the system variant and nowhere else.
  assert.match(results, /featured\?: boolean;/);
});

test("the featured treatment is an accent edge and an accent glyph, with no fill", async () => {
  const css = await read("src/styles/launcher.css");
  const at = css.indexOf(".launcher-result--featured {");
  assert.notEqual(at, -1, "the featured rule must exist");
  // All three featured rules and nothing else: the next rule is the compact
  // row (R12), preceded by its own comment.
  const region = css.slice(at, css.indexOf("\n.launcher-result--compact {", at));
  const block = css.slice(at, css.indexOf(".launcher-result--featured.launcher-result--selected {", at));
  // A 2u accent rule down the leading edge, in the shell's own unit.
  assert.match(block, /box-shadow: inset calc\(var\(--u\) \* 2\) 0 0 var\(--accent\);/);
  // And the selection's keyline composes with it rather than replacing it.
  assert.match(
    region,
    /\.launcher-result--featured\.launcher-result--selected \{[\s\S]*?inset calc\(var\(--u\) \* 2\) 0 0 var\(--accent\),[\s\S]*?inset 0 0 0 1px var\(--accent-edge\);/,
  );
  // The glyph takes the accent; the surface does not.
  assert.match(region, /\.launcher-result--featured \.launcher-result__icon \{\s*color: var\(--accent\);\s*\}/);
  // R46 · no new token: every custom property here exists as a name the shell
  // already declares, and the measure is the unit.
  for (const token of ["--accent", "--accent-edge", "--u"]) {
    assert.ok(region.includes(`var(${token})`), `${token} is the token the mark uses`);
  }
  // No fill: the featured region declares no background, so the accent budget's
  // counted faces (`.launcher-result--selected`'s `--glass-raised`, and the
  // rest of the census) are untouched. This is the R61 red line, asserted.
  assert.ok(!/background\s*:/.test(region), "never an accent fill");
  assert.ok(!/position:\s*absolute/.test(region), "the row is inline in the list, not a floating layer");
});
