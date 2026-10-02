// R67 · from "this tool is missing" to "here is the command" — the data layer.
//
// Floter is not a package manager and never runs the install itself. It knows
// where a tool comes from (`tool_catalog.rs`, a compile-time Rust table) and
// which package manager this machine already has (a `stat` scan of the host
// search path), and it renders one command for the user to type into their own
// shell. That division is deliberate: the install runs where the proxy, the
// mirror and the user's environment already are.
//
// This module owns the half the Rust table deliberately does not: the *syntax*
// of each manager's install command. Rust carries data (`manager`, `package`);
// the command string is a presentation decision and lives here. The two sides
// are pinned together by `tests/r67-tool-catalog-guard.test.ts`, which compares
// the tool id set, the platform key set and the manager id set in both
// directions.
//
// It also carries the terminal hand-off (`openInstallSession`). That function
// is *not wired to any button this round* — it exists, and its tests drive it,
// so the R68 UI round can call it without inventing the mechanism. Crucially it
// spawns a **bare** session (`initialCommand: null`) and types the command
// afterwards, so R62's `isCommandStartedSession` stays `false`: an install
// session is the user's own terminal and must never auto-close when the install
// finishes.

/** The platform keys every per-platform table is addressed by. The Rust twin
 *  is `tool_catalog::PLATFORM_KEYS`; the guard compares them. */
export const RECIPE_PLATFORMS = ["macos", "linux", "windows"] as const;

export type InstallPlatform = (typeof RECIPE_PLATFORMS)[number];

/** The package managers Floter can phrase a command for. The Rust twin is
 *  `tool_catalog::PACKAGE_MANAGERS`; the guard compares the id sets. */
export type PackageManagerId =
  | "brew"
  | "winget"
  | "pacman"
  | "apt"
  | "dnf"
  | "npm"
  | "cargo"
  | "pipx";

/** The frontend twin of the Rust `TOOL_CATALOG` id list. It is a *mirror*, not
 *  a second source of truth: the guard fails if the two ever diverge, and R68
 *  uses it to recognise a catalog id in an IPC payload. */
export const TOOL_CATALOG_IDS = [
  "flameshot",
  "yt-dlp",
  "jq",
  "fd",
  "ripgrep",
  "fzf",
  "bat",
  "eza",
  "tldr",
  "httpie",
  "gh",
  "lazygit",
] as const;

export type ToolCatalogId = (typeof TOOL_CATALOG_IDS)[number];

/** One install recipe as the backend reports it: a manager and the package it
 *  installs. No syntax here — that is [`MANAGER_INSTALL_COMMANDS`]. */
export type ToolRecipe = {
  manager: PackageManagerId;
  package: string;
};

/** A per-platform table, mirroring the Rust `ProbeCandidates`/`RecipeTable`. */
export type PlatformTable<T> = Record<InstallPlatform, T>;

/** A catalog entry as `extensions_tool_catalog` returns it. `detected` is a
 *  `stat` hit on a probe candidate name — never a version. */
export type ToolCatalogEntry = {
  id: string;
  displayName: string;
  homepage: string;
  /** R68 · launcher search vocabulary, a pure data addition to the Rust table
   *  (`tool_catalog.rs`). The row builder matches a needle against the id, the
   *  display name and these; no i18n key exists for them, exactly as none
   *  exists for the rest of the catalog. */
  keywords: string[];
  probeCandidates: PlatformTable<string[]>;
  recipes: PlatformTable<ToolRecipe[]>;
  launch: { argv: string[]; description: string; needsTerminal: boolean } | null;
  detected: boolean;
};

/** The IPC payload of `extensions_tool_catalog`. */
export type ToolCatalogReport = {
  platform: InstallPlatform;
  managers: { id: PackageManagerId; displayName: string; detected: boolean }[];
  tools: ToolCatalogEntry[];
};

/**
 * How each manager's install command reads, verbatim.
 *
 * The system managers take `sudo`: the command is typed into the user's own
 * interactive shell, which is exactly where a password prompt belongs, and a
 * permission-denied error is a worse answer than the prompt. The language
 * managers install into the user's own prefix and need no elevation.
 *
 * Deliberately a function table, not a template string: a manager whose
 * argument shape is not `install <package>` (winget's `--id … -e`) stays a
 * single readable rule, and a new manager is one entry rather than a branch in
 * every caller.
 */
export const MANAGER_INSTALL_COMMANDS: Record<
  PackageManagerId,
  (packageName: string) => string
> = {
  brew: (packageName) => `brew install ${packageName}`,
  winget: (packageName) => `winget install --id ${packageName} -e`,
  pacman: (packageName) => `sudo pacman -S ${packageName}`,
  apt: (packageName) => `sudo apt install ${packageName}`,
  dnf: (packageName) => `sudo dnf install ${packageName}`,
  npm: (packageName) => `npm install -g ${packageName}`,
  cargo: (packageName) => `cargo install ${packageName}`,
  pipx: (packageName) => `pipx install ${packageName}`,
};

/**
 * The recipe `installCommand` would render: the first whose manager is present
 * on the machine, else the table's first. Split out so a caller can also name
 * the manager (the install row's right-hand source) without re-implementing the
 * preference order in a second place.
 */
export const chosenRecipe = (
  entry: Pick<ToolCatalogEntry, "recipes">,
  detectedManagers: readonly string[],
  platform: InstallPlatform,
): ToolRecipe | null => {
  const recipes = entry.recipes[platform];
  if (!recipes || recipes.length === 0) return null;
  return (
    recipes.find((candidate) => detectedManagers.includes(candidate.manager)) ??
    recipes[0]
  );
};

/**
 * The install command for `entry` on `platform`, or `null` when the catalog has
 * no recipe there.
 *
 * The first recipe whose manager is present on the machine wins, so a Linux box
 * with `pacman` gets `sudo pacman -S …` rather than a fallback it would have to
 * install first. When none of the machine's managers appears (a minimal host),
 * the table's first recipe is returned: the command is still the honest
 * preferred way to install the tool, and the user's shell will say what is
 * missing — silently returning nothing would be less useful.
 *
 * Pure over its three inputs: the entry comes from the IPC payload, the
 * detected managers from the same payload, and the platform from the caller.
 */
export const installCommand = (
  entry: Pick<ToolCatalogEntry, "recipes">,
  detectedManagers: readonly string[],
  platform: InstallPlatform,
): string | null => {
  const recipe = chosenRecipe(entry, detectedManagers, platform);
  if (!recipe) return null;
  const render = MANAGER_INSTALL_COMMANDS[recipe.manager];
  // A manager the frontend does not know (a backend table that grew ahead of
  // this one) yields no command rather than a half-built one.
  return render ? render(recipe.package) : null;
};

/**
 * The bytes an install session types into the PTY: the command plus the CR that
 * submits it. `term_input` sends keystrokes, so the trailing break is the
 * caller's job (the broker appends `\r` only to a spawn-time `initialCommand`,
 * which this path deliberately does not use). A blank line yields `null` — an
 * install session with nothing to run is not a session.
 */
export const installSessionInput = (commandLine: string): string | null => {
  const line = commandLine.trim();
  return line.length === 0 ? null : `${line}\r`;
};

/**
 * The terminal hand-off: open a bare session and type `commandLine` into it.
 *
 * `ensureTerminalSession` is called with `initialCommand: null` on purpose.
 * Passing the command at spawn time would make R62's `isCommandStartedSession`
 * true, and the page would leave when the install finishes — but an install is
 * not a one-shot command the user is done with: the shell must stay so they can
 * read the output, fix a package name, or run the tool they just installed.
 *
 * `term_input` is the existing keystroke path (the frontend precedent is
 * `useTerminalView.sendTerminalText`), and `"main"` is the main surface's id
 * (`terminalInputTarget`). The function takes its two collaborators as
 * arguments instead of importing them so the node suite can drive it without a
 * DOM or a Tauri host; the R68 UI supplies the real ones.
 */
export type InstallSessionDeps = {
  ensureTerminalSession: (initialCommand: string | null) => Promise<void>;
  invoke: (
    command: "term_input",
    args: { id: string; data: number[] },
  ) => Promise<unknown>;
};

export const openInstallSession = async (
  commandLine: string,
  deps: InstallSessionDeps,
): Promise<boolean> => {
  const input = installSessionInput(commandLine);
  if (input === null) return false;
  // Bare session first (R62 stays false), then the keystrokes.
  await deps.ensureTerminalSession(null);
  await deps.invoke("term_input", {
    id: "main",
    data: Array.from(new TextEncoder().encode(input)),
  });
  return true;
};
