import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";
import { createTranslator } from "../src/i18n.ts";

const root = new URL("../", import.meta.url);
const read = (path: string) => readFile(new URL(path, root), "utf8");

const NOTE_KEY = "settings.extensions.publisherDescriptorNote";

// Publisher-shipped descriptors (v-tools) carry a command list that tracks the
// release payload, not the local binary. The details drawer must say so, in
// both languages, so a user never expects a local tool upgrade to change it.
test("the publisher-descriptor note is translated in en and zh", () => {
  const en = createTranslator("en")(NOTE_KEY);
  const zh = createTranslator("zh")(NOTE_KEY);
  assert.ok(en.length > 0, "en note must be non-empty");
  assert.ok(zh.length > 0, "zh note must be non-empty");
  // The zh entry is its own translation, not an English fallback.
  assert.notEqual(zh, en);
  assert.ok(/[\u4e00-\u9fff]/.test(zh), "zh note must contain Chinese text");
});

test("the i18n source defines the note key once per language", async () => {
  const source = await read("src/i18n.ts");
  const occurrences = source.split(`"${NOTE_KEY}"`).length - 1;
  assert.equal(occurrences, 2, "the note key must be defined for en and zh");
});

// The frontend contract: the list item carries a `publisherDescriptor` flag and
// the drawer renders the note behind it — and only behind it (no note for
// generated custom integrations).
test("the Extension type carries the publisherDescriptor flag", async () => {
  const panel = await read("src/ExtensionsPanel.tsx");
  assert.match(
    panel,
    /publisherDescriptor:\s*boolean;/,
    "Extension must declare publisherDescriptor: boolean",
  );
});

test("the drawer renders the note only for publisher descriptors", async () => {
  const panel = await read("src/ExtensionsPanel.tsx");
  assert.ok(
    panel.includes('selected.publisherDescriptor &&'),
    "the note must be gated on selected.publisherDescriptor",
  );
  assert.ok(
    panel.includes('t("settings.extensions.publisherDescriptorNote")'),
    "the drawer must render the publisher-descriptor note",
  );
});

// The note must not leak into the generated-integration path: generated
// integrations get the re-scan button instead, and publisher descriptors never
// do (they are never re-probed). The re-scan button's render condition must
// reference `generatedCustom` and never `publisherDescriptor`.
test("the re-scan button stays exclusive to generated integrations", async () => {
  const panel = await read("src/ExtensionsPanel.tsx");
  assert.ok(
    panel.includes("selected.generatedCustom &&"),
    "the re-scan button must remain gated on generatedCustom",
  );
  // Isolate the re-scan button's own render condition: the line that calls
  // `handleReprobeCommands` must be wrapped by `generatedCustom` and must not
  // mention `publisherDescriptor` at all.
  const buttonCall = panel.indexOf("handleReprobeCommands()");
  assert.notEqual(buttonCall, -1, "the re-scan button must call handleReprobeCommands");
  const previousButton = panel.lastIndexOf("<button", buttonCall);
  const conditionStart = panel.lastIndexOf("selected.generatedCustom &&", buttonCall);
  assert.ok(
    conditionStart !== -1 &&
      conditionStart < previousButton &&
      previousButton < buttonCall,
    "the re-scan button must be wrapped by a generatedCustom condition",
  );
  const gate = panel.slice(conditionStart, previousButton);
  assert.ok(gate.length < 200, "the condition must immediately precede the button");
  assert.ok(
    gate.includes("generatedCustom") && !gate.includes("publisherDescriptor"),
    "the re-scan button must be gated on generatedCustom, not publisherDescriptor",
  );
});
