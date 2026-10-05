// R93 · the plugin-consistency closeout batch (the R92 report's first slice).
//
// Three small, fully front-end defects the read-only R92 study proved, closed
// together:
//
//   1. the launcher and the detached window printed a keyed `run_error` payload
//      raw, while the settings panel localised it (`runErrorMessage`);
//   2. the detached window's copy-on-select was a second, weaker implementation
//      of the launcher's rule (anchorNode only, a notice that never cleared, its
//      own i18n key);
//   3. an optimistic clipboard delete restored a whole stale snapshot when its
//      write was refused, with no guard for having left the mode meanwhile.
//
// Each half is pinned where a review could silently undo it: the pure rules are
// driven directly, the JSX/hook wiring is read off the source (the node runner
// does not render React), and the retired i18n key is asserted gone from both
// dictionaries. The mutations this file exists to catch are named in the tests.

import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

import { restoreDeletedEntry, type ClipboardEntry } from "../src/clipboard-history.ts";
import { runErrorMessage } from "../src/extensions/run-errors.ts";
import { createTranslator } from "../src/i18n.ts";

const root = new URL("../", import.meta.url);
const read = (path: string) => readFile(new URL(path, root), "utf8");
const stripJsComments = (source: string) =>
  source.replace(/\/\*[\s\S]*?\*\//g, "").replace(/^\s*\/\/.*$/gm, "");

const entry = (id: string, overrides: Partial<ClipboardEntry> = {}): ClipboardEntry => ({
  id,
  kind: "text",
  text: `text ${id}`,
  paths: null,
  image_file: null,
  width: null,
  height: null,
  hash: `hash-${id}`,
  created_at: 1,
  favorite: false,
  ...overrides,
});

// ── 1 · the keyed run error is localised on both external paths ────────────

test("R93 · the launcher's external-failed row runs the keyed message through runErrorMessage", async () => {
  const hook = stripJsComments(await read("src/hooks/useLauncherCatalog.ts"));
  assert.match(
    hook,
    /import \{ runErrorMessage \} from "\.\.\/extensions\/run-errors"/,
    "the shared mapper is imported",
  );
  // The localised message is consulted first…
  assert.match(
    hook,
    /runErrorMessage\(externalRun\.message, t\)/,
    "the launcher must localise the keyed failure",
  );
  // …and an unrecognised message keeps the old `raw || generic` fallback.
  assert.match(
    hook,
    /runErrorMessage\(externalRun\.message, t\)\s*\|\|\s*externalRun\.message\s*\|\|\s*t\("launcher\.externalFailed"\)/,
    "the raw message and the generic sentence stay as the fallback",
  );
  // Mutation: replace the call with `externalRun.message || t(...)` — the
  // `runErrorMessage(...)` match above goes red, which is exactly the point.
});

test("R93 · the detached window's failed status line localises the keyed message too", async () => {
  const view = stripJsComments(await read("src/plugin-window/DetachedPluginApp.tsx"));
  assert.match(view, /import \{ runErrorMessage \} from "\.\.\/extensions\/run-errors\.ts"/);
  assert.match(view, /runErrorMessage\(state\.failure \?\? "", t\)/);
  assert.match(
    view,
    /runErrorMessage\(state\.failure \?\? "", t\)\s*\|\|\s*state\.failure\s*\|\|\s*t\("launcher\.externalFailed"\)/,
  );
  // Mutation: print `state.failure || t(...)` again — red.
});

test("R93 · the keyed timeout payload is a sentence, not a raw key, in both languages", () => {
  // The behaviour the wiring above buys: the exact payload R92 found on screen.
  for (const language of ["en", "zh"] as const) {
    const t = createTranslator(language);
    const message = runErrorMessage('run_timeout:{"seconds":300}', t);
    assert.ok(message, `${language}: the timeout must map to a message`);
    assert.match(message, /300/);
    assert.ok(
      !message.includes("run_timeout"),
      `${language}: the raw key must not survive localisation`,
    );
  }
});

// ── 2 · the detached copy gesture is the launcher's own rule ───────────────

test("R93 · the detached selection check is the pure selectionTextIn, over its own container ref", async () => {
  const view = stripJsComments(await read("src/plugin-window/DetachedPluginApp.tsx"));
  assert.match(view, /import \{ selectionTextIn \} from "\.\.\/launcher\/plugin-text-copy\.ts"/);
  // The body ref is the container, exactly as the launcher hands its block ref.
  assert.match(view, /selectionTextIn\(outputRef\.current, window\.getSelection\(\)\)/);
  // The hand-written check is gone: no anchorNode-only ownership test survives.
  assert.doesNotMatch(
    view,
    /contains\(selection\.anchorNode\)/,
    "the anchorNode-only judgement must be replaced, not kept alongside",
  );
  assert.doesNotMatch(view, /navigator\.clipboard/);
  // Mutation: restore `container.contains(selection.anchorNode)` — both the
  // `selectionTextIn(...)` match and the `doesNotMatch` guard go red.
});

test("R93 · the detached notice is the shared phase machine, rendered as a status line", async () => {
  const view = stripJsComments(await read("src/plugin-window/DetachedPluginApp.tsx"));
  assert.match(view, /import \{ useCopyNotice \} from "\.\.\/hooks\/useCopyNotice\.ts"/);
  assert.match(view, /const \{ copyNotice, showCopyNotice \} = useCopyNotice\(\);/);
  // It clears itself, so the render is conditional on the message, like the
  // launcher's rows — not a bare `useState<string | null>` that never resets.
  assert.match(view, /\{copyNotice\.message && \(/);
  assert.match(view, /\{t\(copyNotice\.message\)\}/);
  assert.doesNotMatch(view, /useState<string \| null>/, "the never-clearing notice state is gone");
  // The one copy chokepoint, so both keys match the launcher's vocabulary.
  assert.match(view, /import \{ useLauncherTextCopy \} from "\.\.\/hooks\/useLauncherTextCopy\.ts"/);
  assert.match(view, /const \{ copySelection \} = useLauncherTextCopy\(showCopyNotice\);/);
  assert.match(view, /onCopySelection=\{copySelection\}/);
  // Mutation: go back to a raw `useState` notice — the `useCopyNotice` match
  // and the `useState<string | null>` guard both go red.
});

test("R93 · the notice lives in the detached window's own status area", async () => {
  const view = stripJsComments(await read("src/plugin-window/DetachedPluginApp.tsx"));
  // The status class the task names, with the announcement semantics the
  // launcher's row has.
  assert.match(
    view,
    /className="plugin-window__status"\s+role="status"\s+aria-live="polite"/,
  );
  // The retired floating strip is deleted from the sheet, not orphaned in it.
  const css = stripJsComments(await read("src/styles/launcher.css"));
  assert.ok(
    !css.includes(".plugin-window__notice"),
    ".plugin-window__notice must be deleted, not left as dead CSS",
  );
  assert.ok(css.includes(".plugin-window__status {"), "the status rule the notice now uses");
});

test("R93 · the two copy keys are one vocabulary: pluginWindow.copied is retired", async () => {
  const i18n = await read("src/i18n.ts");
  assert.ok(
    !i18n.includes('"pluginWindow.copied"'),
    "pluginWindow.copied is retired in favour of terminal.copyNotice.copied",
  );
  // The kept keys exist once per dictionary (en + zh).
  for (const key of ["terminal.copyNotice.copied", "terminal.copyNotice.failed"]) {
    const occurrences = i18n.split(`"${key}":`).length - 1;
    assert.equal(occurrences, 2, `${key} must appear once in en and once in zh`);
  }
  // And the detached surface no longer names the retired key.
  const view = stripJsComments(await read("src/plugin-window/DetachedPluginApp.tsx"));
  assert.doesNotMatch(view, /pluginWindow\.copied/);
});

test("R93 · the text arm copies through PluginTextView's own gesture, never twice", async () => {
  const view = stripJsComments(await read("src/plugin-window/DetachedPluginApp.tsx"));
  // The body-wide listener is scoped to the list arm: the text arm renders
  // `PluginTextView`, which owns the identical gesture and would otherwise
  // write the clipboard a second time.
  assert.match(
    view,
    /if \(view\?\.form === "text"\) return;/,
    "the body listener must stand down for the text arm",
  );
});

// ── 3 · the refused delete restores one row, only while the mode is open ───

test("R93 · a refused delete puts the deleted entry back where it was", () => {
  const a = entry("a");
  const b = entry("b");
  const c = entry("c");
  // The optimistic delete already removed `b`; the refusal re-inserts it at 1.
  assert.deepEqual(
    restoreDeletedEntry([a, c], { active: true, removed: b, index: 1 }).map((e) => e.id),
    ["a", "b", "c"],
  );
  // A changed list keeps its changes: only the one row comes back.
  assert.deepEqual(
    restoreDeletedEntry([a, c, entry("d")], { active: true, removed: b, index: 1 }).map(
      (e) => e.id,
    ),
    ["a", "b", "c", "d"],
  );
});

test("R93 · leaving the clipboard mode means the refusal restores nothing", () => {
  const a = entry("a");
  const b = entry("b");
  const current = [a];
  const restored = restoreDeletedEntry(current, { active: false, removed: b, index: 1 });
  // Identity, not just equality: the mode's exit already cleared the list, and
  // a refused write must not resurrect the whole table (the old snapshot bug).
  assert.equal(restored, current);
  // Mutation: drop the `!active` gate — this identity assertion goes red.
});

test("R93 · a row the list already has is not duplicated, and a missing row is a no-op", () => {
  const a = entry("a");
  const b = entry("b");
  const current = [a, b];
  assert.equal(restoreDeletedEntry(current, { active: true, removed: b, index: 1 }), current);
  assert.equal(restoreDeletedEntry(current, { active: true, removed: null, index: 0 }), current);
  // An out-of-range index clamps rather than throwing.
  assert.deepEqual(
    restoreDeletedEntry([a], { active: true, removed: b, index: 99 }).map((e) => e.id),
    ["a", "b"],
  );
});

test("R93 · the delete hook restores one row through the pure rule, never the snapshot", async () => {
  const hook = stripJsComments(await read("src/hooks/useLauncherCatalog.ts"));
  assert.match(hook, /import \{ normalizeEntries, restoreDeletedEntry,/);
  // The old whole-snapshot rollback must not come back.
  assert.doesNotMatch(hook, /setClipboardEntries\(previous\)/);
  assert.doesNotMatch(hook, /const previous = clipboardEntriesRef\.current;/);
  // The failure path reads the mode's *current* liveness and hands the one
  // removed row (and its index) to the pure rule.
  assert.match(hook, /restoreDeletedEntry\(entries, \{/);
  assert.match(hook, /active: clipboardActiveRef\.current,/);
  assert.match(hook, /const clipboardActiveRef = useRef\(false\);/);
  // Mutation: delete the `active: clipboardActiveRef.current` line or put the
  // snapshot back — the corresponding guard above goes red.
});
