// R90 · pin one selected result's text into the detached window.
//
// R84's pin left from the plugin *page*; R90 lets the user pin **one row**.
// The detached window already renders a block of text (`PluginTextView`), so a
// row only has to hand it a snapshot. Two things can silently go wrong and
// neither breaks a build:
//
//   * the entry rule widens — a browser row or a textless clipboard row grows a
//     pin button, and the window opens onto nothing;
//   * the `text` arm of the request is not validated — an empty title slips
//     through and the window is nameless.
//
// Both are pinned here: the row-type matrix against the pure function, and the
// request arms against the validator. The wiring (the button's markup, the
// detached view's branch) is pinned at the source, the way the other suites pin
// a page that owns a DOM the node runner cannot instantiate.

import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

import type { LocalApplication } from "../src/App.tsx";
import type { ClipboardEntry } from "../src/clipboard-history.ts";
import { createTranslator } from "../src/i18n.ts";
import type { LauncherItem } from "../src/launcher/LauncherResults.tsx";
import type { DroppedFile } from "../src/launcher/file-drops.ts";
import { pinTextApplies, pinTextFor } from "../src/plugin-window/pin-entry.ts";

const root = new URL("../", import.meta.url);
const read = (path: string) => readFile(new URL(path, root), "utf8");

const entry = (overrides: Partial<ClipboardEntry> = {}): ClipboardEntry => ({
  id: "clip-1",
  kind: "text",
  text: "copied note",
  hash: "hash-1",
  created_at: 1_700_000_000_000,
  favorite: false,
  ...overrides,
});

const app: LocalApplication = {
  name: "Terminal",
  path: "/usr/bin/terminal",
  initials: "t",
};

const file: DroppedFile = {
  path: "/tmp/report.pdf",
  name: "report.pdf",
  directory: "/tmp",
  isDirectory: false,
};

// ── A · the row-type matrix ───────────────────────────────────────────────

test("only the three text-carrying row types are pinnable", () => {
  const history: LauncherItem = {
    type: "history",
    id: "h1",
    title: "git status",
    commandLine: "git status --short",
  };
  const clipboard: LauncherItem = {
    type: "clipboard",
    id: "c1",
    title: "copied note",
    subtitle: "text",
    entry: entry(),
  };
  const plugin: LauncherItem = {
    type: "plugin",
    id: "p1",
    title: "Result",
    subtitle: "the plugin's own text",
    sourceName: "My Plugin",
  };
  assert.equal(pinTextApplies(history), true);
  assert.equal(pinTextApplies(clipboard), true);
  assert.equal(pinTextApplies(plugin), true);
});

test("a row with no single text body never earns a pin", () => {
  const rows: LauncherItem[] = [
    {
      type: "browser",
      id: "b1",
      title: "A page",
      subtitle: "example.com",
      url: "https://example.com",
      profileKey: "default",
    },
    {
      type: "calculator",
      id: "calc1",
      title: "1+1",
      subtitle: "= 2",
      entry: {
        id: "calc1",
        expression: "1+1",
        result: "2",
        created_at: 1,
        favorite: false,
      },
    },
    { type: "app", id: "a1", title: "Terminal", subtitle: "Application", app },
    { type: "system", id: "s1", title: "Restart", subtitle: "System", action: "restart" },
    {
      type: "command",
      id: "cmd1",
      title: "Build",
      subtitle: "Shell",
      warnings: [],
      sourceName: "Commands",
      commandLine: "make build",
      execution: null,
      completion: false,
    },
    { type: "file", id: "f1", title: "report.pdf", subtitle: "/tmp", file },
    { type: "status", id: "st1", title: "Nothing copied yet" },
  ];
  for (const row of rows) {
    assert.equal(pinTextApplies(row), false, `${row.type} must not be pinnable`);
  }
});

test("a clipboard row is pinnable exactly when its entry carries text", () => {
  const base = (entryValue: ClipboardEntry | undefined): LauncherItem => ({
    type: "clipboard",
    id: "c1",
    title: "Row",
    subtitle: "text",
    ...(entryValue ? { entry: entryValue } : {}),
  });
  // An empty body is still a body: the entry has text, so the pin is offered.
  assert.equal(pinTextApplies(base(entry({ text: "" }))), true);
  // An image with a caption rides text along; one without a caption does not.
  assert.equal(pinTextApplies(base(entry({ kind: "image", text: "caption" }))), true);
  assert.equal(pinTextApplies(base(entry({ kind: "image", text: null }))), false);
  assert.equal(pinTextApplies(base(entry({ kind: "files", text: null }))), false);
  // A status line (no entry at all) has nothing to pin.
  assert.equal(pinTextApplies(base(undefined)), false);
});

test("a plugin row is pinnable only when its own content reads as text", () => {
  const row = (subtitle: string): LauncherItem => ({
    type: "plugin",
    id: "p1",
    title: "Result",
    subtitle,
    sourceName: "My Plugin",
  });
  assert.equal(pinTextApplies(row("the plugin's own text")), true);
  // Nothing to say → `resolvePluginView` returns null → no pin.
  assert.equal(pinTextApplies(row("")), false);
});

// ── B · the snapshot the row hands over ───────────────────────────────────

test("the snapshot is the row's own title and text, never a re-run", () => {
  assert.deepEqual(
    pinTextFor({
      type: "history",
      id: "h1",
      title: "git status",
      commandLine: "git status --short",
    }),
    { title: "git status", text: "git status --short" },
  );
  assert.deepEqual(
    pinTextFor({
      type: "clipboard",
      id: "c1",
      title: "copied note",
      subtitle: "text",
      entry: entry({ text: "the full body" }),
    }),
    { title: "copied note", text: "the full body" },
  );
  assert.deepEqual(
    pinTextFor({
      type: "plugin",
      id: "p1",
      title: "Result",
      subtitle: "the plugin's own text",
      sourceName: "My Plugin",
    }),
    { title: "Result", text: "the plugin's own text" },
  );
});

// ── C · the wiring, pinned at the source ──────────────────────────────────

test("the row renders the pin only through the pure gate, in the star's slot", async () => {
  const results = await read("src/launcher/LauncherResults.tsx");
  assert.match(results, /import \{ pinTextApplies \} from "\.\.\/plugin-window\/pin-entry"/);
  assert.match(results, /onPinText\?: \(item: LauncherItem\) => void/);
  // The gate is the pure function, not a hand-rolled row-type test.
  assert.match(results, /const pinnable = onPinText !== undefined && pinTextApplies\(item\)/);
  // The button reuses the field pin's class and the R84 `Pin` glyph, and stops
  // the click before the row's run action.
  assert.match(results, /className="launcher-result__pin collapsed-card__settings"/);
  assert.match(results, /title=\{t\("pluginWindow\.pinText"\)\}/);
  assert.match(results, /<PinIcon size=\{13\} strokeWidth=\{2\} \/>/);
  assert.match(results, /onPinText\?\.\(item\)/);
});

test("the pin class has a rule, and the star's fade shape", async () => {
  const css = await read("src/styles/launcher.css");
  assert.match(css, /\.launcher-result__pin \{/);
  assert.match(css, /\.launcher-result:hover \.launcher-result__pin,/);
});

test("App assembles and validates the text arm, then invokes the same window", async () => {
  const app = await read("src/App.tsx");
  // R90 · the row pin builds a `text` request from the row's snapshot…
  assert.match(app, /const snapshot = pinTextFor\(item\)/);
  assert.match(app, /kind: "text",/);
  assert.match(app, /invoke\("detach_plugin_window", \{ request \}\)/);
  // …and the R84 field pin is tagged with its arm, field for field unchanged.
  assert.match(app, /kind: "external",/);
  assert.match(app, /onPinText=\{pinTextWindow\}/);
});

test("the detached view renders the text arm through PluginTextView, with no rerun", async () => {
  const view = await read("src/plugin-window/DetachedPluginApp.tsx");
  // The text arm short-circuits the run pipeline and measures its own body.
  assert.match(view, /if \(request\.kind === "text"\) \{/);
  assert.match(view, /text: state\.request\.text,/);
  assert.match(view, /metrics: pluginTextMetrics\(state\.request\.text\)/);
  // Rerun is the external arm's alone.
  assert.match(view, /state\.request\.kind === "external" && \(/);
});

// ── D · the language pair ─────────────────────────────────────────────────

test("the row pin's tooltip exists in both languages", () => {
  assert.equal(createTranslator("en")("pluginWindow.pinText"), "Pin as text window");
  assert.equal(createTranslator("zh")("pluginWindow.pinText"), "钉为文本窗口");
});
