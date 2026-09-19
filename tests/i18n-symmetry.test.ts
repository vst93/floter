// R-DOGF-1 · the i18n symmetry sweep.
//
// Two guards already claim to keep the dictionaries "balanced"
// (`extension-freshness`, `extension-freshness-dot`), but both compare key
// *counts*. A rename in one dictionary plus an addition in the other keeps the
// counts equal and leaves both green, while the renamed key silently renders an
// English fallback in the Chinese UI — the exact drift a symmetry guard exists
// to catch. `zh` being typed `Record<MessageKey, string>` catches a *missing*
// key at compile time, but not a value that interpolates a different name, nor
// a string that was never translated.
//
// This file compares the key *sets*, the placeholder *sets* per key, and the
// set of byte-identical values. Each of those is a distinct way the two
// languages can drift, and each turns one assertion red on its own.

import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

const root = new URL("../", import.meta.url);
const read = (path: string) => readFile(new URL(path, root), "utf8");

/** The two dictionary bodies, taken between their declaration markers so the
 *  slice cannot silently swallow a neighbour. `en` stops at the exported
 *  `MessageKey` type; `zh` stops at the `messages` table that consumes both. */
const dictionaries = async () => {
  const source = await read("src/i18n.ts");
  const enStart = source.indexOf("const en = {");
  const enEnd = source.indexOf("export type MessageKey");
  const zhStart = source.indexOf("const zh: Record<MessageKey, string> = {");
  const zhEnd = source.indexOf("const messages: Record<Language");
  assert.ok(enStart >= 0 && enEnd > enStart, "the en dictionary is declared before MessageKey");
  assert.ok(zhStart >= 0 && zhEnd > zhStart, "the zh dictionary is declared before `messages`");
  return { en: source.slice(enStart, enEnd), zh: source.slice(zhStart, zhEnd) };
};

/** Every key declared at the two-space indent the dictionaries use. This sees a
 *  key even when its value sits on the following line. */
const declaredKeys = (body: string) =>
  [...body.matchAll(/^ {2}"((?:[^"\\]|\\.)*)":/gm)].map((match) => match[1]);

/** Every key→value pair; the value is quoted or a template and may start on the
 *  next line (the long hint strings do). */
const entries = (body: string) => {
  const found = new Map<string, string>();
  const entry =
    /^ {2}"((?:[^"\\]|\\.)*)":\s*(?:"((?:[^"\\]|\\.)*)"|`((?:[^`\\]|\\.)*)`)\s*,?\s*$/gm;
  let match: RegExpExecArray | null;
  while ((match = entry.exec(body))) found.set(match[1], match[2] ?? match[3]);
  return found;
};

const placeholders = (value: string) =>
  [...value.matchAll(/\{(\w+)\}/g)].map((match) => match[1]).sort();

// The values that are legitimately identical in both languages: three format
// fragments, an acronym and the product name. Every other key must carry a
// translation. Pinning the set means a *new* untranslated string fails the
// sweep instead of passing as "identical, therefore fine".
const IDENTICAL_VALUES_ALLOWED = new Set([
  "settings.extensions.freshnessCommandsValue", // "{count}"
  "settings.extensions.reprobeNoticeDeltaIncrease", // "+{count}"
  "settings.extensions.reprobeNoticeDeltaDecrease", // "−{count}"
  "settings.extensions.customId", // "ID"
  "notification.title", // "floter"
]);

test("the two dictionaries declare exactly the same keys", async () => {
  const { en, zh } = await dictionaries();
  const enKeys = declaredKeys(en);
  const zhKeys = declaredKeys(zh);
  const onlyEn = enKeys.filter((key) => !zhKeys.includes(key));
  const onlyZh = zhKeys.filter((key) => !enKeys.includes(key));
  assert.deepEqual(onlyEn, [], "no key may exist only in en");
  assert.deepEqual(onlyZh, [], "no key may exist only in zh");
  assert.equal(enKeys.length, zhKeys.length, "the counts agree once the sets do");
  assert.ok(enKeys.length > 500, `the dictionaries are still the full set (${enKeys.length})`);
});

test("neither dictionary declares a key twice", async () => {
  const { en, zh } = await dictionaries();
  for (const [name, body] of [
    ["en", en],
    ["zh", zh],
  ] as const) {
    const keys = declaredKeys(body);
    const duplicates = keys.filter((key, index) => keys.indexOf(key) !== index);
    assert.deepEqual(duplicates, [], `${name} must not declare a key twice`);
  }
});

test("every key interpolates the same placeholders in both languages", async () => {
  const { en, zh } = await dictionaries();
  const enEntries = entries(en);
  const zhEntries = entries(zh);
  assert.equal(enEntries.size, declaredKeys(en).length, "every en entry parsed");
  assert.equal(zhEntries.size, declaredKeys(zh).length, "every zh entry parsed");
  for (const [key, value] of enEntries) {
    const translated = zhEntries.get(key);
    if (translated === undefined) continue;
    assert.deepEqual(
      placeholders(translated),
      placeholders(value),
      `${key} must interpolate the same names in both languages`,
    );
  }
});

test("only the pinned technical values are identical, and the rest are translated", async () => {
  const { en, zh } = await dictionaries();
  const enEntries = entries(en);
  const zhEntries = entries(zh);
  const identical = [...enEntries]
    .filter(([key, value]) => zhEntries.get(key) === value)
    .map(([key]) => key)
    .sort();
  assert.deepEqual(
    identical,
    [...IDENTICAL_VALUES_ALLOWED].sort(),
    "the byte-identical set is exactly the documented technical values",
  );
  // And every value that does differ from en is really Chinese.
  const untranslated = [...enEntries]
    .filter(([key, value]) => {
      const translated = zhEntries.get(key);
      return translated !== undefined && translated !== value && !/[\u4e00-\u9fff]/.test(translated);
    })
    .map(([key]) => key);
  assert.deepEqual(untranslated, [], "a value that differs from en must contain Chinese");
});

test("the guarded slices are the dictionaries the runtime loads", async () => {
  // A guard on the wrong slice would stay green while the live table drifted:
  // `en` is the `MessageKey` source and `zh` is typed against it, so both names
  // must appear in the `messages` table the translator reads.
  const source = await read("src/i18n.ts");
  assert.match(
    source,
    /const messages: Record<Language, Record<MessageKey, string>> = \{ en, zh \}/,
    "the translator reads exactly these two tables",
  );
});
