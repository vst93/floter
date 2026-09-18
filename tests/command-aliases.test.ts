// R7-11 · command-level aliases (search priority, no hotkeys).
//
// The user's ask: a single command can carry an alias (`gfm` → `git commit -m`)
// and the alias takes part in launcher matching with the *same* Raycast-shaped
// ladder the command name does — exact > prefix > fuzzy — with an alias exact
// hit scoring exactly what a command-name exact hit scores.
//
// The round has four moving parts and this suite pins one face of each:
//
//   1. **The Rust settings shape.** `command_aliases: HashMap<String, String>`
//      with a bare `#[serde(default)]`, so a settings file written before the
//      key existed deserializes to an empty map. Two commands sharing one alias
//      is the conflict case, resolved by "first command in name order locks it".
//   2. **The shared ladder** (`src/command-aliases.ts`). The alias and the
//      command name go through the *same* `candidateMatchScore`, so the tiers
//      cannot drift apart. `commandMatchScore` is the `max` of the two.
//   3. **The launcher wiring.** The alias map is sent to `catalog_search`, the
//      typed token is rebased onto the real command before execution, and the
//      visible rows are ranked by the shared ladder.
//   4. **The editor + persistence.** One alias input per connected command in
//      the integration detail drawer, wired to a debounced settings mutator —
//      no private write path.
//
// The mutation locks at the bottom replay the two regressions this round is
// most likely to see: an alias demoted below the command-name tier, and an
// alias edit that never reaches the settings save path.
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

import { createTranslator } from "../src/i18n.ts";
import {
  aliasToCommand,
  candidateMatchScore,
  commandMatchScore,
  matchedCommandAlias,
  rebaseAliasCommandLine,
  resolveCommandAliases,
  withCommandAlias,
  MATCH_CONTAINS,
  MATCH_EXACT,
  MATCH_NAME_CONTAINS,
  MATCH_NAME_PREFIX,
  MATCH_PREFIX_BASE,
  type CommandAliases,
} from "../src/command-aliases.ts";

const root = new URL("../", import.meta.url);
const read = (path: string) => readFile(new URL(path, root), "utf8");

/** The body of a top-level `pub fn`, sliced to the next top-level item. */
const fnBody = (source: string, signature: string) => {
  const at = source.indexOf(signature);
  assert.notEqual(at, -1, `${signature} must exist`);
  const rest = source.slice(at + signature.length);
  const end = rest.search(/\n\}\n/);
  assert.notEqual(end, -1, `${signature} must be a braced function`);
  return rest.slice(0, end);
};

// ── 1 · the search ladder ──────────────────────────────────────────────────

test("the three tiers have a strict order and an exact alias lands on the exact tier", () => {
  const aliases: CommandAliases = { git: "gfm" };
  // Alias exact == command exact. This is the core of the user story.
  assert.equal(commandMatchScore("gfm", "git", aliases.git), MATCH_EXACT);
  assert.equal(commandMatchScore("git", "git", aliases.git), MATCH_EXACT);
  // Prefix sits below exact, above a bare contains.
  const prefix = commandMatchScore("gf", "git", aliases.git);
  assert.ok(prefix < MATCH_EXACT && prefix >= MATCH_PREFIX_BASE - 100, `prefix tier: ${prefix}`);
  // A contains-only haystack is the fuzzy tier.
  assert.equal(commandMatchScore("fm", "git", aliases.git), MATCH_CONTAINS);
  // The display name never outranks the command or the alias.
  assert.ok(commandMatchScore("json", "other", undefined, "JSON Viewer") <= MATCH_NAME_PREFIX);
  assert.ok(commandMatchScore("son", "other", undefined, "JSON Viewer") <= MATCH_NAME_CONTAINS);
  // No match at all.
  assert.equal(commandMatchScore("zzz", "git", aliases.git, "Git"), 0);
});

test("an exact alias is not demoted below a command-name prefix", () => {
  // The Raycast ordering the user asked for: an alias exact hit beats another
  // command's prefix hit. If the alias were scored on the name tier (or with a
  // cap like the application aliases use) this assertion would fail.
  const aliasScore = commandMatchScore("gfm", "git-commit", "gfm");
  const competingPrefix = commandMatchScore("gfm", "gfm-tool");
  assert.equal(aliasScore, MATCH_EXACT);
  assert.ok(aliasScore > competingPrefix, "alias exact must outrank a command prefix");
});

test("prefix length is penalised, so a longer tail sorts lower", () => {
  assert.ok(candidateMatchScore("gi", "git") > candidateMatchScore("gi", "github-cli"));
  assert.ok(candidateMatchScore("gi", "github-cli") < MATCH_PREFIX_BASE);
});

// ── 2 · conflict policy, blanks, and the key ───────────────────────────────

test("a shared alias is locked by the first command in name order", () => {
  // Two commands pointed at one alias — deterministic regardless of object key
  // order: `alpha` wins because it sorts first, not because it was inserted
  // first. The loser's command name still matches normally.
  const raw: CommandAliases = { zeta: "shared", alpha: "shared" };
  const resolved = resolveCommandAliases(raw);
  assert.deepEqual(resolved, { alpha: "shared" });
  assert.equal(commandMatchScore("shared", "zeta", resolved.zeta), 0);
  assert.ok(commandMatchScore("zeta", "zeta", resolved.zeta) > 0);
});

test("the command name is the unique key of the alias map", () => {
  // Keyed by command name: writing a second alias for one command overwrites
  // the first rather than accumulating two aliases for it.
  let aliases = withCommandAlias({}, "git", "g");
  aliases = withCommandAlias(aliases, "git", "gfm");
  assert.deepEqual(aliases, { git: "gfm" });
  assert.equal(Object.keys(aliases).length, 1);
});

test("a blank alias removes the entry instead of matching everything", () => {
  const aliases = withCommandAlias({ git: "gfm" }, "git", "   ");
  assert.deepEqual(aliases, {});
  // And a blank field is not a match-everything candidate either.
  assert.equal(resolveCommandAliases({ git: "  " }).git, undefined);
});

test("an unchanged alias is a reference no-op, so no write is queued", () => {
  const raw: CommandAliases = { git: "gfm" };
  // `withCommandAlias` returns the same object by identity; the settings
  // mutator's `next === settingsRef.current.command_aliases` guard is what
  // turns that into a skipped save.
  assert.equal(withCommandAlias(raw, "git", "gfm"), raw);
  assert.equal(withCommandAlias(raw, "git", "  gfm  "), raw);
});

// ── 3 · typed alias resolving back to the real command ─────────────────────

test("the alias map inverts to alias → command for completion", () => {
  assert.deepEqual(aliasToCommand({ git: "gfm", kubectl: "k" }), { gfm: "git", k: "kubectl" });
  // The conflict policy applies to the inverse too: one target per alias.
  assert.deepEqual(aliasToCommand({ zeta: "shared", alpha: "shared" }), { shared: "alpha" });
});

test("only a whole-token alias is treated as an alias for execution", () => {
  const aliases: CommandAliases = { git: "gfm" };
  assert.equal(matchedCommandAlias(aliases, "git", "gfm"), "gfm");
  assert.equal(matchedCommandAlias(aliases, "git", "GFM"), "gfm", "case-insensitive");
  // A prefix is a discovery surface, not an execution alias: the row still
  // shows for `gf`, but the user is not yet running a half-typed name.
  assert.equal(matchedCommandAlias(aliases, "git", "gf"), undefined);
  assert.equal(matchedCommandAlias(aliases, "git", ""), undefined);
  assert.equal(matchedCommandAlias(aliases, "other", "gfm"), undefined);
});

test("an exact alias runs the real command, preserving the rest of the line", () => {
  // The user story's own example: `gfm` → `git commit -m`.
  assert.equal(rebaseAliasCommandLine("gfm", "gfm", "git"), "git");
  assert.equal(
    rebaseAliasCommandLine('gfm commit -m "hello world"', "gfm", "git"),
    'git commit -m "hello world"',
  );
  // A leading environment assignment is not the command token.
  assert.equal(
    rebaseAliasCommandLine("FOO=gfm gfm status", "gfm", "git"),
    "FOO=gfm git status",
  );
  // A token that merely contains the alias is untouched.
  assert.equal(rebaseAliasCommandLine("gfmx", "gfm", "git"), "gfmx");
});

// ── 4 · the Rust settings shape and its migration ──────────────────────────

test("the Rust settings carry command_aliases with a bare serde default", async () => {
  const rust = await read("src-tauri/src/commands/config.rs");
  assert.match(
    rust,
    /#\[serde\(default\)\]\s*\n\s*pub command_aliases: HashMap<String, String>,/,
    "the field must be `command_aliases: HashMap<String, String>` with `#[serde(default)]`",
  );
  // A bare `#[serde(default)]` is the migration device: an older settings file
  // has no key and must come back as an empty map, not a deserialize failure.
  const defaults = rust.slice(
    rust.indexOf("impl Default for AppSettings"),
    rust.indexOf("/// Platform defaults"),
  );
  assert.match(defaults, /command_aliases: HashMap::new\(\)/, "the default is empty");
});

test("the Rust side applies the same conflict policy before scoring", async () => {
  const rust = await read("src-tauri/src/commands/config.rs");
  const resolver = fnBody(rust, "pub fn resolve_command_aliases(");
  // Ascending name order + a claimed set is the whole policy.
  assert.match(resolver, /commands\.sort\(\)/, "commands are visited in name order");
  assert.match(resolver, /claimed\.insert/, "an alias is claimed once");
  const catalog = await read("src-tauri/src/extensions/catalog.rs");
  // The backend must not trust a caller's already-resolved map: it resolves
  // again (idempotently) so its ranking matches the frontend's.
  assert.match(
    catalog,
    /crate::commands::config::resolve_command_aliases\(&request\.command_aliases\)/,
    "catalog::search resolves the alias map before scoring",
  );
  // Alias exact shares the command-name exact tier: the `max` over the two
  // candidate strings is the contract.
  const score = catalog.slice(catalog.indexOf("fn score_entry("), catalog.indexOf("fn namespace_for("));
  assert.match(score, /candidate_score\(needle, &alias\.to_ascii_lowercase\(\)\)/, "the alias is scored as a candidate");
  assert.match(score, /\.max\(candidate_score/, "alias and command name share one max");
});

test("the search request carries the alias map across the bridge", async () => {
  const catalog = await read("src-tauri/src/extensions/catalog.rs");
  assert.match(
    catalog,
    /pub command_aliases: HashMap<String, String>,/,
    "CatalogSearchRequest carries the map",
  );
  const hook = await read("src/hooks/useLauncherCatalog.ts");
  assert.match(hook, /commandAliases: resolvedAliases,/, "the frontend sends the resolved map");
  assert.match(hook, /resolveCommandAliases\(commandAliases\)/, "and resolves it first");
  // Completion is asked of the provider by the real command id: a typed alias
  // is inverted before `catalog_complete`, and the visible rows are ranked by
  // the shared ladder rather than the backend's command-only order.
  assert.match(
    hook,
    /aliasToCommand\(resolvedAliases\)\[command\.toLowerCase\(\)\] \?\? command/,
    "completion resolves an alias back to the provider's command id",
  );
  assert.match(hook, /commandMatchScore\(\s*\n?\s*commandNeedle,\s*\n?\s*suggestion\.entry\.command/, "rows are ranked by the shared ladder");
});

// ── 5 · the editor and its persistence path ────────────────────────────────

test("the settings hook owns a debounced alias mutator on the shared save path", async () => {
  const hook = await read("src/hooks/useSettings.ts");
  assert.match(hook, /command_aliases: \{\}/, "the frontend default is an empty map");
  assert.match(
    hook,
    /command_aliases: loaded\.command_aliases \?\? \{\}/,
    "hydration backfills a pre-R7-11 response",
  );
  const mutator = hook.slice(hook.indexOf("const changeCommandAlias = useCallback"));
  const body = mutator.slice(0, mutator.indexOf("\n  );"));
  // It rides the settings debounce and the ordinary serialized writer — no
  // private invoke, because a text field would otherwise save per keystroke.
  assert.match(body, /SETTINGS_DEBOUNCE_MS/, "the write is debounced");
  assert.match(body, /persistSettings\(\)/, "the mutator persists through the one writer");
  assert.match(hook, /invoke\("save_settings", \{ settings: next \}\)/, "persistence is the settings command");
  assert.ok(!/invoke\(\s*"(set_alias|save_alias|command_alias)/.test(hook), "no dedicated command");
});

test("the integration detail drawer is the alias editor", async () => {
  const panel = await read("src/ExtensionsPanel.tsx");
  // One input per connected command, in the command list the drawer already
  // renders — the alias is edited where the command is named.
  assert.match(panel, /commandAliases: CommandAliases;/, "the panel receives the map");
  assert.match(panel, /onChangeCommandAlias: \(command: string, alias: string\) => void;/, "and the mutator");
  assert.match(panel, /className="extension-command-alias__input"/, "the input exists");
  // The key is the catalog command id (the searchable string), not the label.
  assert.match(panel, /commandAliases\[command\.id\] \?\? ""/, "the field is keyed by command id");
  assert.match(
    panel,
    /onChangeCommandAlias\(command\.id, event\.target\.value\)/,
    "the edit writes the command id, not the display name",
  );
  // And the App passes the settings field down by identity.
  const app = await read("src/App.tsx");
  assert.match(app, /commandAliases=\{settings\.command_aliases\}/, "the settings map is passed by identity");
  assert.match(app, /onChangeCommandAlias=\{changeCommandAlias\}/, "wired to the settings mutator");
  assert.match(app, /command_aliases: CommandAliases;/, "the AppSettings type carries the field");
  assert.match(app, /commandAliases: settings\.command_aliases,/, "the launcher receives the map too");
  // The conflict policy is surfaced, not hidden: a claimed alias is flagged in
  // the row rather than silently mis-ranking.
  assert.match(panel, /resolveCommandAliases\(commandAliases\)/, "the editor resolves the policy");
  assert.match(panel, /t\("settings\.extensions\.commandAliasTaken"\)/, "the taken alias is labelled");
});

// ── 6 · i18n symmetry ──────────────────────────────────────────────────────

test("the alias keys are declared in both dictionaries and are real Chinese", () => {
  const keys = [
    "settings.extensions.commandAlias",
    "settings.extensions.commandAliasFor",
    "settings.extensions.commandAliasPlaceholder",
    "settings.extensions.commandAliasTaken",
  ] as const;
  const en = createTranslator("en");
  const zh = createTranslator("zh");
  for (const key of keys) {
    assert.ok(en(key).length > 0, `${key} must have en text`);
    assert.ok(zh(key).length > 0, `${key} must have zh text`);
    assert.notEqual(zh(key), en(key), `${key} must be translated, not an English fallback`);
    assert.ok(/[\u4e00-\u9fff]/.test(zh(key)), `${key} must contain Chinese text`);
  }
  assert.equal(en("settings.extensions.commandAlias"), "Alias");
  assert.equal(zh("settings.extensions.commandAlias"), "命令别名");
  // `{command}` is interpolated on both sides, never left literal.
  assert.ok(!en("settings.extensions.commandAliasFor", { command: "git" }).includes("{"));
  assert.ok(!zh("settings.extensions.commandAliasFor", { command: "git" }).includes("{"));
  // The alias-match hint rides the result subtitle; it is a launcher key, not a
  // settings one, and interpolates its alias on both sides.
  assert.ok(/[\u4e00-\u9fff]/.test(zh("launcher.aliasMatch", { alias: "gfm" })));
  assert.ok(!en("launcher.aliasMatch", { alias: "gfm" }).includes("{"));
  assert.ok(!zh("launcher.aliasMatch", { alias: "gfm" }).includes("{"));
});

// ── 7 · mutation locks ─────────────────────────────────────────────────────

/** The predicate the priority lock asserts: an exact alias is an exact match.
 *  Demoting the alias below the command-name tier must fail it. */
const aliasSharesTheExactTier = (score: (needle: string, command: string, alias?: string) => number) =>
  score("gfm", "git", "gfm") === MATCH_EXACT && score("gfm", "gfm-tool") < MATCH_EXACT;

test("mutation: demoting the alias below the command-name tier goes red", async () => {
  assert.ok(aliasSharesTheExactTier((n, c, a) => commandMatchScore(n, c, a)), "the shipped ladder is the contract");
  // The mutation: the alias is folded through the name tier (a cap or a
  // separate branch), which is the "alias is second class" bug.
  const demoted = (needle: string, command: string, alias?: string) =>
    Math.max(candidateMatchScore(needle, command.toLowerCase()), MATCH_NAME_CONTAINS);
  assert.ok(!aliasSharesTheExactTier(demoted), "a name-tier alias must fail the lock");

  // The source really does use one shared tier for both candidate strings.
  const source = await read("src/command-aliases.ts");
  const score = source.slice(source.indexOf("export const commandMatchScore"), source.indexOf("export const resolveCommandAliases"));
  assert.match(score, /candidateMatchScore\(needle, command\.toLowerCase\(\)\)/, "command name uses the ladder");
  assert.match(score, /candidateMatchScore\(needle, userAlias\.toLowerCase\(\)\)/, "the alias uses the same ladder");
});

/** The predicate the save lock asserts: an alias edit reaches `persistSettings`.
 *  An edit that only calls `setSettings` has no persistence at all. */
const aliasEditPersists = (source: string) => {
  const at = source.indexOf("const changeCommandAlias = useCallback");
  if (at === -1) return false;
  const body = source.slice(at, source.indexOf("\n  );", at));
  return body.includes("persistSettings()");
};

test("mutation: an alias edit that skips the save path goes red", async () => {
  const hook = await read("src/hooks/useSettings.ts");
  assert.ok(aliasEditPersists(hook), "the shipped mutator persists");
  // The mutation: the write is dropped, so the alias is lost on restart.
  const mutated = hook.replace(
    /(const changeCommandAlias = useCallback[\s\S]*?)persistSettings\(\)/,
    "$1Promise.resolve()",
  );
  assert.notEqual(mutated, hook, "the mutation must land");
  assert.ok(!aliasEditPersists(mutated), "an edit with no persist call must fail the lock");

  // And the mutator must exist at all — deleting it is the other shape.
  assert.ok(!aliasEditPersists(hook.replace(/const changeCommandAlias = useCallback/, "const gone = useCallback")));
});
