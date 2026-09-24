// R60 · the bare terminal.
//
// The user's ask, verbatim: 「方案 A 增加一个浮动终端的选项，同时在按住 cmd 时也
// 自动增加底部的终端执行项，一样的再按回车进入，只是这是没有任何命令执行」,
// then, on the floating half: 「系统命令『浮动终端』这逻辑算了，不要」.
//
// So the round is three things and no window:
//
//   1. a `SYSTEM_COMMANDS` row — search 「终端 / term / terminal」 and Enter opens
//      a blank session (`ensureTerminalSession(null)`), nothing executed;
//   2. the ⌘-held row — with a query typed on the ordinary search page, holding
//      the app modifier appends one system row below everything else, and Enter
//      on it is the same door;
//   3. the terminal page's inline empty state — the page can be entered with no
//      session at all, and a blank canvas says nothing.
//
// The invariants this file pins are the ones a later round is most likely to
// break silently: that the row's appearance and the window's height stay one
// decision, that no new key path or shortcut was invented for it, and that the
// empty state cannot cover a live session.
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";
import { normalizeSearch, scoreApp, shouldDefaultToActionBar } from "../src/launcher.ts";
import {
  BARE_TERMINAL_ROW_ID,
  BARE_TERMINAL_SUBTITLE_KEY,
  BARE_TERMINAL_TITLE_KEY,
  appModifierHeld,
  bareTerminalRow,
  bareTerminalRowVisible,
  isBareTerminalRow,
} from "../src/launcher/terminal-row.ts";
import { TERMINAL_EMPTY_HINT_DELAY, terminalPageEmpty } from "../src/terminal/empty-state.ts";
import { resultRowContent } from "../src/launcher/row-content.ts";
import { ROW_HEIGHT_TWO_LINE, launcherContentHeight, launcherListUnits, resolveLauncherUnits } from "../src/launcher/result-budget.ts";
import { createTranslator } from "../src/i18n.ts";

const root = new URL("../", import.meta.url);
const read = (path: string) => readFile(new URL(path, root), "utf8");
const stripJsComments = (src: string) =>
  src.replace(/\/\*[\s\S]*?\*\//g, "").replace(/\/\/[^\n]*/g, "");

const en = createTranslator("en");
const zh = createTranslator("zh");

/** The catalog's `SYSTEM_COMMANDS` table, and the terminal entry inside it. */
const terminalEntry = async () => {
  const source = await read("src/hooks/useLauncherCatalog.ts");
  const table = /const SYSTEM_COMMANDS[\s\S]*?\n\];/.exec(source);
  assert.ok(table, "the system-command table must exist");
  const entry = /action: "terminal",[\s\S]*?\n  \},/.exec(table![0]);
  assert.ok(entry, "R60: the terminal must be one of the table's entries");
  return { table: table![0], entry: entry![0] };
};

const searchNamesOf = (entry: string): string[] => {
  const block = /searchNames: \[([\s\S]*?)\]\.map\(normalizeSearch\)/.exec(entry);
  assert.ok(block, "the entry must declare its search names through normalizeSearch");
  return [...block![1].matchAll(/"([^"]+)"/g)].map((match) => match[1]);
};

// ── 1 · the terminal system row ───────────────────────────────────────────

test("the catalog declares one terminal system row, in the R51 shape", async () => {
  const { entry } = await terminalEntry();
  assert.match(entry, /action: "terminal"/);
  assert.match(entry, /titleKey: "system\.terminal"/);
  assert.match(entry, /subtitleKey: "system\.terminalSubtitle"/);
  assert.match(entry, /initials: "[a-z]+"/, "the row carries a pinyin/English initials key");
  for (const name of ["terminal", "term", "终端"]) {
    assert.ok(searchNamesOf(entry).includes(name), `${name} must be a search name`);
  }
  // The row is a *match*, not an action-bar claim: `terminal` stays a shell word
  // on the action bar exactly as `calc` does (`SYSTEM_ACTION_QUERIES`).
  const launcher = stripJsComments(await read("src/launcher.ts"));
  const actions = /const SYSTEM_ACTION_QUERIES[\s\S]*?\n\};/.exec(launcher);
  assert.ok(actions, "the action-bar claim table must exist");
  assert.ok(
    !/terminal:/.test(actions![0]),
    "the terminal must not be claimed by the action bar — it is reached by a match",
  );
  // And it is *additive*: the existing shell fallback keeps its own row and its
  // own word (`launcher.runInShell`), which no round has removed.
  assert.equal(en("launcher.runInShell"), "Run in shell");
  assert.equal(zh("launcher.runInShell"), "在终端中运行");
});

test("every trigger word reaches the row through the catalog's own scorer", async () => {
  const { entry } = await terminalEntry();
  const literals = searchNamesOf(entry);
  const names = literals.map(normalizeSearch);
  const initials = /initials: "([a-z]+)"/.exec(entry)![1];
  // The exact call `useLauncherCatalog` makes for a `system` row: the localized
  // title, the names in every language, the initials key and no aliases.
  const reach = (needle: string) =>
    scoreApp(normalizeSearch(needle), [normalizeSearch(en("system.terminal")), ...names], initials, []);
  for (const word of literals) {
    assert.ok(reach(word) > 0, `"${word}" must reach the terminal row`);
  }
  assert.ok(reach("zd") > 0, "the pinyin key 终端 → zd must reach it");
  assert.ok(reach("tzd") > 0, "the English-initials prefix tzd must reach it");
  assert.ok(reach("termi") > 0, "a prefix of a search name still reaches the row");
  assert.ok(reach("termi") > 0 && reach("term") >= reach("termi"), "an exact name never scores below its prefix");
  // A word that is none of them does not: the row is not a catch-all.
  assert.equal(reach("zzzz"), 0);
});

test("the row's two strings exist in both dictionaries and say something", () => {
  assert.equal(en("system.terminal"), "Terminal");
  assert.equal(zh("system.terminal"), "终端");
  assert.notEqual(en("system.terminalSubtitle"), zh("system.terminalSubtitle"));
  assert.notEqual(en("system.terminal"), en("system.terminalSubtitle"));
  // The row collapses to one line only when the subtitle is the title written
  // again or the row's own type word — neither is true here, so the two-line
  // height is what the window charges (see the height test below).
  const row: Parameters<typeof resultRowContent>[0] = {
    type: "system",
    id: "system-terminal",
    title: en("system.terminal"),
    subtitle: en("system.terminalSubtitle"),
    action: "terminal",
  };
  const content = resultRowContent(row, en);
  assert.equal(content.source, null, "a system row prints no right-hand source word");
  assert.equal(content.subtitle, en("system.terminalSubtitle"), "the subtitle survives");
});

test("Enter on the row opens a blank session — no command, no plugin mode", async () => {
  const actions = stripJsComments(await read("src/hooks/useLauncherActions.ts"));
  // The door: one branch, taken before the power confirmation could arm.
  assert.match(
    actions,
    /if \(item\.action === "terminal"\) \{\s*void openTerminalSession\(\);\s*return;/,
    "the terminal row must run the bare-session path and return",
  );
  // The path itself: the same transition `runCommand` makes, with a null command.
  const at = actions.indexOf("const openTerminalSession");
  assert.notEqual(at, -1, "the bare-session opener must exist");
  const body = actions.slice(at, actions.indexOf("const resumeTerminalSession", at));
  assert.match(body, /ensureTerminalSession\(null\)/, "no command is handed to the PTY");
  assert.match(body, /setMode\("terminal"\)/, "the surface flips to the terminal page");
  assert.match(body, /setQuery\(""\)/, "the launcher's field is emptied like every other run");
  assert.match(body, /focusTerminalView\(\)/, "focus lands on the canvas");
  assert.ok(
    !/rememberCommand/.test(body),
    "nothing was executed, so nothing joins the command history",
  );
  assert.ok(
    !/enterPluginMode/.test(body),
    "the terminal row is not a plugin mode — R51's calculator path is untouched",
  );
  // A destructive confirmation must never arm on it (the same guard the three
  // plugin doors have carried since R51).
  assert.match(
    actions,
    /item\.action === "clipboard" \|\| item\.action === "browser" \|\| item\.action === "calculator" \|\| item\.action === "terminal"/,
    "the terminal joins the doors that never reach the power confirmation",
  );
});

test("the union names the action and the row gets a real glyph", async () => {
  const results = stripJsComments(await read("src/launcher/LauncherResults.tsx"));
  assert.match(results, /export type SystemAction =[\s\S]*?"terminal"/);
  assert.match(
    results,
    /action === "terminal" \? \(\s*<TerminalIcon/,
    "the terminal row gets the terminal glyph, not the clipboard fallback",
  );
});

// ── 2 · the ⌘-held row ────────────────────────────────────────────────────

test("the held row's visibility is one predicate, and it is what the App reads", () => {
  // The three terms, each with its exclusion.
  assert.equal(bareTerminalRowVisible("git", true, null), true, "a query + the modifier shows it");
  assert.equal(bareTerminalRowVisible("git", false, null), false, "released, it is gone");
  assert.equal(bareTerminalRowVisible("", true, null), false, "the empty query is the front door");
  assert.equal(bareTerminalRowVisible("   ", true, null), false, "whitespace is still empty");
  assert.equal(bareTerminalRowVisible("git", true, "browser"), false, "a plugin scope owns the list");
  assert.equal(bareTerminalRowVisible("git", true, "clipboard"), false);
  assert.equal(bareTerminalRowVisible("git", true, "calculator"), false);
  assert.equal(bareTerminalRowVisible("git", true, "external"), false);
});

test("the modifier is the app's own, platform-normalized", () => {
  const held = { metaKey: true, ctrlKey: false };
  const ctrl = { metaKey: false, ctrlKey: true };
  assert.equal(appModifierHeld(held, true), true, "⌘ on macOS");
  assert.equal(appModifierHeld(ctrl, true), false, "Ctrl is not the launcher's modifier on macOS");
  assert.equal(appModifierHeld(ctrl, false), true, "Ctrl elsewhere");
  assert.equal(appModifierHeld(held, false), false, "⊞/Super is not the launcher's modifier");
  assert.equal(appModifierHeld({ metaKey: false, ctrlKey: false }, true), false);
});

test("the held row is an ordinary system row carrying the terminal action", () => {
  const row = bareTerminalRow(en);
  assert.equal(row.type, "system");
  assert.equal(row.id, BARE_TERMINAL_ROW_ID);
  assert.equal(row.action, "terminal", "the same action as the search row — one runner");
  assert.equal(row.title, en(BARE_TERMINAL_TITLE_KEY));
  assert.equal(row.subtitle, en(BARE_TERMINAL_SUBTITLE_KEY));
  assert.equal(bareTerminalRow(zh).title, zh(BARE_TERMINAL_TITLE_KEY), "bilingual");
  assert.equal(zh(BARE_TERMINAL_TITLE_KEY), "打开终端");
  assert.equal(zh(BARE_TERMINAL_SUBTITLE_KEY), "空白会话 — 不执行命令");
  // It is runnable by the launcher's own rule, and it earns a two-line box.
  const content = resultRowContent(row, en);
  assert.equal(content.subtitle, en(BARE_TERMINAL_SUBTITLE_KEY));
  assert.equal(
    content.subtitle === null ? null : ROW_HEIGHT_TWO_LINE,
    ROW_HEIGHT_TWO_LINE,
    "a row with a subtitle is charged the two-line height",
  );
});

test("the App appends the row through the predicate and bills it as a row", async () => {
  const app = stripJsComments(await read("src/App.tsx"));
  assert.match(
    app,
    /const showBareTerminalRow = bareTerminalRowVisible\(query, appModifierDown, launcherScope\);/,
    "the one predicate is computed from the field, the modifier and the scope",
  );
  assert.match(
    app,
    /return showBareTerminalRow \? \[\.\.\.base, bareTerminalRow\(t\)\] : base;/,
    "the row is appended below the query's own rows",
  );
  // Billing: the row is in `displayedResults`, so the existing list accounting
  // charges it — no second height term was invented for it (R58's rule).
  assert.match(
    app,
    /launcherListUnits\(displayedResults\.map\(launcherRowHeightUnits\)\)/,
    "the list's units still come from the displayed rows",
  );
  assert.match(app, /const launcherRows = Math\.max\(\s*1,/, "the row count still counts the list");
  assert.ok(
    !/bareTerminal/.test(app.split("const launcherListUnitsRaw")[1] ?? ""),
    "no R60-specific term inside the height formula",
  );
  assert.ok(
    !/bareTerminal/.test(app.split("const launcherHeight =")[1] ?? ""),
    "and none inside `launcherHeight` either",
  );
  // The height effect already re-runs on the row count, which is what makes the
  // appearance grow the window in the same commit the row is painted.
  assert.match(
    app,
    /useLauncherHeight\(mode, collapsedCardRef, launcherHeight, \[[\s\S]{0,200}?displayedResults\.length/,
    "the window follows the row count it already followed",
  );
});

test("Enter follows the selection, so the held row is not a dead row", async () => {
  // The identity check is the whole exception: the held row is told apart from
  // the terminal *system* row a query can match (`system-terminal` — the ids
  // differ), and from every other row.
  assert.equal(isBareTerminalRow(bareTerminalRow(en)), true);
  assert.equal(isBareTerminalRow(undefined), false);
  // The query-matched terminal system row is `system-` + its action; the held
  // row is its own id, so the two never collapse into one exception.
  assert.equal(BARE_TERMINAL_ROW_ID, "system-terminal-bare");
  const other: Parameters<typeof isBareTerminalRow>[0] = { id: "system-terminal" };
  assert.equal(isBareTerminalRow(other), false, "only the held row is the exception");
  const actions = stripJsComments(await read("src/hooks/useLauncherActions.ts"));
  assert.match(
    actions,
    /matchesShortcutModifiers\(event, shortcuts\.select_result\) &&[\s\S]{0,1000}?!isBareTerminalRow\(launcherResults\[selectedResultIndex\]\)[\s\S]{0,80}?\) \{\s*event\.preventDefault\(\);\s*executeActionBar\(actionBar\);/,
    "the modifier-Enter chord yields to the held row and to nothing else",
  );
  // And the plain Enter still runs the selected row, held row included.
  assert.match(
    actions,
    /if \(actionBar && \(selectedActionBar \|\| !launcherResults\.length\)\) \{\s*executeActionBar\(actionBar\);/,
    "plain Enter runs the selected row when the selection is on a row",
  );
  // ⌘<slot> reaches it too, through the shared slot map — no private key path.
  assert.match(actions, /resultIndexForSlot\(resultShortcutSlots, resultNumber\)/);
});

test("the held row cannot claim the action bar or the focus", async () => {
  const app = stripJsComments(await read("src/App.tsx"));
  // The action bar's gate still reads the *matched* rows (`launcherResults`),
  // never the composed list: an unmatched query keeps showing nothing even while
  // the modifier is held (R37's rule).
  assert.match(
    app,
    /: launcherResults\.length > 0 \|\| fileRows\.length > 0\s*\? actionBar\s*: null;/,
    "the bar is still gated on the query's own matches",
  );
  // The row is a `button tabIndex={-1}` like every other row (the renderer's one
  // row template), so it never takes the keyboard: the field stays the owner and
  // only the arrows move the selection.
  const results = stripJsComments(await read("src/launcher/LauncherResults.tsx"));
  assert.match(results, /role="option"[\s\S]{0,200}?tabIndex=\{-1\}/, "rows never take focus");
  // And the modifier tracking is passive: it must not swallow the press.
  const hook = stripJsComments(await read("src/hooks/useAppModifierHeld.ts"));
  assert.match(hook, /window\.addEventListener\("keydown", sync, true\)/);
  assert.match(hook, /window\.addEventListener\("keyup", sync, true\)/);
  assert.match(
    hook,
    /window\.addEventListener\("blur", release\)/,
    "a chord that ends while the window is away cannot leave the row stuck on",
  );
  assert.ok(
    !/preventDefault|stopPropagation/.test(hook),
    "the listener is passive — every existing chord keeps its press",
  );
});

// ── 3 · the terminal page's empty state ───────────────────────────────────

test("the empty page is a state, and it is debounced past the spawn window", () => {
  assert.equal(terminalPageEmpty("terminal", false), true);
  assert.equal(terminalPageEmpty("terminal", true), false, "a session makes it non-empty");
  assert.equal(terminalPageEmpty("collapsed", false), false, "the launcher is not this page");
  assert.equal(terminalPageEmpty("settings", false), false);
  // A fresh spawn takes a broker round trip before it describes itself, so the
  // hint must wait one idle beat or it would flash on every ordinary run.
  assert.ok(TERMINAL_EMPTY_HINT_DELAY >= 120, "long enough to cover a spawn");
  assert.ok(TERMINAL_EMPTY_HINT_DELAY <= 400, "short enough to feel immediate");
});

test("Enter is claimed by the empty page and by nothing else", async () => {
  const keyboard = stripJsComments(await read("src/hooks/useAppKeyboard.ts"));
  assert.match(
    keyboard,
    /if \(terminalEmpty && event\.key === "Enter"\) \{\s*event\.preventDefault\(\);\s*openBlankTerminal\(\);\s*return;/,
    "the empty page's Enter starts the session the hint offers",
  );
  // The claim sits *after* the dismiss table, so Esc / Cmd+W still leave first.
  const dismiss = keyboard.indexOf('resolveDismissRule("terminal"');
  const claim = keyboard.indexOf("terminalEmpty && event.key");
  assert.ok(dismiss !== -1 && claim > dismiss, "the way out is resolved before the way in");
  const app = stripJsComments(await read("src/App.tsx"));
  assert.match(app, /terminalEmpty: terminalEmptyHint,/);
  assert.match(app, /openBlankTerminal: \(\) => void openTerminalSession\(\),/);
  assert.match(
    app,
    /const terminalHasSession = Boolean\(mainSessionIdentity \|\| terminalResident\);/,
    "a described session or a retained exit frame both count as content",
  );
});

test("a session is always described, so the hint can never cover a live one", async () => {
  const hook = stripJsComments(await read("src/hooks/useTerminalView.ts"));
  // The spawn path describes itself (long-standing), and closing clears it.
  assert.match(hook, /void describeMainSession\(brokerSessionId, initialCommand\)/);
  const close = hook.slice(hook.indexOf("const closeTerminalSession"), hook.indexOf("const ensureTerminalSession"));
  assert.match(
    close,
    /setMainSessionIdentity\(null\)/,
    "closing the session clears the identity the empty state reads",
  );
  // R60 · the attach path had never described itself, so a resumed session would
  // have read as an absent one.
  const actions = stripJsComments(await read("src/hooks/useLauncherActions.ts"));
  const resume = actions.slice(
    actions.indexOf("const resumeTerminalSession"),
    actions.indexOf("const launchApplication"),
  );
  assert.match(
    resume,
    /void describeMainSession\(brokerSessionId, null\)/,
    "an attached session is described like a spawned one",
  );
});

test("the hint is inline in the canvas region, not a floating layer", async () => {
  const app = await read("src/App.tsx");
  const at = app.indexOf('className="terminal-empty"');
  assert.notEqual(at, -1, "the empty state must be rendered");
  // It is a child of the canvas mount (the same parent as the canvas itself), so
  // the canvas keeps its box underneath.
  const mount = app.slice(app.indexOf('className="terminal-panel__mount"'), at);
  assert.ok(mount.length > 0, "the hint is inside the mount element");
  assert.ok(
    !/aria-modal|role="dialog"/.test(app.slice(at - 400, at + 1600)),
    "the empty state is not a dialog",
  );
  const css = await read("src/styles/terminal.css");
  const block = css.slice(css.indexOf(".terminal-empty {"), css.indexOf(".terminal-feedback {"));
  assert.match(block, /position: absolute;/, "it is laid over the (empty) canvas region");
  assert.match(block, /inset: 0;/);
  assert.ok(!/box-shadow:\s*var\(--elev-3\)/.test(block), "no floating elevation rung");
  assert.ok(!/background:\s*var\(--accent/.test(block), "never an accent fill");
  assert.match(block, /border-radius: var\(--radius-xs\)/, "the keycap follows the ladder");
  // Both hint lines are real actions: the key that runs them and the words.
  assert.match(app, /<kbd className="terminal-empty__key">Enter<\/kbd>/);
  assert.match(app, /formatShortcut\(shortcuts\.new_command\)/);
  assert.equal(en("terminal.emptyNew"), "New blank session");
  assert.equal(zh("terminal.emptyNew"), "新建空白会话");
  assert.equal(en("terminal.emptyBack"), "Back to search");
  assert.equal(zh("terminal.emptyBack"), "返回搜索");
});

// ── 4 · the invariants this round must not move ───────────────────────────

test("no key binding, no shortcut action and no subline was invented for R60", async () => {
  const shortcuts = stripJsComments(await read("src/shortcuts.ts"));
  const actions = /export type ShortcutAction =[\s\S]*?;/.exec(shortcuts);
  assert.ok(actions, "the shortcut-action union must exist");
  // `open_external_terminal` is an existing member; *this* round adds nothing to
  // the union, so no member is the bare word.
  assert.ok(!/"terminal"/.test(actions![0]), "R60 adds no configurable shortcut (R55's map is intact)");
  const custom = stripJsComments(await read("src/custom-shortcuts.ts"));
  assert.ok(!/"terminal"/.test(custom), "and no custom action either");
  // R52 · the chips-row predicate still drives the subline, untouched.
  const app = stripJsComments(await read("src/App.tsx"));
  assert.match(app, /filterRowVisible,\s*\)/, "`launcherContentHeight` still takes the chips flag");
  assert.match(app, /const launcherSectionTitle = !launcherScope && !query\.trim\(\) && !fileRows\.length;/);
});

test("the appearance is one row and the release cannot flap the window", () => {
  // The window height is the sum of the rows it holds (R58), so the held row
  // costs exactly one row plus the grid gap it adds — and the release is a
  // one-row shrink, which is the case R43's hysteresis was written for: the
  // window holds the taller band instead of snapping back, so hold/release
  // cannot flap it.
  const rows = [ROW_HEIGHT_TWO_LINE, ROW_HEIGHT_TWO_LINE, ROW_HEIGHT_TWO_LINE];
  const held = [...rows, ROW_HEIGHT_TWO_LINE];
  const height = (units: number[], count: number) =>
    launcherContentHeight(launcherListUnits(units), count, 1, Number.POSITIVE_INFINITY, true, false, false);
  assert.equal(height(held, held.length) - height(rows, rows.length), ROW_HEIGHT_TWO_LINE + 1);
  const heldUnits = launcherListUnits(held);
  assert.equal(
    resolveLauncherUnits(heldUnits, launcherListUnits(rows)),
    heldUnits,
    "one row back is absorbed, exactly as any other row leaving the list",
  );
  assert.ok(
    resolveLauncherUnits(heldUnits, launcherListUnits(rows) - ROW_HEIGHT_TWO_LINE - 1) < heldUnits,
    "and the hold is not permanent: a real collapse still takes the window down",
  );
});

test("the launcher still accounts for the row through the shared row heights", () => {
  // A system row that prints a subtitle is two-line; the held row's own shape is
  // therefore the same 42u box every other system row gets. Pinned as a number
  // because the window height is now the sum of these.
  const row = bareTerminalRow(en);
  assert.notEqual(resultRowContent(row, en).subtitle, null);
  assert.equal(ROW_HEIGHT_TWO_LINE, 42);
  // And the matched query keeps its default selection with the row present: the
  // row is appended *after* the default-selection decision, which reads the
  // query's own matches (`shouldDefaultToActionBar`), so an empty list stays on
  // the shell fallback when the modifier is not held.
  assert.equal(shouldDefaultToActionBar("zzzz", "shell", 0, 0, false), true);
});
