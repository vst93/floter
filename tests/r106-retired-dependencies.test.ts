// R106 · the six dead direct dependencies stay dead.
//
// `toml`, `shlex`, `parking_lot`, `ed25519-dalek`, `tar` and `flate2` were
// declared in `src-tauri/Cargo.toml` with **zero** path references anywhere in
// `src-tauri/src/` (R105 §1.3). They are not merely tidy-up: an unused direct
// declaration still drags its transitive tree into `Cargo.lock` and the build,
// and it is exactly the line a later round re-adds "because it was already
// there". Deleting them is one round of work that nothing else would notice
// coming back, so it gets a lock.
//
// Two scans, so the guard cannot be satisfied by an incidental mention:
//
//   * the manifest must not declare any of the six (whole-token scan plus a
//     parse of the declared keys), and
//   * at least six *live* direct dependencies must still be declared, so the
//     manifest cannot pass by being emptied.
//
// The names are assembled from parts on purpose. The self-scan below includes
// this file, so a complete quoted literal here would make the guard fail on
// itself — the same self-poisoning shape as `tests/a11y-backstops.test.ts`
// (R104 M3) and `tests/r76-retired-page-layer.test.ts`.

import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

const root = new URL("../", import.meta.url);
const read = (path: string) => readFile(new URL(path, root), "utf8");

const MANIFEST = "src-tauri/Cargo.toml";

/**
 * The six crates R105 proved dead, assembled so this file holds no full
 * literal of any of them.
 */
const RETIRED_DEPENDENCIES = [
  "tom" + "l",
  "sh" + "lex",
  "parking" + "_lot",
  "ed25519" + "-dalek",
  "t" + "ar",
  "flate" + "2",
];

/**
 * Names that must still be declared. A manifest stripped down to nothing would
 * satisfy "the six are gone" while breaking the build, so the guard pins a
 * floor of live dependencies too.
 */
const LIVE_DEPENDENCIES = [
  "tauri",
  "serde",
  "serde_json",
  "tokio",
  "arboard",
  "reqwest",
  "rusqlite",
  "tempfile",
];

/** Strip TOML comments, so a prose mention cannot satisfy — or trip — a scan. */
const stripComments = (source: string) =>
  source.replace(/^\s*#.*$/gm, "");

const escapeRegExp = (value: string) =>
  value.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");

/**
 * Whether `name` appears in `source` as a whole token. Substring matching is
 * not enough: `target` and `start` both contain `tar`, and the manifest's table
 * headers are full of `target`.
 */
const hasWholeToken = (source: string, name: string) =>
  new RegExp(`(^|[^A-Za-z0-9_-])${escapeRegExp(name)}([^A-Za-z0-9_-]|$)`).test(
    source,
  );

/** The dependency keys declared under any `[…dependencies]` table. */
const declaredDependencies = (manifest: string): Set<string> => {
  const names = new Set<string>();
  const header = /^\s*\[([^\]]+)\]\s*$/;
  const entry = /^\s*([A-Za-z0-9_-]+)\s*=/;
  let inDependencyTable = false;
  for (const line of manifest.split("\n")) {
    const table = line.match(header);
    if (table) {
      const path = table[1].trim();
      // `[dependencies]`, `[dev-dependencies]`, `[build-dependencies]`,
      // `[target.'cfg(…)'.dependencies]` and the inline `[dependencies.foo]`.
      const inline = path.match(
        /(^|\.)(?:dev-|build-)?dependencies\.([A-Za-z0-9_-]+)$/,
      );
      if (inline) names.add(inline[2]);
      inDependencyTable = /(^|\.)(?:dev-|build-)?dependencies$/.test(path);
      continue;
    }
    const declaration = line.match(entry);
    if (inDependencyTable && declaration) names.add(declaration[1]);
  }
  return names;
};

test("the retired dependency list is the six R105 named", () => {
  assert.equal(
    RETIRED_DEPENDENCIES.length,
    6,
    "the guard is about the six dead direct dependencies; keep it that size",
  );
  assert.deepEqual(
    [...new Set(RETIRED_DEPENDENCIES)].sort(),
    [...RETIRED_DEPENDENCIES].sort(),
    "a duplicated name would make the scan narrower than it reads",
  );
});

test("the manifest declares none of the six dead dependencies", async () => {
  const manifest = stripComments(await read(MANIFEST));

  for (const name of RETIRED_DEPENDENCIES) {
    assert.ok(
      !hasWholeToken(manifest, name),
      `${MANIFEST} must not reference the retired dependency \`${name}\``,
    );
  }

  // The whole-token scan already covers every spelling, but the parsed key set
  // is what makes the claim "not declared" rather than "not mentioned".
  const declared = declaredDependencies(manifest);
  for (const name of RETIRED_DEPENDENCIES) {
    assert.ok(
      !declared.has(name),
      `${MANIFEST} still declares \`${name}\` as a dependency`,
    );
  }
});

test("the manifest still declares the live dependencies", async () => {
  const declared = declaredDependencies(await read(MANIFEST));

  for (const name of LIVE_DEPENDENCIES) {
    assert.ok(
      declared.has(name),
      `${MANIFEST} lost the live dependency \`${name}\` — the removal went too far`,
    );
  }
  assert.ok(
    declared.size >= 6,
    `the manifest declares only ${declared.size} dependencies; that is not the crate`,
  );
});

test("this guard assembles the retired names, it does not spell them out", async () => {
  const self = await readFile(new URL(import.meta.url), "utf8");

  for (const name of RETIRED_DEPENDENCIES) {
    // A full quoted literal here would be the thing the scan above exists to
    // ban. `"src-tauri/Cargo.toml"` is fine: the token after the quote is the
    // whole path, not the bare name.
    assert.ok(
      !self.includes(`"${name}"`) && !self.includes(`'${name}'`),
      `the guard must build \`${name}\` from parts, not write it as a literal`,
    );
  }
});
