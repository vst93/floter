// R97 · the robustness/integration sweep's frontend and bridge guards.
//
// Three independent S-level items land here:
//
//   1. the launcher's session thumbnail memo is bounded (task 3) — the pure
//      pruner plus the wiring that calls it;
//   2. the switched-off clipboard entry's note carries the same "enable it in
//      Settings" guidance the browser note does (task 4);
//   3. the six `clipboardPage.*` keys orphaned by the schema-driven config
//      overlay are gone from both dictionaries (task 5).
//
// The Rust half of the sweep (the poisoned-lock cascade and the `cwd` bypass)
// is pinned by `src-tauri` unit tests; the command's no-`cwd` shape is also
// pinned here because that is the layer the parameter was removed from.
//
// Mutations that must turn this file red:
//   * `pruneThumbnailCache` removed (memo grows again) -> the pure tests and
//     the LauncherResults source pin fail;
//   * `launcher.clipboardDisabledRow` losing its Settings guidance -> the note
//     test fails;
//   * any `clipboardPage.*` key returning -> the dead-key test fails;
//   * `cwd` added back to `external_plugin_run` -> the signature pin fails.

import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

import { MAX_CLIPBOARD_MAX_ITEMS, pruneThumbnailCache } from "../src/clipboard-history.ts";
import { createTranslator } from "../src/i18n.ts";

const root = new URL("../", import.meta.url);
const read = (path: string) => readFile(new URL(path, root), "utf8");
const stripJsComments = (source: string) =>
  source.replace(/\/\*[\s\S]*?\*\//g, "").replace(/\/\/[^\n]*/g, "");

const en = createTranslator("en");
const zh = createTranslator("zh");

// ── 1 · the thumbnail memo is bounded ─────────────────────────────────────

test("a memo at or under the ceiling is returned unchanged", () => {
  const memo = { a: "data:a", b: "data:b" };
  assert.equal(pruneThumbnailCache(memo, ["a"]), memo, "no copy under the ceiling");
  assert.equal(
    pruneThumbnailCache(memo, ["a"], 2),
    memo,
    "the ceiling is inclusive: exactly maxItems is still unchanged",
  );
});

test("past the ceiling only the keys this render references survive", () => {
  const memo: Record<string, string> = {};
  for (let i = 0; i < MAX_CLIPBOARD_MAX_ITEMS + 5; i += 1) memo[`id-${i}`] = `data:${i}`;
  const visible = ["id-3", "id-9"];
  const pruned = pruneThumbnailCache(memo, visible);
  assert.deepEqual(Object.keys(pruned).sort(), ["id-3", "id-9"]);
  assert.equal(pruned["id-3"], "data:3");
  assert.equal(pruned["id-9"], "data:9");
  assert.equal(Object.keys(memo).length, MAX_CLIPBOARD_MAX_ITEMS + 5, "the input is untouched");
});

test("the memo is capped against the history's own ceiling", () => {
  assert.equal(MAX_CLIPBOARD_MAX_ITEMS, 500);
  // The default ceiling is the history constant, not an ad-hoc number.
  const memo: Record<string, string> = {};
  for (let i = 0; i < MAX_CLIPBOARD_MAX_ITEMS; i += 1) memo[`id-${i}`] = "x";
  assert.equal(pruneThumbnailCache(memo, []), memo, "at the ceiling: unchanged");
  memo["one-more"] = "x";
  assert.deepEqual(pruneThumbnailCache(memo, []), {}, "one past it: pruned to the visible set");
});

test("LauncherResults prunes through the shared pure function", async () => {
  const source = stripJsComments(await read("src/launcher/LauncherResults.tsx"));
  assert.match(source, /import \{ pruneThumbnailCache \}/, "the pruner is imported");
  assert.match(
    source,
    /setClipboardThumbnails\(\(previous\) =>\s*\n?\s*pruneThumbnailCache\(\{ \.\.\.previous, \.\.\.next \}, visibleImageIds\)/,
    "the merge goes through the pruner against the visible set",
  );
});

// ── 2 · the switched-off clipboard note points at Settings ────────────────

test("both disabled notes name Settings in both dictionaries", () => {
  for (const [language, t] of [
    ["en", en],
    ["zh", zh],
  ] as const) {
    const browser = t("launcher.browserDisabledRow", { name: "X" });
    const clipboard = t("launcher.clipboardDisabledRow", { name: "X" });
    assert.match(browser, /Settings|设置/, `${language}: the browser note names Settings`);
    assert.match(clipboard, /Settings|设置/, `${language}: the clipboard note names Settings`);
  }
  assert.match(
    en("launcher.clipboardDisabledRow", { name: "X" }),
    /enable it in Settings/,
    "the clipboard note carries the browser note's guidance verbatim",
  );
  assert.match(zh("launcher.clipboardDisabledRow", { name: "X" }), /可在设置中启用/);
});

// ── 3 · the dead clipboardPage.* keys stay gone ───────────────────────────

test("the six clipboardPage.* keys are gone from both dictionaries", async () => {
  const i18n = stripJsComments(await read("src/i18n.ts"));
  for (const key of [
    "clipboardPage.settings",
    "clipboardPage.settingsSaved",
    "clipboardPage.settingsFailed",
    "clipboardPage.maxItems",
    "clipboardPage.maxItemsHint",
    "clipboardPage.maxItemsValue",
  ]) {
    assert.equal(i18n.split(`"${key}"`).length - 1, 0, `${key} must be removed`);
  }
  assert.equal(
    i18n.split("clipboardPage.").length - 1,
    0,
    "no clipboardPage.* key remains in either dictionary",
  );
});

test("no clipboardPage.* key is referenced anywhere in the frontend", async () => {
  const sources = await Promise.all(
    ["src/clipboard-history.ts", "src/plugins/PluginConfigOverlay.tsx"].map((path) => read(path)),
  );
  for (const source of sources) {
    assert.doesNotMatch(source, /clipboardPage\./, "a dead key must not survive as a reference");
  }
});

// ── 4 · the command takes no caller-supplied cwd ──────────────────────────

test("external_plugin_run takes no cwd", async () => {
  const source = stripJsComments(await read("src-tauri/src/commands/extensions.rs"));
  const start = source.indexOf("pub async fn external_plugin_run");
  assert.ok(start >= 0, "the command exists");
  const signature = source.slice(start, source.indexOf("-> Result", start));
  assert.doesNotMatch(signature, /cwd/, "the command must not accept a cwd");
  assert.match(
    source,
    /catalog::run_plugin_command\(&state, &extension_id, &command_id, args\)/,
    "the command calls run_plugin_command without a working directory",
  );
});

test("run_plugin_command builds its plan with no cwd override", async () => {
  const catalog = stripJsComments(await read("src-tauri/src/extensions/catalog.rs"));
  const start = catalog.indexOf("pub async fn run_plugin_command");
  assert.ok(start >= 0, "the catalog function exists");
  const signature = catalog.slice(start, catalog.indexOf("-> Result", start));
  assert.doesNotMatch(signature, /cwd/, "the catalog function must not accept a cwd");
  const body = catalog.slice(start, catalog.indexOf("\n}\n", start));
  assert.match(
    body,
    /execution_plan\(&provider\.descriptor, &provider\.invocation, argv, None\)/,
    "the plan is built with no cwd override",
  );
});
