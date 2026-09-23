// R32 · the launcher's token search — one rule, shared by every plugin list.
//
// The user's report, verbatim: 「搜索逻辑需要支持空格分割的且逻辑」. Until this
// round every list matched its needle as a single substring: `rust async` looked
// for the literal string `rust async` and found nothing, even though a row titled
// "Async Rust" contains both words. The rule is now the obvious one — split the
// needle on whitespace and require **every** token to be found — and it is the
// same rule for bookmarks, history, tabs and clipboard entries, so a query that
// works in one plugin's list works in the others.
//
// Case-insensitive, like the substring rule it replaces, and token-internal
// matching is still `contains`: the split only changes how many needles there
// are, not how each one is tested. An empty needle (or one that is only spaces)
// is no filter at all, which is what the default views need.
//
// This module is pure — no React, no Tauri, no DOM — so the node suite can pin
// the rule without the app runtime, the same arrangement `launcher/plugin-mode.ts`
// and `result-budget.ts` use.

/** Split a needle into its AND tokens: lowercased, whitespace-separated,
 *  empties dropped. `""`, `"   "` and `"  a  "` are `[]` and `["a"]`. */
export const searchTokens = (needle: string): string[] =>
  needle.trim().toLowerCase().split(/\s+/).filter(Boolean);

/**
 * Whether every token is found in at least one of the row's fields.
 *
 * `fields` are the row's own haystacks — a browser row passes title and/or URL
 * depending on the configured search field, a clipboard entry passes whatever
 * its kind can be searched. Nullish and empty fields are dropped so a missing
 * title cannot accidentally satisfy a token. With no tokens the answer is `true`:
 * an empty query matches everything.
 */
export const matchesTokens = (
  tokens: readonly string[],
  fields: readonly (string | null | undefined)[],
): boolean => {
  if (!tokens.length) return true;
  const haystacks = fields
    .filter((field): field is string => typeof field === "string" && field.length > 0)
    .map((field) => field.toLowerCase());
  return tokens.every((token) => haystacks.some((haystack) => haystack.includes(token)));
};
