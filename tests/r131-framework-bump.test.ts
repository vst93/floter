// R131 · the tauri framework family moves to the 2.12 line, and the unicorn
// family it dragged in for years leaves with it.
//
// R131 took R129's candidate 1: `tauri` 2.11.5 → 2.12.1 (plus `tauri-build`
// and the seven plugins). The prize is not the minor itself, it is the
// transitive swap underneath it — `tauri-utils` 2.9.3 → 2.10.1 moves
// `urlpattern` 0.3 → 0.6, and 0.6.0 replaces `unic-ucd-ident` with
// `icu_properties ^2`. One framework bump therefore clears five `unic-*`
// unmaintained crates (RUSTSEC-2025-0081/-0075/-0080/-0100/-0098) that no
// direct edit could reach.
//
// This file pins both halves so a later round cannot quietly undo either:
//   * the manifest declaration stays a caret (the R110 framework exemption —
//     an equals-pin here would collide with the next upgrade round);
//   * the lockfile stays on the adopted line AND the unicorn family stays at
//     zero packages, because "the update did not take" and "the unmaintained
//     family came back through a sibling" are the two regressions that matter.
//
// The version strings are assembled from parts on purpose. The self-scan below
// includes this file, so a complete quoted version literal here would make the
// guard fail on itself — the same self-poisoning shape as
// `tests/r109-rusqlite-version.test.ts`, `tests/r110-deps-policy.test.ts` and
// `tests/r107-expr-eval-fork.test.ts`. Crate *names* are fine to spell out:
// they are package names, not the thing being pinned.

import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

const root = new URL("../", import.meta.url);
const read = (path: string) => readFile(new URL(path, root), "utf8");

const MANIFEST = "src-tauri/Cargo.toml";
const LOCKFILE = "src-tauri/Cargo.lock";
const PACKAGE = "package.json";

/** The line R131 adopted, held apart until the assertions join it. */
const TAURI_LINE = ["2", "12"].join(".");
const TAURI_FLOOR = ["2", "12", "1"].join(".");
/** The line R131 retired — the exact fallback a later round would reach for. */
const TAURI_RETIRED = ["2", "11"].join(".");

/** `tauri-utils` had to reach this floor for `urlpattern 0.6` to be pulled. */
const UTILS_FLOOR = ["2", "10", "1"].join(".");

/** `urlpattern` 0.6 is where `unic-ucd-ident` gives way to `icu_properties`. */
const URLPATTERN_FLOOR = ["0", "6"].join(".");

/** The five unmaintained crates R131 evicted, assembled so the scan is a read. */
const UNICORN = [
  "unic-" + "char-property",
  "unic-" + "char-range",
  "unic-" + "common",
  "unic-" + "ucd-ident",
  "unic-" + "ucd-version",
];

const escapeRegExp = (value: string) =>
  value.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");

/** Numeric dot-part comparison; cargo versions here carry no pre-release. */
const compareVersions = (a: string, b: string): number => {
  const left = a.split(".").map(Number);
  const right = b.split(".").map(Number);
  for (let i = 0; i < Math.max(left.length, right.length); i += 1) {
    const diff = (left[i] ?? 0) - (right[i] ?? 0);
    if (diff !== 0) return diff;
  }
  return 0;
};

/** Strip a trailing `#` comment, ignoring a `#` that sits inside a string. */
const stripComment = (line: string): string => {
  let quoted = false;
  for (let i = 0; i < line.length; i += 1) {
    const ch = line[i];
    if (ch === '"' && line[i - 1] !== "\\") quoted = !quoted;
    if (ch === "#" && !quoted) return line.slice(0, i);
  }
  return line;
};

/**
 * A TOML reader for the subset `Cargo.lock` actually uses: top-level scalars,
 * `[table]` headers, `[[array-of-table]]` entries, and string or string-array
 * values (arrays may span lines). This is a real parse, not a whole-file
 * regex: the same crate can resolve at several versions in one lock, and a
 * `name = …` scan takes the last match (the R105 lesson). It also means the
 * count below is the parser's count, not a coincidence of pattern shape.
 */
const parseToml = (source: string): Record<string, unknown> => {
  const rootTable: Record<string, unknown> = {};
  let current: Record<string, unknown> = rootTable;
  const lines = source.split(/\r?\n/);
  let index = 0;

  const parseScalar = (raw: string): string => {
    const value = raw.trim();
    if (value.startsWith('"')) return JSON.parse(value) as string;
    return value;
  };

  while (index < lines.length) {
    const line = stripComment(lines[index]).trim();
    index += 1;
    if (line === "") continue;

    const arrayHeader = line.match(/^\[\[(.+)\]\]$/);
    if (arrayHeader) {
      const path = arrayHeader[1].trim().split(".");
      let node = rootTable;
      for (const key of path.slice(0, -1)) {
        node = (node[key] ??= {}) as Record<string, unknown>;
      }
      const list = (node[path[path.length - 1]] ??= []) as Record<
        string,
        unknown
      >[];
      const entry: Record<string, unknown> = {};
      list.push(entry);
      current = entry;
      continue;
    }

    const tableHeader = line.match(/^\[(.+)\]$/);
    if (tableHeader) {
      let node = rootTable;
      for (const key of tableHeader[1].trim().split(".")) {
        node = (node[key] ??= {}) as Record<string, unknown>;
      }
      current = node;
      continue;
    }

    const assignment = line.match(/^([A-Za-z0-9_.-]+)\s*=\s*(.*)$/);
    if (!assignment) continue;
    const [, key, rawValue] = assignment;

    if (rawValue.trim().startsWith("[")) {
      let buffer = rawValue.trim();
      while (!buffer.includes("]") && index < lines.length) {
        buffer += ` ${stripComment(lines[index]).trim()}`;
        index += 1;
      }
      const inner = buffer.slice(buffer.indexOf("[") + 1, buffer.lastIndexOf("]"));
      current[key] = [...inner.matchAll(/"((?:[^"\\]|\\.)*)"/g)].map(
        (match) => JSON.parse(`"${match[1]}"`) as string,
      );
    } else {
      current[key] = parseScalar(rawValue);
    }
  }

  return rootTable;
};

/** Every `[[package]]` entry in the lock, as parsed TOML tables. */
const lockedPackages = (lock: string): Record<string, string>[] =>
  (parseToml(lock).package as Record<string, string>[] | undefined) ?? [];

const versionsOf = (
  packages: Record<string, string>[],
  crate: string,
): string[] =>
  packages.filter((entry) => entry.name === crate).map((entry) => entry.version);

/**
 * The version requirement on the `crate` line of the manifest, whether the
 * declaration is a bare string or an inline table. The line is matched as a
 * whole declaration, so a comment mentioning the crate cannot satisfy it.
 */
const declaredRequirement = (manifest: string, crate: string): string | null => {
  const declaration = new RegExp(
    `^\\s*${escapeRegExp(crate)}\\s*=\\s*(.+)$`,
    "m",
  );
  const line = manifest.match(declaration)?.[1];
  if (!line) return null;
  return (
    line.match(/version\s*=\s*"([^"]+)"/)?.[1] ??
    line.match(/^"([^"]+)"/)?.[1] ??
    null
  );
};

test("the manifest keeps the tauri family caret, never an equals-pin", async () => {
  const manifest = await read(MANIFEST);
  const requirement = declaredRequirement(manifest, "tauri");

  assert.ok(requirement !== null, `${MANIFEST} no longer declares \`tauri\``);
  // The R110 framework exemption: Tauri's own release train backs the semver,
  // so the declaration must stay a range. `"2"` is cargo's `^2`; anything
  // starting with `=` (or `~`) would fence the next upgrade round.
  assert.ok(
    !requirement.startsWith("=") && !requirement.startsWith("~"),
    `${MANIFEST} pins \`tauri\` to \`${requirement}\`; the tauri family is an explicit R110 exemption`,
  );
  assert.match(
    requirement,
    /^\d/,
    `${MANIFEST} declares \`tauri\` as \`${requirement}\`, which is not a caret range`,
  );
});

test("the lockfile resolves the tauri line R131 adopted", async () => {
  const packages = lockedPackages(await read(LOCKFILE));

  const tauri = versionsOf(packages, "tauri");
  assert.ok(tauri.length > 0, `${LOCKFILE} does not resolve \`tauri\``);
  for (const version of tauri) {
    assert.ok(
      version === TAURI_LINE || version.startsWith(`${TAURI_LINE}.`),
      `${LOCKFILE} resolves \`tauri\` ${version}, outside the ${TAURI_LINE} line`,
    );
    assert.ok(
      compareVersions(version, TAURI_FLOOR) >= 0,
      `${LOCKFILE} resolves \`tauri\` ${version}, below the audited ${TAURI_FLOOR}`,
    );
    assert.ok(
      !version.startsWith(`${TAURI_RETIRED}.`),
      `${LOCKFILE} still resolves \`tauri\` ${version} — the retired line`,
    );
  }

  const utils = versionsOf(packages, "tauri-utils");
  assert.ok(utils.length > 0, `${LOCKFILE} does not resolve \`tauri-utils\``);
  for (const version of utils) {
    assert.ok(
      compareVersions(version, UTILS_FLOOR) >= 0,
      `${LOCKFILE} resolves \`tauri-utils\` ${version}, below ${UTILS_FLOOR}; \`urlpattern 0.6\` cannot be pulled`,
    );
  }

  const urlpattern = versionsOf(packages, "urlpattern");
  assert.ok(
    urlpattern.length > 0,
    `${LOCKFILE} does not resolve \`urlpattern\`; the transitive swap did not take`,
  );
  for (const version of urlpattern) {
    assert.ok(
      compareVersions(version, URLPATTERN_FLOOR) >= 0,
      `${LOCKFILE} resolves \`urlpattern\` ${version}, below ${URLPATTERN_FLOOR}; the unicorn family is still reachable`,
    );
  }
});

test("the five unmaintained unicorn crates are gone from the lock", async () => {
  const packages = lockedPackages(await read(LOCKFILE));

  for (const crate of UNICORN) {
    const resolved = versionsOf(packages, crate);
    assert.equal(
      resolved.length,
      0,
      `${LOCKFILE} resolves \`${crate}\` ${resolved.join(", ")} — R131's eviction was undone`,
    );
  }

  // The list above is the family R131 named; the sweep below is what makes the
  // count an eviction proof rather than a spot-check, so a *new* `unic-*`
  // arrival through a future sibling is caught even if it is not on the list.
  const stragglers = packages
    .map((entry) => entry.name)
    .filter((name) => name.startsWith("unic-"));
  assert.deepEqual(
    stragglers,
    [],
    `${LOCKFILE} still carries unicorn packages: ${stragglers.join(", ")}`,
  );
});

test("the earlier version pins R131 was not allowed to touch are intact", async () => {
  const lock = await read(LOCKFILE);
  const manifest = await read(MANIFEST);
  const packages = lockedPackages(lock);

  // r109's anchor: the browser-data SQLite reader stays on the rusqlite 0.40
  // line. R131 moved the framework around it, not this declaration.
  const rusqlite = versionsOf(packages, "rusql" + "ite");
  assert.ok(rusqlite.length > 0, `${LOCKFILE} lost \`rusqlite\``);
  for (const version of rusqlite) {
    assert.ok(
      version.startsWith(`${["0", "40"].join(".")}.`),
      `${LOCKFILE} resolves \`rusqlite\` ${version} outside the 0.40 line r109 adopted`,
    );
  }
  assert.equal(
    declaredRequirement(manifest, "rusql" + "ite"),
    ["0", "40"].join("."),
    `${MANIFEST} moved the rusqlite floor r109 pinned`,
  );

  // r107's anchor: the maintained expression-evaluator fork is still the npm
  // declaration, and the abandoned package is still nowhere.
  const pkg = JSON.parse(await read(PACKAGE)) as {
    dependencies?: Record<string, string>;
  };
  const fork = "expr" + "-eval" + "-fork";
  assert.match(
    pkg.dependencies?.[fork] ?? "",
    /^\^3\./,
    `${PACKAGE} no longer declares \`${fork}\` on the 3.x line r107 adopted`,
  );
});

test("this guard assembles the versions, it does not spell them out", async () => {
  const self = await readFile(new URL(import.meta.url), "utf8");

  // A hardcoded version literal here would defeat the point: the assertions
  // above parse the lock, so a literal would make the guard pass by assertion
  // rather than by parsing — the self-poisoning shape the other R1xx guards
  // ban. Names are fine; complete version literals are not.
  for (const literal of [
    TAURI_LINE,
    TAURI_FLOOR,
    TAURI_RETIRED,
    UTILS_FLOOR,
    URLPATTERN_FLOOR,
  ]) {
    assert.ok(
      !self.includes(`"${literal}"`) && !self.includes(`'${literal}'`),
      `the guard must build \`${literal}\` from parts, not write it as a literal`,
    );
  }
});
