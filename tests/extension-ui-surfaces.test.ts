import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

const root = new URL("../", import.meta.url);
const read = (path: string) => readFile(new URL(path, root), "utf8");

// The clipboard history is a shipped surface: its settings toggle defaults to
// on, the panel is reachable from launcher search and `floter clip`, and
// terminal copy/paste rides on the same arboard-backed commands. It must ship
// in the default build — the optional feature exists only so a slim build can
// opt out explicitly.
test("clipboard-history is part of the default cargo feature set", async () => {
  const cargo = await read("src-tauri/Cargo.toml");
  assert.match(
    cargo,
    /^\s*default\s*=\s*\[[^\]]*clipboard-history[^\]]*\]/m,
    "the default feature set must include clipboard-history",
  );
});

// Every overlay the extensions panel renders must have styling; a modal class
// with no rule anywhere drops the dialog into normal document flow (no dim,
// no centering, no card material).
test("every extensions modal class has a CSS rule", async () => {
  const css = [
    await read("src/styles/extensions.css"),
    await read("src/extensions/ComponentizedUninstallDialog.css"),
  ].join("\n");
  for (const className of [
    "extensions-dialog-overlay",
    "extensions-dialog",
    "extensions-dialog-title",
    "extensions-dialog-hint",
    "extensions-dialog-actions",
  ]) {
    assert.ok(
      css.includes(`.${className}`),
      `missing CSS rule for .${className}`,
    );
  }
});

// The in-row operation progress strip is rendered by ExtensionRow; without a
// rule it becomes an unstyled flex item that breaks the row's grid columns.
test("operation progress strip and advisory notice have CSS rules", async () => {
  const css = await read("src/styles/extensions.css");
  assert.ok(css.includes(".extension-row__progress"), "missing .extension-row__progress");
  assert.ok(css.includes(".extensions-notice--warning"), "missing .extensions-notice--warning");
});

// Toasts must be pinned to the card (fixed, via a portal host outside the page
// scroller), never inside the scrollable settings content where they drift off
// screen as the user scrolls a long integrations list.
test("toast stack is a fixed, card-level surface", async () => {
  const css = await read("src/styles/extensions.css");
  const host = css.slice(css.indexOf("#floter-app-toasts"));
  assert.match(host, /position:\s*fixed/, "toast host must be position: fixed");
  for (const className of [".app-toast", ".app-toast--error", ".app-toast--success"]) {
    assert.ok(css.includes(className), `missing ${className}`);
  }
});

// The clipboard page's empty/failure state must not reuse the generic
// "Plugin failed to load" copy: when the backend is unavailable (feature off)
// the page has to say so and how to turn it back on, rather than implying the
// page itself failed to load.
test("clipboard page reports an unavailable backend distinctly", async () => {
  const page = await read("src/plugins/clipboard/main.ts");
  assert.ok(
    page.includes("clipboard.pageUnavailable"),
    "clipboard page must use the backend-unavailable message",
  );
  assert.ok(
    page.includes("clipboard.loadFailed"),
    "clipboard page must use the load-failed message",
  );
  assert.ok(
    !page.includes('t("plugin.pageError")'),
    "clipboard page must not claim the whole plugin failed to load",
  );
});
