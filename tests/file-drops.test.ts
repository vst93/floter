// R7-10a · a dropped file becomes launcher *results*, never an execution.
//
// The reviewer's brief, verbatim: "把拖入主窗口的文件生成为 launcher 结果（非执行）:
// 每个文件给三条动作——「打开」、「在终端中 cd」、「复制路径」". The two red
// lines that carry the whole round are:
//
//   1. the drop never runs anything — a file row's arrival must not open, `cd`
//      or copy, and the three actions run only from an explicit Enter/click on
//      the action bar;
//   2. only the main window's launcher accepts a drop — a terminal or settings
//      mode ignores it.
//
// The pure module (`src/launcher/file-drops.ts`) is driven directly for the
// result rows, the truncation and the three actions. The two mutation locks at
// the bottom prove the guards are load-bearing: deleting either one turns a
// predicate red. The Rust half of guard 2 (`commands/drops.rs`) has its own
// unit test and is pinned here as source text.
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";
import { createTranslator, isMessageKey } from "../src/i18n.ts";
import {
  acceptsFileDrop,
  actionBarKindFor,
  activeFileDropAction,
  actionValue,
  cdCommandForPath,
  cdCommandLine,
  clampActionIndex,
  directoryForCd,
  FILE_DROP_ACTIONS,
  fileActionKindForBar,
  fileActionRequest,
  fileDropActionBar,
  fileDropRows,
  MAIN_WINDOW_LABEL,
  MAX_VISIBLE_DROPPED_FILES,
  nextFileActionIndex,
  selectedDroppedFile,
  shellQuote,
  type DroppedFile,
} from "../src/launcher/file-drops.ts";

const root = new URL("../", import.meta.url);
const read = (path: string) => readFile(new URL(path, root), "utf8");
const stripJsComments = (source: string) =>
  source.replace(/\/\*[\s\S]*?\*\//g, "").replace(/^\s*\/\/.*$/gm, "");
const t = createTranslator("en");

const file = (over: Partial<DroppedFile> = {}): DroppedFile => ({
  path: "/home/u/notes.txt",
  name: "notes.txt",
  directory: "/home/u",
  isDirectory: false,
  ...over,
});

const files = (count: number): DroppedFile[] =>
  Array.from({ length: count }, (_, index) =>
    file({
      path: `/home/u/file-${index}.txt`,
      name: `file-${index}.txt`,
    }),
  );

/** Brace-match the body of a block starting at `start` (index of its `{`). */
const blockAt = (source: string, start: number): string => {
  let depth = 0;
  for (let index = start; index < source.length; index += 1) {
    if (source[index] === "{") depth += 1;
    else if (source[index] === "}") {
      depth -= 1;
      if (depth === 0) return source.slice(start, index + 1);
    }
  }
  assert.fail("unterminated block");
};

/** The `if (item.type === "file") { … }` branch of `runLauncherItem`. */
const fileRowBranch = (source: string): string => {
  const at = source.indexOf('if (item.type === "file") {');
  assert.notEqual(at, -1, "runLauncherItem must handle a dropped file row");
  return blockAt(source, source.indexOf("{", at));
};

// ── A · the three actions ─────────────────────────────────────────────────

test("a dropped file carries exactly the three safe actions, in order", () => {
  assert.deepEqual(
    FILE_DROP_ACTIONS.map((action) => action.kind),
    ["open", "cd", "copy-path"],
  );
  // Open leads: it is what a double-click does and so what a drop most often
  // means. `cd` and copy-path are the two things a launcher adds.
  for (const action of FILE_DROP_ACTIONS) {
    assert.ok(isMessageKey(action.labelKey), `${action.labelKey} must be in the dictionary`);
    const en = t(action.labelKey);
    const zh = createTranslator("zh")(action.labelKey);
    assert.ok(en.length > 0 && zh.length > 0, `${action.labelKey} must be bilingual`);
    assert.notEqual(zh, en, `${action.labelKey} must be translated, not an English fallback`);
    assert.ok(/[\u4e00-\u9fff]/.test(zh), `${action.labelKey} zh must be Chinese`);
  }
  // The action bar's kind is derived from the action, one-to-one and onto the
  // three file kinds — no fourth shape can be invented silently.
  assert.deepEqual(FILE_DROP_ACTIONS.map((a) => actionBarKindFor(a.kind)), [
    "file-open",
    "file-cd",
    "file-copy",
  ]);
  for (const kind of ["file-open", "file-cd", "file-copy"] as const) {
    assert.equal(actionBarKindFor(fileActionKindForBar(kind)), kind, "the mapping round-trips");
  }
});

test("each action builds a request that describes what runs", () => {
  const target = file();
  assert.deepEqual(fileActionRequest("open", target.path), {
    kind: "open",
    path: "/home/u/notes.txt",
  });
  assert.deepEqual(fileActionRequest("copy-path", target.path), {
    kind: "copy",
    path: "/home/u/notes.txt",
  });
  assert.deepEqual(fileActionRequest("cd", directoryForCd(target)), {
    kind: "cd",
    commandLine: "cd /home/u",
  });
  // A folder `cd`s into itself, not into its parent.
  const folder = file({ path: "/home/u/Project", name: "Project", directory: "/home/u", isDirectory: true });
  assert.equal(directoryForCd(folder), "/home/u/Project");
  assert.equal(actionValue(folder, "cd"), "/home/u/Project");
  assert.equal(actionValue(folder, "open"), "/home/u/Project");
});

// ── B · the shell quoting a `cd` needs ────────────────────────────────────

test("a cd command is quoted only when the path needs it", () => {
  assert.equal(cdCommandForPath("/home/u"), "cd /home/u");
  assert.equal(cdCommandForPath("/Users/Jane Doe"), "cd '/Users/Jane Doe'");
  assert.equal(cdCommandForPath("/home/o'brien"), "cd '/home/o'\\''brien'");
  // Windows `cmd` has no single-quote escape, so it doubles the quotes away.
  assert.equal(cdCommandForPath("C:\\Users\\Jane", true), "cd C:\\Users\\Jane");
  assert.equal(cdCommandForPath("C:\\Program Files", true), 'cd "C:\\Program Files"');
  // The same value the action bar shows is the one that is quoted.
  assert.equal(cdCommandLine(file({ directory: "/home/Jane Doe" })), "cd '/home/Jane Doe'");
  assert.equal(shellQuote("plain"), "plain");
  assert.equal(shellQuote("a b"), "'a b'");
});

test("the action switcher wraps at both ends", () => {
  assert.equal(activeFileDropAction(0).kind, "open");
  assert.equal(activeFileDropAction(1).kind, "cd");
  assert.equal(activeFileDropAction(2).kind, "copy-path");
  assert.equal(nextFileActionIndex(2, 1), 0, "past the last wraps to the first");
  assert.equal(nextFileActionIndex(0, -1), 2, "before the first wraps to the last");
  assert.equal(clampActionIndex(7), 1);
  assert.equal(clampActionIndex(-1), 2);
  assert.equal(clampActionIndex(Number.NaN), 0);
});

test("the action bar switches between a file's three actions", () => {
  const target = file({ directory: "/home/u" });
  const open = fileDropActionBar(target, 0, t);
  const cd = fileDropActionBar(target, 1, t);
  const copy = fileDropActionBar(target, 2, t);
  assert.equal(open.type, "file-open");
  assert.equal(cd.type, "file-cd");
  assert.equal(copy.type, "file-copy");
  // The value line is what the action acts on: the path to open, the directory
  // to enter, the path to copy.
  assert.equal(open.value, "/home/u/notes.txt");
  assert.equal(cd.value, "/home/u");
  assert.equal(copy.value, "/home/u/notes.txt");
  for (const bar of [open, cd, copy]) {
    assert.ok(bar.label.length > 0, "the action bar is labelled");
  }
});

// ── C · the result rows and the >5 truncation ─────────────────────────────

test("no drop, no rows", () => {
  assert.deepEqual(fileDropRows([], false, t), []);
  assert.deepEqual(fileDropRows([], true, t), []);
});

test("a dropped file becomes one row carrying its normalized description", () => {
  const target = file();
  const rows = fileDropRows([target], false, t);
  assert.equal(rows.length, 1);
  const row = rows[0];
  assert.equal(row.type, "file");
  assert.equal(row.id, "drop:/home/u/notes.txt", "the path is the identity");
  assert.equal(row.title, "notes.txt");
  assert.equal(row.subtitle, "/home/u");
  assert.deepEqual(row.file, target, "the row carries the file the actions act on");
  // A folder is a file too, and only `isDirectory` differs.
  const folderRow = fileDropRows([file({ isDirectory: true })], false, t)[0];
  assert.equal(folderRow.type, "file");
  assert.equal(folderRow.file.isDirectory, true);
});

test("five or fewer files are listed in full, with no expander", () => {
  for (const count of [1, 3, 5]) {
    const rows = fileDropRows(files(count), false, t);
    assert.equal(rows.length, count, `${count} files must all be listed`);
    assert.ok(rows.every((row) => row.type === "file"));
    // Order is the drop order.
    assert.deepEqual(
      rows.map((row) => row.title),
      files(count).map((f) => f.name),
    );
  }
});

test("more than five files collapse to five plus '…and N more'", () => {
  const eight = files(8);
  const collapsed = fileDropRows(eight, false, t);
  assert.equal(collapsed.length, MAX_VISIBLE_DROPPED_FILES + 1, "five rows plus the expander");
  assert.deepEqual(
    collapsed.slice(0, MAX_VISIBLE_DROPPED_FILES).map((row) => row.title),
    eight.slice(0, MAX_VISIBLE_DROPPED_FILES).map((f) => f.name),
  );
  const expander = collapsed[collapsed.length - 1];
  assert.equal(expander.type, "file-more");
  assert.equal(expander.hidden, 3, "8 − 5 = 3");
  assert.match(expander.title, /3/, "the row names the count");
  assert.ok(expander.subtitle.length > 0, "and explains itself");
  // The expander is not a file: it has no `file` and can never reach one.
  assert.equal("file" in expander, false);

  // Expanding lists them all and drops the expander.
  const expanded = fileDropRows(eight, true, t);
  assert.equal(expanded.length, 8);
  assert.ok(expanded.every((row) => row.type === "file"));
});

test("the expander count is the real remainder, not a constant", () => {
  for (const count of [6, 7, 12]) {
    const rows = fileDropRows(files(count), false, t);
    assert.equal(rows[rows.length - 1].hidden, count - MAX_VISIBLE_DROPPED_FILES);
  }
});

// ── D · the selection helpers ─────────────────────────────────────────────

test("the dropped file is resolved off the row, not off the index", () => {
  const rows = [
    { type: "command" as const, id: "c", title: "c", subtitle: "", warnings: [], sourceName: "s", commandLine: "c", execution: null, completion: false },
    ...fileDropRows(files(2), false, t),
    { type: "file-more" as const, id: "drop:more", hidden: 9, title: "more", subtitle: "" },
  ];
  assert.equal(selectedDroppedFile(rows, 0), null, "a non-file row has no file");
  assert.equal(selectedDroppedFile(rows, 1)?.name, "file-0.txt");
  assert.equal(selectedDroppedFile(rows, 2)?.name, "file-1.txt");
  assert.equal(selectedDroppedFile(rows, 3), null, "the expander is not a file");
  assert.equal(selectedDroppedFile(rows, 99), null, "out of range is null");
});

// ── E · the main-window guard ─────────────────────────────────────────────

test("only the main window's collapsed launcher accepts a drop", () => {
  assert.equal(MAIN_WINDOW_LABEL, "main");
  assert.equal(acceptsFileDrop("main", "collapsed"), true);
  // Terminal and settings are modes of the same window, but not a launcher.
  assert.equal(acceptsFileDrop("main", "terminal"), false);
  assert.equal(acceptsFileDrop("main", "settings"), false);
  assert.equal(acceptsFileDrop("main", "plugin"), false);
  // A second window (any other label) is ignored whatever it is doing.
  for (const label of ["terminal", "settings", "pinned", ""]) {
    assert.equal(acceptsFileDrop(label, "collapsed"), false, `${label} must be ignored`);
  }
});

// ── F · wiring across the boundary ────────────────────────────────────────

test("the drop listener subscribes to the drag-drop event on the webview", async () => {
  const hook = stripJsComments(await read("src/hooks/useFileDrops.ts"));
  assert.match(hook, /getCurrentWebview\(\)\s*\.onDragDropEvent/, "the host event is the source");
  assert.match(hook, /payload\.type !== "drop"/, "hover traffic is not a drop");
  assert.match(hook, /acceptsFileDrop\(label, modeRef\.current\)/, "the window guard is consulted");
  assert.match(hook, /invoke<DroppedFile\[\]>\("resolve_dropped_files"/, "the backend describes the paths");
  // The listener is removed on unmount.
  assert.match(hook, /return \(\) => \{[\s\S]*unlisten\?\.\(\)/, "the listener is disposed");
});

test("the drop is composed into the result list in the App", async () => {
  const app = stripJsComments(await read("src/App.tsx"));
  assert.match(app, /useFileDrops\(\{/, "the listener hook is mounted");
  // The rows are prepended, so a drop is a source group at the top.
  assert.match(app, /\[\.\.\.fileRows, \.\.\.launcherResults\]/, "the drop leads the list");
  // The composed list — not the query list — is what the renderer, the
  // numbered shortcuts and the arrow loop see.
  assert.match(app, /results=\{displayedResults\}/, "the renderer gets the composed list");
  assert.match(app, /resultShortcutSlots: displayedShortcutSlots/, "the shortcut slots follow it");
  assert.match(app, /launcherResults: displayedResults/, "the key handler follows it");
  // The bar describes the selected file's action even with an empty query.
  assert.match(app, /fileDropActionBar\(selectedDroppedFile, fileActionIndex, t\)/, "the bar is the file's");
  // The drop's rows are prepended, so the query-relative default selection is
  // shifted past them: typing after a drop must land on the matched result, not
  // on the file the drop left at the top.
  assert.match(app, /dropped \+ firstRunnableResultIndex/, "the default selection clears the drop rows");
  // A reveal clears the drop through the existing clear path.
  assert.match(app, /clearDrops\(\)/, "a summon starts with a clean launcher");
});

test("a dropped file's row is rendered, and a folder has its own glyph", async () => {
  const results = stripJsComments(await read("src/launcher/LauncherResults.tsx"));
  assert.match(results, /item\.type === "file" \?/, "a file row is a render branch");
  assert.match(results, /item\.file\.isDirectory \? <FolderIcon \/> : <FileIcon \/>/, "folder vs file");
  assert.match(results, /item\.type === "file-more" \?/, "the expander is rendered too");
  // The drop group gets its own heading, reusing the existing section title.
  assert.match(results, /const filesSectionStartsHere =/, "the drop has a section");
  assert.match(results, /t\("launcher\.files"\)/, "the section is the Files group");
});

// The visual feedback is the existing result row, not a new surface. The round
// adds no toast, no overlay and no modal: the drop path is silent.
test("the drop adds no toast, overlay or modal", async () => {
  const hook = stripJsComments(await read("src/hooks/useFileDrops.ts"));
  for (const forbidden of ["notify", "appendToast", "toast", "alert(", "createPortal", "aria-modal"]) {
    assert.ok(
      !hook.includes(forbidden),
      `useFileDrops must not raise ${forbidden} — the drop is shown as result rows`,
    );
  }
  const results = stripJsComments(await read("src/launcher/LauncherResults.tsx"));
  assert.ok(!/launcher-file-drop|drop-overlay|drop-zone/.test(results), "no new visual surface");
});

test("the new rows reuse the existing launcher row styles", async () => {
  const css = stripJsComments(await read("src/styles/launcher.css"));
  // No file-drop-specific rule was added: the rows ride `.launcher-result`, the
  // heading rides `.launcher-section-title` and the bar rides
  // `.launcher-action-bar`. A private sheet would be a second styling family.
  assert.ok(
    !/file-drop|dropped-file|launcher-result--file/.test(css),
    "the drop rows must reuse the existing row family, not add one",
  );
  for (const selector of [".launcher-result", ".launcher-section-title", ".launcher-action-bar"]) {
    assert.ok(css.includes(`${selector} {`), `${selector} must stay the shared row style`);
  }
});

// ── G · mutation locks (red lines 1 and 2) ────────────────────────────────

// Red line 1, mutation A: a dropped file's *row* answers with the action bar.
// The mutation is "the row runs its default action directly" — the exact thing
// the round forbids. The predicate reads the branch's own body, so injecting an
// execution call into it turns the test red.
test("mutation lock: a file row never runs anything on arrival", async () => {
  const source = stripJsComments(await read("src/hooks/useLauncherActions.ts"));
  const noExecution = (body: string) =>
    !/\binvoke\(|runCommand\(|runFileAction\(|runFileActionBar\(|executeActionBar\(|openWithSystem\(|clipboard_write_text/.test(body);

  assert.ok(noExecution(fileRowBranch(source)), "selecting a file row must only show its actions");

  // The mutation: the default action runs the moment the row is chosen.
  const mutated = source.replace(
    fileRowBranch(source),
    '{ void invoke("open_path", { path: item.file.path }); return; }',
  );
  assert.notEqual(mutated, source, "the mutation must land");
  assert.equal(
    noExecution(fileRowBranch(mutated)),
    false,
    "the predicate must reject a row that runs its default action",
  );
});

// Red line 1, mutation B: the whole listener. Nothing on the drop path may
// execute, and the only command it may call is the one that *describes* the
// paths. A second `invoke` in the handler is a second side effect.
test("mutation lock: the drop listener only ever describes the paths", async () => {
  const source = stripJsComments(await read("src/hooks/useFileDrops.ts"));
  const invokedCommands = (body: string) =>
    [...body.matchAll(/invoke<[^>]*>\(\s*"([^"]+)"/g)].map((match) => match[1]);
  const dropBody = source.slice(source.indexOf("onDragDropEvent(("), source.indexOf("  /** Drop the dropped rows"));
  assert.deepEqual(
    invokedCommands(dropBody),
    ["resolve_dropped_files"],
    "the drop handler may call exactly one command, and it is a read",
  );

  const mutated = source.replace("resolve_dropped_files", "open_path");
  assert.notEqual(mutated, source, "the mutation must land");
  const mutatedBody = mutated.slice(mutated.indexOf("onDragDropEvent(("), mutated.indexOf("  /** Drop the dropped rows"));
  assert.notDeepEqual(
    invokedCommands(mutatedBody),
    ["resolve_dropped_files"],
    "the predicate must reject a drop that opens a file",
  );
});

// Red line 2, mutation C: the main-window label check. The mutation removes it,
// which would let a terminal or settings surface answer a drop.
test("mutation lock: dropping the main-window label check goes red", async () => {
  const source = stripJsComments(await read("src/launcher/file-drops.ts"));
  const guardsLabel = (text: string) =>
    /windowLabel === MAIN_WINDOW_LABEL/.test(text) && /export const acceptsFileDrop/.test(text);
  assert.ok(guardsLabel(source), "the listener's gate must compare the window label");

  const mutated = source.replace("windowLabel === MAIN_WINDOW_LABEL && ", "");
  assert.notEqual(mutated, source, "the mutation must land");
  assert.equal(guardsLabel(mutated), false, "the predicate must reject a label-blind gate");
});

// Red line 2, Rust half: the label guard also lives on the payload, so a
// command call from anywhere cannot describe a drop for another window.
test("mutation lock: the backend refuses a non-main window", async () => {
  const source = stripJsComments(await read("src-tauri/src/commands/drops.rs"));
  const production = source.slice(0, source.indexOf("#[cfg(test)]"));
  assert.match(
    production,
    /if window_label != MAIN_WINDOW_LABEL \{\s*return Err\(/,
    "the command answers a foreign label with an error, not a description",
  );
  // The Rust unit test drives it; this pins that the guard is not behind a
  // feature flag or a debug assertion.
  assert.match(source, /fn only_the_main_window_may_resolve_a_drop/, "the refusal is unit-tested");
  // And the main window label is declared once on each side of the boundary.
  assert.match(production, /pub const MAIN_WINDOW_LABEL: &str = "main";/, "the Rust label");
  const ts = stripJsComments(await read("src/launcher/file-drops.ts"));
  assert.match(ts, /export const MAIN_WINDOW_LABEL = "main";/, "the TS label");
});

// The command is registered and mounted, and it needs no capability: it is a
// plain `#[tauri::command]`, and the drag-drop event rides `core:default`.
test("the command is mounted and adds no capability", async () => {
  const lib = stripJsComments(await read("src-tauri/src/lib.rs"));
  assert.match(lib, /resolve_dropped_files,/, "the command is registered");
  assert.match(
    stripJsComments(await read("src-tauri/src/commands/mod.rs")),
    /pub mod drops;/,
    "the module is mounted",
  );
  const capability = JSON.parse(await read("src-tauri/capabilities/default.json")) as {
    permissions: string[];
  };
  // No `fs:default` bundle and no fs plugin permission was added for the drop:
  // the paths arrive on the host event and are described in Rust.
  assert.ok(
    !capability.permissions.some((permission) => permission.startsWith("fs:")),
    "the round must not add an fs capability",
  );
  assert.ok(
    !capability.permissions.includes("fs:default"),
    "no fs:default gift",
  );
});
