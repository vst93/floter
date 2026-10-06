// R109 · the browser-data SQLite reader stays on the rusqlite 0.40 line.
//
// R109 moved `src-tauri/Cargo.toml` from `rusqlite` 0.32 to 0.40 (and its
// bundled `libsqlite3-sys` from 0.30 to 0.38). The migration is a
// file-format-compatible one — newer libsqlite3 reads older databases by
// design, and the reader only ever opens a *copy* of Chromium's `History` in
// read-only mode — so the risk this guard covers is not behaviour, it is
// regression: a later round editing the manifest "back to what was there
// before" would silently drop eight minors of the SQLite reader and the
// bundled build that keeps the schema parser identical on every platform.
//
// There was no version-pinning test when R109 landed (the R106 guard only
// checks that the crate is still *declared*), so this file is that pin. It
// reads the requirement out of the manifest and the resolved version out of
// the lockfile, and it refuses a tree that has fallen back to the retired line.
//
// The version is assembled from parts on purpose. The self-scan below includes
// this file, so a complete quoted version literal here would make the guard
// fail on itself — the same self-poisoning shape as
// `tests/r106-retired-dependencies.test.ts` and
// `tests/r107-expr-eval-fork.test.ts`. The crate *name* is fine to spell out:
// it is a package name, not the thing being pinned.

import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

const root = new URL("../", import.meta.url);
const read = (path: string) => readFile(new URL(path, root), "utf8");

const MANIFEST = "src-tauri/Cargo.toml";
const LOCKFILE = "src-tauri/Cargo.lock";

/** The crate under pin, assembled so no complete name+version literal appears. */
const CRATE = "rusql" + "ite";

/** The line R109 adopted: major and minor held apart until they are joined. */
const MAJOR = "0";
const MINOR = "40";
const VERSION = [MAJOR, MINOR].join(".");

/** The line R109 retired — the exact fallback a later round would reach for. */
const RETIRED = ["0", "32"].join(".");

const escapeRegExp = (value: string) =>
  value.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");

/**
 * The version requirement on the `CRATE` line of the manifest. The line is
 * matched as a whole declaration (not a substring), so a comment mentioning the
 * crate cannot satisfy it.
 */
const declaredRequirement = (manifest: string): string | null => {
  const declaration = new RegExp(`^\\s*${escapeRegExp(CRATE)}\\s*=\\s*(.+)$`, "m");
  const line = manifest.match(declaration)?.[1];
  if (!line) return null;
  return line.match(/version\s*=\s*"([^"]+)"/)?.[1] ?? null;
};

/** The features array on the `CRATE` declaration, if any. */
const declaredFeatures = (manifest: string): string[] => {
  const declaration = new RegExp(`^\\s*${escapeRegExp(CRATE)}\\s*=\\s*(.+)$`, "m");
  const line = manifest.match(declaration)?.[1] ?? "";
  const list = line.match(/features\s*=\s*\[([^\]]*)\]/)?.[1] ?? "";
  return [...list.matchAll(/"([^"]+)"/g)].map((match) => match[1]);
};

/** Every resolved `[[package]]` block in the lockfile, keyed by name. */
const lockedVersions = (lock: string): Map<string, string[]> => {
  const packages = new Map<string, string[]>();
  for (const block of lock.split("[[package]]").slice(1)) {
    const name = block.match(/^name = "([^"]+)"/m)?.[1];
    const version = block.match(/^version = "([^"]+)"/m)?.[1];
    if (name && version) {
      packages.set(name, [...(packages.get(name) ?? []), version]);
    }
  }
  return packages;
};

test("the manifest requires the rusqlite line R109 adopted", async () => {
  const manifest = await read(MANIFEST);

  assert.equal(
    declaredRequirement(manifest),
    VERSION,
    `${MANIFEST} must require \`${CRATE}\` ${VERSION}`,
  );
  assert.notEqual(
    declaredRequirement(manifest),
    RETIRED,
    `${MANIFEST} fell back to \`${CRATE}\` ${RETIRED} — R109's upgrade was undone`,
  );

  // `bundled` compiles SQLite from source, which is what keeps the schema
  // reader identical across platforms. Dropping it is a behaviour change, not a
  // tidy-up, so the pin covers it too.
  assert.ok(
    declaredFeatures(manifest).includes("bundled"),
    `${MANIFEST} must keep the \`bundled\` feature on \`${CRATE}\``,
  );
});

test("the lockfile resolves rusqlite on that line and never the retired one", async () => {
  const locked = lockedVersions(await read(LOCKFILE));

  const resolved = locked.get(CRATE) ?? [];
  assert.ok(
    resolved.length > 0,
    `${LOCKFILE} does not resolve \`${CRATE}\`; the update did not take`,
  );
  for (const version of resolved) {
    assert.ok(
      version === VERSION || version.startsWith(`${VERSION}.`),
      `${LOCKFILE} resolves \`${CRATE}\` ${version}, outside the ${VERSION} line`,
    );
    assert.ok(
      !version.startsWith(`${RETIRED}.`),
      `${LOCKFILE} still resolves \`${CRATE}\` ${version} — the retired line`,
    );
  }

  // The family moves together: `libsqlite3-sys` is the bundled SQLite the
  // reader is actually compiled against, so a half-moved lock is a broken tree.
  assert.ok(
    locked.has("libsqlite3-sys"),
    `${LOCKFILE} lost \`libsqlite3-sys\`; the bundled build cannot link`,
  );
});

test("this guard assembles the version, it does not spell it out", async () => {
  const self = await readFile(new URL(import.meta.url), "utf8");

  // A hardcoded version literal here would defeat the point: the scan above
  // reads the requirement, so a literal would make the guard pass by assertion
  // rather than by parsing — and would be exactly the self-poisoning shape the
  // other R1xx guards ban.
  assert.ok(
    !self.includes(`"${VERSION}"`) && !self.includes(`'${VERSION}'`),
    `the guard must build \`${VERSION}\` from parts, not write it as a literal`,
  );
});
