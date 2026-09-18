// R7-11 · command-level aliases for launcher search.
//
// A user can point an alias at a single catalog command (key = the command name
// the catalog searches on, value = the alias the user types). The alias then
// participates in launcher matching *as a candidate string at the same tiers as
// the command name*: exact > prefix > fuzzy (contains/subsequence), with an
// alias exact hit scoring exactly what a command-name exact hit scores.
//
// Pure and DOM-free so the node suite can drive the three tiers, the conflict
// policy and the "an empty alias removes the entry" rule directly. `launcher.ts`
// and `useLauncherCatalog.ts` are the only consumers; the same three tiers are
// mirrored in `src-tauri/src/extensions/catalog.rs` for the backend search that
// produced the candidate set in the first place.

/** Key = command name (`git`), value = the alias the user types (`gfm`). */
export type CommandAliases = Record<string, string>;

/** The three tiers, as numbers, in descending priority. */
export const MATCH_EXACT = 1_000;
export const MATCH_PREFIX_BASE = 800;
/** A command / alias that merely contains the needle. */
export const MATCH_CONTAINS = 600;
/** The human-readable name starts with the needle. */
export const MATCH_NAME_PREFIX = 500;
/** The human-readable name (or a descriptor alias) merely contains it. */
export const MATCH_NAME_CONTAINS = 300;

/** Longest-tail a prefix match may be penalised for, so the tier stays above
 *  the contains tier for anything short of an absurd command name. */
const PREFIX_PENALTY_CAP = 100;

/**
 * The tier score of one already-normalized candidate string.
 *
 * `needle` and `candidate` are both lowercased by the caller: the whole point
 * of the shared helper is that a command name and an alias run through the very
 * same ladder, so an alias exact hit cannot drift to a different tier.
 */
export const candidateMatchScore = (needle: string, candidate: string): number => {
  if (!needle || !candidate || candidate.length < needle.length) return 0;
  if (candidate === needle) return MATCH_EXACT;
  if (candidate.startsWith(needle)) {
    return MATCH_PREFIX_BASE - Math.min(candidate.length - needle.length, PREFIX_PENALTY_CAP);
  }
  if (candidate.includes(needle)) return MATCH_CONTAINS;
  return 0;
};

/**
 * The best score for one catalog command against `needle`.
 *
 * The command name and the user's alias are both candidate strings, so the
 * `Math.max` is what makes "alias exact == command exact" true rather than
 * something a later edit can quietly break. The display name and the
 * descriptor's own aliases sit below both — the same shape as `score_entry` in
 * `src-tauri/src/extensions/catalog.rs`, so a row the backend matched is not
 * silently re-ranked to zero here.
 */
export const commandMatchScore = (
  needle: string,
  command: string,
  userAlias?: string,
  name?: string,
  descriptorAliases: readonly string[] = [],
): number => {
  if (!needle) return 0;
  let best = candidateMatchScore(needle, command.toLowerCase());
  if (userAlias) {
    best = Math.max(best, candidateMatchScore(needle, userAlias.toLowerCase()));
  }
  if (name) {
    const normalized = name.toLowerCase();
    if (normalized.startsWith(needle)) best = Math.max(best, MATCH_NAME_PREFIX);
    else if (normalized.includes(needle)) best = Math.max(best, MATCH_NAME_CONTAINS);
  }
  if (descriptorAliases.some((alias) => alias.toLowerCase().includes(needle))) {
    best = Math.max(best, MATCH_NAME_CONTAINS);
  }
  return best;
};

/**
 * Apply the conflict policy to the raw settings map.
 *
 * Two commands may be pointed at the same alias — the settings map is keyed by
 * command name, so nothing stops it. The policy is **first command locks the
 * alias**: commands are visited in ascending name order and the first claim
 * wins; a later duplicate is inert (its command name still matches normally).
 * Deterministic regardless of object/`HashMap` iteration order, and it never
 * rewrites what the user typed. Empty and whitespace-only aliases are dropped,
 * so clearing a field removes the entry rather than leaving a match-everything
 * blank string.
 */
export const resolveCommandAliases = (raw: CommandAliases): CommandAliases => {
  const resolved: CommandAliases = {};
  const claimed = new Set<string>();
  for (const command of Object.keys(raw).sort()) {
    const alias = (raw[command] ?? "").trim();
    if (!alias) continue;
    const key = alias.toLowerCase();
    if (claimed.has(key)) continue;
    claimed.add(key);
    resolved[command] = alias;
  }
  return resolved;
};

/** `alias -> command`, for resolving a typed alias back to its command before
 *  asking the backend to complete it. */
export const aliasToCommand = (raw: CommandAliases): Record<string, string> => {
  const resolved = resolveCommandAliases(raw);
  const inverse: Record<string, string> = {};
  for (const [command, alias] of Object.entries(resolved)) {
    inverse[alias.toLowerCase()] = command;
  }
  return inverse;
};

/**
 * The alias of `command`, when the user actually typed that alias instead of the
 * command name.
 *
 * `typedToken` is the command token the user typed. Only a case-insensitive
 * whole-token equal to the resolved alias counts, so `gf` (a prefix) still
 * matches the row but is *not* treated as the alias for execution purposes:
 * the user would be running a half-typed name. This is what keeps a prefix
 * match a discovery surface and an exact alias the thing that runs the command.
 */
export const matchedCommandAlias = (
  raw: CommandAliases,
  command: string,
  typedToken: string,
): string | undefined => {
  const alias = resolveCommandAliases(raw)[command];
  if (!alias || !typedToken) return undefined;
  return alias.toLowerCase() === typedToken.toLowerCase() ? alias : undefined;
};

/**
 * Replace the typed command token with the real command name, preserving every
 * other byte of the line (quoting, escapes, arguments).
 *
 * The alias is a *search* surface: typing `gfm -m x` reaches the row for `git`,
 * and the row must run `git -m x`, not a shell command literally called `gfm`
 * that does not exist. The match is anchored at a token boundary and skips the
 * leading `NAME=value` assignments (their text cannot start a token), so an
 * environment value that happens to contain the alias is left alone.
 */
export const rebaseAliasCommandLine = (
  query: string,
  typedToken: string,
  command: string,
): string => {
  if (!typedToken) return query;
  const escaped = typedToken.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
  return query.replace(new RegExp(`(^|\\s)${escaped}(?=\\s|$)`), (_match, lead: string) => `${lead}${command}`);
};

/**
 * The next raw settings map after editing one command's alias. An empty or
 * whitespace-only value deletes the key, so a cleared field is a real removal
 * and not an entry that matches every query. Setting the alias it already holds
 * returns `raw` by reference, which is what lets the settings mutator treat an
 * unchanged value as a no-op and skip the write.
 */
export const withCommandAlias = (
  raw: CommandAliases,
  command: string,
  alias: string,
): CommandAliases => {
  const trimmed = alias.trim();
  if (trimmed === (raw[command] ?? "").trim()) return raw;
  const next = { ...raw };
  if (trimmed) next[command] = alias;
  else delete next[command];
  return next;
};
