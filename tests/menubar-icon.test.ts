// R7-10c · the menu bar / tray icon switch.
//
// The user's ask: 「设置里加一个开关『显示菜单栏图标』」 — a switch that hides the
// macOS status item (and the Windows/Linux tray icon) without weakening any
// existing summon path. The round has three moving parts and this suite pins
// one face of each:
//
//   1. **Rust settings shape.** The field is `show_menubar_icon`, it carries an
//      explicit `default_true`, and the save path applies it through
//      `apply_tray_visibility`. A settings file written before the key existed
//      must still come back visible — that is the migration lock.
//   2. **The contract.** The status item is visible iff the setting is on.
//      Both sides encode it (`desired_tray_visibility` in Rust,
//      `menubarIconVisible` in `src/settings/menubar-icon.ts`) and the mutation
//      locks replay the two inversions this round is most likely to regress to.
//   3. **The UI.** GeneralPage renders the switch from the shared card/row
//      language, wired to `changeGeneralSetting` — not to a private handler
//      that writes its own key.
//
// The retitle path is asserted *structurally*: `apply_tray_language` may only
// touch menu labels, so a language change can never reset visibility. There is
// no live `AppHandle` in `node --test`, so the assertion is over the source —
// which is the same shape `notifications.rs` and `deep_link.rs` use for their
// platform-only branches.
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

import { createTranslator } from "../src/i18n.ts";
import {
  menubarIconSwitchState,
  menubarIconVisible,
  toggleMenubarIcon,
} from "../src/settings/menubar-icon.ts";

const root = new URL("../", import.meta.url);
const read = (path: string) => readFile(new URL(path, root), "utf8");

/** The body of a top-level `pub fn`/`fn`, sliced to the next top-level item. */
const fnBody = (source: string, signature: string) => {
  const at = source.indexOf(signature);
  assert.notEqual(at, -1, `${signature} must exist`);
  const rest = source.slice(at + signature.length);
  const end = rest.search(/\n\}\n/);
  assert.notEqual(end, -1, `${signature} must be a braced function`);
  return rest.slice(0, end);
};

// ── 1 · the setting, and the migration ─────────────────────────────────────

test("the settings shape carries show_menubar_icon with an explicit true default", async () => {
  const rust = await read("src-tauri/src/commands/config.rs");
  assert.match(
    rust,
    /#\[serde\(default = "default_true"\)\]\s*\n\s*pub show_menubar_icon: bool,/,
    "the field must be `show_menubar_icon: bool` with `default_true`",
  );
  // The named function is what makes the migration callable from a test — a
  // closure or a bare `#[serde(default)]` would both be `false`.
  const defaultTrue = fnBody(rust, "pub fn default_true() -> bool");
  assert.match(defaultTrue, /^\s*true\s*$/m, "default_true must return true");
  // Both live in `AppSettings`: `Default` for the in-process case and the serde
  // attribute for the on-disk case. Missing either one is a different bug.
  assert.match(rust, /show_menubar_icon: default_true\(\)/, "AppSettings::default must agree");
});

test("the save path applies the setting, and retitling never touches visibility", async () => {
  const config = await read("src-tauri/src/commands/config.rs");
  const save = fnBody(config, "pub fn save_settings(app: tauri::AppHandle, settings: AppSettings)");
  assert.match(
    save,
    /crate::apply_tray_visibility\(&app, settings\.show_menubar_icon\)/,
    "save_settings must apply the icon visibility",
  );

  // The red line: the language call stays and only retitles. If a future edit
  // moved visibility into `apply_tray_language`, this fails.
  const lib = await read("src-tauri/src/lib.rs");
  const language = fnBody(lib, "pub fn apply_tray_language(app: &AppHandle, language: &str)");
  assert.ok(!/set_visible/.test(language), "retitling must not set visibility");
  assert.match(language, /set_text/, "retitling writes labels");

  // And the two are separate calls in the save path, so neither can be an
  // accidental side effect of the other.
  assert.match(save, /apply_tray_language/, "the save path still retitles");
});

test("the visual contract is a value, not a side effect", async () => {
  const lib = await read("src-tauri/src/lib.rs");
  const desired = fnBody(lib, "pub fn desired_tray_visibility(show_icon: bool) -> bool");
  assert.match(desired, /^\s*show_icon\s*$/m, "visibility must be the setting itself");

  // The startup path applies it right after the tray is built, so a hidden
  // icon stays hidden across a restart without a second source of truth.
  assert.match(
    lib,
    /\.build\(app\)\?;\s*\n\s*\/\/[^\n]*\n(?:\s*\/\/[^\n]*\n)*\s*apply_tray_visibility\(app\.handle\(\), settings\.show_menubar_icon\);/,
    "startup must apply the stored visibility",
  );
});

// ── 2 · the contract, in the frontend ──────────────────────────────────────

test("the switch inverts the stored value and the contract maps it straight through", () => {
  assert.equal(toggleMenubarIcon(true), false, "clicking an on switch turns it off");
  assert.equal(toggleMenubarIcon(false), true, "clicking an off switch turns it on");
  assert.equal(menubarIconVisible(true), true, "the setting IS the visibility");
  assert.equal(menubarIconVisible(false), false);
});

test("the rendered switch state follows the visible value on both channels", () => {
  // The class and `aria-checked` must agree: a lit track that reports
  // `aria-checked=false` is the a11y face of the inversion bug.
  assert.deepEqual(menubarIconSwitchState(true), { active: true, ariaChecked: true });
  assert.deepEqual(menubarIconSwitchState(false), { active: false, ariaChecked: false });
});

test("the switch ships on, so an upgraded install is unchanged", async () => {
  // Both defaults — the pre-hydration frontend snapshot and the Rust struct —
  // have to be `true` or an existing user's icon disappears on first launch.
  const hook = await read("src/hooks/useSettings.ts");
  const defaults = hook.slice(hook.indexOf("const SETTINGS_DEFAULTS"), hook.indexOf("};", hook.indexOf("const SETTINGS_DEFAULTS")));
  assert.match(defaults, /show_menubar_icon:\s*true/, "the frontend default must be on");
  // Hydration also backfills the field for a response that predates it.
  assert.match(hook, /show_menubar_icon:\s*loaded\.show_menubar_icon \?\? true/, "hydration backfills on");
  const app = await read("src/App.tsx");
  assert.match(app, /show_menubar_icon: boolean;/, "the AppSettings type carries the field");
});

test("one key name crosses the bridge: page → snapshot → Rust field", async () => {
  // The save path is the ordinary `save_settings` snapshot: the page calls the
  // generic mutator, which goes through `persistSettings` →
  // `invoke("save_settings", …)`, and the Rust side reads the same key. A
  // typo on any one side would drop the user's choice silently, so the four
  // spellings are asserted equal here rather than left to a live click.
  const field = "show_menubar_icon";
  const page = await read("src/settings/GeneralPage.tsx");
  const hook = await read("src/hooks/useSettings.ts");
  const app = await read("src/App.tsx");
  const rust = await read("src-tauri/src/commands/config.rs");
  assert.ok(page.includes(`"${field}"`), "the page writes the field by name");
  assert.ok(hook.includes(`${field}: true`), "the frontend default uses the field");
  assert.ok(app.includes(`${field}: boolean`), "the TS type uses the field");
  assert.match(rust, new RegExp(`pub ${field}: bool,`), "the Rust struct uses the field");

  // The mutator really is the persistence path: it calls `persistSettings`,
  // which is the single serialized writer over `invoke("save_settings")`.
  const mutator = hook.slice(hook.indexOf("const changeGeneralSetting = useCallback"));
  assert.match(mutator.slice(0, mutator.indexOf("});")), /persistSettings\(\)/, "the mutator persists");
  assert.match(hook, /invoke\("save_settings", \{ settings: next \}\)/, "persistence is the settings command");
});

// ── 3 · the UI row ─────────────────────────────────────────────────────────

test("GeneralPage renders the switch from the shared row language", async () => {
  const page = await read("src/settings/GeneralPage.tsx");
  // A shared row, an accent-budgeted switch, and the generic mutator that
  // persists any field — no private handler, no second write path.
  assert.match(page, /t\("settings\.showMenubarIcon"\)/, "the row is labelled from i18n");
  assert.match(page, /t\("settings\.showMenubarIconHint"\)/, "the row carries the hint");
  assert.match(
    page,
    /onChangeGeneralSetting\("show_menubar_icon", toggleMenubarIcon\(settings\.show_menubar_icon\)\)/,
    "the click goes through the shared mutator with the inverted value",
  );
  assert.match(page, /menubarIconSwitchState\(settings\.show_menubar_icon\)\.ariaChecked/, "aria-checked reads the contract");
  assert.match(page, /role="switch"/, "the control is a switch");
  // The switch face is the census-counted one, not a new accent fill.
  assert.match(page, /settings-switch--active/, "it reuses the shipped switch face");
  // And it is wrapped in the card the round's design language requires.
  assert.ok(page.includes("settings.group.menuBar"), "the row sits in its own titled group");
});

test("the icon switch does not spend a new accent face or a native channel", async () => {
  const page = await read("src/settings/GeneralPage.tsx");
  // No bespoke invoke: the setting rides the ordinary `save_settings` snapshot
  // (the backend applies it), so there is exactly one persistence path.
  assert.ok(
    !/invoke\(\s*"(set_tray|show_tray|hide_tray|set_menubar)/.test(page),
    "the switch must not add a dedicated command",
  );
  // And no platform branch in the label: one string covers both platforms.
  const i18n = await read("src/i18n.ts");
  const en = i18n.slice(i18n.indexOf('"settings.showMenubarIcon":'), i18n.indexOf('"settings.showMenubarIconHint":'));
  assert.ok(!/macos|macOS|darwin|process\.platform|navigator/.test(en), "the label has no platform branch");
});

// ── 4 · i18n symmetry ──────────────────────────────────────────────────────

test("the three new keys are declared in both dictionaries and are real Chinese", () => {
  const keys = [
    "settings.group.menuBar",
    "settings.showMenubarIcon",
    "settings.showMenubarIconHint",
  ] as const;
  const en = createTranslator("en");
  const zh = createTranslator("zh");
  for (const key of keys) {
    const english = en(key);
    const chinese = zh(key);
    assert.ok(english.length > 0, `${key} must have en text`);
    assert.ok(chinese.length > 0, `${key} must have zh text`);
    assert.notEqual(chinese, english, `${key} must be translated, not an English fallback`);
    assert.ok(/[\u4e00-\u9fff]/.test(chinese), `${key} must contain Chinese text`);
  }
  // The user's own wording, kept verbatim.
  assert.equal(zh("settings.showMenubarIcon"), "显示菜单栏图标");
  assert.equal(en("settings.showMenubarIcon"), "Show menu bar icon");
});

// ── 5 · mutation locks ─────────────────────────────────────────────────────

/** The predicate the migration test really asserts: serde fills a missing
 *  `show_menubar_icon` with `true`. Replaying the mutation (the attribute
 *  dropped to a bare `#[serde(default)]`) must fail it. */
const migrationDefaultsToTrue = (rust: string) => {
  const field = rust.match(
    /(#\[serde\(default = "default_true"\)\])\s*\n\s*pub show_menubar_icon: bool,/,
  );
  if (!field) return false;
  return fnBody(rust, "pub fn default_true() -> bool").includes("true");
};

test("mutation: defaulting the field to false breaks the migration lock", async () => {
  const rust = await read("src-tauri/src/commands/config.rs");
  assert.ok(migrationDefaultsToTrue(rust), "the shipped field must default to true");
  // The two ways the mutation lands: the attribute demoted, or the function
  // flipped. Both are the same bug for an upgrading user.
  const demoted = rust.replace(
    /#\[serde\(default = "default_true"\)\](\s*\n\s*pub show_menubar_icon: bool,)/,
    "#[serde(default)]$1",
  );
  assert.notEqual(demoted, rust, "the demotion must land");
  assert.ok(!migrationDefaultsToTrue(demoted), "a bare serde default must fail the lock");

  const flipped = rust.replace(
    /(pub fn default_true\(\) -> bool \{\s*\n\s*)true/,
    "$1false",
  );
  assert.notEqual(flipped, rust, "the flip must land");
  assert.ok(!migrationDefaultsToTrue(flipped), "default_true = false must fail the lock");
});

/** The predicate the contract test asserts: visibility equals the setting.
 *  The inversion is the mutation "the icon shows when the switch is off". */
const visibilityFollowsTheSetting = (source: string, signature: string) =>
  /^\s*show_icon\s*$/m.test(fnBody(source, signature));

test("mutation: inverting the visibility contract goes red", async () => {
  const lib = await read("src-tauri/src/lib.rs");
  const signature = "pub fn desired_tray_visibility(show_icon: bool) -> bool";
  assert.ok(visibilityFollowsTheSetting(lib, signature), "the shipped contract must be the identity");
  const mutated = lib.replace(
    /(pub fn desired_tray_visibility\(show_icon: bool\) -> bool \{\s*\n\s*)show_icon/,
    "$1!show_icon",
  );
  assert.notEqual(mutated, lib, "the inversion must land");
  assert.ok(!visibilityFollowsTheSetting(mutated, signature), "`!show_icon` must fail the contract");

  // The same inversion on the frontend side — the switch persisting the
  // *current* value instead of its opposite is the UI face of the bug.
  const source = await read("src/settings/menubar-icon.ts");
  const body = fnBody(source, "export function toggleMenubarIcon(current: boolean): boolean");
  assert.match(body, /return !current;/, "the shipped toggle inverts");
  assert.ok(!/return !current;/.test(body.replace("return !current;", "return current;")), "a non-inverting toggle fails");
});

test("mutation: a private write path for the icon goes red", async () => {
  // The round's structural rule is that the switch rides `save_settings`. The
  // predicate fails if the page grows its own command — the shape a "quick
  // fix" takes when visibility is thought of as a native call rather than a
  // setting.
  const page = await read("src/settings/GeneralPage.tsx");
  const dedicatedCommand = /invoke\(\s*"(set_tray_visibility|show_tray_icon|hide_tray_icon)"/;
  assert.ok(!dedicatedCommand.test(page), "the shipped page has one write path");
  assert.ok(
    dedicatedCommand.test(`${page}\n  void invoke("set_tray_visibility", { visible });\n`),
    "a dedicated command must be detectable by this predicate",
  );
});
