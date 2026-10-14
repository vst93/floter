package launcher

import (
	"strings"

	"floter/internal/settings"
)

// R7-11 · the alias rebase, on the run path.
//
// A user alias is a *search* surface: typing `gfm status` reaches the catalog
// row for `git`, and running that row must run `git status`, never a shell
// program literally called `gfm` (which does not exist). The old build owned
// this in `src/command-aliases.ts`'s `rebaseAliasCommandLine` and consulted it
// from `useLauncherCatalog.ts` when it built a row's command line:
//
//   - `aliasToCommand` inverts the resolved map (first command locks an alias),
//   - `rebaseAliasCommandLine` swaps the leading token that is *exactly* the
//     alias for the real command name, at a token boundary, leaving every
//     other byte (arguments, quoting) untouched.
//
// The collapsed launcher here runs a typed command line through
// `ParseCommandLine` (see actionbar.go), which is the same "program and its
// arguments" the old row's `commandLine` carried. `RebaseAliasArgv` applies
// the identical rule to that parsed argv: only the command token is eligible,
// so an environment assignment whose *value* happens to spell the alias is
// never touched, and a prefix of an alias (`gfmt`) stays a program of its own.

// aliasToCommand inverts the resolved alias map: alias (lowercased) -> the
// command it stands for. The resolution is what makes the inversion
// deterministic — two commands may be pointed at the same alias, and the
// first in ascending name order owns it, exactly as `resolveCommandAliases`
// decides for the search.
func aliasToCommand(aliases settings.CommandAliases) map[string]string {
	inverse := map[string]string{}
	for command, alias := range settings.ResolveCommandAliases(aliases) {
		inverse[strings.ToLower(alias)] = command
	}
	return inverse
}

// RebaseAliasArgv replaces a leading command token that is exactly one of the
// user's aliases with the command the alias stands for, so `gfm status` runs
// `git status`. Everything after the command token is left byte for byte.
//
// Only an exact, whole-token, case-insensitive match rebases: a prefix or a
// contains hit is a discovery surface and must not silently run a different
// program than the one typed, which is the same distinction the old build's
// `matchedCommandAlias` drew before it rebased.
func RebaseAliasArgv(aliases settings.CommandAliases, argv []string) []string {
	if len(argv) == 0 {
		return argv
	}
	command, ok := aliasToCommand(aliases)[strings.ToLower(argv[0])]
	if !ok {
		return argv
	}
	rebased := make([]string, len(argv))
	copy(rebased, argv)
	rebased[0] = command
	return rebased
}
