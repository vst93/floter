// R150 · the application icon's light/dark switch.
//
// The user's ask: 「增加深浅色应用图标的切换逻辑，在设置中进行切换，默认深色（黑色）」
// — a Settings choice between the black and white marks, shipping dark. The
// round has four moving parts and this suite pins one face of each:
//
//   1. **Rust settings shape.** The field is `app_icon`, it carries an explicit
//      `default_app_icon`, and the save path installs it through
//      `apply_app_icon`. A settings file written before the key existed must
//      still come back `"dark"` — that is the migration lock.
//   2. **The contract.** A stored value maps onto a shipped appearance, and an
//      unknown/hand-edited value lands on the default. Both sides encode it
//      (`normalize_app_icon` in Rust, `normalizeAppIconAppearance` in
//      `src/settings/app-icon.ts`) and the mutation locks replay the inversion
//      the round is most likely to regress to.
//   3. **The native application.** `apply_app_icon` installs the decoded PNG on
//      the tray and the main window, and the two embedded assets really are two
//      different PNGs.
//   4. **The UI.** GeneralPage renders the picker from the shared segmented
//      language, wired to `changeGeneralSetting` — not to a private command.
//
// There is no live `AppHandle` in `node --test`, so the application assertions
// are over the source — the same shape `menubar-icon` and `notifications` use
// for their platform-only branches.
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

import { createTranslator } from "../src/i18n.ts";
import {
  APP_ICON_APPEARANCES,
  APP_ICON_LABEL_KEYS,
  DEFAULT_APP_ICON,
  appIconLabelKey,
  normalizeAppIconAppearance,
} from "../src/settings/app-icon.ts";

const root = new URL("../", import.meta.url);
const read = (path: string) => readFile(new URL(path, root), "utf8");
const readBytes = (path: string) => readFile(new URL(path, root));

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

test("the settings shape carries app_icon with an explicit dark default", async () => {
  const rust = await read("src-tauri/src/commands/config.rs");
  assert.match(
    rust,
    /#\[serde\(default = "default_app_icon"\)\]\s*\n\s*pub app_icon: String,/,
    "the field must be `app_icon: String` with `default_app_icon`",
  );
  // The named function is what makes the migration callable from a test — a
  // bare `#[serde(default)]` would be `String::default()` (`""`), which names
  // no appearance.
  const defaultFn = fnBody(rust, "pub fn default_app_icon() -> String");
  assert.match(
    defaultFn,
    /crate::app_icon::DEFAULT_APP_ICON\.to_string\(\)/,
    "default_app_icon must return the module's shipped constant",
  );
  // Both live in `AppSettings`: `Default` for the in-process case and the serde
  // attribute for the on-disk case. Missing either one is a different bug.
  assert.match(rust, /app_icon: default_app_icon\(\)/, "AppSettings::default must agree");
});

test("the shipped default is the black mark the earlier builds used", async () => {
  const module = await read("src-tauri/src/app_icon.rs");
  assert.match(module, /pub const DEFAULT_APP_ICON: &str = "dark";/, "the default is dark");
  assert.match(module, /pub const APP_ICON_APPEARANCES: \[&str; 2\] = \["dark", "light"\];/);
});

test("the save path applies the setting, and retitling never touches it", async () => {
  const config = await read("src-tauri/src/commands/config.rs");
  const save = fnBody(config, "pub fn save_settings(app: tauri::AppHandle, settings: AppSettings)");
  assert.match(
    save,
    /crate::apply_app_icon\(&app, &settings\.app_icon\)/,
    "save_settings must apply the icon appearance",
  );
  // The red line: the language call stays and only retitles, so a language
  // change can never reset the icon.
  const lib = await read("src-tauri/src/lib.rs");
  const language = fnBody(lib, "pub fn apply_tray_language(app: &AppHandle, language: &str)");
  assert.ok(!/set_icon/.test(language), "retitling must not set an icon");
  assert.ok(!/apply_app_icon/.test(language), "retitling must not reinstall the icon");
  // And the two are separate calls in the save path.
  assert.match(save, /apply_tray_language/, "the save path still retitles");
});

// ── 2 · the contract, in both languages ────────────────────────────────────

test("the frontend vocabulary is the Rust vocabulary", async () => {
  const module = await read("src-tauri/src/app_icon.rs");
  // The two appearances and the default are declared on both sides; a drift
  // would let the picker offer a value normalization cannot resolve.
  assert.match(module, /"dark"/, "Rust knows dark");
  assert.match(module, /"light"/, "Rust knows light");
  assert.deepEqual([...APP_ICON_APPEARANCES], ["dark", "light"]);
  assert.equal(DEFAULT_APP_ICON, "dark");
});

test("normalization passes shipped values through and defaults the rest", () => {
  assert.equal(normalizeAppIconAppearance("dark"), "dark");
  assert.equal(normalizeAppIconAppearance("light"), "light");
  for (const unknown of ["", "Dark", "LIGHT", "auto", "system", "midnight", undefined, null]) {
    assert.equal(normalizeAppIconAppearance(unknown), DEFAULT_APP_ICON, `${unknown}`);
  }
});

test("every appearance has a spelled label key and the picker resolves it", () => {
  // A dynamic `settings.appIcon.${value}` would leave the keys with no static
  // consumer; the record spells them.
  assert.deepEqual(APP_ICON_LABEL_KEYS, {
    dark: "settings.appIcon.dark",
    light: "settings.appIcon.light",
  });
  assert.equal(appIconLabelKey("dark"), "settings.appIcon.dark");
  assert.equal(appIconLabelKey("light"), "settings.appIcon.light");
  assert.equal(appIconLabelKey("nonsense"), "settings.appIcon.dark");
});

// ── 3 · the native application ─────────────────────────────────────────────

test("apply_app_icon installs the image on the tray and the main window", async () => {
  const lib = await read("src-tauri/src/lib.rs");
  const apply = fnBody(lib, "pub fn apply_app_icon(app: &AppHandle, appearance: &str)");
  assert.match(apply, /app_icon::image_for\(appearance\)/, "it decodes the named appearance");
  assert.match(apply, /tray_by_id\(tray_identity::TRAY_ICON_ID\)/, "it addresses the tray by id");
  assert.match(apply, /tray\.set_icon\(/, "it replaces the tray icon");
  assert.match(apply, /get_webview_window\("main"\)/, "it addresses the main window");
  assert.match(apply, /window\.set_icon\(/, "it replaces the window icon");
  // The startup path applies it right after the tray is built, so the chosen
  // variant is in place before the frontend hydrates.
  assert.match(
    lib,
    /apply_tray_visibility\(app\.handle\(\), settings\.show_menubar_icon\);[\s\S]*?apply_app_icon\(app\.handle\(\), &settings\.app_icon\);/,
    "startup must install the stored appearance",
  );
});

test("both embedded assets exist, are PNGs, and are different images", async () => {
  const signature = Buffer.from([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a]);
  const dark = await readBytes("src-tauri/icons/icon-dark.png");
  const light = await readBytes("src-tauri/icons/icon-light.png");
  for (const [name, bytes] of [
    ["icon-dark.png", dark],
    ["icon-light.png", light],
  ] as const) {
    assert.ok(bytes.subarray(0, 8).equals(signature), `${name} must be a PNG`);
  }
  assert.ok(!dark.equals(light), "the two appearances must not share one image");
  // The module embeds both, by name.
  const module = await read("src-tauri/src/app_icon.rs");
  assert.match(module, /include_bytes!\("\.\.\/icons\/icon-dark\.png"\)/);
  assert.match(module, /include_bytes!\("\.\.\/icons\/icon-light\.png"\)/);
});

test("the image feature the decoder needs is enabled", async () => {
  // `Image::from_bytes` only exists with tauri's `image-png` feature; without
  // it the module does not compile, but a guard that names it catches a
  // manifest edit that silently drops it.
  const manifest = await read("src-tauri/Cargo.toml");
  assert.match(manifest, /tauri = \{[^}]*"image-png"/, "tauri must keep the image-png feature");
});

// ── 4 · the UI row ─────────────────────────────────────────────────────────

test("GeneralPage renders the picker from the shared segmented language", async () => {
  const page = await read("src/settings/GeneralPage.tsx");
  assert.match(page, /t\("settings\.appIcon"\)/, "the row is labelled from i18n");
  assert.match(page, /t\("settings\.appIconHint"\)/, "the row carries the hint");
  assert.match(
    page,
    /onChangeGeneralSetting\("app_icon", value\)/,
    "the click goes through the shared mutator",
  );
  assert.match(page, /<SegmentedChoice/, "the control is the shipped segmented picker");
  assert.match(page, /normalizeAppIconAppearance\(settings\.app_icon\)/, "the value is normalized");
  assert.match(page, /APP_ICON_APPEARANCES\.map/, "the options come from the vocabulary");
  // The setting rides the ordinary `save_settings` snapshot (the backend
  // applies it), so there is exactly one persistence path.
  assert.ok(
    !/invoke\(\s*"(set_app_icon|set_icon|set_dock_icon)/.test(page),
    "the picker must not add a dedicated command",
  );
});

test("one key name crosses the bridge: page → snapshot → Rust field", async () => {
  const field = "app_icon";
  const page = await read("src/settings/GeneralPage.tsx");
  const hook = await read("src/hooks/useSettings.ts");
  const app = await read("src/App.tsx");
  const rust = await read("src-tauri/src/commands/config.rs");
  assert.ok(page.includes(`"${field}"`), "the page writes the field by name");
  assert.match(hook, /app_icon:\s*DEFAULT_APP_ICON/, "the frontend default uses the field");
  assert.match(hook, /app_icon:\s*normalizeAppIconAppearance\(loaded\.app_icon/, "hydration backfills");
  assert.match(app, /app_icon:\s*AppIconAppearance;/, "the TS type carries the field");
  assert.match(rust, new RegExp(`pub ${field}: String,`), "the Rust struct uses the field");
});

// ── 5 · i18n symmetry ──────────────────────────────────────────────────────

test("the four new keys are declared in both dictionaries and are real Chinese", () => {
  const keys = [
    "settings.appIcon",
    "settings.appIconHint",
    "settings.appIcon.dark",
    "settings.appIcon.light",
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
  assert.equal(zh("settings.appIcon"), "应用图标");
  assert.equal(zh("settings.appIcon.dark"), "深色");
  assert.equal(zh("settings.appIcon.light"), "浅色");
});

// ── 6 · mutation locks ─────────────────────────────────────────────────────

/** The predicate the migration test really asserts: serde fills a missing
 *  `app_icon` with the module's shipped constant. Replaying the mutation (the
 *  attribute dropped to a bare `#[serde(default)]`) must fail it. */
const migrationDefaultsToDark = (rust: string) => {
  const field = rust.match(
    /(#\[serde\(default = "default_app_icon"\)\])\s*\n\s*pub app_icon: String,/,
  );
  if (!field) return false;
  return fnBody(rust, "pub fn default_app_icon() -> String").includes(
    "crate::app_icon::DEFAULT_APP_ICON",
  );
};

test("mutation: defaulting the field to an empty string breaks the migration lock", async () => {
  const rust = await read("src-tauri/src/commands/config.rs");
  assert.ok(migrationDefaultsToDark(rust), "the shipped field must default to dark");
  const demoted = rust.replace(
    /#\[serde\(default = "default_app_icon"\)\](\s*\n\s*pub app_icon: String,)/,
    "#[serde(default)]$1",
  );
  assert.notEqual(demoted, rust, "the demotion must land");
  assert.ok(!migrationDefaultsToDark(demoted), "a bare serde default must fail the lock");
});

/** The predicate the contract test asserts: an unknown value resolves to the
 *  default. The mutation "the first appearance wins" is what a `.find()`
 *  without a fallback regresses to. */
const unknownFallsBackToTheDefault = (source: string) =>
  /\.unwrap_or\(DEFAULT_APP_ICON\)/.test(
    fnBody(source, "pub fn normalize_app_icon(value: &str) -> &'static str"),
  );

test("mutation: letting an unknown value win goes red", async () => {
  const module = await read("src-tauri/src/app_icon.rs");
  assert.ok(unknownFallsBackToTheDefault(module), "the shipped contract falls back to dark");
  const mutated = module.replace(
    /\.unwrap_or\(DEFAULT_APP_ICON\)/,
    '.unwrap_or("light")',
  );
  assert.notEqual(mutated, module, "the mutation must land");
  assert.ok(!unknownFallsBackToTheDefault(mutated), "a non-default fallback must fail the contract");
});

test("mutation: a private write path for the icon goes red", async () => {
  // The round's structural rule is that the picker rides `save_settings`. The
  // predicate fails if the page grows its own command — the shape a "quick
  // fix" takes when the icon is thought of as a native call rather than a
  // setting.
  const page = await read("src/settings/GeneralPage.tsx");
  const dedicatedCommand = /invoke\(\s*"(set_app_icon|set_dock_icon)"/;
  assert.ok(!dedicatedCommand.test(page), "the shipped page has one write path");
  assert.ok(
    dedicatedCommand.test(`${page}\n  void invoke("set_app_icon", { appearance });\n`),
    "a dedicated command must be detectable by this predicate",
  );
});
