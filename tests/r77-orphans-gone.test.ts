// R77 · the R76 iframe-layer orphans stay retired.
//
// R76 deleted the retired page layer and exposed the files and symbols only its
// dead documents consumed. R77 deleted those too: the inlined clipboard icon
// module (its only importer was a test), the launcher's delete-key predicate
// (its only caller was the retired page's key resolver), and the shared plugin
// settings-card sheet (its only importers were the retired pages). This is the
// freeze lock, in the shape R-FREEZE-2 / R76 established: scan the real tree
// and turn red if a removed name comes back.
//
// Each name is assembled from parts so this file does not itself reintroduce a
// token the round's zero-hit grep bans.
import assert from "node:assert/strict";
import { stat } from "node:fs/promises";
import test from "node:test";

const root = new URL("../", import.meta.url);

const RETIRED_FILES = [
  "src/clipboard" + "-icons.ts",
  "src/plugins/settings-" + "card.css",
] as const;

/** The launcher predicate whose last caller was the retired clipboard page. */
const RETIRED_LAUNCHER_EXPORT = "isHistory" + "DeleteKey";

test("the R77 orphan files stay deleted", async () => {
  // Non-vacuity: a live sibling of each retired path must resolve, so a typo in
  // the assembled names would not silently pass as "already gone".
  await stat(new URL("src/launcher.ts", root));
  await stat(new URL("src/plugins/config-schema.ts", root));
  for (const path of RETIRED_FILES) {
    await assert.rejects(
      stat(new URL(path, root)),
      { code: "ENOENT" },
      `${path} was deleted in R77 and must stay deleted`,
    );
  }
});

test("the R77 orphan launcher export stays deleted", async () => {
  const launcher = (await import("../src/launcher.ts")) as Record<string, unknown>;
  // The live ⌘⌫ key is still exported and resolved by the shared grammar.
  assert.ok("HISTORY_DELETE_SHORTCUT" in launcher, "the live delete key must still be exported");
  assert.ok(
    !(RETIRED_LAUNCHER_EXPORT in launcher),
    `${RETIRED_LAUNCHER_EXPORT} was deleted in R77 and must stay gone`,
  );
});
