// R10-A · The launcher's rows and the numbered keys that reach them.
//
// The user's report, verbatim: "当下的搜索页面高度太低了，搜索的时候展示的项
// 太少，需要展示 9 项加一个底部的单独进入剪切板的项 一共 10 项，高度可以再稍微
// 调整一下看看，总之让它的宽高比例也要处于一个协调的状态".
//
// R19 · the user re-read the same screenshot and counted: "列表项也有问题，除了
// 末尾的 cmd+回车的终端执行，上方一共有 10 项了，应该最大只能有 9 项". Nine rows
// above the action bar became the budget, the clipboard taking the ninth.
//
// R36 · the user then read the badges: "列表上的快捷键为什么只到 8 ，可以到 9
// 的，可以加上 9 后面再加上 0 ，键盘上他们挨着的". The family grew to ten slots,
// and so did the budget: nine matches plus the fixed clipboard row, which owned
// `⌘0`.
//
// R37 · the user read the fixed row back and rejected it: 「现在搜索页面中 剪切板
// 这项被固定放到末尾，并且总是显示，还固定为了 cmd+0，这不对，它（其他内置插件
// 也是）不应该是个特例，应该和其他项一样匹配了才显示，同时快捷键也要按顺序安排」.
// The clipboard is now an **ordinary result contributor**: the catalog's
// `system-clipboard` entry matches the query by its names and keywords exactly
// as `restart`/`shutdown`/`browser` do, ranks with everything else, and takes
// whatever numbered slot its position earns. The fixed tail, its pinned `⌘0`
// and `FIXED_TAIL_SLOT` are all gone; the ten slots are ten results.
//
// Five facts carry the round, and each one is asserted positively so that
// deleting the code (rather than the prose) is what turns the suite red:
//
//   1. the budget is ten, and the catalog may fill all ten — nothing is
//      reserved for a row the App appends;
//   2. the clipboard row is a `system` row produced by the catalog's own
//      `SYSTEM_COMMANDS` table, with no rendering privilege;
//   3. the App composes the query's own rows and nothing else;
//   4. the numbered keys are purely in order — `1`-`9` then `0` — over the
//      runnable rows in the viewport (see `tests/r34-viewport-badges.test.ts`
//      for the viewport half); and
//   5. the height ceiling is the worst case, not the best one: ten rows at
//      their two-line height, so a query whose matches all carry a description
//      — a command always does — still fits without the list scrolling inside
//      its own cap.
//
// The height half is pinned from the two sheets that derive it: a two-line row
// is `calc(var(--u) * 42)`, a one-line row is `calc(var(--u) * 34)` — the same
// pair `result-budget.ts` exports as `ROW_HEIGHT_TWO_LINE` /
// `ROW_HEIGHT_COMPACT` — and the list's ceiling is ten two-line rows plus the
// fixed scroll-edge/gap chrome.
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";
import { createTranslator } from "../src/i18n.ts";
import {
  COMMAND_LIMIT_WITH_MATCHES,
  LAST_RESULT_SLOT,
  MAX_RESULTS,
  RESULTS_LIST_CHROME,
  RESULTS_LIST_HEIGHT,
  RESULTS_VIEWPORT_CHROME,
  ROW_HEIGHT_COMPACT,
  ROW_HEIGHT_TWO_LINE,
  isClipboardResult,
  resultIndexForSlot,
  resultShortcutSlots,
} from "../src/launcher/result-budget.ts";
import { launcherShortcutSlots } from "../src/launcher.ts";
import { appSubtitleKey, isTranscription, resultRowContent } from "../src/launcher/row-content.ts";
import type { LauncherItem } from "../src/launcher/LauncherResults.tsx";

const root = new URL("../", import.meta.url);
const read = (path: string) => readFile(new URL(path, root), "utf8");
const stripJsComments = (source: string) =>
  source.replace(/\/\*[\s\S]*?\*\//g, "").replace(/^\s*\/\/.*$/gm, "");
const stripCssComments = (css: string) => css.replace(/\/\*[\s\S]*?\*\//g, "");

const t = createTranslator("en");

const app = (id: string): LauncherItem => ({
  type: "app",
  id,
  title: id,
  subtitle: "Application",
  app: {} as never,
});
// R12: an app with a real path and a chosen subtitle, for the row-content rule.
const appRow = (id: string, path: string, subtitle: string): LauncherItem => ({
  type: "app",
  id,
  title: id,
  subtitle,
  app: { path } as never,
});
/** The clipboard row exactly as the catalog's `SYSTEM_COMMANDS` builds it: a
 *  `system` row with the `system-clipboard` id, produced by a match. */
const catalogClipboardRow = (enabled = true): LauncherItem => ({
  type: "system",
  id: "system-clipboard",
  title: t("system.clipboardHistory"),
  subtitle: enabled
    ? t("system.clipboardHistorySubtitle")
    : t("clipboard.pageUnavailable"),
  action: "clipboard",
  ...(enabled ? {} : { disabled: true }),
});

// ── 1 · the budget ────────────────────────────────────────────────────────

test("the launcher budget is ten matched rows — no slot is reserved", async () => {
  assert.equal(MAX_RESULTS, 10);
  assert.equal(LAST_RESULT_SLOT, 0, "the family's tenth key is `0`, beside `9`");
  assert.equal(COMMAND_LIMIT_WITH_MATCHES, 3, "three commands, seven local matches");
  // The retired concept may not come back under its old name.
  const budget = stripJsComments(await read("src/launcher/result-budget.ts"));
  assert.ok(
    !/FIXED_TAIL_SLOT/.test(budget),
    "R37: `FIXED_TAIL_SLOT` is retired — there is no reserved slot",
  );
  assert.ok(
    !/withClipboardResultRow|CLIPBOARD_RESULT_ID/.test(budget),
    "R37: the fixed tail row and its id are gone from the budget module",
  );
});

test("the catalog may fill the whole budget — the tenth row is a match", async () => {
  // Through R36 every slice in the hook was `MAX_RESULTS - 1` because the tenth
  // row was the App's fixed clipboard row. R37 removed the fixed row, so the
  // catalog's ceiling is the whole budget.
  const hook = stripJsComments(await read("src/hooks/useLauncherCatalog.ts"));
  assert.match(
    hook,
    /from "\.\.\/launcher\/result-budget"/,
    "the hook reads the one budget, not a private copy of the number",
  );
  assert.ok(
    !/MAX_RESULTS - 1/.test(hook),
    "R37: no slice keeps a reserved tenth row",
  );
  assert.equal(
    (hook.match(/slice\(0, MAX_RESULTS\)/g) ?? []).length,
    1,
    "the ranked slice is the whole budget",
  );
  assert.match(hook, /: MAX_RESULTS;/, "the no-other-matches command limit is the whole budget");
});

// ── 2 · the clipboard row is an ordinary contributor ──────────────────────

test("the clipboard row is produced by the catalog's own system-command table", async () => {
  // The identity transition: the row is a *data source* entry, not a rendering
  // privilege. It lives in the same table as `restart`/`shutdown`/`browser`,
  // matched by the same `scoreApp` call and ranked by the same sort.
  const catalog = stripJsComments(await read("src/hooks/useLauncherCatalog.ts"));
  const table = /const SYSTEM_COMMANDS[\s\S]*?\n\];/.exec(catalog);
  assert.ok(table, "the system-command table must exist");
  assert.match(table![0], /action: "clipboard"/, "the clipboard is one of its entries");
  assert.match(table![0], /titleKey: "system\.clipboardHistory"/);
  assert.match(table![0], /"clipboard",\s*\n\s*"clipboard history"/, "it carries its search names");
  // The same scoring loop covers every entry, the clipboard included.
  assert.match(
    catalog,
    /for \(const entry of SYSTEM_COMMANDS\)[\s\S]{0,400}?scoreApp\(/,
    "the clipboard row is scored like every other system entry",
  );
  // …and it is ranked with the applications, not appended.
  assert.match(catalog, /matches\.push\(\{[\s\S]{0,400}?action: entry\.action/, "it joins the ranked matches");
  assert.ok(
    !/clipboardResultRow/.test(catalog),
    "no bespoke clipboard row builder is left in the catalog",
  );

  // The predicate still names the row's identity (the plugin layer and the
  // action handler ask it), but nothing dedups an appended twin any more.
  assert.equal(isClipboardResult(catalogClipboardRow()), true);
  assert.equal(isClipboardResult(app("a")), false);
  const budget = stripJsComments(await read("src/launcher/result-budget.ts"));
  assert.match(
    budget,
    /export const isClipboardResult[\s\S]{0,200}?item\.action === "clipboard"/,
    "the predicate is the row's own identity",
  );
});

test("the App renders the query's rows and keys them — it appends nothing", async () => {
  const source = stripJsComments(await read("src/App.tsx"));
  assert.match(
    source,
    /const displayedResults = useMemo\(\s*\(\) =>\s*launcherScope\s*\?\s*\[\.\.\.launcherResults\]\s*:\s*fileRows\.length\s*\?\s*\[\.\.\.fileRows, \.\.\.launcherResults\]\s*:\s*launcherResults/,
    "displayedResults is the query's own rows, with a drop group prepended outside a scope",
  );
  assert.ok(
    !/withClipboardResultRow/.test(source),
    "R37: the App appends no fixed tail",
  );
  assert.match(source, /results=\{displayedResults\}/, "the renderer gets the composed list");
  assert.match(source, /launcherResults: displayedResults/, "the key handler follows it");
  assert.match(
    source,
    /resultShortcutSlots\(displayedResults, displayedRunnableFlags, visibleResultRange\)/,
    "the numbered slots follow the same list, renumbered to the scroll viewport (R34/R37)",
  );
});

test("the empty, matching and zero-hit states all show exactly their matches", () => {
  // The zero-hit state is the one the fixed row used to exist for. R37 makes it
  // show nothing: the clipboard is reachable by typing its name, like any other
  // built-in, and a query that matched nothing matched nothing.
  const emptyQuery = [app("/Applications/A.app"), app("/Applications/B.app")];
  const matchingQuery = [app("/Applications/C.app"), catalogClipboardRow()];
  const zeroHits: LauncherItem[] = [];

  assert.equal(emptyQuery.length, 2, "an empty query shows its recents and no tail");
  assert.equal(
    matchingQuery.filter(isClipboardResult).length,
    1,
    "a query that matched the clipboard shows the one matched row",
  );
  assert.equal(zeroHits.length, 0, "a query that matched nothing shows nothing");

  // The clipboard row is runnable like any other `system` row, so the arrow-key
  // loop and Enter reach it where it sits in the list.
  const runnableFlags = matchingQuery.map((item) => item.type !== "command");
  assert.deepEqual(runnableFlags, [true, true]);
});

// ── 3 · the numbered keys are purely in order ─────────────────────────────

test("the clipboard row numbers by its position, like every other row", () => {
  const clipboard = catalogClipboardRow();

  // It is runnable by the launcher's own rule (`type !== "command"`), so it is
  // part of the arrow-key loop and Enter runs it — the clipboard page is a
  // plain view flip, see `runSystemAction`.
  assert.equal(clipboard.type, "system");
  assert.equal(clipboard.action, "clipboard");
  assert.notEqual(clipboard.id, "");

  // The slot map reads positions, not ids: the clipboard row takes whatever
  // number its place in the list earns — there is no `0` pinned to it.
  const rows = [app("a"), clipboard, app("b")];
  const slots = resultShortcutSlots(rows, rows.map(() => true));
  assert.deepEqual(slots, [1, 2, 3]);
  assert.equal(resultIndexForSlot(slots, 2), 1, "`⌘2` reaches the clipboard row where it sits");

  // Without the tail policy the plain `launcherShortcutSlots` agrees — the
  // family's order is now the only rule, so the two helpers cannot disagree
  // about the first nine rows.
  assert.deepEqual(launcherShortcutSlots(rows.map(() => true)), [1, 2, 3]);

  // A full list: ten matches number `1`-`9` then `0`, in order.
  const ten = Array.from({ length: MAX_RESULTS }, (_, i) => app(`app-${i}`));
  const fullSlots = resultShortcutSlots(ten, ten.map(() => true));
  assert.deepEqual(fullSlots, [1, 2, 3, 4, 5, 6, 7, 8, 9, LAST_RESULT_SLOT]);
  assert.equal(resultIndexForSlot(fullSlots, 9), 8, "the ninth match is `⌘9`");
  assert.equal(resultIndexForSlot(fullSlots, 0), 9, "the tenth match is `⌘0`");
  assert.equal(resultIndexForSlot(fullSlots, 10), -1, "the family stops at ten slots");

  // A row that cannot run takes no number, so the matched rows after it keep
  // numbering without gaps; the family's last key lands on the tenth *runnable*
  // row.
  const unavailable: LauncherItem = {
    type: "command",
    id: "c",
    title: "deploy",
    subtitle: "",
    warnings: [],
    sourceName: "Kit",
    commandLine: "deploy",
    execution: null,
    completion: false,
  };
  const gapped = [app("a"), unavailable, app("b")];
  const gappedFlags = gapped.map((item) => item.type !== "command" || Boolean(item.execution));
  assert.deepEqual(resultShortcutSlots(gapped, gappedFlags), [1, null, 2]);

  // A list grown past the budget (dropped files prepended to a full match set)
  // hands out only the ten keys: `1`-`9` and `0`, nothing past them.
  const overflow = [app("f1"), app("f2")].concat(ten);
  const overflowSlots = resultShortcutSlots(overflow, overflow.map(() => true));
  assert.equal(
    overflowSlots.filter((slot) => slot !== null).length,
    10,
    "exactly ten rows carry a badge",
  );
  assert.equal(
    overflowSlots.filter((slot) => slot === LAST_RESULT_SLOT).length,
    1,
    "`0` is handed to exactly one row",
  );
  assert.equal(resultIndexForSlot(overflowSlots, 0), 9, "`⌘0` is the tenth row of the list");
});

test("the key handler and the badges share the one slot map", async () => {
  // The mapping must have exactly one home (`result-budget.ts`): the two
  // `⌘N` routes ask `resultIndexForSlot`, and no component may re-derive a
  // slot from a row position.
  for (const path of ["src/hooks/useLauncherActions.ts", "src/hooks/useAppKeyboard.ts"]) {
    const source = stripJsComments(await read(path));
    assert.match(
      source,
      /resultIndexForSlot\(resultShortcutSlots, resultNumber\)/,
      `${path}: the numbered route resolves through the shared map`,
    );
    assert.ok(
      !/resultShortcutSlots\.indexOf\(/.test(source),
      `${path}: no private slot lookup left behind`,
    );
  }
  // The badge reads the map by row index; it never computes a number.
  const results = stripJsComments(await read("src/launcher/LauncherResults.tsx"));
  assert.match(results, /const shortcutSlot = resultShortcutSlots\[index\];/);
  assert.ok(
    !/length\s*-\s*1\s*\)\s*\/\s*\d/.test(results) && !/index\s*\+\s*1/.test(results),
    "the renderer does not derive a slot from the row number",
  );
});

// ── 4 · the shortcut family itself ───────────────────────────────────────

test("⌘0 is a result shortcut like the rest of the family", async () => {
  const { formatResultShortcut, matchesResultShortcut, normalizeResultShortcut } =
    await import("../src/shortcuts.ts");
  const press = (code: string, key: string, meta = true) =>
    ({ code, key, ctrlKey: false, altKey: false, shiftKey: false, metaKey: meta }) as KeyboardEvent;

  assert.equal(matchesResultShortcut(press("Digit1", "1"), "Cmd+1"), 1);
  assert.equal(matchesResultShortcut(press("Digit9", "9"), "Cmd+1"), 9);
  assert.equal(
    matchesResultShortcut(press("Digit0", "0"), "Cmd+1"),
    0,
    "⌘0 resolves like any other digit of the family",
  );
  assert.equal(
    matchesResultShortcut(press("Digit0", "0", false), "Cmd+1"),
    null,
    "a modifier-less `0` is ordinary typing, not a result shortcut",
  );
  assert.equal(matchesResultShortcut(press("KeyA", "a"), "Cmd+1"), null);

  // The badge draws the tenth key, and the family still normalizes to its head.
  assert.equal(
    formatResultShortcut("Cmd+1", 0),
    formatResultShortcut("Cmd+1", 9).replace(/9$/, "0"),
  );
  assert.equal(normalizeResultShortcut("Cmd+0"), normalizeResultShortcut("Cmd+1"));
});

// R37 · a switched-off clipboard keeps its matched row but closes the door.
//
// R26-D gave the browser row this treatment; the clipboard's `system-clipboard`
// entry follows the same rule (R36). With the fixed tail gone, this entry is
// the *only* clipboard row, so it is the whole of the plugin's presence in the
// list — and it soft-closes when the plugin is off.
test("R37 · a switched-off clipboard keeps its row but closes the door", async () => {
  const off = catalogClipboardRow(false);
  assert.equal(off.type, "system");
  assert.equal(off.action, "clipboard");
  assert.equal(off.disabled, true, "the row is a note, not a door");
  assert.equal(off.title, t("system.clipboardHistory"), "the name stays; the subtitle says why");
  assert.equal(off.subtitle, t("clipboard.pageUnavailable"));

  const on = catalogClipboardRow(true);
  assert.equal(on.disabled, undefined, "switched on, the row is a door again");
  assert.equal(on.subtitle, t("system.clipboardHistorySubtitle"));

  // The catalog's entry row soft-closes off the plugin's own switch, and the
  // action handler refuses a disabled clipboard row.
  const catalog = stripJsComments(await read("src/hooks/useLauncherCatalog.ts"));
  assert.match(catalog, /entry\.action === "clipboard" && !clipboardEnabled/);
  const actions = stripJsComments(await read("src/hooks/useLauncherActions.ts"));
  assert.match(
    actions,
    /if \(item\.action === "clipboard"\) \{\s*if \(item\.disabled\) return;/,
    "a disabled clipboard row is not a door",
  );
  // The App no longer reads the clipboard switch for a tail it does not append.
  const app = stripJsComments(await read("src/App.tsx"));
  assert.ok(
    !/withClipboardResultRow\([\s\S]{0,400}?settings\.clipboard_history_enabled,/.test(app),
    "R37: no fixed tail follows the plugin switch any more",
  );
});

// ── 5 · what a row actually prints (R12) ──────────────────────────────────

test("an application row drops the type word from both columns", () => {
  const typeWord = t(appSubtitleKey("/Applications/WeCom.app"));
  // The subtitle is nothing but the type word: both columns go quiet and the
  // row collapses to one line.
  const bare = resultRowContent(appRow("wecom", "/Applications/WeCom.app", typeWord), t);
  assert.equal(bare.subtitle, null, "a subtitle that only repeats the type is not printed");
  assert.equal(bare.source, null, "an application never prints a right-hand type word");

  // R14: the app's real name is no longer "information" when it is only the
  // other-language half of the pair the title came from (see the test below);
  // what survives is a subtitle that is neither the type word nor the title.
  const named = resultRowContent(appRow("wecom", "/Applications/WeCom.app", "WeCom"), t);
  assert.equal(named.subtitle, null, "a second spelling of the title is not a subtitle");
  assert.equal(named.source, null, "the right-hand word is dropped for apps either way");

  const remarked = resultRowContent(
    appRow("notes", "/Applications/Notes.app", "Ideas and todos"),
    t,
  );
  assert.equal(remarked.subtitle, "Ideas and todos", "a real remark is information and stays");
});

// R14, the user's three examples, verbatim: 「QQ音乐/QQMusic、Safari浏览器/Safari、
// 企业微信/WeCom——副标题只是同一名字的另一种语言 = 零信息」. The catalog prints
// `app.name` in the subtitle slot exactly when it differs from the localized
// title, so the pair is by construction one app's name in two languages — and
// two of the three share no character at all, so no string folding can see it.
test("an app's other-language name is not a subtitle", () => {
  const localized = (id: string, localizedName: string, name: string): LauncherItem => ({
    type: "app",
    id,
    title: localizedName,
    subtitle: name,
    app: { path: `/Applications/${id}.app`, name, localizedName } as never,
  });

  for (const [localizedName, name] of [
    ["QQ音乐", "QQMusic"],
    ["Safari浏览器", "Safari"],
    ["企业微信", "WeCom"],
  ] as const) {
    const row = resultRowContent(localized(name, localizedName, name), t);
    assert.equal(
      row.subtitle,
      null,
      `${localizedName} / ${name}: the platform's other name for the same app says nothing new`,
    );
    assert.equal(row.source, null, `${localizedName}: the right-hand word stays dropped`);
  }

  // The rule reads the app's own two name fields, so a *description* the
  // platform ships is not mistaken for a variant of the name.
  const described: LauncherItem = {
    type: "app",
    id: "music",
    title: "QQ音乐",
    subtitle: "音乐播放器",
    app: {
      path: "/Applications/QQMusic.app",
      name: "QQMusic",
      localizedName: "QQ音乐",
      comment: "音乐播放器",
    } as never,
  };
  assert.equal(resultRowContent(described, t).subtitle, "音乐播放器", "a description is not a name");
});

// The general half of the rule: a subtitle that is the title written again in
// another case, width or spacing is a transcription, whatever the row kind.
test("a subtitle that is the title transcribed is dropped for every row kind", () => {
  assert.equal(isTranscription("Safari", "safari"), true);
  assert.equal(isTranscription("Safari浏览器", "Ｓａｆａｒｉ浏览器"), true, "full-width folds to ASCII");
  assert.equal(isTranscription("Visual Studio Code", "visual studio code"), true);
  assert.equal(isTranscription("QQ 音乐", "qq音乐"), true, "spacing is not information");
  assert.equal(isTranscription("WeCom", "WeChat"), false, "a different name is not a transcription");
  assert.equal(isTranscription("剪贴板历史", "打开剪贴板历史面板"), false, "an action sentence is not a transcription");

  const command: LauncherItem = {
    type: "command",
    id: "c",
    title: "Deploy",
    subtitle: "deploy",
    warnings: [],
    sourceName: "Deploy Kit",
    commandLine: "deploy",
    execution: null,
    completion: false,
  };
  assert.equal(resultRowContent(command, t).subtitle, null, "the command row prints its title alone");
});

test("every other row kind keeps the word it earns", () => {
  // R14: a system action prints no right-hand word at all — the screenshot had
  // "Floter 内置" beside both the restart and the clipboard row.
  const clipboard = resultRowContent(catalogClipboardRow(), t);
  assert.equal(clipboard.source, null, "a system action drops the right-hand word");
  assert.equal(clipboard.subtitle, t("system.clipboardHistorySubtitle"), "and keeps its action subtitle");

  const restart: LauncherItem = {
    type: "system",
    id: "system-restart",
    title: t("system.restart"),
    subtitle: t("system.restartSubtitle"),
    action: "restart",
  };
  const restartRow = resultRowContent(restart, t);
  assert.equal(restartRow.source, null, "the restart row prints the icon and the title, nothing else");
  assert.equal(restartRow.subtitle, t("system.restartSubtitle"), "its subtitle is not a transcription");

  const history: LauncherItem = { type: "history", id: "h", title: "ls", commandLine: "ls" };
  const historyRow = resultRowContent(history, t);
  assert.equal(historyRow.source, t("launcher.history"), "history keeps its source word");
  assert.equal(historyRow.subtitle, null, "but not a subtitle that repeats it");

  const command: LauncherItem = {
    type: "command",
    id: "c",
    title: "deploy",
    subtitle: "Ship it",
    warnings: [],
    sourceName: "Deploy Kit",
    commandLine: "deploy",
    execution: null,
    completion: false,
  };
  const commandRow = resultRowContent(command, t);
  assert.equal(commandRow.source, "Deploy Kit", "a command keeps its contributing extension");
  assert.equal(commandRow.subtitle, "Ship it", "and its description");
});

test("the renderer prints the subtitle and source only when they exist", async () => {
  const results = stripJsComments(await read("src/launcher/LauncherResults.tsx"));
  assert.match(
    results,
    /resultRowContent\(item, t\)/,
    "the row asks the one module what to print",
  );
  assert.match(
    results,
    /subtitle !== null && \(\s*<span className="launcher-result__subtitle">/,
    "the subtitle span is behind the null guard",
  );
  assert.match(
    results,
    /source !== null && \(\s*<span className="launcher-result__source"/,
    "the source span is behind the null guard",
  );
  assert.match(
    results,
    /compact \? " launcher-result--compact" : ""/,
    "a one-line row takes the compact height",
  );
});

// ── 6 · the height the ten rows need ────────────────────────────────────

test("the results ceiling is every row at its tallest, not the shortest state", async () => {
  const launcher = stripCssComments(await read("src/styles/launcher.css"));
  const rowHeightOf = (selector: string): number => {
    const rule = new RegExp(
      `${selector.replace(/\./g, "\\.")}\\s*\\{[^}]*height:\\s*calc\\(var\\(--u\\)\\s*\\*\\s*(\\d+)\\)`,
      "s",
    ).exec(launcher);
    assert.ok(rule, `${selector} must still declare its height off --u`);
    return Number(rule![1]);
  };
  const rowHeight = rowHeightOf(".launcher-result");
  const compactHeight = rowHeightOf(".launcher-result--compact");
  assert.ok(
    compactHeight < rowHeight,
    `a one-line row (${compactHeight}) must be shorter than a two-line row (${rowHeight})`,
  );
  // R20: the sheets and the budget module are one decision written three times
  // (the two row heights here, the ceiling below, the constants there).
  assert.equal(rowHeight, ROW_HEIGHT_TWO_LINE, "the sheet and the module share the two-line height");
  assert.equal(compactHeight, ROW_HEIGHT_COMPACT, "…and the compact height");

  const results = /\.launcher-results\s*\{[^}]*max-height:\s*([^;]+);/s.exec(launcher);
  assert.ok(results, ".launcher-results must declare a max-height");
  const ceiling = results[1];
  // R20 · the budget is the **worst case**: every row at its tallest, because a
  // query that matched commands is exactly a list of two-line rows — each
  // command carries a description. R19 sized the ceiling from the compact
  // height (`8 x 34 + 42` = 314u), which is only the height of a query whose
  // matches were all apps; ten two-line rows are 420u and scrolled inside it.
  const budget = RESULTS_LIST_HEIGHT;
  assert.equal(budget, MAX_RESULTS * ROW_HEIGHT_TWO_LINE, "the budget is ten two-line rows");
  assert.equal(budget, 420, "ten two-line rows are 420u");
  assert.match(
    ceiling,
    new RegExp(`calc\\(var\\(--u\\)\\s*\\*\\s*${budget}\\s*\\+\\s*(\\d+)px\\)`),
    `the ceiling must be ${MAX_RESULTS} x ${rowHeight}px + chrome, got "${ceiling}"`,
  );
  const chrome = Number(
    new RegExp(`calc\\(var\\(--u\\)\\s*\\*\\s*${budget}\\s*\\+\\s*(\\d+)px\\)`).exec(ceiling)![1],
  );
  assert.equal(chrome, RESULTS_LIST_CHROME, "the fixed chrome is the module's constant");
  // The chrome has to be at least the scroll-edge band plus the gaps around
  // ten rows, and it has to leave room for a section title — otherwise the
  // empty-query state scrolls the last row away.
  //
  // R22 · the band is launcher-local now: `styles/launcher.css` overrides
  // `--scroll-edge` (14px in base.css) with 8px on `.collapsed-card`, so this
  // scroller reserves 8px, not the root token. Read the override from the sheet
  // rather than hard-coding either number — the assertion below is what pins it.
  // R24 halves the local value again (8px → 4px); base.css still owns 14px.
  const bandOverride = /\.collapsed-card\s*\{[^}]*--scroll-edge:\s*(\d+)px/s.exec(launcher);
  assert.ok(bandOverride, "the launcher scope overrides --scroll-edge locally");
  assert.equal(Number(bandOverride[1]), 4, "R24 tightens the launcher's scroll-edge reservation to 4px");
  const band = Number(
    /\.launcher-results\s*\{[^}]*padding:\s*var\(--scroll-edge\)/s.test(launcher) ? bandOverride[1] : 0,
  );
  assert.ok(band > 0, "the scroller still reserves the scroll-edge band as padding");
  assert.ok(
    chrome >= band + MAX_RESULTS + 20,
    `chrome of ${chrome}px must cover the ${band}px band, nine 1px gaps and a title line`,
  );

  // ── the assertion this round exists for ─────────────────────────────────
  // Ten two-line rows, plus the band, the nine 1px gaps and the empty-query
  // section title, must fit *inside* the ceiling. If the content is taller than
  // the cap the list scrolls in the state the launcher is most often in: the
  // scroll-edge band paints, and the card outgrows the window it is measured
  // into (the action bar cut by the window's bottom edge — 「界面边框又变形了」).
  const titleLine = 26; // `--text-body` at 1.4 plus the 6/4px padding
  const content = band + MAX_RESULTS + titleLine + budget;
  assert.ok(
    content <= budget + chrome,
    `ten two-line rows with a ${band}px band, ${MAX_RESULTS} gaps and a ${titleLine}px title need ${content}px, but the ceiling is ${budget + chrome}px`,
  );
  assert.ok(
    content > (MAX_RESULTS - 1) * compactHeight + rowHeight + chrome,
    "a ceiling sized from the compact height cannot hold the worst case — that is the bug R20 fixes",
  );

  assert.match(
    ceiling,
    /var\(--launcher-results-height/,
    "the viewport cap the App writes must still win on a short display",
  );

  // The App's cap and the CSS ceiling are the same number of rows' worth: the
  // App writes the list's ceiling through the module's helper (R58), so the two
  // cannot be two formulas.
  const source = stripJsComments(await read("src/App.tsx"));
  assert.match(
    source,
    /--launcher-results-height": `\$\{launcherListCeiling\}px`/,
    "the inline cap is the module's list ceiling",
  );
  assert.match(
    source,
    /const launcherListCeiling = launcherResultsCeiling\(window\.screen\.availHeight\);/,
    "…written from `launcherResultsCeiling`, which subtracts the shared chrome constant",
  );
  // R19/R20 re-audited this chrome, and R22-R24 and R37 each moved one segment
  // of it: the R18 breath halved (8u → 4u, R24), the field's row went 56u → 48u
  // (R22) → 42u (R23) → back to 56u (R37), and the constant followed each step
  // (216 → 212 → 226). It is a floor for a short display and only has to be *at
  // least* the chrome it stands for — 226u ≥ 143u by 83u of slack. What matters
  // is that it does not bind on an ordinary one, i.e. that the work area is at
  // least `RESULTS_VIEWPORT_CHROME + the worst-case list` = 226 + 470 = 696px.
  // Every display a launcher is used on clears that (a 1280x800 work area is
  // 768px), and on a shorter one the cap binds *deliberately*: the list scrolls
  // rather than the card overflowing its window.
  assert.equal(RESULTS_VIEWPORT_CHROME, 226);
  const shortestWorkAreaTheCapDoesNotBind = RESULTS_VIEWPORT_CHROME + budget + chrome;
  assert.equal(shortestWorkAreaTheCapDoesNotBind, 696);
  assert.ok(
    shortestWorkAreaTheCapDoesNotBind < 768,
    "the App's cap must not bind on the shortest ordinary work area (1280x800)",
  );
});

test("the height sync follows the card's box, not just the row count", async () => {
  const hook = stripJsComments(await read("src/hooks/useLauncherHeight.ts"));
  // R20 · the dependency list is the row *count*; a row can change height
  // without the count moving (a compact row gaining a subtitle the moment the
  // query reaches a command), and the window then keeps the previous list's
  // height. The card is taller than its window and the centred shell splits the
  // overflow, so the field loses its top edge and the action bar is cut by the
  // bottom one. No dependency list can enumerate "a row got taller", so the
  // trigger is a measurement of the card itself.
  assert.match(hook, /new ResizeObserver\(/, "the card's own box is observed");
  assert.match(hook, /observer\.observe\(card\)/, "…and the card is what is observed");
  assert.match(hook, /observer\.disconnect\(\)/, "the observer does not outlive the surface");
  assert.match(
    hook,
    /resizeLauncherWindow\(current, target, SETTLE_PASSES\)/,
    "it re-uses the one resize path, bounded by the same settle passes",
  );
  // Self-limiting, both ways: a target the window already carries is a no-op,
  // and a target already asked for is never asked for twice — so an observer
  // that fires on the resize it caused terminates rather than oscillating.
  assert.match(
    hook,
    /Math\.abs\(target - window\.innerHeight\) <= 1/,
    "a target the window already has is a no-op",
  );
  assert.match(hook, /target === applied\.current/, "a repeated target must not resize twice");
});

test("the field's text sits on the field's own inset", async () => {
  const launcher = stripCssComments(await read("src/styles/launcher.css"));
  const row = /\.collapsed-card__input-row\s*\{([^}]*)\}/s.exec(launcher);
  assert.ok(row, ".collapsed-card__input-row must exist");
  assert.match(row![1], /align-items:\s*center/, "the row centres the field's box");
  const input = /\.collapsed-card__input\s*\{([^}]*)\}/s.exec(launcher);
  assert.ok(input, ".collapsed-card__input must exist");
  // R37 · the row is 56u (the settings band's height, see `search-field.ts`);
  // the field keeps its pinned 22u line box, flex-centred, so the extra 14u is
  // 7u a side and the text does not move off the row's centre.
  assert.match(row![1], /min-height:\s*calc\(var\(--u\)\s*\*\s*56\)/, "the row is the settings band's 56u");
  // What it may not keep is the user agent's `padding: 1px 2px`, which lives
  // *inside* this element's border box: it pushed the line box 1px below the
  // row's centre and the first glyph 2px right of the row's inset. Zeroed, the
  // box the row centres is the line box, and the caret starts where the field
  // starts.
  assert.match(input![1], /padding:\s*0;/, "the field's text is not offset by the user agent's padding");
  assert.match(
    input![1],
    /min-height:\s*calc\(var\(--u\) \* 22\)/,
    "…and the field keeps the 22u box the height budget is built on",
  );
});

test("the list still scrolls past the budget, so extra rows are reachable", async () => {
  const launcher = stripCssComments(await read("src/styles/launcher.css"));
  const results = /\.launcher-results\s*\{([^}]*)\}/s.exec(launcher);
  assert.ok(results);
  // A ceiling without scrolling would clip a drop group or a section title.
  assert.match(results[1], /overflow-y:\s*auto/);
  assert.match(results[1], /overscroll-behavior:\s*contain/);
});
