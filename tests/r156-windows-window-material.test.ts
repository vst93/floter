// R156 · the Windows window material, and the runtime probe that goes with it.
//
// R154 turned the three shells' `backdrop-filter` off on Windows and the user's
// retest came back "还是不行，没变化" — so the loss was never the filter's. What
// R154 left standing is the window-level alpha path: tao makes the window
// per-pixel transparent (`DwmEnableBlurBehindWindow` with an *empty* blur
// region, tao 0.37.1 `platform_impl/windows/window.rs:1203-1221`) and wry asks
// WebView2 for a transparent default background (wry 0.57.0
// `webview2/mod.rs:127-132`, `:458-465`, `set_background_color` at
// `:1894-1912`), and on the runtimes tracked by WebView2Feedback#5481 / #5752
// the webview paints a base of its own instead of being a hole. Every link this
// tree owns is correct, so R156 stops trying to make the webview transparent
// and puts a material behind it instead: `apply_windows_window_material` in
// `src-tauri/src/lib.rs` applies the OS window effects (acrylic as the floor,
// mica on top where Windows 11 can draw it) so the CSS alpha — the
// transparency slider's only output — composites over a fixed base rather than
// over raw desktop.
//
// This file locks the four things that make that sentence true, all of them
// source-scan (the gate has no Windows runtime; the report carries the retest
// matrix for what only the user can see):
//
//   1. the material is applied only from `#[cfg(target_os = "windows")]` code,
//      so macOS and Linux window creation is byte-for-byte what it was;
//   2. the effects are applied acrylic-then-mica, the one order that needs no
//      failure probe (the second call overwrites the first exactly where the
//      machine can draw it, and the mica call is the one that fails on
//      Windows 10, leaving the acrylic);
//   3. the material adds no corner shape and no colour/alpha of its own — the
//      CSS radius and the transparency slider stay the only sources of both;
//   4. the shared `tauri.conf.json` carries no `windowEffects`, which is what
//      keeps the material from reaching macOS through the shared window config.
//
// Plus the probe: `parseWebViewRuntime` is the one thing the next retest has to
// quote, and it must stay silent on the platforms that have no WebView2.
//
// Mutations that must turn this red (all run, see the report):
//   * swapping the effect order to mica-then-acrylic -> (2);
//   * deleting the call from `configure_windows_frame` -> (1);
//   * adding a `windowEffects` block to tauri.conf.json -> (4);
//   * making the probe match `Safari/` too -> the WKWebView case.
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

import { formatWebViewRuntime, parseWebViewRuntime } from "../src/webview-version.ts";

const root = new URL("../", import.meta.url);
const read = (path: string) => readFile(new URL(path, root), "utf8");

/** A Rust `fn`'s body, by name, brace-counted so a nested block cannot cut it
 *  short. Returns the body *including* the surrounding braces. */
const rustFunction = (source: string, name: string): string => {
  const start = source.indexOf(`fn ${name}(`);
  assert.notEqual(start, -1, `src-tauri/src/lib.rs must define fn ${name}`);
  const open = source.indexOf("{", start);
  let depth = 0;
  for (let i = open; i < source.length; i += 1) {
    if (source[i] === "{") depth += 1;
    else if (source[i] === "}") {
      depth -= 1;
      if (depth === 0) return source.slice(start, i + 1);
    }
  }
  throw new Error(`fn ${name} is not brace-balanced`);
};

// ── 1 · the material is Windows-only ──────────────────────────────────────

test("the window material is applied only from Windows-gated code", async () => {
  const lib = await read("src-tauri/src/lib.rs");
  const material = rustFunction(lib, "apply_windows_window_material");

  // The definition itself carries the cfg: without it the function would not
  // even compile on macOS/Linux (`tauri::window::Effect` is Windows/macOS, and
  // the whole point is that this round does not touch those platforms).
  const beforeDefinition = lib.slice(0, lib.indexOf("fn apply_windows_window_material("));
  const guard = beforeDefinition.lastIndexOf("#[cfg(");
  assert.notEqual(guard, -1, "the material function must carry a cfg attribute");
  assert.match(
    beforeDefinition.slice(guard, guard + 40),
    /#\[cfg\(target_os = "windows"\)\]/,
    "the material function must be gated on Windows",
  );

  // And it is reached from the existing Windows frame configuration, which is
  // the one Windows-only entry point setup and reveal already share.
  const frame = rustFunction(lib, "configure_windows_frame");
  assert.match(
    frame,
    /apply_windows_window_material\(window\)/,
    "configure_windows_frame must apply the window material",
  );
});

// ── 2 · acrylic first, mica second ────────────────────────────────────────

test("the material lays acrylic down first and mica on top", async () => {
  const lib = await read("src-tauri/src/lib.rs");
  const material = rustFunction(lib, "apply_windows_window_material");

  const acrylic = material.indexOf("Effect::Acrylic");
  const mica = material.indexOf("Effect::Mica");
  assert.notEqual(acrylic, -1, "the material must ask for acrylic");
  assert.notEqual(mica, -1, "the material must ask for mica");
  assert.ok(
    acrylic < mica,
    "acrylic must be applied before mica: the mica call is the one that fails on Windows 10, so the acrylic has to already be there",
  );

  // One `set_effects` per effect, both through the builder — a hand-written
  // `WindowEffectsConfig` literal would be a second place the effect list lives.
  assert.equal(
    (material.match(/set_effects\(/g) ?? []).length,
    1,
    "the material must apply its effects through the one set_effects call in the loop",
  );
});

// ── 3 · the material adds no corner shape, no colour, no alpha ────────────

test("the material leaves the corner shape and the material axes alone", async () => {
  const lib = await read("src-tauri/src/lib.rs");
  const material = rustFunction(lib, "apply_windows_window_material");

  // DWM rounding stays off (`configure_windows_frame` owns it) so the CSS
  // radius remains the only corner shape; the material may only ever show
  // where the app paints nothing.
  for (const forbidden of [
    "DWMWCP_ROUND",
    "DWMWA_WINDOW_CORNER_PREFERENCE",
    "DWMWA_SYSTEMBACKDROP_TYPE",
    "Color(",
    "opacity",
    "alpha",
  ]) {
    assert.ok(
      !material.includes(forbidden),
      `the window material must not carry ${forbidden} — the corner shape is CSS's and the slider is the only alpha`,
    );
  }
});

// ── 4 · the shared window config stays material-free ──────────────────────

test("the shared tauri.conf.json carries no windowEffects", async () => {
  const conf = JSON.parse(await read("src-tauri/tauri.conf.json")) as {
    app?: { windows?: { label?: string; windowEffects?: unknown }[] };
  };
  const windows = conf.app?.windows ?? [];
  assert.ok(windows.length > 0, "the app must declare its window");
  for (const window of windows) {
    assert.equal(
      window.windowEffects,
      undefined,
      `${window.label ?? "a window"} must not carry windowEffects: a shared config would reach macOS, and the material is applied from the Windows-gated Rust path instead`,
    );
  }
  // The transparency the material is a floor for is still the window's own.
  for (const window of windows) {
    assert.equal(
      (window as { transparent?: boolean }).transparent,
      true,
      "the window must stay transparent — the material is a base under the CSS alpha, not a replacement for it",
    );
  }
});

// ── the probe ─────────────────────────────────────────────────────────────

test("the runtime probe reads the WebView2 version and stays silent elsewhere", () => {
  // The shape WebView2 actually ships: Edge's user agent with the same
  // Chromium build beside it. The Edge number is the runtime number.
  const webview2 =
    "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) " +
    "Chrome/154.0.4258.62 Safari/537.36 Edg/154.0.4258.62";
  assert.deepEqual(parseWebViewRuntime(webview2), {
    runtime: "WebView2",
    version: "154.0.4258.62",
  });

  // A Chromium without the Edge token is still worth a version.
  const chromium =
    "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) " +
    "Chrome/148.0.7778.178 Safari/537.36";
  assert.deepEqual(parseWebViewRuntime(chromium), {
    runtime: "Chromium",
    version: "148.0.7778.178",
  });

  // The two engines this app runs on outside Windows identify themselves as
  // Safari, and the probe must return nothing there: the row is cross-platform
  // harmless by being absent.
  const wkWebView =
    "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko)";
  const webKitGtk =
    "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/605.1.15 (KHTML, like Gecko) " +
    "Version/17.0 Safari/605.1.15";
  assert.equal(parseWebViewRuntime(wkWebView), null);
  assert.equal(parseWebViewRuntime(webKitGtk), null);

  // The label the row prints is engine + version, in that order.
  assert.equal(
    formatWebViewRuntime({ runtime: "WebView2", version: "154.0.4258.62" }),
    "WebView2 154.0.4258.62",
  );
});

test("the About page reports the probe and the dictionaries carry its row", async () => {
  const page = await read("src/settings/AboutPage.tsx");
  assert.match(page, /parseWebViewRuntime\(navigator\.userAgent\)/, "the About page must read the engine version");
  assert.match(page, /formatWebViewRuntime\(webViewRuntime\)/, "the About page must print it");
  assert.match(page, /settings\.group\.runtime/, "the row lives in its own labelled section");
  assert.match(
    page,
    /webViewRuntime \?/,
    "the row must be conditional — a null parse (WKWebView, WebKitGTK) draws nothing",
  );

  const i18n = await read("src/i18n.ts");
  for (const key of [
    "settings.group.runtime",
    "settings.webViewVersion",
    "settings.webViewVersionHint",
  ]) {
    // Twice: the English table is the source of truth and the Chinese one has
    // to carry every key (`tests/i18n-symmetry.test.ts` enforces the set, this
    // pins that the row's keys are in it at all).
    assert.equal(
      (i18n.match(new RegExp(`"${key.replace(/\./g, "\\.")}"`, "g")) ?? []).length,
      2,
      `${key} must exist in both dictionaries`,
    );
  }
});
