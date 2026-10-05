// R88 · the switched-off plugin's search entry is a *status note*, not a result.
//
// R26-D/R36 gave the browser and clipboard entries a `system` row carrying
// `disabled: true`: it still looked like a result row (icon plate, subtitle
// stack), it was dimmed, and it was skipped by Enter. The user's direction 1
// (R78 §四-1) unified it with the note the plugin modes have drawn since R30 —
// the launcher's own `status` item, one muted sentence in the list's icon
// column, no plate, no pointer state, no Enter, **no `⌘N`**.
//
// This suite guards the two halves of that change:
//
//   1. the *shape*: the catalog builds a `status` item for the switched-off
//      entry, and no `system` row can carry `disabled` any more — so the
//      renderer's unavailable branch for it is gone with the state;
//   2. the *slot sequence*: a status note never takes a number, and the rows
//      after it number exactly as they would if the note were not there.
//
// Mutation that must turn this red: restoring `type: "system"` + `disabled:
// true` in the catalog (assertion 1), or putting the renderer's
// `system && disabled` unavailable branch back (assertion 2).

import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

import { launcherShortcutSlots } from "../src/launcher.ts";
import { resultRunnableFlags, resultShortcutSlots } from "../src/launcher/result-budget.ts";
import { createTranslator } from "../src/i18n.ts";
import { statusItem, statusRowBase } from "../src/plugins/status.ts";
import { decl, ruleFor, rules, stripComments } from "./css.ts";
import type { LauncherItem } from "../src/launcher/LauncherResults.tsx";

const root = new URL("../", import.meta.url);
const read = (path: string) => readFile(new URL(path, root), "utf8");
const stripJsComments = (source: string) =>
  source.replace(/\/\*[\s\S]*?\*\//g, "").replace(/^\s*\/\/.*$/gm, "");

const en = createTranslator("en");
const zh = createTranslator("zh");

const app = (id: string): LauncherItem => ({
  type: "app",
  id,
  title: id,
  subtitle: "Application",
  app: {} as never,
});

/** The switched-off browser entry exactly as the catalog builds it. */
const browserNote = (): LauncherItem =>
  statusItem("system-browser", en("launcher.browserDisabledRow", { name: en("system.browserSearch") }));

// ── 1 · the shape: a status note, not a result row ────────────────────────

test("the switched-off plugin's search entry is the launcher's status item", async () => {
  // The shared primitive really is the core (R40): the projected item is the
  // base's own id and title, with the family fields left behind.
  const base = statusRowBase("system-browser", "Browser · off");
  const item = statusItem("system-browser", "Browser · off");
  assert.deepEqual(item, { type: "status", id: base.id, title: base.title });
  assert.equal(base.kind, "status");
  assert.equal(base.disabled, true);

  // The catalog's switched-off branch builds that item, and it is the *only*
  // place the plugin switch is read for a row shape.
  const catalog = stripJsComments(await read("src/hooks/useLauncherCatalog.ts"));
  assert.match(
    catalog,
    /if \(pluginOff\) \{\s*const note = t\(/,
    "the switched-off entry takes its own branch",
  );
  assert.match(
    catalog,
    /matches\.push\(\{ item: statusItem\(`system-\$\{entry\.action\}`, note\), score \}\);/,
    "…and pushes the launcher's status item, not a `system` row",
  );
  assert.ok(
    !/disabled: true/.test(catalog),
    "no matched row is marked `disabled` any more — the note is out of the sequence by shape",
  );
  assert.match(
    catalog,
    /entry\.action === "browser" && !browserEnabled/,
    "the browser's own switch still decides",
  );
  assert.match(
    catalog,
    /entry\.action === "clipboard" && !clipboardEnabled/,
    "the clipboard's own switch still decides",
  );
});

test("the `system` row has no disabled state left, and the renderer has no branch for one", async () => {
  const results = stripJsComments(await read("src/launcher/LauncherResults.tsx"));
  // The variant: the R26-D `disabled?: boolean` is gone.
  const systemVariant = /\| \{\s*type: "system";[\s\S]*?\n    \}/.exec(results);
  assert.ok(systemVariant, "the system variant must exist");
  assert.ok(
    !/disabled/.test(systemVariant[0]),
    "R88: a `system` row is always a door — the variant carries no disabled field",
  );
  // The unavailable predicate: its `system && disabled` clause is gone.
  assert.ok(
    !/\(item\.type === "system" && item\.disabled === true\)/.test(results),
    "R88: the renderer's system-unavailable branch retired with the state",
  );
  // …while the status branch it now flows through is the one it always had.
  assert.match(
    results,
    /item\.type === "status"[\s\S]{0,400}?className="launcher-status" role="presentation"/,
    "a status item renders as the muted note",
  );

  // The runnable rule: no `system && disabled` clause either, and the plugin
  // clauses it does keep are untouched.
  const budget = stripJsComments(await read("src/launcher/result-budget.ts"));
  assert.ok(
    !/item\.type === "system" && item\.disabled/.test(budget),
    "R88: the slot rule no longer needs a system-disabled exception",
  );
  assert.match(budget, /item\.type === "status" \|\|/, "a status note is never runnable");
  assert.match(
    budget,
    /\(item\.type === "plugin" && \(item\.disabled === true \|\| item\.action === undefined\)\)/,
    "R39's plugin rule is untouched",
  );

  // The action handler: a system row always runs, because it is always a door.
  const actions = stripJsComments(await read("src/hooks/useLauncherActions.ts"));
  const handler = /const runSystemAction[\s\S]*?\n  };/.exec(actions);
  assert.ok(handler, "the system handler must exist");
  assert.ok(
    !/item\.disabled/.test(handler[0]),
    "R88: the system handler has no disabled state to read",
  );
});

// ── 2 · the slot sequence: a note never takes a number ────────────────────

test("a status note never takes a ⌘N slot, and the rows after it do not shift", () => {
  const note = browserNote();
  assert.equal(note.type, "status", "the fixture is the note the catalog builds");

  const withNote = [app("a"), note, app("b")];
  const flags = resultRunnableFlags(withNote);
  assert.deepEqual(
    flags,
    [true, false, true],
    "the note is not runnable; the two apps are",
  );
  assert.deepEqual(
    launcherShortcutSlots(flags),
    [1, null, 2],
    "the note takes no slot, and the row after it keeps the next number",
  );
  assert.deepEqual(resultShortcutSlots(withNote, flags), [1, null, 2], "the badge map agrees");

  // The key assertion: the sequence is exactly the one the same list without
  // the note gives — the note's presence cannot shift a single number.
  const withoutNote = [app("a"), app("b")];
  assert.deepEqual(
    launcherShortcutSlots(resultRunnableFlags(withoutNote)),
    [1, 2],
    "the note's absence is what the numbered sequence is computed from",
  );
  const withSlots = resultShortcutSlots(withNote, flags);
  assert.deepEqual(
    withSlots.filter((slot) => slot !== null),
    [1, 2],
    "the runnable rows' numbers are the same with the note in the list",
  );
  assert.equal(withSlots[2], 2, "…and `⌘2` still reaches the second app");

  // Wherever the note sits, it is the only row without a number.
  for (const rows of [
    [note, app("a"), app("b")],
    [app("a"), app("b"), note],
    [app("a"), note],
  ]) {
    const rowFlags = resultRunnableFlags(rows);
    const slots = resultShortcutSlots(rows, rowFlags);
    assert.equal(
      slots[rows.indexOf(note)],
      null,
      "the note's own row never carries a slot",
    );
    assert.deepEqual(
      slots.filter((slot) => slot !== null),
      rows.filter((row) => row !== note).map((_, index) => index + 1),
      "and the runnable rows number 1..n in order",
    );
  }
});

test("the existing non-result rules are untouched", () => {
  // R30/R39 · the status note and the action-less plugin row stay out, and a
  // history row / the terminal action bar are not results either.
  const history: LauncherItem = {
    type: "history",
    id: "history-1",
    title: "git status",
    commandLine: "git status",
  };
  const noPlan: LauncherItem = {
    type: "command",
    id: "command-1",
    title: "deploy",
    subtitle: "",
    warnings: [],
    sourceName: "Kit",
    commandLine: "deploy",
    execution: null,
    completion: false,
  };
  const pluginNote: LauncherItem = {
    type: "plugin",
    id: "plugin-1",
    title: "no output",
    subtitle: "",
    sourceName: "demo",
  };
  assert.deepEqual(
    resultRunnableFlags([app("a"), history, noPlan, pluginNote, app("b")]),
    [true, true, false, false, true],
    "history is runnable; a plan-less command and an action-less plugin row are not",
  );
  // The action bar is not part of the list, so it can never take a numbered
  // slot: `resultShortcutSlots` only ever sees rows.
  assert.deepEqual(
    resultShortcutSlots([app("a"), noPlan, app("b")], resultRunnableFlags([app("a"), noPlan, app("b")])),
    [1, null, 2],
  );
});

// ── 3 · the note's words exist in both dictionaries ───────────────────────

test("the merged note exists in both dictionaries and names the entry", () => {
  for (const key of ["launcher.browserDisabledRow", "launcher.clipboardDisabledRow"] as const) {
    const name = key === "launcher.browserDisabledRow" ? "browser" : "clipboard";
    assert.notEqual(zh(key, { name }), en(key, { name }), `${key} must be translated`);
    assert.ok(/[\u4e00-\u9fff]/.test(zh(key, { name })), `${key}'s zh value is Chinese`);
    assert.ok(en(key, { name }).includes(name), `${key} interpolates the entry's name`);
    assert.ok(zh(key, { name }).includes(name), `${key} interpolates the entry's name in zh too`);
  }
  // The catalog's title is the entry's own name plus the note, both translated.
  assert.equal(
    zh("launcher.clipboardDisabledRow", { name: zh("system.clipboardHistory") }),
    `${zh("system.clipboardHistory")} · 插件已停用`,
  );
});

// ── 4 · the note's CSS is list-level, not plugin-mode-level ───────────────

test("`.launcher-status` is a list-level selector, so the note reads the same in the search list", async () => {
  const sheet = stripComments(await read("src/styles/launcher.css"));
  const statusRules = rules(sheet).filter(({ selector }) =>
    selector.split(",").some((part) => part.trim().startsWith(".launcher-status")),
  );
  assert.ok(statusRules.length > 0, "the status family must exist");
  for (const { selector } of statusRules) {
    for (const part of selector.split(",").map((p) => p.trim())) {
      assert.ok(
        part.startsWith(".launcher-status"),
        `the note's selectors must not be scoped to a plugin mode (saw \`${part}\`)`,
      );
    }
  }
  // The note's own box: the 30u line the search list already charges it.
  const status = ruleFor(sheet, ".launcher-status");
  assert.match(decl(status, "min-height")!, /calc\(var\(--u\) \* 30\)/);
  assert.match(decl(status, "color")!, /var\(--text-muted\)/);
  ruleFor(sheet, ".launcher-status__icon");
  ruleFor(sheet, ".launcher-status__title");
});
