// R146 · every version carrier in the tree agrees, and the agreement is read
// out of the carriers instead of being written down here.
//
// floter's release version lives in six places:
//   * `src-tauri/tauri.conf.json`  — what the bundler stamps into the artifacts;
//   * `src-tauri/Cargo.toml`       — the `[package]` version of the binary;
//   * `src-tauri/Cargo.lock`       — the `[[package]]` entry whose name is
//     `floter` (the lock is a list, so the entry is selected by name);
//   * `package.json`               — the npm root version;
//   * `package-lock.json`          — both the top-level version and the root
//     `packages[""]` entry;
//   * `packaging/arch/PKGBUILD`    — `pkgver`, the Arch package's own idea of it.
//
// The release workflow rewrites all of them inside each runner (`.github/
// workflows/release.yml`, "Apply release version"), so one missed carrier ships
// a build whose `--version`, package metadata and Arch pkgver disagree — the
// one class of version drift a user can observe directly. This guard makes the
// drift loud before a tag exists.
//
// Two properties keep it honest:
//   * it compares the *extracted* values to each other. No `X.Y.Z` literal
//     appears in this file, so the guard is version-independent: bumping the
//     six carriers to the next patch keeps it green with no edit here. What is
//     pinned is agreement, not a number.
//   * the Cargo lock is parsed into `[[package]]` entries and the version is
//     read from the entry named exactly `floter`. At this baseline two
//     unrelated crates (`field-offset`, `tauri-winres`) also resolved the same
//     version as floter, so a line-number or first-match scan would land on a
//     sibling; a name lookup cannot. The decoy count is not asserted — it moves
//     with every bump — only that the name lookup stays exact while decoys do
//     or do not exist.

import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

const root = new URL("../", import.meta.url);
const read = (path: string) => readFile(new URL(path, root), "utf8");

const TAURI_CONFIG = "src-tauri/tauri.conf.json";
const CARGO_MANIFEST = "src-tauri/Cargo.toml";
const CARGO_LOCK = "src-tauri/Cargo.lock";
const PACKAGE_JSON = "package.json";
const PACKAGE_LOCK = "package-lock.json";
const PKGBUILD = "packaging/arch/PKGBUILD";

const FLOTER = "floter";
/** The semver prefix the release workflow's tag validation requires. */
const SEMVER_SHAPE = /^\d+\.\d+\.\d+/;

/** One version carrier, named for the failure message. */
interface Carrier {
  readonly where: string;
  readonly value: string;
}

/** The `version` of the `[package]` table — not of any dependency table. */
const manifestVersion = (manifest: string): string => {
  const table = manifest.match(/^\[package\]([\s\S]*?)(?=^\[)/m);
  assert.ok(table, `${CARGO_MANIFEST} has no [package] table`);
  const version = table[1].match(/^version\s*=\s*"([^"]+)"/m);
  assert.ok(version, `${CARGO_MANIFEST} [package] declares no version`);
  return version[1];
};

/** The lock's `[[package]]` blocks, as `{name, version}` entries. */
const lockedPackages = (lock: string): { name: string; version: string }[] =>
  lock
    .split(/^\[\[package\]\]$/m)
    .slice(1)
    .map((block) => ({
      name: block.match(/^name\s*=\s*"([^"]+)"/m)?.[1] ?? "",
      version: block.match(/^version\s*=\s*"([^"]+)"/m)?.[1] ?? "",
    }));

/** The version of the one package named `crate`, located by name. */
const versionOf = (
  packages: { name: string; version: string }[],
  crate: string,
): string => {
  const found = packages.filter((entry) => entry.name === crate);
  assert.equal(
    found.length,
    1,
    `${CARGO_LOCK} resolves \`${crate}\` ${found.length} times, not once`,
  );
  assert.notEqual(found[0].version, "", `${CARGO_LOCK} entry \`${crate}\` has no version`);
  return found[0].version;
};

/** Every carrier, extracted from its own file. */
const carriers = async (): Promise<Carrier[]> => {
  const tauri = (JSON.parse(await read(TAURI_CONFIG)) as { version: string })
    .version;
  const cargo = manifestVersion(await read(CARGO_MANIFEST));
  const packages = lockedPackages(await read(CARGO_LOCK));
  const locked = versionOf(packages, FLOTER);
  const pkg = JSON.parse(await read(PACKAGE_JSON)) as { version: string };
  const npm = JSON.parse(await read(PACKAGE_LOCK)) as {
    version: string;
    packages: Record<string, { version?: string }>;
  };
  const npmRoot = npm.packages[""]?.version;
  assert.ok(npmRoot, `${PACKAGE_LOCK} has no root \`packages[""]\` version`);
  const pkgbuild = (await read(PKGBUILD)).match(/^pkgver=(.+)$/m)?.[1];
  assert.ok(pkgbuild, `${PKGBUILD} declares no pkgver`);

  return [
    { where: TAURI_CONFIG, value: tauri },
    { where: CARGO_MANIFEST, value: cargo },
    { where: `${CARGO_LOCK} (${FLOTER})`, value: locked },
    { where: PACKAGE_JSON, value: pkg.version },
    { where: `${PACKAGE_LOCK} (top level)`, value: npm.version },
    { where: `${PACKAGE_LOCK} (packages[""])`, value: npmRoot },
    { where: PKGBUILD, value: pkgbuild },
  ];
};

test("R146 · the six version carriers all carry the same value", async () => {
  const found = await carriers();
  const reference = found[0].value;
  assert.notEqual(reference, "", `${found[0].where} carries an empty version`);

  for (const { where, value } of found) {
    assert.equal(
      value,
      reference,
      `${where} carries ${value}, but ${found[0].where} carries ${reference}`,
    );
  }

  // The lock lookup is keyed on the name. Whatever other crates resolve the
  // same version string — the baseline had two decoys — they are separate
  // entries and must not be what the floter version was read from.
  const packages = lockedPackages(await read(CARGO_LOCK));
  const decoys = packages.filter(
    (entry) => entry.version === reference && entry.name !== FLOTER,
  );
  const floter = packages.filter((entry) => entry.name === FLOTER);
  assert.equal(
    floter.length,
    1,
    `${CARGO_LOCK} must hold exactly one \`${FLOTER}\` package`,
  );
  assert.equal(
    floter[0].version,
    reference,
    `the ${FLOTER} lock entry and the other carriers disagree`,
  );
  assert.ok(
    decoys.every((entry) => entry.name !== FLOTER),
    "a name lookup must not confuse a same-version sibling with floter",
  );
});

test("R146 · the agreed version has the shape the release workflow tags", async () => {
  for (const { where, value } of await carriers()) {
    assert.match(
      value,
      SEMVER_SHAPE,
      `${where} carries \`${value}\`, which is not an \`X.Y.Z\` version`,
    );
  }
});

test("R146 · this guard compares extracted values, it spells no version", async () => {
  // The self-proof for version-independence: if a `X.Y.Z` literal crept in,
  // the guard would pin one release and turn red on the next bump for the wrong
  // reason. Comments are stripped so prose may explain the rule without
  // tripping it; code may not carry a number.
  const self = await read("tests/r146-release-hygiene.test.ts");
  const code = self
    .replace(/\/\*[\s\S]*?\*\//g, "")
    .replace(/^\s*\/\/.*$/gm, "");
  assert.doesNotMatch(
    code,
    /\d+\.\d+\.\d+/,
    "the guard must build its version from the carriers, never write one down",
  );
});
