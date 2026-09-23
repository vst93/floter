// R41 · two of the round's smaller promises, pinned together because both are
// "one control, one place" rules:
//
//   1. Hyprland (Wayland): the launcher must open as a floating, centered
//      window rather than a tiled full-screen one, and `hide_on_blur` must
//      default **off** there while staying **on** everywhere else. The user's
//      words: 「在 hyprland 桌面下启动时，默认并不是居中的悬浮窗……让它始终
//      保持悬浮. 而且如果检查到是这个系统 应该默认关闭失焦隐藏这个逻辑 其他情况下
//      还是默认开启」.
//
//   2. The plugin configuration overlay's close control collapses onto the
//      gear: while the overlay is open the field row's trailing button becomes
//      an X and closes it; the overlay itself carries no second close button.
//      The user's words: 「当打开设置时，该配置按钮最好能自动变成"关闭"的 icon
//      这样保持统一 也不用在配置页面单独再加一个关闭」.
//
// The detection is a pure function of `HYPRLAND_INSTANCE_SIGNATURE`, so the
// matrix below drives it directly; the wiring is pinned at the source, the same
// way the other Rust-touching suites pin a function the node runner cannot
// execute.

import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

const root = new URL("../", import.meta.url);
const read = (path: string) => readFile(new URL(path, root), "utf8");
const stripJsComments = (src: string) =>
  src.replace(/\/\*[\s\S]*?\*\//g, "").replace(/\/\/[^\n]*/g, "");
// Rust `//` / `///` / `//!` line comments and `/* … */` blocks, so a negative
// assertion tests the code and not the prose that explains it.
const stripRustComments = (src: string) =>
  src.replace(/\/\*[\s\S]*?\*\//g, "").replace(/\/\/[^\n]*/g, "");

// ── 1 · Hyprland detection and the blur default ───────────────────────────

test("only a non-empty HYPRLAND_INSTANCE_SIGNATURE means Hyprland", async () => {
  const rust = await read("src-tauri/src/hyprland.rs");
  assert.match(
    rust,
    /pub const HYPRLAND_SIGNATURE_ENV: &str = "HYPRLAND_INSTANCE_SIGNATURE"/,
    "the one variable Hyprland exports is the one read",
  );
  assert.match(
    rust,
    /pub fn hyprland_signature_detected\(signature: Option<&str>\) -> bool \{\s*signature\.is_some_and\(\|value\| !value\.trim\(\)\.is_empty\(\)\)\s*\}/,
    "detection is a pure function of the signature value",
  );
  // A missing or blank value is not a Hyprland session; an empty export would
  // otherwise make every machine look like one.
  assert.match(rust, /signature\.is_some_and/);
  // Only Hyprland is special-cased: no sway / niri / generic Wayland guess.
  assert.doesNotMatch(rust, /SWAYSOCK|NIRI_SOCKET|XDG_SESSION_TYPE/);
});

test("hide_on_blur defaults off on Hyprland and on everywhere else", async () => {
  const rust = await read("src-tauri/src/hyprland.rs");
  assert.match(
    rust,
    /pub fn default_hide_on_blur_for\(signature: Option<&str>\) -> bool \{\s*!hyprland_signature_detected\(signature\)\s*\}/,
    "the default is the negation of the detection",
  );
  // The settings default reads the live environment through that function, not
  // a hard-coded `true`.
  const config = await read("src-tauri/src/commands/config.rs");
  assert.match(
    config,
    /hide_on_blur: crate::hyprland::default_hide_on_blur\(\),/,
    "a fresh settings file is born from the environment-aware default",
  );
  // An explicit stored key still wins: the default only feeds serde's fallback.
  assert.match(config, /pub hide_on_blur: bool,/);
});

// ── 2 · the floating window ───────────────────────────────────────────────

test("Hyprland floats the active window with a set dispatcher, before geometry", async () => {
  const rust = await read("src-tauri/src/hyprland.rs");
  // `setfloating` is idempotent; `togglefloating` would flip an already-floating
  // panel back into the tiling layout on the second reveal.
  assert.match(rust, /&\["dispatch", "setfloating"\]/);
  // The dispatch table itself is checked before the `#[cfg(test)]` block, so
  // the test's own `togglefloating` guard does not read as a real dispatcher.
  const dispatchCode = stripRustComments(rust).split("#[cfg(test)]")[0];
  assert.doesNotMatch(dispatchCode, /togglefloating/);
  assert.match(rust, /&\["dispatch", "centerwindow"\]/);
  assert.match(
    rust,
    /pub fn ensure_floating\(\) \{\s*if !hyprland_detected\(\) \{\s*return;\s*\}/,
    "the dispatcher is a no-op outside Hyprland",
  );

  const lib = await read("src-tauri/src/lib.rs");
  assert.match(lib, /mod hyprland;/, "the module is declared");
  // The call sits inside `reveal_window`, after `show()` + `set_focus()`, so the
  // launcher is the active window when the dispatcher runs.
  const reveal = lib.slice(
    lib.indexOf("fn reveal_window("),
    lib.indexOf("fn reveal_saved_mode("),
  );
  assert.match(reveal, /window\.show\(\)/);
  assert.match(reveal, /window\.set_focus\(\)/);
  assert.match(reveal, /#\[cfg\(target_os = "linux"\)\]\s*hyprland::ensure_floating\(\);/);
  assert.ok(
    reveal.indexOf("window.set_focus()") < reveal.indexOf("hyprland::ensure_floating()"),
    "the float is requested after focus, so it lands on our window",
  );
});

// ── 3 · the gear becomes the overlay's close control ──────────────────────

test("the plugin gear flips to an X while the overlay is open", async () => {
  const app = stripJsComments(await read("src/App.tsx"));
  assert.match(
    app,
    /aria-label=\{t\(pluginConfigOpen \? "plugins\.config\.close" : "plugins\.config\.open"\)\}/,
    "the button's accessible name follows its state",
  );
  assert.match(
    app,
    /title=\{t\(pluginConfigOpen \? "plugins\.config\.close" : "plugins\.config\.openHint"\)\}/,
  );
  assert.match(app, /aria-expanded=\{pluginConfigOpen\}/);
  // Both glyphs are present and selected by the same state.
  assert.match(
    app,
    /\{pluginConfigOpen \? \(\s*<X size=\{16\}/,
    "the open state draws the X",
  );
  assert.match(
    app,
    /\) : \(\s*<SlidersHorizontal size=\{16\}/,
    "the closed state keeps the gear",
  );
  // The click still toggles the one open-state owner.
  assert.match(app, /setPluginConfigOpen\(\(open\) => !open\)/);
});

test("the overlay carries no close button of its own", async () => {
  const overlay = stripJsComments(await read("src/plugins/PluginConfigOverlay.tsx"));
  assert.doesNotMatch(overlay, /plugin-config__close/, "the overlay's close button is gone");
  assert.doesNotMatch(overlay, /onClose/, "the now-unused close prop is gone too");
  // The overlay's header keeps the title and nothing else.
  assert.match(
    overlay,
    /<div className="plugin-config__header">\s*<span className="plugin-config__title">/,
  );
  const css = await read("src/styles/plugin-config.css");
  assert.doesNotMatch(css, /\.plugin-config__close/, "the dead close-button rules are gone");
});

test("Esc / Cmd+W still close the overlay first — the gear is parallel, not a replacement", async () => {
  const app = stripJsComments(await read("src/App.tsx"));
  const dismiss = app.slice(
    app.indexOf("const onLauncherDismiss = useCallback("),
    app.indexOf("const {", app.indexOf("const onLauncherDismiss = useCallback(")),
  );
  assert.match(
    dismiss,
    /if \(pluginConfigOpen\) \{\s*event\.preventDefault\(\);\s*setPluginConfigOpen\(false\);\s*return true;/,
    "the first dismiss level still closes the overlay",
  );
});

// ── 4 · the copy exists in both languages ─────────────────────────────────

test("the close copy is declared in both dictionaries", async () => {
  const source = await read("src/i18n.ts");
  for (const key of ["plugins.config.open", "plugins.config.close"]) {
    const occurrences = source.split(`"${key}"`).length - 1;
    assert.equal(occurrences, 2, `${key} must be declared in both dictionaries`);
  }
});
