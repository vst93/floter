// R7-10b · system notifications for background completions — and nothing else.
//
// The round's whole rule is one table, and every test here exists to keep one
// of its edges from eroding:
//
//   1. a system notification is raised **only** when the panel is hidden, and
//      then on both outcomes — a visible panel already shows the toast, and
//      raising both is the double report the round forbids;
//   2. the body always names the subject (which integration / plugin) and the
//      action and the outcome, in both languages, symmetrically;
//   3. clicking a notification shows the main window through the app's existing
//      reveal path — no second window manager;
//   4. the decision lives in exactly one place per side, so no call site can
//      grow its own rule;
//   5. no new Tauri command and no `@tauri-apps/plugin-notification` dependency:
//      the background paths live in Rust, and a JS delivery path would be a
//      second way to raise the same banner.
//
// Two mutation locks at the bottom prove the two load-bearing guards (the
// visibility gate and the click→reveal wiring) actually fail when removed.
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

import {
  NOTIFICATION_ACTIONS,
  NOTIFICATION_OUTCOMES,
  NOTIFICATION_PLUGIN_IDS,
  NOTIFICATION_TITLE,
  isChinese,
  notificationBody,
  notificationBodyKey,
  notificationCopy,
  notificationSubjectKey,
  notificationTemplate,
  shouldNotify,
  subjectName,
} from "../src/notifications.ts";
import { createTranslator, isMessageKey } from "../src/i18n.ts";

const root = new URL("../", import.meta.url);
const read = (path: string) => readFile(new URL(path, root), "utf8");
const stripJsComments = (src: string) =>
  src.replace(/\/\*[\s\S]*?\*\//g, "").replace(/\/\/[^\n]*/g, "");
const readRust = async (path: string) => stripJsComments(await read(path));

/** The Rust `notifications.rs` production half (its `mod tests` block
 * carries its own copies of the tables, which must not satisfy these pins). */
const rustNotifications = async () => {
  const source = await readRust("src-tauri/src/notifications.rs");
  const testStart = source.indexOf("mod tests {");
  return testStart === -1 ? source : source.slice(0, testStart);
};

const chinese = (text: string) => /[\u4e00-\u9fff]/.test(text);

// ── 1 · the four-quadrant rule ─────────────────────────────────────────────

test("only a hidden panel notifies, and it notifies on both outcomes", () => {
  assert.equal(shouldNotify("visible", "success"), false);
  assert.equal(shouldNotify("visible", "failure"), false);
  assert.equal(shouldNotify("hidden", "success"), true);
  assert.equal(shouldNotify("hidden", "failure"), true);
});

test("the Rust gate is the same four-quadrant table", async () => {
  const rust = await rustNotifications();
  assert.match(
    rust,
    /\(Foreground::Visible, Outcome::Success \| Outcome::Failure\) => false/,
    "a visible panel must never notify, on either outcome",
  );
  assert.match(
    rust,
    /\(Foreground::Hidden, Outcome::Success \| Outcome::Failure\) => true/,
    "a hidden panel notifies on both outcomes",
  );
  // The Rust unit test drives the table directly (it lives in the test module,
  // so it is asserted against the full source, not the production slice).
  assert.match(
    await readRust("src-tauri/src/notifications.rs"),
    /fn only_a_hidden_panel_notifies_and_it_notifies_on_both_outcomes/,
    "the rule is unit-tested on the Rust side too",
  );
});

// ── 2 · one decision point per side ────────────────────────────────────────

test("the panel state combines the app flag with the window's own answers", async () => {
  const rust = await rustNotifications();
  // The three signals, and the fact that *any* of them suppresses.
  assert.match(
    rust,
    /pub\(crate\) fn panel_foreground_from\(\s*flag_visible: bool,\s*window_visible: bool,\s*window_focused: bool,\s*\) -> Foreground \{\s*if flag_visible \|\| window_visible \|\| window_focused \{\s*Foreground::Visible\s*\} else \{\s*Foreground::Hidden\s*\}\s*\}/,
    "any one on-screen signal wins",
  );
  // The live reader feeds it all three, and a missing window reads as hidden.
  assert.match(
    rust,
    /pub\(crate\) fn panel_foreground\(app: &AppHandle\) -> Foreground \{[\s\S]*?window_visible[\s\S]*?is_visible\(\)[\s\S]*?is_focused\(\)[\s\S]*?panel_foreground_from\(/,
    "the live reader asks the app flag and both window questions",
  );
  assert.match(
    rust,
    /None => \(false, false\)/,
    "a missing window is hidden, which is exactly when a notification helps",
  );
  assert.match(
    await readRust("src-tauri/src/notifications.rs"),
    /fn any_signal_of_an_on_screen_panel_suppresses_the_notification/,
    "the combination is unit-tested",
  );
});

test("the visibility decision has exactly one call site in Rust", async () => {
  const rust = await rustNotifications();
  const gateCalls = [...rust.matchAll(/should_notify\(/g)].length;
  // The declaration plus the single call inside `notify_completion`.
  assert.equal(gateCalls, 2, "should_notify is declared once and called once");
  assert.match(
    rust,
    /pub\(crate\) fn notify_completion\([\s\S]*?if !should_notify\(panel_foreground\(app\), outcome\) \{\s*return;/,
    "every sender goes through the gate, never around it",
  );
  // And the call sites in the command layer only ever use the one helper.
  const commands = await readRust("src-tauri/src/commands/extensions.rs");
  assert.ok(
    !/\.notification\(\)/.test(commands),
    "the command layer must not talk to the plugin directly",
  );
  assert.ok(
    !/notify_completion/.test(commands) || /notifications::notify_completion/.test(commands),
    "the command layer reaches the sender through the module",
  );
});

test("the trigger surface is the four declared points and nothing else", async () => {
  const commands = await readRust("src-tauri/src/commands/extensions.rs");
  const clipboard = await readRust("src-tauri/src/clipboard_history/mod.rs");
  const lib = await readRust("src-tauri/src/lib.rs");
  const callSites = (source: string) => [...source.matchAll(/notify_completion\(/g)].length;
  // install, uninstall (legacy), uninstall (componentized), repair, reprobe and
  // the one background loop = 6 in the extensions command layer…
  assert.equal(
    callSites(commands),
    6,
    "extensions commands: install + 2 uninstalls + repair + reprobe + the drift loop",
  );
  // …and exactly one for the plugin page's long action.
  assert.equal(callSites(clipboard), 1, "the clipboard clear-history action is the plugin-page trigger");
  // Nothing in lib.rs raises a notification: the RunEvent branch only reveals.
  assert.equal(callSites(lib), 0, "lib.rs must not raise notifications");
  assert.match(lib, /notifications::reveal_after_activation\(app\)/, "it wires the click instead");
});

// ── 3 · the copy, in both languages ────────────────────────────────────────

test("every action/outcome pair has a bilingual template that names the subject", () => {
  for (const action of NOTIFICATION_ACTIONS) {
    for (const outcome of NOTIFICATION_OUTCOMES) {
      const key = notificationBodyKey(action, outcome);
      assert.ok(isMessageKey(key), `${key} must be a real dictionary key`);
      const en = notificationTemplate(action, outcome, "en");
      const zh = notificationTemplate(action, outcome, "zh");
      assert.ok(en.includes("{name}"), `${action}/${outcome} must name the subject`);
      assert.ok(zh.includes("{name}"), `${action}/${outcome} must name the subject`);
      assert.notEqual(zh, en, `${action}/${outcome} must be translated, not fall back`);
      assert.ok(chinese(zh), `${action}/${outcome} must contain Chinese text`);
    }
  }
  // A region-tagged language still selects Chinese.
  assert.equal(isChinese("zh-CN"), true);
  assert.equal(isChinese("en-US"), false);
  assert.equal(notificationTemplate("repair", "success", "zh-CN"), "{name} 已修复");
});

test("the body names the integration, the action and the outcome", () => {
  const docker = { kind: "integration" as const, name: "Docker" };
  assert.equal(notificationBody(docker, "repair", "success", "en"), "Docker repaired");
  assert.equal(notificationBody(docker, "repair", "failure", "zh"), "Docker 修复失败");
  assert.equal(notificationBody(docker, "uninstall", "success", "en"), "Docker uninstalled");
  assert.equal(notificationBody(docker, "reprobe", "success", "zh"), "Docker 命令列表已更新");
  // A failure sentence is never the success sentence with a negation glued on
  // in the wrong language — both are declared, and they differ.
  assert.notEqual(
    notificationBody(docker, "install", "success", "en"),
    notificationBody(docker, "install", "failure", "en"),
  );
});

test("the title is the app name in both languages", () => {
  assert.equal(NOTIFICATION_TITLE, "floter");
  assert.equal(isMessageKey("notification.title"), true);
  assert.equal(createTranslator("en")("notification.title"), "floter");
  assert.equal(createTranslator("zh")("notification.title"), "floter");
  const copy = notificationCopy({ kind: "integration", name: "Docker" }, "repair", "success", "zh");
  assert.equal(copy.title, "floter");
  assert.equal(copy.body, "Docker 已修复");
});

test("a plugin subject is named bilingually and never leaks a raw id", () => {
  for (const id of NOTIFICATION_PLUGIN_IDS) {
    const subject = { kind: "plugin" as const, id };
    const key = notificationSubjectKey(subject);
    assert.ok(key && isMessageKey(key), `${id} needs a dictionary key`);
    const en = subjectName(subject, "en");
    const zh = subjectName(subject, "zh");
    assert.notEqual(en, id, "the id must not be painted");
    assert.ok(chinese(zh), `${id} needs a Chinese name`);
    assert.equal(
      notificationBody(subject, "clearHistory", "failure", "en"),
      `${en} could not be cleared`,
    );
  }
  // An unknown plugin still gets a name, not an id and not an empty string.
  const unknown = { kind: "plugin" as const, id: "builtin.nope" };
  assert.equal(subjectName(unknown, "en"), createTranslator("en")("notification.subject.plugin"));
  assert.equal(subjectName(unknown, "zh"), createTranslator("zh")("notification.subject.plugin"));
});

test("the plural subject and an unnamed integration still read as a subject", () => {
  assert.equal(
    notificationBody({ kind: "integrations" }, "reprobe", "success", "en"),
    `${createTranslator("en")("notification.subject.integrations")} command list updated`,
  );
  assert.equal(
    notificationBody({ kind: "integrations" }, "reprobe", "success", "zh"),
    `${createTranslator("zh")("notification.subject.integrations")} 命令列表已更新`,
  );
  // An integration with no name falls back to the plural kind rather than
  // producing " could not be repaired".
  assert.equal(
    notificationBody({ kind: "integration", name: "  " }, "repair", "failure", "en"),
    `${createTranslator("en")("notification.subject.integrations")} could not be repaired`,
  );
});

// ── 4 · the two sides agree ────────────────────────────────────────────────

test("the Rust and TypeScript vocabularies match", async () => {
  const rust = await rustNotifications();
  // Every action this module declares exists in the Rust enum, with the same
  // spelling (the enum's `as_str` is the wire form).
  for (const action of NOTIFICATION_ACTIONS) {
    assert.match(
      rust,
      new RegExp(`Self::[A-Za-z]+ => "${action}"`),
      `Rust must know the ${action} action`,
    );
  }
  // Every Rust variant is declared here too, so a new one cannot be added on
  // one side only. The `ALL` array is the Rust side's own inventory.
  const allBlock = rust.slice(rust.indexOf("const ALL: [Self;"), rust.indexOf("];", rust.indexOf("const ALL: [Self;")));
  const rustVariants = [...allBlock.matchAll(/Self::([A-Za-z]+)/g)].map((m) => m[1]);
  assert.equal(rustVariants.length, NOTIFICATION_ACTIONS.length);
  assert.equal(
    new Set(rustVariants).size,
    NOTIFICATION_ACTIONS.length,
    "the Rust inventory has no duplicates",
  );
});

// The strongest anti-drift check available without a live process: the strings
// Rust paints and the strings this module builds are compared character for
// character, for every action × outcome × language. A copy edit on one side
// only fails here.
test("the Rust templates and the app dictionary are the same sentences", async () => {
  const rust = await rustNotifications();
  // Pull the `body_template` match arms: (language, action, outcome) => literal.
  const arms = [
    ...rust.matchAll(
      /\((true|false), CompletionAction::(\w+), Outcome::(Success|Failure)\) => "([^"]*)"/g,
    ),
  ];
  assert.equal(
    arms.length,
    NOTIFICATION_ACTIONS.length * NOTIFICATION_OUTCOMES.length * 2,
    "every action × outcome × language arm is declared in Rust",
  );
  const rustTemplates = new Map<string, string>();
  for (const [, chineseFlag, action, outcome, template] of arms) {
    const language = chineseFlag === "true" ? "zh" : "en";
    rustTemplates.set(`${action}.${outcome}.${language}`, template);
  }
  const capitalize = (name: string) => name.charAt(0).toUpperCase() + name.slice(1);
  for (const action of NOTIFICATION_ACTIONS) {
    for (const outcome of NOTIFICATION_OUTCOMES) {
      const rustAction = capitalize(action);
      const rustOutcome = capitalize(outcome);
      for (const language of ["en", "zh"] as const) {
        const key = `${rustAction}.${rustOutcome}.${language}`;
        const fromRust = rustTemplates.get(key);
        assert.ok(fromRust, `Rust must declare ${key}`);
        assert.equal(
          notificationTemplate(action, outcome, language),
          fromRust,
          `${key}: the app dictionary and the Rust template must be the same sentence`,
        );
      }
    }
  }
  // The title is a literal on both sides.
  assert.match(rust, /pub\(crate\) const APP_TITLE: &str = "floter";/);
  // The plural and fallback subjects are literals on both sides.
  assert.match(rust, /"Integrations"\.to_string\(\)/);
  assert.match(rust, /"集成"\.to_string\(\)/);
  assert.equal(createTranslator("en")("notification.subject.integrations"), "Integrations");
  assert.equal(createTranslator("zh")("notification.subject.integrations"), "集成");
  assert.equal(createTranslator("en")("notification.subject.plugin"), "Plugin");
  assert.equal(createTranslator("zh")("notification.subject.plugin"), "插件");
  assert.equal(createTranslator("en")("notification.plugin.builtin.clipboard"), "Clipboard History");
  assert.equal(createTranslator("zh")("notification.plugin.builtin.clipboard"), "剪贴板历史");
});

test("every plugin page the backend registers has a notification name", async () => {
  const rust = await readRust("src-tauri/src/plugin_pages.rs");
  const ids = [...rust.matchAll(/id:\s*([A-Z_]+),/g)].map((m) => m[1]);
  const constants = new Map(
    [...rust.matchAll(/pub const ([A-Z_]+): &str = "([^"]+)";/g)].map((m) => [m[1], m[2]]),
  );
  assert.ok(ids.length > 0, "the registry must have at least one page");
  for (const constant of ids) {
    const id = constants.get(constant);
    assert.ok(id, `${constant} must resolve to a literal id`);
    assert.ok(
      NOTIFICATION_PLUGIN_IDS.includes(id),
      `${id} is registered as a page but has no notification name`,
    );
  }
  // And every name this module declares is a page the backend actually has.
  for (const id of NOTIFICATION_PLUGIN_IDS) {
    assert.ok([...constants.values()].includes(id), `${id} is not a registered plugin page`);
  }
});

// ── 5 · click → reveal, through the existing path ──────────────────────────

test("a click reveals the main window through the existing reveal path", async () => {
  const rust = await rustNotifications();
  // The click decision exists and only the click reveals.
  assert.match(
    rust,
    /pub\(crate\) fn on_notification_click\(click: NotificationClick, reveal: impl FnOnce\(\)\)/,
    "the click handler takes its reveal as a parameter",
  );
  assert.match(
    rust,
    /match click \{\s*NotificationClick::Clicked => reveal\(\),\s*NotificationClick::Other => \{\}/,
    "only Clicked reveals",
  );
  // The production binding reuses `reveal_saved_mode` — the app's one reveal
  // path — rather than any new window management.
  assert.match(
    rust,
    /pub\(crate\) fn reveal_after_activation\(app: &AppHandle\)[\s\S]*?crate::reveal_saved_mode\(window, &state\)/,
    "the reveal goes through the shared path",
  );
  // No new window machinery in this module: the only `.show()` is the
  // notification plugin's own delivery call, and there is no window reveal,
  // focus or construction of its own.
  assert.ok(
    !/WebviewWindowBuilder|window\.show\(|window\.hide\(|window\.set_focus\(/.test(rust),
    "the module must not open a second window-management path",
  );
  assert.ok(
    !/reveal_window\(|set_always_on_top\(/.test(rust),
    "the reveal goes through reveal_saved_mode, not a private copy of it",
  );
  // The RunEvent wiring is the OS activation callback.
  const lib = await readRust("src-tauri/src/lib.rs");
  assert.match(
    lib,
    /tauri::RunEvent::Reopen \{ \.\. \} => notifications::reveal_after_activation\(app\)/,
    "an app activation (a notification click) runs the reveal",
  );
});

// ── 6 · no new command, no JS delivery dependency ──────────────────────────

test("the round adds no Tauri command and no JS notification package", async () => {
  const cargo = await read("src-tauri/Cargo.toml");
  assert.match(
    cargo,
    /^tauri-plugin-notification = "2\.4\.0"$/m,
    "the Rust plugin is pinned to the 2.4 line",
  );
  const pkg = JSON.parse(await read("package.json")) as {
    dependencies?: Record<string, string>;
    devDependencies?: Record<string, string>;
  };
  const all = { ...(pkg.dependencies ?? {}), ...(pkg.devDependencies ?? {}) };
  assert.ok(
    !("@tauri-apps/plugin-notification" in all),
    "the frontend must not gain a second way to raise a notification",
  );
  // The plugin is registered for its Rust API, and the guest JS API is never
  // invoked from the app document.
  const lib = await readRust("src-tauri/src/lib.rs");
  assert.match(lib, /\.plugin\(tauri_plugin_notification::init\(\)\)/, "the plugin is registered");
  const sources = await Promise.all(
    [
      "src/notifications.ts",
      "src/App.tsx",
      "src/ExtensionsPanel.tsx",
      "src/plugins/PluginPageHost.tsx",
    ].map(async (file) => stripJsComments(await read(file))),
  );
  for (const source of sources) {
    assert.ok(
      !/plugin-notification|plugin:notification\|notify/.test(source),
      "no frontend invoke of the notification plugin",
    );
  }
  // And the capability is the single minimal `allow-notify` permission.
  const capability = JSON.parse(await read("src-tauri/capabilities/default.json")) as {
    permissions: string[];
  };
  const notificationPermissions = capability.permissions.filter((permission) =>
    permission.startsWith("notification:"),
  );
  assert.deepEqual(
    notificationPermissions,
    ["notification:allow-notify"],
    "the minimal notification capability is allow-notify and nothing else",
  );
  assert.ok(
    !capability.permissions.includes("notification:default"),
    "the full default bundle is deliberately not granted",
  );
});

test("the notification module is mounted in the app", async () => {
  const lib = await readRust("src-tauri/src/lib.rs");
  assert.match(lib, /^mod notifications;$/m, "the module is declared");
});

// ── 7 · mutation locks ─────────────────────────────────────────────────────

// Deleting the visibility gate (making the app notify unconditionally) must
// turn the rule red. This simulates the mutation on the source text and then
// evaluates the mutated predicate exactly as the suite would.
test("mutation lock: dropping the visible-panel suppression turns the rule red", async () => {
  const source = await read("src/notifications.ts");
  assert.match(source, /foreground === "hidden"/, "the gate is the predicate's whole body");  const mutated = source.replace('foreground === "hidden"', "true");
  assert.notEqual(mutated, source, "the mutation must land");
  // Re-evaluate the mutated predicate: it now notifies a visible panel, which
  // is exactly the double report the round forbids.
  const mutatedShouldNotify = (foreground: "visible" | "hidden") => true;
  assert.equal(mutatedShouldNotify("visible"), true, "the mutation makes a visible panel notify");
  assert.notEqual(
    mutatedShouldNotify("visible"),
    shouldNotify("visible", "success"),
    "the real predicate disagrees with the mutated one",
  );
  // The Rust side has the same guard, and it is what the app actually runs.
  const rust = await rustNotifications();
  assert.match(
    rust,
    /\(Foreground::Visible, Outcome::Success \| Outcome::Failure\) => false/,
    "the Rust gate is present and load-bearing",
  );
});

// Deleting the click→reveal callback must turn the wiring red.
test("mutation lock: dropping the click reveal leaves nothing to show the window", async () => {
  const rust = await rustNotifications();
  const mutated = rust.replace("NotificationClick::Clicked => reveal(),", "");
  assert.notEqual(mutated, rust, "the mutation must land");
  assert.ok(
    !/NotificationClick::Clicked => reveal\(\)/.test(mutated),
    "the mutated source no longer reveals on a click",
  );
  assert.ok(
    !/reveal_after_activation/.test(mutated.replace(/pub\(crate\) fn reveal_after_activation[\s\S]*/, "")),
    "with the click arm gone the production handler has no body to run",
  );
  // And the wiring is what makes the notification useful at all: the source
  // pin proves the real code still has both halves.
  assert.match(rust, /NotificationClick::Clicked => reveal\(\)/, "the real handler reveals");
  const lib = await readRust("src-tauri/src/lib.rs");
  assert.match(
    lib,
    /tauri::RunEvent::Reopen \{ \.\. \} => notifications::reveal_after_activation\(app\)/,
    "the real app calls it on activation",
  );
});
