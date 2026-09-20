// R10-A · The launcher's ten rows: nine matched results, then one fixed row
// that opens the clipboard history.
//
// The user's report, verbatim: "当下的搜索页面高度太低了，搜索的时候展示的项
// 太少，需要展示 9 项加一个底部的单独进入剪切板的项 一共 10 项，高度可以再稍微
// 调整一下看看，总之让它的宽高比例也要处于一个协调的状态".
//
// Four facts carry the round, and each one is asserted positively so that
// deleting the code (rather than the prose) is what turns the suite red:
//
//   1. the budget is ten, and the catalog's own ceiling is nine — the tenth slot
//      belongs to the fixed row, not to a match;
//   2. the tail row is a clipboard system row in every query state: empty,
//      matching, and matching nothing;
//   3. a query that already matched the clipboard command does not get a second
//      clipboard row;
//   4. the tail row is selectable and runnable, and the numbered `1`-`9` family
//      keeps numbering the matched rows exactly as it did before.
//
// The height half is pinned from the two sheets that derive it: a row is
// `calc(var(--u) * 42)` and the list's ceiling is ten of them plus the fixed
// scroll-edge/gap chrome.
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";
import { createTranslator } from "../src/i18n.ts";
import {
  CLIPBOARD_RESULT_ID,
  COMMAND_LIMIT_WITH_MATCHES,
  MAX_RESULTS,
  RESULTS_VIEWPORT_CHROME,
  clipboardResultRow,
  isClipboardResult,
  shortcutSlotsWithFixedTail,
  withClipboardResultRow,
} from "../src/launcher/result-budget.ts";
import { launcherShortcutSlots } from "../src/launcher.ts";
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
const catalogClipboardRow = (): LauncherItem => ({
  type: "system",
  id: "system-clipboard",
  title: t("system.clipboardHistory"),
  subtitle: t("system.clipboardHistorySubtitle"),
  action: "clipboard",
});

// ── 1 · the budget ────────────────────────────────────────────────────────

test("the launcher budget is ten rows: nine matches plus the clipboard row", () => {
  assert.equal(MAX_RESULTS, 10);
  assert.equal(COMMAND_LIMIT_WITH_MATCHES, 3, "three commands, six local matches");
});

test("the catalog itself never returns the tenth row", async () => {
  // The fixed row is the App's, not the catalog's: every slice in the hook is
  // `MAX_RESULTS - 1`. If one of them grew to `MAX_RESULTS` the tail row would
  // push the list to eleven.
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
    "the empty-query recents limit is nine too",
  );
  assert.ok(
    !/slice\(0, MAX_RESULTS\)/.test(hook) && !/MAX_RESULTS \*/.test(hook),
    "nothing in the catalog may hand out a tenth matched row",
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

// ── 4 · the row is selectable and runnable, and 1-9 is unchanged ──────────

test("the clipboard row is runnable and keeps the shortcut family at nine", () => {
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

  // The family is 1-9, so the tenth row's badge stays blank rather than
  // promising a `⌘10` that `matchesResultShortcut` can never produce.
  const slots = shortcutSlotsWithFixedTail(rows, runnableFlags);
  assert.deepEqual(slots, [1, 2, null]);
  assert.deepEqual(
    launcherShortcutSlots(runnableFlags),
    [1, 2, 3],
    "without the tail policy the row would claim a third number — the helper is load-bearing",
  );

  // Nine matched rows still number one through nine, in order.
  const nine = Array.from({ length: MAX_RESULTS - 1 }, (_, i) => app(`app-${i}`));
  const nineSlots = shortcutSlotsWithFixedTail(
    withClipboardResultRow(nine, t),
    nine.map(() => true).concat([true]),
  );
  assert.deepEqual(nineSlots.slice(0, 9), [1, 2, 3, 4, 5, 6, 7, 8, 9]);
  assert.equal(nineSlots[9], null, "the tenth row is the fixed one and carries no number");
});

// ── 5 · the height the ten rows need ──────────────────────────────────────

test("the results ceiling is ten rows plus the fixed chrome", async () => {
  const launcher = stripCssComments(await read("src/styles/launcher.css"));
  const resultRow = /\.launcher-result\s*\{[^}]*height:\s*calc\(var\(--u\)\s*\*\s*(\d+)\)/s.exec(
    launcher,
  );
  assert.ok(resultRow, ".launcher-result must still declare its height off --u");
  const rowHeight = Number(resultRow[1]);

  const results = /\.launcher-results\s*\{[^}]*max-height:\s*([^;]+);/s.exec(launcher);
  assert.ok(results, ".launcher-results must declare a max-height");
  const ceiling = results[1];
  // Ten rows, at the row's own height, plus the fixed chrome: the scroll-edge
  // reservation, the ten 1px gaps and the empty-query section title. None of
  // those scale with `--u`.
  assert.match(
    ceiling,
    new RegExp(`calc\\(var\\(--u\\)\\s*\\*\\s*${MAX_RESULTS * rowHeight}\\s*\\+\\s*(\\d+)px\\)`),
    `the ceiling must be ${MAX_RESULTS} x ${rowHeight}px + chrome, got "${ceiling}"`,
  );
  const chrome = Number(
    new RegExp(`calc\\(var\\(--u\\)\\s*\\*\\s*${MAX_RESULTS * rowHeight}\\s*\\+\\s*(\\d+)px\\)`)
      .exec(ceiling)![1],
  );
  // The chrome has to be at least the scroll-edge band (14px, see base.css)
  // plus the gaps around ten rows, and it has to leave room for a section
  // title — otherwise the empty-query state scrolls the clipboard row away.
  const band = Number(/\.launcher-results\s*\{[^}]*padding:\s*var\(--scroll-edge\)/s.test(launcher) ? 14 : 0);
  assert.ok(band > 0, "the scroller still reserves the scroll-edge band as padding");
  assert.ok(
    chrome >= band + MAX_RESULTS + 20,
    `chrome of ${chrome}px must cover the ${band}px band, ten 1px gaps and a title line`,
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
  assert.equal(RESULTS_VIEWPORT_CHROME, 220);
});

test("the list still scrolls past the budget, so extra rows are reachable", async () => {
  const launcher = stripCssComments(await read("src/styles/launcher.css"));
  const results = /\.launcher-results\s*\{([^}]*)\}/s.exec(launcher);
  assert.ok(results);
  // A ceiling without scrolling would clip a drop group or a section title.
  assert.match(results[1], /overflow-y:\s*auto/);
  assert.match(results[1], /overscroll-behavior:\s*contain/);
});
