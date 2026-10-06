// R107 · the retired expression evaluator stays retired; the maintained fork is
// the one the manifest declares.
//
// The package below (`expr-eval`, assembled from parts) was last released in
// 2019 and carries three advisories, one of them CRITICAL. R107 replaced it
// with its maintained fork, whose API is a drop-in: the same `Parser`, the same
// `options` object, the same nine operators switched off. The mitigation in
// `src/calculator.ts` is unchanged — this guard is about the *declaration*, so a
// later round cannot put the abandoned package back "because it was already
// there", and cannot let a dependency pull it back in transitively.
//
// The names are assembled on purpose. The self-scan includes this file, so a
// complete quoted literal here would make the guard fail on itself — the same
// self-poisoning shape as `tests/r106-retired-dependencies.test.ts` and
// `tests/a11y-backstops.test.ts` (R104 M3).

import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

const root = new URL("../", import.meta.url);
const MANIFEST = "package.json";
const LOCKFILE = "package-lock.json";

/** The abandoned evaluator, assembled so no full literal appears in this file. */
const RETIRED = "expr" + "-eval";

/** Its maintained fork: the same core, the same API, a live release line. */
const FORK = RETIRED + "-fork";

const escapeRegExp = (value: string) =>
  value.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");

/**
 * Whether `name` appears in `source` as a whole token. Substring matching is
 * not enough in either direction: the fork's own name *contains* the retired
 * one, so a naive `includes` would be red on a correct tree.
 */
const hasWholeToken = (source: string, name: string) =>
  new RegExp(`(^|[^A-Za-z0-9_-])${escapeRegExp(name)}([^A-Za-z0-9_-]|$)`).test(
    source,
  );

/** Every dependency key the manifest declares, direct or development. */
const declaredDependencies = (manifest: string): Record<string, string> => {
  const parsed = JSON.parse(manifest) as {
    dependencies?: Record<string, string>;
    devDependencies?: Record<string, string>;
  };
  return { ...(parsed.dependencies ?? {}), ...(parsed.devDependencies ?? {}) };
};

test("the manifest declares the fork and not the retired evaluator", async () => {
  const raw = await readFile(new URL(MANIFEST, root), "utf8");
  const declared = declaredDependencies(raw);

  // The exact-key test, not a substring: the fork's name contains the retired
  // one, so `raw.includes(...)` would be red on a correct tree.
  assert.ok(
    !(RETIRED in declared),
    `${MANIFEST} must not declare \`${RETIRED}\`; the maintained fork is \`${FORK}\``,
  );
  assert.ok(
    FORK in declared,
    `${MANIFEST} lost the fork \`${FORK}\` — the replacement went too far`,
  );
  assert.match(
    declared[FORK] ?? "",
    /^\^3\.\d+\.\d+$/,
    `${FORK} must stay on the 3.x line R107 adopted`,
  );

  // The whole-token scan catches a declaration spelled anywhere in the raw
  // file, including a key the parse above might miss. `-` is not a token
  // boundary character here, so a `…-fork` name does not count as a hit.
  assert.ok(
    !hasWholeToken(raw, RETIRED),
    `${MANIFEST} references the retired evaluator \`${RETIRED}\``,
  );
});

test("the lockfile resolves the fork and no copy of the retired evaluator", async () => {
  const lock = JSON.parse(await readFile(new URL(LOCKFILE, root), "utf8")) as {
    packages?: Record<string, unknown>;
  };
  const packages = lock.packages ?? {};

  assert.ok(
    `node_modules/${FORK}` in packages,
    `${LOCKFILE} does not resolve \`${FORK}\`; the install did not take`,
  );
  // An exact key, not a substring: a transitive re-introduction lands here,
  // which is exactly how the advisory would come back.
  assert.ok(
    !(`node_modules/${RETIRED}` in packages),
    `${LOCKFILE} still resolves \`${RETIRED}\` — some dependency pulls it back in`,
  );
});

test("this guard assembles the retired name, it does not spell it out", async () => {
  const self = await readFile(new URL(import.meta.url), "utf8");

  assert.ok(
    !self.includes(`"${RETIRED}"`) && !self.includes(`'${RETIRED}'`),
    "the guard must build the retired name from parts, not write it as a literal",
  );
});

test("the retired upstream's stale claims are not re-planted", async () => {
  // R107 replaced the package, but the prose it left behind in
  // `src/calculator.ts` and `tests/r50-calculator.test.ts` still argued from
  // "upstream never shipped a fix". That reasoning is retired: the fork fixes
  // the advisories, and the mitigation stands as defence in depth. The claims
  // below are assembled from parts on purpose — the scan includes this file, so
  // a complete literal here would make the guard fail on itself, the same
  // self-poisoning shape as the test above and
  // `tests/r106-retired-dependencies.test.ts`.
  const STALE_CLAIMS = [
    "ships no" + " fix",
    "no fixed " + "release",
    "locked at 2." + "0.2",
  ];
  // The comments are *not* stripped: the stale claims lived in prose, so a scan
  // of code alone would miss exactly the text this guard exists to ban.
  const SOURCES: Array<[string, URL]> = [
    ["src/calculator.ts", new URL("src/calculator.ts", root)],
    ["tests/r50-calculator.test.ts", new URL("tests/r50-calculator.test.ts", root)],
    ["this guard", new URL(import.meta.url)],
  ];

  for (const [label, url] of SOURCES) {
    const raw = await readFile(url, "utf8");
    for (const claim of STALE_CLAIMS) {
      assert.ok(
        !raw.includes(claim),
        `${label} repeats "${claim}" — the retired upstream's stale claims must not be re-planted`,
      );
    }
  }
});
