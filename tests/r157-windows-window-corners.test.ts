// R157 · the Windows window's own corner, now that R156's material made it
// visible.
//
// R7-HIG turned DWM rounding off (`DWMWCP_DONOTROUND`, commit `8054de8`) so CSS
// was the sole source of the corner shape: the card filled the window edge and
// DWM rounded that same boundary at the system's 8px while the card rounded it
// at its own figure, so the two radii disagreed along one edge. R156's material
// changed the premise — the gutter the shell leaves around its card now shows
// material — and a material surface that stops at the window's raw square
// corner, around a rounded card, is the "square outside, round inside" the user
// saw. R157 therefore rounds the window again (`DWMWCP_ROUND`, the ~8px preset
// that is already the number the Windows card uses) in a step of its own,
// immediately after the material.
//
// What this file locks, all of it source-scan (no Windows runtime on the gate):
//
//   1. the old contract is *replaced*, not duplicated — `DWMWCP_DONOTROUND`
//      appears nowhere in lib.rs and the corner preference has exactly one
//      site, inside the R157 helper;
//   2. the helper runs right after `apply_windows_window_material`, so the
//      corner it rounds is the one the material filled;
//   3. the helper is Windows-gated and swallows the DWM failure, so Windows 10
//      (no such attribute) keeps its square corner and window creation is never
//      broken by the call;
//   4. the CSS card is untouched this round — `14px` default, `8px` on Windows,
//      the same 8px `DWMWCP_ROUND` draws, and the outer round cannot clip the
//      card (the 10u settings/terminal gutter is outside DWM's 8px edge band,
//      and the 4u collapsed-shell gutter keeps its corner arc inside DWM's);
//   5. the detached plugin window is *not* on this path and must not be: it is
//      a decorated, opaque window (tao defaults; `WindowBuilder::new` never
//      reads tauri.conf.json), so it never had the transparent-webview problem
//      the material exists for. If that ever changes, this guard says so.
//
// Mutations that must turn this red (all run, see the report):
//   * `DWMWCP_ROUND` -> `DWMWCP_DONOTROUND` in the helper -> (1);
//   * deleting the `round_windows_window_material_corners(window)?` call from
//     `configure_windows_frame` -> (1) and (2);
//   * moving the call above `apply_windows_window_material` -> (2);
//   * letting the DWM error out of the helper (`?` instead of `let _ =`) -> (3);
//   * `8px` -> `18px` in the `.platform-windows` block -> (4);
//   * `.transparent(true)` on the detached builder -> (5).
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

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

/** The text immediately above a `fn` definition, back to the previous blank
 *  line after the preceding item — enough to see the `#[cfg(...)]` guard. */
const attributesAbove = (source: string, name: string): string => {
  const start = source.indexOf(`fn ${name}(`);
  assert.notEqual(start, -1, `src-tauri/src/lib.rs must define fn ${name}`);
  return source.slice(0, start);
};

/** One selector's declaration block from a stylesheet, by exact selector text. */
const cssBlock = (css: string, selector: string): string => {
  const start = css.indexOf(`${selector} {`);
  assert.notEqual(start, -1, `base.css must carry a \`${selector}\` block`);
  const open = css.indexOf("{", start);
  const close = css.indexOf("}", open);
  return css.slice(start, close + 1);
};

// ── 1 · the old contract is replaced, not duplicated ──────────────────────

test("the corner preference has one site and it is DWMWCP_ROUND", async () => {
  const lib = await read("src-tauri/src/lib.rs");

  assert.ok(
    !lib.includes("DWMWCP_DONOTROUND"),
    "R7-HIG's DWMWCP_DONOTROUND must be gone: R157 replaces that contract rather than leaving a second, contradictory corner decision beside it",
  );
  const helper = rustFunction(lib, "round_windows_window_material_corners");
  // Exactly one *site*: the attribute is named nowhere else in lib.rs (not in
  // `configure_windows_frame`, not beside the material).
  assert.ok(
    !lib.replace(helper, "").includes("DWMWA_WINDOW_CORNER_PREFERENCE"),
    "the corner preference must be decided from exactly one place — the R157 helper — not also from configure_windows_frame",
  );
  assert.match(helper, /DWMWCP_ROUND\b/, "the window must round with DWMWCP_ROUND");
  assert.ok(
    !helper.includes("DWMWCP_ROUNDSMALL"),
    "ROUNDSMALL (~4px) would leave the outer edge squarer than the card inside it — ROUND is the preset that matches the card's 8px",
  );

  // The R156 material function stays shape-free: the corner decision lives in
  // its own step, which is what keeps the two concerns separable.
  const material = rustFunction(lib, "apply_windows_window_material");
  assert.ok(
    !material.includes("DWMWCP_ROUND"),
    "the material function must not round: the corner is its own step (see tests/r156-windows-window-material.test.ts)",
  );
});

// ── 2 · the corner step runs right after the material ─────────────────────

test("the corner is rounded immediately after the material fills the gutter", async () => {
  const lib = await read("src-tauri/src/lib.rs");
  const frame = rustFunction(lib, "configure_windows_frame");

  const materialCall = frame.indexOf("apply_windows_window_material(window)");
  const cornerCall = frame.indexOf("round_windows_window_material_corners(window)");
  assert.notEqual(materialCall, -1, "configure_windows_frame must apply the material");
  assert.notEqual(
    cornerCall,
    -1,
    "configure_windows_frame must round the window's corner — the material is what makes that corner visible",
  );
  assert.ok(
    materialCall < cornerCall,
    "the material must be applied before the corner is rounded: the rounding exists because the material fills the gutter",
  );
});

// ── 3 · Windows-gated, and Windows 10 tolerant ────────────────────────────

test("the corner call is Windows-only and swallows the Windows 10 failure", async () => {
  const lib = await read("src-tauri/src/lib.rs");

  const above = attributesAbove(lib, "round_windows_window_material_corners");
  const guard = above.lastIndexOf("#[cfg(");
  assert.notEqual(guard, -1, "the corner helper must carry a cfg attribute");
  assert.match(
    above.slice(guard, guard + 40),
    /#\[cfg\(target_os = "windows"\)\]/,
    "the corner helper must be gated on Windows — macOS and Linux window creation is untouched",
  );

  const helper = rustFunction(lib, "round_windows_window_material_corners");
  // Windows 10 has no DWMWA_WINDOW_CORNER_PREFERENCE; the DWM call fails there.
  // The failure must be discarded exactly like the frame path's other DWM
  // calls, or window creation breaks on a platform that is merely older.
  assert.match(
    helper,
    /let _ = DwmSetWindowAttribute\(/,
    "the DWM corner call must discard its error (Windows 10 predates the attribute), never `?` it into window creation",
  );
  assert.ok(
    !/DwmSetWindowAttribute\([^)]*\?/.test(helper),
    "the corner helper must not propagate the DWM error",
  );
});

// ── 4 · the CSS card is untouched, and its radius is the DWM figure ───────

test("the CSS card keeps its radii, and Windows' 8px is the DWM round figure", async () => {
  const css = await read("src/styles/base.css");

  // The default radius is unchanged (macOS/Linux): R157 is Windows-only.
  const rootBlock = cssBlock(css, ":root");
  assert.match(
    rootBlock,
    /--window-radius: 14px;/,
    "the default --window-radius stays 14px — this round does not touch the non-Windows card",
  );

  // Windows rounds to the system figure, which is also DWMWCP_ROUND's ~8px, so
  // the outer and inner radii are the same number rather than a nested pair.
  const windowsBlock = cssBlock(css, ".platform-windows");
  assert.match(
    windowsBlock,
    /--window-radius: 8px;/,
    "the Windows card must stay 8px: it is the same figure DWMWCP_ROUND draws, which is why the two radii cannot leave a seam",
  );

  // The 8px is the figure DWM itself draws, and the outer round cannot clip the
  // card. DWM cuts only the 8px band along each edge, so a card the shell
  // insets by at least 8px sits wholly outside that band; a thinner gutter
  // keeps the card's own corner arc inside DWM's because both arcs are 8px (the
  // conservative bound for that side — the card's corner point at `inset`
  // against a hypothetical zero-radius corner — is inset·√2 ≤ 8).
  const gutters = [
    [".platform-windows .collapsed-shell", "collapsed shell"],
    [".platform-windows .settings-shell,\n.platform-windows .terminal-shell", "settings/terminal shell"],
  ] as const;
  for (const [selector, name] of gutters) {
    const inset = /calc\(var\(--u\) \* (\d+)\)/.exec(cssBlock(css, selector));
    assert.notEqual(inset, null, `the Windows ${name} must still inset its card by a u-scaled padding`);
    const px = Number(inset![1]);
    assert.ok(
      px >= 8 || px * Math.SQRT2 <= 8,
      `the Windows ${name} insets its card by ${px}px, which is on neither safe side of DWM's 8px corner band`,
    );
  }
});

// ── 5 · the detached plugin window is deliberately off this path ──────────

test("the detached plugin window stays a decorated, opaque window", async () => {
  const lib = await read("src-tauri/src/lib.rs");
  const detach = rustFunction(lib, "detach_plugin_window");

  // The builder is the runtime path (`WebviewWindowBuilder::new`), which reads
  // tao's defaults — decorations on, transparent off — and never
  // tauri.conf.json. That is what keeps the window out of the class the R156
  // material exists for: it paints its own opaque background, so the
  // transparent-webview regression cannot reach it, and DWM already rounds its
  // native frame.
  for (const forbidden of [
    ".transparent(",
    ".decorations(",
    "configure_windows_frame",
    "apply_windows_window_material",
    "round_windows_window_material_corners",
    "windowEffects",
  ]) {
    assert.ok(
      !detach.includes(forbidden),
      `the detached window must not carry ${forbidden}: it is a decorated, opaque window, and putting it on the transparent/material path is a change this guard exists to make visible`,
    );
  }

  // The two halves of that claim, from the files that own them: only `main` is
  // declared (so the detached window cannot inherit a transparent config), and
  // the detached surface is opaque and has no CSS radius of its own.
  const conf = JSON.parse(await read("src-tauri/tauri.conf.json")) as {
    app?: { windows?: { label?: string }[] };
  };
  assert.deepEqual(
    (conf.app?.windows ?? []).map((window) => window.label),
    ["main"],
    "only `main` may be declared: a config entry for plugin-detached would give the detached window transparency it was never built for",
  );

  const launcher = await read("src/styles/launcher.css");
  const surface = cssBlock(launcher, ".plugin-window");
  assert.match(
    surface,
    /background: var\(--surface-opaque\)/,
    "the detached surface must stay opaque — it is a real window, not the launcher's floating card",
  );
  assert.ok(
    !surface.includes("--window-radius"),
    "the detached window root must carry no CSS radius: its frame is native, so there is no CSS/DWM double-radius to reconcile",
  );
});

// ── the contract texts the round registers ────────────────────────────────

test("the material model and the redundant clear say what R156/R157 changed", async () => {
  // glass-material.ts is the paper contract for the window-alpha path; R157
  // adds the note that a Windows material now sits *under* it, without changing
  // a line of logic.
  const material = await read("src/glass-material.ts");
  assert.match(
    material,
    /apply_windows_window_material/,
    "glass-material.ts must name the Windows material base (R156) in its contract text",
  );
  assert.match(
    material,
    /round_windows_window_material_corners/,
    "glass-material.ts must name the R157 corner step beside it",
  );

  // lib.rs's redundant `set_background_color` stays (defence against a wry
  // change), and now says so where it is read.
  const lib = await read("src-tauri/src/lib.rs");
  const clear = lib.slice(lib.indexOf("set_background_color(Some(Color(0, 0, 0, 0)))") - 1200);
  assert.match(
    clear.slice(0, 1200),
    /[Rr]edundant/,
    "the redundant window-background clear must carry its reason (wry already does it; kept on purpose)",
  );
});
