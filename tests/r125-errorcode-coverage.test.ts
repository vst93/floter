// R125 · the dictionary's completeness guard for verify-error codes.
//
// R124 confirmed by hand what the row badge had been showing all along: the
// five codes `classify_verify_error` (install.rs) writes into the lock file
// beyond the `ProviderErrorCode` set had no `settings.extensions.errorCode.*`
// entry, so `t()` returned `undefined` and a failed verification read as
// `Unavailable · undefined`.
//
// R125 fills the ten strings (five codes × two languages) and this suite keeps
// the seam closed: every code the Rust classifier can *return* must have a
// key on both sides of the dictionary. The `ProviderErrorCode` set is pinned
// too, so a future enum variant cannot slip through the same gap.
//
// The extraction reads the Rust source rather than a copied list: a code that
// is added to the classifier and left untranslated turns this guard red. The
// keyed run/binding families are deliberately out of scope — they already have
// their own suites (extension-run-errors, extension-binding-errors).
//
// Every load-bearing literal here is assembled from fragments, and the last
// test proves this guard does not spell the keys it scans for.
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

const root = new URL("../", import.meta.url);
const read = (path: string) => readFile(new URL(path, root), "utf8");

// Assembled so this guard does not spell the keys it scans for.
const PREFIX = "settings.extensions." + "errorCode.";
const RENDER_FAILED = "render." + "failed";
const RENDER_RETRY = "render." + "retry";
const I18N = "src/" + "i18n.ts";
const INSTALL = "src-tauri/src/extensions/" + "install.rs";
const ERROR_CODES = "src-tauri/src/extensions/" + "error_codes.rs";

// The five codes this round lands; kept beside the extraction so the
// deliverable is named even if the Rust file is restructured.
const R125_CODES = [
  "integrity-mismatch",
  "runtime-unavailable",
  "manifest-unreadable",
  "provider-failed",
  "verification-failed",
];

const PROVIDER_CODE_COUNT = 13;
const ERROR_CODE_FLOOR = 18;

// Slice a Rust function body by brace matching from its signature. These
// functions carry no braces inside string literals, so a depth count is exact.
const bodyOf = (source: string, signature: string): string => {
  const start = source.indexOf(signature);
  assert.ok(start >= 0, `signature not found: ${signature}`);
  const open = source.indexOf("{", start);
  assert.ok(open >= 0, `body not found: ${signature}`);
  let depth = 0;
  for (let index = open; index < source.length; index++) {
    const char = source[index];
    if (char === "{") depth++;
    else if (char === "}" && --depth === 0) return source.slice(open + 1, index);
  }
  throw new Error(`unbalanced braces after ${signature}`);
};

// A code literal is kebab-case: lower-case alphanumerics joined by hyphens.
// The classifier's prose probes ("runtime is unavailable", "describe") never
// match, so the extraction is the returned codes and nothing else.
const KEBAB = /^[a-z][a-z0-9]*(?:-[a-z0-9]+)+$/;
const kebabLiterals = (body: string): string[] =>
  [...body.matchAll(/"([^"]+)"/g)]
    .map((match) => match[1])
    .filter((literal) => KEBAB.test(literal));

const verifyCodesFrom = (install: string): string[] =>
  kebabLiterals(bodyOf(install, "fn classify_verify_error"));

// The enum's own `as_str` arms are the authority for the ProviderErrorCode
// set; every literal there is a code, including the single-word ones
// (`timeout`, `cancelled`) the kebab filter would drop.
const providerCodesFrom = (errorCodes: string): string[] =>
  [...bodyOf(errorCodes, "fn as_str").matchAll(/=>\s*"([^"]+)"/g)].map(
    (match) => match[1],
  );

const sources = (async () => {
  const [i18n, install, errorCodes] = await Promise.all([
    read(I18N),
    read(INSTALL),
    read(ERROR_CODES),
  ]);
  return { i18n, verifyCodes: verifyCodesFrom(install), providerCodes: providerCodesFrom(errorCodes) };
})();

// How many dictionaries carry `key` verbatim. The dictionary is one file with
// an `en` table and a `zh` table, so a code that exists on both sides counts 2.
const keyHits = (i18n: string, key: string) => i18n.split(`"${key}"`).length - 1;

// ── 1 · every code the classifier can return has a key on both sides ───────

test("the classifier's returned codes are extracted, not assumed", async () => {
  const { verifyCodes } = await sources;
  // Non-hollow: the probe must really have read the branches. Six codes, and
  // the five this round lands among them.
  assert.ok(verifyCodes.length >= 6, `expected ≥6 verify codes, found ${verifyCodes.length}`);
  for (const code of R125_CODES) {
    assert.ok(verifyCodes.includes(code), `${code} must be extracted from the classifier`);
  }
});

test("every classify_verify_error code is translated in both dictionaries", async () => {
  const { i18n, verifyCodes } = await sources;
  for (const code of verifyCodes) {
    const hits = keyHits(i18n, PREFIX + code);
    assert.equal(hits, 2, `${PREFIX}${code} must exist in en and zh (found ${hits})`);
  }
});

test("the five R125 codes are translated and distinct across languages", async () => {
  const { i18n } = await sources;
  for (const code of R125_CODES) {
    const hits = keyHits(i18n, PREFIX + code);
    assert.equal(hits, 2, `${PREFIX}${code} must exist in en and zh (found ${hits})`);
  }
});

// ── 2 · the whole ProviderErrorCode set stays covered ──────────────────────

test("the as_str extraction covers the full ProviderErrorCode set", async () => {
  const { providerCodes } = await sources;
  assert.equal(providerCodes.length, PROVIDER_CODE_COUNT, `found ${providerCodes.length}`);
  assert.equal(new Set(providerCodes).size, PROVIDER_CODE_COUNT, "codes must be unique");
});

test("every ProviderErrorCode has a key on both sides", async () => {
  const { i18n, providerCodes } = await sources;
  for (const code of providerCodes) {
    const hits = keyHits(i18n, PREFIX + code);
    assert.equal(hits, 2, `${PREFIX}${code} must exist in en and zh (found ${hits})`);
  }
});

// ── 3 · the dictionary is not hollow ───────────────────────────────────────

test("the errorCode table carries at least the 13 + 5 codes", async () => {
  const { i18n } = await sources;
  const pattern = new RegExp(`"${PREFIX.replace(/\./g, "\\.")}([a-z0-9-]+)"`, "g");
  const keys = new Set([...i18n.matchAll(pattern)].map((match) => match[1]));
  assert.ok(
    keys.size >= ERROR_CODE_FLOOR,
    `expected ≥${ERROR_CODE_FLOOR} errorCode keys, found ${keys.size}`,
  );
});

test("the R113 render backstop anchors are still present", async () => {
  const { i18n } = await sources;
  assert.equal(keyHits(i18n, RENDER_FAILED), 2, "render.failed must stay in both tables");
  assert.equal(keyHits(i18n, RENDER_RETRY), 2, "render.retry must stay in both tables");
});

// ── 4 · self-proof: this guard assembles what it scans for ─────────────────

test("this guard does not spell the keys it scans for", async () => {
  const self = await read("tests/r125-errorcode-coverage.test.ts");
  const { verifyCodes, providerCodes } = await sources;
  for (const code of [...verifyCodes, ...providerCodes, ...R125_CODES]) {
    assert.ok(
      !self.includes(`"${PREFIX}${code}"`),
      `the guard must not spell the complete key for ${code}`,
    );
  }
  assert.ok(!self.includes(`"${RENDER_FAILED}"`), "the guard must not spell render.failed");
  assert.ok(!self.includes(`"${RENDER_RETRY}"`), "the guard must not spell render.retry");
});
