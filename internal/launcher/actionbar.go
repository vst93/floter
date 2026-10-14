package launcher

import (
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"floter/internal/i18n"
)

// i18nLauncher is the launcher's copy type, for the helpers above.
type i18nLauncher = i18n.Launcher

// The action bar: what the field itself is asking for, before any catalog
// matches.
//
// A user who types a URL means "open this", one who types a path means "open
// this" (or a terminal beside it), and one who types anything else that looks
// like a command means "run it". The old build rendered these as a bar under
// the field, which Enter defaulted to; here they are the first rows of the
// same list, so the keys and the pointer reach them exactly like every other
// result.

// URLQuery matches a page address: its scheme is what makes it one, not its
// shape. The scheme is case-insensitive, as the old build's own pattern was:
// `HTTPS://` is a page the user pasted, not a command.
var URLQuery = regexp.MustCompile(`(?i)^(?:https?|ftp)://`)

// PathQuery matches a filesystem path: an absolute POSIX path, one starting
// with `~` or `.`, a Windows drive, or a UNC share.
var PathQuery = regexp.MustCompile(`^[/~.]|^[A-Za-z]:[\\/]|^\\\\`)

// commandWords are the commands whose bare name means "run me in a terminal",
// so Enter defaults to the shell row for them even when the catalog also has
// matches.
var commandWords = map[string]bool{
	"cd": true, "git": true, "npm": true, "ls": true, "cat": true, "echo": true,
	"curl": true, "wget": true, "ssh": true, "cp": true, "mv": true, "rm": true,
	"mkdir": true, "touch": true, "chmod": true, "grep": true, "find": true,
	"sed": true, "awk": true, "make": true, "docker": true, "kubectl": true,
	"python": true, "python3": true, "node": true, "go": true, "cargo": true,
	"brew": true, "apt": true, "yum": true, "pip": true, "yarn": true,
	"pnpm": true, "tar": true, "gzip": true, "unzip": true, "head": true,
	"tail": true, "wc": true, "sort": true, "uniq": true, "diff": true,
	"kill": true, "ps": true, "top": true, "df": true, "du": true,
	"free": true, "uname": true, "whoami": true, "hostname": true,
	"ping": true, "ifconfig": true, "ip": true, "netstat": true,
	"lsof": true, "systemctl": true, "journalctl": true, "man": true,
	"which": true, "whereis": true, "export": true, "source": true,
	"alias": true, "history": true, "sudo": true,
}

// The action kinds the field can ask for.
const (
	actionShell = "shell"
	actionURL   = "url"
	actionPath  = "path"
)

// classifyActionBar is what the query is asking for: a page address, a
// filesystem path, or a command to run. A query with no content is nothing at
// all.
func classifyActionBar(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if URLQuery.MatchString(value) {
		return actionURL
	}
	if PathQuery.MatchString(value) {
		return actionPath
	}
	return actionShell
}

// actionBarItems is the action bar as result rows, one or two per query.
//
// A URL opens, and can be copied. A path that exists opens in the file
// manager (or the file manager beside it, when it is a file), has a terminal
// row for its directory, and a copy row.
//
// A shell command's row is built here unconditionally: it is what the old
// build's Enter handler ran for a list with no results (`useLauncherActions`'
// `!launcherResults.length` fallback). Whether the row is *drawn* is the
// search's decision (see actionBarItemsWith).
func (a *App) actionBarItems() []Item {
	return a.actionBarItemsWith(nil, false)
}

// actionBarItemsWith is the action bar over the results the search already
// built, so a shell command is only offered when it would be the thing Enter
// runs.
//
// The row's *presence* is not gated on `defaultsToShell`: the old build's
// `actionBar` was non-null for every non-empty query, and the row was merely
// not the default selection when a runnable command matched. The rewrite hid
// the row in that case, so a query that matched a catalog command offered no
// way to run it in a terminal at all — the report 「底部应固定出现「在终端中
// 打开」选项…目前缺失」. The default selection (not the presence) is what
// `defaultsToShell` still decides; see App.defaultSelection.
//
// `gated` is the search's call (the drawn list); the other callers
// (activate's no-result fallback, the held-modifier's terminal row) reach the
// row directly and must not lose it over an empty page.
func (a *App) actionBarItemsWith(other []Item, gated bool) []Item {
	kind := classifyActionBar(a.Query)
	if kind == "" {
		return nil
	}
	// The shell fallback is a *secondary* control for a matched list: the old
	// build's `visibleActionBar` drew the bar only when `launcherResults` was
	// non-empty (or a drop had put rows up). A command that matches nothing
	// draws no terminal row — the fallback is not offered over an empty page —
	// though Enter still runs it (activate's no-result fallback, the reference's
	// `!launcherResults.length`). The gate reads the *matches* (`other`), not
	// the field's own text, so a command line that matched a catalogue row
	// still offers its terminal hand-off; only the default selection moves off
	// it (see App.defaultSelection). A URL or a path is the query's *primary*
	// action, not a fallback, so those rows stay unconditional (the reference
	// drew its bar for them even when the catalogue matched nothing).
	if gated && kind == actionShell && len(other) == 0 {
		return nil
	}
	value := strings.TrimSpace(a.Query)
	copy := a.copy()
	if kind == actionURL {
		if _, err := url.Parse(value); err != nil {
			return nil
		}
		return []Item{{
			ID:       "bar:url",
			Title:    copy.OpenInBrowser,
			Detail:   value,
			Shortcut: a.enterBadge(),
			Run: func() {
				if a.Actions.OpenURL != nil {
					a.Actions.OpenURL("", value)
				}
				a.ResetQuery()
				a.Hide()
			},
		}}
	}
	if kind == actionPath {
		expanded := expandPath(value, a.homeDirOrEmpty())
		info, err := os.Stat(expanded)
		if err != nil {
			// A path that is not there yet is still offered: creating it is
			// what the command is for. The terminal row stands beside it.
			return []Item{a.barPathRows(expanded, value, false, copy)[1]}
		}
		rows := a.barPathRows(expanded, value, info.IsDir(), copy)
		out := []Item{rows[0]}
		out = append(out, rows[1:]...)
		return out
	}
	// A shell command: run it in the terminal surface, whose session is the
	// user's own. The tokens come from the structured parse (so a quote never
	// splits a path), and the leading NAME=value assignments ride the
	// command's environment rather than being re-interpreted by a shell.
	parsed := ParseCommandLine(value)
	// A line that carries shell syntax (a pipe, a redirect, a chain) is not a
	// program with arguments: no argv can say `ls | grep x`. The old build's
	// run entry typed the line into the user's own shell, which is the only
	// thing that can read it — so that is the route here, and the structured
	// argv route is kept for the lines that really are a program and its
	// arguments. The env assignments need no special case on this path: a
	// shell reads them itself.
	if parsed.ShellSyntax {
		return []Item{{
			ID:       "bar:shell",
			Title:    copy.RunInShell,
			Detail:   value,
			Shortcut: a.enterBadge(),
			Run: func() {
				a.historyRemember(value)
				if a.Actions.RunInShell != nil {
					a.Actions.RunInShell(value)
				}
			},
		}}
	}
	argv := parsed.Tokens
	if parsed.CommandIndex < 0 {
		return nil // assignments and nothing to run
	}
	if parsed.CommandIndex > 0 {
		// The leading NAME=value assignments ride the command's environment;
		// the tokens that remain are the command and its arguments.
		argv = parsed.Tokens[parsed.CommandIndex:]
	}
	// R7-11: an exact alias is a search surface only. `gfm status` names the
	// `git` row, so the run must reach `git status` and never a program called
	// `gfm` that does not exist. The environment path shares the argv, so the
	// rebase happens once, before either run route.
	argv = RebaseAliasArgv(a.Aliases, argv)
	if len(parsed.Environment) > 0 {
		return []Item{{
			ID:       "bar:shell",
			Title:    copy.RunInShell,
			Detail:   value,
			Shortcut: a.enterBadge(),
			Run: func() {
				a.historyRemember(value)
				if a.Actions.RunInTerminalWithEnv != nil {
					env := make([]string, 0, len(parsed.Environment))
					for key, set := range parsed.Environment {
						env = append(env, key+"="+set)
					}
					a.Actions.RunInTerminalWithEnv(argv, env)
				}
			},
		}}
	}
	return []Item{{
		ID:       "bar:shell",
		Title:    copy.RunInShell,
		Detail:   value,
		Shortcut: a.enterBadge(),
		Run: func() {
			a.historyRemember(value)
			if a.Actions.RunInTerminal != nil {
				a.Actions.RunInTerminal(argv)
			}
		},
	}}
}

// barPathRows is a path's rows: open, a terminal beside it, and a copy of the
// path itself.
func (a *App) barPathRows(expanded, raw string, isDir bool, copy i18nLauncher) []Item {
	directory := expanded
	title := copy.OpenInFiles
	if !isDir {
		directory = filepath.Dir(expanded)
		title = copy.OpenFile
	}
	quoted := shellQuoteForCD(directory)
	return []Item{
		{
			ID:       "bar:path-open",
			Title:    title,
			Detail:   expanded,
			Shortcut: a.enterBadge(),
			Run: func() {
				if a.Actions.OpenPath != nil {
					a.Actions.OpenPath(expanded)
				}
				a.ResetQuery()
				a.Hide()
			},
		},
		{
			ID:       "bar:path-cd",
			Title:    copy.FileCd,
			Detail:   directory,
			Shortcut: a.enterBadge(),
			Run: func() {
				if a.Actions.OpenInTerminal != nil {
					a.Actions.OpenInTerminal(directory)
				}
				a.ResetQuery()
				a.Hide()
			},
		},
		{
			ID:     "bar:path-copy",
			Title:  copy.FileCopyPath + "  " + filepath.Base(expanded),
			Detail: quoted,
			Run: func() {
				if a.Actions.Copy != nil {
					a.Actions.Copy(expanded)
					a.toast = copy.Copied
				}
			},
		},
	}
}

// expandPath resolves `~` and a relative path against the user's home.
func expandPath(value, home string) string {
	expanded := filepath.FromSlash(strings.TrimSpace(value))
	if expanded == "~" {
		return home
	}
	if rest, ok := strings.CutPrefix(expanded, "~/"); ok && home != "" {
		return filepath.Join(home, rest)
	}
	if !filepath.IsAbs(expanded) && home != "" {
		return filepath.Join(home, expanded)
	}
	return expanded
}

// homeDirOrEmpty is the user's home, empty when the platform cannot name one.
func (a *App) homeDirOrEmpty() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return home
}

// shellQuoteForCD quotes a directory for the interactive shell, the way the
// drops package does for its own `cd`.
func shellQuoteForCD(directory string) string {
	if !strings.ContainsAny(directory, " \t'\"\\$`&|;<>()*?[]{}~!#") {
		return directory
	}
	return "'" + strings.ReplaceAll(directory, "'", `'\''`) + "'"
}

// defaultsToShell is whether Enter should land on the shell row rather than
// the catalog's matches: the old build's rule, which keeps a command the user
// typed from being captured by a discovery row that cannot run it.
func (a *App) defaultsToShell(results []Item) bool {
	value := strings.TrimSpace(a.Query)
	if value == "" || classifyActionBar(value) != actionShell {
		return false
	}
	runnable := 0
	hasCommand := false
	for _, item := range results {
		if item.Run == nil {
			continue
		}
		runnable++
		if item.entry != nil {
			hasCommand = true
		}
	}
	if runnable == 0 {
		return true
	}
	if hasCommand {
		return false
	}
	if strings.ContainsAny(value, "|>&") || strings.Contains(value, " ") {
		return true
	}
	return commandWords[strings.ToLower(value)]
}
