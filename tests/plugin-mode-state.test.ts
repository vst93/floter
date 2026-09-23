// R31 · the plugin mode is explicit state, and Esc / Cmd+W leave it.
//
// R26/R27 let the mode word live in the search field: `browser ` entered the
// browser mode and the parser read the mode back out of the query on every
// keystroke. That made the trigger word part of what the user saw, so a box that
// was *already* the browser's search field also printed `browser` in front of
// the query, beside a scope glyph saying the same thing (「既然概念上是已经进入插件
// 了，输入框左侧只保留插件信息就行了，browser 这段就可以不用了」).
//
// This round lifts the mode out of the text: entering strips the trigger word,
// the field shows only the needle, and leaving (Esc / Cmd+W) puts the needle
// back as ordinary text. The pure transitions are imported and exercised here;
// the App/hook wiring is pinned at the source, the same arrangement the rest of
// the launcher suite uses.
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";
import {
  browserModeFor,
  clipboardModeFor,
  pluginModeEntry,
} from "../src/launcher.ts";

const root = new URL("../", import.meta.url);
const read = (path: string) => readFile(new URL(path, root), "utf8");
const stripJsComments = (src: string) =>
  src.replace(/\/\*[\s\S]*?\*\//g, "").replace(/\/\/[^\n]*/g, "");

// ── A · the transition into a mode ────────────────────────────────────────

test("a trigger word plus a space enters the mode and keeps only the needle", () => {
  assert.deepEqual(pluginModeEntry("browser "), {
    mode: { scope: "browser", kind: "all" },
    needle: "",
  });
  assert.deepEqual(pluginModeEntry("browser rust async"), {
    mode: { scope: "browser", kind: "all" },
    needle: "rust async",
  });
  assert.deepEqual(pluginModeEntry("bookmarks rust"), {
    mode: { scope: "browser", kind: "bookmarks" },
    needle: "rust",
  });
  assert.deepEqual(pluginModeEntry("history rust"), {
    mode: { scope: "browser", kind: "history" },
    needle: "rust",
  });
  assert.deepEqual(pluginModeEntry("clip "), {
    mode: { scope: "clipboard", filter: "all" },
    needle: "",
  });
  assert.deepEqual(pluginModeEntry("clip  rust  "), {
    mode: { scope: "clipboard", filter: "all" },
    needle: "rust",
  });
  // The Chinese triggers enter the same two modes.
  assert.deepEqual(pluginModeEntry("书签 文档"), {
    mode: { scope: "browser", kind: "bookmarks" },
    needle: "文档",
  });
  assert.deepEqual(pluginModeEntry("剪贴板 "), {
    mode: { scope: "clipboard", filter: "all" },
    needle: "",
  });
});

test("the bare word is never an entry — the space is what makes it deliberate", () => {
  // `history` is the shell's own command, `clip` is a real Windows command, and
  // `bookmarks` is the system row the user Enters. None of them may enter a mode
  // on their own.
  for (const value of [
    "",
    "browser",
    "bookmarks",
    "bookmark",
    "history",
    "hist",
    "clip",
    "clipboard",
    "剪贴板",
    "粘贴板",
    "浏览器",
    "书签",
    "git commit",
    "bookmarklet rust",
    "chrome",
  ]) {
    assert.equal(pluginModeEntry(value), null, `"${value}" must not enter a mode`);
  }
});

// ── B · the requests the catalog hook reads ───────────────────────────────

test("an active mode + the field's text is the request the hook fetches", () => {
  assert.deepEqual(browserModeFor({ scope: "browser", kind: "history" }, "  rust  "), {
    kind: "history",
    needle: "rust",
  });
  assert.equal(browserModeFor({ scope: "clipboard", filter: "all" }, "rust"), null);
  assert.equal(browserModeFor(null, "rust"), null);

  assert.deepEqual(clipboardModeFor({ scope: "clipboard", filter: "all" }, "  rust  "), {
    needle: "rust",
    filter: "all",
  });
  assert.equal(clipboardModeFor({ scope: "browser", kind: "all" }, "rust"), null);
  assert.equal(clipboardModeFor(null, "rust"), null);
});

// ── C · the wiring, pinned at the source ──────────────────────────────────

test("the App holds the mode as state and strips the trigger on entry", async () => {
  const app = stripJsComments(await read("src/App.tsx"));
  // One piece of state owns the mode.
  assert.match(
    app,
    /const \[pluginMode, setPluginMode\] = useState<ActivePluginMode \| null>\(null\)/,
    "the mode is explicit state, not read back out of the query",
  );
  // The scope comes off that state, and the hook is handed resolved requests.
  assert.match(app, /const launcherScope = pluginMode\?\.scope \?\? null;/);
  assert.match(app, /const browserMode = useMemo\(\(\) => browserModeFor\(pluginMode, query\), \[pluginMode, query\]\);/);
  assert.match(app, /const clipboardMode = useMemo\(\(\) => clipboardModeFor\(pluginMode, query\), \[pluginMode, query\]\);/);
  assert.match(app, /browserMode,\s*clipboardMode,/, "the resolved requests are handed to the hook");
  // Entering from the field strips the word: the input's value stays `query`,
  // and the change handler sets the mode + the *needle*, never the whole text.
  assert.match(app, /const entry = pluginModeEntry\(value\);/);
  assert.match(
    app,
    /setPluginMode\(entry\.mode\);\s*setQuery\(entry\.needle\);/,
    "the trigger word is dropped and only the needle survives",
  );
  // Backspacing to empty must NOT leave the mode: the handler returns before the
  // plain `setQuery(value)` only for a non-null entry, and an empty field is no
  // entry, so the mode stays put.
  assert.match(app, /if \(pluginModeRef\.current === null\) \{\s*const entry = pluginModeEntry\(value\);/);
});

test("the catalog hook no longer parses a mode word out of the query", async () => {
  const hook = stripJsComments(await read("src/hooks/useLauncherCatalog.ts"));
  assert.doesNotMatch(hook, /parseBrowserMode\(query\)/);
  assert.doesNotMatch(hook, /parseClipboardMode\(query\)/);
  // It reads the resolved requests instead.
  assert.match(hook, /browserMode: BrowserMode \| null;/);
  assert.match(hook, /clipboardMode: ClipboardMode \| null;/);
  assert.match(hook, /const \{[\s\S]*?browserMode,\s*clipboardMode,/);
});

test("the browser system row enters the mode instead of rewriting the query", async () => {
  const actions = stripJsComments(await read("src/hooks/useLauncherActions.ts"));
  assert.match(actions, /enterPluginMode: \(mode: ActivePluginMode\) => void;/);
  assert.match(actions, /enterPluginMode\(\{ scope: "browser", kind: "all" \}\)/);
  assert.doesNotMatch(actions, /setQuery\("browser "\)/);
});

test("a programmatic reset leaves the mode, but a backspace to empty does not", async () => {
  const app = stripJsComments(await read("src/App.tsx"));
  // The setter the hooks receive clears the mode on a literal empty string…
  assert.match(
    app,
    /const setQueryExitingPlugin = useCallback<React\.Dispatch<React\.SetStateAction<string>>>\(/,
  );
  assert.match(app, /if \(action === ""\) \{\s*setPluginMode\(null\);\s*setQuery\(""\);/);
  // …and the App's own summon/return resets go through it.
  assert.match(app, /setQueryExitingPlugin\(""\)/);
  assert.doesNotMatch(app, /setQuery\(""\);\n    setTerminalMounted/);
  // The keyboard fallback's function updaters are deliberately not run through
  // the entry branch, so backspacing the needle to empty keeps the mode.
  const keyboard = stripJsComments(await read("src/hooks/useAppKeyboard.ts"));
  assert.match(keyboard, /setQuery\(\(current\) => current\.slice\(0, -1\)\)/);
});

// ── D · Esc / Cmd+W, in the user's three-level order ──────────────────────

test("the collapsed dismiss rule is config overlay, then mode, then window", async () => {
  const app = stripJsComments(await read("src/App.tsx"));
  const start = app.indexOf("const onLauncherDismiss");
  assert.ok(start > -1, "the App must declare the mode-aware dismiss rule");
  const rule = app.slice(start, app.indexOf("// The card is mounted", start));
  // The three levels appear in order.
  const overlay = rule.indexOf("if (pluginConfigOpen)");
  const mode = rule.indexOf("if (pluginModeRef.current)");
  const window_ = rule.indexOf('if (modW)');
  assert.ok(overlay > -1 && mode > -1 && window_ > -1, "all three levels are present");
  assert.ok(overlay < mode && mode < window_, "the overlay closes first, the mode second, the window last");
  // The overlay close and the mode exit both consume the press.
  assert.match(rule, /if \(pluginConfigOpen\) \{\s*event\.preventDefault\(\);\s*setPluginConfigOpen\(false\);/);
  assert.match(rule, /if \(pluginModeRef\.current\) \{\s*event\.preventDefault\(\);\s*exitPluginMode\(\);/);
  // Cmd+W is intercepted: preventDefault, then hide the native window.
  assert.match(rule, /if \(modW\) \{\s*event\.preventDefault\(\);\s*invoke\("hide_window"\);/);
});

test("the mode-aware rule runs before the dismiss table on both paths", async () => {
  const app = stripJsComments(await read("src/App.tsx"));
  assert.match(
    app,
    /const onInputKeyDown = \(event: React\.KeyboardEvent<HTMLInputElement>\) => \{[\s\S]{0,400}?if \(onLauncherDismiss\(event\.nativeEvent\)\) return;[\s\S]{0,200}?handleLauncherKey\(event\.nativeEvent\);/,
    "the input's own handler asks the mode-aware rule first",
  );
  const keyboard = stripJsComments(await read("src/hooks/useAppKeyboard.ts"));
  assert.match(
    keyboard,
    /const dismiss = resolveDismissRule\("collapsed", event, shortcuts\);[\s\S]{0,260}?if \(onLauncherDismiss\(event\)\) return;/,
    "the window-level fallback asks the same rule before the table acts",
  );
});

test("leaving a mode keeps the needle as ordinary text", async () => {
  const app = stripJsComments(await read("src/App.tsx"));
  const start = app.indexOf("const exitPluginMode");
  const body = app.slice(start, app.indexOf("const onLauncherDismiss", start));
  assert.match(body, /setPluginMode\(null\)/);
  assert.doesNotMatch(body, /setQuery\(/, "the field's text is deliberately kept, not cleared");
});
