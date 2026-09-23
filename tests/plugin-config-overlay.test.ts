// R33 · the plugin configuration overlay's mount contract.
//
// Until R33 a plugin's configuration lived in a sandboxed iframe, kept alive
// above every mode branch so it survived mode switches. That host is retired:
// the configuration is now the launcher's own `PluginConfigOverlay`, rendered
// inside the collapsed card from the plugin's declarative schema. This file
// replaced `plugin-page-persistence.test.ts` — the old host-persistence
// assertions have no subject any more.
//
// What must stay true:
//   * no plugin-page host is mounted anywhere (the iframe path is gone);
//   * the overlay is gated on an explicit `pluginConfigOpen` AND a plugin
//     scope, so it can never float over the ordinary app list;
//   * the unified entry (`openPluginConfig`) writes mode, plugin scope and
//     overlay-open in one place, and leaving the scope closes the overlay.
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

const root = new URL("../", import.meta.url);
const read = (path: string) => readFile(new URL(path, root), "utf8");

test("no plugin-page host is mounted in the app any more", async () => {
  const app = await read("src/App.tsx");
  assert.equal(
    (app.match(/<PluginPageHost\b/g) ?? []).length,
    0,
    "the sandboxed iframe host must not be mounted (R33 retired the page)",
  );
  assert.equal(
    (app.match(/\{pluginLayer\}/g) ?? []).length,
    0,
    "the persistent plugin layer is gone with the host",
  );
  assert.ok(
    !app.includes("PluginPageHost"),
    "App must not even import the retired host",
  );
});

test("the configuration overlay renders inside the collapsed branch only", async () => {
  const app = await read("src/App.tsx");
  const overlay = app.indexOf("<PluginConfigOverlay");
  assert.ok(overlay > -1, "the overlay must be rendered somewhere");
  // The nearest mode branch above the overlay must be the collapsed one.
  const before = app.slice(0, overlay);
  const anchors = [
    'if (mode === "collapsed")',
    'if (mode === "terminal")',
    'if (mode === "settings")',
  ].map((needle) => before.lastIndexOf(needle));
  const nearest = Math.max(...anchors);
  assert.equal(
    app.slice(nearest, nearest + 'if (mode === "collapsed")'.length),
    'if (mode === "collapsed")',
    "the overlay must live in the collapsed branch, not beside the mode shell",
  );
  // It is mounted exactly once.
  assert.equal((app.match(/<PluginConfigOverlay\b/g) ?? []).length, 1);
});

test("the overlay is gated on both the open flag and a live plugin scope", async () => {
  const app = await read("src/App.tsx");
  assert.match(
    app,
    /\{pluginConfigOpen && launcherPluginId \?/,
    "the overlay must require the open flag AND a plugin scope",
  );
});

test("openPluginConfig writes the collapsed mode, the plugin mode and the flag", async () => {
  const app = await read("src/App.tsx");
  const start = app.indexOf("const openPluginConfig");
  assert.ok(start > -1, "openPluginConfig must exist");
  const body = app.slice(start, app.indexOf("\n  }, [enterPluginMode]);", start));
  assert.match(body, /setMode\("collapsed"\)/, "it must leave for the launcher surface");
  assert.match(body, /enterPluginMode\(/, "it must enter the plugin's own mode");
  assert.match(body, /setPluginConfigOpen\(true\)/, "it must open the overlay");
  // The browser refusal keeps every trigger honest.
  assert.match(body, /browserPluginEnabledRef\.current/, "a disabled browser plugin must refuse");
  assert.match(body, /settings\.browserDisabled/, "the refusal must name the reason");
});

test("leaving the plugin scope closes the overlay", async () => {
  const app = await read("src/App.tsx");
  assert.match(
    app,
    /if \(!launcherPluginId\) setPluginConfigOpen\(false\);/,
    "a scope that leaves must take its overlay with it",
  );
  assert.match(
    app,
    /if \(mode !== "collapsed"\) setPluginConfigOpen\(false\);/,
    "leaving the collapsed surface must close the overlay too",
  );
});

test("the overlay is closed by the first Esc level", async () => {
  const app = await read("src/App.tsx");
  const start = app.indexOf("const onLauncherDismiss");
  const body = app.slice(start, app.indexOf("\n  },", start));
  const overlayClose = body.indexOf("if (pluginConfigOpen)");
  const modeExit = body.indexOf("if (pluginModeRef.current)");
  assert.ok(overlayClose > -1, "Esc must test the overlay first");
  assert.ok(
    overlayClose < modeExit,
    "the overlay closes before the plugin mode is left (R31's three levels)",
  );
  assert.match(body, /setPluginConfigOpen\(false\)/);
});

test("the plugin request listener maps the hotkey toggle onto the overlay", async () => {
  const app = await read("src/App.tsx");
  const start = app.indexOf('listen<{ id: string; toggle: boolean }>');
  assert.ok(start > -1, "the plugin request listener must exist");
  const body = app.slice(start, app.indexOf("}, []);", start));
  assert.match(body, /pluginConfigOpenRef\.current/, "the toggle must see the live flag");
  assert.match(body, /launcherPluginIdRef\.current === id/, "and only close the same plugin's overlay");
  assert.match(body, /setPluginConfigOpen\(false\)/, "a second press closes the overlay");
  assert.match(body, /openPluginConfig\(id\)/, "any other request opens it");
});

test("the cold-start request lands on the overlay too", async () => {
  const app = await read("src/App.tsx");
  const start = app.indexOf('invoke<string | null>("take_pending_plugin_page")');
  assert.ok(start > -1, "the pending request must still be consumed");
  const body = app.slice(start, app.indexOf("}, []);", start));
  assert.match(body, /openPluginConfig\(pending\)/, "it must use the unified entry");
});

test("the toast host still leads every mode tree", async () => {
  const app = await read("src/App.tsx");
  const tails = app.split("{toastHost}").slice(1);
  assert.equal(tails.length, 3, "toastHost must be rendered in all three modes");
  for (const tail of tails) {
    assert.match(
      tail,
      /^\s*<div className="(?:settings|collapsed|terminal)-shell">/,
      "the toast host must be the first sibling, before the mode shell",
    );
  }
});
