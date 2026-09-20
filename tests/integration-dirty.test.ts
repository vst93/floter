import { test } from "node:test";
import assert from "node:assert/strict";

import {
  customIntegrationDirty,
  normalizeForm,
  type ComparableIntegrationForm,
} from "../src/extensions/integration-dirty.ts";

const baseForm: ComparableIntegrationForm = {
  mode: "script",
  id: "local.abc12345",
  name: "My Tool",
  command: "mytool",
  version: "1.0.0",
  executablePath: "/usr/local/bin/mytool",
  scriptLanguage: "shell",
  scriptContent: "printf hi",
  argsPrefix: [],
  versionArgs: [],
  permissions: ["environment"],
  platforms: ["macos"],
  output: "background",
  params: [
    {
      id: "p1",
      label: "Target",
      kind: "text" as const,
      default: null,
      required: false,
      placeholder: null,
      options: [],
      flag: null,
    },
  ],
};

const legacyManifestShape = {
  ...baseForm,
  // What a legacy manifest round-trips into the editor as: fields the backend
  // omits arrive absent, and fromWireParams materializes editor defaults.
  scriptLanguage: undefined,
  output: undefined,
  params: [
    {
      id: "p1",
      label: "Target",
      kind: "text",
      default: null,
      required: false,
      placeholder: null,
      options: [],
      flag: null,
    },
  ],
} as unknown as ComparableIntegrationForm;

test("an untouched edit of a legacy definition is not dirty", () => {
  // The old JSON.stringify comparison reported dirty the instant a legacy
  // integration was opened (absent fields vs editor defaults). Semantic
  // comparison must see them as equal.
  assert.equal(customIntegrationDirty(legacyManifestShape, baseForm), false);
});

test("a real field change is still dirty", () => {
  const renamed = { ...baseForm, name: "Renamed" };
  assert.equal(customIntegrationDirty(renamed, baseForm), true);

  const editedCommand = { ...baseForm, command: "other" };
  assert.equal(customIntegrationDirty(editedCommand, baseForm), true);
});

test("explicit defaults and absent values mean the same thing", () => {
  const explicit = { ...baseForm, output: "background" as const };
  const absent = { ...baseForm, output: undefined } as unknown as ComparableIntegrationForm;
  assert.equal(customIntegrationDirty(explicit, absent), false);
});

test("permission order does not matter, membership does", () => {
  const reordered = {
    ...baseForm,
    permissions: ["files", "environment"],
  };
  const original = { ...baseForm, permissions: ["environment", "files"] };
  assert.equal(customIntegrationDirty(reordered, original), false);

  const added = { ...baseForm, permissions: ["environment", "files"] };
  assert.equal(customIntegrationDirty(added, baseForm), true);
});

test("param rows compare field by field with null/empty folding", () => {
  const wireShaped = {
    ...baseForm,
    params: [
      {
        id: "p1",
        label: "Target",
        kind: "text" as const,
        default: null,
        required: false,
        placeholder: null,
        options: [],
        flag: null,
      },
    ],
  };
  const filled = {
    ...baseForm,
    params: [
      {
        id: "p1",
        label: "Target",
        kind: "text" as const,
        default: "",
        required: false,
        placeholder: "",
        options: [],
        flag: "",
      },
    ],
  };
  // null in the wire shape, "" after an edit round-trip: same meaning.
  assert.equal(customIntegrationDirty(filled, wireShaped), false);

  const changedDefault = {
    ...baseForm,
    params: [{ ...wireShaped.params[0], default: "x" }],
  };
  assert.equal(customIntegrationDirty(changedDefault, wireShaped), true);
});

test("normalizeForm produces a stable canonical key", () => {
  assert.equal(
    JSON.stringify(normalizeForm(legacyManifestShape)),
    JSON.stringify(normalizeForm(baseForm)),
  );
});
