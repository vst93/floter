// R84 · the detached plugin window's contracts, pinned where a review could
// silently break them.
//
// Four halves, one file:
//
// * the *label* contract — `PLUGIN_WINDOW_LABEL` is the literal the Rust
//   builder, the capability file and the render branch in `main.tsx` all name.
//   A one-sided rename breaks the feature at runtime (the window opens onto
//   the full launcher, or the capability misses the window), so the label and
//   the capability file's own window list are read and pinned here. R91 · the
//   first instance keeps that literal; later instances are suffixed and the
//   capability covers them with a glob, which the same tests read.
// * the *validation* contract — `validateDetachRequest` refuses anything
//   without the routing truth (a non-empty command id) and anything whose
//   args are not a string list. The backend's `DetachRequest::valid` mirrors
//   the convention; this side is what the tests can reach.
// * the *blur* contract — the one rule the feature exists for: the launcher
//   card hides on blur when the setting says so; the detached window NEVER
//   does, whatever the setting says (the user pinned it precisely so it would
//   not go away).
// * the *language* contract — the new chrome keys exist in both message
//   tables; a missing zh key would fall back to English at runtime and the
//   type system cannot see it from the detached surface alone.

import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

import { createTranslator, isMessageKey } from "../src/i18n.ts";
import {
  hideOnBlurApplies,
  isPluginWindowLabel,
  MAIN_WINDOW_LABEL,
  PLUGIN_WINDOW_LABEL,
  validateDetachRequest,
} from "../src/plugin-window/detach.ts";

const validRequest = {
  kind: "external",
  extensionId: "local.tool",
  commandId: "run",
  commandLabel: "My Tool",
  args: ["--flag", "value"],
};

test("the detached window label is the literal every side names", () => {
  // The Rust test `the_detached_label_is_the_literal_every_side_names` pins
  // the same literal on its side of the boundary.
  assert.equal(PLUGIN_WINDOW_LABEL, "plugin-detached");
  assert.equal(MAIN_WINDOW_LABEL, "main");
  assert.notEqual(PLUGIN_WINDOW_LABEL, MAIN_WINDOW_LABEL);
});

test("the capability file governs exactly the detached windows", async () => {
  const capability = JSON.parse(
    await readFile(
      new URL("../src-tauri/capabilities/plugin-detached.json", import.meta.url),
      "utf8",
    ),
  );
  // R91 · the capability governs the first instance by its exact literal and
  // every later instance by the `plugin-detached-*` glob. Set-equal, not
  // order-equal: the list is a set of matchers.
  assert.deepEqual(
    new Set(capability.windows),
    new Set([PLUGIN_WINDOW_LABEL, `${PLUGIN_WINDOW_LABEL}-*`]),
  );
  // The window's own close button and its event listener are the two powers
  // it needs; `core:default` carries the rest of the core surface.
  assert.ok(capability.permissions.includes("core:event:default"));
  assert.ok(capability.permissions.includes("core:window:allow-close"));
});

test("the render branch recognises every detached instance label", async () => {
  // The first instance is the R84 literal; instances two and up are suffixed.
  assert.equal(isPluginWindowLabel(PLUGIN_WINDOW_LABEL), true);
  assert.equal(isPluginWindowLabel(`${PLUGIN_WINDOW_LABEL}-2`), true);
  assert.equal(isPluginWindowLabel(`${PLUGIN_WINDOW_LABEL}-17`), true);
  // The launcher and anything else stay on the App branch.
  assert.equal(isPluginWindowLabel(MAIN_WINDOW_LABEL), false);
  assert.equal(isPluginWindowLabel("something-else"), false);
  assert.equal(isPluginWindowLabel(""), false);

  // The wiring: main.tsx branches on the helper, not on a bare equality that
  // window two would fail.
  const main = await readFile(new URL("../src/main.tsx", import.meta.url), "utf8");
  assert.match(main, /isPluginWindowLabel\(label\)/);
  assert.doesNotMatch(main, /label === PLUGIN_WINDOW_LABEL/);
});

test("the detached view names its own label when it pulls and closes", async () => {
  // R91 · the slot and the close are per label now, so the page has to say
  // which window it is. A call that forgets `label` would read (or drop) the
  // wrong window's request.
  const view = await readFile(
    new URL("../src/plugin-window/DetachedPluginApp.tsx", import.meta.url),
    "utf8",
  );
  assert.match(view, /invoke<unknown>\("take_plugin_window_request", \{ label \}\)/);
  assert.match(
    view,
    /invoke\("close_plugin_window", \{ label: getCurrentWindow\(\)\.label \}\)/,
  );
});

test("validateDetachRequest accepts the launcher's well-formed request", () => {
  assert.deepEqual(validateDetachRequest(validRequest), validRequest);
  // An empty argv is a legitimate run (the command may need no arguments).
  assert.deepEqual(
    validateDetachRequest({ ...validRequest, args: [] }),
    { ...validRequest, args: [] },
  );
});

test("validateDetachRequest refuses anything without its routing truth", () => {
  assert.equal(validateDetachRequest(null), null);
  assert.equal(validateDetachRequest("run"), null);
  assert.equal(validateDetachRequest({}), null);
  // R90 · the shape is a discriminated union now: an untagged payload — the
  // pre-R90 external request — is no arm at all, and an unknown kind is
  // refused rather than guessed at.
  const { kind: _kind, ...untagged } = validRequest;
  assert.equal(validateDetachRequest(untagged), null);
  assert.equal(validateDetachRequest({ ...validRequest, kind: "nope" }), null);
  assert.equal(validateDetachRequest({ ...validRequest, commandId: "" }), null);
  assert.equal(validateDetachRequest({ ...validRequest, commandId: "   " }), null);
  assert.equal(validateDetachRequest({ ...validRequest, extensionId: "" }), null);
  assert.equal(validateDetachRequest({ ...validRequest, commandLabel: "" }), null);
  assert.equal(validateDetachRequest({ ...validRequest, args: ["ok", 3] }), null);
  assert.equal(validateDetachRequest({ ...validRequest, args: "--flag" }), null);
});

test("validateDetachRequest accepts the text arm, an empty body included", () => {
  // R90 · a single snapshot of text: the title names the window, the body is
  // the content. An empty body is a legitimate snapshot — a command that
  // printed nothing is still content the user pinned — so it is accepted, not
  // treated as malformed.
  assert.deepEqual(validateDetachRequest({ kind: "text", title: "Note", text: "hello" }), {
    kind: "text",
    title: "Note",
    text: "hello",
  });
  assert.deepEqual(validateDetachRequest({ kind: "text", title: "Note", text: "" }), {
    kind: "text",
    title: "Note",
    text: "",
  });
});

test("validateDetachRequest refuses a text arm without a title or a body", () => {
  assert.equal(validateDetachRequest({ kind: "text", title: "", text: "hello" }), null);
  assert.equal(validateDetachRequest({ kind: "text", title: "   ", text: "hello" }), null);
  assert.equal(validateDetachRequest({ kind: "text", title: "Note" }), null);
  assert.equal(validateDetachRequest({ kind: "text", title: "Note", text: 3 }), null);
  assert.equal(validateDetachRequest({ kind: "text", title: "Note", text: null }), null);
});

test("hide on blur keeps governing the launcher card exactly when the setting is on", () => {
  assert.equal(hideOnBlurApplies(MAIN_WINDOW_LABEL, true), true);
  assert.equal(hideOnBlurApplies(MAIN_WINDOW_LABEL, false), false);
});

test("the detached window never hides on blur, whatever the setting says", () => {
  // The core ask (「独立之后……不再跟随呼出和隐藏」): the pin is a lie if a
  // blur can take the window away. Both settings, same answer.
  assert.equal(hideOnBlurApplies(PLUGIN_WINDOW_LABEL, true), false);
  assert.equal(hideOnBlurApplies(PLUGIN_WINDOW_LABEL, false), false);
  // Defensive default: an unknown window earns no hiding either.
  assert.equal(hideOnBlurApplies("something-else", true), false);
});

test("the detached window's chrome keys exist and translate in both languages", () => {
  // The i18n-symmetry suite already pins zh ↔ en key symmetry for the whole
  // table; here the new family is resolved through the real translator in both
  // languages — a missing key would fall back to the other table and read as
  // the wrong language, which is exactly what this catches.
  const keys = [
    "pluginWindow.detach",
    "pluginWindow.detachHint",
    "pluginWindow.pinText",
    "pluginWindow.rerun",
    "pluginWindow.close",
    "pluginWindow.idle",
    "pluginWindow.fallbackTitle",
  ];
  for (const key of keys) {
    assert.ok(isMessageKey(key), `${key} is not in the message set`);
    assert.ok(createTranslator("en")(key as never).length > 0);
    assert.ok(createTranslator("zh")(key as never).length > 0);
  }
  assert.equal(createTranslator("zh")("pluginWindow.detach" as never), "固定到独立窗口");
});

test("every plugin-window class the detached view names has a rule in launcher.css", async () => {
  // R85 · `plugin-window__bar-button--icon` shipped in R84 with no rule behind
  // it: a dead class that read as styling and did nothing. The class was
  // removed; this guard holds the TSX and the stylesheet in step so the next
  // one fails here instead of surviving to a review.
  const tsx = await readFile(
    new URL("../src/plugin-window/DetachedPluginApp.tsx", import.meta.url),
    "utf8",
  );
  const css = await readFile(
    new URL("../src/styles/launcher.css", import.meta.url),
    "utf8",
  );
  const named = new Set(tsx.match(/plugin-window__[a-z-]+/g) ?? []);
  assert.ok(named.size > 0, "the detached view names plugin-window classes");
  for (const className of named) {
    assert.ok(
      css.includes(`.${className}`),
      `${className} is named by DetachedPluginApp.tsx but has no rule in launcher.css`,
    );
  }
});
