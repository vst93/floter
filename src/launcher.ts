export type ExecutionMode = "pty" | "external";

export type ExecutionPlan = {
  program: string;
  args: string[];
  mode: ExecutionMode;
  cwd: string | null;
  environment: Record<string, string>;
  inheritEnvironment: boolean;
  planToken?: string;
  argumentOverride?: string[];
};

export type CompletionItem = {
  value: string;
  label: string;
  description: string;
};

export type ActionBarKind = "shell" | "url" | "path";

export type LauncherSelection = {
  actionBar: boolean;
  resultIndex: number;
};

/** Number only runnable results, leaving unavailable discovery rows without a slot. */
export const launcherShortcutSlots = (runnableResults: boolean[]): Array<number | null> => {
  let shortcut = 0;
  return runnableResults.map((runnable) => {
    if (!runnable) return null;
    shortcut += 1;
    return shortcut;
  });
};

export const COMMAND_WORDS = new Set([
  "cd", "git", "npm", "ls", "cat", "echo", "curl", "wget", "ssh",
  "cp", "mv", "rm", "mkdir", "touch", "chmod", "grep", "find",
  "sed", "awk", "make", "docker", "kubectl", "python", "python3",
  "node", "go", "cargo", "brew", "apt", "yum", "pip", "yarn",
  "pnpm", "tar", "gzip", "unzip", "head", "tail", "wc", "sort",
  "uniq", "diff", "kill", "ps", "top", "df", "du", "free", "uname",
  "whoami", "hostname", "ping", "ifconfig", "ip", "netstat", "lsof",
  "systemctl", "journalctl", "man", "which", "whereis", "export",
  "source", "alias", "history", "sudo",
]);

const URL_QUERY = /^(?:https?|ftp):\/\//i;
const PATH_QUERY = /^[/~.]|^[A-Za-z]:[\\/]|^\\\\/;
const ALIAS_SCORE_CAP = 690;

export const normalizeSearch = (value: string): string =>
  value.toLowerCase().normalize("NFKC").replace(/[^\p{L}\p{N}]+/gu, " ").trim();

export type ParsedCommandLine = { tokens: string[]; fragmentStart: number };

/** Tokenize a command line for discovery without asking a shell to interpret it. */
export const parseCommandLine = (value: string, trailingEmpty = false): ParsedCommandLine => {
  const tokens: string[] = [];
  let token = "";
  let tokenStarted = false;
  let tokenStart = value.length;
  let quote: "'" | '"' | null = null;
  let escaped = false;

  const finishToken = () => {
    if (!tokenStarted) return;
    tokens.push(token);
    token = "";
    tokenStarted = false;
  };

  for (let index = 0; index < value.length; index += 1) {
    const char = value[index];
    if (escaped) {
      token += char;
      escaped = false;
      continue;
    }
    if (char === "\\" && quote !== "'") {
      if (!tokenStarted) tokenStart = index;
      tokenStarted = true;
      escaped = true;
      continue;
    }
    if (char === "'" || char === '"') {
      if (!tokenStarted) tokenStart = index;
      tokenStarted = true;
      quote = quote === char ? null : quote ?? char;
      if (quote !== null && quote !== char) token += char;
      continue;
    }
    if (/\s/.test(char) && quote === null) {
      finishToken();
      continue;
    }
    if (!tokenStarted) tokenStart = index;
    tokenStarted = true;
    token += char;
  }
  if (escaped) token += "\\";
  finishToken();
  if (trailingEmpty && /\s$/.test(value)) {
    tokens.push("");
    tokenStart = value.length;
  }
  return { tokens, fragmentStart: tokenStart };
};

const formatCompletionValue = (value: string): string => {
  // Keep simple CLI tokens readable. Everything else is quoted so a value
  // selected with Tab can be parsed back into the exact same argument.
  if (/^[\p{L}\p{N}_./:@%+=,-]+$/u.test(value)) return value;
  return `"${value.replace(/(["\\$`])/g, "\\$1")}"`;
};

export const completedCommandLine = (
  query: string,
  fragmentStart: number,
  completion: CompletionItem,
): string => {
  const value = formatCompletionValue(completion.value);
  const keepOpen = /[\\/]$/.test(completion.value);
  return `${query.slice(0, fragmentStart)}${value}${keepOpen ? "" : " "}`;
};

export const executionWithCompletion = <T extends { execution: ExecutionPlan | null }>(
  entry: T,
  currentTokens: string[],
  completion: CompletionItem,
): ExecutionPlan | null => {
  if (!entry.execution) return null;
  const completedArgs = [...currentTokens.slice(1)];
  if (completedArgs.length) completedArgs[completedArgs.length - 1] = completion.value;
  else completedArgs.push(completion.value);
  return {
    ...entry.execution,
    argumentOverride: completedArgs,
  };
};

/** Score two already-normalized strings for exact, prefix, contains and subsequence matches. */
export const scoreNormalized = (needle: string, haystack: string): number => {
  if (!needle || !haystack || haystack.length < needle.length) return 0;
  if (haystack === needle) return 1000;
  if (haystack.startsWith(needle)) return 900 - haystack.length;
  const contained = haystack.indexOf(needle);
  if (contained !== -1) return 700 - contained;

  let score = 0;
  let cursor = 0;
  for (const char of needle) {
    const index = haystack.indexOf(char, cursor);
    if (index === -1) return 0;
    score += index === cursor ? 12 : 5;
    cursor = index + 1;
  }
  return score;
};

export const scoreApp = (
  needle: string,
  names: string[],
  initials: string,
  aliases: string[],
): number => {
  let best = 0;
  for (const name of names) {
    best = Math.max(best, scoreNormalized(needle, name));
  }
  if (initials) {
    const score = scoreNormalized(needle, initials);
    best = Math.max(best, score >= 1000 ? 950 : score);
  }
  for (const alias of aliases) {
    const score = scoreNormalized(needle, alias);
    if (score >= 700) best = Math.max(best, Math.min(score, ALIAS_SCORE_CAP));
  }
  return best;
};

export const classifyActionBar = (value: string): ActionBarKind => {
  if (URL_QUERY.test(value)) return "url";
  if (PATH_QUERY.test(value)) return "path";
  return "shell";
};

/** Decide which row a fresh query should select before the user navigates. */
export const shouldDefaultToActionBar = (
  query: string,
  actionKind: ActionBarKind,
  resultCount: number,
  runnableResultCount: number,
  hasRunnableCommandResult: boolean,
): boolean => {
  const value = query.trim();
  if (!value) return false;
  if (actionKind !== "shell" || resultCount === 0) return true;
  // Discovery keeps commands whose integration is currently unavailable in
  // the list. They must not capture Enter: unlike an application result or an
  // executable catalog plan, those rows cannot perform the launcher's job.
  if (hasRunnableCommandResult) return false;
  if (runnableResultCount === 0) return true;
  if (/\s/.test(value) || /[|>&]/.test(value)) return true;
  return COMMAND_WORDS.has(value.toLowerCase());
};

/** Move through runnable results and the action bar as one wrapping list. */
export const nextLauncherSelection = (
  runnableResults: boolean[],
  selectedResultIndex: number,
  selectedActionBar: boolean,
  hasActionBar: boolean,
  direction: -1 | 1,
): LauncherSelection => {
  const targets: LauncherSelection[] = runnableResults.flatMap((runnable, resultIndex) =>
    runnable ? [{ actionBar: false, resultIndex }] : [],
  );
  if (hasActionBar) targets.push({ actionBar: true, resultIndex: selectedResultIndex });
  if (!targets.length) return { actionBar: selectedActionBar, resultIndex: selectedResultIndex };

  const currentIndex = targets.findIndex((target) =>
    target.actionBar === selectedActionBar &&
    (target.actionBar || target.resultIndex === selectedResultIndex),
  );
  if (currentIndex >= 0) {
    return targets[(currentIndex + direction + targets.length) % targets.length];
  }

  // A result may have become unavailable between renders, or the pointer may
  // be hovering one for discovery. Continue in the requested visual direction
  // instead of making that disabled row part of the keyboard loop.
  if (direction > 0) {
    return targets.find((target) => !target.actionBar && target.resultIndex > selectedResultIndex)
      ?? targets.find((target) => target.actionBar)
      ?? targets[0];
  }
  for (let index = targets.length - 1; index >= 0; index -= 1) {
    const target = targets[index];
    if (!target.actionBar && target.resultIndex < selectedResultIndex) return target;
  }
  return targets.find((target) => target.actionBar) ?? targets[targets.length - 1];
};
