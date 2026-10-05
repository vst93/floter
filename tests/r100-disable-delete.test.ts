// R100 · the switch's two directions are not the same act.
//
// The user's direction: an integration needs *two* logics — 停用 (disable) is a
// *mark* that must ask first, 删除 (delete) is a real delete that already asks.
//
// This round closes the disable half (the frontend confirmation) and verifies
// the delete half (every removal entry already sits behind a confirmation).
// The backend is deliberately untouched: `extensions_disable` already writes
// the `enabled` flag and kills in-flight runs (R98) without deleting a file,
// and the uninstall paths already delete files behind `RemovalConfirmation` /
// the componentized dialog.
//
// Three mutations this file must catch:
//
//   1. the disable direction invoking `extensions_disable` directly instead of
//      arming `disableTarget` — the source guard on `toggleExtension` goes red;
//   2. `toggleIntent` returning `confirm-disable` for a disabled integration —
//      the pure "enable stays immediate" assertion goes red;
//   3. the description losing "running commands stop" (or "files/data kept") —
//      the wording assertion goes red.

import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

import { createTranslator } from "../src/i18n.ts";
import { toggleIntent } from "../src/extensions/toggle-intent.ts";
import type { Extension } from "../src/ExtensionsPanel.tsx";

const root = new URL("../", import.meta.url);
const read = (path: string) => readFile(new URL(path, root), "utf8");
const stripJsComments = (source: string) =>
  source.replace(/\/\*[\s\S]*?\*\//g, "").replace(/^\s*\/\/.*$/gm, "");

const en = createTranslator("en");
const zh = createTranslator("zh");

const extension = (patch: Partial<Extension> = {}): Extension =>
  ({ id: "local.a1b2c3d4", name: "Demo", enabled: true, ...patch }) as Extension;

// ── 1 · the pure intent: disable asks, enable is instant ───────────────────

test("R100 · an enabled integration's switch asks to confirm the disable", () => {
  const target = extension({ enabled: true });
  const intent = toggleIntent(target);
  assert.equal(intent.kind, "confirm-disable", "turning it off is the asking direction");
  // The same row object travels, so the confirmation can name it without a
  // lookup and the caller cannot drift from the row the user pressed.
  assert.equal(intent.extension, target);
});

test("R100 · a disabled integration's switch stays immediate (negative guard)", () => {
  // Mutation 2: returning `confirm-disable` here would make a non-destructive
  // re-enable ask a question — the round's explicit "enable is instant" rule.
  const target = extension({ enabled: false });
  const intent = toggleIntent(target);
  assert.equal(intent.kind, "enable", "re-enabling destroys nothing and must not ask");
  assert.equal(intent.extension, target);
  // The matrix is exactly the `enabled` field, nothing else.
  assert.equal(toggleIntent(extension({ enabled: true, state: "disabled" })).kind, "confirm-disable");
  assert.equal(toggleIntent(extension({ enabled: false, state: "enabled" })).kind, "enable");
});

// ── 2 · the panel routes the switch through that intent ────────────────────

test("R100 · the disable direction arms the confirmation, not the command", async () => {
  const panel = stripJsComments(await read("src/ExtensionsPanel.tsx"));
  const toggle = /const toggleExtension = \(extension: Extension\) => \{[\s\S]*?\n  \};/.exec(panel);
  assert.ok(toggle, "toggleExtension must exist");
  assert.match(toggle[0], /toggleIntent\(extension\)/, "the switch reads the shared intent");
  assert.match(
    toggle[0],
    /intent\.kind === "confirm-disable"[\s\S]{0,160}?setDisableTarget\(intent\.extension\)/,
    "the disable direction arms the target",
  );
  // Mutation 1: a direct `invoke("extensions_disable")` here is the bypass.
  assert.ok(
    !/invoke\("extensions_disable"/.test(toggle[0]),
    "toggleExtension must never call extensions_disable directly — that is the bypass mutation",
  );
  assert.match(
    toggle[0],
    /runMutation\(intent\.extension\.id, "enable", \(\) => invoke\("extensions_enable"/,
    "the enable direction still runs immediately, through the shared mutation path",
  );
});

test("R100 · the confirmed disable reuses runMutation's disable kind", async () => {
  const panel = stripJsComments(await read("src/ExtensionsPanel.tsx"));
  const confirm = /const confirmDisable = async \(\) => \{[\s\S]*?\n  \};/.exec(panel);
  assert.ok(confirm, "confirmDisable must exist");
  assert.match(confirm[0], /!disableTarget \|\| busyRef\.current/, "the confirm is guarded against a stale/double press");
  assert.match(
    confirm[0],
    /runMutation\(\s*extension\.id,\s*"disable",\s*\(\) => invoke\("extensions_disable", \{ id: extension\.id \}\),/,
    "the confirmed disable runs the existing command through runMutation",
  );
  assert.match(confirm[0], /if \(disabled\) setDisableTarget\(null\)/, "a successful disable dismisses the bar");
});

test("R100 · the switch is the one toggle entry point, and it is wired to the bar", async () => {
  const panel = stripJsComments(await read("src/ExtensionsPanel.tsx"));
  // The switch is the only control that calls the toggle...
  assert.equal(
    [...panel.matchAll(/toggleExtension\(extension\)/g)].length,
    1,
    "a second toggle call point must be registered in this suite, not slipped in",
  );
  assert.match(panel, /onToggle=\{\(\) => void toggleExtension\(extension\)\}/, "the row switch is that call point");
  // ...the bar is mounted in both surfaces, exactly like the removal bar...
  assert.match(
    panel,
    /\{!selected && disableTarget\?\.id === extension\.id && disableConfirmation\}/,
    "the inline bar follows its row in the list",
  );
  assert.match(panel, /\{disableConfirmation\}/, "the drawer mounts the same bar");
  // ...and dismissal follows the removal flow: the drawer's close clears an
  // armed disable before it closes.
  const close = /const closeDetails = \(\) => \{[\s\S]*?\n  \};/.exec(panel);
  assert.ok(close, "closeDetails must exist");
  assert.match(close[0], /if \(disableTarget\) \{ setDisableTarget\(null\); return; \}/, "Escape/outside cancels the disable first");
});

// ── 3 · the wording is a symmetric pair that states both facts ─────────────

test("R100 · the disable confirmation's wording is an en/zh pair", () => {
  const placeholders = (value: string) => [...value.matchAll(/\{(\w+)\}/g)].map((match) => match[1]).sort();
  for (const key of ["settings.extensions.disableTitle", "settings.extensions.disableDescription"] as const) {
    assert.ok(en(key).length > 0 && zh(key).length > 0, `${key} must be filled in both dictionaries`);
    assert.notEqual(en(key), zh(key), `${key} must be translated, not copied`);
    assert.deepEqual(placeholders(en(key)), placeholders(zh(key)), `${key} placeholders must match`);
  }
  assert.deepEqual(placeholders(en("settings.extensions.disableTitle")), ["name"], "the title names the integration");
  assert.ok(/[\u4e00-\u9fff]/.test(zh("settings.extensions.disableDescription")), "the zh description is Chinese");
});

test("R100 · the description states both facts: runs stop, and files/data stay", () => {
  // ① the R98 kill is told honestly — the user is warned a live command dies.
  assert.match(en("settings.extensions.disableDescription"), /stop any running commands/i);
  assert.match(zh("settings.extensions.disableDescription"), /正在运行的命令会立即停止/);
  // ② the mark's meaning: nothing is deleted, and it is reversible.
  assert.match(en("settings.extensions.disableDescription"), /files and data are kept/i);
  assert.match(en("settings.extensions.disableDescription"), /re-enable/i);
  assert.match(zh("settings.extensions.disableDescription"), /文件与数据都会保留/);
  assert.match(zh("settings.extensions.disableDescription"), /重新启用/);
});

test("R100 · both facts are rendered, not merely declared", async () => {
  const bar = stripJsComments(await read("src/extensions/DisableConfirmation.tsx"));
  assert.match(bar, /t\("settings\.extensions\.disableTitle", \{ name: extension\.name \}\)/, "the title is rendered with the name");
  assert.match(bar, /t\("settings\.extensions\.disableDescription"\)/, "the two-fact sentence is rendered");
  assert.match(bar, /t\("settings\.extensions\.disable"\)/, "the affirmative keeps the shared Disable label");
});

// ── 4 · the disable bar keeps the removal bar's interaction contract ───────

test("R100 · the disable bar shares the removal bar's a11y contract", async () => {
  const removal = stripJsComments(await read("src/extensions/RemovalConfirmation.tsx"));
  const disable = stripJsComments(await read("src/extensions/DisableConfirmation.tsx"));
  for (const [name, source] of [["RemovalConfirmation", removal], ["DisableConfirmation", disable]] as const) {
    assert.match(source, /cancelRef\.current\?\.focus\(\{ preventScroll: true \}\)/, `${name}: the cancel button takes focus`);
    assert.match(source, /document\.addEventListener\("keydown", escape, true\)/, `${name}: Escape is captured`);
    assert.match(source, /event\.stopImmediatePropagation\(\)/, `${name}: …and the press is not re-handled`);
    assert.match(source, /data-destructive-confirm/, `${name}: the affirmative carries the keyboard marker`);
    assert.match(source, /disabled=\{busy\}/, `${name}: the affirmative is disabled while busy`);
    assert.match(source, /role="alert"/, `${name}: the bar announces itself`);
  }
  // …but it is the *reversible* sibling: neutral band, no danger fill.
  assert.ok(!/--danger/.test(disable), "a reversible disable must not wear the danger fill");
  assert.ok(!/extensions-notice--error/.test(disable), "…nor the error band the removal bar uses");
  assert.match(disable, /className="extensions-notice extension-discard-bar"/, "the neutral notice band");
});

// ── 5 · the delete chain: every entry is behind a confirmation ─────────────

test("R100 · every removal entry point is behind a confirmation", async () => {
  const panel = stripJsComments(await read("src/ExtensionsPanel.tsx"));
  // The row menu and the drawer button both only *arm* the removal target; the
  // click never reaches a command.
  assert.match(
    panel,
    /const uninstallExtension = \(extension: Extension\) => setRemovalTarget\(extension\);/,
    "uninstallExtension arms the confirmation, it does not delete",
  );
  assert.ok(
    !/invoke\("extensions_uninstall/.test(/const uninstallExtension = [\s\S]*?;\n/.exec(panel)?.[0] ?? ""),
    "no removal command is reachable from the raw click",
  );
  // The two uninstall commands live only inside their confirm handlers.
  const simple = /const confirmRemoval = async \(\) => \{[\s\S]*?\n  \};/.exec(panel);
  const componentized = /const confirmComponentizedUninstall = async \([\s\S]*?\n  \};/.exec(panel);
  assert.ok(simple && componentized, "both confirm handlers must exist");
  assert.match(simple![0], /invoke\("extensions_uninstall", \{ id: extension\.id, removeData: false \}\)/, "the simple path deletes after confirmation");
  assert.match(simple![0], /setUninstallDialogTarget\(extension\)/, "npm/custom hand off to the componentized dialog");
  assert.match(componentized![0], /invoke\("extensions_uninstall_componentized"/, "the componentized path deletes after confirmation");
  // The componentized dialog is the only surface that reaches that command.
  assert.equal(
    [...panel.matchAll(/extensions_uninstall_componentized/g)].length,
    1,
    "extensions_uninstall_componentized has exactly one call site (the confirm handler)",
  );
  assert.match(panel, /const uninstallDialog = uninstallDialogTarget && \(/, "the dialog is driven by its own target");
});

test("R100 · the four removal wordings exist in both dictionaries", () => {
  const stems = ["deleteCustom", "uninstall", "disconnect", "removePackage"] as const;
  const placeholders = (value: string) => [...value.matchAll(/\{(\w+)\}/g)].map((match) => match[1]).sort();
  for (const stem of stems) {
    for (const suffix of ["", "Title", "Description"] as const) {
      const key = `settings.extensions.${stem}${suffix}` as Parameters<typeof en>[0];
      assert.ok(en(key).length > 0 && zh(key).length > 0, `${key} must be filled in both dictionaries`);
      assert.notEqual(en(key), zh(key), `${key} must be translated`);
      assert.deepEqual(placeholders(en(key)), placeholders(zh(key)), `${key} placeholders must match`);
    }
  }
});
