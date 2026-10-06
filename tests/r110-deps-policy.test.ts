// R110 · the dependency-declaration policy, in two directions.
//
// R105→R109 cleared the *state* of the supply chain (rustls patched, six dead
// dependencies deleted, rusqlite on 0.40, `cargo audit` at zero). What they
// left behind was the *policy*: `src-tauri/Cargo.toml` still declared most of
// its dependencies as a bare major (`"1"`, `"3"`, `"5"`) or a bare minor
// (`"0.4"`, `"0.28"`), so `cargo update` had no audited floor to stay above and
// a later round could drift a high-risk crate without anyone noticing.
//
// R110 tightened the high-risk A-class declarations — the ones whose
// transitive tree is large, whose line has a destructive-minor history, or
// which sit on a security boundary (see docs/AGENT-NOTES.md, "依赖政策") — to a
// patch-position floor. This guard is the forward half of that: a fallback to
// the wide pin must be red.
//
// It is also the reverse half. The tauri family (`tauri`, `tauri-build`,
// `tauri-plugin-*`, `tauri-nspanel`) is an explicit exemption: R110 left those
// declarations alone on purpose, because their semver is backed by Tauri's own
// release train and a future C3 upgrade round would collide with a local pin.
// An equals-pin smuggled onto any of them is therefore a policy breach, not a
// tightening, and this file refuses it.
//
// The version strings are assembled from parts on purpose. The self-scan below
// includes this file, so a complete quoted version literal here would make the
// guard fail on itself — the same self-poisoning shape as
// `tests/r109-rusqlite-version.test.ts` and
// `tests/r106-retired-dependencies.test.ts`. Crate *names* are fine to spell
// out: they are package names, not the thing being pinned.

import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

const root = new URL("../", import.meta.url);
const read = (path: string) => readFile(new URL(path, root), "utf8");

const MANIFEST = "src-tauri/Cargo.toml";
const PACKAGE = "package.json";

/**
 * The declarations R110 tightened, each as `major/minor/patch` parts (joined
 * at use) plus the wide pin it replaced. `wide` is what a regression would
 * reach for, so it gets its own assertion and its own failure message.
 */
const TIGHTENED: ReadonlyArray<{
  crate: string;
  parts: readonly string[];
  wide: string;
}> = [
  { crate: "serde", parts: ["1", "0", "229"], wide: "1" },
  { crate: "serde_json", parts: ["1", "0", "151"], wide: "1" },
  { crate: "chrono", parts: ["0", "4", "45"], wide: "0.4" },
  { crate: "tokio", parts: ["1", "53", "1"], wide: "1" },
  { crate: "crossterm", parts: ["0", "28", "1"], wide: "0.28" },
  { crate: "jsonschema", parts: ["0", "33", "0"], wide: "0.33" },
  { crate: "libc", parts: ["0", "2", "189"], wide: "0.2" },
  { crate: "png", parts: ["0", "17", "16"], wide: "0.17" },
  // R112 extended the same policy to three more A-class declarations: the
  // terminal emulator core (large transitive tree), tempfile (filesystem
  // boundary) and arboard (system clipboard / untrusted image data).
  { crate: "alacritty_terminal", parts: ["0", "26", "0"], wide: "0.26" },
  { crate: "tempfile", parts: ["3", "27", "0"], wide: "3" },
  { crate: "arboard", parts: ["3", "6", "1"], wide: "3" },
];

/**
 * R110's budget was "≤ 8 declaration changes"; R112 raised it to 11 by adding
 * the three crates above. The guard pins the size so a later round cannot
 * quietly widen the policy into a mass re-pin.
 */
const TIGHTENED_BUDGET = 11;

const escapeRegExp = (value: string) =>
  value.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");

/**
 * The version requirement on the `crate` line of the manifest, whether the
 * declaration is a bare string (`serde_json = "…"`) or an inline table
 * (`tokio = { version = "…", features = […] }`). The line is matched as a
 * whole declaration, so a comment mentioning the crate cannot satisfy it.
 */
const declaredRequirement = (
  manifest: string,
  crate: string,
): string | null => {
  const declaration = new RegExp(
    `^\\s*${escapeRegExp(crate)}\\s*=\\s*(.+)$`,
    "m",
  );
  const line = manifest.match(declaration)?.[1];
  if (!line) return null;
  return line.match(/version\s*=\s*"([^"]+)"/)?.[1] ?? line.match(/^"([^"]+)"/)?.[1] ?? null;
};

/** Every `name = …` declaration under a `[…dependencies]` table, name → RHS. */
const declaredDependencies = (manifest: string): Map<string, string> => {
  const out = new Map<string, string>();
  const header = /^\s*\[([^\]]+)\]\s*$/;
  const entry = /^\s*([A-Za-z0-9_-]+)\s*=\s*(.+)$/;
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
      if (inline) out.set(inline[2], "");
      inDependencyTable = /(^|\.)(?:dev-|build-)?dependencies$/.test(path);
      continue;
    }
    const declaration = line.match(entry);
    if (inDependencyTable && declaration) out.set(declaration[1], declaration[2].trim());
  }
  return out;
};

/** The tauri family R110 exempted, as the Cargo-side naming spells it. */
const isTauriFamily = (crate: string) =>
  crate === "tauri" || /^tauri[-_]/.test(crate);

/** The requirement inside an inline-table or bare declaration RHS. */
const requirementOf = (rhs: string): string | null =>
  rhs.match(/version\s*=\s*"([^"]+)"/)?.[1] ??
  rhs.match(/^"([^"]+)"/)?.[1] ??
  null;

test("the tightened set is the eleven R110/R112 chose, and no more", () => {
  assert.equal(
    TIGHTENED.length,
    TIGHTENED_BUDGET,
    "R110 tightened eight declarations and R112 three; keep the budget and the list in step",
  );
  assert.equal(
    new Set(TIGHTENED.map((entry) => entry.crate)).size,
    TIGHTENED.length,
    "a duplicated crate would make the scan narrower than it reads",
  );
});

test("the manifest holds the R110 floors and not the wide pins", async () => {
  const manifest = await read(MANIFEST);

  for (const { crate, parts, wide } of TIGHTENED) {
    const version = parts.join(".");
    const declared = declaredRequirement(manifest, crate);

    assert.equal(
      declared,
      version,
      `${MANIFEST} must require \`${crate}\` ${version}`,
    );
    assert.notEqual(
      declared,
      wide,
      `${MANIFEST} fell back to the wide \`${crate}\` pin \`${wide}\` — R110's floor was undone`,
    );
  }
});

test("the tauri family stays exempt and unpinned", async () => {
  const manifest = await read(MANIFEST);
  const declared = declaredDependencies(manifest);

  const family = [...declared.entries()].filter(([crate]) => isTauriFamily(crate));
  assert.ok(
    family.length >= 6,
    `the manifest declares only ${family.length} tauri-family dependencies; that is not the crate`,
  );

  for (const [crate, rhs] of family) {
    const requirement = requirementOf(rhs);
    if (requirement === null) {
      // A git or path dependency carries no semver requirement; it is pinned by
      // `rev`/`path` instead, which is already tighter than any caret. Only the
      // semver declarations are subject to the equals-pin ban.
      assert.ok(
        /\b(?:git|path)\s*=/.test(rhs),
        `${MANIFEST} declares \`${crate}\` without a readable version requirement`,
      );
      continue;
    }
    assert.ok(
      !requirement.startsWith("="),
      `${MANIFEST} equals-pins \`${crate}\`; the tauri family is an explicit R110 exemption`,
    );
  }
});

test("the npm tauri packages stay ranged, not exact", async () => {
  const manifest = JSON.parse(await read(PACKAGE)) as {
    dependencies?: Record<string, string>;
    devDependencies?: Record<string, string>;
  };
  const all = { ...manifest.dependencies, ...manifest.devDependencies };

  const family = Object.entries(all).filter(([name]) =>
    name.startsWith("@tauri-apps/"),
  );
  assert.ok(
    family.length >= 2,
    `package.json declares only ${family.length} @tauri-apps packages; that is not the crate`,
  );

  for (const [name, range] of family) {
    assert.ok(
      /^[\^~]/.test(range),
      `package.json pins \`${name}\` to \`${range}\`; the tauri family keeps its range`,
    );
  }
});

test("this guard assembles the versions, it does not spell them out", async () => {
  const self = await readFile(new URL(import.meta.url), "utf8");

  // A hardcoded version literal here would defeat the point: the scan above
  // reads the requirement, so a literal would make the guard pass by assertion
  // rather than by parsing — and would be exactly the self-poisoning shape the
  // other R1xx guards ban.
  for (const { parts } of TIGHTENED) {
    const version = parts.join(".");
    assert.ok(
      !self.includes(`"${version}"`) && !self.includes(`'${version}'`),
      `the guard must build \`${version}\` from parts, not write it as a literal`,
    );
  }
});
