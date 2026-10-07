// R141 · the export family's join failure is keyed, and nineteen dead keys stay
// deleted.
//
// Task 1 — R135 moved the three export commands' blocking writes off the async
// worker, but the join failure was spelled in English at three anchors and
// reached the toast through `errorMessage`, which passes a non-key through
// verbatim. The backend now sends a dictionary key, a colon and the runtime's
// own error; the panel's KEYED_VALUE_PARAMS table fills the dictionary's
// `{error}` slot — the same keyed-value shape R128 gave the form family and the
// open-URL refusal. No consumer changed: the table is data the existing
// `:`-split branch reads.
//
// Task 2 — the nineteen keys R140 proved dead are gone from both dictionaries.
// This guard pins the set so a rename cannot silently resurrect one.
//
// Every scanned literal is assembled from fragments, and the last test proves
// this guard does not spell the strings it looks for.
import assert from "node:assert/strict";
import { readdir, readFile } from "node:fs/promises";
import test from "node:test";

const root = new URL("../", import.meta.url);
const read = (path: string) => readFile(new URL(path, root), "utf8");
const count = (haystack: string, needle: string) => haystack.split(needle).length - 1;

const EXTENSIONS = "src-tauri/src/commands/" + "extensions.rs";
const I18N = "src/" + "i18n.ts";
const PANEL = "src/" + "ExtensionsPanel.tsx";

/** Remove JS/TS/Rust comments so an anchor can only be satisfied by real code. */
const stripComments = (source: string) =>
  source.replace(/\/\*[\s\S]*?\*\//g, "").replace(/^\s*\/\/.*$/gm, "");

// Assembled so this guard does not spell the strings it scans for.
const KEYED_PREFIX = "settings.extensions." + "exportTaskFailed.";
const exportKey = (what: string) => KEYED_PREFIX + what;
const WHATS = ["extension", "script", "config"];
const ERROR_SLOT = ":" + "{error}";
const OLD_ENGLISH = ["Extension export ", "Script export ", "Configuration export "].map(
  (head) => head + "task failed: ",
);

// The nineteen keys R140 proved dead. Built from a family prefix plus a stem so
// the guard itself never spells a full key.
const SETTINGS = "settings.";
const CLIPBOARD = "clipboard.";
const LAUNCHER = "launcher.";
const DEAD_KEYS = [
  "extensions." + "source",
  LAUNCHER + "browserTabsUnavailable",
  SETTINGS + "group.link",
  SETTINGS + "language.en",
  SETTINGS + "language.zh",
  SETTINGS + "browserTarget",
  SETTINGS + "browserTargetAuto",
  SETTINGS + "browserCustomDir",
  SETTINGS + "browserHistoryDays",
  SETTINGS + "browserSort",
  SETTINGS + "browserSortHint",
  CLIPBOARD + "title",
  CLIPBOARD + "filter",
  CLIPBOARD + "clear",
  CLIPBOARD + "loadFailed",
  CLIPBOARD + "typeColor",
  SETTINGS + "shortcuts",
  SETTINGS + "extensions.customRunning",
  SETTINGS + "extensions.customRunRememberHint",
];

const QUOTES = ['"', "'", "`"];
const quoted = (key: string) => QUOTES.map((quote) => quote + key + quote);

/** Every `.ts` / `.tsx` / `.rs` file under a directory, recursively. */
const sourcesUnder = async (dir: string): Promise<string[]> => {
  const entries = await readdir(new URL(dir + "/", root), { withFileTypes: true });
  const files: string[] = [];
  for (const entry of entries) {
    const child = dir + "/" + entry.name;
    if (entry.isDirectory()) files.push(...(await sourcesUnder(child)));
    else if (/\.(ts|tsx|rs)$/.test(entry.name)) files.push(child);
  }
  return files;
};

test("the three export task failures are keyed at the source", async () => {
  const source = await read(EXTENSIONS);
  assert.equal(count(source, KEYED_PREFIX), 3, "the export family must key all three joins");
  for (const what of WHATS) {
    const key = exportKey(what);
    assert.equal(
      count(source, key + ERROR_SLOT),
      1,
      `${key} must carry the runtime's error after ':'`,
    );
  }
  for (const sentence of OLD_ENGLISH) {
    assert.equal(count(source, sentence), 0, `the raw English sentence must be gone: ${sentence}`);
  }
});

test("the panel's keyed-value table and both dictionaries carry the three keys", async () => {
  const panel = await read(PANEL);
  const i18n = await read(I18N);
  for (const what of WHATS) {
    const key = exportKey(what);
    const line = panel.split("\n").find((candidate) => candidate.includes(`"${key}"`));
    assert.ok(line, `${key} must be listed in KEYED_VALUE_PARAMS`);
    assert.ok(line.includes(`"error"`), `${key} must name the error placeholder`);

    assert.equal(count(i18n, `"${key}":`), 2, `${key} must have an English and a Chinese entry`);
    const entries = i18n.split("\n").filter((candidate) => candidate.includes(`"${key}":`));
    assert.equal(entries.length, 2, `${key} must carry a value on both sides`);
    assert.ok(entries.every((entry) => entry.includes("{error}")), `${key} must interpolate {error}`);
    assert.notEqual(entries[0], entries[1], `${key} languages must not share one string`);
  }
});

test("the nineteen dead keys stay out of every frontend and backend source", async () => {
  const files = [...(await sourcesUnder("src")), ...(await sourcesUnder("src-tauri/src"))];
  assert.ok(files.length > 100, `the scan must cover the tree (${files.length} files)`);
  for (const file of files) {
    const text = stripComments(await read(file));
    for (const key of DEAD_KEYS) {
      for (const literal of quoted(key)) {
        assert.equal(count(text, literal), 0, `${key} must not be resurrected in ${file}`);
      }
    }
  }
});

test("this guard assembles its literals, it does not spell them", async () => {
  const self = await read("tests/r141-export-task-failed-keyed.test.ts");
  for (const literal of [KEYED_PREFIX, ...WHATS.map(exportKey), ...OLD_ENGLISH, ...DEAD_KEYS]) {
    assert.ok(!self.includes(literal), `the guard must not spell: ${literal}`);
  }
});

// ---------------------------------------------------------------------------
// R142 · the generic hygiene sweep this file's third test only sampled.
//
// G2 — R141 proved nineteen keys dead by hand, and a guard that remembers only
// those nineteen cannot see the twentieth. This sweep takes the whole
// dictionary and asks of every key: is it spelled somewhere a reader can see
// (`src/**` + `src-tauri/src/**`, comments stripped), or is it built by one of
// the dynamic sites R140 §1.2.2 catalogued? A key that is neither is dead, and
// the assertion is that no such key exists — so the dead set stays a subset of
// what R141 deleted.
//
// G3 — the same idea on the other half of the i18n contract: a tsx file may not
// carry user-visible English of its own. The scan covers the three surfaces
// R140 §1.3 measured — visible attributes, JSX text nodes, Chinese literals —
// and stays deliberately narrow, because a false positive costs more than a
// missed string (the guard only earns its keep while it stays trustworthy).

/** Every comment form the three languages use, removed so a key named only in
 *  prose cannot pass for a consumer. `[^:]` keeps `https://` intact. */
const stripCommentsDeep = (source: string) =>
  source.replace(/\/\*[\s\S]*?\*\//g, "").replace(/(^|[^:])\/\/[^\n]*/g, "$1");

/** Every key declared at the two-space indent both dictionaries use. */
const declaredKeys = (body: string) =>
  [...body.matchAll(/^ {2}"((?:[^"\\]|\\.)*)":/gm)].map((match) => match[1]);

/** The union of the two dictionaries' key sets, sliced by the same markers
 *  `i18n-symmetry` uses so the guard cannot read the wrong body. */
const dictionaryKeySet = async () => {
  const source = await read(I18N);
  const enStart = source.indexOf("const en = {");
  const enEnd = source.indexOf("export type MessageKey");
  const zhStart = source.indexOf("const zh: Record<MessageKey, string> = {");
  const zhEnd = source.indexOf("const messages: Record<Language");
  assert.ok(enStart >= 0 && enEnd > enStart, "the en dictionary is declared before MessageKey");
  assert.ok(zhStart >= 0 && zhEnd > zhStart, "the zh dictionary is declared before `messages`");
  return [
    ...new Set([
      ...declaredKeys(source.slice(enStart, enEnd)),
      ...declaredKeys(source.slice(zhStart, zhEnd)),
    ]),
  ];
};

// The dynamic producers, assembled from fragments so this guard never spells a
// full pattern. Each anchor names the site R140 §1.2.2 catalogued.
const SETTINGS_PREFIX = "settings" + ".";
const EXTENSIONS_PREFIX = SETTINGS_PREFIX + "extensions" + ".";
const DYNAMIC_PREFIXES = [
  EXTENSIONS_PREFIX + "errorCode.", // ExtensionRow.tsx:258, binding-errors.ts:117
  EXTENSIONS_PREFIX + "pickerClosed.", // ExtensionsPanel.tsx:690 (isMessageKey channel)
  EXTENSIONS_PREFIX + "form.", // ExtensionsPanel.tsx:690 (isMessageKey channel)
  SETTINGS_PREFIX + "terminalTheme.", // terminal-appearance.ts:306
  SETTINGS_PREFIX + "terminalPadding.", // terminal-appearance.ts:310
  SETTINGS_PREFIX + "terminalBold.", // terminal-appearance.ts:315
  EXTENSIONS_PREFIX + "customParamType.", // CustomIntegrationDrawer.tsx:78
  EXTENSIONS_PREFIX + "freshnessResult.", // ExtensionsPanel.tsx:3135
  SETTINGS_PREFIX + "uiScale.", // GeneralPage.tsx:56
  EXTENSIONS_PREFIX + "healthStatus.", // ExtensionsPanel.tsx:2909
  EXTENSIONS_PREFIX + "runtimeSource.", // ExtensionRow.tsx:233
  EXTENSIONS_PREFIX + "status.", // ExtensionRow.tsx:159
  EXTENSIONS_PREFIX + "freshnessDot.", // ExtensionRow.tsx:174
  SETTINGS_PREFIX + "menu.", // App.tsx:2840
  "notification" + ".", // notifications.ts:108
  "shortcut" + ".", // ShortcutsPage.tsx:398
];
// `removalTextKey` (ExtensionsPanel.tsx:2135) builds the family from this cross
// product of stems and suffixes.
const REMOVAL_KEYS = new Set(
  ["deleteCustom", "uninstall", "disconnect", "removePackage"].flatMap((stem) =>
    ["", "Title", "Description"].map((suffix) => EXTENSIONS_PREFIX + stem + suffix),
  ),
);
// CustomIntegrationDrawer.tsx:150 builds the two permission legends from this
// pair of names.
const CUSTOM_PERMISSION_KEYS = new Set([
  EXTENSIONS_PREFIX + "customEnforcedPermissions",
  EXTENSIONS_PREFIX + "customDeclaredPermissions",
]);
const isDynamicKey = (key: string) =>
  DYNAMIC_PREFIXES.some((prefix) => key.startsWith(prefix)) ||
  REMOVAL_KEYS.has(key) ||
  CUSTOM_PERMISSION_KEYS.has(key);

test("every dictionary key is spelled statically or built by a dynamic producer", async () => {
  const keys = await dictionaryKeySet();
  assert.ok(keys.length > 700, `the dictionaries are still the full set (${keys.length})`);
  const files = [...(await sourcesUnder("src")), ...(await sourcesUnder("src-tauri/src"))].filter(
    (file) => file !== I18N,
  );
  assert.ok(files.length > 100, `the scan must cover both trees (${files.length} files)`);
  const scanned = await Promise.all(
    files.map(async (file) => ({ file, text: stripCommentsDeep(await read(file)) })),
  );
  const dead = keys.filter((key) => {
    const literals = quoted(key);
    if (scanned.some(({ text }) => literals.some((literal) => text.includes(literal)))) return false;
    return !isDynamicKey(key);
  });
  assert.deepEqual(dead, [], "a key with no static consumer and no dynamic producer is dead");
});

/** Remove balanced `<…>` type-argument spans — the `<` that follows an
 *  identifier or `)` is a generic, never a JSX tag. This is what turns the six
 *  `Promise<`/`Parameters<`/`RefObject<` pseudo text-nodes R140 §1.3 saw into
 *  the zero text nodes they really are. */
const stripTypeArguments = (source: string) => {
  let out = "";
  let index = 0;
  while (index < source.length) {
    if (source[index] === "<" && index > 0 && /[A-Za-z0-9_$)]/.test(source[index - 1])) {
      let depth = 0;
      let cursor = index;
      let balanced = true;
      while (cursor < source.length) {
        const char = source[cursor];
        if (char === "<") depth += 1;
        else if (char === ">") {
          depth -= 1;
          if (depth === 0) break;
        } else if (char === "\n" || char === ";" || char === "{" || char === "}") {
          balanced = false;
          break;
        }
        cursor += 1;
      }
      if (balanced && cursor < source.length && source[cursor] === ">") {
        index = cursor + 1;
        continue;
      }
    }
    out += source[index];
    index += 1;
  }
  return out;
};

// A JSX attribute, not a property write: `document.title = "Floter"` must not
// read as an attribute, so the name may not follow a `.` or a word character.
const VISIBLE_ATTRIBUTE =
  /(?<![\w.$])(placeholder|title|aria-label|alt)\s*=\s*(?:"([^"]*)"|'([^']*)'|\{\s*"([^"]*)"\s*\}|\{\s*'([^']*)'\s*\}|\{\s*`([^`]*)`\s*\})/g;
// CustomIntegrationDrawer.tsx:93 shows the CLI flag a custom argument looks
// like as a technical example, not prose. Assembled so this guard does not
// spell it. The two template interpolations R140 §1.3 listed never reach the
// scan: a template with `${…}` is dynamic and the scanner skips it.
const TECHNICAL_EXAMPLE = "--" + "target";
const VISIBLE_ATTRIBUTE_ALLOWED = new Set([TECHNICAL_EXAMPLE]);

test("no tsx visible attribute hardcodes English prose", async () => {
  const files = (await sourcesUnder("src")).filter((file) => file.endsWith(".tsx"));
  assert.ok(files.length > 20, `the scan must cover the tsx tree (${files.length} files)`);
  const offenders: string[] = [];
  for (const file of files) {
    const text = stripCommentsDeep(await read(file));
    for (const match of text.matchAll(VISIBLE_ATTRIBUTE)) {
      const value = match[2] ?? match[3] ?? match[4] ?? match[5] ?? match[6] ?? "";
      if (value === "") continue; // `alt=""` is the decorative-image convention
      if (match[6] !== undefined && match[6].includes("${")) continue; // dynamic template
      if (VISIBLE_ATTRIBUTE_ALLOWED.has(value)) continue;
      offenders.push(`${file}: [${match[1]}] ${JSON.stringify(value)}`);
    }
  }
  assert.deepEqual(offenders, [], "a visible attribute may not carry an English literal");
});

// A JSX text node: content between `>` and `<` with no tag or brace. Two or
// more English words, and no code punctuation. The one structural false
// positive the generic strip leaves behind is JSX nested in a ternary
// expression container (`… </span> : integration.x.trim() ? <span>`); its
// text carries `:`/`(`, so it fails the sentence shape.
const JSX_TEXT = />([^<>{}]+)</g;
const SENTENCE_SHAPE = /^[A-Za-z][A-Za-z0-9 ,.'’!?-]*$/;
const ENGLISH_WORD = /[A-Za-z][A-Za-z'’-]*/g;

test("no tsx JSX text node carries a bare English sentence", async () => {
  const files = (await sourcesUnder("src")).filter((file) => file.endsWith(".tsx"));
  const offenders: string[] = [];
  for (const file of files) {
    const text = stripTypeArguments(stripCommentsDeep(await read(file)));
    for (const match of text.matchAll(JSX_TEXT)) {
      const content = match[1].trim();
      if ((content.match(ENGLISH_WORD) ?? []).length < 2) continue;
      if (!SENTENCE_SHAPE.test(content)) continue;
      offenders.push(`${file}: ${JSON.stringify(content)}`);
    }
  }
  assert.deepEqual(offenders, [], "a JSX text node may not carry a bare English sentence");
});

test("no tsx file carries a Chinese literal outside a comment", async () => {
  const files = (await sourcesUnder("src")).filter((file) => file.endsWith(".tsx"));
  const offenders: string[] = [];
  for (const file of files) {
    if (/[\u4e00-\u9fff]/.test(stripCommentsDeep(await read(file)))) offenders.push(file);
  }
  assert.deepEqual(offenders, [], "Chinese belongs in the dictionary, not in a tsx literal");
});

test("the hygiene sweep assembles its keys, patterns and example", async () => {
  const self = await read("tests/r141-export-task-failed-keyed.test.ts");
  for (const key of await dictionaryKeySet()) {
    assert.ok(!self.includes(`"${key}"`), `the guard must not spell: ${key}`);
  }
  for (const prefix of DYNAMIC_PREFIXES) {
    assert.ok(!self.includes(prefix), `the guard must not spell the pattern: ${prefix}`);
  }
  for (const key of [...REMOVAL_KEYS, ...CUSTOM_PERMISSION_KEYS]) {
    assert.ok(!self.includes(`"${key}"`), `the guard must not spell: ${key}`);
  }
  assert.ok(!self.includes(TECHNICAL_EXAMPLE), "the guard must not spell its whitelisted example");
});
