// R40 · the one rule that enters a plugin mode: a trigger word, a space, and
// the rest.
//
// R31 lifted the built-in plugin modes out of the query text and gave them a
// single transition, `pluginModeEntry`; R39 gave external commands the same
// transition, `externalPluginModeEntry`. Both rest on the same tiny rule — the
// first whitespace-separated word names the plugin (its id or an alias) and the
// value must have *something after it* (at least one whitespace character), so
// the bare word stays an ordinary query and the mode is a deliberate place.
//
// That rule was spelled out three times, in two regex dialects (`(.*)$/s` in the
// two built-in parsers, `([\s\S]*)$` in the external one), which is how a
// documented invariant ("the rule is deliberately the same for built-ins and
// external commands") drifts from the code that is supposed to hold it. This
// module is the rule, written once.
//
// Pure: no React, no Tauri, no DOM.

/**
 * Split `value` at its first whitespace run: the lowercased trigger word and
 * everything after the whitespace, verbatim. `null` when there is no whitespace
 * (a bare word) or nothing before it.
 *
 * `rest` is **not** trimmed — the callers disagree about what to do with it (the
 * built-in parsers trim it into a needle; the external argv splitter needs the
 * raw text so a trailing space or an empty quoted argument survives). It is also
 * allowed to be empty: `"bookmarks "` enters the mode with an empty needle.
 */
export const splitTriggerWord = (
  value: string,
): { word: string; rest: string } | null => {
  const match = /^(\S+)\s+([\s\S]*)$/.exec(value);
  if (!match) return null;
  return { word: match[1].toLowerCase(), rest: match[2] };
};
