// R151 · the desktop conventions (the browser's tells, removed once).
//
// The reference is Raycast's "A Technical Deep Dive Into the New Raycast"
// (May 2026). Its "Platform conventions" section lists the tells that make a
// WebView app read as a website to a desktop user:
//
//   * No `cursor: pointer` on interactive controls. Native toolkits
//     (AppKit / WinUI / GTK) use the arrow; the hand is the browser's signal
//     for a hyperlink.
//   * No hover highlights on most controls, native popovers, and a separate
//     Settings window. These are macOS-shaped, and floter is a single-panel
//     cross-platform app, so they are deliberately **not** adopted wholesale:
//     the panel is the app on every OS, and hover is the only discoverability
//     affordance on Windows/Linux. The round takes the two conventions that
//     are correct on all three platforms and leaves the rest.
//   * Chrome is not selectable text; icons are not drag sources; command
//     fields carry no spellcheck squiggles.
//
// The suite asserts the *absence* of the tell as a census over every
// stylesheet, and the *presence* of the native cursors that do carry meaning,
// so re-introducing the tell is what turns it red — not deleting the prose.
import assert from "node:assert/strict";
import { readFile, readdir } from "node:fs/promises";
import { join } from "node:path";
import test from "node:test";

import { rules, stripComments } from "./css.ts";

const root = new URL("../", import.meta.url);
const read = (path: string) => readFile(new URL(path, root), "utf8");

/** Every `.css` file under `src`, walked recursively. */
const cssFiles = async (dir = "src"): Promise<string[]> => {
  const absolute = new URL(dir, root);
  const entries = await readdir(absolute, { withFileTypes: true });
  const out: string[] = [];
  for (const entry of entries) {
    const child = `${dir}/${entry.name}`;
    if (entry.isDirectory()) out.push(...(await cssFiles(child)));
    else if (entry.name.endsWith(".css")) out.push(child);
  }
  return out;
};

const stylesheets = async () => {
  const files = await cssFiles();
  assert.ok(files.length >= 6, `the census must cover the tree (${files.length} files)`);
  return Promise.all(
    files.map(async (file) => ({ file, css: stripComments(await read(file)) })),
  );
};

// ── 1 · the hand cursor is gone ────────────────────────────────────────────

test("no stylesheet turns a control into a hand cursor", async () => {
  const sheets = await stylesheets();
  const offenders = sheets
    .filter(({ css }) => /cursor\s*:\s*pointer/.test(css))
    .map(({ file }) => file);
  assert.deepEqual(offenders, [], "a hand cursor is the browser's tell, not a desktop one");
});

test("mutation: re-adding the hand cursor is detected", async () => {
  // The predicate the census really asserts. A "quick fix" that adds one
  // `cursor: pointer` back to a new control must fail, so the detection is
  // pinned rather than the empty result alone.
  const handCursor = (css: string) => /cursor\s*:\s*pointer/.test(stripComments(css));
  const base = await read("src/styles/base.css");
  assert.ok(!handCursor(base), "the shipped sheet is clean");
  assert.ok(handCursor(`${base}\n.foo { cursor: pointer; }\n`), "the tell must be detectable");
});

test("the native cursors that carry meaning are kept", async () => {
  // The round is not "delete every cursor". A drag band advertises `grab`; a
  // text surface advertises `text`; a busy or disabled control says so. These
  // are native signals, not web tells.
  const launcher = await read("src/styles/launcher.css");
  assert.match(launcher, /cursor:\s*grab/, "the drag band keeps its grab cursor");
  assert.match(launcher, /cursor:\s*grabbing/, "the active drag keeps its grabbing cursor");
  assert.match(launcher, /cursor:\s*text/, "a text surface keeps the I-beam");
  assert.match(launcher, /cursor:\s*not-allowed/, "an unavailable row says so");
  const settings = await read("src/styles/settings.css");
  assert.match(settings, /cursor:\s*wait/, "a busy control says so");
  assert.match(settings, /cursor:\s*default/, "a disabled control returns to the arrow");
});

// ── 2 · chrome is not selectable text ──────────────────────────────────────

test("the base sheet makes interactive chrome non-selectable", async () => {
  const base = await read("src/styles/base.css");
  const block = base.slice(base.indexOf("Desktop conventions"));
  assert.notEqual(block.length, base.length, "the conventions block must exist");
  // The control roles the app actually renders. `input`/`textarea` must NOT be
  // in the list: a text field is a text surface by definition.
  for (const selector of [
    'button',
    '[role="button"]',
    '[role="switch"]',
    '[role="tab"]',
    '[role="radio"]',
    '[role="checkbox"]',
    '[role="menuitem"]',
    '[role="option"]',
  ]) {
    assert.ok(block.includes(selector), `the baseline must cover ${selector}`);
  }
  assert.ok(
    /-webkit-user-select:\s*none;\s*\n\s*user-select:\s*none;/.test(block),
    "the control baseline disables selection (prefixed + standard)",
  );
  // The arrow is on the baseline too: the drag surfaces set `cursor: grab` on a
  // container, so a control that only *removed* its pointer declaration would
  // inherit the grab.
  assert.match(
    block,
    /user-select:\s*none;\s*\n\s*cursor:\s*default;/,
    "the control baseline restores the arrow cursor",
  );
  assert.ok(!/\binput\b\s*,/.test(block.slice(0, block.indexOf("{"))), "inputs keep selection");
});

test("the text surfaces opt back in", async () => {
  // The counterpart to the baseline: the places a user legitimately selects
  // from re-enable it explicitly, so the convention is a boundary rather than
  // a blanket.
  const launcher = await read("src/styles/launcher.css");
  const extensions = await read("src/styles/extensions.css");
  assert.match(launcher, /user-select:\s*text/, "the query field selects");
  assert.match(extensions, /user-select:\s*text/, "command output selects");
});

// ── 3 · icons are not drag sources, and there is no tap flash ──────────────

test("images and glyphs cannot start a browser drag", async () => {
  const base = await read("src/styles/base.css");
  assert.match(
    base,
    /img,\s*\n\s*svg\s*\{\s*\n\s*-webkit-user-drag:\s*none;/,
    "the icon drag ghost is disabled at the base",
  );
});

test("the touch tap highlight is switched off", async () => {
  const base = await read("src/styles/base.css");
  assert.match(base, /-webkit-tap-highlight-color:\s*transparent/, "no touch-web flash");
});

// ── 4 · command fields carry no spellcheck squiggles ───────────────────────

test("the document root disables spellcheck for the whole app", async () => {
  const html = await read("index.html");
  // `spellcheck` is an inherited attribute, so one declaration on <body> covers
  // every text field that does not opt in. A command box with a red squiggle
  // under a flag or a path is the web telling on itself.
  assert.match(html, /<body[^>]*\bspellcheck="false"/, "the root disables spellcheck");
  assert.match(html, /<body[^>]*\bautocorrect="off"/, "and OS autocorrect");
  assert.match(html, /<body[^>]*\bautocapitalize="off"/, "and OS autocapitalisation");
});

// ── 5 · the macOS control treatment ────────────────────────────────────────

test("the switch reads as the macOS toggle: accent track, white thumb", async () => {
  // The one control whose macOS shape is unmistakable, and whose change is
  // accent-budget-neutral: the accent used to ride the thumb, it now rides the
  // track, so the census still counts exactly one face for the switch (the
  // ledger names `.settings-switch--active` instead of the thumb).
  const settings = stripComments(await read("src/styles/settings.css"));
  const on = rules(settings).find((r) => r.selector === ".settings-switch--active");
  assert.ok(on, ".settings-switch--active must exist");
  assert.match(on!.body, /background:\s*var\(--accent\)/, "the on track is accent-filled");
  const thumb = rules(settings).find(
    (r) => r.selector === ".settings-switch--active .settings-switch__thumb",
  );
  assert.ok(thumb, "the on thumb must exist");
  assert.match(thumb!.body, /background:\s*var\(--switch-thumb\)/, "the on thumb is white");
  const base = await read("src/styles/base.css");
  assert.match(base, /--switch-thumb:\s*#ffffff/, "the knob is white in both palettes");
});

// ── 6 · the census is honest ───────────────────────────────────────────────
test("the stylesheet census covers every surface file", async () => {
  const files = await cssFiles();
  for (const expected of [
    "src/styles/base.css",
    "src/styles/launcher.css",
    "src/styles/settings.css",
    "src/styles/extensions.css",
    "src/styles/terminal.css",
    "src/styles/plugin-config.css",
    "src/extensions/ComponentizedUninstallDialog.css",
  ]) {
    assert.ok(files.includes(expected), `the census must reach ${expected}`);
  }
});
