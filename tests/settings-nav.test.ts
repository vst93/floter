import assert from "node:assert/strict";
import test from "node:test";
import { isArrowKeyEditableTarget, nextSettingsPage } from "../src/settings-nav.ts";
import { SETTINGS_PAGES } from "../src/settings-persistence.ts";

// The settings sidebar must respond to ↑/↓ even when nothing but `<body>` holds
// focus, yet never steal the arrows from a field that needs them. The window
// handler delegates both decisions to these pure helpers.

test("arrows are left to inputs, textareas, selects and contenteditable", () => {
  assert.equal(isArrowKeyEditableTarget({ tagName: "INPUT" }), true);
  assert.equal(isArrowKeyEditableTarget({ tagName: "TEXTAREA" }), true);
  assert.equal(isArrowKeyEditableTarget({ tagName: "SELECT" }), true);
  assert.equal(isArrowKeyEditableTarget({ tagName: "DIV", isContentEditable: true }), true);
  assert.equal(
    isArrowKeyEditableTarget({ tagName: "SPAN", closest: () => ({}) }),
    true,
    "a node inside a contenteditable region must be treated as editable",
  );
});

test("non-editable targets and missing targets keep the arrows", () => {
  assert.equal(isArrowKeyEditableTarget({ tagName: "BODY" }), false);
  assert.equal(isArrowKeyEditableTarget({ tagName: "BUTTON" }), false);
  assert.equal(isArrowKeyEditableTarget({ tagName: "DIV", closest: () => null }), false);
  assert.equal(isArrowKeyEditableTarget(null), false);
  assert.equal(isArrowKeyEditableTarget(undefined), false);
  assert.equal(isArrowKeyEditableTarget("body"), false);
});

test("page navigation wraps around the sidebar order", () => {
  assert.equal(nextSettingsPage("general", "down"), "sessions");
  assert.equal(nextSettingsPage("sessions", "down"), "shortcuts");
  assert.equal(nextSettingsPage("sessions", "up"), "general");
  // Wrapping: down from the last lands on the first, up from the first on the last.
  assert.equal(nextSettingsPage(SETTINGS_PAGES[SETTINGS_PAGES.length - 1], "down"), "general");
  assert.equal(nextSettingsPage("general", "up"), SETTINGS_PAGES[SETTINGS_PAGES.length - 1]);
});
