// R10-A · The launcher's rows: eight matched results, then one fixed row
// that opens the clipboard history.
//
// The user's report, verbatim: "当下的搜索页面高度太低了，搜索的时候展示的项
// 太少，需要展示 9 项加一个底部的单独进入剪切板的项 一共 10 项，高度可以再稍微
// 调整一下看看，总之让它的宽高比例也要处于一个协调的状态".
//
// R19 · the user re-read the same screenshot and counted: "列表项也有问题，除了
// 末尾的 cmd+回车的终端执行，上方一共有 10 项了，应该最大只能有 9 项". The
// action bar is not a list row, so nine rows above it is the whole budget — eight
// matches plus the fixed tail. The tail is the ninth row of a nine-row list, so
// it takes the ninth slot of the `1`-`9` family (`⌘9`), which is also the
// user's third point: "同时内置插件也要支持 cmd+n 的快捷键".
//
// Five facts carry the round, and each one is asserted positively so that
// deleting the code (rather than the prose) is what turns the suite red:
//
//   1. the budget is nine, and the catalog's own ceiling is eight — the ninth
//      slot belongs to the fixed row, not to a match;
//   2. the tail row is a clipboard system row in every query state: empty,
//      matching, and matching nothing;
//   3. a query that already matched the clipboard command does not get a second
//      clipboard row;
//   4. the tail row is selectable and runnable, and it always owns slot nine —
//      `⌘9` opens the clipboard panel, and the matched rows number 1-8 above it;
//   5. the height ceiling is eight compact rows plus the clipboard row.
//
// The height half is pinned from the two sheets that derive it: a two-line row
// is `calc(var(--u) * 42)`, a one-line row is `calc(var(--u) * 34)`, and the
// list's ceiling is eight compact result rows plus the fixed clipboard row plus
// the fixed scroll-edge/gap chrome.
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";
import { createTranslator } from "../src/i18n.ts";
import {
  CLIPBOARD_RESULT_ID,
  COMMAND_LIMIT_WITH_MATCHES,
  FIXED_TAIL_SLOT,
  MAX_RESULTS,
  RESULTS_VIEWPORT_CHROME,
  clipboardResultRow,
  isClipboardResult,
  resultIndexForSlot,
  shortcutSlotsWithFixedTail,
  withClipboardResultRow,
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
const catalogClipboardRow = (): LauncherItem => ({
  type: "system",
  id: "system-clipboard",
  title: t("system.clipboardHistory"),
  subtitle: t("system.clipboardHistorySubtitle"),
  action: "clipboard",
});

// ── 1 · the budget ────────────────────────────────────────────────────────

test("the launcher budget is nine rows: eight matches plus the clipboard row", () => {
  assert.equal(MAX_RESULTS, 9);
  assert.equal(FIXED_TAIL_SLOT, 9, "the fixed row owns the last slot of the 1-9 family");
  assert.equal(COMMAND_LIMIT_WITH_MATCHES, 3, "three commands, five local matches");
});

test("the catalog itself never returns the ninth row", async () => {
  // The fixed row is the App's, not the catalog's: every slice in the hook is
  // `MAX_RESULTS - 1`. If one of them grew to `MAX_RESULTS` the tail row would
  // push the list to ten.
  const hook = stripJsComments(await read("src/hooks/useLauncherCatalog.ts"));
  assert.match(
    hook,
    /from "\.\.\/launcher\/result-budget"/,
    "the hook reads the one budget, not a private copy of the number",
  );
  const slices = hook.match(/slice\(0, MAX_RESULTS - 1\)/g) ?? [];
  assert.equal(slices.length, 1, "the ranked slice is the catalog's own ceiling");
  assert.ok(
    /MAX_RESULTS - 1,\n\s*\);/.test(hook),
    "the empty-query recents limit is eight too",
  );
  assert.ok(
    !/slice\(0, MAX_RESULTS\)/.test(hook) && !/MAX_RESULTS \*/.test(hook),
    "nothing in the catalog may hand out a ninth matched row",
  );
});

// ── 2 · the fixed row, in all three query states ──────────────────────────

test("the clipboard row is the tail in the empty, matching and zero-hit states", () => {
  const emptyQuery = [app("/Applications/A.app"), app("/Applications/B.app")];
  const matchingQuery = [app("/Applications/C.app")];
  const zeroHits: LauncherItem[] = [];

  for (const [name, state] of [
    ["empty query", emptyQuery],
    ["matching query", matchingQuery],
    ["zero hits", zeroHits],
  ] as const) {
    const composed = withClipboardResultRow(state, t);
    assert.equal(composed.length, state.length + 1, `${name}: one row was appended`);
    const tail = composed[composed.length - 1];
    assert.equal(tail.type, "system", `${name}: the tail is a system row`);
    assert.equal(tail.action, "clipboard", `${name}: the tail opens the clipboard`);
    assert.equal(tail.id, CLIPBOARD_RESULT_ID, `${name}: the tail has the fixed id`);
    assert.equal(tail.title, t("system.clipboardHistory"), `${name}: the i18n title`);
    assert.equal(
      tail.subtitle,
      t("system.clipboardHistorySubtitle"),
      `${name}: the i18n subtitle`,
    );
    // The rows that came in keep their order; the fixed row only ever appends.
    assert.deepEqual(composed.slice(0, state.length), state, `${name}: the head is untouched`);
  }

  // Zero hits is the state the row exists for: the list is no longer empty, so
  // the clip opens and the row is visible and reachable.
  assert.equal(withClipboardResultRow([], t).length, 1);
});

test("the App composes the fixed row into the list it renders and keys", async () => {
  const source = stripJsComments(await read("src/App.tsx"));
  assert.match(
    source,
    /const displayedResults = useMemo\(\s*\(\) => withClipboardResultRow\(/,
    "displayedResults is where the tail row is appended",
  );
  assert.match(source, /results=\{displayedResults\}/, "the renderer gets the composed list");
  assert.match(source, /launcherResults: displayedResults/, "the key handler follows it");
  assert.match(
    source,
    /shortcutSlotsWithFixedTail\(displayedResults, displayedRunnableFlags\)/,
    "the numbered slots follow the same composed list",
  );
});

// ── 3 · no duplicate clipboard row ────────────────────────────────────────

test("a query that matched the clipboard command does not grow a second row", () => {
  const matched = [catalogClipboardRow()];
  const composed = withClipboardResultRow(matched, t);
  assert.equal(composed.length, 1, "the match stands in for the fixed row");
  assert.equal(composed[0].id, "system-clipboard");
  assert.equal(composed.filter(isClipboardResult).length, 1);

  // The same holds when the query matched the clipboard *among* other rows.
  const mixed = [app("/Applications/A.app"), catalogClipboardRow()];
  assert.equal(withClipboardResultRow(mixed, t).length, 2);
  assert.equal(withClipboardResultRow(mixed, t).filter(isClipboardResult).length, 1);

  // …and the append is idempotent, so a re-render cannot stack a second one.
  const once = withClipboardResultRow([], t);
  const twice = withClipboardResultRow(once, t);
  assert.equal(twice.length, 1);
  assert.deepEqual(twice, once);
});

// ── 4 · the row is runnable, and it always owns slot nine ────────────────

test("the clipboard row is runnable and always owns the ninth slot", () => {
  const tail = clipboardResultRow(t);
  assert.equal(tail.type, "system");
  assert.equal(tail.action, "clipboard");
  assert.notEqual(tail.id, "");

  // The row is runnable by the launcher's own rule (`type !== "command"`), so
  // it is part of the arrow-key loop and Enter runs it — the clipboard page is
  // a plain view flip, see `runSystemAction`.
  const rows = [app("a"), app("b"), tail];
  const runnableFlags = rows.map((item) => item.type !== "command");
  assert.deepEqual(runnableFlags, [true, true, true]);

  // R19: the tail is the ninth row, so it takes the ninth slot of the family —
  // a real `⌘9` badge, not a blank one. It owns that slot however few rows came
  // before it: the two matches number 1-2, the clipboard row is still 9.
  const slots = shortcutSlotsWithFixedTail(rows, runnableFlags);
  assert.deepEqual(slots, [1, 2, FIXED_TAIL_SLOT]);
  assert.deepEqual(
    launcherShortcutSlots(runnableFlags),
    [1, 2, 3],
    "without the tail policy the row would claim the next number — the helper is load-bearing",
  );
  assert.equal(
    resultIndexForSlot(slots, 9),
    2,
    "⌘9 resolves to the clipboard row, the same index a click on it would use",
  );

  // The full list: eight matches number one through eight, in order, and the
  // fixed row is the ninth.
  const eight = Array.from({ length: MAX_RESULTS - 1 }, (_, i) => app(`app-${i}`));
  const fullSlots = shortcutSlotsWithFixedTail(
    withClipboardResultRow(eight, t),
    eight.map(() => true).concat([true]),
  );
  assert.deepEqual(fullSlots, [1, 2, 3, 4, 5, 6, 7, 8, FIXED_TAIL_SLOT]);
  assert.equal(resultIndexForSlot(fullSlots, 9), 8, "the ninth row is the clipboard row");
  assert.equal(resultIndexForSlot(fullSlots, 10), -1, "the family stops at nine");

  // A row that cannot run takes no number, so the matched rows after it keep
  // numbering without gaps — but the tail still owns 9.
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
  const gapped = [app("a"), unavailable, app("b"), tail];
  const gappedFlags = gapped.map((item) => item.type !== "command" || Boolean(item.execution));
  assert.deepEqual(shortcutSlotsWithFixedTail(gapped, gappedFlags), [1, null, 2, FIXED_TAIL_SLOT]);

  // A list grown past the budget (dropped files prepended to a full match set)
  // must not hand slot nine to a match: the clipboard panel stays on `⌘9`.
  const overflow = [app("f1"), app("f2")].concat(eight, [tail]);
  const overflowSlots = shortcutSlotsWithFixedTail(overflow, overflow.map(() => true));
  assert.equal(overflowSlots[overflowSlots.length - 1], FIXED_TAIL_SLOT);
  assert.equal(overflowSlots.filter((slot) => slot === FIXED_TAIL_SLOT).length, 1);
  assert.equal(
    resultIndexForSlot(overflowSlots, 9),
    overflow.length - 1,
    "⌘9 reaches the clipboard row even when the list overflows the budget",
  );

  // A query that matched the clipboard command appends no fixed row, so there
  // is no ninth row to own the slot: the match numbers naturally, like any row.
  const matched = withClipboardResultRow([app("a"), catalogClipboardRow()], t);
  assert.deepEqual(
    shortcutSlotsWithFixedTail(matched, matched.map(() => true)),
    [1, 2],
    "a matched clipboard row is an ordinary match, not the fixed tail",
  );
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

// ── 6 · what a row actually prints (R12) ──────────────────────────────────

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
  const clipboard = resultRowContent(clipboardResultRow(t), t);
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

// ── 5 · the height the nine rows need ────────────────────────────────────

test("the results ceiling is eight compact rows plus the clipboard row", async () => {
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

  const results = /\.launcher-results\s*\{[^}]*max-height:\s*([^;]+);/s.exec(launcher);
  assert.ok(results, ".launcher-results must declare a max-height");
  const ceiling = results[1];
  // Eight matched rows collapse to one line each; the fixed clipboard row keeps
  // its subtitle, so it stays at the full height.
  const budget = (MAX_RESULTS - 1) * compactHeight + rowHeight;
  assert.match(
    ceiling,
    new RegExp(`calc\\(var\\(--u\\)\\s*\\*\\s*${budget}\\s*\\+\\s*(\\d+)px\\)`),
    `the ceiling must be eight x ${compactHeight}px + ${rowHeight}px + chrome, got "${ceiling}"`,
  );
  const chrome = Number(
    new RegExp(`calc\\(var\\(--u\\)\\s*\\*\\s*${budget}\\s*\\+\\s*(\\d+)px\\)`).exec(ceiling)![1],
  );
  // The chrome has to be at least the scroll-edge band (14px, see base.css)
  // plus the gaps around nine rows, and it has to leave room for a section
  // title — otherwise the empty-query state scrolls the clipboard row away.
  const band = Number(/\.launcher-results\s*\{[^}]*padding:\s*var\(--scroll-edge\)/s.test(launcher) ? 14 : 0);
  assert.ok(band > 0, "the scroller still reserves the scroll-edge band as padding");
  assert.ok(
    chrome >= band + MAX_RESULTS + 20,
    `chrome of ${chrome}px must cover the ${band}px band, nine 1px gaps and a title line`,
  );
  assert.match(
    ceiling,
    /var\(--launcher-results-height/,
    "the viewport cap the App writes must still win on a short display",
  );

  // The App's cap and the CSS ceiling are the same number of rows' worth: the
  // App subtracts only the window chrome it knows about.
  const source = stripJsComments(await read("src/App.tsx"));
  assert.match(
    source,
    /--launcher-results-height": `\$\{Math\.max\(84, window\.screen\.availHeight - RESULTS_VIEWPORT_CHROME\)\}px`/,
    "the inline cap subtracts the shared chrome constant",
  );
  // R19: the chrome the App subtracts was re-audited for R18's 12u breath —
  // 127u of non-list chrome became 139u, and the constant grew by the same 12.
  assert.equal(RESULTS_VIEWPORT_CHROME, 232);
});

test("the list still scrolls past the budget, so extra rows are reachable", async () => {
  const launcher = stripCssComments(await read("src/styles/launcher.css"));
  const results = /\.launcher-results\s*\{([^}]*)\}/s.exec(launcher);
  assert.ok(results);
  // A ceiling without scrolling would clip a drop group or a section title.
  assert.match(results[1], /overflow-y:\s*auto/);
  assert.match(results[1], /overscroll-behavior:\s*contain/);
});
