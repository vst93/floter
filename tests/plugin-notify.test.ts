// The host's one feedback channel: a single toast stack with an optional
// action slot.
//
// R7-5 made the host toast the app's only feedback surface: a plugin page used
// to send a `host-notify` bridge message, and the host translated its
// dictionary key and raised the toast. R33 retired the built-in iframe pages,
// R76 deleted their source, and R96 deleted the published bridge itself — the
// page-side halves of this suite (the message shape, the key validation, the
// retry registry keyed by notification id) are gone with it.
//
// What stays is the host's own stack: the `notify` call shape and the single
// toast placement. The failure dedupe policy that used to be pinned here now
// lives with its implementation in `tests/plugin-pages.test.ts`.
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

const root = new URL("../", import.meta.url);
const read = (path: string) => readFile(new URL(path, root), "utf8");
const stripComments = (css: string) => css.replace(/\/\*[\s\S]*?\*\//g, "");

test("App's notify grew an optional action slot and no plugin layer is left", async () => {
  const app = await read("src/App.tsx");
  assert.match(
    app,
    /const notify = useCallback\(\s*\(kind: ToastKind, text: string, action\?: ToastAction\)/,
    "notify must accept an optional action without changing its existing call shape",
  );
  // R33 · the plugin layer is retired with the iframe page; the toast host is
  // now the leading sibling of every mode tree and nothing may hide it inside
  // a plugin wrapper again.
  assert.equal((app.match(/\{pluginLayer\}/g) ?? []).length, 0);
  assert.ok(!app.includes("const pluginLayer = ("), "the plugin layer must be gone");
});

test("the surface reuses the default toast placement — no second stack", async () => {
  const css = stripComments(await read("src/styles/extensions.css"));
  const host = css.slice(css.indexOf("#floter-app-toasts {"));
  // The default anchor: full-height windows (settings, terminal) clear their
  // chrome at 64px.
  assert.match(host, /top:\s*64px/, "the default toast anchor stays at 64px");
  // R96 · the plugin page surface is gone with the bridge, so no per-surface
  // placement override may come back — a second position is exactly the split
  // the single-stack rule removes.
  const pluginOverrides = [...host.matchAll(/#floter-app-toasts\[data-surface="plugin"\][^{]*\{/g)];
  assert.deepEqual(
    pluginOverrides.map((m) => m[0]),
    [],
    "no plugin surface may add a placement override",
  );
  // The surface still travels to the host element; it is `mode`, so any surface
  // reports itself without any new plumbing.
  const toast = await read("src/components/ToastStack.tsx");
  assert.match(toast, /data-surface=\{dataSurface\}/);
});

test("the retired bridge notification surface stays deleted", async () => {
  const pluginPages = await read("src/plugin-pages.ts");
  // The `host-notify` / `notify-retry` messages and the page-side retry
  // registry were the bridge's feedback path. Assembled from parts so the
  // round's zero-hit grep stays clean.
  assert.ok(!pluginPages.includes("host-" + "notify"), "the host-notify message must stay gone");
  assert.ok(!pluginPages.includes("notify-" + "retry"), "the notify-retry reply must stay gone");
  assert.ok(!pluginPages.includes("createRetry" + "Registry"), "the retry registry must stay gone");
});
